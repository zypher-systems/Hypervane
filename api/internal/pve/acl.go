package pve

import (
	"context"
	"regexp"
	"strings"
)

type ACL map[string]map[string]int

// NormalizePermissions reads an /access/permissions answer. Each value there is
// a propagate flag, so a key means the right is held and 0 only means it
// doesn't pass to child paths.
func NormalizePermissions(raw map[string]any) ACL {
	out := make(ACL)
	for path, v := range raw {
		privs, ok := v.(map[string]any)
		if !ok {
			continue
		}
		entry := make(map[string]int, len(privs))
		for priv, n := range privs {
			switch t := n.(type) {
			case float64:
				entry[priv] = int(t)
			case int:
				entry[priv] = t
			case bool:
				entry[priv] = 0
				if t {
					entry[priv] = 1
				}
			}
		}
		out[path] = entry
	}
	return out
}

// Has answers from the nearest listed path. Proxmox lists a path's effective
// rights, so that entry already holds everything inherited, and an ancestor
// only speaks for a child through rights that propagate.
func (a ACL) Has(path, priv string) bool {
	for i, prefix := range pathPrefixes(path) {
		entry, listed := a[prefix]
		if !listed {
			continue
		}
		exact := i == 0
		for held, propagates := range entry {
			if (held == priv || strings.HasPrefix(held, priv+".")) && (exact || propagates != 0) {
				return true
			}
		}
		return false
	}
	return false
}

// CanRead reports whether the ACL holds the privilege Proxmox itself requires
// to read what lives at path. Any other privilege on the path grants nothing:
// cached data comes from the API token, so Proxmox never gets to refuse it.
func (a ACL) CanRead(path string) bool {
	return a.Has(path, readPrivilege(path))
}

// readPrivilege follows the checks in Proxmox's /cluster/resources.
func readPrivilege(path string) string {
	switch strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0] {
	case "vms":
		return "VM.Audit"
	case "storage":
		return "Datastore.Audit"
	case "pool":
		return "Pool.Audit"
	case "sdn":
		return "SDN.Audit"
	}
	return "Sys.Audit"
}

func pathPrefixes(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return []string{"/"}
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	specific := make([]string, 0, len(parts)+1)
	cur := ""
	for _, p := range parts {
		cur += "/" + p
		specific = append(specific, cur)
	}
	for i, j := 0, len(specific)-1; i < j; i, j = i+1, j-1 {
		specific[i], specific[j] = specific[j], specific[i]
	}
	return append(specific, "/")
}

// resourcePath maps a /cluster/resources id to the path Proxmox checks before
// listing it. ok is false for any other shape: like Proxmox with an SDN type it
// doesn't know, a viewer gets nothing rather than a guess.
func resourcePath(id string) (path string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
	last := parts[len(parts)-1]
	if last == "" {
		return "", false
	}
	switch {
	case len(parts) == 2 && (parts[0] == "qemu" || parts[0] == "lxc"):
		if vmID.MatchString(last) {
			return "/vms/" + last, true
		}
	case len(parts) == 2 && parts[0] == "node":
		return "/nodes/" + last, true
	case len(parts) == 2 && parts[0] == "pool":
		return "/pool/" + last, true
	case len(parts) == 3 && parts[0] == "storage":
		return "/storage/" + last, true
	case len(parts) == 3 && parts[0] == "sdn":
		return "/sdn/zones/" + last, true
	case len(parts) == 4 && parts[0] == "network" && parts[2] == "zone":
		return "/sdn/zones/" + last, true
	case len(parts) == 4 && parts[0] == "network" && parts[2] == "fabric":
		return "/sdn/fabrics/" + last, true
	}
	return "", false
}

func FilterResources(resources []Resource, acl ACL) []Resource {
	if acl == nil {
		return resources
	}
	out := make([]Resource, 0, len(resources))
	for _, r := range resources {
		if r.Type == "cluster" || CanSeeResourceID(r.ID, acl) {
			out = append(out, r)
		}
	}
	return out
}

// FilterStatus keeps the cluster summary and the nodes the ACL can see. Node
// entries carry addresses that /cluster/status only gives to Sys.Audit holders.
func FilterStatus(status []ClusterStatus, acl ACL) []ClusterStatus {
	if acl == nil {
		return status
	}
	out := make([]ClusterStatus, 0, len(status))
	for _, st := range status {
		if st.Type != "node" || acl.CanRead("/nodes/"+st.Name) {
			out = append(out, st)
		}
	}
	return out
}

// CanSeeResourceID reports whether the ACL covers a /cluster/resources id.
func CanSeeResourceID(id string, acl ACL) bool {
	if acl == nil {
		return true
	}
	path, ok := resourcePath(id)
	return ok && acl.CanRead(path)
}

func FilterTasks(tasks []Task, acl ACL, userid string) []Task {
	if acl == nil {
		return tasks
	}
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if CanSeeTask(t, acl, userid) {
			out = append(out, t)
		}
	}
	return out
}

// CanSeeTask follows Proxmox: a task belongs to whoever started it, and anyone
// else needs Sys.Audit on the node it ran on. A token's tasks are its owner's.
func CanSeeTask(t Task, acl ACL, userid string) bool {
	if acl == nil {
		return true
	}
	owner, node := t.User, t.Node
	if owner == "" {
		owner = upidUser(t.UPID)
	}
	if node == "" {
		node = ParseUPIDNode(t.UPID)
	}
	if userid != "" && (owner == userid || strings.HasPrefix(owner, userid+"!")) {
		return true
	}
	if node == "" {
		return acl.CanRead("/")
	}
	return acl.CanRead("/nodes/" + node)
}

// upidUser reads the user from UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:
func upidUser(upid string) string {
	if parts := strings.Split(upid, ":"); len(parts) >= 8 {
		return parts[7]
	}
	return ""
}

// ValidTopic accepts only the exact shape of a live detail topic. A topic is
// read by the ACL check and again by the poller, which spends the connection's
// token, so nothing may be able to mean one thing to the first and another to
// the second.
func ValidTopic(detail string) bool {
	parts := strings.Split(detail, ":")
	switch parts[0] {
	case "node":
		return len(parts) == 2 && nodeName.MatchString(parts[1])
	case "guest":
		return len(parts) == 4 && validGuest(parts[1], parts[2], parts[3])
	case "task":
		return validUPID(parts[1:])
	case "rrd":
		return validRRD(parts[1:])
	}
	return false
}

var (
	nodeName  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$`)
	storageID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*$`)
	vmID      = regexp.MustCompile(`^[1-9][0-9]*$`)
	hexField  = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
)

func validGuest(node, kind, vmid string) bool {
	return nodeName.MatchString(node) && (kind == "qemu" || kind == "lxc") && vmID.MatchString(vmid)
}

// validUPID checks UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:
func validUPID(parts []string) bool {
	if len(parts) != 9 || parts[0] != "UPID" || parts[8] != "" || parts[5] == "" || parts[7] == "" {
		return false
	}
	return nodeName.MatchString(parts[1]) && hexField.MatchString(parts[2]) && hexField.MatchString(parts[3]) && hexField.MatchString(parts[4])
}

func validRRD(parts []string) bool {
	if len(parts) < 3 {
		return false
	}
	switch parts[len(parts)-1] {
	case "hour", "day", "week", "month", "year":
	default:
		return false
	}
	switch parts[0] {
	case "node":
		return len(parts) == 3 && nodeName.MatchString(parts[1])
	case "qemu", "lxc":
		return len(parts) == 4 && validGuest(parts[1], parts[0], parts[2])
	case "storage":
		return len(parts) == 4 && nodeName.MatchString(parts[1]) && storageID.MatchString(parts[2])
	}
	return false
}

func CanSeeGuestTopic(topic string, acl ACL) bool {
	if !ValidTopic(topic) || !strings.HasPrefix(topic, "guest:") {
		return false
	}
	if acl == nil {
		return true
	}
	parts := strings.Split(topic, ":")
	return acl.CanRead("/vms/" + parts[3])
}

func CanSeeTaskTopic(topic string, acl ACL, userid string) bool {
	if !ValidTopic(topic) || !strings.HasPrefix(topic, "task:") {
		return false
	}
	return CanSeeTask(Task{UPID: strings.TrimPrefix(topic, "task:")}, acl, userid)
}

func CanSeeNodeTopic(topic string, acl ACL) bool {
	if !ValidTopic(topic) || !strings.HasPrefix(topic, "node:") {
		return false
	}
	return acl == nil || acl.CanRead("/nodes/"+strings.TrimPrefix(topic, "node:"))
}

func CanSeeRRDTopic(topic string, acl ACL) bool {
	if !ValidTopic(topic) || !strings.HasPrefix(topic, "rrd:") {
		return false
	}
	if acl == nil {
		return true
	}
	parts := strings.Split(topic, ":")
	switch parts[1] {
	case "node":
		return acl.CanRead("/nodes/" + parts[2])
	case "storage":
		return acl.CanRead("/storage/" + parts[3])
	}
	return acl.CanRead("/vms/" + parts[3])
}

// closedAudits are the read privileges Proxmox has already answered for in the
// viewer's own /cluster/resources.
var closedAudits = []string{"VM.Audit", "Datastore.Audit", "Pool.Audit", "SDN.Audit"}

// restrictTo makes the viewer's own /cluster/resources the whole truth for what
// that call covers. The permission tree can't show a NoAccess exception, because
// Proxmox leaves a path with no rights out of it, but this list already reflects
// it, along with pools and propagation.
func (a ACL) restrictTo(resources []Resource) {
	for _, entry := range a {
		for _, priv := range closedAudits {
			delete(entry, priv)
		}
	}
	for _, r := range resources {
		path, ok := resourcePath(r.ID)
		if !ok || r.Type == "node" {
			continue
		}
		if a[path] == nil {
			a[path] = map[string]int{}
		}
		a[path][readPrivilege(path)] = 1
	}
}

// ViewerACL works out what the client's user may read, asking Proxmox with that
// user's own ticket. Nodes are always listed by /cluster/resources, so each is
// asked about directly: an exact entry, even an empty one, ends the walk up to /.
func (c *Client) ViewerACL(ctx context.Context) (ACL, error) {
	raw, err := c.Permissions(ctx)
	if err != nil {
		return nil, err
	}
	acl := NormalizePermissions(raw)
	resources, err := c.ClusterResources(ctx)
	if err != nil {
		return nil, err
	}
	acl.restrictTo(resources)
	for _, r := range resources {
		if r.Type != "node" || r.Node == "" {
			continue
		}
		path := "/nodes/" + r.Node
		exact, err := c.PermissionsAt(ctx, path)
		if err != nil {
			return nil, err
		}
		acl[path] = NormalizePermissions(exact)[path]
		if acl[path] == nil {
			acl[path] = map[string]int{}
		}
	}
	return acl, nil
}
