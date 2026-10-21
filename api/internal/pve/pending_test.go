package pve

import "testing"

func TestHasPendingChanges(t *testing.T) {
	t.Parallel()
	if HasPendingChanges(nil) {
		t.Fatal("empty list should not be pending")
	}
	if HasPendingChanges([]PendingChange{{Key: "memory", Value: 8192}}) {
		t.Fatal("current-only key should not be pending")
	}
	if !HasPendingChanges([]PendingChange{{Key: "memory", Value: 8192, Pending: 16384}}) {
		t.Fatal("pending value should count")
	}
	if !HasPendingChanges([]PendingChange{{Key: "net0", Delete: 1}}) {
		t.Fatal("delete should count")
	}
}
