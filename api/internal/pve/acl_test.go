package pve

import "testing"

func TestNormalizeAndHasPriv(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/":           map[string]any{"Sys.Audit": 1.0},
		"/vms":        map[string]any{"VM.Audit": 1.0},
		"/vms/100":    map[string]any{"VM.PowerMgmt": 1.0, "VM.Config.Disk": 1.0},
		"/nodes/pve1": map[string]any{"Sys.Console": true},
	})
	if !acl.Has("/vms/100", "VM.PowerMgmt") {
		t.Fatal("exact priv")
	}
	if !acl.Has("/vms/100", "VM.Config") {
		t.Fatal("prefix priv VM.Config")
	}
	if !acl.Has("/vms/100", "VM.Audit") {
		t.Fatal("inherited from /vms")
	}
	if !acl.Has("/vms/200", "Sys.Audit") {
		t.Fatal("inherited from /")
	}
	if acl.Has("/vms/200", "VM.PowerMgmt") {
		t.Fatal("should not have power on 200")
	}
	if !acl.CanSee("/nodes/pve1") || !acl.CanSee("/vms/100") {
		t.Fatal("can see")
	}
}

func TestFilterResourcesAndTasks(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/vms/100":       map[string]any{"VM.Audit": 1.0},
		"/nodes/pve1":    map[string]any{"Sys.Audit": 1.0},
		"/storage/local": map[string]any{"Datastore.Audit": 1.0},
	})
	resources := []Resource{
		{ID: "cluster/cluster", Type: "cluster"},
		{ID: "qemu/100", Type: "qemu", VMID: 100, Node: "pve1"},
		{ID: "qemu/200", Type: "qemu", VMID: 200, Node: "pve1"},
		{ID: "node/pve1", Type: "node", Node: "pve1"},
		{ID: "storage/pve1/local", Type: "storage", Storage: "local", Node: "pve1"},
		{ID: "storage/pve1/other", Type: "storage", Storage: "other", Node: "pve1"},
	}
	got := FilterResources(resources, acl)
	if len(got) != 4 {
		t.Fatalf("filtered resources %d %+v", len(got), got)
	}
	tasks := []Task{
		{UPID: "UPID:pve1:1:1:1:qmstart:100:user@pam:", Node: "pve1", ID: "100"},
		{UPID: "UPID:pve1:1:1:1:qmstart:200:user@pam:", Node: "pve2", ID: "200"},
	}
	kept := FilterTasks(tasks, acl)
	if len(kept) != 1 || kept[0].ID != "100" {
		t.Fatalf("tasks %+v", kept)
	}
	if !CanSeeGuestTopic("guest:pve1:qemu:100", acl) {
		t.Fatal("guest 100")
	}
	if CanSeeGuestTopic("guest:pve1:qemu:200", acl) {
		t.Fatal("guest 200 should be hidden")
	}
	if !CanSeeTaskTopic("task:UPID:pve1:1:1:1:qmstart:100:user@pam:", acl) {
		t.Fatal("task 100")
	}
	if CanSeeTaskTopic("task:UPID:pve2:1:1:1:qmstart:200:user@pam:", acl) {
		t.Fatal("task 200 should be hidden")
	}
	if !CanSeeNodeTopic("node:pve1", acl) {
		t.Fatal("node pve1")
	}
	if CanSeeNodeTopic("node:pve2", acl) {
		t.Fatal("node pve2 should be hidden")
	}
	if !CanSeeRRDTopic("rrd:qemu:pve1:100:hour", acl) {
		t.Fatal("rrd guest 100")
	}
	if CanSeeRRDTopic("rrd:qemu:pve1:200:hour", acl) {
		t.Fatal("rrd guest 200 should be hidden")
	}
	if !CanSeeRRDTopic("rrd:node:pve1:hour", acl) {
		t.Fatal("rrd node pve1")
	}
	if CanSeeRRDTopic("rrd:storage:pve1:other:hour", acl) {
		t.Fatal("rrd other storage should be hidden")
	}
}

func TestFilterStatus(t *testing.T) {
	acl := NormalizePermissions(map[string]any{"/nodes/pve1": map[string]any{"Sys.Audit": 1.0}})
	in := []ClusterStatus{
		{Type: "cluster", Name: "lab", Quorate: 1},
		{Type: "node", Name: "pve1", IP: "10.0.0.1"},
		{Type: "node", Name: "pve2", IP: "10.0.0.2"},
	}
	out := FilterStatus(in, acl)
	if len(out) != 2 || out[0].Type != "cluster" || out[1].Name != "pve1" {
		t.Fatalf("%+v", out)
	}
	if got := FilterStatus(in, nil); len(got) != 3 {
		t.Fatalf("nil ACL filtered: %+v", got)
	}
	if got := FilterStatus(in, ACL{}); len(got) != 1 {
		t.Fatalf("empty ACL: %+v", got)
	}
}

func TestCanSeeResourceID(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/vms/100":     map[string]any{"VM.Audit": 1.0},
		"/nodes/pve1":  map[string]any{"Sys.Audit": 1.0},
		"/storage/nfs": map[string]any{"Datastore.Audit": 1.0},
		"/pool/lab":    map[string]any{"Pool.Audit": 1.0},
	})
	for id, want := range map[string]bool{
		"qemu/100":           true,
		"lxc/100":            true,
		"qemu/200":           false,
		"node/pve1":          true,
		"node/pve2":          false,
		"storage/pve1/nfs":   true,
		"storage/pve1/local": false,
		"pool/lab":           true,
		"pool/other":         false,
		"qemu/100/extra":     false,
		"qemu/abc":           false,
		"":                   false,
		"anything":           false,
	} {
		if got := CanSeeResourceID(id, acl); got != want {
			t.Errorf("%q: got %v", id, got)
		}
	}
	root := NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0}})
	if !CanSeeResourceID("anything", root) || !CanSeeResourceID("qemu/999", root) {
		t.Fatal("root access should cover everything")
	}
	if !CanSeeResourceID("qemu/200", nil) {
		t.Fatal("nil ACL is unrestricted")
	}
	if CanSeeResourceID("qemu/100", ACL{}) {
		t.Fatal("empty ACL sees nothing")
	}
}
