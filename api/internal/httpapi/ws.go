package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"hypervane/internal/pve"
)

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

	client := s.hub.Register()
	defer client.Close()

	held := map[string]int{}
	var heldMu sync.Mutex

	acl, filter := s.requestACL(r)

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
				client.Subscribe(cmd.Topics)
				heldMu.Lock()
				for _, t := range cmd.Topics {
					s.poller.Watch(t)
					held[t]++
				}
				heldMu.Unlock()
			case "unsubscribe":
				client.Unsubscribe(cmd.Topics)
				heldMu.Lock()
				for _, t := range cmd.Topics {
					s.poller.Unwatch(t)
					if held[t] > 0 {
						held[t]--
						if held[t] == 0 {
							delete(held, t)
						}
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
		for t, n := range held {
			for i := 0; i < n; i++ {
				s.poller.Unwatch(t)
			}
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
				filtered, keep := filterHubMessage(msg, acl)
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

func filterHubMessage(raw []byte, acl pve.ACL) ([]byte, bool) {
	var ev struct {
		Type  string          `json:"type"`
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &ev) != nil {
		return raw, true
	}
	switch {
	case ev.Topic == "resources":
		var payload struct {
			Resources []pve.Resource      `json:"resources"`
			Status    []pve.ClusterStatus `json:"status"`
		}
		if json.Unmarshal(ev.Data, &payload) != nil {
			return raw, true
		}
		payload.Resources = pve.FilterResources(payload.Resources, acl)
		b, err := json.Marshal(payload)
		if err != nil {
			return raw, true
		}
		ev.Data = b
	case ev.Topic == "tasks":
		var payload struct {
			Connection string     `json:"connection,omitempty"`
			Tasks      []pve.Task `json:"tasks"`
		}
		if json.Unmarshal(ev.Data, &payload) != nil {
			var tasks []pve.Task
			if json.Unmarshal(ev.Data, &tasks) != nil {
				return nil, false
			}
			payload.Tasks = tasks
		}
		payload.Tasks = pve.FilterTasks(payload.Tasks, acl)
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, false
		}
		ev.Data = b
	case strings.HasPrefix(ev.Topic, "guest:"):
		if !pve.CanSeeGuestTopic(ev.Topic, acl) {
			return nil, false
		}
		return raw, true
	case strings.HasPrefix(ev.Topic, "task:"):
		if !pve.CanSeeTaskTopic(ev.Topic, acl) {
			return nil, false
		}
		return raw, true
	case strings.HasPrefix(ev.Topic, "node:"):
		if !pve.CanSeeNodeTopic(ev.Topic, acl) {
			return nil, false
		}
		return raw, true
	case strings.HasPrefix(ev.Topic, "rrd:"):
		if !pve.CanSeeRRDTopic(ev.Topic, acl) {
			return nil, false
		}
		return raw, true
	default:
		return raw, true
	}
	out, err := json.Marshal(ev)
	if err != nil {
		return raw, true
	}
	return out, true
}
