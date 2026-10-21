package httpapi

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"hypervane/internal/pve"
)

func (s *Server) handleClone(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.CloneRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.NewID == 0 {
		id, err := c.NextID(r.Context())
		if err != nil {
			writePVEError(w, err)
			return
		}
		req.NewID = id
	}
	upid, err := c.CloneGuest(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"upid": upid, "vmid": req.NewID})
}

func (s *Server) handleTemplate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	upid, err := c.ConvertTemplate(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleMigrate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.MigrateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}
	upid, err := c.MigrateGuest(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleRemoteMigrate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.RemoteMigrateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Remote == "" && req.TargetEndpoint == "" {
		writeError(w, http.StatusBadRequest, "remote or target-endpoint is required")
		return
	}
	upid, err := c.RemoteMigrateGuest(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleResizeDisk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.DiskResizeRequest
	if err := decodeJSON(r, &req); err != nil || req.Disk == "" || req.Size == "" {
		writeError(w, http.StatusBadRequest, "disk and size are required")
		return
	}
	upid, err := c.ResizeDisk(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleMoveDisk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.DiskMoveRequest
	if err := decodeJSON(r, &req); err != nil || req.Disk == "" || req.Storage == "" {
		writeError(w, http.StatusBadRequest, "disk and storage are required")
		return
	}
	upid, err := c.MoveDisk(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleUnlinkDisk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.DiskUnlinkRequest
	if err := decodeJSON(r, &req); err != nil || req.IDList == "" {
		writeError(w, http.StatusBadRequest, "idlist is required")
		return
	}
	if err := c.UnlinkDisk(r.Context(), node, kind, vmid, req.IDList); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRescanDisks(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	res, err := c.RescanDisks(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if !res.Supported {
		writeJSON(w, http.StatusOK, res)
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func (s *Server) handleGetCloudInit(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	cfg, err := c.GuestConfig(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	ci, _ := c.CloudInit(r.Context(), node, kind, vmid)
	writeJSON(w, http.StatusOK, map[string]any{
		"cloudinit":    ci,
		"ciuser":       cfg["ciuser"],
		"sshkeys":      cfg["sshkeys"],
		"ipconfig0":    cfg["ipconfig0"],
		"nameserver":   cfg["nameserver"],
		"searchdomain": cfg["searchdomain"],
	})
}

func (s *Server) handlePutCloudInit(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.CloudInitConfig
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateCloudInit(r.Context(), node, kind, vmid, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCloudInitDump(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	item, err := c.CloudInitDump(r.Context(), node, kind, r.URL.Query().Get("type"), vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleRegenerateCloudInit(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	res, err := c.RegenerateCloudInit(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if !res.Supported {
		writeJSON(w, http.StatusOK, res)
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func (s *Server) handleRevertPending(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	if err := c.RevertPending(r.Context(), node, kind, vmid, r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGuestFeature(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	item, err := c.GuestFeature(r.Context(), node, kind, r.URL.Query().Get("feature"), vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleNodeHardware(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeHardware(r.Context(), urlParam(r, "node"), urlParam(r, "kind"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleNodeMdev(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeMdev(r.Context(), urlParam(r, "node"), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	caps, err := c.QEMUCapabilities(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, caps)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	content := queryDefault(r, "content", "iso")
	nodeFilter := r.URL.Query().Get("node")
	storages, err := c.Storages(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	resources, err := c.ClusterResources(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	nodes := make([]string, 0)
	for _, res := range resources {
		if res.Type == "node" && res.Status != "offline" && res.Node != "" {
			if nodeFilter == "" || res.Node == nodeFilter {
				nodes = append(nodes, res.Node)
			}
		}
	}
	items := make([]pve.MediaItem, 0)
	seen := map[string]bool{}
	for _, st := range storages {
		if st.Disable == 1 || !strings.Contains(st.Content, content) {
			continue
		}
		for _, node := range nodes {
			list, err := c.StorageContent(r.Context(), node, st.Storage, content)
			if err != nil {
				continue
			}
			for _, it := range list {
				if seen[it.VolID] {
					continue
				}
				seen[it.VolID] = true
				items = append(items, pve.MediaItem{
					VolID: it.VolID, Content: it.Content, Storage: st.Storage, Node: node, Size: it.Size,
				})
			}
			break
		}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDownloadURL(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	var req pve.DownloadURLRequest
	if err := decodeJSON(r, &req); err != nil || req.URL == "" || req.Filename == "" {
		writeError(w, http.StatusBadRequest, "url and filename are required")
		return
	}
	upid, err := c.DownloadURL(r.Context(), node, urlParam(r, "storage"), req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	content := queryDefault(r, "content", "iso")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart body required")
		return
	}
	var (
		filename string
		file     io.ReadCloser
	)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart")
			return
		}
		if part.FormName() == "content" {
			buf, _ := io.ReadAll(io.LimitReader(part, 64))
			if v := strings.TrimSpace(string(buf)); v != "" {
				content = v
			}
			_ = part.Close()
			continue
		}
		if part.FormName() == "file" || part.FormName() == "filename" {
			filename = part.FileName()
			file = part
			break
		}
		_ = part.Close()
	}
	if file == nil || filename == "" {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	if err := c.Upload(r.Context(), node, urlParam(r, "storage"), content, filename, file); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBulk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	action := urlParam(r, "action")
	switch action {
	case "start", "shutdown", "stop", "migrate", "suspend":
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	var req pve.BulkRequest
	if err := decodeJSON(r, &req); err != nil || len(req.Guests) == 0 {
		writeError(w, http.StatusBadRequest, "guests are required")
		return
	}
	if action == "migrate" && req.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}
	upids, err := c.BulkGuestAction(r.Context(), action, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"upids": upids})
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	out := map[string]any{}
	available := false
	var lastErr error
	for _, item := range []struct{ cmd, key string }{
		{"ping", "ping"},
		{"get-osinfo", "osinfo"},
		{"get-fsinfo", "fsinfo"},
		{"network-get-interfaces", "nics"},
		{"get-users", "users"},
		{"get-host-name", "hostname"},
		{"get-timezone", "timezone"},
	} {
		v, err := c.AgentGet(r.Context(), node, vmid, item.cmd)
		if err != nil {
			lastErr = err
			continue
		}
		out[item.key] = v
		available = true
	}
	out["available"] = available
	if !available && lastErr != nil {
		out["error"] = lastErr.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAgentFstrim(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	res, err := c.AgentPost(r.Context(), node, vmid, "fstrim", url.Values{})
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAgentPassword(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.AgentPasswordRequest
	if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	form := url.Values{"username": {req.Username}, "password": {req.Password}}
	res, err := c.AgentPost(r.Context(), node, vmid, "set-user-password", form)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

var agentGetCommands = map[string]bool{
	"get-users": true, "ping": true, "get-osinfo": true, "get-fsinfo": true,
	"network-get-interfaces": true, "exec-status": true,
	"get-host-name": true, "get-time": true, "get-timezone": true,
	"get-vcpus": true, "get-memory-blocks": true,
}

var agentPostCommands = map[string]bool{
	"exec": true, "fsfreeze-freeze": true, "fsfreeze-thaw": true,
	"fstrim": true, "set-user-password": true,
}

func (s *Server) handleAgentCommand(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	cmd := urlParam(r, "command")
	if !agentGetCommands[cmd] {
		writeError(w, http.StatusBadRequest, "unsupported agent command")
		return
	}
	if cmd == "exec-status" {
		if pid := r.URL.Query().Get("pid"); pid != "" {
			cmd = "exec-status?pid=" + url.QueryEscape(pid)
		}
	}
	res, err := c.AgentGet(r.Context(), node, vmid, cmd)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAgentCommandPost(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	cmd := urlParam(r, "command")
	if !agentPostCommands[cmd] {
		writeError(w, http.StatusBadRequest, "unsupported agent command")
		return
	}
	var body map[string]any
	_ = decodeJSON(r, &body)
	form := url.Values{}
	if cmd == "exec" {
		raw := strings.TrimSpace(fmtString(body["command"]))
		if raw == "" {
			writeError(w, http.StatusBadRequest, "command is required")
			return
		}
		for _, part := range strings.Fields(raw) {
			form.Add("command", part)
		}
		if in := fmtString(body["input"]); in != "" {
			form.Set("input-data", in)
		}
	}
	if cmd == "set-user-password" {
		form.Set("username", fmtString(body["username"]))
		form.Set("password", fmtString(body["password"]))
	}
	res, err := c.AgentPost(r.Context(), node, vmid, cmd, form)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func fmtString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (s *Server) handleSendKey(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	if kind != "qemu" {
		writeError(w, http.StatusBadRequest, "sendkey is only available for QEMU guests")
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if err := c.SendKey(r.Context(), node, vmid, body.Key); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleQEMUMonitor(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	if kind != "qemu" {
		writeError(w, http.StatusBadRequest, "monitor is only available for QEMU guests")
		return
	}
	var body struct {
		Command string `json:"command"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}
	out, err := c.QEMUMonitor(r.Context(), node, vmid, body.Command)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}
