package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"hypervane/internal/hub"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

const (
	// sessionCheckEvery is how often a long-lived socket re-checks its session.
	sessionCheckEvery = 20 * time.Second

	socketACLEvery  = 10 * time.Second
	socketACLMaxAge = time.Minute
)

// socketViewer is who a socket serves. A nil acl is an unrestricted viewer.
type socketViewer struct {
	acl        pve.ACL
	connection string
	userid     string
}

type wsCommand struct {
	Type   string   `json:"type"`
	Topics []string `json:"topics"`
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	up := upgrader()
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	defer s.closeWithSession(r, conn)()

	client := s.hub.Register()
	defer client.Close()

	held := map[string]struct{}{}
	var heldMu sync.Mutex

	acl, filter := s.requestACL(r)
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	user, _ := r.Context().Value(userKey).(store.User)
	var viewer atomic.Pointer[socketViewer]
	viewer.Store(&socketViewer{acl: acl, connection: sess.ConnectionID, userid: user.PVEUserID})
	if filter {
		stop := make(chan struct{})
		defer close(stop)
		go s.refreshSocketViewer(r, &viewer, stop)
	}

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// App-level writes from the read goroutine must join this loop.
	// gorilla/websocket allows one writer; WriteJSON pong used to race WriteMessage.
	writes := make(chan []byte, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var cmd wsCommand
			if json.Unmarshal(raw, &cmd) != nil {
				continue
			}
			switch cmd.Type {
			case "subscribe":
				heldMu.Lock()
				for _, t := range cmd.Topics {
					if _, dup := held[t]; dup || !topicAllowed(t, *viewer.Load()) {
						continue
					}
					held[t] = struct{}{}
					s.poller.Watch(t)
					client.Subscribe([]string{t})
				}
				heldMu.Unlock()
			case "unsubscribe":
				heldMu.Lock()
				for _, t := range cmd.Topics {
					if _, ok := held[t]; ok {
						delete(held, t)
						s.poller.Unwatch(t)
						client.Unsubscribe([]string{t})
					}
				}
				heldMu.Unlock()
			case "ping":
				select {
				case writes <- []byte(`{"type":"pong"}`):
				default:
				}
			}
		}
	}()
	defer func() {
		_ = conn.Close()
		<-done
		heldMu.Lock()
		defer heldMu.Unlock()
		for t := range held {
			s.poller.Unwatch(t)
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	writeText := func(msg []byte) bool {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("ws write: %v", err)
			return false
		}
		return true
	}

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case payload := <-writes:
			if !writeText(payload) {
				return
			}
		case msg, ok := <-client.Send():
			if !ok {
				return
			}
			if filter {
				filtered, keep := filterHubMessage(msg, *viewer.Load())
				if !keep {
					continue
				}
				msg = filtered
			}
			if !writeText(msg) {
				return
			}
		}
	}
}

// refreshSocketViewer keeps a restricted socket's permissions current. The ACL
// lists exactly what the viewer may read, so without this a new guest would
// stay hidden and a revoked one visible for the life of the socket. It runs
// apart from the write loop so a slow Proxmox can't hold up delivery. A brief
// failure keeps the last good answer, because an empty one would blank the
// inventory, but only for socketACLMaxAge.
func (s *Server) refreshSocketViewer(r *http.Request, viewer *atomic.Pointer[socketViewer], stop <-chan struct{}) {
	every, maxAge := s.socketACLEvery, s.socketACLMaxAge
	if every == 0 {
		every = socketACLEvery
	}
	if maxAge == 0 {
		maxAge = socketACLMaxAge
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	lastGood := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		acl, err := s.viewerACL(r)
		switch {
		case err == nil:
			lastGood = time.Now()
		case time.Since(lastGood) < maxAge:
			continue
		default:
			acl = pve.ACL{}
		}
		next := *viewer.Load()
		next.acl = acl
		viewer.Store(&next)
	}
}

// closeWithSession closes conn once the session behind r ends, and returns a
// func that stops watching. A socket is only authenticated when it opens, so
// without this it would outlive sign-out. The hub carries a sign-out at once;
// the periodic check catches expiry and a revocation that was missed.
func (s *Server) closeWithSession(r *http.Request, conn *websocket.Conn) func() {
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	revoked, unwatch := s.hub.WatchSession(sess.ID)
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(sessionCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-revoked:
				_ = conn.Close()
				return
			case <-ticker.C:
				if _, err := s.store.SessionByTokenHash(r.Context(), sess.TokenHash); errors.Is(err, store.ErrNotFound) {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() {
		close(stop)
		unwatch()
	}
}

func detailVisible(detail string, v socketViewer) bool {
	switch {
	case strings.HasPrefix(detail, "guest:"):
		return pve.CanSeeGuestTopic(detail, v.acl)
	case strings.HasPrefix(detail, "task:"):
		return pve.CanSeeTaskTopic(detail, v.acl, v.userid)
	case strings.HasPrefix(detail, "node:"):
		return pve.CanSeeNodeTopic(detail, v.acl)
	case strings.HasPrefix(detail, "rrd:"):
		return pve.CanSeeRRDTopic(detail, v.acl)
	}
	return false
}

// topicAllowed decides a subscription before the poller spends the
// connection's token on it. Anyone restricted stays on their own connection,
// within what their ACL shows.
func topicAllowed(topic string, v socketViewer) bool {
	if topic == "resources" || topic == "tasks" {
		return true
	}
	id, detail, ok := hub.SplitTopic(topic)
	if !ok || (v.acl != nil && id != v.connection) {
		return false
	}
	return detailVisible(detail, v)
}

// filterHubMessage narrows a hub event for a restricted socket. Cached data
// comes from the connection's API token and covers every cluster, so anything
// that can't be shown to belong to this viewer is dropped.
func filterHubMessage(raw []byte, v socketViewer) ([]byte, bool) {
	var ev struct {
		Type  string          `json:"type"`
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &ev) != nil {
		return nil, false
	}
	var data any
	switch ev.Topic {
	case "resources":
		var payload struct {
			Connection string              `json:"connection"`
			Status     []pve.ClusterStatus `json:"status"`
			Resources  []pve.Resource      `json:"resources"`
			TS         int64               `json:"ts,omitempty"`
		}
		if json.Unmarshal(ev.Data, &payload) != nil || payload.Connection != v.connection {
			return nil, false
		}
		payload.Status = pve.FilterStatus(payload.Status, v.acl)
		payload.Resources = pve.FilterResources(payload.Resources, v.acl)
		data = payload
	case "tasks":
		var payload struct {
			Connection string     `json:"connection"`
			Tasks      []pve.Task `json:"tasks"`
		}
		if json.Unmarshal(ev.Data, &payload) != nil || payload.Connection != v.connection {
			return nil, false
		}
		payload.Tasks = pve.FilterTasks(payload.Tasks, v.acl, v.userid)
		data = payload
	default:
		id, detail, ok := hub.SplitTopic(ev.Topic)
		if !ok || id != v.connection || !detailVisible(detail, v) {
			return nil, false
		}
		return raw, true
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	ev.Data = b
	out, err := json.Marshal(ev)
	if err != nil {
		return nil, false
	}
	return out, true
}
