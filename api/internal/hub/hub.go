package hub

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/redis/go-redis/v9"
)

const RedisChannel = "hypervane:events"

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
	mu      sync.RWMutex
	clients map[*client]struct{}
	rdb     *redis.Client
}

func New(rdb *redis.Client) *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		rdb:     rdb,
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
	ev := Event{Type: "event", Topic: topic, Data: data}
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
