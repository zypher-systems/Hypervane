package scripts

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"hypervane/internal/pve"
)

// Proxmox's "urlencoded" format (pve-common pve_verify_urlencoded).
var pveURLEncoded = regexp.MustCompile(`^[-%a-zA-Z0-9_.!~*'()]*$`)

func TestCloudTemplateBuildsAndTidiesUp(t *testing.T) {
	f, c := newFake(t)
	keys := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI+abc/def= me@laptop\nssh-rsa AAAAB3Nza== other"
	pl := mustPlan(t, c, "ubuntu-2404-template", Params{Node: "pve1", Storage: "local-lvm", User: "ops", SSHKeys: keys, VLAN: 20})
	// Templates stay as small as the image unless a size is set.
	if !slices.Equal(stepLabels(pl), []string{"download", "create", "template", "tidy"}) || pl.VMID != 104 {
		t.Fatalf("plan %v vmid %d", stepLabels(pl), pl.VMID)
	}
	j := runJob(t, c, pl)
	if j.Status != JobDone {
		t.Fatalf("job %+v", j)
	}
	dl := f.form("POST", "/nodes/pve1/storage/local/download-url")
	if dl.Get("content") != "import" || dl.Get("filename") != "noble-server-cloudimg-amd64.qcow2" ||
		dl.Get("url") != "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img" {
		t.Fatalf("download %v", dl)
	}
	cr := f.form("POST", "/nodes/pve1/qemu")
	want := map[string]string{
		"vmid": "104", "name": "ubuntu-2404-cloud", "cores": "2", "memory": "2048",
		"scsi0":     "local-lvm:0,import-from=local:import/noble-server-cloudimg-amd64.qcow2,discard=on",
		"ide2":      "local-lvm:cloudinit",
		"boot":      "order=scsi0",
		"serial0":   "socket",
		"net0":      "virtio,bridge=vmbr0,tag=20",
		"ipconfig0": "ip=dhcp",
		"ciuser":    "ops",
		"agent":     "1",
		"ostype":    "l26",
		"scsihw":    "virtio-scsi-single",
	}
	for k, v := range want {
		if cr.Get(k) != v {
			t.Fatalf("create %s = %q, want %q (form %v)", k, cr.Get(k), v, cr)
		}
	}
	if cr.Get("cpu") != "" {
		t.Fatalf("Ubuntu keeps Proxmox's default CPU type, got %q", cr.Get("cpu"))
	}
	if !pveURLEncoded.MatchString(cr.Get("sshkeys")) {
		t.Fatalf("sshkeys %q isn't in Proxmox's urlencoded format", cr.Get("sshkeys"))
	}
	if dec, _ := url.PathUnescape(cr.Get("sshkeys")); dec != keys {
		t.Fatalf("sshkeys decode to %q", dec)
	}
	if !strings.Contains(cr.Get("description"), "hypervane-recipe: ubuntu-2404-template") {
		t.Fatalf("description %q", cr.Get("description"))
	}
	writes := f.writes()
	wantOrder := []string{
		"POST /nodes/pve1/storage/local/download-url",
		"POST /nodes/pve1/qemu",
		"POST /nodes/pve1/qemu/104/template",
		"DELETE /nodes/pve1/storage/local/content/local:import/noble-server-cloudimg-amd64.qcow2",
	}
	if !slices.Equal(writes, wantOrder) {
		t.Fatalf("calls\n%v\nwant\n%v", writes, wantOrder)
	}
}

func TestCloudTemplateGrowsItsDiskOnlyWhenAsked(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "debian-13-template", Params{Node: "pve1", Storage: "local-lvm", DiskGB: 32})
	if !slices.Equal(stepLabels(pl), []string{"download", "create", "grow", "template", "tidy"}) {
		t.Fatalf("plan %v", stepLabels(pl))
	}
	runJob(t, c, pl)
	if f.form("PUT", "/nodes/pve1/qemu/104/resize").Get("size") != "32G" {
		t.Fatal("resize")
	}
}

func TestCloudTemplateReplacesAnEarlierDownload(t *testing.T) {
	f, c := newFake(t)
	f.content["pve1/local"] = []pve.StorageContent{{VolID: "local:import/debian-13-generic-amd64.qcow2", Content: "import"}}
	j := runJob(t, c, mustPlan(t, c, "debian-13-template", Params{Node: "pve1", Storage: "local-lvm"}))
	if j.Status != JobDone || !strings.Contains(j.Steps[0].Note, "earlier download") {
		t.Fatalf("job %+v", j)
	}
	if w := f.writes(); w[0] != "DELETE /nodes/pve1/storage/local/content/local:import/debian-13-generic-amd64.qcow2" ||
		w[1] != "POST /nodes/pve1/storage/local/download-url" {
		t.Fatalf("an earlier download must be removed first: %v", w)
	}
}

func TestEnterpriseLinux10GetsANewEnoughCPU(t *testing.T) {
	f, c := newFake(t)
	runJob(t, c, mustPlan(t, c, "rocky-10-template", Params{Node: "pve1", Storage: "local-lvm"}))
	if got := f.form("POST", "/nodes/pve1/qemu").Get("cpu"); got != "x86-64-v3" {
		t.Fatalf("Rocky 10 needs x86-64-v3, got %q", got)
	}
}

func TestCloudTemplateStopsWhereItFails(t *testing.T) {
	f, c := newFake(t)
	f.failTasks["resize:104"] = true
	j := runJob(t, c, mustPlan(t, c, "debian-13-template", Params{Node: "pve1", Storage: "local-lvm", DiskGB: 32}))
	if j.Status != JobFailed || j.Steps[2].Status != StepFailed || j.Steps[3].Status != StepSkipped ||
		!strings.Contains(j.Message, "VM 104 exists but isn't a template") {
		t.Fatalf("job %+v", j)
	}
}

func TestCloudTemplateChecksItsInputs(t *testing.T) {
	f, c := newFake(t)
	cases := []struct {
		p    Params
		want string
	}{
		{Params{}, "choose a node"},
		{Params{Node: "pve9"}, "isn't in this cluster"},
		{Params{Node: "pve1", Storage: "pbs"}, "cannot hold disk images"},
		{Params{Node: "pve1", Storage: "local-lvm", VMID: 101}, "already in use"},
		{Params{Node: "pve1", Storage: "local-lvm", Name: "bad name"}, "isn't valid"},
		{Params{Node: "pve1", Storage: "local-lvm", Bridge: "vmbr7"}, "has no bridge vmbr7"},
		{Params{Node: "pve1", Storage: "local-lvm", VLAN: 5000}, "VLAN tag"},
		{Params{Node: "pve1", Storage: "local-lvm", SSHKeys: "not a key"}, "SSH keys"},
		{Params{Node: "pve1", Storage: "local-lvm", User: "Bad User"}, "Linux user"},
		{Params{Node: "pve1", Storage: "local-lvm", DiskGB: -5}, "disk"},
	}
	for _, tc := range cases {
		_, err := plan(t, c, "debian-13-template", tc.p)
		wantInputError(t, err, tc.want)
	}
	// Without an import-capable store there is nowhere to download to.
	f.storages[0].Content = "iso,vztmpl,backup"
	_, err := plan(t, c, "debian-13-template", Params{Node: "pve1", Storage: "local-lvm"})
	wantInputError(t, err, "import")
	if len(f.writes()) != 0 {
		t.Fatalf("planning must not change anything: %v", f.writes())
	}
}

func TestRefreshKeepsTheTemplateAndDownloadsFirst(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "refresh-template", Params{VMID: 9000})
	if pl.Confirm == "" {
		t.Fatal("refreshing deletes a template, so it must ask first")
	}
	if !slices.Equal(stepLabels(pl), []string{"download", "delete", "create", "grow", "template", "tidy"}) {
		t.Fatalf("plan %v", stepLabels(pl))
	}
	j := runJob(t, c, pl)
	if j.Status != JobDone {
		t.Fatalf("job %+v", j)
	}
	if w := f.writes(); w[0] != "POST /nodes/pve1/storage/local/download-url" || w[1] != "DELETE /nodes/pve1/qemu/9000" {
		t.Fatalf("download first, then delete: %v", w)
	}
	cr := f.form("POST", "/nodes/pve1/qemu")
	want := map[string]string{
		"vmid": "9000", "name": "debian-13-cloud", "cores": "4", "memory": "4096", "cpu": "host",
		"net0":      "virtio=BC:24:11:00:00:01,bridge=vmbr1,tag=20",
		"scsi0":     "local-lvm:0,import-from=local:import/debian-13-generic-amd64.qcow2,discard=on,iothread=1",
		"ide2":      "local-lvm:cloudinit",
		"ciuser":    "ops",
		"ipconfig0": "ip=dhcp",
		"tags":      "base",
	}
	for k, v := range want {
		if cr.Get(k) != v {
			t.Fatalf("create %s = %q, want %q", k, cr.Get(k), v)
		}
	}
	if dec, _ := url.PathUnescape(cr.Get("sshkeys")); dec != "ssh-ed25519 AAAAC3Nza+k/ey= me@laptop" {
		t.Fatalf("keys must be carried over exactly once-encoded, got %q", cr.Get("sshkeys"))
	}
	desc := cr.Get("description")
	if !strings.HasPrefix(desc, "Our base image.") || strings.Count(desc, "hypervane-recipe:") != 1 {
		t.Fatalf("description %q", desc)
	}
	if f.form("PUT", "/nodes/pve1/qemu/9000/resize").Get("size") != "40G" {
		t.Fatal("the disk returns to its old size")
	}
	del := f.form("DELETE", "/nodes/pve1/qemu/9000")
	if del.Get("purge") != "1" || del.Get("destroy-unreferenced-disks") != "1" {
		t.Fatalf("delete %v", del)
	}
}

func TestRefreshDoesntDeleteWhenTheDownloadFails(t *testing.T) {
	f, c := newFake(t)
	f.failTasks["download:debian-13-generic-amd64.qcow2"] = true
	j := runJob(t, c, mustPlan(t, c, "refresh-template", Params{VMID: 9000}))
	if j.Status != JobFailed || j.Steps[1].Status != StepSkipped {
		t.Fatalf("job %+v", j)
	}
	for _, w := range f.writes() {
		if strings.HasPrefix(w, "DELETE /nodes/pve1/qemu/9000") {
			t.Fatal("the template must survive a failed download")
		}
	}
}

func TestRefreshRefusesTemplatesItCantRebuild(t *testing.T) {
	f, c := newFake(t)
	_, err := plan(t, c, "refresh-template", Params{VMID: 101})
	wantInputError(t, err, "isn't a template")
	f.configs[9000]["protection"] = 1
	_, err = plan(t, c, "refresh-template", Params{VMID: 9000})
	wantInputError(t, err, "protected")
	delete(f.configs[9000], "protection")
	f.configs[9000]["scsi1"] = "local-lvm:base-9000-disk-1,size=100G"
	_, err = plan(t, c, "refresh-template", Params{VMID: 9000})
	wantInputError(t, err, "more than one disk")
	delete(f.configs[9000], "scsi1")
	f.configs[9000]["description"] = "hand made"
	_, err = plan(t, c, "refresh-template", Params{VMID: 9000})
	wantInputError(t, err, "choose the image")
	if _, err := plan(t, c, "refresh-template", Params{VMID: 9000, Source: "fedora-44-template"}); err != nil {
		t.Fatal(err)
	}
}

func TestDeployClonesConfiguresAndStarts(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "web2", User: "admin",
		SSHKeys: "ssh-ed25519 AAAA+/= a@b", IP: "192.168.1.50/24", Gateway: "192.168.1.1", Memory: 8192})
	if pl.VMID != 104 {
		t.Fatalf("vmid %d", pl.VMID)
	}
	j := runJob(t, c, pl)
	if j.Status != JobDone {
		t.Fatalf("job %+v", j)
	}
	cl := f.form("POST", "/nodes/pve1/qemu/9000/clone")
	if cl.Get("newid") != "104" || cl.Get("name") != "web2" || cl.Get("full") != "1" {
		t.Fatalf("clone %v", cl)
	}
	cfg := f.form("PUT", "/nodes/pve1/qemu/104/config")
	if cfg.Get("ciuser") != "admin" || cfg.Get("ipconfig0") != "ip=192.168.1.50/24,gw=192.168.1.1" ||
		cfg.Get("memory") != "8192" || !pveURLEncoded.MatchString(cfg.Get("sshkeys")) {
		t.Fatalf("config %v", cfg)
	}
	if w := f.writes(); w[len(w)-1] != "POST /nodes/pve1/qemu/104/status/start" {
		t.Fatalf("start last: %v", w)
	}
}

// Lean templates: each deployed guest grows its own disk before it starts.
func TestDeployGrowsEachGuestsDisk(t *testing.T) {
	f, c := newFake(t)
	f.configs[9000]["boot"] = "order=scsi0;ide2;net0"
	pl := mustPlan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "web2", DiskGB: 80})
	if !slices.Equal(stepLabels(pl), []string{"clone-104", "grow-104", "start-104"}) {
		t.Fatalf("plan %v", stepLabels(pl))
	}
	runJob(t, c, pl)
	rs := f.form("PUT", "/nodes/pve1/qemu/104/resize")
	if rs.Get("disk") != "scsi0" || rs.Get("size") != "80G" {
		t.Fatalf("resize %v", rs)
	}
	if w := f.writes(); w[len(w)-2] != "PUT /nodes/pve1/qemu/104/resize" {
		t.Fatalf("grow before start: %v", w)
	}
	// The template's disk is 40G: asking for less can't shrink it.
	_, err := plan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "web2", DiskGB: 20})
	wantInputError(t, err, "already 40 GiB")

	// Several guests each get grown; a container grows its rootfs.
	f.reset()
	runJob(t, c, mustPlan(t, c, "deploy-many", Params{VMID: 9000, Name: "web", Count: 2, DiskGB: 50, Start: bptr(false)}))
	if n := strings.Count(strings.Join(f.writes(), "\n"), "/resize"); n != 2 {
		t.Fatalf("resizes %d", n)
	}
	f.resources = append(f.resources, pve.Resource{Type: "lxc", VMID: 9001, Name: "debian-ct", Node: "pve1", Status: "stopped", Template: 1})
	f.configs[9001] = map[string]any{"hostname": "debian-ct", "rootfs": "local-lvm:base-9001-disk-0,size=4G", "template": 1}
	f.reset()
	runJob(t, c, mustPlan(t, c, "deploy-from-template", Params{VMID: 9001, Name: "ct2", DiskGB: 16, Start: bptr(false)}))
	if rs := f.form("PUT", "/nodes/pve1/lxc/104/resize"); rs.Get("disk") != "rootfs" || rs.Get("size") != "16G" {
		t.Fatalf("container resize %v", rs)
	}
}

func TestDeployChecksItsInputs(t *testing.T) {
	_, c := newFake(t)
	_, err := plan(t, c, "deploy-from-template", Params{VMID: 101, Name: "x"})
	wantInputError(t, err, "isn't a template")
	_, err = plan(t, c, "deploy-from-template", Params{VMID: 9000})
	wantInputError(t, err, "enter a name")
	_, err = plan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "x", IP: "10.0.0.5"})
	wantInputError(t, err, "needs a prefix")
	_, err = plan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "x", IP: "10.0.0.5/24"})
	wantInputError(t, err, "gateway")
	_, err = plan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "x", NewID: 102})
	wantInputError(t, err, "already in use")
	_, err = plan(t, c, "deploy-from-template", Params{VMID: 9000, Name: "x", Full: bptr(false), Storage: "nfs"})
	wantInputError(t, err, "linked clone")
}

func TestDeployManyCarriesOnPastAFailedGuest(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "deploy-many", Params{VMID: 9000, Name: "web", Count: 3, Start: bptr(false)})
	if !slices.Equal(stepLabels(pl), []string{"clone-104", "clone-105", "clone-106"}) {
		t.Fatalf("three clones, no start: %v", stepLabels(pl))
	}
	// The second clone fails; the first and third still happen.
	f.failTasks["qemuclone:105"] = true
	j := runJob(t, c, pl)
	if j.Status != JobPartial || j.Steps[0].Status != StepDone || j.Steps[1].Status != StepFailed || j.Steps[2].Status != StepDone {
		t.Fatalf("job %+v", j)
	}
	var names []string
	for _, cl := range f.calls {
		if strings.HasSuffix(cl.Path, "/clone") {
			names = append(names, cl.Form.Get("name")+"="+cl.Form.Get("newid"))
		}
	}
	if !slices.Equal(names, []string{"web-1=104", "web-2=105", "web-3=106"}) {
		t.Fatalf("clones %v", names)
	}
	_, err := plan(t, c, "deploy-many", Params{VMID: 9000, Name: "web", Count: 50})
	wantInputError(t, err, "how many")
}

func TestEnableAgentKeepsItsOptions(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "enable-agent", Params{VMID: 101})
	if len(pl.Notes) == 0 || !strings.Contains(pl.Notes[0], "shut down and started again") {
		t.Fatalf("a running VM needs a cold restart: %v", pl.Notes)
	}
	runJob(t, c, pl)
	if got := f.form("PUT", "/nodes/pve1/qemu/101/config").Get("agent"); got != "1,fstrim_cloned_disks=1" {
		t.Fatalf("agent %q", got)
	}
	f.configs[102]["agent"] = "enabled=1"
	_, err := plan(t, c, "enable-agent", Params{VMID: 102})
	wantInputError(t, err, "already on")
	_, err = plan(t, c, "enable-agent", Params{VMID: 103})
	wantInputError(t, err, "for VMs")
}

func TestConvertNeedsAStoppedGuest(t *testing.T) {
	f, c := newFake(t)
	_, err := plan(t, c, "convert-to-template", Params{VMID: 101})
	wantInputError(t, err, "shut down")
	pl := mustPlan(t, c, "convert-to-template", Params{VMID: 102})
	if pl.Confirm == "" {
		t.Fatal("converting can't be undone, so it must ask first")
	}
	runJob(t, c, pl)
	if w := f.writes(); !slices.Equal(w, []string{"POST /nodes/pve1/qemu/102/template"}) {
		t.Fatalf("%v", w)
	}
}

func TestCheckUpdatesReportsEachNode(t *testing.T) {
	f, c := newFake(t)
	j := runJob(t, c, mustPlan(t, c, "check-updates", Params{}))
	if j.Status != JobDone || j.Steps[0].Note != "3 updates available." || j.Steps[1].Note != "Up to date." {
		t.Fatalf("job %+v", j)
	}
	if w := f.writes(); !slices.Equal(w, []string{"POST /nodes/pve1/apt/update", "POST /nodes/pve2/apt/update"}) {
		t.Fatalf("%v", w)
	}
}

func TestShutdownAndStartANode(t *testing.T) {
	f, c := newFake(t)
	pl := mustPlan(t, c, "shutdown-node-guests", Params{Node: "pve1"})
	if pl.Confirm == "" {
		t.Fatal("shutting guests down must ask first")
	}
	runJob(t, c, pl)
	st := f.form("POST", "/nodes/pve1/stopall")
	if st.Get("vms") != "101,103" || st.Get("force-stop") != "1" || st.Get("timeout") != "180" {
		t.Fatalf("stopall %v", st)
	}
	f.reset()
	// Only stopped guests set to start at boot: 102 (101 is running).
	runJob(t, c, mustPlan(t, c, "start-node-guests", Params{Node: "pve1"}))
	sa := f.form("POST", "/nodes/pve1/startall")
	if sa.Get("vms") != "102" || sa.Get("force") != "1" {
		t.Fatalf("startall %v", sa)
	}
	f.configs[102]["onboot"] = 0
	_, err := plan(t, c, "start-node-guests", Params{Node: "pve1"})
	wantInputError(t, err, "set to start at boot")
	if _, err := plan(t, c, "start-node-guests", Params{Node: "pve1", OnbootOnly: bptr(false)}); err != nil {
		t.Fatal(err)
	}
}

func TestEvacuateMovesEveryGuest(t *testing.T) {
	f, c := newFake(t)
	_, err := plan(t, c, "evacuate-node", Params{Node: "pve1", Target: "pve1"})
	wantInputError(t, err, "different node")
	pl := mustPlan(t, c, "evacuate-node", Params{Node: "pve1", Target: "pve2"})
	if pl.Confirm == "" {
		t.Fatal("moving guests must ask first")
	}
	runJob(t, c, pl)
	m := f.form("POST", "/nodes/pve1/migrateall")
	if m.Get("target") != "pve2" || m.Get("vms") != "101,102,103" || m.Get("with-local-disks") != "1" || m.Get("maxworkers") != "2" {
		t.Fatalf("migrateall %v", m)
	}
}

func TestBackupNowRunsOneTaskPerNode(t *testing.T) {
	f, c := newFake(t)
	j := runJob(t, c, mustPlan(t, c, "backup-unprotected", Params{Storage: "nfs"}))
	if j.Status != JobDone || len(j.Steps) != 2 {
		t.Fatalf("job %+v", j)
	}
	b1 := f.form("POST", "/nodes/pve1/vzdump")
	if b1.Get("vmid") != "101,103" || b1.Get("storage") != "nfs" || b1.Get("mode") != "snapshot" || b1.Get("compress") != "zstd" {
		t.Fatalf("pve1 vzdump %v", b1)
	}
	if f.form("POST", "/nodes/pve2/vzdump").Get("vmid") != "201" {
		t.Fatal("pve2 vzdump")
	}
	f.reset()
	runJob(t, c, mustPlan(t, c, "backup-unprotected", Params{Storage: "pbs", Node: "pve2"}))
	if b := f.form("POST", "/nodes/pve2/vzdump"); b.Get("compress") != "" {
		t.Fatalf("PBS compresses on its own: %v", b)
	}
	_, err := plan(t, c, "backup-unprotected", Params{Storage: "pve2-only"})
	wantInputError(t, err, "isn't available on pve1")
	_, err = plan(t, c, "backup-unprotected", Params{Storage: "nfs", Mode: "yolo"})
	wantInputError(t, err, "mode")
	f.missing = nil
	_, err = plan(t, c, "backup-unprotected", Params{Storage: "nfs"})
	wantInputError(t, err, "covered")
}

func TestBackupJobCoversTheUnprotected(t *testing.T) {
	f, c := newFake(t)
	j := runJob(t, c, mustPlan(t, c, "backup-job-unprotected", Params{Storage: "nfs", Schedule: "sat 02:00", Keep: 10}))
	if j.Status != JobDone {
		t.Fatalf("job %+v", j)
	}
	b := f.form("POST", "/cluster/backup")
	if b.Get("vmid") != "101,103,201" || b.Get("schedule") != "sat 02:00" || b.Get("storage") != "nfs" ||
		b.Get("prune-backups") != "keep-daily=10,keep-weekly=4,keep-monthly=3" || b.Get("enabled") != "1" {
		t.Fatalf("job %v", b)
	}
	_, err := plan(t, c, "backup-job-unprotected", Params{Storage: "nfs", Schedule: "every; rm"})
	wantInputError(t, err, "calendar")
	_, err = plan(t, c, "backup-job-unprotected", Params{Storage: "pve2-only"})
	wantInputError(t, err, "shared storage")
}

func TestSnapshotNodeTakesOnePerGuest(t *testing.T) {
	f, c := newFake(t)
	_, err := plan(t, c, "snapshot-node", Params{Node: "pve1", SnapName: "9bad"})
	wantInputError(t, err, "start with a letter")
	f.failTasks["qemusnapshot:101"] = true
	j := runJob(t, c, mustPlan(t, c, "snapshot-node", Params{Node: "pve1", SnapName: "pre-patch"}))
	if j.Status != JobPartial || len(j.Steps) != 3 || j.Steps[0].Status != StepFailed || j.Steps[2].Status != StepDone {
		t.Fatalf("job %+v", j)
	}
	if f.form("POST", "/nodes/pve1/lxc/103/snapshot").Get("snapname") != "pre-patch" {
		t.Fatal("container snapshot")
	}
	pl := mustPlan(t, c, "snapshot-node", Params{Node: "pve1", RunningOnly: true})
	if len(pl.Steps) != 2 || !strings.Contains(pl.Steps[0].Label, " as hv-") {
		t.Fatalf("running only, dated name: %v", pl.Steps)
	}
}

func TestSnapshotCleanupDeletesOnlyOldOnes(t *testing.T) {
	f, c := newFake(t)
	day := int64(86400)
	now := time.Now().Unix()
	f.snapshots[101] = []pve.Snapshot{
		{Name: "hv-old", SnapTime: now - 40*day},
		{Name: "hv-new", SnapTime: now - 2*day},
		{Name: "before-upgrade", SnapTime: now - 90*day},
	}
	f.snapshots[201] = []pve.Snapshot{{Name: "hv-older", SnapTime: now - 60*day}}
	pl := mustPlan(t, c, "snapshot-cleanup", Params{Days: 30})
	if len(pl.Steps) != 2 || pl.Confirm == "" {
		t.Fatalf("plan %v", stepLabels(pl))
	}
	runJob(t, c, pl)
	w := f.writes()
	if !slices.Equal(w, []string{"DELETE /nodes/pve1/qemu/101/snapshot/hv-old", "DELETE /nodes/pve2/qemu/201/snapshot/hv-older"}) {
		t.Fatalf("%v", w)
	}
	// Every snapshot, one node.
	pl = mustPlan(t, c, "snapshot-cleanup", Params{Days: 30, Node: "pve1", OnlyHypervane: bptr(false)})
	if len(pl.Steps) != 2 {
		t.Fatalf("plan %v", stepLabels(pl))
	}
	_, err := plan(t, c, "snapshot-cleanup", Params{Days: 365})
	wantInputError(t, err, "older than 365 days")
}

func TestEncodeSSHKeysMatchesProxmox(t *testing.T) {
	keys := "ssh-ed25519 AAAA+b/c= me@host\r\nssh-rsa AAAAB3== you@there"
	enc := pve.EncodeSSHKeys(keys)
	if !pveURLEncoded.MatchString(enc) || strings.Contains(enc, "+") {
		t.Fatalf("%q", enc)
	}
	if dec, _ := url.PathUnescape(enc); dec != "ssh-ed25519 AAAA+b/c= me@host\nssh-rsa AAAAB3== you@there" {
		t.Fatalf("round trip %q", dec)
	}
	if pve.EncodeSSHKeys(enc) != enc {
		t.Fatal("encoding twice must not double-encode")
	}
}
