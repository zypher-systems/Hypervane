package poller

import (
	"testing"

	"hypervane/internal/hub"
	"hypervane/internal/pve"
)

func TestWatchUnwatchReleasesTopic(t *testing.T) {
	p := New(nil, nil, nil, nil)
	p.Watch("task:UPID:pve1:1:1:1:qmstart:100:root@pam:")
	p.Watch("task:UPID:pve1:1:1:1:qmstart:100:root@pam:")
	if got := len(p.watching("task:")); got != 1 {
		t.Fatalf("watching after two watches: %d", got)
	}
	p.Unwatch("task:UPID:pve1:1:1:1:qmstart:100:root@pam:")
	if got := len(p.watching("task:")); got != 1 {
		t.Fatalf("watching after one unwatch: %d", got)
	}
	p.Unwatch("task:UPID:pve1:1:1:1:qmstart:100:root@pam:")
	if got := len(p.watching("task:")); got != 0 {
		t.Fatalf("watching after matching unwatches: %d", got)
	}
}

func TestWatchedDetailsStayOnTheirConnection(t *testing.T) {
	a, err := pve.New(pve.Options{BaseURL: "http://127.0.0.1:1", Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := pve.New(pve.Options{BaseURL: "http://127.0.0.1:2", Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	conns := []ConnectionClient{{ID: "a", Client: a}, {ID: "b", Client: b}}
	p := New(nil, nil, nil, nil)
	topicA := hub.ScopedTopic("a", "node:pve1")
	topicB := hub.ScopedTopic("b", "node:pve1")
	p.Watch(topicA)
	p.Watch(topicB)
	p.Watch(hub.ScopedTopic("deleted", "node:pve1"))
	p.Watch(hub.ScopedTopic("a", "guest:pve1:qemu:100"))
	p.Watch("node:pve1")
	p.Watch(hub.ScopedTopic("a", "node:pve1/../pve2"))

	got := map[string]*pve.Client{}
	for _, w := range p.watchedDetails("node:", conns) {
		if w.detail != "pve1" {
			t.Fatalf("detail %q", w.detail)
		}
		got[w.topic] = w.client
	}
	if len(got) != 2 || got[topicA] != a || got[topicB] != b {
		t.Fatalf("got %v", got)
	}
}
