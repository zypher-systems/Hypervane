package pve

import (
	"encoding/json"
	"strings"
)

// GuestFSUsed sums guest-agent get-fsinfo used-bytes, skipping virtual/network
// filesystems. QEMU status/current.disk is typically 0 without this.
func GuestFSUsed(raw any) int64 {
	var used int64
	for _, item := range fsinfoItems(raw) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if skipFSType(strings.ToLower(strVal(m["type"]))) {
			continue
		}
		if v, ok := numVal(m["used-bytes"]); ok && v > 0 {
			used += v
			continue
		}
		if v, ok := numVal(m["used"]); ok && v > 0 {
			used += v
		}
	}
	return used
}

func skipFSType(typ string) bool {
	switch typ {
	case "iso9660", "udf", "nfs", "cifs", "smb", "9p", "squashfs",
		"tmpfs", "devtmpfs", "overlay", "proc", "sysfs", "devfs":
		return true
	default:
		return false
	}
}

func fsinfoItems(raw any) []any {
	cur := raw
	for i := 0; i < 4; i++ {
		switch v := cur.(type) {
		case []any:
			return v
		case map[string]any:
			if next, ok := v["result"]; ok {
				cur = next
				continue
			}
			if next, ok := v["data"]; ok {
				cur = next
				continue
			}
		}
		break
	}
	return nil
}

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

func numVal(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}
