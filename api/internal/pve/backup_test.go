package pve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackupJobUnmarshal(t *testing.T) {
	raw := `{
		"id":"backup-nightly",
		"enabled":true,
		"all":false,
		"schedule":"21:00",
		"storage":"zs-pbs-01",
		"mode":"snapshot",
		"compress":0,
		"vmid":112,
		"fleecing":{"enabled":1,"storage":"local-zfs"},
		"prune-backups":{"keep-last":"3","keep-daily":7},
		"mailto":["ops@example.com","oncall@example.com"]
	}`
	var job BackupJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID != "backup-nightly" || job.Enabled != 1 || job.All != 0 {
		t.Fatalf("flags %+v", job)
	}
	if job.VMID != "112" || job.Compress != "0" {
		t.Fatalf("scalar %+v", job)
	}
	if job.Fleecing != "enabled=1,storage=local-zfs" {
		t.Fatalf("fleecing %q", job.Fleecing)
	}
	if job.PruneBackups != "keep-daily=7,keep-last=3" {
		t.Fatalf("prune %q", job.PruneBackups)
	}
	if job.Mailto != "ops@example.com,oncall@example.com" {
		t.Fatalf("mailto %q", job.Mailto)
	}

	if err := json.Unmarshal([]byte(`{"id":"old","enabled":"","vmid":"100,101"}`), &job); err != nil {
		t.Fatal(err)
	}
	if job.Enabled != 1 || job.VMID != "100,101" {
		t.Fatalf("legacy %+v", job)
	}
}

func TestBackupVolume(t *testing.T) {
	cases := map[string][2]string{
		"pbs:backup/vm/100/2024-01-01T00:00:00Z":   {"pbs", "backup/vm/100/2024-01-01T00:00:00Z"},
		"backup/vm/100/2024-01-01T00:00:00Z":       {"pbs", "backup/vm/100/2024-01-01T00:00:00Z"},
		"other:backup/ct/101/2024-01-01T00:00:00Z": {"pbs", "backup/ct/101/2024-01-01T00:00:00Z"},
	}
	for in, pair := range cases {
		if got := backupVolume(pair[0], in); got != pair[1] {
			t.Fatalf("backupVolume(%q, %q)=%q want %q", pair[0], in, got, pair[1])
		}
	}
}

func TestCreateReplicationNextID(t *testing.T) {
	var posted string
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/replication", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "112-0", "guest": 112, "jobnum": 0, "target": "zs-hv2"}},
			})
		case http.MethodPost:
			_ = r.ParseForm()
			posted = r.Form.Get("id")
			if posted != "112-1" {
				http.Error(w, "expected 112-1 got "+posted, http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CreateReplication(context.Background(), ReplicationRequest{Guest: 112, Target: "zs-hv3"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if posted != "112-1" {
		t.Fatalf("posted id %q", posted)
	}
}

func TestFileRestorePath(t *testing.T) {
	if got := fileRestorePath(""); got != "/" {
		t.Fatalf("empty filepath = %q, want /", got)
	}
	if got := fileRestorePath("  "); got != "/" {
		t.Fatalf("blank filepath = %q, want /", got)
	}
	if got := fileRestorePath("L2V0Yy9wYXNzd2Q="); got != "L2V0Yy9wYXNzd2Q=" {
		t.Fatalf("base64 filepath rewritten: %q", got)
	}
}

func TestNextReplicationID(t *testing.T) {
	cases := []struct {
		name  string
		guest int
		jobs  []ReplicationJob
		want  string
	}{
		{name: "first job", guest: 112, want: "112-0"},
		{
			name:  "second job",
			guest: 112,
			jobs:  []ReplicationJob{{ID: "112-0", Guest: 112, JobNum: 0}},
			want:  "112-1",
		},
		{
			name:  "fill hole",
			guest: 112,
			jobs:  []ReplicationJob{{ID: "112-0", Guest: 112}, {ID: "112-2", Guest: 112, JobNum: 2}},
			want:  "112-1",
		},
		{
			name:  "ignore other guests",
			guest: 100,
			jobs:  []ReplicationJob{{ID: "1000-0", Guest: 1000}, {ID: "100-0", Guest: 100}},
			want:  "100-1",
		},
	}
	for _, tc := range cases {
		if got := nextReplicationID(tc.jobs, tc.guest); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestFileRestoreEntryLeafJSON(t *testing.T) {
	for _, raw := range []string{
		`{"filepath":"abc","leaf":true}`,
		`{"filepath":"abc","leaf":1}`,
	} {
		var e FileRestoreEntry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		if e.Leaf == nil || !*e.Leaf {
			t.Fatalf("leaf not true for %s", raw)
		}
	}
	for _, raw := range []string{
		`{"filepath":"abc","leaf":false}`,
		`{"filepath":"abc","leaf":0}`,
	} {
		var e FileRestoreEntry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		if e.Leaf == nil || *e.Leaf {
			t.Fatalf("leaf not false for %s", raw)
		}
	}
}
