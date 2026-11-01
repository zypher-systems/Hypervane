package hub

import (
	"context"
	"testing"
	"time"
)

func TestScopedTopicRoundTrip(t *testing.T) {
	topic := ScopedTopic("3f1c-a", "task:UPID:pve1:1:1:1:qmstart:100:root@pam:")
	id, detail, ok := SplitTopic(topic)
	if !ok || id != "3f1c-a" || detail != "task:UPID:pve1:1:1:1:qmstart:100:root@pam:" {
		t.Fatalf("%q %q %v", id, detail, ok)
	}
	for _, bad := range []string{"node:pve1", "resources", "conn:", "conn:id", "conn::node:pve1", "conn:id:"} {
		if _, _, ok := SplitTopic(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}

func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	case <-time.After(20 * time.Millisecond):
		return false
	}
}

func TestRevokeSessionClosesOnlyThatSession(t *testing.T) {
	h := New(nil)
	first, stopFirst := h.WatchSession("a")
	defer stopFirst()
	second, stopSecond := h.WatchSession("a")
	defer stopSecond()
	other, stopOther := h.WatchSession("b")
	defer stopOther()

	h.RevokeSession(context.Background(), "a")
	if !closed(first) || !closed(second) {
		t.Fatal("watchers of the revoked session stayed open")
	}
	if closed(other) {
		t.Fatal("another session was revoked")
	}
	// Stopping after a revoke must not panic, and revoking twice is harmless.
	stopFirst()
	h.RevokeSession(context.Background(), "a")
}

func TestWatchSessionStopForgetsTheWatcher(t *testing.T) {
	h := New(nil)
	ch, stop := h.WatchSession("a")
	stop()
	h.RevokeSession(context.Background(), "a")
	if closed(ch) {
		t.Fatal("stopped watcher was still tracked")
	}
	if len(h.sessions) != 0 {
		t.Fatalf("%d sessions left", len(h.sessions))
	}
}

func TestRevokeIsNotDeliveredToSubscribers(t *testing.T) {
	h := New(nil)
	c := h.Register()
	defer c.Close()
	c.Subscribe([]string{"a"})
	h.RevokeSession(context.Background(), "a")
	select {
	case msg := <-c.Send():
		t.Fatalf("revoke reached a subscriber: %s", msg)
	case <-time.After(20 * time.Millisecond):
	}
}
