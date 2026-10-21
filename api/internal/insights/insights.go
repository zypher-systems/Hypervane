// Package insights turns cluster state into a short list of things an operator
// should decide on, the way a colleague would summarize the cluster: only what
// needs attention, each with the numbers behind it and one next step.
//
// Build is pure: callers gather the inputs (and decide which guests the viewer
// may see); nothing here talks to Proxmox.
package insights

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hypervane/internal/pve"
)

type Severity string

const (
	// Action needs a decision now.
	Action Severity = "action"
	// Notice is worth a look but not urgent.
	Notice Severity = "notice"
)

// Target is where an insight's action leads in the UI.
type Target struct {
	Kind    string `json:"kind"` // guest | node | storage | backups | ha | task | datacenter
	Node    string `json:"node,omitempty"`
	VMID    int    `json:"vmid,omitempty"`
	Type    string `json:"type,omitempty"`
	Tab     string `json:"tab,omitempty"`
	Storage string `json:"storage,omitempty"`
	UPID    string `json:"upid,omitempty"`
}

// Subject is a guest, node, or storage an insight is about.
type Subject struct {
	Kind string `json:"kind"` // guest | node | storage
	Node string `json:"node,omitempty"`
	VMID int    `json:"vmid,omitempty"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

type Insight struct {
	ID       string    `json:"id"`
	Severity Severity  `json:"severity"`
	Title    string    `json:"title"`
	Detail   string    `json:"detail"`
	Action   string    `json:"action,omitempty"`
	Target   *Target   `json:"target,omitempty"`
	Subjects []Subject `json:"subjects,omitempty"`
}

// FailoverNode answers "if this node fails, can the others take its guests?"
// in memory terms: configured memory of its running guests against the free
// memory on the remaining online nodes.
type FailoverNode struct {
	Node       string `json:"node"`
	Required   int64  `json:"required"`
	HARequired int64  `json:"haRequired"`
	Available  int64  `json:"available"`
	Fits       bool   `json:"fits"`
	HAFits     bool   `json:"haFits"`
}

type Failover struct {
	Nodes []FailoverNode `json:"nodes"`
}

// BackupRun summarizes the most recent scheduled backup across nodes.
type BackupRun struct {
	LastRun int64 `json:"lastRun"`
	OK      int   `json:"ok"`
	Failed  int   `json:"failed"`
	Running bool  `json:"running"`
}

type Report struct {
	GeneratedAt int64      `json:"generatedAt"`
	Insights    []Insight  `json:"insights"`
	Failover    *Failover  `json:"failover,omitempty"`
	Backups     *BackupRun `json:"backups,omitempty"`
	// LastBackups maps a guest's VMID to the start of the most recent run that
	// backed it up, for guests the viewer may see (recent runs only).
	LastBackups map[int]int64 `json:"lastBackups,omitempty"`
	// Unavailable names checks that could not run (permissions, timeouts), so
	// an empty list is not mistaken for a clean bill of health.
	Unavailable []string `json:"unavailable,omitempty"`
}

// BackupFailure is one guest a vzdump job could not back up.
type BackupFailure struct {
	VMID   int
	Reason string
}

// BackupJobRun is one vzdump task (one node's share of a backup job).
type BackupJobRun struct {
	Node    string
	UPID    string
	Start   int64
	End     int64
	Status  string // Proxmox exit status; "OK" on success, empty while running
	Running bool
	OK      []int
	Failed  []BackupFailure
}

// Input is everything Build looks at. A nil map or slice means "unknown"
// (the check is skipped); an empty one means "known to be empty".
type Input struct {
	Now          time.Time
	Status       []pve.ClusterStatus
	Resources    []pve.Resource // already limited to what the viewer may see
	ActiveTasks  []pve.Task
	NotInJob     []pve.CoverageGuest
	BackupRuns   []BackupJobRun
	Snapshots    map[int][]pve.Snapshot
	StoppedSince map[int]int64
	Certs        map[string][]pve.CertInfo
	HA           []pve.HACurrent
}

const (
	day               = 24 * time.Hour
	backupWindow      = 36 * time.Hour
	oldSnapshotAge    = 30 * day
	longStoppedAge    = 30 * day
	storageNotice     = 80.0
	storageAction     = 90.0
	certNoticeDays    = 30
	certActionDays    = 14
	maxNamesInSummary = 3
)

// order keeps the list stable and puts the most consequential items first
// within each severity.
var order = []string{
	"quorum", "node-offline", "ha-error", "backup-failed", "backup-job-failed", "stale-lock",
	"storage-full", "cert-expiring", "pending-restart", "no-backup-job", "failover",
	"old-snapshots", "long-stopped",
}

func Build(in Input) Report {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	idx := newIndex(in.Resources)
	var out []Insight
	add := func(i *Insight) {
		if i != nil {
			out = append(out, *i)
		}
	}

	add(quorum(in.Status, idx))
	out = append(out, offlineNodes(idx)...)
	add(haErrors(in.HA, idx))
	runs := recentRuns(in.BackupRuns, now)
	out = append(out, backupFailures(runs, idx)...)
	out = append(out, staleLocks(in.ActiveTasks, idx)...)
	out = append(out, storageFull(idx)...)
	out = append(out, certificates(in.Certs, idx, now)...)
	add(pendingRestart(idx))
	add(noBackupJob(in.NotInJob, idx))
	failover := computeFailover(idx)
	add(failoverInsight(failover))
	add(oldSnapshots(in.Snapshots, idx, now))
	add(longStopped(in.StoppedSince, idx, now))

	rank := map[string]int{}
	for i, id := range order {
		rank[id] = i
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Severity != out[b].Severity {
			return out[a].Severity == Action
		}
		return rank[baseID(out[a].ID)] < rank[baseID(out[b].ID)]
	})
	if out == nil {
		out = []Insight{}
	}
	return Report{
		GeneratedAt: now.Unix(),
		Insights:    out,
		Failover:    failover,
		Backups:     summarizeRuns(runs, in.BackupRuns),
		LastBackups: lastBackups(runs, idx),
	}
}

// ---- resource index -------------------------------------------------------

type index struct {
	nodes   []pve.Resource
	guests  []pve.Resource
	storage []pve.Resource
	byVMID  map[int]pve.Resource
	online  map[string]bool
	visible map[string]bool
}

func newIndex(resources []pve.Resource) index {
	idx := index{byVMID: map[int]pve.Resource{}, online: map[string]bool{}, visible: map[string]bool{}}
	for _, r := range resources {
		switch r.Type {
		case "node":
			idx.nodes = append(idx.nodes, r)
			idx.visible[r.Node] = true
			if r.Status == "online" {
				idx.online[r.Node] = true
			}
		case "qemu", "lxc":
			idx.guests = append(idx.guests, r)
			idx.byVMID[r.VMID] = r
		case "storage":
			idx.storage = append(idx.storage, r)
		}
	}
	sort.Slice(idx.nodes, func(a, b int) bool { return idx.nodes[a].Node < idx.nodes[b].Node })
	sort.Slice(idx.guests, func(a, b int) bool { return idx.guests[a].VMID < idx.guests[b].VMID })
	return idx
}

func guestName(r pve.Resource) string {
	if strings.TrimSpace(r.Name) != "" {
		return r.Name
	}
	if r.Type == "lxc" {
		return "CT " + strconv.Itoa(r.VMID)
	}
	return "VM " + strconv.Itoa(r.VMID)
}

func guestSubject(r pve.Resource) Subject {
	return Subject{Kind: "guest", Node: r.Node, VMID: r.VMID, Type: r.Type, Name: guestName(r)}
}

func guestTarget(r pve.Resource, tab string) *Target {
	return &Target{Kind: "guest", Node: r.Node, VMID: r.VMID, Type: r.Type, Tab: tab}
}

// ---- checks ----------------------------------------------------------------

func quorum(status []pve.ClusterStatus, idx index) *Insight {
	if len(idx.nodes) == 0 {
		return nil
	}
	for _, s := range status {
		if s.Type == "cluster" && s.Nodes > 1 && s.Quorate == 0 {
			return &Insight{
				ID:       "quorum",
				Severity: Action,
				Title:    "The cluster has lost quorum",
				Detail:   "Configuration is read-only and HA can't move guests until enough nodes are back online.",
				Target:   &Target{Kind: "datacenter"},
			}
		}
	}
	return nil
}

func offlineNodes(idx index) []Insight {
	var out []Insight
	for _, n := range idx.nodes {
		if n.Status == "online" {
			continue
		}
		count := 0
		for _, g := range idx.guests {
			if g.Node == n.Node && g.Template == 0 {
				count++
			}
		}
		detail := "Its guests are unreachable unless HA restarted them elsewhere."
		if count > 0 {
			detail = fmt.Sprintf("%s on it %s unreachable unless HA restarted them elsewhere.", plural(count, "guest", "guests"), isAre(count))
		}
		out = append(out, Insight{
			ID:       "node-offline:" + n.Node,
			Severity: Action,
			Title:    n.Node + " is offline",
			Detail:   detail,
			Target:   &Target{Kind: "node", Node: n.Node},
			Subjects: []Subject{{Kind: "node", Node: n.Node, Name: n.Node}},
		})
	}
	return out
}

func haErrors(ha []pve.HACurrent, idx index) *Insight {
	var bad []pve.HACurrent
	for _, h := range ha {
		if h.Type != "service" && h.SID == "" {
			continue
		}
		state := strings.ToLower(h.State + " " + h.Status)
		if !strings.Contains(state, "error") && !strings.Contains(state, "fence") && !strings.Contains(state, "recovery") {
			continue
		}
		if vmid := sidVMID(h.SID); vmid > 0 {
			if _, ok := idx.byVMID[vmid]; !ok {
				continue
			}
		}
		bad = append(bad, h)
	}
	if len(bad) == 0 {
		return nil
	}
	var names []string
	var subjects []Subject
	for _, h := range bad {
		name := h.SID
		if g, ok := idx.byVMID[sidVMID(h.SID)]; ok {
			name = guestName(g)
			subjects = append(subjects, guestSubject(g))
		}
		names = append(names, name)
	}
	title := "HA can't run " + names[0]
	if len(bad) > 1 {
		title = fmt.Sprintf("%d HA resources are in error", len(bad))
	}
	state := strings.TrimSpace(bad[0].State)
	if state == "" {
		state = strings.TrimSpace(bad[0].Status)
	}
	detail := fmt.Sprintf("State %q. HA stops managing a resource in error until it is cleared.", state)
	if len(bad) > 1 {
		detail = "Affected: " + listNames(names) + ". HA stops managing a resource in error until it is cleared."
	}
	return &Insight{ID: "ha-error", Severity: Action, Title: title, Detail: detail, Action: "Open HA", Target: &Target{Kind: "ha"}, Subjects: subjects}
}

func sidVMID(sid string) int {
	if i := strings.LastIndex(sid, ":"); i >= 0 {
		n, _ := strconv.Atoi(sid[i+1:])
		return n
	}
	return 0
}

func recentRuns(runs []BackupJobRun, now time.Time) []BackupJobRun {
	var out []BackupJobRun
	cutoff := now.Add(-backupWindow).Unix()
	for _, r := range runs {
		if r.Start >= cutoff {
			out = append(out, r)
		}
	}
	return out
}

func backupFailures(runs []BackupJobRun, idx index) []Insight {
	type failed struct {
		guest pve.Resource
		run   BackupJobRun
		why   string
	}
	var list []failed
	var out []Insight
	// A guest that failed in one run but was backed up by a later one (a second
	// job, or a manual retry) is fine.
	lastOK := map[int]int64{}
	for _, run := range runs {
		for _, id := range run.OK {
			if run.Start > lastOK[id] {
				lastOK[id] = run.Start
			}
		}
	}
	for _, run := range runs {
		if run.Running {
			continue
		}
		for _, f := range run.Failed {
			g, ok := idx.byVMID[f.VMID]
			if !ok || lastOK[f.VMID] > run.Start {
				continue
			}
			list = append(list, failed{guest: g, run: run, why: f.Reason})
		}
		// A job that failed as a whole (bad storage, lock timeout) names no
		// guest in the log; report the job itself when the viewer can see the node.
		if len(run.Failed) == 0 && run.Status != "" && run.Status != "OK" && idx.visible[run.Node] {
			out = append(out, Insight{
				ID:       "backup-job-failed:" + run.Node,
				Severity: Action,
				Title:    "Last night's backup on " + run.Node + " failed",
				Detail:   "Proxmox reported: " + run.Status + ".",
				Action:   "View log",
				Target:   &Target{Kind: "task", Node: run.Node, UPID: run.UPID},
			})
		}
	}
	if len(list) == 0 {
		return out
	}
	if len(list) == 1 {
		f := list[0]
		return append(out, Insight{
			ID:       "backup-failed",
			Severity: Action,
			Title:    guestName(f.guest) + " wasn't backed up in the last run",
			Detail:   sentence(f.why) + " (backup job on " + f.run.Node + ")",
			Action:   "View log",
			Target:   &Target{Kind: "task", Node: f.run.Node, UPID: f.run.UPID},
			Subjects: []Subject{guestSubject(f.guest)},
		})
	}
	names := make([]string, 0, len(list))
	subjects := make([]Subject, 0, len(list))
	for _, f := range list {
		names = append(names, guestName(f.guest))
		subjects = append(subjects, guestSubject(f.guest))
	}
	return append(out, Insight{
		ID:       "backup-failed",
		Severity: Action,
		Title:    fmt.Sprintf("%d guests weren't backed up in the last run", len(list)),
		Detail:   listNames(names) + ". First error: " + sentence(list[0].why),
		Action:   "View log",
		Target:   &Target{Kind: "task", Node: list[0].run.Node, UPID: list[0].run.UPID},
		Subjects: subjects,
	})
}

// Locks Proxmox sets for long-lived, legitimate states.
var benignLocks = map[string]bool{"suspended": true, "mounted": true, "": true}

func staleLocks(active []pve.Task, idx index) []Insight {
	if active == nil {
		return nil
	}
	busy := map[int]bool{}
	backupOn := map[string]bool{}
	for _, t := range active {
		if t.Type == "vzdump" {
			backupOn[t.Node] = true
		}
		if n, err := strconv.Atoi(t.ID); err == nil && n > 0 {
			busy[n] = true
		}
	}
	var out []Insight
	for _, g := range idx.guests {
		if benignLocks[g.Lock] || busy[g.VMID] {
			continue
		}
		if g.Lock == "backup" && backupOn[g.Node] {
			continue
		}
		cmd := "qm"
		if g.Type == "lxc" {
			cmd = "pct"
		}
		out = append(out, Insight{
			ID:       "stale-lock:" + strconv.Itoa(g.VMID),
			Severity: Action,
			Title:    fmt.Sprintf("%s is locked (%s) with no task running", guestName(g), g.Lock),
			Detail:   fmt.Sprintf("The lock is left over from an interrupted job, and Proxmox refuses most actions until it's cleared (%s unlock %d on %s).", cmd, g.VMID, g.Node),
			Action:   "Open guest",
			Target:   guestTarget(g, ""),
			Subjects: []Subject{guestSubject(g)},
		})
	}
	return out
}

func storageFull(idx index) []Insight {
	type seen struct {
		r   pve.Resource
		pct float64
	}
	best := map[string]seen{}
	var names []string
	for _, s := range idx.storage {
		if s.MaxDisk <= 0 || (s.Status != "" && s.Status != "available") {
			continue
		}
		pct := float64(s.Disk) / float64(s.MaxDisk) * 100
		if cur, ok := best[s.Storage]; !ok || pct > cur.pct {
			if !ok {
				names = append(names, s.Storage)
			}
			best[s.Storage] = seen{r: s, pct: pct}
		}
	}
	sort.Strings(names)
	var out []Insight
	for _, name := range names {
		b := best[name]
		if b.pct < storageNotice {
			continue
		}
		sev := Notice
		if b.pct >= storageAction {
			sev = Action
		}
		free := b.r.MaxDisk - b.r.Disk
		out = append(out, Insight{
			ID:       "storage-full:" + name,
			Severity: sev,
			Title:    fmt.Sprintf("%s is %.0f%% full", name, b.pct),
			Detail:   fmt.Sprintf("%s free of %s. Writes fail and guests can pause when it fills up.", Bytes(free), Bytes(b.r.MaxDisk)),
			Action:   "Open storage",
			Target:   &Target{Kind: "storage", Storage: name, Node: b.r.Node},
			Subjects: []Subject{{Kind: "storage", Node: b.r.Node, Name: name}},
		})
	}
	return out
}

func certificates(certs map[string][]pve.CertInfo, idx index, now time.Time) []Insight {
	if certs == nil {
		return nil
	}
	nodes := make([]string, 0, len(certs))
	for n := range certs {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	var out []Insight
	for _, node := range nodes {
		if !idx.visible[node] {
			continue
		}
		var worst *pve.CertInfo
		for i := range certs[node] {
			c := certs[node][i]
			if c.NotAfter == 0 || c.Filename == "pve-root-ca.pem" {
				continue
			}
			if worst == nil || c.NotAfter < worst.NotAfter {
				worst = &c
			}
		}
		if worst == nil {
			continue
		}
		days := int(time.Unix(worst.NotAfter, 0).Sub(now).Hours() / 24)
		if days >= certNoticeDays {
			continue
		}
		sev := Notice
		if days < certActionDays {
			sev = Action
		}
		title := fmt.Sprintf("The certificate on %s expires in %s", node, plural(days, "day", "days"))
		if days < 0 {
			title = "The certificate on " + node + " has expired"
		} else if days == 0 {
			title = "The certificate on " + node + " expires today"
		}
		out = append(out, Insight{
			ID:       "cert-expiring:" + node,
			Severity: sev,
			Title:    title,
			Detail:   fmt.Sprintf("%s is valid until %s. Browsers and API clients reject an expired certificate.", worst.Filename, time.Unix(worst.NotAfter, 0).UTC().Format("Jan 2, 2006")),
			Action:   "Open certificates",
			Target:   &Target{Kind: "node", Node: node, Tab: "certificates"},
			Subjects: []Subject{{Kind: "node", Node: node, Name: node}},
		})
	}
	return out
}

func pendingRestart(idx index) *Insight {
	var hits []pve.Resource
	for _, g := range idx.guests {
		if g.Pending && g.Template == 0 {
			hits = append(hits, g)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	in := &Insight{ID: "pending-restart", Severity: Action, Subjects: subjectsOf(hits)}
	if len(hits) == 1 {
		in.Title = guestName(hits[0]) + " needs a restart to apply changes"
		in.Detail = "Saved hardware or option changes aren't live until the guest restarts."
		in.Action = "Review changes"
		in.Target = guestTarget(hits[0], "hardware")
		return in
	}
	in.Title = fmt.Sprintf("%d guests need a restart to apply changes", len(hits))
	in.Detail = namesOf(hits) + ". Saved changes aren't live until each guest restarts."
	return in
}

func noBackupJob(notInJob []pve.CoverageGuest, idx index) *Insight {
	if notInJob == nil {
		return nil
	}
	var hits []pve.Resource
	running := false
	for _, c := range notInJob {
		g, ok := idx.byVMID[c.VMID]
		if !ok || g.Template != 0 {
			continue
		}
		hits = append(hits, g)
		if g.Status == "running" {
			running = true
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sev := Notice
	if running {
		sev = Action
	}
	title := guestName(hits[0]) + " isn't in any backup job"
	if len(hits) > 1 {
		title = fmt.Sprintf("%d guests aren't in any backup job", len(hits))
	}
	detail := "Nothing schedules a backup for it, so a disk failure or bad change can't be undone."
	if len(hits) > 1 {
		detail = namesOf(hits) + ". Nothing schedules a backup for them."
	}
	return &Insight{
		ID: "no-backup-job", Severity: sev, Title: title, Detail: detail,
		Action: "Review coverage", Target: &Target{Kind: "backups", Tab: "coverage"}, Subjects: subjectsOf(hits),
	}
}

func computeFailover(idx index) *Failover {
	var online []pve.Resource
	for _, n := range idx.nodes {
		if n.Status == "online" && n.MaxMem > 0 {
			online = append(online, n)
		}
	}
	if len(online) < 2 {
		return nil
	}
	free := map[string]int64{}
	for _, n := range online {
		f := n.MaxMem - n.Mem
		if f < 0 {
			f = 0
		}
		free[n.Node] = f
	}
	f := &Failover{}
	for _, n := range online {
		var req, haReq int64
		for _, g := range idx.guests {
			if g.Node != n.Node || g.Status != "running" || g.Template != 0 {
				continue
			}
			req += g.MaxMem
			if g.HAState != "" && g.HAState != "ignored" && g.HAState != "disabled" {
				haReq += g.MaxMem
			}
		}
		var avail int64
		for _, other := range online {
			if other.Node != n.Node {
				avail += free[other.Node]
			}
		}
		f.Nodes = append(f.Nodes, FailoverNode{
			Node: n.Node, Required: req, HARequired: haReq, Available: avail,
			Fits: req <= avail, HAFits: haReq <= avail,
		})
	}
	return f
}

func failoverInsight(f *Failover) *Insight {
	if f == nil {
		return nil
	}
	for _, n := range f.Nodes {
		if !n.HAFits {
			return &Insight{
				ID: "failover", Severity: Action,
				Title:  "If " + n.Node + " fails, HA can't restart all its guests",
				Detail: fmt.Sprintf("Its HA-managed guests need %s of memory; the other nodes have %s free.", Bytes(n.HARequired), Bytes(n.Available)),
				Target: &Target{Kind: "node", Node: n.Node},
			}
		}
	}
	for _, n := range f.Nodes {
		if !n.Fits {
			return &Insight{
				ID: "failover", Severity: Notice,
				Title:  "If " + n.Node + " fails, the others can't take all its running guests",
				Detail: fmt.Sprintf("They need %s of memory; the other nodes have %s free.", Bytes(n.Required), Bytes(n.Available)),
				Target: &Target{Kind: "node", Node: n.Node},
			}
		}
	}
	return nil
}

func oldSnapshots(snaps map[int][]pve.Snapshot, idx index, now time.Time) *Insight {
	if snaps == nil {
		return nil
	}
	type old struct {
		guest pve.Resource
		snap  pve.Snapshot
	}
	var list []old
	guests := map[int]pve.Resource{}
	cutoff := now.Add(-oldSnapshotAge).Unix()
	for _, g := range idx.guests {
		if g.Template != 0 {
			continue
		}
		for _, s := range snaps[g.VMID] {
			if s.Name == "current" || s.SnapTime == 0 || s.SnapTime > cutoff {
				continue
			}
			list = append(list, old{guest: g, snap: s})
			guests[g.VMID] = g
		}
	}
	if len(list) == 0 {
		return nil
	}
	sort.Slice(list, func(a, b int) bool { return list[a].snap.SnapTime < list[b].snap.SnapTime })
	oldest := list[0]
	age := int(now.Sub(time.Unix(oldest.snap.SnapTime, 0)).Hours() / 24)
	in := &Insight{
		ID: "old-snapshots", Severity: Notice,
		Title:  fmt.Sprintf("%s older than 30 days", plural(len(list), "snapshot is", "snapshots are")),
		Detail: fmt.Sprintf("Oldest: “%s” on %s, %d days. Old snapshots keep holding disk space and are easy to forget.", oldest.snap.Name, guestName(oldest.guest), age),
	}
	var hits []pve.Resource
	for _, g := range guests {
		hits = append(hits, g)
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].VMID < hits[b].VMID })
	in.Subjects = subjectsOf(hits)
	if len(hits) == 1 {
		in.Action = "Open snapshots"
		in.Target = guestTarget(hits[0], "snapshots")
	}
	return in
}

func longStopped(since map[int]int64, idx index, now time.Time) *Insight {
	if since == nil {
		return nil
	}
	cutoff := now.Add(-longStoppedAge).Unix()
	var hits []pve.Resource
	for _, g := range idx.guests {
		if g.Status != "stopped" || g.Template != 0 {
			continue
		}
		if t, ok := since[g.VMID]; ok && t > 0 && t <= cutoff {
			hits = append(hits, g)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	in := &Insight{ID: "long-stopped", Severity: Notice, Subjects: subjectsOf(hits)}
	if len(hits) == 1 {
		days := int(now.Sub(time.Unix(since[hits[0].VMID], 0)).Hours() / 24)
		in.Title = fmt.Sprintf("%s has been stopped for %d days", guestName(hits[0]), days)
		in.Detail = "If it's no longer needed, removing it frees its disks and backup space."
		in.Action = "Open guest"
		in.Target = guestTarget(hits[0], "")
		return in
	}
	in.Title = fmt.Sprintf("%d guests have been stopped for over 30 days", len(hits))
	in.Detail = namesOf(hits) + ". Removing ones you no longer need frees their disks and backup space."
	return in
}

// summarizeRuns describes the latest run on each node, counting each guest
// once (the window can hold two nights, or a job and a manual retry).
func summarizeRuns(recent, all []BackupJobRun) *BackupRun {
	if all == nil {
		return nil
	}
	latest := map[string]BackupJobRun{}
	for _, r := range recent {
		if cur, ok := latest[r.Node]; !ok || r.Start > cur.Start {
			latest[r.Node] = r
		}
	}
	sum := &BackupRun{}
	ok := map[int]bool{}
	failed := map[int]bool{}
	for _, r := range latest {
		if r.Start > sum.LastRun {
			sum.LastRun = r.Start
		}
		if r.Running {
			sum.Running = true
		}
		for _, id := range r.OK {
			ok[id] = true
		}
		for _, f := range r.Failed {
			failed[f.VMID] = true
		}
	}
	for id := range failed {
		if ok[id] {
			delete(failed, id)
		}
	}
	sum.OK = len(ok)
	sum.Failed = len(failed)
	return sum
}

func lastBackups(runs []BackupJobRun, idx index) map[int]int64 {
	out := map[int]int64{}
	for _, r := range runs {
		for _, id := range r.OK {
			if _, ok := idx.byVMID[id]; ok && r.Start > out[id] {
				out[id] = r.Start
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ---- vzdump log parsing -----------------------------------------------------

var (
	finishedRe = regexp.MustCompile(`Finished Backup of VM (\d+)\b`)
	failedRe   = regexp.MustCompile(`ERROR: Backup of VM (\d+) failed - (.*)$`)
)

// ParseBackupLog reads a vzdump task log and returns the guests it backed up
// and the ones it failed on, with the reason Proxmox logged.
func ParseBackupLog(lines []string) (ok []int, failed []BackupFailure) {
	seenOK := map[int]bool{}
	seenFail := map[int]bool{}
	for _, l := range lines {
		if m := failedRe.FindStringSubmatch(l); m != nil {
			id, _ := strconv.Atoi(m[1])
			if id > 0 && !seenFail[id] {
				seenFail[id] = true
				failed = append(failed, BackupFailure{VMID: id, Reason: strings.TrimSpace(m[2])})
			}
			continue
		}
		if m := finishedRe.FindStringSubmatch(l); m != nil {
			id, _ := strconv.Atoi(m[1])
			if id > 0 && !seenOK[id] {
				seenOK[id] = true
				ok = append(ok, id)
			}
		}
	}
	return ok, failed
}

// ---- text helpers ----------------------------------------------------------

func baseID(id string) string {
	if i := strings.IndexByte(id, ':'); i >= 0 {
		return id[:i]
	}
	return id
}

func subjectsOf(rs []pve.Resource) []Subject {
	out := make([]Subject, 0, len(rs))
	for _, r := range rs {
		out = append(out, guestSubject(r))
	}
	return out
}

func namesOf(rs []pve.Resource) string {
	names := make([]string, 0, len(rs))
	for _, r := range rs {
		names = append(names, guestName(r))
	}
	return listNames(names)
}

// listNames renders "a", "a and b", "a, b and c", "a, b, c and 4 more".
func listNames(names []string) string {
	switch n := len(names); {
	case n == 0:
		return ""
	case n == 1:
		return names[0]
	case n <= maxNamesInSummary:
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	default:
		return strings.Join(names[:maxNamesInSummary], ", ") + fmt.Sprintf(" and %d more", n-maxNamesInSummary)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Proxmox logged no reason."
	}
	s = strings.ToUpper(s[:1]) + s[1:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// Bytes renders a byte count the way the UI does (binary units, one decimal).
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	val := float64(n) / float64(div)
	suffix := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}[exp]
	if val >= 100 {
		return fmt.Sprintf("%.0f %s", val, suffix)
	}
	return fmt.Sprintf("%.1f %s", val, suffix)
}
