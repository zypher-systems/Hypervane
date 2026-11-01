package httpapi

import (
	"encoding/json"
	"testing"

	"hypervane/internal/pve"
	"hypervane/internal/store"
)

func TestAuditScope(t *testing.T) {
	local := store.User{Kind: store.UserKindLocal, Username: "admin"}
	if got := auditScope(local, store.Session{}, nil); got != (store.AuditFilter{}) {
		t.Fatalf("local admin scope %+v", got)
	}
	alice := store.User{Kind: store.UserKindPVE, Username: "alice@pve"}
	sess := store.Session{ConnectionID: "lab-a"}
	plain := pve.NormalizePermissions(map[string]any{"/vms/100": map[string]any{"VM.Audit": 1.0}})
	if got := auditScope(alice, sess, plain); got != (store.AuditFilter{ConnectionID: "lab-a", Username: "alice@pve"}) {
		t.Fatalf("plain user scope %+v", got)
	}
	auditor := pve.NormalizePermissions(map[string]any{"/": map[string]any{"Sys.Audit": 1.0}})
	if got := auditScope(alice, sess, auditor); got != (store.AuditFilter{ConnectionID: "lab-a"}) {
		t.Fatalf("auditor scope %+v", got)
	}
	// A VM-level grant is not the same as auditing the whole cluster.
	nested := pve.NormalizePermissions(map[string]any{"/vms/100": map[string]any{"Sys.Audit": 1.0}})
	if got := auditScope(alice, sess, nested); got.Username == "" {
		t.Fatalf("nested Sys.Audit widened scope: %+v", got)
	}
	if got := auditScope(alice, sess, pve.ACL{}); got.Username == "" || got.ConnectionID == "" {
		t.Fatalf("failed ACL lookup widened scope: %+v", got)
	}
}

func TestMCPResourceToolsShowOnlyWhatTheyAreGiven(t *testing.T) {
	acl := pve.NormalizePermissions(map[string]any{"/vms/100": map[string]any{"VM.Audit": 1.0}})
	all := []pve.Resource{
		{ID: "qemu/100", Type: "qemu", VMID: 100},
		{ID: "qemu/200", Type: "qemu", VMID: 200},
		{ID: "node/pve1", Type: "node", Node: "pve1"},
	}
	visible := pve.FilterResources(all, acl)

	text, err := mcpResourceTool("hypervane_guests", nil, nil, visible)
	if err != nil {
		t.Fatal(err)
	}
	var guests []pve.Resource
	if err := json.Unmarshal([]byte(text), &guests); err != nil {
		t.Fatal(err)
	}
	if len(guests) != 1 || guests[0].VMID != 100 {
		t.Fatalf("guests %+v", guests)
	}
	if _, err := mcpResourceTool("hypervane_guest", map[string]any{"vmid": 200.0}, nil, visible); err == nil {
		t.Fatal("hidden guest was returned")
	}
	text, err = mcpResourceTool("hypervane_summary", nil, nil, visible)
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Resources int `json:"resources"`
	}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Resources != 1 {
		t.Fatalf("summary counted hidden resources: %d", summary.Resources)
	}
}
