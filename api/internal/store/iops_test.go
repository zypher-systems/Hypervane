package store

import "testing"

func TestIOPSWindowStep(t *testing.T) {
	if IOPSWindow("hour") != IOPSWindow("") {
		t.Fatal("hour default")
	}
	if IOPSStep("year") <= IOPSStep("hour") {
		t.Fatal("year should be coarser than hour")
	}
}
