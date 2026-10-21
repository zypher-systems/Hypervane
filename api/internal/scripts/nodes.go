package scripts

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"hypervane/internal/pve"
)

func planCheckUpdates(cl *cluster) (*Plan, error) {
	nodes := cl.onlineNodes()
	if len(nodes) == 0 {
		return nil, inputErr("no nodes are online")
	}
	plan := &Plan{}
	for _, node := range nodes {
		node := node
		plan.Steps = append(plan.Steps, Step{
			ID:    "update-" + node,
			Label: "Refresh the package lists on " + node + " and count the updates",
			Group: node,
			run: func(ctx context.Context, x *Exec) error {
				upid, err := x.C.APTUpdate(ctx, node)
				if err != nil {
					return err
				}
				if err := x.Wait(ctx, node, upid); err != nil {
					return err
				}
				pkgs, err := x.C.APTUpdates(ctx, node)
				if err != nil {
					return err
				}
				switch len(pkgs) {
				case 0:
					x.Note("Up to date.")
				case 1:
					x.Note("1 update available.")
				default:
					x.Note(fmt.Sprintf("%d updates available.", len(pkgs)))
				}
				return nil
			},
		})
	}
	plan.Notes = append(plan.Notes, "Nothing is installed. Install updates from each node's Updates panel.")
	return plan, nil
}

func planShutdown(cl *cluster, p Params) (*Plan, error) {
	if _, err := cl.node(p.Node); err != nil {
		return nil, err
	}
	var running []pve.Resource
	for _, g := range cl.guests(p.Node) {
		if g.Status == "running" {
			running = append(running, g)
		}
	}
	if len(running) == 0 {
		return nil, inputErr("nothing is running on %s", p.Node)
	}
	force := on(p.Force, true)
	ids := vmids(running)
	plan := &Plan{Steps: []Step{{
		ID:    "stopall",
		Label: fmt.Sprintf("Shut down %s on %s", listGuests(running), p.Node),
		run: func(ctx context.Context, x *Exec) error {
			opts := map[string]any{"vms": ids, "timeout": 180, "force-stop": boolInt(force)}
			upid, err := x.C.NodeGuestAll(ctx, p.Node, "stopall", opts)
			if err != nil {
				return err
			}
			return x.Wait(ctx, p.Node, upid)
		},
	}}}
	plan.Confirm = fmt.Sprintf("%d running guest%s on %s shut down.", len(running), plural(len(running)), p.Node)
	if force {
		plan.Confirm += " Any still running after 3 minutes are turned off."
	}
	for _, g := range running {
		if g.HAState != "" {
			plan.Notes = append(plan.Notes, "HA-managed guests are stopped through HA and stay stopped until you start them again.")
			break
		}
	}
	plan.Notes = append(plan.Notes, "Start them again with the Start a node's guests recipe.")
	return plan, nil
}

func planStart(ctx context.Context, c *pve.Client, cl *cluster, p Params) (*Plan, error) {
	if _, err := cl.node(p.Node); err != nil {
		return nil, err
	}
	onbootOnly := on(p.OnbootOnly, true)
	var stopped []pve.Resource
	for _, g := range cl.guests(p.Node) {
		if g.Status != "stopped" {
			continue
		}
		if onbootOnly {
			cfg, err := c.GuestConfig(ctx, g.Node, g.Type, g.VMID)
			if err != nil {
				if pve.IsForbidden(err) {
					continue
				}
				return nil, err
			}
			if str(cfg["onboot"]) != "1" {
				continue
			}
		}
		stopped = append(stopped, g)
	}
	if len(stopped) == 0 {
		if onbootOnly {
			return nil, inputErr("no stopped guests on %s are set to start at boot", p.Node)
		}
		return nil, inputErr("no guests are stopped on %s", p.Node)
	}
	ids := vmids(stopped)
	return &Plan{Steps: []Step{{
		ID:    "startall",
		Label: fmt.Sprintf("Start %s on %s, in their start-up order", listGuests(stopped), p.Node),
		run: func(ctx context.Context, x *Exec) error {
			// force: the list is already filtered, so start exactly these.
			upid, err := x.C.NodeGuestAll(ctx, p.Node, "startall", map[string]any{"vms": ids, "force": 1})
			if err != nil {
				return err
			}
			return x.Wait(ctx, p.Node, upid)
		},
	}}}, nil
}

func planEvacuate(cl *cluster, p Params) (*Plan, error) {
	if _, err := cl.node(p.Node); err != nil {
		return nil, err
	}
	if p.Target == "" {
		return nil, inputErr("choose where to move the guests")
	}
	if p.Target == p.Node {
		return nil, inputErr("choose a different node to move the guests to")
	}
	if _, err := cl.node(p.Target); err != nil {
		return nil, err
	}
	guests := cl.guests(p.Node)
	if len(guests) == 0 {
		return nil, inputErr("%s has no guests to move", p.Node)
	}
	ids := vmids(guests)
	plan := &Plan{Steps: []Step{{
		ID:    "migrateall",
		Label: fmt.Sprintf("Move %s from %s to %s", listGuests(guests), p.Node, p.Target),
		run: func(ctx context.Context, x *Exec) error {
			upid, err := x.C.NodeGuestAll(ctx, p.Node, "migrateall", map[string]any{
				"target": p.Target, "vms": ids, "with-local-disks": 1, "maxworkers": 2,
			})
			if err != nil {
				return err
			}
			return x.Wait(ctx, p.Node, upid)
		},
	}}}
	plan.Confirm = fmt.Sprintf("%d guest%s move from %s to %s, two at a time. Running VMs move live; running containers restart on %s.", len(guests), plural(len(guests)), p.Node, p.Target, p.Target)
	plan.Notes = append(plan.Notes,
		"Disks on local storage are copied across, which takes a while; the target needs a storage with the same name.",
		"Guests with passed-through devices can't move and are listed as failed in the task log.")
	return plan, nil
}

func vmids(gs []pve.Resource) string {
	ids := make([]string, len(gs))
	for i, g := range gs {
		ids[i] = strconv.Itoa(g.VMID)
	}
	return strings.Join(ids, ",")
}

// listGuests names up to six guests and counts the rest.
func listGuests(gs []pve.Resource) string {
	const show = 6
	var names []string
	for i, g := range gs {
		if i == show {
			names[len(names)-1] += fmt.Sprintf(" and %d more", len(gs)-show)
			break
		}
		names = append(names, label(g))
	}
	return strings.Join(names, ", ")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
