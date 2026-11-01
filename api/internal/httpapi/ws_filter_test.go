package httpapi

import (
	"encoding/json"
	"testing"

	"hypervane/internal/hub"
	"hypervane/internal/pve"
)

const (
	labA = "lab-a"
	labB = "lab-b"
)

func vm100ACL() pve.ACL {
	return pve.NormalizePermissions(map[string]any{
		"/vms/100":    map[string]any{"VM.Audit": 1.0},
		"/nodes/pve1": map[string]any{"Sys.Audit": 1.0},
	})
}

func hubEvent(t *testing.T, topic string, data any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"type": "event", "topic": topic, "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFilterHubMessageResources(t *testing.T) {
	raw := hubEvent(t, "resources", map[string]any{
		"connection": labA,
		"ts":         1234,
		"status": []pve.ClusterStatus{
			{Type: "cluster", Name: "lab", Quorate: 1},
			{Type: "node", Name: "pve1", IP: "10.0.0.1"},
			{Type: "node", Name: "pve2", IP: "10.0.0.2"},
		},
		"resources": []pve.Resource{
			{ID: "cluster/c", Type: "cluster"},
			{ID: "qemu/100", Type: "qemu", VMID: 100},
			{ID: "qemu/200", Type: "qemu", VMID: 200},
		},
	})
	out, keep := filterHubMessage(raw, vm100ACL(), labA)
	if !keep {
		t.Fatal("keep resources")
	}
	var ev struct {
		Data struct {
			Connection string              `json:"connection"`
			TS         int64               `json:"ts"`
			Status     []pve.ClusterStatus `json:"status"`
			Resources  []pve.Resource      `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Data.Connection != labA || ev.Data.TS != 1234 {
		t.Fatalf("envelope lost: %+v", ev.Data)
	}
	if len(ev.Data.Resources) != 2 {
		t.Fatalf("got %d resources", len(ev.Data.Resources))
	}
	if len(ev.Data.Status) != 2 || ev.Data.Status[1].Name != "pve1" {
		t.Fatalf("status %+v", ev.Data.Status)
	}
}

// A matching VMID on another cluster is a different guest.
func TestFilterHubMessageDropsOtherConnections(t *testing.T) {
	tasks := []pve.Task{{UPID: "UPID:pve1:1:1:1:qmstart:100:user@pam:", Node: "pve1", ID: "100"}}
	resources := []pve.Resource{{ID: "qemu/100", Type: "qemu", VMID: 100}}
	cases := map[string][]byte{
		"tasks elsewhere":        hubEvent(t, "tasks", map[string]any{"connection": labB, "tasks": tasks}),
		"tasks unattributed":     hubEvent(t, "tasks", map[string]any{"tasks": tasks}),
		"bare task array":        hubEvent(t, "tasks", tasks),
		"resources elsewhere":    hubEvent(t, "resources", map[string]any{"connection": labB, "resources": resources}),
		"resources unattributed": hubEvent(t, "resources", map[string]any{"resources": resources}),
		"resources malformed":    hubEvent(t, "resources", "not-a-payload"),
		"guest elsewhere":        hubEvent(t, hub.ScopedTopic(labB, "guest:pve1:qemu:100"), map[string]any{}),
		"node elsewhere":         hubEvent(t, hub.ScopedTopic(labB, "node:pve1"), map[string]any{}),
		"task elsewhere":         hubEvent(t, hub.ScopedTopic(labB, "task:UPID:pve1:1:1:1:qmstart:100:user@pam:"), map[string]any{}),
		"bare guest":             hubEvent(t, "guest:pve1:qemu:100", map[string]any{}),
		"bare node":              hubEvent(t, "node:pve1", map[string]any{}),
		"unknown topic":          hubEvent(t, "something-new", map[string]any{}),
		"not json":               []byte("{"),
	}
	for name, raw := range cases {
		if _, keep := filterHubMessage(raw, vm100ACL(), labA); keep {
			t.Errorf("%s must drop", name)
		}
	}
}

func TestFilterHubMessageTasksPayload(t *testing.T) {
	raw := hubEvent(t, "tasks", map[string]any{
		"connection": labA,
		"tasks": []pve.Task{
			{UPID: "UPID:pve1:1:1:1:qmstart:100:user@pam:", Node: "pve1", ID: "100"},
			{UPID: "UPID:pve2:1:1:1:qmstart:200:user@pam:", Node: "pve2", ID: "200"},
		},
	})
	out, keep := filterHubMessage(raw, vm100ACL(), labA)
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
	if ev.Data.Connection != labA {
		t.Fatalf("connection %q", ev.Data.Connection)
	}
	if len(ev.Data.Tasks) != 1 || ev.Data.Tasks[0].ID != "100" {
		t.Fatalf("tasks %+v", ev.Data.Tasks)
	}
}

func TestFilterHubMessageDetailTopics(t *testing.T) {
	hidden := []string{
		"task:UPID:pve2:1:1:1:qmstart:200:user@pam:",
		"node:pve2",
		"rrd:qemu:pve1:200:hour",
		"rrd:node:pve2:hour",
		"guest:pve1:qemu:200",
	}
	for _, detail := range hidden {
		if _, keep := filterHubMessage(hubEvent(t, hub.ScopedTopic(labA, detail), map[string]any{}), vm100ACL(), labA); keep {
			t.Fatalf("should drop %s", detail)
		}
	}
	visible := []string{
		"task:UPID:pve1:1:1:1:qmstart:100:user@pam:",
		"node:pve1",
		"rrd:qemu:pve1:100:hour",
		"guest:pve1:qemu:100",
	}
	for _, detail := range visible {
		if _, keep := filterHubMessage(hubEvent(t, hub.ScopedTopic(labA, detail), map[string]any{}), vm100ACL(), labA); !keep {
			t.Fatalf("should keep %s", detail)
		}
	}
}

func TestFilterHubMessageDropsUnparseableTasks(t *testing.T) {
	if _, keep := filterHubMessage(hubEvent(t, "tasks", "not-a-task-list"), vm100ACL(), labA); keep {
		t.Fatal("unparseable tasks must drop, not leak")
	}
}

func TestTopicAllowed(t *testing.T) {
	guest := hub.ScopedTopic(labA, "guest:pve1:qemu:100")
	other := hub.ScopedTopic(labB, "guest:pve1:qemu:100")
	hiddenGuest := hub.ScopedTopic(labA, "guest:pve1:qemu:200")
	cases := []struct {
		name  string
		topic string
		acl   pve.ACL
		want  bool
	}{
		{"broadcast", "resources", vm100ACL(), true},
		{"own guest", guest, vm100ACL(), true},
		{"other connection", other, vm100ACL(), false},
		{"hidden guest", hiddenGuest, vm100ACL(), false},
		{"bare topic", "guest:pve1:qemu:100", vm100ACL(), false},
		{"unknown detail", hub.ScopedTopic(labA, "bogus"), vm100ACL(), false},
		{"empty ACL", guest, pve.ACL{}, false},
		{"admin elsewhere", other, nil, true},
		{"admin bare topic", "node:pve1", nil, false},
		{"admin unknown detail", hub.ScopedTopic(labB, "bogus"), nil, false},
	}
	for _, c := range cases {
		if got := topicAllowed(c.topic, c.acl, labA); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
