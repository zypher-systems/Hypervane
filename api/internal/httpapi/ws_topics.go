package httpapi

import (
	"sync"
	"sync/atomic"

	"hypervane/internal/hub"
	"hypervane/internal/poller"
	"hypervane/internal/pve"
)

// socketPendingMax bounds what one socket can leave waiting.
const socketPendingMax = 256

// socketTopics owns what one socket follows. A topic is held while the viewer
// may read it, which is the only time the poller spends the connection's token
// on it. A topic the viewer can't read yet waits, because the browser believes
// it is subscribed and won't ask again.
type socketTopics struct {
	mu      sync.Mutex
	held    map[string]struct{}
	pending map[string]struct{}
	closed  bool
	client  *hub.ClientConn
	poller  *poller.Poller
}

func newSocketTopics(client *hub.ClientConn, p *poller.Poller) *socketTopics {
	return &socketTopics{
		held:    map[string]struct{}{},
		pending: map[string]struct{}{},
		client:  client,
		poller:  p,
	}
}

// subscribe reads the viewer under the lock, so a refresh can't slip between
// the check and the topic being recorded for the next reconcile.
func (t *socketTopics) subscribe(topics []string, viewer *atomic.Pointer[socketViewer]) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v := *viewer.Load()
	for _, topic := range topics {
		if _, dup := t.held[topic]; !dup {
			t.place(topic, v)
		}
	}
}

func (t *socketTopics) unsubscribe(topics []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, topic := range topics {
		delete(t.pending, topic)
		if _, ok := t.held[topic]; ok {
			t.drop(topic)
		}
	}
}

// reconcile applies a refreshed viewer in both directions.
func (t *socketTopics) reconcile(v socketViewer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	for topic := range t.held {
		if !topicAllowed(topic, v) {
			t.drop(topic)
			t.wait(topic, v)
		}
	}
	for topic := range t.pending {
		t.place(topic, v)
	}
}

func (t *socketTopics) release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	for topic := range t.held {
		t.poller.Unwatch(topic)
	}
	t.held = map[string]struct{}{}
	t.pending = map[string]struct{}{}
}

func (t *socketTopics) place(topic string, v socketViewer) {
	if !topicAllowed(topic, v) {
		t.wait(topic, v)
		return
	}
	delete(t.pending, topic)
	t.held[topic] = struct{}{}
	t.poller.Watch(topic)
	t.client.Subscribe([]string{topic})
}

func (t *socketTopics) drop(topic string) {
	delete(t.held, topic)
	t.poller.Unwatch(topic)
	t.client.Unsubscribe([]string{topic})
}

// wait keeps a topic that a later refresh could allow. Anything malformed or on
// another connection never can be.
func (t *socketTopics) wait(topic string, v socketViewer) {
	id, detail, ok := hub.SplitTopic(topic)
	if !ok || v.acl == nil || id != v.connection || !pve.ValidTopic(detail) {
		return
	}
	if _, dup := t.pending[topic]; dup || len(t.pending) < socketPendingMax {
		t.pending[topic] = struct{}{}
	}
}
