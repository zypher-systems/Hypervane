package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"hypervane/internal/insights"
	"hypervane/internal/pve"
)

// Cache lifetimes for the slower insight sources. Everything here is read-only;
// the lifetimes only bound how often Hypervane asks Proxmox.
const (
	ttlActiveTasks  = 15 * time.Second
	ttlHA           = 30 * time.Second
	ttlBackupRuns   = 2 * time.Minute
	ttlNotInJob     = 5 * time.Minute
	ttlSnapshots    = 10 * time.Minute
	ttlStoppedSince = 10 * time.Minute
	ttlCerts        = time.Hour

	maxSnapshotGuests = 400
	maxStoppedGuests  = 150
	insightWorkers    = 4
)

type ttlCache struct {
	mu      sync.Mutex
	entries map[string]ttlEntry
}

type ttlEntry struct {
	at    time.Time
	value any
}

func (c *ttlCache) get(key string, ttl time.Duration) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.at) > ttl {
		return nil, false
	}
	return e.value, true
}

func (c *ttlCache) put(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]ttlEntry{}
	}
	c.entries[key] = ttlEntry{at: time.Now(), value: v}
}

// cachedFetch returns a cached value for key, or runs fetch and caches a
// successful result. Failures are not cached so the next view retries.
func cachedFetch[T any](c *ttlCache, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	if v, ok := c.get(key, ttl); ok {
		if t, ok := v.(T); ok {
			return t, nil
		}
	}
	v, err := fetch()
	if err == nil {
		c.put(key, v)
	}
	return v, err
}

// handleInsights answers GET /api/insights: the short list of things worth a
// decision on the active connection, limited to what the viewer may see.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	connID := s.requestConnectionID(r)
	base := s.ClientByID(connID)
	if base == nil {
		base = c
	}

	var status []pve.ClusterStatus
	var all []pve.Resource
	if s.poller != nil {
		status, all, _ = s.poller.CachedResources(r.Context(), connID)
	}
	if len(all) == 0 {
		var err error
		if status, err = c.ClusterStatus(r.Context()); err != nil {
			writePVEError(w, err)
			return
		}
		if all, err = c.ClusterResources(r.Context()); err != nil {
			writePVEError(w, err)
			return
		}
	}
	visible := all
	acl, filtered := s.requestACL(r)
	if filtered {
		visible = pve.FilterResources(all, acl)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	g := insightGatherer{cache: &s.insightCache, conn: connID, c: base, all: all}
	in := insights.Input{Now: time.Now(), Status: status, Resources: visible}

	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		unavailable []string
	)
	step := func(label string, run func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(); err != nil {
				mu.Lock()
				unavailable = append(unavailable, label)
				mu.Unlock()
			}
		}()
	}
	set := func(apply func()) {
		mu.Lock()
		apply()
		mu.Unlock()
	}
	step("running tasks", func() error {
		v, err := g.activeTasks(ctx)
		if err == nil {
			set(func() { in.ActiveTasks = v })
		}
		return err
	})
	step("HA status", func() error {
		v, err := cachedFetch(g.cache, g.key("ha"), ttlHA, func() ([]pve.HACurrent, error) { return g.c.HACurrent(ctx) })
		if err == nil {
			set(func() { in.HA = v })
		}
		return err
	})
	step("backup history", func() error {
		v, err := g.backupRuns(ctx)
		if err == nil {
			set(func() { in.BackupRuns = v })
		}
		return err
	})
	step("backup jobs", func() error {
		v, err := cachedFetch(g.cache, g.key("not-in-job"), ttlNotInJob, func() ([]pve.CoverageGuest, error) {
			out, err := g.c.NotBackedUp(ctx)
			if err != nil {
				out, err = coverageFromJobsCtx(ctx, g.c, g.all)
			}
			if out == nil && err == nil {
				out = []pve.CoverageGuest{}
			}
			return out, err
		})
		if err == nil {
			set(func() { in.NotInJob = v })
		}
		return err
	})
	step("snapshots", func() error {
		v, err := g.snapshots(ctx)
		if err == nil {
			set(func() { in.Snapshots = v })
		}
		return err
	})
	step("stop history", func() error {
		v, err := g.stoppedSince(ctx)
		if err == nil {
			set(func() { in.StoppedSince = v })
		}
		return err
	})
	step("certificates", func() error {
		v, err := g.certificates(ctx)
		if err == nil {
			set(func() { in.Certs = v })
		}
		return err
	})
	wg.Wait()

	report := insights.Build(in)
	// Failover arithmetic needs every running guest; a viewer who sees only some
	// would get numbers that look reassuring and are not.
	if filtered && countGuests(visible) < countGuests(all) {
		report.Failover = nil
		kept := report.Insights[:0]
		for _, i := range report.Insights {
			if i.ID != "failover" {
				kept = append(kept, i)
			}
		}
		report.Insights = kept
	}
	sort.Strings(unavailable)
	report.Unavailable = unavailable
	writeJSON(w, http.StatusOK, report)
}

func countGuests(rs []pve.Resource) int {
	n := 0
	for _, r := range rs {
		if r.Type == "qemu" || r.Type == "lxc" {
			n++
		}
	}
	return n
}

type insightGatherer struct {
	cache *ttlCache
	conn  string
	c     *pve.Client
	all   []pve.Resource
}

func (g insightGatherer) key(name string) string { return g.conn + ":" + name }

func (g insightGatherer) onlineNodes() []string {
	var out []string
	for _, r := range g.all {
		if r.Type == "node" && r.Status == "online" && r.Node != "" {
			out = append(out, r.Node)
		}
	}
	sort.Strings(out)
	return out
}

// perNode runs fetch for every online node and fails only if all of them fail.
func perNode[T any](g insightGatherer, fetch func(node string) (T, error)) (map[string]T, error) {
	nodes := g.onlineNodes()
	out := make(map[string]T, len(nodes))
	var firstErr error
	for _, n := range nodes {
		v, err := fetch(n)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[n] = v
	}
	if len(nodes) > 0 && len(out) == 0 {
		return nil, firstErr
	}
	return out, nil
}

func (g insightGatherer) activeTasks(ctx context.Context) ([]pve.Task, error) {
	return cachedFetch(g.cache, g.key("active-tasks"), ttlActiveTasks, func() ([]pve.Task, error) {
		byNode, err := perNode(g, func(node string) ([]pve.Task, error) {
			return g.c.NodeTasks(ctx, node, pve.NodeTaskQuery{Source: "active", Limit: 500})
		})
		if err != nil {
			return nil, err
		}
		out := []pve.Task{}
		for _, ts := range byNode {
			out = append(out, ts...)
		}
		return out, nil
	})
}

// backupRuns reads each node's recent vzdump tasks and parses their logs.
func (g insightGatherer) backupRuns(ctx context.Context) ([]insights.BackupJobRun, error) {
	return cachedFetch(g.cache, g.key("backup-runs"), ttlBackupRuns, func() ([]insights.BackupJobRun, error) {
		cutoff := time.Now().Add(-36 * time.Hour).Unix()
		byNode, err := perNode(g, func(node string) ([]insights.BackupJobRun, error) {
			tasks, err := g.c.NodeTasks(ctx, node, pve.NodeTaskQuery{TypeFilter: "vzdump", Limit: 10})
			if err != nil {
				return nil, err
			}
			var runs []insights.BackupJobRun
			for _, t := range tasks {
				if t.Type != "vzdump" || t.StartTime < cutoff {
					continue
				}
				run := insights.BackupJobRun{Node: node, UPID: t.UPID, Start: t.StartTime, End: t.EndTime, Status: t.Status}
				if t.EndTime == 0 && t.Status == "" {
					run.Running = true
					runs = append(runs, run)
					continue
				}
				lines, _, err := g.c.TaskLogTail(ctx, node, t.UPID, 5000)
				if err == nil {
					text := make([]string, len(lines))
					for i, l := range lines {
						text[i] = l.T
					}
					run.OK, run.Failed = insights.ParseBackupLog(text)
				}
				runs = append(runs, run)
			}
			return runs, nil
		})
		if err != nil {
			return nil, err
		}
		out := []insights.BackupJobRun{}
		for _, rs := range byNode {
			out = append(out, rs...)
		}
		return out, nil
	})
}

type guestRef struct {
	node, kind string
	vmid       int
}

func (g insightGatherer) guests(match func(pve.Resource) bool, limit int) []guestRef {
	var out []guestRef
	for _, r := range g.all {
		if (r.Type != "qemu" && r.Type != "lxc") || r.Template != 0 || r.Node == "" || !match(r) {
			continue
		}
		out = append(out, guestRef{node: r.Node, kind: r.Type, vmid: r.VMID})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// forGuests fetches per guest with a small worker pool and fails when most
// lookups fail (a permission problem, not one missing guest).
func forGuests[T any](ctx context.Context, refs []guestRef, fetch func(guestRef) (T, bool, error)) (map[int]T, error) {
	out := make(map[int]T, len(refs))
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		failed int
		first  error
	)
	jobs := make(chan guestRef)
	for i := 0; i < insightWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ref := range jobs {
				v, ok, err := fetch(ref)
				mu.Lock()
				if err != nil {
					failed++
					if first == nil {
						first = err
					}
				} else if ok {
					out[ref.vmid] = v
				}
				mu.Unlock()
			}
		}()
	}
	for _, ref := range refs {
		if ctx.Err() != nil {
			break
		}
		jobs <- ref
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(refs) > 0 && failed*2 > len(refs) {
		if first == nil {
			first = errors.New("lookups failed")
		}
		return nil, first
	}
	return out, nil
}

func (g insightGatherer) snapshots(ctx context.Context) (map[int][]pve.Snapshot, error) {
	return cachedFetch(g.cache, g.key("snapshots"), ttlSnapshots, func() (map[int][]pve.Snapshot, error) {
		refs := g.guests(func(pve.Resource) bool { return true }, maxSnapshotGuests)
		return forGuests(ctx, refs, func(ref guestRef) ([]pve.Snapshot, bool, error) {
			v, err := g.c.Snapshots(ctx, ref.node, ref.kind, ref.vmid)
			return v, err == nil, err
		})
	})
}

var stopTaskTypes = map[string]bool{"qmstop": true, "qmshutdown": true, "vzstop": true, "vzshutdown": true}

// stoppedSince finds when each stopped guest was last stopped, from its task
// history. Guests with no stop task on record are left out (unknown).
func (g insightGatherer) stoppedSince(ctx context.Context) (map[int]int64, error) {
	return cachedFetch(g.cache, g.key("stopped-since"), ttlStoppedSince, func() (map[int]int64, error) {
		refs := g.guests(func(r pve.Resource) bool { return r.Status == "stopped" }, maxStoppedGuests)
		return forGuests(ctx, refs, func(ref guestRef) (int64, bool, error) {
			tasks, err := g.c.NodeTasks(ctx, ref.node, pve.NodeTaskQuery{VMID: strconv.Itoa(ref.vmid), Limit: 50})
			if err != nil {
				return 0, false, err
			}
			var at int64
			for _, t := range tasks {
				if !stopTaskTypes[t.Type] {
					continue
				}
				ts := t.EndTime
				if ts == 0 {
					ts = t.StartTime
				}
				if ts > at {
					at = ts
				}
			}
			return at, at > 0, nil
		})
	})
}

func (g insightGatherer) certificates(ctx context.Context) (map[string][]pve.CertInfo, error) {
	return cachedFetch(g.cache, g.key("certificates"), ttlCerts, func() (map[string][]pve.CertInfo, error) {
		return perNode(g, func(node string) ([]pve.CertInfo, error) {
			return g.c.NodeCertificates(ctx, node)
		})
	})
}
