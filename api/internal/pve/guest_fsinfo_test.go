package pve

import "testing"

func TestGuestFSUsed(t *testing.T) {
	raw := map[string]any{
		"result": []any{
			map[string]any{"type": "ext4", "mountpoint": "/", "used-bytes": 4_500_000_000.0, "total-bytes": 20_000_000_000.0},
			map[string]any{"type": "vfat", "mountpoint": "/boot/efi", "used-bytes": 50_000_000.0},
			map[string]any{"type": "tmpfs", "used-bytes": 9_000_000_000.0},
			map[string]any{"type": "nfs", "used-bytes": 1_000_000_000.0},
		},
	}
	got := GuestFSUsed(raw)
	want := int64(4_550_000_000)
	if got != want {
		t.Fatalf("GuestFSUsed=%d want %d", got, want)
	}
}

func TestGuestFSUsedNested(t *testing.T) {
	raw := map[string]any{
		"data": map[string]any{
			"result": []any{
				map[string]any{"type": "NTFS", "used": 1234.0},
			},
		},
	}
	if got := GuestFSUsed(raw); got != 1234 {
		t.Fatalf("nested used=%d", got)
	}
}

func TestGuestFSUsedEmpty(t *testing.T) {
	if GuestFSUsed(nil) != 0 || GuestFSUsed(map[string]any{"result": map[string]any{"error": "gone"}}) != 0 {
		t.Fatal("empty should be 0")
	}
}
