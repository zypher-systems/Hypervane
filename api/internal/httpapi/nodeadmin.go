package httpapi

import (
	"net/http"
	"strconv"

	"hypervane/internal/pve"
)

func (s *Server) handleNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.NodeNetwork(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateNodeNetwork(r.Context(), urlParam(r, "node"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNodeNetwork(r.Context(), urlParam(r, "node"), urlParam(r, "iface"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteNodeNetwork(r.Context(), urlParam(r, "node"), urlParam(r, "iface"), r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleApplyNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.ApplyNodeNetwork(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleRevertNodeNetwork(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.RevertNodeNetwork(r.Context(), urlParam(r, "node")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeDisks(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.NodeDisks(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleNodeDiskSMART(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	disk := r.URL.Query().Get("disk")
	if disk == "" {
		writeError(w, http.StatusBadRequest, "disk is required")
		return
	}
	item, err := c.NodeDiskSMART(r.Context(), urlParam(r, "node"), disk)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleWipeNodeDisk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Disk string `json:"disk"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Disk == "" {
		writeError(w, http.StatusBadRequest, "disk is required")
		return
	}
	upid, err := c.WipeNodeDisk(r.Context(), urlParam(r, "node"), body.Disk)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleNodeDiskBackends(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.NodeDiskBackends(r.Context(), urlParam(r, "node"), urlParam(r, "kind"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateNodeDiskBackend(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	upid, err := c.CreateNodeDiskBackend(r.Context(), urlParam(r, "node"), urlParam(r, "kind"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if storage, _ := body["storage"].(string); storage != "" {
		stype := storageType(urlParam(r, "kind"))
		fields := map[string]any{"storage": storage, "type": stype, "nodes": urlParam(r, "node")}
		if vg, ok := body["name"]; ok {
			switch stype {
			case "lvm", "lvmthin":
				fields["vgname"] = vg
				if stype == "lvmthin" {
					fields["thinpool"] = vg
				}
			case "zfspool":
				fields["pool"] = vg
			case "dir":
				fields["path"] = body["path"]
				if fields["path"] == nil {
					fields["path"] = "/mnt/" + storage
				}
			}
		}
		_ = c.CreateStorage(r.Context(), fields)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleDeleteNodeDiskBackend(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.DeleteNodeDiskBackend(r.Context(), urlParam(r, "node"), urlParam(r, "kind"), urlParam(r, "name"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func storageType(kind string) string {
	switch kind {
	case "lvmthin":
		return "lvmthin"
	case "lvm":
		return "lvm"
	case "zfs":
		return "zfspool"
	default:
		return "dir"
	}
}

func (s *Server) handleAPTUpdates(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.APTUpdates(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAPTUpdate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.APTUpdate(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleAPTVersions(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.APTVersions(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAPTChangelog(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	text, err := c.APTChangelog(r.Context(), urlParam(r, "node"), r.URL.Query().Get("name"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"changelog": text})
}

func (s *Server) handleAPTRepositories(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.APTRepositories(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateAPTRepository(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateAPTRepository(r.Context(), urlParam(r, "node"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeServices(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.NodeServices(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleNodeServiceAction(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.NodeServiceAction(r.Context(), urlParam(r, "node"), urlParam(r, "service"), urlParam(r, "action"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleNodeDNS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeDNS(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateNodeDNS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.NodeDNS
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNodeDNS(r.Context(), urlParam(r, "node"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeTime(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeTime(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateNodeTime(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Timezone == "" {
		writeError(w, http.StatusBadRequest, "timezone is required")
		return
	}
	if err := c.UpdateNodeTime(r.Context(), urlParam(r, "node"), body.Timezone); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeHosts(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeHosts(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateNodeHosts(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.NodeHosts
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNodeHosts(r.Context(), urlParam(r, "node"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeJournal(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	last, _ := strconv.Atoi(r.URL.Query().Get("lastentries"))
	items, err := c.NodeJournal(r.Context(), urlParam(r, "node"), start, last)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleNodeSubscription(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeSubscription(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSetNodeSubscription(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if err := c.SetNodeSubscription(r.Context(), urlParam(r, "node"), body.Key); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeCertificates(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.NodeCertificates(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUploadNodeCertificate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UploadNodeCertificate(r.Context(), urlParam(r, "node"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteNodeCertificate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteNodeCertificate(r.Context(), urlParam(r, "node")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleOrderACMECertificate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Force bool `json:"force"`
	}
	_ = decodeJSON(r, &body)
	upid, err := c.OrderACMECertificate(r.Context(), urlParam(r, "node"), body.Force)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleRenewACMECertificate(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.RenewACMECertificate(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleNodeExecute(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Commands any `json:"commands"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Commands == nil {
		writeError(w, http.StatusBadRequest, "commands are required")
		return
	}
	out, err := c.NodeExecute(r.Context(), urlParam(r, "node"), body.Commands)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleNodeCommand(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid, err := c.NodeCommand(r.Context(), urlParam(r, "node"), urlParam(r, "command"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleNodeReport(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeReport(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleWakeOnLAN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.WakeOnLAN(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleNodeConfig(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeConfig(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateNodeConfig(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNodeConfig(r.Context(), urlParam(r, "node"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleInitGPT(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		Disk string `json:"disk"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Disk == "" {
		writeError(w, http.StatusBadRequest, "disk is required")
		return
	}
	upid, err := c.InitGPT(r.Context(), urlParam(r, "node"), body.Disk)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleZFSStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ZFSStatus(r.Context(), urlParam(r, "node"), urlParam(r, "name"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleOptionalTask(w http.ResponseWriter, res pve.DiskRescanResult, err error) {
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

func (s *Server) handleZFSScrub(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	res, err := c.ZFSScrub(r.Context(), urlParam(r, "node"), urlParam(r, "name"))
	s.handleOptionalTask(w, res, err)
}

func (s *Server) handleZFSImport(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	res, err := c.ZFSImport(r.Context(), urlParam(r, "node"), body)
	s.handleOptionalTask(w, res, err)
}

func (s *Server) handleNodeGuestAll(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		body = map[string]any{}
	}
	upid, err := c.NodeGuestAll(r.Context(), urlParam(r, "node"), urlParam(r, "action"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleAPLInfo(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.APLInfo(r.Context(), urlParam(r, "node"))
	})
}
