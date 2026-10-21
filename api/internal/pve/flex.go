package pve

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// FlexInt decodes an integer Proxmox may send as a JSON number or a string
// ("vmid": "100" on some storage endpoints, 100 on others).
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*f = FlexInt(n)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	i, err := n.Int64()
	if err != nil {
		fl, ferr := n.Float64()
		if ferr != nil {
			return err
		}
		i = int64(fl)
	}
	*f = FlexInt(i)
	return nil
}

// FlexCSV decodes a list Proxmox sends either as a comma-separated string or
// as a JSON array (user "groups" is a string in the user list and an array on
// the single-user endpoint). It always re-encodes as a comma-separated string.
type FlexCSV string

func (f *FlexCSV) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '[' {
		var items []any
		if err := json.Unmarshal(b, &items); err != nil {
			return err
		}
		parts := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(anyString(it))
			if s != "" {
				parts = append(parts, s)
			}
		}
		*f = FlexCSV(strings.Join(parts, ","))
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*f = FlexCSV(s)
	return nil
}

func anyString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// decodeFlexibleMaps reads a Proxmox payload that should be a list of objects
// but is sometimes a single object (qdevice status) or a tree whose rows sit
// under "children" (LVM volume groups).
func decodeFlexibleMaps(raw json.RawMessage) ([]map[string]any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return []map[string]any{}, nil
	}
	if raw[0] == '[' {
		var out []map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		if out == nil {
			out = []map[string]any{}
		}
		return out, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if children, ok := obj["children"].([]any); ok {
		out := make([]map[string]any, 0, len(children))
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	}
	if len(obj) == 0 {
		return []map[string]any{}, nil
	}
	return []map[string]any{obj}, nil
}
