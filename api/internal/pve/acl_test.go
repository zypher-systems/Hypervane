package pve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeAndHasPriv(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/":           map[string]any{"Sys.Audit": 1.0},
		"/vms":        map[string]any{"Sys.Audit": 1.0, "VM.Audit": 1.0},
		"/vms/100":    map[string]any{"Sys.Audit": 1.0, "VM.Audit": 1.0, "VM.PowerMgmt": 1.0, "VM.Config.Disk": 1.0},
		"/nodes/pve1": map[string]any{"Sys.Audit": 1.0, "Sys.Console": true},
	})
	if !acl.Has("/vms/100", "VM.PowerMgmt") {
		t.Fatal("exact priv")
	}
	if !acl.Has("/vms/100", "VM.Config") {
		t.Fatal("prefix priv VM.Config")
	}
	if !acl.Has("/vms/100", "VM.Audit") {
		t.Fatal("listed on the path itself")
	}
	if !acl.Has("/vms/200", "Sys.Audit") {
		t.Fatal("an unlisted guest takes what /vms passes down")
	}
	if acl.Has("/vms/200", "VM.PowerMgmt") {
		t.Fatal("should not have power on 200")
	}
	if !acl.CanRead("/nodes/pve1") || !acl.CanRead("/vms/100") {
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
	kept := FilterTasks(tasks, acl, "")
	if len(kept) != 1 || kept[0].ID != "100" {
		t.Fatalf("tasks %+v", kept)
	}
	if !CanSeeGuestTopic("guest:pve1:qemu:100", acl) {
		t.Fatal("guest 100")
	}
	if CanSeeGuestTopic("guest:pve1:qemu:200", acl) {
		t.Fatal("guest 200 should be hidden")
	}
	if !CanSeeTaskTopic("task:UPID:pve1:1:1:1:qmstart:100:user@pam:", acl, "") {
		t.Fatal("task 100")
	}
	if CanSeeTaskTopic("task:UPID:pve2:1:1:1:qmstart:200:user@pam:", acl, "") {
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
	root := NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0, "VM.Audit": 1.0}})
	if !CanSeeResourceID("qemu/999", root) {
		t.Fatal("the right privileges on / should carry down")
	}
	sysOnly := NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0}})
	if CanSeeResourceID("qemu/999", sysOnly) {
		t.Fatal("Sys.Audit does not read guests")
	}
	if !CanSeeResourceID("qemu/200", nil) {
		t.Fatal("nil ACL is unrestricted")
	}
	if CanSeeResourceID("qemu/100", ACL{}) {
		t.Fatal("empty ACL sees nothing")
	}
}

// A privilege only counts for the thing it reads. Proxmox checks these exact
// privileges in /cluster/resources and in its task endpoints.
func TestUnrelatedPrivilegesReadNothing(t *testing.T) {
	sdnOnly := NormalizePermissions(map[string]any{"/": map[string]any{"SDN.Use": 1.0}})
	for _, id := range []string{"qemu/100", "lxc/100", "node/pve1", "storage/pve1/local", "pool/lab", "anything"} {
		if CanSeeResourceID(id, sdnOnly) {
			t.Errorf("SDN.Use on / read %s", id)
		}
	}
	power := NormalizePermissions(map[string]any{"/nodes/pve1": map[string]any{"Sys.PowerMgmt": 1.0}})
	status := []ClusterStatus{{Type: "node", Name: "pve1", IP: "10.0.0.1"}}
	if got := FilterStatus(status, power); len(got) != 0 {
		t.Errorf("Sys.PowerMgmt showed a node address: %+v", got)
	}
	if got := FilterResources([]Resource{{ID: "node/pve1", Type: "node", Node: "pve1"}}, power); len(got) != 0 {
		t.Errorf("Sys.PowerMgmt showed a node: %+v", got)
	}
	powerVM := NormalizePermissions(map[string]any{"/vms/100": map[string]any{"VM.PowerMgmt": 1.0}})
	if got := FilterResources([]Resource{{ID: "qemu/100", Type: "qemu", VMID: 100}}, powerVM); len(got) != 0 {
		t.Errorf("VM.PowerMgmt showed a guest: %+v", got)
	}
	if CanSeeGuestTopic("guest:pve1:qemu:100", powerVM) {
		t.Error("VM.PowerMgmt followed a guest")
	}
	if CanSeeNodeTopic("node:pve1", power) {
		t.Error("Sys.PowerMgmt followed a node")
	}
	if CanSeeRRDTopic("rrd:storage:pve1:local:hour", sdnOnly) {
		t.Error("SDN.Use followed storage metrics")
	}
}

func TestCanSeeTaskFollowsProxmox(t *testing.T) {
	task := Task{UPID: "UPID:pve1:1:1:1:qmstart:100:bob@pve:", Node: "pve1", ID: "100", User: "bob@pve"}
	cases := []struct {
		name   string
		acl    ACL
		userid string
		want   bool
	}{
		{"owner with no rights", ACL{}, "bob@pve", true},
		{"a stranger", ACL{}, "alice@pve", false},
		{"VM.Audit on the guest", NormalizePermissions(map[string]any{"/vms/100": map[string]any{"VM.Audit": 1.0}}), "alice@pve", false},
		{"Sys.Audit on the node", NormalizePermissions(map[string]any{"/nodes/pve1": map[string]any{"Sys.Audit": 1.0}}), "alice@pve", true},
		{"Sys.Audit on root", NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0}}), "alice@pve", true},
		{"Sys.PowerMgmt on the node", NormalizePermissions(map[string]any{"/nodes/pve1": map[string]any{"Sys.PowerMgmt": 1.0}}), "alice@pve", false},
		{"no viewer", ACL{}, "", false},
	}
	for _, c := range cases {
		if got := CanSeeTask(task, c.acl, c.userid); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	token := Task{UPID: "UPID:pve1:1:1:1:qmstart:100:alice@pve!ci:"}
	if !CanSeeTask(token, ACL{}, "alice@pve") {
		t.Error("a user owns their token's tasks")
	}
	if CanSeeTask(token, ACL{}, "alice@pv") {
		t.Error("a name prefix is not ownership")
	}
}

// A topic is read by the ACL and fetched by the poller. Anything that could
// mean one guest to the first and another to the second has to be refused.
func TestCraftedTopicsAreRefused(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/vms/100":    map[string]any{"VM.Audit": 1.0},
		"/nodes/pve1": map[string]any{"Sys.Audit": 1.0},
	})
	for _, topic := range []string{
		"guest:pve1:qemu/200/status/current#:100",
		"guest:pve1:qemu/200/status/current?x=:100",
		"guest:pve1:qemu%2f200:100",
		"guest:pve1/../pve2:qemu:100",
		"guest:pve1:qemu:100:extra",
		"guest:pve1:qemu:0100",
		"guest:pve1:qemu:100x",
		"guest:pve1:bogus:100",
		"guest::qemu:100",
	} {
		if CanSeeGuestTopic(topic, acl) {
			t.Errorf("%q passed as VM 100", topic)
		}
		if CanSeeGuestTopic(topic, nil) {
			t.Errorf("%q reached an unrestricted viewer", topic)
		}
	}
	for _, topic := range []string{"node:pve1/../pve2", "node:pve1#", "node:pve1:extra"} {
		if CanSeeNodeTopic(topic, acl) {
			t.Errorf("%q passed as pve1", topic)
		}
	}
	if !CanSeeGuestTopic("guest:pve1:qemu:100", acl) || !CanSeeGuestTopic("guest:pve1:lxc:100", acl) {
		t.Fatal("real guest topics must still pass")
	}
}

// Proxmox's value is the propagate flag, so 0 still means the right is held.
func TestNonPropagatingGrants(t *testing.T) {
	acl := NormalizePermissions(map[string]any{
		"/vms/200": map[string]any{"VM.Audit": 0.0},
		"/":        map[string]any{"Sys.Audit": 0.0},
	})
	if !acl.CanRead("/vms/200") {
		t.Fatal("a grant that doesn't propagate is still held on its own path")
	}
	if !acl.Has("/", "Sys.Audit") {
		t.Fatal("the same goes for /")
	}
	if acl.Has("/nodes/pve1", "Sys.Audit") {
		t.Fatal("it must not reach anything below")
	}
}

func TestViewerACLUsesProxmoxsOwnAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any = []any{}
		switch {
		case r.URL.Path == "/api2/json/access/permissions" && r.URL.Query().Get("path") == "/nodes/pve1":
			data = map[string]any{"/nodes/pve1": map[string]any{"Sys.Audit": 1}}
		case r.URL.Path == "/api2/json/access/permissions" && r.URL.Query().Get("path") == "/nodes/pve2":
			data = map[string]any{"/nodes/pve2": map[string]any{}}
		case r.URL.Path == "/api2/json/access/permissions":
			data = map[string]any{"/": map[string]any{"Sys.Audit": 1, "VM.Audit": 1, "Datastore.Audit": 1}, "/vms": map[string]any{"VM.Audit": 1}}
		case r.URL.Path == "/api2/json/cluster/resources":
			data = []map[string]any{
				{"id": "node/pve1", "type": "node", "node": "pve1"},
				{"id": "node/pve2", "type": "node", "node": "pve2"},
				{"id": "qemu/100", "type": "qemu", "vmid": 100, "node": "pve1"},
				{"id": "storage/pve1/local", "type": "storage", "storage": "local", "node": "pve1"},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Ticket: "PVE:alice@pve:x"})
	if err != nil {
		t.Fatal(err)
	}
	acl, err := c.ViewerACL(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"/vms/100":       true,
		"/vms/200":       false, // NoAccess: not in the viewer's own list
		"/storage/local": true,
		"/storage/nfs":   false,
		"/nodes/pve1":    true,
		"/nodes/pve2":    false, // NoAccess: the exact lookup is empty, despite /
	} {
		if got := acl.CanRead(path); got != want {
			t.Errorf("%s: got %v", path, got)
		}
	}
	if !acl.Has("/", "Sys.Audit") {
		t.Fatal("rights that resources doesn't answer for must stay")
	}
}

func TestSDNFollowsItsZone(t *testing.T) {
	acl := NormalizePermissions(map[string]any{"/sdn/zones/lab": map[string]any{"SDN.Audit": 1.0}})
	got := FilterResources([]Resource{
		{ID: "sdn/pve1/lab", Type: "sdn"},
		{ID: "sdn/pve1/other", Type: "sdn"},
	}, acl)
	if len(got) != 1 || got[0].ID != "sdn/pve1/lab" {
		t.Fatalf("%+v", got)
	}
}

// Ids exactly as Proxmox's /cluster/resources sends them.
func TestResourceIDsFollowProxmox(t *testing.T) {
	pool := NormalizePermissions(map[string]any{"/pool/lab": map[string]any{"Pool.Audit": 1.0}})
	if !CanSeeResourceID("/pool/lab", pool) {
		t.Error("Pool.Audit must read its pool by Proxmox's id")
	}
	sysOnly := NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0}})
	for _, id := range []string{"/pool/lab", "network/pve1/zone/lab", "network/pve1/fabric/core", "sdn/pve1/lab"} {
		if CanSeeResourceID(id, sysOnly) {
			t.Errorf("Sys.Audit on / read %s", id)
		}
	}
	zone := NormalizePermissions(map[string]any{"/sdn/zones/lab": map[string]any{"SDN.Audit": 1.0}})
	if !CanSeeResourceID("network/pve1/zone/lab", zone) || CanSeeResourceID("network/pve1/zone/other", zone) {
		t.Error("a zone follows its own path")
	}
	fabric := NormalizePermissions(map[string]any{"/sdn/fabrics/core": map[string]any{"SDN.Audit": 1.0}})
	if !CanSeeResourceID("network/pve1/fabric/core", fabric) {
		t.Error("a fabric follows its own path")
	}
	got := FilterResources([]Resource{
		{ID: "network/pve1/zone/lab", Type: "network"},
		{ID: "network/pve1/zone/other", Type: "network"},
	}, zone)
	if len(got) != 1 || got[0].ID != "network/pve1/zone/lab" {
		t.Errorf("network rows: %+v", got)
	}
	if got := FilterResources([]Resource{{ID: "network/pve1/zone/lab", Type: "network"}}, sysOnly); len(got) != 0 {
		t.Errorf("Sys.Audit showed a zone: %+v", got)
	}
}

// Proxmox suppresses types it doesn't know so nothing leaks. So do we.
func TestUnknownResourcesAreDenied(t *testing.T) {
	root := NormalizePermissions(map[string]any{"/": map[string]any{
		"Sys.Audit": 1.0, "VM.Audit": 1.0, "Datastore.Audit": 1.0, "Pool.Audit": 1.0, "SDN.Audit": 1.0,
	}})
	for _, id := range []string{"", "anything", "future/thing", "network/pve1/unknown/x", "qemu/abc", "qemu/100/extra"} {
		if CanSeeResourceID(id, root) {
			t.Errorf("%q was readable", id)
		}
	}
	if got := FilterResources([]Resource{{ID: "future/thing", Type: "future"}}, root); len(got) != 0 {
		t.Errorf("unknown row kept: %+v", got)
	}
}

func TestRestrictToCoversNetworkRows(t *testing.T) {
	acl := NormalizePermissions(map[string]any{"/": map[string]any{"SDN.Audit": 1.0}, "/sdn": map[string]any{"SDN.Audit": 1.0}})
	acl.restrictTo([]Resource{
		{ID: "network/pve1/zone/lab", Type: "network"},
		{ID: "/pool/team", Type: "pool", Pool: "team"},
	})
	if !CanSeeResourceID("network/pve1/zone/lab", acl) || !CanSeeResourceID("/pool/team", acl) {
		t.Error("the viewer's own rows must stay readable")
	}
	if CanSeeResourceID("network/pve1/zone/other", acl) {
		t.Error("a zone outside the viewer's own list must be hidden")
	}
}
