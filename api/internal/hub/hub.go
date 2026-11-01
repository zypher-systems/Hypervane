package hub

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

const RedisChannel = "hypervane:events"

const (
	eventRevoke = "revoke"

	// ScopePrefix starts every topic made by ScopedTopic.
	ScopePrefix = "conn:"
)

// ScopedTopic names a detail topic on one connection. Node names, VMIDs and
// UPIDs are only unique within a cluster, so they are never published bare.
func ScopedTopic(connection, detail string) string {
	return ScopePrefix + connection + ":" + detail
}

// SplitTopic undoes ScopedTopic.
func SplitTopic(topic string) (connection, detail string, ok bool) {
	rest, found := strings.CutPrefix(topic, ScopePrefix)
	if !found {
		return "", "", false
	}
	connection, detail, found = strings.Cut(rest, ":")
	if !found || connection == "" || detail == "" {
		return "", "", false
	}
	return connection, detail, true
}

type Event struct {
	Type  string `json:"type"`
	Topic string `json:"topic"`
	Data  any    `json:"data,omitempty"`
}

type client struct {
	send   chan []byte
	topics map[string]struct{}
	mu     sync.Mutex
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	sessions map[string]map[chan struct{}]struct{}
	rdb      *redis.Client
}

func New(rdb *redis.Client) *Hub {
	return &Hub{
		clients:  make(map[*client]struct{}),
		sessions: make(map[string]map[chan struct{}]struct{}),
		rdb:      rdb,
	}
}

func (h *Hub) Run(ctx context.Context) {
	if h.rdb == nil {
		return
	}
	sub := h.rdb.Subscribe(ctx, RedisChannel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			h.dispatch(ev)
		}
	}
}

func (h *Hub) Publish(ctx context.Context, topic string, data any) {
	h.emit(ctx, Event{Type: "event", Topic: topic, Data: data})
}

// RevokeSession tells every socket held open by a session to close, on every
// API instance.
func (h *Hub) RevokeSession(ctx context.Context, sessionID string) {
	h.emit(ctx, Event{Type: eventRevoke, Topic: sessionID})
}

// WatchSession returns a channel that closes when the session is revoked. The
// returned func stops watching.
func (h *Hub) WatchSession(sessionID string) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[sessionID] == nil {
		h.sessions[sessionID] = make(map[chan struct{}]struct{})
	}
	h.sessions[sessionID][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.sessions[sessionID], ch)
		if len(h.sessions[sessionID]) == 0 {
			delete(h.sessions, sessionID)
		}
	}
}

func (h *Hub) revoke(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.sessions[sessionID] {
		close(ch)
	}
	delete(h.sessions, sessionID)
}

func (h *Hub) emit(ctx context.Context, ev Event) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if h.rdb != nil {
		if err := h.rdb.Publish(ctx, RedisChannel, raw).Err(); err != nil {
			log.Printf("hub publish: %v", err)
			h.dispatch(ev)
		}
		return
	}
	h.dispatch(ev)
}

func (h *Hub) dispatch(ev Event) {
	if ev.Type == eventRevoke {
		h.revoke(ev.Topic)
		return
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.mu.Lock()
		_, ok := c.topics[ev.Topic]
		c.mu.Unlock()
		if !ok {
			continue
		}
		select {
		case c.send <- raw:
		default:
		}
	}
}

func (h *Hub) Register() *ClientConn {
	c := &client{
		send:   make(chan []byte, 32),
		topics: make(map[string]struct{}),
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return &ClientConn{hub: h, client: c}
}

type ClientConn struct {
	hub    *Hub
	client *client
}

func (c *ClientConn) Send() <-chan []byte {
	return c.client.send
}

func (c *ClientConn) Subscribe(topics []string) {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()
	for _, t := range topics {
		c.client.topics[t] = struct{}{}
	}
}

func (c *ClientConn) Unsubscribe(topics []string) {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()
	for _, t := range topics {
		delete(c.client.topics, t)
	}
}

func (c *ClientConn) Close() {
	c.hub.mu.Lock()
	delete(c.hub.clients, c.client)
	c.hub.mu.Unlock()
	close(c.client.send)
}
