package pve

import (
	"testing"
	"time"
)

func TestSumBlockOps(t *testing.T) {
	rd, wr, ok := SumBlockOps(nil)
	if ok || rd != 0 || wr != 0 {
		t.Fatalf("empty: %v %v %v", rd, wr, ok)
	}
	rd, wr, ok = SumBlockOps(map[string]BlockDeviceStats{
		"scsi0": {RdOperations: 10, WrOperations: 4},
		"ide2":  {RdOperations: 2, WrOperations: 0},
	})
	if !ok || rd != 12 || wr != 4 {
		t.Fatalf("sum: %v %v %v", rd, wr, ok)
	}
}

func TestIOPSRate(t *testing.T) {
	got, ok := IOPSRate(100, 250, 10*time.Second)
	if !ok || got != 15 {
		t.Fatalf("rate: %v %v", got, ok)
	}
	if _, ok := IOPSRate(100, 90, 10*time.Second); ok {
		t.Fatal("reboot should skip")
	}
	if _, ok := IOPSRate(100, 200, 2*time.Minute); ok {
		t.Fatal("gap should skip")
	}
}

func TestSumBlockTimeNS(t *testing.T) {
	ns, ok := SumBlockTimeNS(nil)
	if ok || ns != 0 {
		t.Fatalf("empty: %v %v", ns, ok)
	}
	ns, ok = SumBlockTimeNS(map[string]BlockDeviceStats{
		"scsi0": {RdTotalTimeNS: 4e6, WrTotalTimeNS: 1e6, FlushTotalTimeNS: 5e5},
		"ide2":  {RdTotalTimeNS: 5e5},
	})
	if !ok || ns != 6e6 {
		t.Fatalf("sum: %v %v", ns, ok)
	}
}

func TestIOWaitFrac(t *testing.T) {
	got, ok := IOWaitFrac(0, 2e9, 10*time.Second)
	if !ok || got != 0.2 {
		t.Fatalf("frac: %v %v", got, ok)
	}
	got, ok = IOWaitFrac(0, 20e9, 10*time.Second)
	if !ok || got != 1 {
		t.Fatalf("clamp: %v %v", got, ok)
	}
	if _, ok := IOWaitFrac(10, 5, 10*time.Second); ok {
		t.Fatal("reboot should skip")
	}
}
