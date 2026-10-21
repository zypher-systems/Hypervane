package pve

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientVersionAndResources(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=root@pam!hv=abc" {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": Version{Version: "8.3", Release: "1", Repoid: "test"},
		})
	})
	mux.HandleFunc("/api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []Resource{{ID: "node/pve1", Type: "node", Node: "pve1", Status: "online", MaxCPU: 8}},
		})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/status/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0000:0000:00000000:qmstart:100:root@pam:"})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	c, err := New(Options{
		BaseURL: srv.URL,
		Token:   "root@pam!hv=abc",
		TLSMode: TLSSkipVerify,
		Client:  srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ver, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ver.Version != "8.3" {
		t.Fatalf("version %s", ver.Version)
	}
	res, err := c.ClusterResources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Node != "pve1" {
		t.Fatalf("resources %+v", res)
	}
	upid, err := c.GuestPower(context.Background(), "pve1", "qemu", "start", 100)
	if err != nil {
		t.Fatal(err)
	}
	if ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("upid node %s", upid)
	}
}

func TestGuestPowerHibernateUsesSuspendToDisk(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/status/hibernate", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no such action", http.StatusBadRequest)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/status/suspend", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("todisk") != "1" {
			http.Error(w, "todisk required", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0000:0000:00000000:qmsuspend:100:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	upid, err := c.GuestPower(context.Background(), "pve1", "qemu", "hibernate", 100)
	if err != nil {
		t.Fatal(err)
	}
	if ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("upid node %s", upid)
	}
	if _, err := c.GuestPower(context.Background(), "pve1", "lxc", "hibernate", 100); err == nil {
		t.Fatal("lxc hibernate should fail")
	}
}

func TestCloneMigrateResizeDownload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/clone", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0001:0000:00000000:qmclone:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/migrate", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("target") != "pve2" {
			http.Error(w, "target", 400)
			return
		}
		if r.Form.Get("targetstorage") != "" && (r.Form.Get("targetstorage") != "local-lvm" || r.Form.Get("migration_network") != "10.0.0.0/24") {
			http.Error(w, "migrate extras", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0002:0000:00000000:qmigrate:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/resize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0003:0000:00000000:resize:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/download-url", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0004:0000:00000000:download:local:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	upid, err := c.CloneGuest(ctx, "pve1", "qemu", 100, CloneRequest{NewID: 101, Name: "copy", Full: true})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("clone %s %v", upid, err)
	}
	upid, err = c.MigrateGuest(ctx, "pve1", "qemu", 100, MigrateRequest{Target: "pve2", Online: true})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("migrate %s %v", upid, err)
	}
	upid, err = c.MigrateGuest(ctx, "pve1", "qemu", 100, MigrateRequest{
		Target: "pve2", Online: true, TargetStorage: "local-lvm", MigrationNetwork: "10.0.0.0/24",
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("migrate extras %s %v", upid, err)
	}
	upids, err := c.BulkGuestAction(ctx, "migrate", BulkRequest{
		Guests: []BulkGuest{{VMID: 100, Node: "pve1", Type: "qemu"}},
		Target: "pve2", TargetStorage: "local-lvm", MigrationNetwork: "10.0.0.0/24",
	})
	if err != nil || len(upids) != 1 || ParseUPIDNode(upids[0]) != "pve1" {
		t.Fatalf("bulk migrate extras %v %v", upids, err)
	}
	upid, err = c.ResizeDisk(ctx, "pve1", "qemu", 100, DiskResizeRequest{Disk: "scsi0", Size: "+8G"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("resize %s %v", upid, err)
	}
	upid, err = c.DownloadURL(ctx, "pve1", "local", DownloadURLRequest{URL: "https://example.com/a.iso", Filename: "a.iso", Content: "iso"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("download %s %v", upid, err)
	}
}

func TestBackupRestorePruneReplicate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/backup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("storage") != "local" || r.Form.Get("fleecing") != "enabled=1,storage=local-lvm" || r.Form.Get("mailnotification") != "failure" || r.Form.Get("node") != "pve1" {
			http.Error(w, "backup fields", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/backup/backup-test", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": BackupJob{ID: "backup-test", Storage: "local", Schedule: "sun 00:00", Enabled: 1}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/vzdump", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("fleecing") != "" && (r.Form.Get("fleecing") != "enabled=1,storage=local-lvm" || r.Form.Get("bwlimit") != "1024" || r.Form.Get("compress") != "zstd") {
			http.Error(w, "vzdump extras", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0005:0000:00000000:vzdump:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0006:0000:00000000:qmrestore:200:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/prunebackups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []PrunePreview{{VolID: "local:backup/vzdump-qemu-100.vma.zst", Mark: "keep", VMID: 100}},
		})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/replication/100-0/schedule_now", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0007:0000:00000000:qreplicate:100:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	job, err := c.CreateBackupJob(ctx, BackupJobRequest{
		ID: "backup-test", Storage: "local", Schedule: "sun 00:00",
		Fleecing: "enabled=1,storage=local-lvm", MailNotification: "failure", Node: "pve1",
	})
	if err != nil || job.ID != "backup-test" {
		t.Fatalf("create job %+v %v", job, err)
	}
	upid, err := c.VZDump(ctx, "pve1", VZDumpRequest{VMID: 100, Storage: "local", Mode: "snapshot"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("vzdump %s %v", upid, err)
	}
	upid, err = c.VZDump(ctx, "pve1", VZDumpRequest{
		VMID: 100, Storage: "local", Mode: "snapshot", Compress: "zstd",
		Fleecing: "enabled=1,storage=local-lvm", Bwlimit: "1024",
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("vzdump extras %s %v", upid, err)
	}
	upid, err = c.RestoreGuest(ctx, RestoreRequest{Node: "pve1", Archive: "local:backup/vzdump-qemu-100.vma.zst", VMID: 200, Type: "qemu"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("restore %s %v", upid, err)
	}
	preview, err := c.PruneBackupsPreview(ctx, "pve1", "local", "keep-last=3", 0, "")
	if err != nil || len(preview) != 1 || preview[0].Mark != "keep" {
		t.Fatalf("prune %+v %v", preview, err)
	}
	upid, err = c.ReplicationScheduleNow(ctx, "pve1", "100-0")
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("schedule-now %s %v", upid, err)
	}
	if InferBackupType("local:backup/vzdump-lxc-101.tar.zst") != "lxc" {
		t.Fatal("infer lxc")
	}
}

func TestHAResourceRelocateGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/ha/resources", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/ha/resources/vm:100/relocate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/ha/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.CreateHAResource(ctx, HAResourceRequest{VMID: 100, Type: "qemu", State: "started"}); err != nil {
		t.Fatalf("create resource %v", err)
	}
	upid, err := c.HARelocate(ctx, "vm:100", "pve2")
	if err != nil {
		t.Fatalf("relocate %v", err)
	}
	if upid != "" {
		t.Fatalf("expected empty upid, got %s", upid)
	}
	if err := c.CreateHAGroup(ctx, HAGroupRequest{Group: "prod", Nodes: "pve1:1,pve2:2"}); err != nil {
		t.Fatalf("create group %v", err)
	}
	if HASid("lxc", 101) != "ct:101" || HASid("qemu", 100) != "vm:100" {
		t.Fatal("sid helper")
	}
}

func TestTicketAuthAndNeedTFA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("tfa-challenge") != "" {
			if r.Form.Get("password") != "123456" {
				http.Error(w, "bad otp", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"username":            "ops@pam",
					"ticket":              "PVE:ops@pam:full",
					"CSRFPreventionToken": "csrf-2",
				},
			})
			return
		}
		if r.Form.Get("username") != "ops@pam" || r.Form.Get("password") != "secret" {
			http.Error(w, "bad creds", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"username": "ops@pam",
				"ticket":   "PVE:!tfa!challenge",
				"NeedTFA":  1,
			},
		})
	})
	mux.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "PVEAuthCookie=PVE:ops@pam:full" {
			http.Error(w, "no ticket", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "token not allowed", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": Version{Version: "8.3"}})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "root@pam!hv=abc", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	first, err := c.CreateTicket(context.Background(), "ops@pam", "secret", "", "")
	if err != nil || !first.NeedTFA || first.Ticket != "PVE:!tfa!challenge" {
		t.Fatalf("need tfa %+v %v", first, err)
	}
	second, err := c.CreateTicket(context.Background(), "ops@pam", "secret", "123456", first.Ticket)
	if err != nil || second.NeedTFA || second.Ticket != "PVE:ops@pam:full" {
		t.Fatalf("complete tfa %+v %v", second, err)
	}
	ticketClient := c.WithTicket(second.Ticket, second.CSRF)
	ver, err := ticketClient.Version(context.Background())
	if err != nil || ver.Version != "8.3" {
		t.Fatalf("ticket version %v %v", ver, err)
	}
}

func TestAccessCRUDAndRealmSync404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/access/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []AccessUser{{UserID: "ops@pam", Enable: 1}},
		})
	})
	mux.HandleFunc("/api2/json/access/acl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []ACLEntry{{Path: "/vms/100", UGID: "ops@pam", Type: "user", RoleID: "PVEVMAdmin", Propagate: 1}},
		})
	})
	mux.HandleFunc("/api2/json/access/users/ops@pam/token/ci", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": AccessTokenCreate{TokenID: "ci", Value: "ops@pam!ci=secret-once"},
		})
	})
	mux.HandleFunc("/api2/json/access/roles/HVOps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("privs") != "VM.Audit,VM.Console" {
			http.Error(w, "privs", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/access/domains/ad/sync", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	users, err := c.ListUsers(ctx)
	if err != nil || len(users) != 1 || users[0].UserID != "ops@pam" {
		t.Fatalf("users %+v %v", users, err)
	}
	if err := c.CreateUser(ctx, AccessUserRequest{UserID: "new@pve", Password: "abcdefgh"}); err != nil {
		t.Fatalf("create user %v", err)
	}
	acl, err := c.ListACL(ctx)
	if err != nil || len(acl) != 1 {
		t.Fatalf("acl %+v %v", acl, err)
	}
	if err := c.UpdateACL(ctx, ACLRequest{Path: "/vms/100", Roles: "PVEVMAdmin", Users: "ops@pam"}); err != nil {
		t.Fatalf("update acl %v", err)
	}
	if err := c.UpdateRole(ctx, "HVOps", "VM.Audit,VM.Console"); err != nil {
		t.Fatalf("update role %v", err)
	}
	tok, err := c.CreateToken(ctx, "ops@pam", AccessTokenCreate{TokenID: "ci"})
	if err != nil || tok.Value == "" {
		t.Fatalf("token %+v %v", tok, err)
	}
	_, err = c.SyncDomain(ctx, "ad", nil)
	if !IsNotFound(err) {
		t.Fatalf("expected 404 sync, got %v", err)
	}
}

func TestClientAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "Permission denied"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "x", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Version(context.Background())
	apiErr, ok := IsAPIError(err)
	if !ok || apiErr.Status != 403 {
		t.Fatalf("expected api error, got %v", err)
	}
}

func TestFirewallRulesGroupsIPSetRefs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/firewall/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("action") != "ACCEPT" || r.Form.Get("type") != "in" {
				http.Error(w, "bad rule", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []FWRule{{Pos: 0, Action: "ACCEPT", Type: "in", Enable: 1, Digest: "abc"}},
		})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/rules/0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("digest") != "abc" {
			http.Error(w, "digest", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []FWGroup{{Group: "web"}}})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/groups/web", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []FWRule{{Pos: 0, Action: "ACCEPT", Type: "in", DPort: "80"}},
		})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/ipset", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []FWIPSet{{Name: "office"}}})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/ipset/office", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("cidr") != "10.0.0.0/24" {
				http.Error(w, "cidr", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []FWIPSetCIDR{{CIDR: "10.0.0.0/24"}},
		})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/ipset/office/10.0.0.0%2F24", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/refs", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/firewall/log", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") != "2" {
			http.Error(w, "start", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []FWLogLine{{N: 2, T: "ACCEPT"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/firewall/options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": FWOptions{Enable: intPtr(1), DHCP: intPtr(1), PolicyIn: "DROP"}})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	dc := FWScope{Level: "dc"}
	rules, err := c.FWRules(ctx, dc)
	if err != nil || len(rules) != 1 || rules[0].Digest != "abc" {
		t.Fatalf("rules %+v %v", rules, err)
	}
	if err := c.CreateFWRule(ctx, dc, FWRuleRequest{Action: "ACCEPT", Type: "in"}); err != nil {
		t.Fatalf("create rule %v", err)
	}
	en := 0
	if err := c.UpdateFWRule(ctx, dc, 0, FWRuleRequest{Enable: &en, Digest: "abc"}); err != nil {
		t.Fatalf("update rule %v", err)
	}
	if err := c.CreateFWGroup(ctx, FWGroup{Group: "web"}); err != nil {
		t.Fatalf("create group %v", err)
	}
	gr, err := c.FWGroupRules(ctx, "web")
	if err != nil || len(gr) != 1 {
		t.Fatalf("group rules %+v %v", gr, err)
	}
	if err := c.CreateFWGroupRule(ctx, "web", FWRuleRequest{Action: "ACCEPT", Type: "in"}); err != nil {
		t.Fatalf("group rule %v", err)
	}
	if err := c.CreateFWIPSetCIDR(ctx, dc, "office", FWIPSetCIDR{CIDR: "10.0.0.0/24"}); err != nil {
		t.Fatalf("ipset cidr %v", err)
	}
	if err := c.DeleteFWIPSetCIDR(ctx, dc, "office", "10.0.0.0/24", ""); err != nil {
		t.Fatalf("delete cidr %v", err)
	}
	refs, err := c.FWRefs(ctx, dc)
	if err != nil || refs.Supported {
		t.Fatalf("refs 404 %+v %v", refs, err)
	}
	log, err := c.FWLog(ctx, dc, 2)
	if err != nil || len(log) != 1 || log[0].T != "ACCEPT" {
		t.Fatalf("log %+v %v", log, err)
	}
	guest := FWScope{Level: "guest", Node: "pve1", Kind: "qemu", VMID: 100}
	opts, err := c.FWOptions(ctx, guest)
	if err != nil || opts.Enable == nil || *opts.Enable != 1 || opts.PolicyIn != "DROP" {
		t.Fatalf("guest opts %+v %v", opts, err)
	}
	if err := c.UpdateFWOptions(ctx, guest, FWOptions{Enable: intPtr(1), DHCP: intPtr(1)}); err != nil {
		t.Fatalf("update guest opts %v", err)
	}
}

func intPtr(v int) *int { return &v }

func TestNodeAdminDatacenter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/network", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []NetIface{{Iface: "vmbr0", Type: "bridge"}}})
		case http.MethodPost:
			_ = r.ParseForm()
			if r.Form.Get("iface") != "vmbr1" || r.Form.Get("type") != "bridge" {
				http.Error(w, "iface", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		case http.MethodPut:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:netapply"})
		case http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		default:
			http.Error(w, "method", 405)
		}
	})
	mux.HandleFunc("/api2/json/nodes/pve1/disks/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []DiskInfo{{Devpath: "/dev/sdb", Size: 1000}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/disks/lvmthin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("name") != "data" || r.Form.Get("add_storage") != "1" {
			http.Error(w, "lvmthin", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:lvmthin"})
	})
	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("storage") != "data" || r.Form.Get("type") != "lvmthin" {
			http.Error(w, "storage", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/apt/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:apt"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"Package": "pve-manager"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/apt/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("enabled") != "1" {
				http.Error(w, "enabled", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": APTRepositories{Digest: "d1"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/journal", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") != "3" {
			http.Error(w, "start", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{"boot ok"}})
	})
	mux.HandleFunc("/api2/json/pools", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("poolid") != "lab" {
				http.Error(w, "pool", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []Pool{{PoolID: "lab"}}})
	})
	mux.HandleFunc("/api2/json/pools/lab", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": Pool{PoolID: "lab", Comment: "x"}})
	})
	mux.HandleFunc("/api2/json/cluster/notifications/endpoints/smtp", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/acme/account", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ifaces, err := c.NodeNetwork(ctx, "pve1")
	if err != nil || len(ifaces) != 1 {
		t.Fatalf("network %+v %v", ifaces, err)
	}
	if err := c.CreateNodeNetwork(ctx, "pve1", map[string]any{"iface": "vmbr1", "type": "bridge"}); err != nil {
		t.Fatalf("create net %v", err)
	}
	upid, err := c.ApplyNodeNetwork(ctx, "pve1")
	if err != nil || upid == "" {
		t.Fatalf("apply %q %v", upid, err)
	}
	if err := c.RevertNodeNetwork(ctx, "pve1"); err != nil {
		t.Fatalf("revert %v", err)
	}
	disks, err := c.NodeDisks(ctx, "pve1")
	if err != nil || len(disks) != 1 {
		t.Fatalf("disks %+v %v", disks, err)
	}
	upid, err = c.CreateNodeDiskBackend(ctx, "pve1", "lvmthin", map[string]any{"name": "data", "device": "/dev/sdb"})
	if err != nil || upid == "" {
		t.Fatalf("lvmthin %q %v", upid, err)
	}
	if err := c.CreateStorage(ctx, map[string]any{"storage": "data", "type": "lvmthin"}); err != nil {
		t.Fatalf("storage %v", err)
	}
	upid, err = c.APTUpdate(ctx, "pve1")
	if err != nil || upid == "" {
		t.Fatalf("apt %q %v", upid, err)
	}
	if err := c.UpdateAPTRepository(ctx, "pve1", map[string]any{"enabled": 1, "path": "/etc/apt/sources.list", "index": 0}); err != nil {
		t.Fatalf("repo %v", err)
	}
	lines, err := c.NodeJournal(ctx, "pve1", 3, 0)
	if err != nil || len(lines) != 1 || lines[0].T != "boot ok" {
		t.Fatalf("journal %+v %v", lines, err)
	}
	if err := c.CreatePool(ctx, "lab", "x"); err != nil {
		t.Fatalf("create pool %v", err)
	}
	pools, err := c.Pools(ctx)
	if err != nil || len(pools) != 1 {
		t.Fatalf("pools %+v %v", pools, err)
	}
	if err := c.UpdatePool(ctx, "lab", map[string]any{"comment": "y"}); err != nil {
		t.Fatalf("update pool %v", err)
	}
	if err := c.DeletePool(ctx, "lab"); err != nil {
		t.Fatalf("delete pool %v", err)
	}
	notes, err := c.NotificationEndpoints(ctx, "smtp")
	if err != nil || notes.Supported {
		t.Fatalf("notify 404 %+v %v", notes, err)
	}
	acme, err := c.ACMEAccounts(ctx)
	if err != nil || acme.Supported {
		t.Fatalf("acme 404 %+v %v", acme, err)
	}
}

func TestSDNZonesVNetsApplyFirewall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/sdn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:sdnapply"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "sdn"}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/zones", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("zone") != "z1" || r.Form.Get("type") != "simple" {
				http.Error(w, "zone", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []SDNZone{{Zone: "z1", Type: "simple"}}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("vnet") != "vnet0" || r.Form.Get("zone") != "z1" {
				http.Error(w, "vnet", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []SDNVNet{{VNet: "vnet0", Zone: "z1"}}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0/subnets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("subnet") != "10.10.0.0/24" {
				http.Error(w, "subnet", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []SDNSubnet{{Subnet: "10.10.0.0/24"}}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0/firewall/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("action") != "ACCEPT" {
				http.Error(w, "rule", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []FWRule{{Pos: 0, Action: "ACCEPT", Digest: "abc"}}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0/firewall/rules/0", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("digest") != "abc" {
			http.Error(w, "digest", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/fabrics", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/sdn", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/sdn/zones", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := c.SDNStatus(ctx)
	if err != nil || !st.Supported {
		t.Fatalf("status %+v %v", st, err)
	}
	if err := c.CreateSDNZone(ctx, map[string]any{"zone": "z1", "type": "simple"}); err != nil {
		t.Fatalf("zone %v", err)
	}
	if err := c.CreateSDNVNet(ctx, map[string]any{"vnet": "vnet0", "zone": "z1"}); err != nil {
		t.Fatalf("vnet %v", err)
	}
	if err := c.CreateSDNSubnet(ctx, "vnet0", map[string]any{"subnet": "10.10.0.0/24"}); err != nil {
		t.Fatalf("subnet %v", err)
	}
	upid, err := c.ApplySDN(ctx, nil)
	if err != nil || upid == "" {
		t.Fatalf("apply %q %v", upid, err)
	}
	scope := FWScope{Level: "vnet", VNet: "vnet0"}
	if err := c.CreateFWRule(ctx, scope, FWRuleRequest{Action: "ACCEPT", Type: "in"}); err != nil {
		t.Fatalf("vnet fw %v", err)
	}
	en := 1
	if err := c.UpdateFWRule(ctx, scope, 0, FWRuleRequest{Enable: &en, Digest: "abc"}); err != nil {
		t.Fatalf("vnet fw digest %v", err)
	}
	fab, err := c.SDNFabrics(ctx)
	if err != nil || fab.Supported {
		t.Fatalf("fabrics 404 %+v %v", fab, err)
	}
	rt, err := c.NodeSDN(ctx, "pve1")
	if err != nil || rt.Supported {
		t.Fatalf("node sdn 404 %+v %v", rt, err)
	}
	fabNodes, err := c.SDNFabricNodes(ctx, "fab1")
	if err != nil || fabNodes.Supported {
		t.Fatalf("fabric nodes 404 %+v %v", fabNodes, err)
	}
}

func TestSDNStatusIndexArray(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/sdn", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "zones"}, {"id": "vnets"}}})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.SDNStatus(context.Background())
	if err != nil || !st.Supported {
		t.Fatalf("status %+v %v", st, err)
	}
	if _, ok := st.Data["index"]; !ok {
		t.Fatalf("expected index wrap %+v", st.Data)
	}
}

func TestSDNStatusUnsupported(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/sdn", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.SDNStatus(context.Background())
	if err != nil || st.Supported {
		t.Fatalf("expected unsupported %+v %v", st, err)
	}
}

func TestCephStatusUnsupported(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/ceph/status", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/status", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.CephStatus(context.Background(), "pve1")
	if err != nil || st.Supported {
		t.Fatalf("expected unsupported %+v %v", st, err)
	}
}

func TestCephInstallPoolOSDFlags(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/ceph/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"health": map[string]any{"status": "HEALTH_OK"}, "fsid": "abc"}})
	})
	mux.HandleFunc("/api2/json/cluster/ceph/flags", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"noout": 0}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/flags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = r.ParseForm()
			if r.Form.Get("noout") != "1" {
				http.Error(w, "flag", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"noout": 0}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/install", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cephinstall"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/init", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("network") != "10.0.0.0/24" {
			http.Error(w, "network", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cephinit"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/pool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("name") != "rbd" || r.Form.Get("add_storages") != "1" {
				http.Error(w, "pool", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:poolcreate"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []CephPool{{PoolName: "rbd", Size: 3}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/0/in", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:osdin"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/0/out", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:osdout"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:osddestroy"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/fs", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := c.CephStatus(ctx, "pve1")
	if err != nil || !st.Supported {
		t.Fatalf("status %+v %v", st, err)
	}
	upid, err := c.InstallCeph(ctx, "pve1", map[string]any{"version": "squid"})
	if err != nil || upid == "" {
		t.Fatalf("install %q %v", upid, err)
	}
	upid, err = c.InitCeph(ctx, "pve1", map[string]any{"network": "10.0.0.0/24"})
	if err != nil || upid == "" {
		t.Fatalf("init %q %v", upid, err)
	}
	upid, err = c.CreateCephPool(ctx, "pve1", map[string]any{"name": "rbd", "add_storages": 1})
	if err != nil || upid == "" {
		t.Fatalf("pool %q %v", upid, err)
	}
	if err := c.UpdateCephFlags(ctx, "pve1", map[string]any{"noout": 1}); err != nil {
		t.Fatalf("flags %v", err)
	}
	upid, err = c.CephOSDIn(ctx, "pve1", 0)
	if err != nil || upid == "" {
		t.Fatalf("osd in %q %v", upid, err)
	}
	upid, err = c.CephOSDOut(ctx, "pve1", 0)
	if err != nil || upid == "" {
		t.Fatalf("osd out %q %v", upid, err)
	}
	upid, err = c.DestroyCephOSD(ctx, "pve1", 0, true)
	if err != nil || upid == "" {
		t.Fatalf("osd destroy %q %v", upid, err)
	}
	fs, err := c.CephFSList(ctx, "pve1")
	if err != nil || fs.Supported {
		t.Fatalf("cephfs 404 %+v %v", fs, err)
	}
}

func TestGuestCPUSpiceSendKeyMonitor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("cpu") != "host" || r.Form.Get("vga") != "qxl" || r.Form.Get("sockets") != "2" {
			http.Error(w, "cpu/vga", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:qmcreate:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("cpu") != "x86-64-v2-AES" {
			http.Error(w, "cpu", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/spiceproxy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": SpiceProxy{Port: "61000", Ticket: "spice-ticket", Password: "spice-ticket"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/sendkey", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("key") != "ctrl-alt-delete" {
			http.Error(w, "key", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/monitor", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("command") != "info status" {
			http.Error(w, "command", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "VM status: running"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tablet := true
	upid, err := c.CreateQEMU(ctx, CreateGuestRequest{
		Node: "pve1", VMID: 100, Name: "win", CPU: "host", Sockets: 2, VGA: "qxl", Tablet: &tablet,
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("create %s %v", upid, err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"cpu": "x86-64-v2-AES"}, nil, ""); err != nil {
		t.Fatalf("update cpu %v", err)
	}
	spice, err := c.SpiceProxy(ctx, "pve1", 100)
	if err != nil || spice.Ticket != "spice-ticket" || spice.Port != "61000" {
		t.Fatalf("spice %+v %v", spice, err)
	}
	if err := c.SendKey(ctx, "pve1", 100, "ctrl-alt-delete"); err != nil {
		t.Fatalf("sendkey %v", err)
	}
	out, err := c.QEMUMonitor(ctx, "pve1", 100, "info status")
	if err != nil || out != "VM status: running" {
		t.Fatalf("monitor %q %v", out, err)
	}
}

func TestCreateQEMUImport(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("vmid") != "200" || r.Form.Get("name") != "esxi-vm" {
			http.Error(w, "identity", 400)
			return
		}
		if r.Form.Get("ostype") != "l26" || r.Form.Get("memory") != "4096" {
			http.Error(w, "create-args", 400)
			return
		}
		if r.Form.Get("scsi0") != "local-lvm:0,import-from=esxi:ha/vm/disk1.vmdk" {
			http.Error(w, "import-from", 400)
			return
		}
		if r.Form.Get("import-working-storage") != "local" {
			http.Error(w, "working", 400)
			return
		}
		if r.Form.Get("net0") != "e1000,bridge=vmbr0" {
			http.Error(w, "net", 400)
			return
		}
		if r.Form.Get("scsihw") != "lsilogic" {
			http.Error(w, "scsihw", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:qmcreate:200:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	upid, err := c.CreateQEMU(context.Background(), CreateGuestRequest{
		Node: "pve1", VMID: 200, Name: "esxi-vm", Memory: 4096, Storage: "local-lvm",
		CreateArgs:           map[string]any{"ostype": "l26", "scsihw": "lsilogic", "memory": 2048},
		ImportFrom:           map[string]string{"scsi0": "esxi:ha/vm/disk1.vmdk"},
		ImportWorkingStorage: "local",
		Nets:                 map[string]string{"net0": "e1000,bridge=vmbr0"},
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("import %s %v", upid, err)
	}
}

func TestCreateQEMUImportRemap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("scsi0") != "local-lvm:0,import-from=esxi:ha/vm/disk1.vmdk" {
			http.Error(w, "scsi0", 400)
			return
		}
		if r.Form.Get("virtio0") != "tank:0,import-from=esxi:ha/vm/disk2.vmdk" {
			http.Error(w, "virtio0", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:qmcreate:201:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	upid, err := c.CreateQEMU(context.Background(), CreateGuestRequest{
		Node: "pve1", VMID: 201, Name: "remap-vm", Storage: "local-lvm",
		ImportFrom: map[string]string{
			"scsi0":   "esxi:ha/vm/disk1.vmdk",
			"virtio0": "esxi:ha/vm/disk2.vmdk",
		},
		ImportDest: map[string]string{"virtio0": "tank"},
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("import remap %s %v", upid, err)
	}
}

func TestRemoteMigrateTargetEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/remote_migrate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		want := "apitoken=PVEAPIToken=root@pam!mig=secret,host=10.0.0.8,fingerprint=AA:BB,port=8006"
		if r.Form.Get("target-endpoint") != want || r.Form.Get("target-storage") != "local-lvm" {
			http.Error(w, "target-endpoint", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:rmig-ep"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	upid, err := c.RemoteMigrateGuest(context.Background(), "pve1", "qemu", 100, RemoteMigrateRequest{
		TargetEndpoint: "apitoken=PVEAPIToken=root@pam!mig=secret,host=10.0.0.8,fingerprint=AA:BB,port=8006",
		TargetStorage:  "local-lvm",
		TargetBridge:   "vmbr0",
	})
	if err != nil || !strings.Contains(upid, "rmig-ep") {
		t.Fatalf("target-endpoint %q %v", upid, err)
	}
}

func TestGuestHardwareConfig(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		switch {
		case r.Form.Get("scsi1") == "local-lvm:8":
		case r.Form.Get("net1") == "virtio,bridge=vmbr0":
		case r.Form.Get("delete") == "net1":
		case r.Form.Get("scsi2") == "unused0":
		default:
			http.Error(w, "fields", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/rescan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:rescan:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/101/rescan", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"scsi1": "local-lvm:8"}, nil, ""); err != nil {
		t.Fatalf("add disk %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"net1": "virtio,bridge=vmbr0"}, nil, ""); err != nil {
		t.Fatalf("add nic %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, nil, []string{"net1"}, ""); err != nil {
		t.Fatalf("delete nic %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"scsi2": "unused0"}, nil, ""); err != nil {
		t.Fatalf("unused attach %v", err)
	}
	res, err := c.RescanDisks(ctx, "pve1", "qemu", 100)
	if err != nil || !res.Supported || ParseUPIDNode(res.UPID) != "pve1" {
		t.Fatalf("rescan %+v %v", res, err)
	}
	missing, err := c.RescanDisks(ctx, "pve1", "qemu", 101)
	if err != nil || missing.Supported {
		t.Fatalf("rescan 404 %+v %v", missing, err)
	}
}

func TestGuestDevicesAndLXC(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/hardware/pci", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "0000:01:00.0", "device_name": "GPU"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/hardware/usb", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"vendid": "1234", "prodid": "5678", "product": "Dongle"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/hardware/pci", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		switch {
		case r.Form.Get("hostpci0") == "0000:01:00,pcie=1":
		case r.Form.Get("usb0") == "host=1234:5678,usb3=1":
		case r.Form.Get("tpmstate0") == "local-lvm:1,version=v2.0":
		case r.Form.Get("efidisk0") == "local-lvm:1,efitype=4m,pre-enrolled-keys=1":
		case r.Form.Get("virtiofs0") == "shared":
		default:
			http.Error(w, "qemu fields", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/lxc/101/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		switch {
		case r.Form.Get("mp0") == "local-lvm:8,mp=/mnt/data":
		case r.Form.Get("features") == "nesting=1,keyctl=1":
		default:
			http.Error(w, "lxc fields", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pci, err := c.NodeHardware(ctx, "pve1", "pci")
	if err != nil || !pci.Supported || len(pci.Items) != 1 {
		t.Fatalf("pci %+v %v", pci, err)
	}
	usb, err := c.NodeHardware(ctx, "pve1", "usb")
	if err != nil || !usb.Supported || len(usb.Items) != 1 {
		t.Fatalf("usb %+v %v", usb, err)
	}
	missing, err := c.NodeHardware(ctx, "pve2", "pci")
	if err != nil || missing.Supported {
		t.Fatalf("pci 404 %+v %v", missing, err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"hostpci0": "0000:01:00,pcie=1"}, nil, ""); err != nil {
		t.Fatalf("hostpci %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"usb0": "host=1234:5678,usb3=1"}, nil, ""); err != nil {
		t.Fatalf("usb %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"tpmstate0": "local-lvm:1,version=v2.0"}, nil, ""); err != nil {
		t.Fatalf("tpm %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"efidisk0": "local-lvm:1,efitype=4m,pre-enrolled-keys=1"}, nil, ""); err != nil {
		t.Fatalf("efi %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "qemu", 100, map[string]any{"virtiofs0": "shared"}, nil, ""); err != nil {
		t.Fatalf("virtiofs %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "lxc", 101, map[string]any{"mp0": "local-lvm:8,mp=/mnt/data"}, nil, ""); err != nil {
		t.Fatalf("mp %v", err)
	}
	if err := c.UpdateGuestConfig(ctx, "pve1", "lxc", 101, map[string]any{"features": "nesting=1,keyctl=1"}, nil, ""); err != nil {
		t.Fatalf("features %v", err)
	}
}

func TestGuestLifecycle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/pending", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Query().Get("digest") != "abc" {
			http.Error(w, "pending", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/feature", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("feature") == "clone" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"hasFeature": 0}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"hasFeature": 1, "nodes": []string{"pve1"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/qemu/100/feature", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/snapshot/pre/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("description") != "before upgrade" {
			http.Error(w, "desc", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/cloudinit/dump", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "user" {
			http.Error(w, "type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "#cloud-config\n"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/cloudinit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cloudinit:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/101/cloudinit", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/agent/get-users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": []any{map[string]any{"user": "root"}}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/agent/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"pid": 9}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/agent/fsfreeze-freeze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "frozen"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/import-metadata", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/query-url-metadata", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"filename": "disk.ova", "mimetype": "application/ovf"}})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.RevertPending(ctx, "pve1", "qemu", 100, "abc"); err != nil {
		t.Fatalf("pending %v", err)
	}
	snap, err := c.GuestFeature(ctx, "pve1", "qemu", "snapshot", 100)
	if err != nil || !snap.Supported || !snap.HasFeature {
		t.Fatalf("feature snap %+v %v", snap, err)
	}
	clone, err := c.GuestFeature(ctx, "pve1", "qemu", "clone", 100)
	if err != nil || !clone.Supported || clone.HasFeature {
		t.Fatalf("feature clone %+v %v", clone, err)
	}
	missing, err := c.GuestFeature(ctx, "pve2", "qemu", "snapshot", 100)
	if err != nil || missing.Supported || !missing.HasFeature {
		t.Fatalf("feature 404 %+v %v", missing, err)
	}
	if err := c.UpdateSnapshot(ctx, "pve1", "qemu", "pre", 100, "before upgrade"); err != nil {
		t.Fatalf("snap config %v", err)
	}
	dump, err := c.CloudInitDump(ctx, "pve1", "qemu", "user", 100)
	if err != nil || !dump.Supported || dump.Dump == "" {
		t.Fatalf("dump %+v %v", dump, err)
	}
	regen, err := c.RegenerateCloudInit(ctx, "pve1", "qemu", 100)
	if err != nil || !regen.Supported {
		t.Fatalf("regen %+v %v", regen, err)
	}
	noRegen, err := c.RegenerateCloudInit(ctx, "pve1", "qemu", 101)
	if err != nil || noRegen.Supported {
		t.Fatalf("regen 404 %+v %v", noRegen, err)
	}
	users, err := c.AgentGet(ctx, "pve1", 100, "get-users")
	if err != nil || users == nil {
		t.Fatalf("users %v %v", users, err)
	}
	if _, err := c.AgentPost(ctx, "pve1", 100, "exec", nil); err != nil {
		t.Fatalf("exec %v", err)
	}
	if _, err := c.AgentPost(ctx, "pve1", 100, "fsfreeze-freeze", nil); err != nil {
		t.Fatalf("freeze %v", err)
	}
	meta, err := c.ImportMetadata(ctx, "pve1", "local", "local:import/disk.ova")
	if err != nil || meta.Supported {
		t.Fatalf("import-metadata 404 %+v %v", meta, err)
	}
	urlMeta, err := c.QueryURLMetadata(ctx, "pve1", "https://example.com/disk.ova")
	if err != nil || !urlMeta.Supported || urlMeta.Data["filename"] != "disk.ova" {
		t.Fatalf("url meta %+v %v", urlMeta, err)
	}
}

func TestStoragePBSAndScan404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("storage") != "pbs1" || r.Form.Get("type") != "pbs" || r.Form.Get("server") != "pbs.local" {
				http.Error(w, "pbs", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []Storage{{Storage: "pbs1", Type: "pbs"}}})
	})
	mux.HandleFunc("/api2/json/storage/pbs1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = r.ParseForm()
			if r.Form.Get("disable") != "1" {
				http.Error(w, "disable", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		http.Error(w, "method", 405)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/nfs", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/cifs", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/iscsi", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.CreateStorage(ctx, map[string]any{"storage": "pbs1", "type": "pbs", "server": "pbs.local", "datastore": "store", "username": "user@pbs"}); err != nil {
		t.Fatalf("create pbs %v", err)
	}
	if err := c.UpdateStorage(ctx, "pbs1", map[string]any{"disable": 1}); err != nil {
		t.Fatalf("update %v", err)
	}
	if err := c.DeleteStorage(ctx, "pbs1"); err != nil {
		t.Fatalf("delete %v", err)
	}
	for _, kind := range []string{"nfs", "cifs", "iscsi", "lvm", "lvmthin", "zfs", "pbs"} {
		scan, err := c.ScanStorage(ctx, "pve1", kind, map[string]any{"server": "nas.local"})
		if err != nil || scan.Supported {
			t.Fatalf("scan %s 404 %+v %v", kind, scan, err)
		}
	}
}

func TestClusterConfigJoinQDevice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("clustername") != "lab" || r.Form.Get("link1") != "192.168.2.10" {
				http.Error(w, "name", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		// Real Proxmox answers GET /cluster/config with a directory index.
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "totem"}, {"name": "nodes"}}})
	})
	mux.HandleFunc("/api2/json/cluster/config/totem", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"cluster_name": "lab", "config_version": "3", "link_mode": "passive",
		}})
	})
	mux.HandleFunc("/api2/json/cluster/config/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("hostname") != "pve2" {
				http.Error(w, "join", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/config/nodes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "pve1"}}})
	})
	mux.HandleFunc("/api2/json/cluster/config/qdevice", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := c.ClusterConfig(ctx)
	if err != nil || cfg["supported"] != true || cfg["cluster_name"] != "lab" || cfg["clustered"] != true {
		t.Fatalf("config %+v %v", cfg, err)
	}
	if totem, _ := cfg["totem"].(map[string]any); totem["link_mode"] != "passive" {
		t.Fatalf("totem %+v", cfg["totem"])
	}
	if err := c.CreateCluster(ctx, map[string]any{"clustername": "lab", "link0": "192.168.1.10", "link1": "192.168.2.10"}); err != nil {
		t.Fatalf("create %v", err)
	}
	join, err := c.ClusterJoinInfo(ctx)
	if err != nil || join["supported"] != false {
		t.Fatalf("join info 404 %+v %v", join, err)
	}
	if err := c.JoinCluster(ctx, map[string]any{"hostname": "pve2", "fingerprint": "AA:BB", "password": "x"}); err != nil {
		t.Fatalf("join %v", err)
	}
	nodes, err := c.ClusterConfigNodes(ctx)
	if err != nil || !nodes.Supported || len(nodes.Items) != 1 {
		t.Fatalf("nodes %+v %v", nodes, err)
	}
	qdev, err := c.ClusterQDevice(ctx)
	if err != nil || qdev.Supported {
		t.Fatalf("qdevice 404 %+v %v", qdev, err)
	}
}

func TestACMENotifyUpdateForm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/acme/account/default", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("contact") != "ops@example.com" {
			http.Error(w, "contact", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:acme:root@pam:"})
	})
	mux.HandleFunc("/api2/json/cluster/acme/plugins/dns-cf", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("api") != "cf" {
			http.Error(w, "api", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/notifications/endpoints/gotify/ops", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("server") != "https://gotify.local" || r.Form.Get("token") != "secret" {
			http.Error(w, "gotify", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/notifications/matchers/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("target") != "ops" || r.Form.Get("match-severity") != "error,warning" {
			http.Error(w, "target", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	upid, err := c.UpdateACMEAccount(ctx, "default", map[string]any{"contact": "ops@example.com"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("acme account %s %v", upid, err)
	}
	if err := c.UpdateACMEPlugin(ctx, "dns-cf", map[string]any{"api": "cf"}); err != nil {
		t.Fatalf("acme plugin %v", err)
	}
	if err := c.UpdateNotificationEndpoint(ctx, "gotify", "ops", map[string]any{"server": "https://gotify.local", "token": "secret"}); err != nil {
		t.Fatalf("notify %v", err)
	}
	if err := c.UpdateNotificationMatcher(ctx, "all", map[string]any{"target": "ops", "match-severity": "error,warning"}); err != nil {
		t.Fatalf("matcher %v", err)
	}
}

func TestNodeLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/report", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "pve-report\ncpu: ok"})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/report", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/wakeonlan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "aa:bb:cc:dd:ee:ff"})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/wakeonlan", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = r.ParseForm()
			if r.Form.Get("description") != "lab node" {
				http.Error(w, "description", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"description": "lab node", "digest": "d1"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/disks/initgpt", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("disk") != "/dev/sdb" {
			http.Error(w, "disk", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:initgpt:sdb:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/disks/zfs/rpool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:zfsscrub:rpool:root@pam:"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"name": "rpool", "state": "ONLINE", "scan": "none requested"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/disks/zfs/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("name") != "tank" {
			http.Error(w, "name", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:zfsimport:tank:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/disks/zfs/rpool", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve2/disks/zfs/import", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/lvm", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"vg": "pve"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/zfs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"pool": "rpool"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/scan/pbs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"store": "store1"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/scan/lvm", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/startall", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:startall::root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/migrateall", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("target") != "pve2" {
			http.Error(w, "target", 400)
			return
		}
		if r.Form.Get("targetstorage") != "" && (r.Form.Get("targetstorage") != "local-lvm" || r.Form.Get("migration_network") != "10.0.0.0/24") {
			http.Error(w, "migrateall extras", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:migrateall::root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/aplinfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"package": "turnkey-core", "location": "http://example.com/core.tar.gz"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/aplinfo", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rep, err := c.NodeReport(ctx, "pve1")
	if err != nil || !rep.Supported || rep.Report == "" {
		t.Fatalf("report %+v %v", rep, err)
	}
	missing, err := c.NodeReport(ctx, "pve2")
	if err != nil || missing.Supported {
		t.Fatalf("report 404 %+v %v", missing, err)
	}
	wol, err := c.WakeOnLAN(ctx, "pve1")
	if err != nil || !wol.Supported || wol.MAC == "" {
		t.Fatalf("wol %+v %v", wol, err)
	}
	wolMiss, err := c.WakeOnLAN(ctx, "pve2")
	if err != nil || wolMiss.Supported {
		t.Fatalf("wol 404 %+v %v", wolMiss, err)
	}
	cfg, err := c.NodeConfig(ctx, "pve1")
	if err != nil || cfg["description"] != "lab node" {
		t.Fatalf("config %+v %v", cfg, err)
	}
	if err := c.UpdateNodeConfig(ctx, "pve1", map[string]any{"description": "lab node"}); err != nil {
		t.Fatalf("config put %v", err)
	}
	upid, err := c.InitGPT(ctx, "pve1", "/dev/sdb")
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("initgpt %s %v", upid, err)
	}
	st, err := c.ZFSStatus(ctx, "pve1", "rpool")
	if err != nil || !st.Supported || st.Pool["state"] != "ONLINE" {
		t.Fatalf("zfs status %+v %v", st, err)
	}
	scrub, err := c.ZFSScrub(ctx, "pve1", "rpool")
	if err != nil || !scrub.Supported || ParseUPIDNode(scrub.UPID) != "pve1" {
		t.Fatalf("zfs scrub %+v %v", scrub, err)
	}
	imp, err := c.ZFSImport(ctx, "pve1", map[string]any{"name": "tank"})
	if err != nil || !imp.Supported || ParseUPIDNode(imp.UPID) != "pve1" {
		t.Fatalf("zfs import %+v %v", imp, err)
	}
	scrubMiss, err := c.ZFSScrub(ctx, "pve2", "rpool")
	if err != nil || scrubMiss.Supported {
		t.Fatalf("zfs scrub 404 %+v %v", scrubMiss, err)
	}
	for _, kind := range []string{"lvm", "zfs", "pbs"} {
		scan, err := c.ScanStorage(ctx, "pve1", kind, nil)
		if err != nil || !scan.Supported || len(scan.Items) == 0 {
			t.Fatalf("scan %s %+v %v", kind, scan, err)
		}
	}
	scan404, err := c.ScanStorage(ctx, "pve2", "lvm", nil)
	if err != nil || scan404.Supported {
		t.Fatalf("scan lvm 404 %+v %v", scan404, err)
	}
	upid, err = c.NodeGuestAll(ctx, "pve1", "startall", nil)
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("startall %s %v", upid, err)
	}
	upid, err = c.NodeGuestAll(ctx, "pve1", "migrateall", map[string]any{"target": "pve2"})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("migrateall %s %v", upid, err)
	}
	upid, err = c.NodeGuestAll(ctx, "pve1", "migrateall", map[string]any{
		"target": "pve2", "targetstorage": "local-lvm", "migration_network": "10.0.0.0/24",
	})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("migrateall extras %s %v", upid, err)
	}
	apl, err := c.APLInfo(ctx, "pve1")
	if err != nil || !apl.Supported || len(apl.Items) != 1 {
		t.Fatalf("aplinfo %+v %v", apl, err)
	}
	aplMiss, err := c.APLInfo(ctx, "pve2")
	if err != nil || aplMiss.Supported {
		t.Fatalf("aplinfo 404 %+v %v", aplMiss, err)
	}
}

func TestDatacenterLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/metrics/server", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("id") != "influx1" || r.Form.Get("type") != "influxdb" {
				http.Error(w, "metrics create", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "influx1", "type": "influxdb"}}})
	})
	mux.HandleFunc("/api2/json/cluster/metrics/server/influx1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = r.ParseForm()
			if r.Form.Get("server") != "metrics.local" {
				http.Error(w, "metrics put", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		http.Error(w, "method", 405)
	})
	mux.HandleFunc("/api2/json/cluster/log", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"uid": 1, "msg": "node joined"}}})
	})
	mux.HandleFunc("/api2/json/cluster/jobs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"type": "realm-sync"}}})
	})
	mux.HandleFunc("/api2/json/cluster/jobs/realm-sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("id") != "ldap-nightly" || r.Form.Get("schedule") != "0 3 * * *" {
				http.Error(w, "job", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ldap-nightly", "schedule": "0 3 * * *"}}})
	})
	mux.HandleFunc("/api2/json/cluster/jobs/realm-sync/ldap-nightly", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("schedule") != "0 4 * * *" {
			http.Error(w, "realm put", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/jobs/schedule-analyze", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/acme/directories", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "Let's Encrypt V2", "url": "https://acme-v02.api.letsencrypt.org/directory"}}})
	})
	mux.HandleFunc("/api2/json/cluster/acme/meta", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"termsOfService": "https://tos.example"}})
	})
	mux.HandleFunc("/api2/json/cluster/acme/tos", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "https://tos.example"})
	})
	mux.HandleFunc("/api2/json/cluster/mapping/pci", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("id") != "gpu0" || len(r.Form["map"]) != 2 || r.Form["map"][0] != "pve1:0000:01:00.0" {
				http.Error(w, "map", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpu0"}}})
	})
	mux.HandleFunc("/api2/json/cluster/notifications/endpoints/sendmail", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("name") != "ops-mail" {
				http.Error(w, "sendmail", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "ops-mail"}}})
	})
	mux.HandleFunc("/api2/json/cluster/notifications/targets/ops-mail/test", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/qemu/custom-cpu-models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("name") != "labcpu" {
				http.Error(w, "cpu", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "labcpu"}}})
	})
	mux.HandleFunc("/api2/json/cluster/qemu/custom-cpu-models/labcpu", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("flags") != "+aes" {
			http.Error(w, "cpu put", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/vzdump/defaults", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"mode": "snapshot", "bwlimit": 0, "fleecing": 0}})
	})
	mux.HandleFunc("/api2/json/access/domains", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("realm") != "oidc" || r.Form.Get("type") != "openid" {
			http.Error(w, "openid", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/access/users/ops@pve/unlock-tfa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/access/openid", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ms, err := c.MetricServers(ctx)
	if err != nil || !ms.Supported || len(ms.Items) != 1 {
		t.Fatalf("metrics %+v %v", ms, err)
	}
	if err := c.CreateMetricServer(ctx, map[string]any{"id": "influx1", "type": "influxdb"}); err != nil {
		t.Fatalf("create metrics %v", err)
	}
	if err := c.UpdateMetricServer(ctx, "influx1", map[string]any{"server": "metrics.local"}); err != nil {
		t.Fatalf("update metrics %v", err)
	}
	if err := c.DeleteMetricServer(ctx, "influx1"); err != nil {
		t.Fatalf("delete metrics %v", err)
	}
	log, err := c.ClusterLog(ctx, 20)
	if err != nil || !log.Supported || len(log.Items) != 1 {
		t.Fatalf("log %+v %v", log, err)
	}
	if err := c.CreateRealmSyncJob(ctx, map[string]any{"id": "ldap-nightly", "schedule": "0 3 * * *"}); err != nil {
		t.Fatalf("realm-sync %v", err)
	}
	if err := c.UpdateRealmSyncJob(ctx, "ldap-nightly", map[string]any{"schedule": "0 4 * * *"}); err != nil {
		t.Fatalf("realm-sync put %v", err)
	}
	analyze, err := c.ScheduleAnalyze(ctx, "0 3 * * *")
	if err != nil || analyze.Supported {
		t.Fatalf("analyze 404 %+v %v", analyze, err)
	}
	dirs, err := c.ACMEDirectories(ctx)
	if err != nil || !dirs.Supported || len(dirs.Items) != 1 {
		t.Fatalf("acme dirs %+v %v", dirs, err)
	}
	meta, err := c.ACMEMeta(ctx, "")
	if err != nil || !meta.Supported {
		t.Fatalf("acme meta %+v %v", meta, err)
	}
	tos, err := c.ACMETOS(ctx, "")
	if err != nil || !tos.Supported || tos.Value != "https://tos.example" {
		t.Fatalf("acme tos %+v %v", tos, err)
	}
	if err := c.CreateMapping(ctx, "pci", map[string]any{"id": "gpu0", "map": []string{"pve1:0000:01:00.0", "pve2:0000:02:00.0"}}); err != nil {
		t.Fatalf("mapping %v", err)
	}
	if err := c.CreateNotificationEndpoint(ctx, "sendmail", map[string]any{"name": "ops-mail"}); err != nil {
		t.Fatalf("sendmail %v", err)
	}
	test, err := c.TestNotificationTarget(ctx, "ops-mail")
	if err != nil || test.Supported {
		t.Fatalf("target test 404 %+v %v", test, err)
	}
	if err := c.CreateCustomCPUModel(ctx, map[string]any{"name": "labcpu"}); err != nil {
		t.Fatalf("cpu %v", err)
	}
	if err := c.UpdateCustomCPUModel(ctx, "labcpu", map[string]any{"flags": "+aes"}); err != nil {
		t.Fatalf("cpu put %v", err)
	}
	defs, err := c.VZDumpDefaults(ctx, "pve1", "")
	if err != nil || !defs.Supported || defs.Item["mode"] != "snapshot" {
		t.Fatalf("vzdump %+v %v", defs, err)
	}
	write, err := c.UpdateVZDumpDefaults(ctx, "pve1", map[string]any{"mode": "snapshot"})
	if err != nil || write.Supported {
		t.Fatalf("vzdump put 404 %+v %v", write, err)
	}
	if err := c.CreateDomain(ctx, map[string]any{"realm": "oidc", "type": "openid", "issuer-url": "https://idp.local"}); err != nil {
		t.Fatalf("openid %v", err)
	}
	if err := c.UnlockTFA(ctx, "ops@pve"); err != nil {
		t.Fatalf("unlock %v", err)
	}
	oidc, err := c.OpenID(ctx)
	if err != nil || oidc.Supported {
		t.Fatalf("openid 404 %+v %v", oidc, err)
	}
}

func TestStorageLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		switch r.Form.Get("type") {
		case "rbd":
			if r.Form.Get("storage") != "rbd1" || r.Form.Get("pool") != "rbd" {
				http.Error(w, "rbd", 400)
				return
			}
		case "cephfs":
			if r.Form.Get("storage") != "cephfs1" || r.Form.Get("monhost") != "10.0.0.1" {
				http.Error(w, "cephfs", 400)
				return
			}
		case "zfs":
			if r.Form.Get("storage") != "ziscsi" || r.Form.Get("portal") != "10.0.0.8" || r.Form.Get("target") != "iqn.lab" {
				http.Error(w, "zfs", 400)
				return
			}
		default:
			http.Error(w, "type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/storage/rbd1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"storage": "rbd1", "type": "rbd", "pool": "rbd"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/content/disk.iso", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("notes") != "lab iso" {
			http.Error(w, "notes", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/content/missing", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/oci-registry-pull", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/pbs1/identity", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = r.ParseForm()
			if r.Form.Get("fingerprint") != "AA:BB" {
				http.Error(w, "fp", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"fingerprint": "AA:BB", "crypt-mode": "none"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/identity", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.CreateStorage(ctx, map[string]any{"storage": "rbd1", "type": "rbd", "pool": "rbd", "monhost": "10.0.0.1"}); err != nil {
		t.Fatalf("rbd %v", err)
	}
	if err := c.CreateStorage(ctx, map[string]any{"storage": "cephfs1", "type": "cephfs", "monhost": "10.0.0.1"}); err != nil {
		t.Fatalf("cephfs %v", err)
	}
	if err := c.CreateStorage(ctx, map[string]any{"storage": "ziscsi", "type": "zfs", "portal": "10.0.0.8", "target": "iqn.lab", "pool": "tank"}); err != nil {
		t.Fatalf("zfs-iscsi %v", err)
	}
	cfg, err := c.Storage(ctx, "rbd1")
	if err != nil || cfg["type"] != "rbd" {
		t.Fatalf("get storage %+v %v", cfg, err)
	}
	notes, err := c.UpdateVolumeNotes(ctx, "pve1", "local", "disk.iso", "lab iso")
	if err != nil || !notes.Supported {
		t.Fatalf("notes %+v %v", notes, err)
	}
	missing, err := c.UpdateVolumeNotes(ctx, "pve1", "local", "missing", "x")
	if err != nil || missing.Supported {
		t.Fatalf("notes 404 %+v %v", missing, err)
	}
	oci, err := c.OCIPull(ctx, "pve1", "local", map[string]any{"url": "oci://ghcr.io/lab/img:latest"})
	if err != nil || oci.Supported {
		t.Fatalf("oci 404 %+v %v", oci, err)
	}
	ident, err := c.StorageIdentity(ctx, "pve1", "pbs1")
	if err != nil || !ident.Supported || ident.Item["fingerprint"] != "AA:BB" {
		t.Fatalf("identity %+v %v", ident, err)
	}
	if err := func() error {
		res, err := c.UpdateStorageIdentity(ctx, "pve1", "pbs1", map[string]any{"fingerprint": "AA:BB"})
		if err != nil || !res.Supported {
			return err
		}
		return nil
	}(); err != nil {
		t.Fatalf("identity put %v", err)
	}
	gone, err := c.StorageIdentity(ctx, "pve1", "local")
	if err != nil || gone.Supported {
		t.Fatalf("identity 404 %+v %v", gone, err)
	}
}

func TestCephLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/ceph/metadata", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"mgr": map[string]any{"pve1": map[string]any{"name": "pve1"}}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cephstart"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cephstop"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/cmd-safety", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != "osd" || r.URL.Query().Get("id") != "0" {
			http.Error(w, "query", 400)
			return
		}
		safe := r.URL.Query().Get("action") == "stop"
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"safe": safe, "status": "ok-to-stop"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/0/metadata", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"osd": 0, "hostname": "pve1"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/0/lv-info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"lv_name": "osd-0"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/osd/9/metadata", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/pool/rbd/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"pg_num": 128}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/pool/missing/status", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/radosgw", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/pool/rbd", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("crush_rule") != "replicated_rule" {
			http.Error(w, "crush", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	meta, err := c.CephMetadata(ctx, "pve1")
	if err != nil || !meta.Supported {
		t.Fatalf("metadata %+v %v", meta, err)
	}
	upid, err := c.StartCeph(ctx, "pve1", "")
	if err != nil || upid == "" {
		t.Fatalf("start %q %v", upid, err)
	}
	upid, err = c.StopCeph(ctx, "pve1", "osd.0")
	if err != nil || upid == "" {
		t.Fatalf("stop %q %v", upid, err)
	}
	safe, err := c.CephCmdSafety(ctx, "pve1", "osd", "0", "stop")
	if err != nil || !safe.Supported || !safe.Safe {
		t.Fatalf("safety stop %+v %v", safe, err)
	}
	unsafe, err := c.CephCmdSafety(ctx, "pve1", "osd", "0", "destroy")
	if err != nil || !unsafe.Supported || unsafe.Safe {
		t.Fatalf("safety destroy %+v %v", unsafe, err)
	}
	osd, err := c.CephOSDMetadata(ctx, "pve1", 0)
	if err != nil || !osd.Supported || osd.Item["hostname"] != "pve1" {
		t.Fatalf("osd meta %+v %v", osd, err)
	}
	lv, err := c.CephOSDLVInfo(ctx, "pve1", 0)
	if err != nil || !lv.Supported {
		t.Fatalf("lv %+v %v", lv, err)
	}
	missingOSD, err := c.CephOSDMetadata(ctx, "pve1", 9)
	if err != nil || missingOSD.Supported {
		t.Fatalf("osd 404 %+v %v", missingOSD, err)
	}
	pool, err := c.CephPoolStatus(ctx, "pve1", "rbd")
	if err != nil || !pool.Supported {
		t.Fatalf("pool status %+v %v", pool, err)
	}
	missingPool, err := c.CephPoolStatus(ctx, "pve1", "missing")
	if err != nil || missingPool.Supported {
		t.Fatalf("pool 404 %+v %v", missingPool, err)
	}
	rgw, err := c.CephRGW(ctx, "pve1")
	if err != nil || rgw.Supported {
		t.Fatalf("rgw 404 %+v %v", rgw, err)
	}
	if err := c.UpdateCephPool(ctx, "pve1", "rbd", map[string]any{"crush_rule": "replicated_rule"}); err != nil {
		t.Fatalf("crush_rule %v", err)
	}
}

func TestSDNLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/sdn/dry-run", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("node") != "pve1" {
			http.Error(w, "node", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"frr-diff": "+bgp", "interfaces-diff": "+vmbr0"}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/lock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "lock-secret"})
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		http.Error(w, "method", 405)
	})
	mux.HandleFunc("/api2/json/cluster/sdn/ipams/pve/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "pve", "free": 10}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/ipams/gone/status", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/sdn/fabrics/fab1/interfaces", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "eth0", "state": "up", "type": "Point-to-Point"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/sdn/fabrics/missing/interfaces", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0/subnets/10.10.0.0%2F24", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("gateway") != "10.10.0.1" {
			http.Error(w, "subnet put", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("alias") != "lab" || r.Form.Get("tag") != "20" || r.Form.Get("vlanaware") != "1" {
			http.Error(w, "vnet put", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/vnets/vnet0/ips", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.Form.Get("ip") != "10.10.0.5" {
				http.Error(w, "ip", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/cluster/sdn/missing-lock", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	dry, err := c.SDNDryRun(ctx, "pve1")
	if err != nil || !dry.Supported || dry.Item["frr-diff"] != "+bgp" {
		t.Fatalf("dry-run %+v %v", dry, err)
	}
	lock, err := c.LockSDN(ctx, false)
	if err != nil || !lock.Supported || lock.Value != "lock-secret" {
		t.Fatalf("lock %+v %v", lock, err)
	}
	unlock, err := c.UnlockSDN(ctx, "lock-secret", false)
	if err != nil || !unlock.Supported {
		t.Fatalf("unlock %+v %v", unlock, err)
	}
	ipam, err := c.SDNIPAMStatus(ctx, "pve")
	if err != nil || !ipam.Supported {
		t.Fatalf("ipam %+v %v", ipam, err)
	}
	gone, err := c.SDNIPAMStatus(ctx, "gone")
	if err != nil || gone.Supported {
		t.Fatalf("ipam 404 %+v %v", gone, err)
	}
	ifaces, err := c.SDNFabricInterfaces(ctx, "pve1", "fab1")
	if err != nil || !ifaces.Supported || len(ifaces.Items) != 1 {
		t.Fatalf("ifaces %+v %v", ifaces, err)
	}
	missingIf, err := c.SDNFabricInterfaces(ctx, "pve1", "missing")
	if err != nil || missingIf.Supported {
		t.Fatalf("ifaces 404 %+v %v", missingIf, err)
	}
	if err := c.UpdateSDNVNet(ctx, "vnet0", map[string]any{"alias": "lab", "tag": 20, "vlanaware": 1}); err != nil {
		t.Fatalf("vnet put %v", err)
	}
	if err := c.UpdateSDNSubnet(ctx, "vnet0", "10.10.0.0/24", map[string]any{"gateway": "10.10.0.1"}); err != nil {
		t.Fatalf("subnet put %v", err)
	}
	created, err := c.CreateSDNVNetIP(ctx, "vnet0", map[string]any{"ip": "10.10.0.5", "mac": "AA:BB:CC:DD:EE:FF"})
	if err != nil || !created.Supported {
		t.Fatalf("ip create %+v %v", created, err)
	}
	deleted, err := c.DeleteSDNVNetIP(ctx, "vnet0", map[string]any{"ip": "10.10.0.5"})
	if err != nil || !deleted.Supported {
		t.Fatalf("ip delete %+v %v", deleted, err)
	}

	mux404 := http.NewServeMux()
	mux404.HandleFunc("/api2/json/cluster/sdn/dry-run", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux404.HandleFunc("/api2/json/cluster/sdn/lock", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv404 := httptest.NewTLSServer(mux404)
	defer srv404.Close()
	c404, err := New(Options{BaseURL: srv404.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv404.Client()})
	if err != nil {
		t.Fatal(err)
	}
	dry404, err := c404.SDNDryRun(ctx, "pve1")
	if err != nil || dry404.Supported {
		t.Fatalf("dry-run 404 %+v %v", dry404, err)
	}
	lock404, err := c404.LockSDN(ctx, false)
	if err != nil || lock404.Supported {
		t.Fatalf("lock 404 %+v %v", lock404, err)
	}
	unlock404, err := c404.UnlockSDN(ctx, "", true)
	if err != nil || unlock404.Supported {
		t.Fatalf("unlock 404 %+v %v", unlock404, err)
	}
}

func TestStockButtons(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		q := r.URL.Query()
		if q.Get("purge") != "1" || q.Get("destroy-unreferenced-disks") != "1" {
			http.Error(w, "delete flags", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:qmdestroy:100:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/lxc/101", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Query().Get("force") != "1" {
			http.Error(w, "lxc force", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:vzdestroy:101:root@pam:"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/tasks/UPID:pve1:000:qmstart:100:root@pam:", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/ha/status/arm-ha", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/ha/status/disarm-ha", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("resource-mode") != "freeze" {
			http.Error(w, "mode", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/route-maps", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "rm1"}}})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("id") != "rm1" {
			http.Error(w, "id", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/route-maps/rm1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/route-maps/entries/rm1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"order": 10, "action": "permit"}}})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/route-maps/entries/rm1/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("action") != "permit" {
			http.Error(w, "action", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/route-maps/entries/rm1/entry/10", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/prefix-lists", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "pl1"}}})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("id") != "pl1" {
			http.Error(w, "id", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/prefix-lists/pl1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/prefix-lists/pl1/entries", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"seq": 10, "action": "permit", "prefix": "10.0.0.0/8"}}})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("prefix") != "10.0.0.0/8" {
			http.Error(w, "prefix", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/prefix-lists/pl1/entries/10", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/releases", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "squid", "version": "19.2"}}})
	})
	mux.HandleFunc("/api2/json/cluster/ceph/restart-bulk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("service-type") != "osd" || r.Form.Get("dry-run") != "1" {
			http.Error(w, "bulk", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:cephbulk"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/restart-bulk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("resume") != "1" {
			http.Error(w, "resume", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:osdbulk"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("commands") == "" {
			http.Error(w, "commands", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"status": 200}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/vncshell", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": VNCProxy{Port: "5900", Ticket: "vnc-ticket"}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/spiceshell", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": SpiceProxy{Port: "61000", Ticket: "spice-ticket"}})
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	upid, err := c.DeleteGuest(ctx, "pve1", "qemu", 100, DeleteGuestOptions{Purge: true, DestroyUnreferencedDisks: true})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("delete qemu %q %v", upid, err)
	}
	upid, err = c.DeleteGuest(ctx, "pve1", "lxc", 101, DeleteGuestOptions{Force: true})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("delete lxc %q %v", upid, err)
	}
	if err := c.StopTask(ctx, "pve1", "UPID:pve1:000:qmstart:100:root@pam:"); err != nil {
		t.Fatalf("stop task %v", err)
	}
	if err := c.ArmHA(ctx); err != nil {
		t.Fatalf("arm %v", err)
	}
	if err := c.DisarmHA(ctx, "freeze"); err != nil {
		t.Fatalf("disarm %v", err)
	}
	maps, err := c.SDNRouteMaps(ctx)
	if err != nil || !maps.Supported || len(maps.Items) != 1 {
		t.Fatalf("route maps %+v %v", maps, err)
	}
	if err := c.CreateSDNRouteMap(ctx, map[string]any{"id": "rm1"}); err != nil {
		t.Fatalf("create map %v", err)
	}
	if err := c.CreateSDNRouteMapEntry(ctx, "rm1", map[string]any{"action": "permit", "order": 10}); err != nil {
		t.Fatalf("create map entry %v", err)
	}
	if err := c.DeleteSDNRouteMapEntry(ctx, "rm1", "10"); err != nil {
		t.Fatalf("delete map entry %v", err)
	}
	if err := c.DeleteSDNRouteMap(ctx, "rm1"); err != nil {
		t.Fatalf("delete map %v", err)
	}
	if err := c.CreateSDNPrefixList(ctx, map[string]any{"id": "pl1"}); err != nil {
		t.Fatalf("create prefix %v", err)
	}
	if err := c.CreateSDNPrefixListEntry(ctx, "pl1", map[string]any{"action": "permit", "prefix": "10.0.0.0/8", "seq": 10}); err != nil {
		t.Fatalf("create prefix entry %v", err)
	}
	if err := c.DeleteSDNPrefixListEntry(ctx, "pl1", "10"); err != nil {
		t.Fatalf("delete prefix entry %v", err)
	}
	if err := c.DeleteSDNPrefixList(ctx, "pl1"); err != nil {
		t.Fatalf("delete prefix %v", err)
	}
	rels, err := c.CephReleases(ctx, "pve1")
	if err != nil || !rels.Supported || len(rels.Items) != 1 {
		t.Fatalf("releases %+v %v", rels, err)
	}
	upid, err = c.CephRestartBulk(ctx, map[string]any{"service-type": "osd", "dry-run": 1})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("cluster bulk %q %v", upid, err)
	}
	upid, err = c.NodeCephRestartBulk(ctx, "pve1", map[string]any{"resume": 1})
	if err != nil || ParseUPIDNode(upid) != "pve1" {
		t.Fatalf("node bulk %q %v", upid, err)
	}
	out, err := c.NodeExecute(ctx, "pve1", []map[string]any{{"method": "GET", "path": "/version"}})
	if err != nil || len(out) != 1 {
		t.Fatalf("execute %+v %v", out, err)
	}
	vnc, err := c.NodeVNCShell(ctx, "pve1")
	if err != nil || vnc.Ticket != "vnc-ticket" {
		t.Fatalf("vncshell %+v %v", vnc, err)
	}
	spice, err := c.NodeSpiceShell(ctx, "pve1")
	if err != nil || spice.Ticket != "spice-ticket" {
		t.Fatalf("spiceshell %+v %v", spice, err)
	}

	mux404 := http.NewServeMux()
	mux404.HandleFunc("/api2/json/nodes/pve1/ceph/releases", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux404.HandleFunc("/api2/json/cluster/sdn/route-maps", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux404.HandleFunc("/api2/json/cluster/sdn/prefix-lists", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv404 := httptest.NewTLSServer(mux404)
	defer srv404.Close()
	c404, err := New(Options{BaseURL: srv404.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv404.Client()})
	if err != nil {
		t.Fatal(err)
	}
	gone, err := c404.CephReleases(ctx, "pve1")
	if err != nil || gone.Supported {
		t.Fatalf("releases 404 %+v %v", gone, err)
	}
	maps404, err := c404.SDNRouteMaps(ctx)
	if err != nil || maps404.Supported {
		t.Fatalf("maps 404 %+v %v", maps404, err)
	}
	pfx404, err := c404.SDNPrefixLists(ctx)
	if err != nil || pfx404.Supported {
		t.Fatalf("prefix 404 %+v %v", pfx404, err)
	}
}

func TestTypedWizards(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/replication/100-0", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			_ = r.ParseForm()
			if r.Form.Get("remove_job") != "full" && !(r.Form.Get("rate") == "10" && r.Form.Get("disable") == "1") {
				http.Error(w, "remove_job", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		case http.MethodDelete:
			q := r.URL.Query()
			if q.Get("force") != "1" || q.Get("keep") != "1" {
				http.Error(w, "delete flags", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		default:
			http.Error(w, "method", 405)
		}
	})
	mux.HandleFunc("/api2/json/cluster/ha/resources/vm:100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Query().Get("purge") != "1" {
			http.Error(w, "purge", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/fabrics/fab1/nodes/pve1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("ip") != "10.0.0.1" {
			http.Error(w, "ip", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	disable := true
	if err := c.UpdateReplication(ctx, "100-0", ReplicationRequest{Rate: "10", Disable: &disable}); err != nil {
		t.Fatalf("repl edit %v", err)
	}
	if err := c.UpdateReplication(ctx, "100-0", ReplicationRequest{RemoveJob: "full"}); err != nil {
		t.Fatalf("remove_job %v", err)
	}
	if err := c.DeleteReplication(ctx, "100-0", DeleteReplicationOptions{Force: true, Keep: true}); err != nil {
		t.Fatalf("delete replication %v", err)
	}
	if err := c.DeleteHAResource(ctx, "vm:100", true); err != nil {
		t.Fatalf("purge ha %v", err)
	}
	if err := c.UpdateSDNFabricNode(ctx, "fab1", "pve1", map[string]any{"ip": "10.0.0.1"}); err != nil {
		t.Fatalf("fabric node put %v", err)
	}
}

func TestStock113Writes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/mapping/pci/gpu0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("map") == "" && len(r.Form["map"]) == 0 {
			http.Error(w, "map", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/ha/rules/keep-together", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("disable") != "1" {
			http.Error(w, "disable", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/access/groups/ops", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("comment") != "on-call" {
			http.Error(w, "comment", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/firewall/aliases/office", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("cidr") != "10.0.0.0/24" {
			http.Error(w, "cidr", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/remote_migrate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("remote") != "site-b" || r.Form.Get("target-vmid") != "200" || r.Form.Get("target-storage") != "ceph" || r.Form.Get("target-bridge") != "vmbr0" {
			http.Error(w, "remote_migrate", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:000:rmig"})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/remotes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("remote") != "site-b" || r.Form.Get("server") != "10.0.0.8" {
			http.Error(w, "remote create", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/sdn/remotes/site-b", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Method {
		case http.MethodPut:
			if r.Form.Get("server") != "10.0.0.9" {
				http.Error(w, "remote put", 400)
				return
			}
		case http.MethodDelete:
		default:
			http.Error(w, "method", 405)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/crush", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("crushmap") == "" {
			http.Error(w, "crushmap", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/ceph/radosgw/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("uid") != "alice" {
			http.Error(w, "uid", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/cluster/config/totem", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("consensus") != "3600" || r.Form.Get("link_mode") != "passive" {
			http.Error(w, "totem", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("type") != "glusterfs" || r.Form.Get("server") == "" || r.Form.Get("volume") != "gv0" {
			http.Error(w, "gluster", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.UpdateMapping(ctx, "pci", "gpu0", map[string]any{"map": []string{"pve1:0000:01:00.0"}}); err != nil {
		t.Fatalf("mapping put %v", err)
	}
	if err := c.UpdateHARule(ctx, "keep-together", HARuleRequest{Disable: true}); err != nil {
		t.Fatalf("ha disable %v", err)
	}
	if err := c.UpdateGroup(ctx, "ops", "on-call"); err != nil {
		t.Fatalf("group put %v", err)
	}
	if err := c.UpdateFWAlias(ctx, FWScope{}, "office", FWAlias{CIDR: "10.0.0.0/24", Comment: "lan"}); err != nil {
		t.Fatalf("alias put %v", err)
	}
	upid, err := c.RemoteMigrateGuest(ctx, "pve1", "qemu", 100, RemoteMigrateRequest{
		Remote: "site-b", TargetVMID: 200, TargetStorage: "ceph", TargetBridge: "vmbr0",
	})
	if err != nil || !strings.Contains(upid, "rmig") {
		t.Fatalf("remote_migrate %q %v", upid, err)
	}
	if err := c.CreateClusterRemote(ctx, map[string]any{"remote": "site-b", "type": "evpn", "server": "10.0.0.8"}); err != nil {
		t.Fatalf("remote create %v", err)
	}
	if err := c.UpdateClusterRemote(ctx, "site-b", map[string]any{"server": "10.0.0.9"}); err != nil {
		t.Fatalf("remote put %v", err)
	}
	if err := c.DeleteClusterRemote(ctx, "site-b"); err != nil {
		t.Fatalf("remote delete %v", err)
	}
	if err := c.UpdateCephCrush(ctx, "pve1", map[string]any{"crushmap": "# crush map"}); err != nil {
		t.Fatalf("crush put %v", err)
	}
	if err := c.CreateCephRGWUser(ctx, "pve1", map[string]any{"uid": "alice", "display-name": "Alice"}); err != nil {
		t.Fatalf("rgw user %v", err)
	}
	if err := c.UpdateClusterTotem(ctx, map[string]any{"consensus": "3600", "link_mode": "passive"}); err != nil {
		t.Fatalf("totem put %v", err)
	}
	if err := c.CreateStorage(ctx, map[string]any{"storage": "gfs", "type": "glusterfs", "server": "gluster1", "volume": "gv0", "transport": "tcp"}); err != nil {
		t.Fatalf("gluster create %v", err)
	}
}

func TestTermProxyPortString(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/zs-hv1/termproxy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"port":"5901","ticket":"PVEVNC:1:sig","user":"root@pam"}}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "root@pam!Hypervane=x", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.NodeTermProxy(context.Background(), "zs-hv1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != "5901" || got.Ticket == "" || got.User != "root@pam" {
		t.Fatalf("%+v", got)
	}
}

func TestStockLeftovers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("vmid") != "100" || r.URL.Query().Get("source") != "archive" {
			http.Error(w, "query", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"upid": "UPID:pve1:000:qmstart:100:", "node": "pve1", "type": "qmstart", "id": "100"}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/100/move_disk", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("disk") != "scsi0" || r.Form.Get("storage") != "local" || r.Form.Get("format") != "qcow2" || r.Form.Get("delete") != "1" {
			http.Error(w, "move", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:move"})
	})
	mux.HandleFunc("/api2/json/nodes/pve1/hardware/pci/0000:01:00.0/mdev", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"type": "nvidia-11", "available": 2}}})
	})
	mux.HandleFunc("/api2/json/nodes/pve2/hardware/pci/missing/mdev", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api2/json/nodes/pve1/storage/local/content/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("download") != "1" {
			http.Error(w, "download", 400)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("vma"))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tasks, err := c.NodeTasks(ctx, "pve1", NodeTaskQuery{VMID: "100", Source: "archive"})
	if err != nil || len(tasks) != 1 || tasks[0].Type != "qmstart" {
		t.Fatalf("tasks %+v %v", tasks, err)
	}
	upid, err := c.MoveDisk(ctx, "pve1", "qemu", 100, DiskMoveRequest{Disk: "scsi0", Storage: "local", Format: "qcow2", Delete: true})
	if err != nil || upid != "UPID:move" {
		t.Fatalf("move %q %v", upid, err)
	}
	mdev, err := c.NodeMdev(ctx, "pve1", "0000:01:00.0")
	if err != nil || !mdev.Supported || len(mdev.Items) != 1 {
		t.Fatalf("mdev %+v %v", mdev, err)
	}
	missing, err := c.NodeMdev(ctx, "pve2", "missing")
	if err != nil || missing.Supported {
		t.Fatalf("mdev 404 %+v %v", missing, err)
	}
	res, err := c.VolumeDownload(ctx, "pve1", "local", "local:backup/vzdump.vma.zst")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "vma" {
		t.Fatalf("download %q", body)
	}
}

func TestClusterRemotesUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/sdn/remotes", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"data":null,"errors":{"remote":"not implemented"}}`, http.StatusNotImplemented)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "t", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	list, err := c.ClusterRemotes(context.Background())
	if err != nil || list.Supported {
		t.Fatalf("remotes 501 %+v %v", list, err)
	}
}

// A same-node target and a linked clone's storage are both refused by
// Proxmox, so they are never sent.
func TestCloneSendsOnlyWhatProxmoxAccepts(t *testing.T) {
	var got []url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/qemu/9000/clone", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = append(got, r.PostForm)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:0001:0000:00000000:qmclone:9000:root@pam:"})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "root@pam!hv=abc", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, req := range []CloneRequest{
		{NewID: 101, Name: "same-node", Target: "pve1", Full: true, Storage: "local-lvm"},
		{NewID: 102, Name: "linked", Target: "pve1", Full: false, Storage: "local-lvm"},
		{NewID: 103, Name: "elsewhere", Target: "pve2", Full: true, Storage: "ceph"},
	} {
		if _, err := c.CloneGuest(ctx, "pve1", "qemu", 9000, req); err != nil {
			t.Fatal(err)
		}
	}
	if got[0].Has("target") || got[0].Get("storage") != "local-lvm" || got[0].Get("full") != "1" {
		t.Fatalf("same node: %v", got[0])
	}
	if got[1].Has("target") || got[1].Has("storage") || got[1].Has("full") {
		t.Fatalf("linked clone: %v", got[1])
	}
	if got[2].Get("target") != "pve2" || got[2].Get("storage") != "ceph" {
		t.Fatalf("other node: %v", got[2])
	}
}

func TestNodeCommandTermProxySendsTheCommand(t *testing.T) {
	var form url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/pve1/termproxy", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"port": "5900", "ticket": "PVEVNC:x", "user": "root@pam"}})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Token: "root@pam!hv=abc", TLSMode: TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NodeCommandTermProxy(context.Background(), "pve1", "upgrade")
	if err != nil || p.Port != "5900" || form.Get("cmd") != "upgrade" {
		t.Fatalf("termproxy %+v %v form %v", p, err, form)
	}
}
