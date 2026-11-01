package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func (c *liveCluster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failed {
		http.Error(w, "unavailable", http.StatusBadGateway)
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
func openLiveSocket(t *testing.T, cluster *liveCluster, maxAge time.Duration) (*Server, *websocket.Conn) {
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
	return s, conn
}

// waitForGuests publishes both guests until the socket shows the wanted set.
func waitForGuests(t *testing.T, s *Server, conn *websocket.Conn, want int) {
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
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		var ev struct {
			Data struct {
				Resources []pve.Resource `json:"resources"`
			} `json:"data"`
		}
		if err := conn.ReadJSON(&ev); err != nil {
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
