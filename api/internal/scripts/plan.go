package scripts

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"hypervane/internal/pve"
)

// Params are the inputs from a recipe's form. Each recipe reads the ones it
// lists as fields; pointers tell "not sent" apart from false or zero.
type Params struct {
	Node          string `json:"node"`
	Target        string `json:"target"`
	Storage       string `json:"storage"`
	VMID          int    `json:"vmid"`
	NewID         int    `json:"newId"`
	Name          string `json:"name"`
	Cores         int    `json:"cores"`
	Memory        int    `json:"memory"`
	DiskGB        int    `json:"diskGb"`
	Bridge        string `json:"bridge"`
	VLAN          int    `json:"vlan"`
	Agent         *bool  `json:"agent"`
	User          string `json:"user"`
	SSHKeys       string `json:"sshKeys"`
	IP            string `json:"ip"`
	Gateway       string `json:"gateway"`
	Full          *bool  `json:"full"`
	Start         *bool  `json:"start"`
	Count         int    `json:"count"`
	Source        string `json:"source"`
	RunningOnly   bool   `json:"runningOnly"`
	SnapName      string `json:"snapName"`
	Days          int    `json:"days"`
	OnlyHypervane *bool  `json:"onlyHypervane"`
	Mode          string `json:"mode"`
	Schedule      string `json:"schedule"`
	Keep          int    `json:"keep"`
	Force         *bool  `json:"force"`
	OnbootOnly    *bool  `json:"onbootOnly"`
}

func on(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// Plan is a recipe's exact steps for these inputs, worked out before anything
// changes. The preview shows it and running it executes the same steps.
type Plan struct {
	Recipe Recipe   `json:"-"`
	Steps  []Step   `json:"steps"`
	Notes  []string `json:"notes,omitempty"`
	// Confirm is set for recipes that stop, delete or move things; the page
	// asks before running.
	Confirm string `json:"confirm,omitempty"`
	VMID    int    `json:"vmid,omitempty"`
}

// Step is one action. Steps run in order. A failed step stops the job,
// unless it has a Group (one guest or node in a batch): then only the rest of
// that group is skipped and the other groups still run. Optional steps
// (tidying up) never fail the job.
type Step struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Group    string `json:"-"`
	Optional bool   `json:"-"`
	// OnFail explains what a failure leaves behind.
	OnFail string                                   `json:"-"`
	run    func(ctx context.Context, x *Exec) error `json:"-"`
}

// InputError is a problem with what the user entered or selected; the page
// shows it next to the form rather than as a Proxmox failure.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

func inputErr(format string, args ...any) error {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}

// IsInputError reports whether err came from validating the inputs.
func IsInputError(err error) bool {
	var ie *InputError
	return errors.As(err, &ie)
}

// Prepare validates the inputs against the cluster and plans the steps. It
// only reads from Proxmox, so it also serves the preview.
func Prepare(ctx context.Context, c *pve.Client, im *Images, r Recipe, p Params) (*Plan, error) {
	cl, err := loadCluster(ctx, c)
	if err != nil {
		return nil, err
	}
	var plan *Plan
	switch r.Kind {
	case KindCloudTemplate:
		plan, err = planCloud(ctx, c, im, cl, r, p)
	case KindRefreshTemplate:
		plan, err = planRefresh(ctx, c, im, cl, p)
	case KindDeploy:
		plan, err = planDeploy(ctx, c, cl, p, false)
	case KindDeployMany:
		plan, err = planDeploy(ctx, c, cl, p, true)
	case KindEnableAgent:
		plan, err = planEnableAgent(ctx, c, cl, p)
	case KindConvertTemplate:
		plan, err = planConvert(cl, p)
	case KindCheckUpdates:
		plan, err = planCheckUpdates(cl)
	case KindShutdownNode:
		plan, err = planShutdown(cl, p)
	case KindStartNode:
		plan, err = planStart(ctx, c, cl, p)
	case KindEvacuateNode:
		plan, err = planEvacuate(cl, p)
	case KindBackupNow:
		plan, err = planBackupNow(ctx, c, cl, p)
	case KindBackupJob:
		plan, err = planBackupJob(ctx, c, cl, p)
	case KindSnapshotNode:
		plan, err = planSnapshotNode(cl, p)
	case KindSnapshotCleanup:
		plan, err = planSnapshotCleanup(ctx, c, cl, p)
	default:
		return nil, inputErr("unknown recipe kind %q", r.Kind)
	}
	if err != nil {
		return nil, err
	}
	plan.Recipe = r
	return plan, nil
}

// cluster is one read of the cluster the planners share.
type cluster struct {
	resources []pve.Resource
	storages  []pve.Storage
}

func loadCluster(ctx context.Context, c *pve.Client) (*cluster, error) {
	res, err := c.ClusterResources(ctx)
	if err != nil {
		return nil, err
	}
	st, err := c.Storages(ctx)
	if err != nil {
		return nil, err
	}
	return &cluster{resources: res, storages: st}, nil
}

func (cl *cluster) node(name string) (pve.Resource, error) {
	if strings.TrimSpace(name) == "" {
		return pve.Resource{}, inputErr("choose a node")
	}
	for _, r := range cl.resources {
		if r.Type == "node" && r.Node == name {
			if r.Status != "" && r.Status != "online" {
				return r, inputErr("node %s is offline", name)
			}
			return r, nil
		}
	}
	return pve.Resource{}, inputErr("node %s isn't in this cluster", name)
}

func (cl *cluster) onlineNodes() []string {
	var out []string
	for _, r := range cl.resources {
		if r.Type == "node" && (r.Status == "" || r.Status == "online") {
			out = append(out, r.Node)
		}
	}
	return out
}

func (cl *cluster) guest(vmid int) (pve.Resource, error) {
	if vmid <= 0 {
		return pve.Resource{}, inputErr("choose a guest")
	}
	for _, r := range cl.resources {
		if (r.Type == "qemu" || r.Type == "lxc") && r.VMID == vmid {
			return r, nil
		}
	}
	return pve.Resource{}, inputErr("guest %d doesn't exist or isn't visible to you", vmid)
}

// guests lists guests, optionally on one node, never templates.
func (cl *cluster) guests(node string) []pve.Resource {
	var out []pve.Resource
	for _, r := range cl.resources {
		if (r.Type == "qemu" || r.Type == "lxc") && r.Template == 0 && (node == "" || r.Node == node) {
			out = append(out, r)
		}
	}
	return out
}

func (cl *cluster) vmidUsed(id int) bool {
	for _, r := range cl.resources {
		if (r.Type == "qemu" || r.Type == "lxc") && r.VMID == id {
			return true
		}
	}
	return false
}

// freeIDs returns n unused VMIDs from Proxmox's next free ID upwards.
func (cl *cluster) freeIDs(ctx context.Context, c *pve.Client, n int) ([]int, error) {
	next, err := c.NextID(ctx)
	if err != nil {
		return nil, err
	}
	var out []int
	for id := next; len(out) < n && id < 1_000_000_000; id++ {
		if !cl.vmidUsed(id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func label(g pve.Resource) string {
	if g.Name != "" {
		return fmt.Sprintf("%d (%s)", g.VMID, g.Name)
	}
	return fmt.Sprint(g.VMID)
}

// ---- input checks ----------------------------------------------------------

var (
	dnsNameRe  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
	snapNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{1,39}$`)
	bridgeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,14}$`)
	userRe     = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}$`)
	cidrRe     = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}$`)
	ipRe       = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}$`)
)

func checkName(name string) error {
	if !dnsNameRe.MatchString(name) || len(name) > 63 {
		return inputErr("name %q isn't valid: use letters, digits, dots and dashes", name)
	}
	return nil
}

func checkSSHKeys(keys string) error {
	for _, line := range strings.Split(keys, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "ssh-") && !strings.HasPrefix(line, "ecdsa-") && !strings.HasPrefix(line, "sk-") {
			return inputErr("each line of SSH keys must be a public key starting with ssh-, ecdsa- or sk-")
		}
	}
	return nil
}

func checkUser(u string) error {
	if u != "" && !userRe.MatchString(u) {
		return inputErr("user %q isn't a valid Linux user name", u)
	}
	return nil
}

// ipConfig turns the IP and gateway inputs into a cloud-init ipconfig0 value.
func ipConfig(ip, gw string) (string, error) {
	ip, gw = strings.TrimSpace(ip), strings.TrimSpace(gw)
	if ip == "" || strings.EqualFold(ip, "dhcp") {
		return "ip=dhcp", nil
	}
	if !cidrRe.MatchString(ip) {
		return "", inputErr("IP address %q needs a prefix, like 192.168.1.50/24 (or use dhcp)", ip)
	}
	if gw == "" {
		return "", inputErr("a fixed address needs a gateway")
	}
	if !ipRe.MatchString(gw) {
		return "", inputErr("gateway %q isn't an IPv4 address", gw)
	}
	return "ip=" + ip + ",gw=" + gw, nil
}

func between(v, lo, hi int, what string) error {
	if v < lo || v > hi {
		return inputErr("%s must be between %d and %d", what, lo, hi)
	}
	return nil
}
