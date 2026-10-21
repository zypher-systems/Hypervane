package scripts

import (
	"fmt"
	"strings"

	"hypervane/internal/pve"
)

func HasContent(st pve.Storage, kind string) bool {
	for _, part := range strings.Split(st.Content, ",") {
		if strings.TrimSpace(part) == kind {
			return true
		}
	}
	return false
}

func OnNode(st pve.Storage, node string) bool {
	if st.Nodes == "" {
		return true
	}
	for _, part := range strings.Split(st.Nodes, ",") {
		if strings.TrimSpace(part) == node {
			return true
		}
	}
	return false
}

func Pick(list []pve.Storage, node, kind string) (pve.Storage, bool) {
	var cands []pve.Storage
	for _, st := range list {
		if st.Disable != 0 || !HasContent(st, kind) || !OnNode(st, node) {
			continue
		}
		cands = append(cands, st)
	}
	if kind == "import" {
		for _, st := range cands {
			if st.Storage == "local" {
				return st, true
			}
		}
	}
	if len(cands) == 0 {
		return pve.Storage{}, false
	}
	return cands[0], true
}

func ResolveStores(list []pve.Storage, node, diskStorage string) (disk, download string, err error) {
	if diskStorage != "" {
		for _, st := range list {
			if st.Storage != diskStorage {
				continue
			}
			if st.Disable != 0 || !OnNode(st, node) {
				return "", "", fmt.Errorf("storage %s is not available on %s", diskStorage, node)
			}
			if !HasContent(st, "images") {
				return "", "", fmt.Errorf("storage %s cannot hold disk images", diskStorage)
			}
			disk = st.Storage
			break
		}
		if disk == "" {
			return "", "", fmt.Errorf("unknown storage %s", diskStorage)
		}
	} else if st, ok := Pick(list, node, "images"); ok {
		disk = st.Storage
	} else {
		return "", "", fmt.Errorf("no storage on %s can hold disk images", node)
	}
	if st, ok := Pick(list, node, "import"); ok {
		return disk, st.Storage, nil
	}
	return "", "", fmt.Errorf("no storage on %s has import content; add import to a directory store (usually local)", node)
}

func ResolveBackupStore(list []pve.Storage, node, name string) (string, error) {
	if name != "" {
		for _, st := range list {
			if st.Storage != name {
				continue
			}
			if st.Disable != 0 || (node != "" && !OnNode(st, node)) {
				return "", fmt.Errorf("storage %s is not available on %s", name, node)
			}
			if !HasContent(st, "backup") {
				return "", fmt.Errorf("storage %s cannot hold backups", name)
			}
			return st.Storage, nil
		}
		return "", fmt.Errorf("unknown storage %s", name)
	}
	if st, ok := Pick(list, node, "backup"); ok {
		return st.Storage, nil
	}
	if node != "" {
		return "", fmt.Errorf("no backup-capable storage on %s", node)
	}
	return "", fmt.Errorf("no backup-capable storage")
}
