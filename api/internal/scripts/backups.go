package scripts

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"hypervane/internal/pve"
)

var scheduleRe = regexp.MustCompile(`^[a-z0-9:,.*/ -]{1,64}$`)

func checkMode(mode string) (string, error) {
	switch mode {
	case "":
		return "snapshot", nil
	case "snapshot", "suspend", "stop":
		return mode, nil
	}
	return "", inputErr("mode must be snapshot, suspend or stop")
}

// unprotected lists guests no backup job covers, optionally on one node.
func unprotected(ctx context.Context, c *pve.Client, cl *cluster, node string) ([]pve.Resource, error) {
	missing, err := c.NotBackedUp(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int]pve.Resource{}
	for _, g := range cl.guests(node) {
		byID[g.VMID] = g
	}
	var out []pve.Resource
	for _, m := range missing {
		if g, ok := byID[m.VMID]; ok {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VMID < out[j].VMID })
	return out, nil
}

func storageByName(list []pve.Storage, name string) (pve.Storage, bool) {
	for _, st := range list {
		if st.Storage == name {
			return st, true
		}
	}
	return pve.Storage{}, false
}

// compression is zstd for file backups; Proxmox Backup Server compresses and
// deduplicates on its own.
func compression(st pve.Storage) string {
	if st.Type == "pbs" {
		return ""
	}
	return "zstd"
}

func planBackupNow(ctx context.Context, c *pve.Client, cl *cluster, p Params) (*Plan, error) {
	if p.Node != "" {
		if _, err := cl.node(p.Node); err != nil {
			return nil, err
		}
	}
	mode, err := checkMode(p.Mode)
	if err != nil {
		return nil, err
	}
	store, err := ResolveBackupStore(cl.storages, p.Node, p.Storage)
	if err != nil {
		return nil, &InputError{Msg: err.Error()}
	}
	st, _ := storageByName(cl.storages, store)
	guests, err := unprotected(ctx, c, cl, p.Node)
	if err != nil {
		return nil, err
	}
	if len(guests) == 0 {
		if p.Node != "" {
			return nil, inputErr("every guest on %s is covered by a backup job", p.Node)
		}
		return nil, inputErr("every guest is covered by a backup job")
	}
	byNode := map[string][]pve.Resource{}
	var nodes []string
	for _, g := range guests {
		if _, ok := byNode[g.Node]; !ok {
			nodes = append(nodes, g.Node)
		}
		byNode[g.Node] = append(byNode[g.Node], g)
	}
	sort.Strings(nodes)
	plan := &Plan{}
	for _, node := range nodes {
		if !OnNode(st, node) {
			return nil, inputErr("storage %s isn't available on %s, which has unprotected guests; pick a shared storage or one node", store, node)
		}
		node, gs := node, byNode[node]
		ids := make([]int, len(gs))
		for i, g := range gs {
			ids[i] = g.VMID
		}
		plan.Steps = append(plan.Steps, Step{
			ID:    "backup-" + node,
			Label: fmt.Sprintf("Back up %s on %s to %s (%s mode)", listGuests(gs), node, store, mode),
			Group: node,
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.VZDump(ctx, node, pve.VZDumpRequest{VMIDs: ids, Storage: store, Mode: mode, Compress: compression(st)})
				if err != nil {
					return err
				}
				return x.Wait(ctx, node, upid)
			},
		})
	}
	if mode == "stop" {
		plan.Confirm = fmt.Sprintf("Stop mode shuts down each of the %d guests while it's backed up, then starts it again.", len(guests))
	}
	plan.Notes = append(plan.Notes, "This is a one-off backup. Schedule backups for unprotected guests (Pro) keeps them covered.")
	return plan, nil
}

func planBackupJob(ctx context.Context, c *pve.Client, cl *cluster, p Params) (*Plan, error) {
	mode, err := checkMode(p.Mode)
	if err != nil {
		return nil, err
	}
	schedule := strings.TrimSpace(p.Schedule)
	if schedule == "" {
		schedule = "21:00"
	}
	if !scheduleRe.MatchString(schedule) {
		return nil, inputErr("schedule %q isn't a Proxmox calendar event, like 21:00 or sat 02:00", schedule)
	}
	keep := nonzero(p.Keep, 7)
	if err := between(keep, 1, 366, "days to keep"); err != nil {
		return nil, err
	}
	store, err := ResolveBackupStore(cl.storages, "", p.Storage)
	if err != nil {
		return nil, &InputError{Msg: err.Error()}
	}
	st, _ := storageByName(cl.storages, store)
	guests, err := unprotected(ctx, c, cl, "")
	if err != nil {
		return nil, err
	}
	if len(guests) == 0 {
		return nil, inputErr("every guest is covered by a backup job")
	}
	for _, g := range guests {
		if !OnNode(st, g.Node) {
			return nil, inputErr("storage %s isn't available on %s, where %s runs; pick a shared storage", store, g.Node, label(g))
		}
	}
	prune := fmt.Sprintf("keep-daily=%d,keep-weekly=4,keep-monthly=3", keep)
	enabled := true
	req := pve.BackupJobRequest{
		Enabled:      &enabled,
		Schedule:     schedule,
		Storage:      store,
		Mode:         mode,
		Compress:     compression(st),
		VMID:         vmids(guests),
		PruneBackups: prune,
		Comment:      "Hypervane: guests without backups, " + time.Now().Format("2006-01-02"),
	}
	plan := &Plan{Steps: []Step{{
		ID:    "job",
		Label: fmt.Sprintf("Create a backup job for %s at %s to %s, keeping %d daily, 4 weekly and 3 monthly backups", listGuests(guests), schedule, store, keep),
		run: func(ctx context.Context, x *Exec) error {
			job, err := x.C.CreateBackupJob(ctx, req)
			if err != nil {
				return err
			}
			if job.ID != "" {
				x.Note("Job " + job.ID + " created.")
			}
			return nil
		},
	}}}
	plan.Notes = append(plan.Notes, "Guests you add later aren't in this job. Run this again for them, or edit the job under Datacenter, Backups.")
	return plan, nil
}
