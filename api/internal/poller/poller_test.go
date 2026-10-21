package poller

import "testing"

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
