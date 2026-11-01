package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"hypervane/internal/crypto"
	"hypervane/internal/hub"
	"hypervane/internal/poller"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

// liveCluster is a simulated Proxmox whose resources can change while a socket
// is open.
type liveCluster struct {
	mu     sync.Mutex
	vmids  []int
	failed bool
	hung   bool
	ticket string // when set, only this ticket is accepted
}

func (c *liveCluster) set(vmids ...int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vmids = vmids
}

func (c *liveCluster) fail(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failed = on
}

func (c *liveCluster) hang(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hung = on
}

func (c *liveCluster) acceptOnly(ticket string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ticket = ticket
}

func (c *liveCluster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	hung := c.hung
	c.mu.Unlock()
	if hung {
		<-r.Context().Done()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	if c.ticket != "" && !strings.Contains(r.Header.Get("Cookie"), c.ticket) {
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	var data any = []any{}
	switch r.URL.Path {
	case "/api2/json/access/permissions":
		data = map[string]any{"/vms": map[string]any{"VM.Audit": 1}}
	case "/api2/json/cluster/resources":
		rows := []map[string]any{}
		for _, id := range c.vmids {
			rows = append(rows, map[string]any{"id": "qemu/" + itoa(id), "type": "qemu", "vmid": id})
		}
		data = rows
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// openLiveSocket connects a real websocket to handleWS as a Proxmox user who
// inherits VM.Audit.
func openLiveSocket(t *testing.T, cluster *liveCluster, maxAge time.Duration) (*Server, *liveSocket) {
	t.Helper()
	srv := httptest.NewServer(cluster)
	t.Cleanup(srv.Close)
	box, err := crypto.NewBox("test-only-master-key")
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(nil)
	s := &Server{box: box, hub: h, poller: poller.New(nil, h, nil, nil), socketACLEvery: 20 * time.Millisecond, socketACLMaxAge: maxAge, aclTTL: time.Millisecond}
	s.putClient("lab-a", testClient(t, srv.URL))
	sess := store.Session{ID: "s1", ConnectionID: "lab-a"}
	if err := s.sealTicket(&sess, "PVE:alice@pve:ticket", "csrf"); err != nil {
		t.Fatal(err)
	}
	row := &sessionRow{sess: sess}
	s.sessionLookup = row.load
	t.Cleanup(func() { rows.Delete(s) })
	rows.Store(s, row)
	user := store.User{Kind: store.UserKindPVE, PVEUserID: "alice@pve"}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleWS(w, withViewer(r, user, sess))
	}))
	t.Cleanup(api.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(api.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "topics": []string{"resources"}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	socket := &liveSocket{conn: conn, events: make(chan []byte, 256)}
	go socket.pump()
	return s, socket
}

// liveSocket reads on one goroutine, because a read deadline would end the
// connection for good.
type liveSocket struct {
	conn   *websocket.Conn
	events chan []byte
}

func (l *liveSocket) pump() {
	defer close(l.events)
	for {
		_, raw, err := l.conn.ReadMessage()
		if err != nil {
			return
		}
		l.events <- raw
	}
}

func (l *liveSocket) WriteJSON(v any) error { return l.conn.WriteJSON(v) }

// next returns the next event, or nil if none arrives in time.
func (l *liveSocket) next(wait time.Duration) []byte {
	select {
	case raw := <-l.events:
		return raw
	case <-time.After(wait):
		return nil
	}
}

// sessionRow stands in for the sessions table.
type sessionRow struct {
	mu   sync.Mutex
	sess store.Session
}

func (r *sessionRow) load(context.Context, string) (store.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sess, nil
}

var rows sync.Map

// rotateTicket does what a request elsewhere does when it refreshes the ticket.
func rotateTicket(t *testing.T, s *Server, ticket string) {
	t.Helper()
	v, _ := rows.Load(s)
	row := v.(*sessionRow)
	row.mu.Lock()
	defer row.mu.Unlock()
	if err := s.sealTicket(&row.sess, ticket, "csrf"); err != nil {
		t.Fatal(err)
	}
}

// waitForDetail publishes a guest's status until the socket receives it.
func waitForDetail(t *testing.T, s *Server, conn *liveSocket, topic string, want bool) {
	t.Helper()
	deadline := time.Now().Add(1500 * time.Millisecond)
	if want {
		deadline = time.Now().Add(3 * time.Second)
	}
	for time.Now().Before(deadline) {
		s.hub.Publish(context.Background(), topic, map[string]any{"name": "vm"})
		var ev struct {
			Topic string `json:"topic"`
		}
		if json.Unmarshal(conn.next(50*time.Millisecond), &ev) != nil {
			continue
		}
		if ev.Topic == topic {
			if !want {
				t.Fatalf("%s was delivered", topic)
			}
			return
		}
	}
	if want {
		t.Fatalf("%s never arrived", topic)
	}
}

// waitForGuests publishes both guests until the socket shows the wanted set.
func waitForGuests(t *testing.T, s *Server, conn *liveSocket, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last int
	for time.Now().Before(deadline) {
		s.hub.Publish(context.Background(), "resources", map[string]any{
			"connection": "lab-a",
			"resources": []pve.Resource{
				{ID: "qemu/100", Type: "qemu", VMID: 100},
				{ID: "qemu/200", Type: "qemu", VMID: 200},
			},
		})
		var ev struct {
			Topic string `json:"topic"`
			Data  struct {
				Resources []pve.Resource `json:"resources"`
			} `json:"data"`
		}
		if json.Unmarshal(conn.next(100*time.Millisecond), &ev) != nil || ev.Topic != "resources" {
			continue
		}
		last = len(ev.Data.Resources)
		if last == want {
			return
		}
	}
	t.Fatalf("socket showed %d guests, want %d", last, want)
}

func TestOpenSocketFollowsPermissionChanges(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, time.Minute)
	waitForGuests(t, s, conn, 1)

	cluster.set(100, 200)
	waitForGuests(t, s, conn, 2)

	cluster.set(100)
	waitForGuests(t, s, conn, 1)
}

func TestOpenSocketKeepsItsListThroughABriefOutage(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, time.Minute)
	waitForGuests(t, s, conn, 1)
	cluster.fail(true)
	time.Sleep(100 * time.Millisecond)
	waitForGuests(t, s, conn, 1)
}

func TestOpenSocketFailsClosedWhenTheOutageLasts(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, 100*time.Millisecond)
	waitForGuests(t, s, conn, 1)
	cluster.fail(true)
	waitForGuests(t, s, conn, 0)
}

// Opening a guest just before the refresh must not leave its page dead.
func TestRefusedSubscriptionRecoversAfterRefresh(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, time.Minute)
	topic := hub.ScopedTopic("lab-a", "guest:pve1:qemu:200")
	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "topics": []string{topic}}); err != nil {
		t.Fatal(err)
	}
	waitForDetail(t, s, conn, topic, false)
	cluster.set(100, 200)
	waitForDetail(t, s, conn, topic, true)
}

func TestRevokedSubscriptionStopsAndCanReturn(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, time.Minute)
	topic := hub.ScopedTopic("lab-a", "guest:pve1:qemu:100")
	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "topics": []string{topic}}); err != nil {
		t.Fatal(err)
	}
	waitForDetail(t, s, conn, topic, true)
	cluster.set()
	time.Sleep(150 * time.Millisecond)
	waitForDetail(t, s, conn, topic, false)
	cluster.set(100)
	waitForDetail(t, s, conn, topic, true)
}

// A socket outlives its first ticket by hours, so it has to use the current one.
func TestOpenSocketFollowsATicketRefreshedElsewhere(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, time.Minute)
	waitForGuests(t, s, conn, 1)
	rotateTicket(t, s, "PVE:alice@pve:newer")
	cluster.acceptOnly("PVE:alice@pve:newer")
	cluster.set(100, 200)
	waitForGuests(t, s, conn, 2)
}

// The cap has to hold while Proxmox hangs, not only when it answers with an error.
func TestOpenSocketFailsClosedDuringAHang(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, 200*time.Millisecond)
	waitForGuests(t, s, conn, 1)
	cluster.hang(true)
	t.Cleanup(func() { cluster.hang(false) })
	waitForGuests(t, s, conn, 0)
}

// A socket that failed closed has to come back once Proxmox does.
func TestOpenSocketRecoversAfterFailingClosed(t *testing.T) {
	cluster := &liveCluster{vmids: []int{100}}
	s, conn := openLiveSocket(t, cluster, 100*time.Millisecond)
	waitForGuests(t, s, conn, 1)
	cluster.fail(true)
	waitForGuests(t, s, conn, 0)
	cluster.fail(false)
	waitForGuests(t, s, conn, 1)
}

// refreshProbe watches a running refresh from inside its lookups, so a test can
// see what a lookup starts with instead of guessing from timing.
type refreshProbe struct {
	viewer *atomic.Pointer[socketViewer]
	topics *socketTopics
	topic  string
}

// blanked reports that the viewer lost the guest and the poller stopped being
// asked for it.
func (p refreshProbe) blanked() bool {
	acl := p.viewer.Load().acl
	if acl == nil || len(acl) != 0 {
		return false
	}
	p.topics.mu.Lock()
	defer p.topics.mu.Unlock()
	return !hasTopic(p.topics.held, p.topic) && hasTopic(p.topics.pending, p.topic)
}

func startRefresh(t *testing.T, every, maxAge time.Duration, lookup func(context.Context, refreshProbe) (store.Session, error)) refreshProbe {
	t.Helper()
	topics, viewer := topicsFor(t, vm100ACL())
	probe := refreshProbe{viewer: viewer, topics: topics, topic: hub.ScopedTopic(labA, "guest:pve1:qemu:100")}
	topics.subscribe([]string{probe.topic}, viewer)
	if !hasTopic(topics.held, probe.topic) {
		t.Fatalf("held %v", topics.held)
	}
	s := &Server{socketACLEvery: every, socketACLMaxAge: maxAge}
	s.sessionLookup = func(ctx context.Context, _ string) (store.Session, error) { return lookup(ctx, probe) }
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.refreshSocketViewer(context.Background(), store.Session{}, store.User{}, viewer, topics, stop)
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})
	return probe
}

// Fast failures up to the cap, then a hang. Only an expired answer earns a
// lookup a full budget, so one that long must start with the guest already gone,
// and must leave it gone. No tick lands on the cap, so the old answer can't be
// blanked by a failure that happens to arrive at the boundary.
func TestRefreshBlanksBeforeARecoveryLookupStarts(t *testing.T) {
	const every, maxAge = 30 * time.Millisecond, 100 * time.Millisecond
	recoveries := make(chan bool, 8)
	startRefresh(t, every, maxAge, func(ctx context.Context, p refreshProbe) (store.Session, error) {
		deadline, _ := ctx.Deadline()
		if time.Until(deadline) > maxAge-every/2 {
			select {
			case recoveries <- p.blanked():
			default:
			}
			<-ctx.Done()
			return store.Session{}, ctx.Err()
		}
		return store.Session{}, errors.New("unavailable")
	})
	for range 2 {
		select {
		case blanked := <-recoveries:
			if !blanked {
				t.Fatal("a recovery lookup started with the expired answer still in place")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no recovery lookup started")
		}
	}
}

// With no tick due, only the cap itself can end the old answer.
func TestRefreshExpiresBetweenTicks(t *testing.T) {
	probe := startRefresh(t, time.Hour, 50*time.Millisecond, func(context.Context, refreshProbe) (store.Session, error) {
		return store.Session{}, errors.New("unavailable")
	})
	deadline := time.Now().Add(3 * time.Second)
	for !probe.blanked() {
		if time.Now().After(deadline) {
			t.Fatal("the answer outlived its cap waiting for a tick")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
