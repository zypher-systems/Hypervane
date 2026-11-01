package httpapi

import (
	"strconv"
	"sync/atomic"
	"testing"

	"hypervane/internal/hub"
	"hypervane/internal/poller"
	"hypervane/internal/pve"
)

func topicsFor(t *testing.T, acl pve.ACL) (*socketTopics, *atomic.Pointer[socketViewer]) {
	t.Helper()
	h := hub.New(nil)
	client := h.Register()
	t.Cleanup(client.Close)
	var viewer atomic.Pointer[socketViewer]
	v := testViewer(acl)
	viewer.Store(&v)
	return newSocketTopics(client, poller.New(nil, h, nil, nil)), &viewer
}

func hasTopic(set map[string]struct{}, topic string) bool {
	_, ok := set[topic]
	return ok
}

func TestSocketTopicsKeepUnreadableTopicsFromThePoller(t *testing.T) {
	topics, viewer := topicsFor(t, vm100ACL())
	visible := hub.ScopedTopic(labA, "guest:pve1:qemu:100")
	hidden := hub.ScopedTopic(labA, "guest:pve1:qemu:200")
	topics.subscribe([]string{visible, hidden}, viewer)
	if !hasTopic(topics.held, visible) || hasTopic(topics.held, hidden) {
		t.Fatalf("held %v", topics.held)
	}
	if !hasTopic(topics.pending, hidden) {
		t.Fatalf("pending %v", topics.pending)
	}

	both := pve.NormalizePermissions(map[string]any{"/vms": map[string]any{"VM.Audit": 1.0}})
	topics.reconcile(testViewer(both))
	if !hasTopic(topics.held, hidden) || len(topics.pending) != 0 {
		t.Fatalf("granted topic not held: %v %v", topics.held, topics.pending)
	}

	topics.reconcile(testViewer(pve.ACL{}))
	if len(topics.held) != 0 || len(topics.pending) != 2 {
		t.Fatalf("a revoked topic must leave the poller and wait: %v %v", topics.held, topics.pending)
	}
	topics.reconcile(testViewer(both))
	if len(topics.held) != 2 || len(topics.pending) != 0 {
		t.Fatalf("restored topics: %v %v", topics.held, topics.pending)
	}
}

func TestSocketTopicsRefuseWhatCouldNeverBeAllowed(t *testing.T) {
	topics, viewer := topicsFor(t, vm100ACL())
	topics.subscribe([]string{
		hub.ScopedTopic(labB, "guest:pve1:qemu:100"),
		hub.ScopedTopic(labA, "guest:pve1:qemu/200/status/current#:100"),
		hub.ScopedTopic(labA, "bogus"),
		"guest:pve1:qemu:100",
	}, viewer)
	if len(topics.held) != 0 || len(topics.pending) != 0 {
		t.Fatalf("held %v pending %v", topics.held, topics.pending)
	}
}

func TestSocketTopicsBoundWhatWaits(t *testing.T) {
	topics, viewer := topicsFor(t, pve.ACL{})
	var list []string
	for i := 1; i <= socketPendingMax*3; i++ {
		list = append(list, hub.ScopedTopic(labA, "guest:pve1:qemu:"+strconv.Itoa(i)))
	}
	topics.subscribe(list, viewer)
	if len(topics.pending) != socketPendingMax {
		t.Fatalf("%d topics waiting", len(topics.pending))
	}
}

func TestSocketTopicsUnsubscribeClearsWaiting(t *testing.T) {
	topics, viewer := topicsFor(t, vm100ACL())
	hidden := hub.ScopedTopic(labA, "guest:pve1:qemu:200")
	topics.subscribe([]string{hidden}, viewer)
	topics.unsubscribe([]string{hidden})
	topics.reconcile(testViewer(pve.NormalizePermissions(map[string]any{"/vms": map[string]any{"VM.Audit": 1.0}})))
	if len(topics.held) != 0 {
		t.Fatalf("a topic the browser left came back: %v", topics.held)
	}
}

func TestSocketTopicsIgnoreARefreshAfterRelease(t *testing.T) {
	topics, viewer := topicsFor(t, vm100ACL())
	topics.subscribe([]string{hub.ScopedTopic(labA, "guest:pve1:qemu:100")}, viewer)
	topics.release()
	topics.reconcile(testViewer(vm100ACL()))
	if len(topics.held) != 0 {
		t.Fatalf("a closed socket picked topics back up: %v", topics.held)
	}
}
