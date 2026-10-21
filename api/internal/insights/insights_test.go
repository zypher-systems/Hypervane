package insights

import (
	"strings"
	"testing"
	"time"

	"hypervane/internal/pve"
)

const gib = int64(1 << 30)

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

func node(name string, mem, maxmem int64) pve.Resource {
	return pve.Resource{ID: "node/" + name, Type: "node", Node: name, Status: "online", Mem: mem, MaxMem: maxmem}
}

func vm(id int, name, nodeName, status string, maxmem int64) pve.Resource {
	return pve.Resource{ID: "qemu/" + name, Type: "qemu", VMID: id, Name: name, Node: nodeName, Status: status, MaxMem: maxmem}
}

func find(r Report, id string) *Insight {
	for i := range r.Insights {
		if r.Insights[i].ID == id || strings.HasPrefix(r.Insights[i].ID, id+":") {
			return &r.Insights[i]
		}
	}
	return nil
}

func baseInput() Input {
	return Input{
		Now: now,
		Resources: []pve.Resource{
			node("pve-a", 87*gib, 251*gib),
			node("pve-b", 58*gib, 251*gib),
			vm(100, "edge-proxy", "pve-a", "running", 4*gib),
			vm(107, "docker-01", "pve-a", "running", 16*gib),
			vm(114, "gitlab", "pve-a", "stopped", 8*gib),
			vm(119, "journal", "pve-b", "running", 6*gib),
		},
	}
}

func TestHealthyClusterHasNoInsights(t *testing.T) {
	in := baseInput()
	in.ActiveTasks = []pve.Task{}
	in.NotInJob = []pve.CoverageGuest{}
	in.BackupRuns = []BackupJobRun{}
	in.Snapshots = map[int][]pve.Snapshot{}
	in.StoppedSince = map[int]int64{}
	in.Certs = map[string][]pve.CertInfo{}
	r := Build(in)
	if len(r.Insights) != 0 {
		t.Fatalf("expected a calm report, got %+v", r.Insights)
	}
	if r.Failover == nil || len(r.Failover.Nodes) != 2 || !r.Failover.Nodes[0].Fits {
		t.Fatalf("failover %+v", r.Failover)
	}
	if r.Backups == nil || r.Backups.LastRun != 0 {
		t.Fatalf("backups summary %+v", r.Backups)
	}
}

func TestUnknownInputsSkipChecks(t *testing.T) {
	r := Build(baseInput())
	if r.Backups != nil {
		t.Fatal("backup summary must be absent when backup history is unknown")
	}
	for _, i := range r.Insights {
		t.Fatalf("no data, no claims: %+v", i)
	}
}

func TestQuorumLostIsFirst(t *testing.T) {
	in := baseInput()
	in.Status = []pve.ClusterStatus{{Type: "cluster", Nodes: 2, Quorate: 0}}
	in.Resources[2].Pending = true
	r := Build(in)
	if len(r.Insights) < 2 || r.Insights[0].ID != "quorum" {
		t.Fatalf("quorum should lead: %+v", r.Insights)
	}
}

func TestOfflineNodeCountsGuests(t *testing.T) {
	in := baseInput()
	in.Resources[1].Status = "offline"
	r := Build(in)
	i := find(r, "node-offline")
	if i == nil || i.Severity != Action || !strings.Contains(i.Detail, "1 guest on it is unreachable") {
		t.Fatalf("%+v", i)
	}
	if r.Failover != nil {
		t.Fatal("failover needs two online nodes")
	}
}

func TestPendingRestartSingleAndMany(t *testing.T) {
	in := baseInput()
	in.Resources[5].Pending = true
	i := find(Build(in), "pending-restart")
	if i == nil || i.Title != "journal needs a restart to apply changes" || i.Target == nil || i.Target.Tab != "hardware" {
		t.Fatalf("%+v", i)
	}
	in.Resources[2].Pending = true
	i = find(Build(in), "pending-restart")
	if i == nil || i.Title != "2 guests need a restart to apply changes" || !strings.HasPrefix(i.Detail, "edge-proxy and journal.") {
		t.Fatalf("%+v", i)
	}
}

func TestStaleLockIgnoresBusyGuestsAndRunningBackups(t *testing.T) {
	in := baseInput()
	in.Resources[3].Lock = "backup"    // docker-01 on pve-a
	in.Resources[5].Lock = "migrate"   // journal on pve-b
	in.Resources[4].Lock = "suspended" // gitlab: legitimate state
	in.ActiveTasks = []pve.Task{}
	r := Build(in)
	if find(r, "stale-lock:107") == nil || find(r, "stale-lock:119") == nil || find(r, "stale-lock:114") != nil {
		t.Fatalf("%+v", r.Insights)
	}
	in.ActiveTasks = []pve.Task{{Type: "vzdump", Node: "pve-a"}, {Type: "qmigrate", ID: "119", Node: "pve-b"}}
	r = Build(in)
	if find(r, "stale-lock") != nil {
		t.Fatalf("locks with running tasks are not stale: %+v", r.Insights)
	}
	if lock := find(Build(Input{Now: now, Resources: in.Resources}), "stale-lock"); lock != nil {
		t.Fatal("unknown task activity must not produce lock claims")
	}
}

func TestBackupFailuresFromRealLogFormat(t *testing.T) {
	log := []string{
		"INFO: starting new backup job: vzdump --all 1 --storage pbs-01 --mode snapshot",
		"INFO: skip external VMs: 119",
		"INFO: Starting Backup of VM 100 (qemu)",
		"INFO: Finished Backup of VM 100 (00:00:11)",
		"INFO: Starting Backup of VM 107 (qemu)",
		"INFO: Finished Backup of VM 107 (00:02:03)",
		"INFO: Starting Backup of VM 114 (qemu)",
		"ERROR: Backup of VM 114 failed - storage timeout on pbs-01",
		"INFO: Failed at 2026-09-24 02:14:31",
		"INFO: Backup job finished with errors",
		"TASK ERROR: job errors",
	}
	ok, failed := ParseBackupLog(log)
	if len(ok) != 2 || ok[0] != 100 || ok[1] != 107 {
		t.Fatalf("ok %v", ok)
	}
	if len(failed) != 1 || failed[0].VMID != 114 || failed[0].Reason != "storage timeout on pbs-01" {
		t.Fatalf("failed %+v", failed)
	}
	in := baseInput()
	in.BackupRuns = []BackupJobRun{{Node: "pve-a", UPID: "UPID:pve-a:x", Start: now.Add(-10 * time.Hour).Unix(), Status: "job errors", OK: ok, Failed: failed}}
	r := Build(in)
	i := find(r, "backup-failed")
	if i == nil || i.Title != "gitlab wasn't backed up in the last run" || i.Target.Kind != "task" || i.Target.UPID != "UPID:pve-a:x" {
		t.Fatalf("%+v", i)
	}
	if !strings.HasPrefix(i.Detail, "Storage timeout on pbs-01.") {
		t.Fatalf("detail %q", i.Detail)
	}
	if r.Backups == nil || r.Backups.OK != 2 || r.Backups.Failed != 1 {
		t.Fatalf("summary %+v", r.Backups)
	}
	if find(r, "backup-job-failed") != nil {
		t.Fatal("a job with per-guest errors is reported per guest")
	}
}

func TestOldBackupRunsAreIgnored(t *testing.T) {
	in := baseInput()
	in.BackupRuns = []BackupJobRun{{Node: "pve-a", Start: now.Add(-72 * time.Hour).Unix(), Status: "job errors", Failed: []BackupFailure{{VMID: 114, Reason: "x"}}}}
	r := Build(in)
	if find(r, "backup-failed") != nil {
		t.Fatal("a failure from three days ago is not last night's news")
	}
}

func TestWholeJobFailure(t *testing.T) {
	in := baseInput()
	in.BackupRuns = []BackupJobRun{{Node: "pve-b", UPID: "U", Start: now.Add(-5 * time.Hour).Unix(), Status: "unable to activate storage 'pbs-01'"}}
	i := find(Build(in), "backup-job-failed")
	if i == nil || i.Severity != Action || !strings.Contains(i.Detail, "unable to activate storage") {
		t.Fatalf("%+v", i)
	}
}

func TestBackupFailureHiddenGuestIsNotReported(t *testing.T) {
	in := baseInput()
	in.BackupRuns = []BackupJobRun{{Node: "pve-a", Start: now.Add(-5 * time.Hour).Unix(), Status: "job errors", Failed: []BackupFailure{{VMID: 999, Reason: "secret"}}}}
	r := Build(in)
	if i := find(r, "backup-failed"); i != nil {
		t.Fatalf("leaked a guest the viewer can't see: %+v", i)
	}
	for _, i := range r.Insights {
		if strings.Contains(strings.ToLower(i.Title+i.Detail), "secret") {
			t.Fatalf("leaked a guest the viewer can't see: %+v", i)
		}
	}
}

func TestStorageThresholdsAndDedup(t *testing.T) {
	in := baseInput()
	in.Resources = append(in.Resources,
		pve.Resource{Type: "storage", Storage: "pbs-01", Node: "pve-a", Status: "available", Disk: 85, MaxDisk: 100},
		pve.Resource{Type: "storage", Storage: "pbs-01", Node: "pve-b", Status: "available", Disk: 85, MaxDisk: 100},
		pve.Resource{Type: "storage", Storage: "local", Node: "pve-a", Status: "available", Disk: 95, MaxDisk: 100},
		pve.Resource{Type: "storage", Storage: "tank", Node: "pve-a", Status: "available", Disk: 10, MaxDisk: 100},
	)
	r := Build(in)
	var got []string
	for _, i := range r.Insights {
		if strings.HasPrefix(i.ID, "storage-full") {
			got = append(got, string(i.Severity)+" "+i.Title)
		}
	}
	want := []string{"action local is 95% full", "notice pbs-01 is 85% full"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCertificateWindows(t *testing.T) {
	in := baseInput()
	in.Certs = map[string][]pve.CertInfo{
		"pve-a":  {{Filename: "pve-root-ca.pem", NotAfter: now.Add(2 * day).Unix()}, {Filename: "pveproxy-ssl.pem", NotAfter: now.Add(10 * day).Unix()}},
		"pve-b":  {{Filename: "pve-ssl.pem", NotAfter: now.Add(20 * day).Unix()}},
		"hidden": {{Filename: "pve-ssl.pem", NotAfter: now.Add(-1 * day).Unix()}},
	}
	r := Build(in)
	a, b := find(r, "cert-expiring:pve-a"), find(r, "cert-expiring:pve-b")
	if a == nil || a.Severity != Action || !strings.Contains(a.Title, "10 days") {
		t.Fatalf("root CA must be ignored, proxy cert flagged: %+v", a)
	}
	if b == nil || b.Severity != Notice {
		t.Fatalf("%+v", b)
	}
	if find(r, "cert-expiring:hidden") != nil {
		t.Fatal("a node the viewer can't see must not be reported")
	}
}

func TestNoBackupJobSeverityDependsOnRunning(t *testing.T) {
	in := baseInput()
	in.NotInJob = []pve.CoverageGuest{{VMID: 114}}
	if i := find(Build(in), "no-backup-job"); i == nil || i.Severity != Notice || i.Title != "gitlab isn't in any backup job" {
		t.Fatalf("stopped only: %+v", i)
	}
	in.NotInJob = append(in.NotInJob, pve.CoverageGuest{VMID: 100})
	if i := find(Build(in), "no-backup-job"); i == nil || i.Severity != Action || i.Title != "2 guests aren't in any backup job" {
		t.Fatalf("running included: %+v", i)
	}
}

func TestFailoverMath(t *testing.T) {
	in := baseInput()
	// pve-b has 251-58 = 193 GiB free; pve-a's running guests need 20 GiB: fits.
	r := Build(in)
	a := r.Failover.Nodes[0]
	if a.Node != "pve-a" || a.Required != 20*gib || a.Available != 193*gib || !a.Fits {
		t.Fatalf("%+v", a)
	}
	// Squeeze pve-b so pve-a's HA guests no longer fit.
	in.Resources[1].Mem = 240 * gib
	in.Resources[3].HAState = "started"
	r = Build(in)
	i := find(r, "failover")
	if i == nil || i.Severity != Action || !strings.Contains(i.Title, "HA can't restart") {
		t.Fatalf("%+v", i)
	}
}

func TestOldSnapshotsAndLongStopped(t *testing.T) {
	in := baseInput()
	in.Snapshots = map[int][]pve.Snapshot{
		107: {{Name: "pre-upgrade", SnapTime: now.Add(-47 * day).Unix()}, {Name: "fresh", SnapTime: now.Add(-2 * day).Unix()}, {Name: "current"}},
	}
	in.StoppedSince = map[int]int64{114: now.Add(-40 * day).Unix()}
	r := Build(in)
	s := find(r, "old-snapshots")
	if s == nil || s.Title != "1 snapshot is older than 30 days" || !strings.Contains(s.Detail, "“pre-upgrade” on docker-01, 47 days") || s.Target.Tab != "snapshots" {
		t.Fatalf("%+v", s)
	}
	l := find(r, "long-stopped")
	if l == nil || l.Title != "gitlab has been stopped for 40 days" {
		t.Fatalf("%+v", l)
	}
}

func TestActionsSortBeforeNotices(t *testing.T) {
	in := baseInput()
	in.StoppedSince = map[int]int64{114: now.Add(-40 * day).Unix()}
	in.Resources[5].Pending = true
	r := Build(in)
	if len(r.Insights) != 2 || r.Insights[0].Severity != Action || r.Insights[1].Severity != Notice {
		t.Fatalf("%+v", r.Insights)
	}
}

func TestListNamesAndBytes(t *testing.T) {
	if got := listNames([]string{"a", "b", "c", "d", "e"}); got != "a, b, c and 2 more" {
		t.Fatal(got)
	}
	if got := listNames([]string{"a", "b"}); got != "a and b" {
		t.Fatal(got)
	}
	if got := Bytes(193 * gib); got != "193 GiB" {
		t.Fatal(got)
	}
	if got := Bytes(3*gib + gib/2); got != "3.5 GiB" {
		t.Fatal(got)
	}
}

func TestLaterSuccessClearsFailure(t *testing.T) {
	in := baseInput()
	in.BackupRuns = []BackupJobRun{
		{Node: "pve-a", Start: now.Add(-10 * time.Hour).Unix(), Status: "job errors", Failed: []BackupFailure{{VMID: 114, Reason: "timeout"}}},
		{Node: "pve-a", Start: now.Add(-2 * time.Hour).Unix(), Status: "OK", OK: []int{114}},
	}
	if i := find(Build(in), "backup-failed"); i != nil {
		t.Fatalf("a retry that succeeded clears the failure: %+v", i)
	}
	in.BackupRuns[1].Start = now.Add(-12 * time.Hour).Unix() // success came first, failure after
	if i := find(Build(in), "backup-failed"); i == nil {
		t.Fatal("a failure after the last success still counts")
	}
}

func TestBackupSummaryCountsLatestRunPerNode(t *testing.T) {
	in := baseInput()
	in.BackupRuns = []BackupJobRun{
		{Node: "pve-a", Start: now.Add(-30 * time.Hour).Unix(), Status: "OK", OK: []int{100, 107, 114}},
		{Node: "pve-a", Start: now.Add(-6 * time.Hour).Unix(), Status: "OK", OK: []int{100, 107, 114}},
		{Node: "pve-b", Start: now.Add(-6 * time.Hour).Unix(), Status: "OK", OK: []int{119}},
	}
	r := Build(in)
	if r.Backups == nil || r.Backups.OK != 4 || r.Backups.Failed != 0 || r.Backups.LastRun != now.Add(-6*time.Hour).Unix() {
		t.Fatalf("each guest once, from the latest run per node: %+v", r.Backups)
	}
}

func TestLastBackupsOnlyForVisibleGuests(t *testing.T) {
	in := baseInput()
	early, late := now.Add(-30*time.Hour).Unix(), now.Add(-6*time.Hour).Unix()
	in.BackupRuns = []BackupJobRun{
		{Node: "pve-a", Start: early, Status: "OK", OK: []int{100, 999}},
		{Node: "pve-a", Start: late, Status: "OK", OK: []int{100}},
	}
	r := Build(in)
	if r.LastBackups[100] != late {
		t.Fatalf("latest run wins: %+v", r.LastBackups)
	}
	if _, ok := r.LastBackups[999]; ok {
		t.Fatal("a guest the viewer can't see must not be listed")
	}
}
