package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"hypervane/internal/store"
)

type mcpReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":    "hypervane",
			"version": "3.1.0",
			"tools":   mcpToolDefs(),
		})
		return
	}
	var req mcpReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, mcpErr(req.ID, -32700, "parse error"))
		return
	}
	switch req.Method {
	case "initialize":
		writeJSON(w, http.StatusOK, mcpOK(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "hypervane", "version": "3.1.0"},
		}))
	case "ping":
		writeJSON(w, http.StatusOK, mcpOK(req.ID, map[string]any{}))
	case "tools/list":
		writeJSON(w, http.StatusOK, mcpOK(req.ID, map[string]any{"tools": mcpToolDefs()}))
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		text, err := s.mcpCall(r, p.Name, p.Arguments)
		if err != nil {
			writeJSON(w, http.StatusOK, mcpOK(req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}))
			return
		}
		writeJSON(w, http.StatusOK, mcpOK(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}))
	default:
		writeJSON(w, http.StatusOK, mcpErr(req.ID, -32601, "method not found"))
	}
}

func mcpToolDefs() []map[string]any {
	return []map[string]any{
		{"name": "hypervane_summary", "description": "Cluster status and resource counts for the active connection.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
		{"name": "hypervane_guests", "description": "List QEMU and LXC guests on the active connection.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
		{"name": "hypervane_guest", "description": "One guest by VMID.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"vmid": map[string]any{"type": "number"}}, "required": []string{"vmid"}}},
		{"name": "hypervane_history", "description": "Long-retention CPU/memory samples for a resource id.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"key": map[string]any{"type": "string"}}, "required": []string{"key"}}},
	}
}

func (s *Server) mcpCall(r *http.Request, name string, args map[string]any) (string, error) {
	conn := s.requestConnectionID(r)
	switch name {
	case "hypervane_summary":
		status, resources, err := s.poller.CachedResources(r.Context(), conn)
		if err != nil {
			return "", err
		}
		raw, _ := json.Marshal(map[string]any{"status": status, "resources": len(resources)})
		return string(raw), nil
	case "hypervane_guests":
		_, resources, err := s.poller.CachedResources(r.Context(), conn)
		if err != nil {
			return "", err
		}
		var guests []any
		for _, g := range resources {
			if g.Type == "qemu" || g.Type == "lxc" {
				guests = append(guests, g)
			}
		}
		raw, _ := json.Marshal(guests)
		return string(raw), nil
	case "hypervane_guest":
		vmid := int(asFloat(args["vmid"]))
		_, resources, err := s.poller.CachedResources(r.Context(), conn)
		if err != nil {
			return "", err
		}
		for _, g := range resources {
			if g.VMID == vmid {
				raw, _ := json.Marshal(g)
				return string(raw), nil
			}
		}
		return "", errNotFound("guest")
	case "hypervane_history":
		key, _ := args["key"].(string)
		if key == "" {
			return "", errNotFound("key")
		}
		rows, err := s.store.QueryMetricHistory(r.Context(), conn, key, asTime(args["from"]), asTime(args["to"]))
		if err != nil {
			return "", err
		}
		if rows == nil {
			rows = []store.MetricSample{}
		}
		raw, _ := json.Marshal(rows)
		return string(raw), nil
	default:
		return "", errNotFound("tool")
	}
}

func asTime(v any) time.Time {
	s, _ := v.(string)
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

func mcpOK(id any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func mcpErr(id any, code int, msg string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}}
}

type notFoundError string

func (e notFoundError) Error() string { return string(e) + " not found" }

func errNotFound(name string) error { return notFoundError(name) }
