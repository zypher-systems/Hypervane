package scripts

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hypervane/internal/pve"
)

// ---- guest agent -----------------------------------------------------------

func planEnableAgent(ctx context.Context, c *pve.Client, cl *cluster, p Params) (*Plan, error) {
	g, err := cl.guest(p.VMID)
	if err != nil {
		return nil, err
	}
	if g.Type != "qemu" {
		return nil, inputErr("the QEMU guest agent is for VMs; containers don't need it")
	}
	cfg, err := c.GuestConfig(ctx, g.Node, "qemu", g.VMID)
	if err != nil {
		return nil, err
	}
	value, already := agentOn(str(cfg["agent"]))
	if already {
		return nil, inputErr("the guest agent is already on for %s", label(g))
	}
	plan := &Plan{VMID: g.VMID, Steps: []Step{{
		ID:    "agent",
		Label: "Turn on the guest agent for " + label(g),
		run: func(ctx context.Context, x *Exec) error {
			return x.C.UpdateGuestConfig(ctx, g.Node, "qemu", g.VMID, map[string]any{"agent": value}, nil, "")
		},
	}}}
	if g.Status == "running" {
		plan.Notes = append(plan.Notes, "It takes effect after the VM is shut down and started again; a reboot from inside the guest isn't enough.")
	}
	plan.Notes = append(plan.Notes, "The guest needs the qemu-guest-agent package installed and running.")
	return plan, nil
}

// agentOn turns an agent setting on, keeping its other options, and reports
// whether it was on already.
func agentOn(cur string) (string, bool) {
	if cur == "" {
		return "1", false
	}
	parts := strings.Split(cur, ",")
	on := false
	for i, part := range parts {
		switch {
		case part == "1" || part == "enabled=1":
			on = true
		case part == "0":
			parts[i] = "1"
		case part == "enabled=0":
			parts[i] = "enabled=1"
		}
	}
	return strings.Join(parts, ","), on
}

// ---- convert to template ---------------------------------------------------

func planConvert(cl *cluster, p Params) (*Plan, error) {
	g, err := cl.guest(p.VMID)
	if err != nil {
		return nil, err
	}
	if g.Template != 0 {
		return nil, inputErr("%s is already a template", label(g))
	}
	if g.Status != "stopped" {
		return nil, inputErr("shut down %s first; only a stopped guest can become a template", label(g))
	}
	return &Plan{
		VMID:    g.VMID,
		Confirm: fmt.Sprintf("%s becomes a template. Templates can't be started or turned back into a guest; you clone them instead.", label(g)),
		Steps: []Step{{
			ID:    "template",
			Label: "Convert " + label(g) + " to a template",
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.ConvertTemplate(ctx, g.Node, g.Type, g.VMID)
				if err != nil {
					return err
				}
				return x.Wait(ctx, g.Node, upid)
			},
		}},
	}, nil
}

// ---- snapshots -------------------------------------------------------------

func planSnapshotNode(cl *cluster, p Params) (*Plan, error) {
	if _, err := cl.node(p.Node); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(p.SnapName)
	if name == "" {
		name = "hv-" + time.Now().Format("20060102-1504")
	}
	if !snapNameRe.MatchString(name) {
		return nil, inputErr("snapshot names start with a letter and use letters, digits, - and _ (2 to 40 characters)")
	}
	var guests []pve.Resource
	for _, g := range cl.guests(p.Node) {
		if !p.RunningOnly || g.Status == "running" {
			guests = append(guests, g)
		}
	}
	if len(guests) == 0 {
		if p.RunningOnly {
			return nil, inputErr("no guests are running on %s", p.Node)
		}
		return nil, inputErr("%s has no guests", p.Node)
	}
	plan := &Plan{}
	for _, g := range guests {
		g := g
		plan.Steps = append(plan.Steps, Step{
			ID:    "snapshot-" + strconv.Itoa(g.VMID),
			Label: fmt.Sprintf("Snapshot %s as %s", label(g), name),
			Group: strconv.Itoa(g.VMID),
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.CreateSnapshot(ctx, g.Node, g.Type, g.VMID, pve.SnapshotRequest{
					Name: name, Description: "Taken by Hypervane",
				})
				if err != nil {
					return err
				}
				return x.Wait(ctx, g.Node, upid)
			},
		})
	}
	plan.Notes = append(plan.Notes, "Guests on storage without snapshot support (raw disks on a directory, for example) fail on their own; the rest carry on.")
	return plan, nil
}

type oldSnapshot struct {
	guest pve.Resource
	snap  pve.Snapshot
}

func planSnapshotCleanup(ctx context.Context, c *pve.Client, cl *cluster, p Params) (*Plan, error) {
	if p.Node != "" {
		if _, err := cl.node(p.Node); err != nil {
			return nil, err
		}
	}
	days := nonzero(p.Days, 30)
	if err := between(days, 1, 3650, "days"); err != nil {
		return nil, err
	}
	onlyHV := on(p.OnlyHypervane, true)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	online := map[string]bool{}
	for _, n := range cl.onlineNodes() {
		online[n] = true
	}
	var guests []pve.Resource
	for _, g := range cl.guests(p.Node) {
		if online[g.Node] {
			guests = append(guests, g)
		}
	}

	// Snapshots are listed per guest; read a few at a time.
	var (
		mu    sync.Mutex
		found []oldSnapshot
		first error
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 8)
	)
	for _, g := range guests {
		g := g
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			snaps, err := c.Snapshots(ctx, g.Node, g.Type, g.VMID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if first == nil && !pve.IsForbidden(err) {
					first = err
				}
				return
			}
			for _, s := range snaps {
				if s.Name == "current" || s.SnapTime == 0 || s.SnapTime > cutoff {
					continue
				}
				if onlyHV && !strings.HasPrefix(s.Name, "hv-") {
					continue
				}
				found = append(found, oldSnapshot{guest: g, snap: s})
			}
		}()
	}
	wg.Wait()
	if first != nil {
		return nil, first
	}
	if len(found) == 0 {
		what := "snapshots"
		if onlyHV {
			what = "Hypervane snapshots"
		}
		return nil, inputErr("no %s are older than %d days", what, days)
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].guest.VMID != found[j].guest.VMID {
			return found[i].guest.VMID < found[j].guest.VMID
		}
		return found[i].snap.SnapTime < found[j].snap.SnapTime
	})
	plan := &Plan{}
	now := time.Now().Unix()
	for _, f := range found {
		f := f
		age := (now - f.snap.SnapTime) / 86400
		plan.Steps = append(plan.Steps, Step{
			ID:    fmt.Sprintf("delete-%d-%s", f.guest.VMID, f.snap.Name),
			Label: fmt.Sprintf("Delete snapshot %s of %s (%d days old)", f.snap.Name, label(f.guest), age),
			Group: strconv.Itoa(f.guest.VMID),
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.DeleteSnapshot(ctx, f.guest.Node, f.guest.Type, f.snap.Name, f.guest.VMID)
				if err != nil {
					return err
				}
				return x.Wait(ctx, f.guest.Node, upid)
			},
		})
	}
	plan.Confirm = fmt.Sprintf("%d snapshot%s will be deleted. The guests keep running and their current state is kept; you can no longer roll back to these points.", len(found), plural(len(found)))
	return plan, nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
