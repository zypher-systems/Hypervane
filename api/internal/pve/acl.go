package pve

import (
	"fmt"
	"strconv"
	"strings"
)

type ACL map[string]map[string]int

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
				if t != 0 {
					entry[priv] = int(t)
				}
			case int:
				if t != 0 {
					entry[priv] = t
				}
			case bool:
				if t {
					entry[priv] = 1
				}
			}
		}
		if len(entry) > 0 {
			out[path] = entry
		}
	}
	return out
}

func (a ACL) Has(path, priv string) bool {
	if a == nil {
		return false
	}
	for _, prefix := range pathPrefixes(path) {
		entry := a[prefix]
		if entry == nil {
			continue
		}
		if entry[priv] != 0 {
			return true
		}
		for k, v := range entry {
			if v != 0 && (k == priv || strings.HasPrefix(k, priv+".")) {
				return true
			}
		}
	}
	return false
}

func (a ACL) CanSee(path string) bool {
	if a == nil {
		return false
	}
	for _, prefix := range pathPrefixes(path) {
		if len(a[prefix]) > 0 {
			return true
		}
	}
	return false
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

func ResourceACLPath(r Resource) string {
	switch r.Type {
	case "qemu", "lxc":
		if r.VMID > 0 {
			return fmt.Sprintf("/vms/%d", r.VMID)
		}
	case "node":
		if r.Node != "" {
			return "/nodes/" + r.Node
		}
	case "storage":
		if r.Storage != "" {
			return "/storage/" + r.Storage
		}
	case "pool":
		if r.Pool != "" {
			return "/pool/" + r.Pool
		}
	case "sdn":
		return "/sdn"
	case "cluster":
		return "/"
	}
	return "/"
}

func FilterResources(resources []Resource, acl ACL) []Resource {
	if acl == nil {
		return resources
	}
	out := make([]Resource, 0, len(resources))
	for _, r := range resources {
		if r.Type == "cluster" || acl.CanSee(ResourceACLPath(r)) {
			out = append(out, r)
		}
	}
	return out
}

func FilterTasks(tasks []Task, acl ACL) []Task {
	if acl == nil {
		return tasks
	}
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if CanSeeTask(t, acl) {
			out = append(out, t)
		}
	}
	return out
}

func CanSeeTask(t Task, acl ACL) bool {
	if acl == nil {
		return true
	}
	if t.Node != "" && acl.CanSee("/nodes/"+t.Node) {
		return true
	}
	if t.ID != "" {
		if vmid, err := strconv.Atoi(t.ID); err == nil && vmid > 0 && acl.CanSee(fmt.Sprintf("/vms/%d", vmid)) {
			return true
		}
	}
	return acl.CanSee("/")
}

func CanSeeGuestTopic(topic string, acl ACL) bool {
	if acl == nil {
		return true
	}
	// guest:node:kind:vmid
	parts := strings.Split(topic, ":")
	if len(parts) < 4 {
		return acl.CanSee("/")
	}
	vmid, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return acl.CanSee("/")
	}
	return acl.CanSee(fmt.Sprintf("/vms/%d", vmid))
}

func CanSeeTaskTopic(topic string, acl ACL) bool {
	if acl == nil {
		return true
	}
	upid := strings.TrimPrefix(topic, "task:")
	node, _, id := ParseUPID(upid)
	return CanSeeTask(Task{UPID: upid, Node: node, ID: id}, acl)
}

func CanSeeNodeTopic(topic string, acl ACL) bool {
	if acl == nil {
		return true
	}
	node := strings.TrimPrefix(topic, "node:")
	if node == "" || node == topic {
		return acl.CanSee("/")
	}
	return acl.CanSee("/nodes/" + node)
}

func CanSeeRRDTopic(topic string, acl ACL) bool {
	if acl == nil {
		return true
	}
	rest := strings.TrimPrefix(topic, "rrd:")
	parts := strings.Split(rest, ":")
	if len(parts) < 2 {
		return acl.CanSee("/")
	}
	switch parts[0] {
	case "node":
		return acl.CanSee("/nodes/" + parts[1])
	case "qemu", "lxc":
		if len(parts) >= 3 {
			return acl.CanSee("/vms/" + parts[2])
		}
	case "storage":
		if len(parts) >= 3 {
			return acl.CanSee("/storage/" + parts[2])
		}
	}
	return acl.CanSee("/")
}
