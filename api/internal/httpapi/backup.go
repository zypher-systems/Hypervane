package httpapi

import (
	"context"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"hypervane/internal/pve"
)

func (s *Server) handleBackupJobs(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	jobs, err := c.BackupJobs(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleCreateBackupJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.BackupJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Storage == "" || req.Schedule == "" {
		writeError(w, http.StatusBadRequest, "storage and schedule are required")
		return
	}
	job, err := c.CreateBackupJob(r.Context(), req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleBackupJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	job, err := c.BackupJob(r.Context(), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleUpdateBackupJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.BackupJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateBackupJob(r.Context(), urlParam(r, "id"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteBackupJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteBackupJob(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleJobVolumes(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	tree, err := c.IncludedVolumes(r.Context(), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tree)
}

func (s *Server) handleCoverage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	resources, err := c.ClusterResources(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	notBackedUp, err := c.NotBackedUp(r.Context())
	if err != nil {
		notBackedUp, err = coverageFromJobs(c, r, resources)
		if err != nil {
			notBackedUp = nil
		}
	}
	unprotected := map[int]bool{}
	for _, g := range notBackedUp {
		unprotected[g.VMID] = true
	}

	last := lastBackupTimes(c, r, resources)
	guests := make([]pve.GuestBackupInfo, 0)
	for _, res := range resources {
		if res.Type != "qemu" && res.Type != "lxc" {
			continue
		}
		info := pve.GuestBackupInfo{
			VMID:  res.VMID,
			Name:  res.Name,
			Type:  res.Type,
			Node:  res.Node,
			InJob: !unprotected[res.VMID],
		}
		if snap, ok := last[res.VMID]; ok {
			info.LastBackup = snap.CTime
			info.LastVolID = snap.VolID
		}
		guests = append(guests, info)
	}
	writeJSON(w, http.StatusOK, pve.CoverageReport{NotBackedUp: notBackedUp, Guests: guests})
}

func (s *Server) handleGuestBackup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.VZDumpRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.VMID = vmid
	upid, err := c.VZDump(r.Context(), node, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleVZDumpDefaults(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := urlParam(r, "node")
	storage := r.URL.Query().Get("storage")
	if storage == "" {
		// Without a storage Proxmox resolves "local", which usually refuses backup
		// content and fails the whole request; ask for a backup-capable one.
		storage = backupStorageFor(r.Context(), c, node)
	}
	item, err := c.VZDumpDefaults(r.Context(), node, storage)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateVZDumpDefaults(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	res, err := c.UpdateVZDumpDefaults(r.Context(), urlParam(r, "node"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.RestoreRequest
	if err := decodeJSON(r, &req); err != nil || req.Node == "" || req.Archive == "" {
		writeError(w, http.StatusBadRequest, "node and archive are required")
		return
	}
	if req.Type == "" {
		req.Type = pve.InferBackupType(req.Archive)
	}
	if req.VMID == 0 {
		id, err := c.NextID(r.Context())
		if err != nil {
			writePVEError(w, err)
			return
		}
		req.VMID = id
	}
	upid, err := c.RestoreGuest(r.Context(), req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"upid": upid, "vmid": req.VMID})
}

func (s *Server) handleExtractConfig(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	volume := r.URL.Query().Get("volume")
	if node == "" || volume == "" {
		writeError(w, http.StatusBadRequest, "node and volume are required")
		return
	}
	cfg, err := c.ExtractBackupConfig(r.Context(), node, volume)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"config": cfg, "type": pve.InferBackupType(volume)})
}

func (s *Server) handleFileRestoreList(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	volume := r.URL.Query().Get("volume")
	if node == "" || volume == "" {
		writeError(w, http.StatusBadRequest, "node and volume are required")
		return
	}
	items, err := c.FileRestoreList(r.Context(), node, urlParam(r, "storage"), volume, r.URL.Query().Get("filepath"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleFileRestoreDownload(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	volume := r.URL.Query().Get("volume")
	filepath := r.URL.Query().Get("filepath")
	if node == "" || volume == "" || filepath == "" {
		writeError(w, http.StatusBadRequest, "node, volume, and filepath are required")
		return
	}
	res, err := c.FileRestoreDownload(r.Context(), node, urlParam(r, "storage"), volume, filepath)
	if err != nil {
		writePVEError(w, err)
		return
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if cd := res.Header.Get("Content-Disposition"); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	} else {
		w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(filepath)+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, res.Body)
}

func (s *Server) handleVolumeDownload(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	volume := r.URL.Query().Get("volume")
	if node == "" || volume == "" {
		writeError(w, http.StatusBadRequest, "node and volume are required")
		return
	}
	res, err := c.VolumeDownload(r.Context(), node, urlParam(r, "storage"), volume)
	if err != nil {
		if pve.IsNotFound(err) {
			writeJSON(w, http.StatusOK, map[string]any{"supported": false})
			return
		}
		writePVEError(w, err)
		return
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if cd := res.Header.Get("Content-Disposition"); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	} else {
		w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(volume)+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, res.Body)
}

func (s *Server) handlePrunePreview(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	vmid, _ := strconv.Atoi(r.URL.Query().Get("vmid"))
	items, err := c.PruneBackupsPreview(r.Context(), node, urlParam(r, "storage"), r.URL.Query().Get("prune"), vmid, r.URL.Query().Get("type"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	vmid, _ := strconv.Atoi(r.URL.Query().Get("vmid"))
	upid, err := c.PruneBackups(r.Context(), node, urlParam(r, "storage"), r.URL.Query().Get("prune"), vmid, r.URL.Query().Get("type"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleReplicationJobs(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	jobs, err := c.ReplicationJobs(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleCreateReplication(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.ReplicationRequest
	if err := decodeJSON(r, &req); err != nil || req.Guest == 0 || req.Target == "" {
		writeError(w, http.StatusBadRequest, "guest and target are required")
		return
	}
	if err := c.CreateReplication(r.Context(), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleReplicationJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	job, err := c.ReplicationJob(r.Context(), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleUpdateReplication(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.ReplicationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateReplication(r.Context(), urlParam(r, "id"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteReplication(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteReplication(r.Context(), urlParam(r, "id"), pve.DeleteReplicationOptions{
		Force: queryFlag(r, "force"),
		Keep:  queryFlag(r, "keep"),
	}); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleReplicationStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, ok := s.replicationNode(w, r)
	if !ok {
		return
	}
	st, err := c.ReplicationStatus(r.Context(), node, urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleReplicationLog(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, ok := s.replicationNode(w, r)
	if !ok {
		return
	}
	lines, err := c.ReplicationLog(r.Context(), node, urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) handleReplicationNow(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, ok := s.replicationNode(w, r)
	if !ok {
		return
	}
	upid, err := c.ReplicationScheduleNow(r.Context(), node, urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) replicationNode(w http.ResponseWriter, r *http.Request) (string, bool) {
	if node := r.URL.Query().Get("node"); node != "" {
		return node, true
	}
	c := s.pveOrError(w, r)
	if c == nil {
		return "", false
	}
	job, err := c.ReplicationJob(r.Context(), urlParam(r, "id"))
	if err == nil && job.Source != "" {
		return job.Source, true
	}
	if err == nil && job.Guest > 0 {
		resources, rerr := c.ClusterResources(r.Context())
		if rerr == nil {
			for _, res := range resources {
				if res.VMID == job.Guest && (res.Type == "qemu" || res.Type == "lxc") && res.Node != "" {
					return res.Node, true
				}
			}
		}
	}
	writeError(w, http.StatusBadRequest, "node is required")
	return "", false
}

func coverageFromJobs(c *pve.Client, r *http.Request, resources []pve.Resource) ([]pve.CoverageGuest, error) {
	return coverageFromJobsCtx(r.Context(), c, resources)
}

// coverageFromJobsCtx lists guests no enabled backup job includes, for
// clusters without /cluster/backup-info/not-backed-up.
func coverageFromJobsCtx(ctx context.Context, c *pve.Client, resources []pve.Resource) ([]pve.CoverageGuest, error) {
	jobs, err := c.BackupJobs(ctx)
	if err != nil {
		return nil, err
	}
	covered := map[int]bool{}
	all := false
	excluded := map[int]bool{}
	for _, job := range jobs {
		if job.Enabled == 0 {
			continue
		}
		if job.All == 1 {
			all = true
			for _, id := range splitIDs(job.Exclude) {
				excluded[id] = true
			}
			continue
		}
		for _, id := range splitIDs(job.VMID) {
			covered[id] = true
		}
	}
	out := make([]pve.CoverageGuest, 0)
	for _, res := range resources {
		if res.Type != "qemu" && res.Type != "lxc" {
			continue
		}
		inJob := covered[res.VMID] || (all && !excluded[res.VMID])
		if !inJob {
			out = append(out, pve.CoverageGuest{VMID: res.VMID, Name: res.Name, Type: res.Type})
		}
	}
	return out, nil
}

func lastBackupTimes(c *pve.Client, r *http.Request, resources []pve.Resource) map[int]pve.StorageContent {
	storages, err := c.Storages(r.Context())
	if err != nil {
		return nil
	}
	nodes := make([]string, 0)
	for _, res := range resources {
		if res.Type == "node" && res.Status != "offline" && res.Node != "" {
			nodes = append(nodes, res.Node)
		}
	}
	last := map[int]pve.StorageContent{}
	for _, st := range storages {
		if st.Disable == 1 || !strings.Contains(st.Content, "backup") {
			continue
		}
		for _, node := range nodes {
			items, err := c.StorageContent(r.Context(), node, st.Storage, "backup")
			if err != nil {
				continue
			}
			for _, it := range items {
				if it.VMID == 0 {
					continue
				}
				prev, ok := last[it.VMID]
				if !ok || it.CTime > prev.CTime {
					last[it.VMID] = it
				}
			}
			break
		}
	}
	return last
}

func splitIDs(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

// backupStorageFor returns the first enabled storage on node that accepts
// backups, or "" when none is found.
func backupStorageFor(ctx context.Context, c *pve.Client, node string) string {
	storages, err := c.Storages(ctx)
	if err != nil {
		return ""
	}
	for _, st := range storages {
		if st.Disable == 1 || !strings.Contains(st.Content, "backup") {
			continue
		}
		if st.Nodes != "" && !containsCSV(st.Nodes, node) {
			continue
		}
		return st.Storage
	}
	return ""
}

func containsCSV(list, item string) bool {
	for _, part := range strings.Split(list, ",") {
		if strings.TrimSpace(part) == item {
			return true
		}
	}
	return false
}
