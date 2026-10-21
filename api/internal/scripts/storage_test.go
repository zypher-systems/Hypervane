package scripts

import (
	"testing"

	"hypervane/internal/pve"
)

func TestPickSkipsBackupAndPrefersLocalImport(t *testing.T) {
	list := []pve.Storage{
		{Storage: "zs-pbs-01", Type: "pbs", Content: "backup"},
		{Storage: "local-zfs-array", Type: "zfspool", Content: "images,rootdir"},
		{Storage: "local", Type: "dir", Content: "iso,vztmpl,backup,snippets,import"},
	}
	got, ok := Pick(list, "zs-hv1", "images")
	if !ok || got.Storage != "local-zfs-array" {
		t.Fatalf("images: %+v", got)
	}
	got, ok = Pick(list, "zs-hv1", "import")
	if !ok || got.Storage != "local" {
		t.Fatalf("import: %+v", got)
	}
	if _, ok := Pick(list, "zs-hv1", "images"); ok {
		if HasContent(list[0], "images") {
			t.Fatal("pbs should not hold images")
		}
	}
}

func TestResolveStoresRejectsPBS(t *testing.T) {
	list := []pve.Storage{
		{Storage: "zs-pbs-01", Type: "pbs", Content: "backup"},
		{Storage: "local-zfs-array", Type: "zfspool", Content: "images,rootdir"},
		{Storage: "local", Type: "dir", Content: "iso,vztmpl,import"},
	}
	_, _, err := ResolveStores(list, "zs-hv1", "zs-pbs-01")
	if err == nil {
		t.Fatal("expected pbs reject")
	}
	disk, dl, err := ResolveStores(list, "zs-hv1", "local-zfs-array")
	if err != nil || disk != "local-zfs-array" || dl != "local" {
		t.Fatalf("%s %s %v", disk, dl, err)
	}
}

func TestResolveBackupStoreAllowsPBS(t *testing.T) {
	list := []pve.Storage{
		{Storage: "zs-pbs-01", Type: "pbs", Content: "backup"},
		{Storage: "local-zfs-array", Type: "zfspool", Content: "images,rootdir"},
	}
	got, err := ResolveBackupStore(list, "zs-hv1", "zs-pbs-01")
	if err != nil || got != "zs-pbs-01" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := ResolveBackupStore(list, "zs-hv1", "local-zfs-array"); err == nil {
		t.Fatal("images-only store should not accept backups")
	}
}
