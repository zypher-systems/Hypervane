package httpapi

import (
	"encoding/json"
	"testing"

	"hypervane/internal/pve"
)

func TestFilterHubMessage(t *testing.T) {
	acl := pve.NormalizePermissions(map[string]any{
		"/vms/100": map[string]any{"VM.Audit": 1.0},
	})
	raw, _ := json.Marshal(map[string]any{
		"type":  "event",
		"topic": "resources",
		"data": map[string]any{
			"resources": []pve.Resource{
				{ID: "cluster/c", Type: "cluster"},
				{ID: "qemu/100", Type: "qemu", VMID: 100},
				{ID: "qemu/200", Type: "qemu", VMID: 200},
			},
		},
	})
	out, keep := filterHubMessage(raw, acl)
	if !keep {
		t.Fatal("keep resources")
	}
	var ev struct {
		Data struct {
			Resources []pve.Resource `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev.Data.Resources) != 2 {
		t.Fatalf("got %d resources", len(ev.Data.Resources))
	}
	guest, _ := json.Marshal(map[string]any{"type": "event", "topic": "guest:pve1:qemu:200", "data": map[string]any{}})
	if _, keep := filterHubMessage(guest, acl); keep {
		t.Fatal("guest 200 should drop")
	}
}

func TestFilterHubMessageTasksPayload(t *testing.T) {
	acl := pve.NormalizePermissions(map[string]any{
		"/vms/100": map[string]any{"VM.Audit": 1.0},
	})
	raw, _ := json.Marshal(map[string]any{
		"type":  "event",
		"topic": "tasks",
		"data": map[string]any{
			"connection": "lab-a",
			"tasks": []pve.Task{
				{UPID: "UPID:pve1:1:1:1:qmstart:100:user@pam:", Node: "pve1", ID: "100"},
				{UPID: "UPID:pve1:1:1:1:qmstart:200:user@pam:", Node: "pve2", ID: "200"},
			},
		},
	})
	out, keep := filterHubMessage(raw, acl)
	if !keep {
		t.Fatal("keep tasks")
	}
	var ev struct {
		Data struct {
			Connection string     `json:"connection"`
			Tasks      []pve.Task `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Data.Connection != "lab-a" {
		t.Fatalf("connection %q", ev.Data.Connection)
	}
	if len(ev.Data.Tasks) != 1 || ev.Data.Tasks[0].ID != "100" {
		t.Fatalf("tasks %+v", ev.Data.Tasks)
	}
}

func TestFilterHubMessageDropsHiddenTopics(t *testing.T) {
	acl := pve.NormalizePermissions(map[string]any{
		"/vms/100":    map[string]any{"VM.Audit": 1.0},
		"/nodes/pve1": map[string]any{"Sys.Audit": 1.0},
	})
	hidden := []string{
		"task:UPID:pve2:1:1:1:qmstart:200:user@pam:",
		"node:pve2",
		"rrd:qemu:pve1:200:hour",
		"rrd:node:pve2:hour",
	}
	for _, topic := range hidden {
		raw, _ := json.Marshal(map[string]any{"type": "event", "topic": topic, "data": map[string]any{}})
		if _, keep := filterHubMessage(raw, acl); keep {
			t.Fatalf("should drop %s", topic)
		}
	}
	visible := []string{
		"task:UPID:pve1:1:1:1:qmstart:100:user@pam:",
		"node:pve1",
		"rrd:qemu:pve1:100:hour",
		"guest:pve1:qemu:100",
	}
	for _, topic := range visible {
		raw, _ := json.Marshal(map[string]any{"type": "event", "topic": topic, "data": map[string]any{}})
		if _, keep := filterHubMessage(raw, acl); !keep {
			t.Fatalf("should keep %s", topic)
		}
	}
}

func TestFilterHubMessageDropsUnparseableTasks(t *testing.T) {
	acl := pve.NormalizePermissions(map[string]any{
		"/vms/100": map[string]any{"VM.Audit": 1.0},
	})
	raw, _ := json.Marshal(map[string]any{
		"type":  "event",
		"topic": "tasks",
		"data":  "not-a-task-list",
	})
	if _, keep := filterHubMessage(raw, acl); keep {
		t.Fatal("unparseable tasks must drop, not leak")
	}
}
