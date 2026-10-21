package scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"hypervane/internal/pve"
)

// fakePVE is a small Proxmox stand-in: it serves the reads the recipes need
// from its state, records every write, and finishes every task at once
// (failing the ones listed in failTasks).
type fakePVE struct {
	t *testing.T

	mu        sync.Mutex
	resources []pve.Resource
	storages  []pve.Storage
	configs   map[int]map[string]any
	content   map[string][]pve.StorageContent // "node/storage"
	snapshots map[int][]pve.Snapshot
	networks  map[string][]pve.NetIface
	missing   []pve.CoverageGuest
	updates   map[string]int
	nextID    int
	// failTasks makes the task started by "<type>:<id>" end in an error.
	failTasks map[string]bool
	// failCalls makes "METHOD path" return a 500.
	failCalls map[string]bool

	calls []call
}

type call struct {
	Method string
	Path   string
	Form   url.Values
}

func (c call) String() string { return c.Method + " " + c.Path }

func newFake(t *testing.T) (*fakePVE, *pve.Client) {
	f := &fakePVE{
		t: t,
		resources: []pve.Resource{
			{Type: "node", Node: "pve1", Status: "online"},
			{Type: "node", Node: "pve2", Status: "online"},
			{Type: "qemu", VMID: 101, Name: "web", Node: "pve1", Status: "running"},
			{Type: "qemu", VMID: 102, Name: "db", Node: "pve1", Status: "stopped"},
			{Type: "lxc", VMID: 103, Name: "dns", Node: "pve1", Status: "running"},
			{Type: "qemu", VMID: 201, Name: "app", Node: "pve2", Status: "running"},
			{Type: "qemu", VMID: 9000, Name: "debian-13-cloud", Node: "pve1", Status: "stopped", Template: 1},
		},
		storages: []pve.Storage{
			{Storage: "local", Type: "dir", Content: "iso,vztmpl,backup,import"},
			{Storage: "local-lvm", Type: "lvmthin", Content: "images,rootdir"},
			{Storage: "pbs", Type: "pbs", Content: "backup"},
			{Storage: "nfs", Type: "nfs", Content: "backup,images", Shared: 1},
			{Storage: "pve2-only", Type: "dir", Content: "backup", Nodes: "pve2"},
		},
		configs: map[int]map[string]any{
			101: {"name": "web", "agent": "0,fstrim_cloned_disks=1", "onboot": 1},
			102: {"name": "db", "onboot": 1},
			103: {"hostname": "dns"},
			201: {"name": "app"},
			9000: {
				"name": "debian-13-cloud", "cores": 4, "memory": 4096, "cpu": "host",
				"net0":        "virtio=BC:24:11:00:00:01,bridge=vmbr1,tag=20",
				"scsi0":       "local-lvm:base-9000-disk-0,discard=on,iothread=1,size=40G",
				"ide2":        "local-lvm:vm-9000-cloudinit,media=cdrom",
				"ciuser":      "ops",
				"sshkeys":     pve.EncodeSSHKeys("ssh-ed25519 AAAAC3Nza+k/ey= me@laptop"),
				"ipconfig0":   "ip=dhcp",
				"tags":        "base",
				"description": "Our base image.\n\nBuilt by Hypervane from Debian 13 (trixie) (debian-13-generic-amd64.qcow2) on 2026-01-01.\nhypervane-recipe: debian-13-template",
				"template":    1,
			},
		},
		content:   map[string][]pve.StorageContent{},
		snapshots: map[int][]pve.Snapshot{},
		networks: map[string][]pve.NetIface{
			"pve1": {{Iface: "vmbr0", Type: "bridge"}, {Iface: "vmbr1", Type: "bridge"}, {Iface: "eno1", Type: "eth"}},
			"pve2": {{Iface: "vmbr0", Type: "bridge"}},
		},
		missing:   []pve.CoverageGuest{{VMID: 101, Type: "qemu"}, {VMID: 103, Type: "lxc"}, {VMID: 201, Type: "qemu"}},
		updates:   map[string]int{"pve1": 3, "pve2": 0},
		nextID:    104,
		failTasks: map[string]bool{},
		failCalls: map[string]bool{},
	}
	srv := httptest.NewUnstartedServer(f)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	c, err := pve.New(pve.Options{BaseURL: srv.URL, Token: "root@pam!hv=abc", TLSMode: pve.TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

var (
	guestPathRe = regexp.MustCompile(`^/nodes/([^/]+)/(qemu|lxc)/([0-9]+)(/.*)?$`)
	nodePathRe  = regexp.MustCompile(`^/nodes/([^/]+)(/.*)$`)
)

func (f *fakePVE) upid(node, typ, id string) string {
	return fmt.Sprintf("UPID:%s:00000001:00000001:%08X:%s:%s:root@pam:", node, time.Now().UnixNano()&0xffffff, typ, id)
}

func (f *fakePVE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	path := strings.TrimPrefix(r.URL.Path, "/api2/json")
	f.mu.Lock()
	defer f.mu.Unlock()
	form := url.Values{}
	for k, v := range r.PostForm {
		form[k] = v
	}
	if r.Method == http.MethodDelete {
		form = r.URL.Query()
	}
	if r.Method != http.MethodGet {
		f.calls = append(f.calls, call{Method: r.Method, Path: path, Form: form})
	}
	if f.failCalls[r.Method+" "+path] {
		http.Error(w, `{"message":"forced failure"}`, http.StatusInternalServerError)
		return
	}
	data, status := f.route(r, path)
	if status != 0 {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": fmt.Sprint(data)})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (f *fakePVE) route(r *http.Request, path string) (any, int) {
	switch {
	case path == "/cluster/resources":
		return f.resources, 0
	case path == "/storage":
		return f.storages, 0
	case path == "/cluster/nextid":
		return fmt.Sprint(f.nextID), 0
	case path == "/cluster/backup-info/not-backed-up":
		return f.missing, 0
	case path == "/cluster/backup" && r.Method == http.MethodPost:
		return nil, 0
	case strings.HasPrefix(path, "/cluster/backup/"):
		return map[string]any{"id": strings.TrimPrefix(path, "/cluster/backup/"), "enabled": 1}, 0
	}
	if m := guestPathRe.FindStringSubmatch(path); m != nil {
		node, kind, id, rest := m[1], m[2], m[3], m[4]
		vmid := atoi(id, 0)
		switch {
		case rest == "/config" && r.Method == http.MethodGet:
			if cfg, ok := f.configs[vmid]; ok {
				return cfg, 0
			}
			return "no such guest", http.StatusNotFound
		case rest == "/config" && r.Method == http.MethodPut:
			return nil, 0
		case rest == "/snapshot" && r.Method == http.MethodGet:
			return append(f.snapshots[vmid], pve.Snapshot{Name: "current"}), 0
		case rest == "/snapshot":
			return f.upid(node, kind+"snapshot", id), 0
		case strings.HasPrefix(rest, "/snapshot/"):
			return f.upid(node, kind+"delsnapshot", id), 0
		case rest == "/clone":
			// Keyed by the new VMID, so a test can fail one clone of many.
			return f.upid(node, kind+"clone", r.PostForm.Get("newid")), 0
		case rest == "/template":
			return f.upid(node, kind+"template", id), 0
		case rest == "/resize":
			return f.upid(node, "resize", id), 0
		case rest == "/status/start":
			return f.upid(node, kind+"start", id), 0
		case rest == "" && r.Method == http.MethodDelete:
			delete(f.configs, vmid)
			return f.upid(node, kind+"destroy", id), 0
		}
	}
	if m := nodePathRe.FindStringSubmatch(path); m != nil {
		node, rest := m[1], m[2]
		switch {
		case rest == "/qemu" && r.Method == http.MethodPost:
			// A new VM's imported disk is the size of the cloud image.
			vmid := r.PostForm.Get("vmid")
			disk := strings.SplitN(r.PostForm.Get("scsi0"), ":", 2)[0]
			f.configs[atoi(vmid, 0)] = map[string]any{"name": r.PostForm.Get("name"), "scsi0": disk + ":vm-" + vmid + "-disk-0,size=3G"}
			return f.upid(node, "qmcreate", vmid), 0
		case rest == "/network":
			return f.networks[node], 0
		case strings.HasPrefix(rest, "/tasks/") && strings.HasSuffix(rest, "/status"):
			upid := strings.TrimSuffix(strings.TrimPrefix(rest, "/tasks/"), "/status")
			_, typ, id := pve.ParseUPID(upid)
			if f.failTasks[typ+":"+id] {
				return map[string]any{"status": "stopped", "exitstatus": "task failed on purpose"}, 0
			}
			return map[string]any{"status": "stopped", "exitstatus": "OK"}, 0
		case rest == "/apt/update" && r.Method == http.MethodPost:
			return f.upid(node, "aptupdate", ""), 0
		case rest == "/apt/update":
			out := []map[string]any{}
			for i := 0; i < f.updates[node]; i++ {
				out = append(out, map[string]any{"Package": fmt.Sprintf("pkg%d", i)})
			}
			return out, 0
		case rest == "/stopall", rest == "/startall", rest == "/migrateall":
			return f.upid(node, strings.TrimPrefix(rest, "/"), ""), 0
		case rest == "/vzdump":
			return f.upid(node, "vzdump", r.PostForm.Get("vmid")), 0
		case strings.HasPrefix(rest, "/storage/"):
			parts := strings.SplitN(strings.TrimPrefix(rest, "/storage/"), "/", 2)
			store, sub := parts[0], ""
			if len(parts) == 2 {
				sub = parts[1]
			}
			switch {
			case sub == "download-url":
				return f.upid(node, "download", r.PostForm.Get("filename")), 0
			case sub == "content" && r.Method == http.MethodGet:
				return f.content[node+"/"+store], 0
			case strings.HasPrefix(sub, "content/") && r.Method == http.MethodDelete:
				return f.upid(node, "imgdel", ""), 0
			}
		}
	}
	return "not faked: " + r.Method + " " + path, http.StatusNotImplemented
}

// writes returns the recorded calls, as "METHOD path".
func (f *fakePVE) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c.String())
	}
	return out
}

// form returns the form of the first call to METHOD path.
func (f *fakePVE) form(method, path string) url.Values {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			return c.Form
		}
	}
	f.t.Fatalf("no %s %s among %v", method, path, f.calls)
	return nil
}

func (f *fakePVE) reset() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

// plan prepares a recipe against the fake, with no network lookups for images.
func plan(t *testing.T, c *pve.Client, id string, p Params) (*Plan, error) {
	t.Helper()
	r, ok := Lookup(id)
	if !ok {
		t.Fatalf("no recipe %s", id)
	}
	return Prepare(context.Background(), c, nil, r, p)
}

func mustPlan(t *testing.T, c *pve.Client, id string, p Params) *Plan {
	t.Helper()
	pl, err := plan(t, c, id, p)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	return pl
}

// runJob runs a plan to the end and returns the finished job.
func runJob(t *testing.T, c *pve.Client, pl *Plan) Job {
	t.Helper()
	js := NewJobs()
	started := js.Start(c, pl, "u1")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := js.Get(started.ID, "u1", false)
		if j.Status != JobRunning {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}

func stepLabels(pl *Plan) []string {
	out := make([]string, len(pl.Steps))
	for i, s := range pl.Steps {
		out[i] = s.ID
	}
	return out
}

func wantInputError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil || !IsInputError(err) || !strings.Contains(err.Error(), contains) {
		t.Fatalf("want an input error containing %q, got %v", contains, err)
	}
}

func bptr(b bool) *bool { return &b }
