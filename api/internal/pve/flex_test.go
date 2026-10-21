package pve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFlexIntAcceptsNumberAndString(t *testing.T) {
	var out []PrunePreview
	raw := `[{"volid":"a","vmid":"100"},{"volid":"b","vmid":101},{"volid":"c","vmid":""},{"volid":"d"}]`
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	want := []FlexInt{100, 101, 0, 0}
	for i, w := range want {
		if out[i].VMID != w {
			t.Fatalf("row %d: got %d want %d", i, out[i].VMID, w)
		}
	}
}

func TestFlexCSVAcceptsStringAndArray(t *testing.T) {
	var single, list AccessUser
	if err := json.Unmarshal([]byte(`{"userid":"a@pve","groups":"ops,dev"}`), &single); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"userid":"root@pam","groups":["ops","dev"]}`), &list); err != nil {
		t.Fatal(err)
	}
	if single.Groups != "ops,dev" || list.Groups != "ops,dev" {
		t.Fatalf("got %q and %q", single.Groups, list.Groups)
	}
	b, _ := json.Marshal(list)
	if string(b) == "" || !json.Valid(b) {
		t.Fatalf("re-encode %s", b)
	}
	var back map[string]any
	_ = json.Unmarshal(b, &back)
	if back["groups"] != "ops,dev" {
		t.Fatalf("groups should re-encode as a string, got %#v", back["groups"])
	}
}

func TestCertInfoDecodesEpochDates(t *testing.T) {
	var out []CertInfo
	raw := `[{"filename":"pve-ssl.pem","notbefore":1700000000,"notafter":1763072000,"subject":"CN=zs-hv1"}]`
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out[0].NotAfter != 1763072000 || out[0].NotBefore != 1700000000 {
		t.Fatalf("%+v", out[0])
	}
}

func TestReplicationDurationIsFractional(t *testing.T) {
	var st ReplicationStatus
	if err := json.Unmarshal([]byte(`{"id":"100-0","duration":4.517741,"last_sync":1790000000}`), &st); err != nil {
		t.Fatal(err)
	}
	if st.Duration < 4.5 || st.Duration > 4.6 {
		t.Fatalf("%v", st.Duration)
	}
}

func TestDecodeFlexibleMaps(t *testing.T) {
	cases := map[string]int{
		`[{"a":1},{"a":2}]`: 2,
		`{"leaf":0,"children":[{"name":"pve","size":1},{"name":"data","size":2}]}`: 2, // LVM tree
		`{"State":"Connected","Algorithm":"ffsplit"}`:                              1, // qdevice status
		`{}`:   0,
		`null`: 0,
		`[]`:   0,
	}
	for raw, want := range cases {
		got, err := decodeFlexibleMaps(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if len(got) != want {
			t.Fatalf("%s: got %d rows want %d", raw, len(got), want)
		}
	}
}

// Proxmox returns 50 log lines unless a limit is given; the tail must come
// from the end of the log, with the original line numbers.
func TestTaskLogTailReadsTheEnd(t *testing.T) {
	const total = 1234
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/n1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 50
		}
		lines := []TaskLogLine{}
		for n := start + 1; n <= total && len(lines) < limit; n++ {
			lines = append(lines, TaskLogLine{N: n, T: fmt.Sprintf("line %d", n)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": lines, "total": total})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	lines, n, err := c.TaskLogTail(context.Background(), "n1", "UPID:n1:1:2:3:vzdump::root@pam:", 200)
	if err != nil {
		t.Fatal(err)
	}
	if n != total || len(lines) != 200 {
		t.Fatalf("total %d lines %d", n, len(lines))
	}
	if lines[0].N != total-199 || lines[len(lines)-1].N != total {
		t.Fatalf("range %d..%d", lines[0].N, lines[len(lines)-1].N)
	}
}
