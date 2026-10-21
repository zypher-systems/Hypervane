package httpapi

import (
	"net/http"

	"hypervane/internal/pve"
)

func (s *Server) handleStorages(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.Storages(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateStorage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateStorage(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateStorage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateStorage(r.Context(), urlParam(r, "storage"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.Storage(r.Context(), urlParam(r, "storage"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteStorage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteStorage(r.Context(), urlParam(r, "storage")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleScanStorage(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	query := map[string]any{}
	for k, vals := range r.URL.Query() {
		if k == "node" || len(vals) == 0 {
			continue
		}
		query[k] = vals[0]
	}
	item, err := c.ScanStorage(r.Context(), node, urlParam(r, "kind"), query)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleImportMetadata(w http.ResponseWriter, r *http.Request) {
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
	item, err := c.ImportMetadata(r.Context(), node, urlParam(r, "storage"), volume)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleQueryURLMetadata(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	rawURL := r.URL.Query().Get("url")
	if node == "" || rawURL == "" {
		writeError(w, http.StatusBadRequest, "node and url are required")
		return
	}
	item, err := c.QueryURLMetadata(r.Context(), node, rawURL)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleStorageContent(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	items, err := c.StorageContent(r.Context(), node, urlParam(r, "storage"), r.URL.Query().Get("content"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUpdateVolumeNotes(w http.ResponseWriter, r *http.Request) {
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
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	notes, _ := body["notes"].(string)
	item, err := c.UpdateVolumeNotes(r.Context(), node, urlParam(r, "storage"), volume, notes)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleOCIPull(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := c.OCIPull(r.Context(), node, urlParam(r, "storage"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleStorageIdentity(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	item, err := c.StorageIdentity(r.Context(), node, urlParam(r, "storage"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateStorageIdentity(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := c.UpdateStorageIdentity(r.Context(), node, urlParam(r, "storage"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
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
	upid, err := c.DeleteVolume(r.Context(), node, urlParam(r, "storage"), volume)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleStorageRRD(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	data, err := c.StorageRRD(r.Context(), node, urlParam(r, "storage"), queryDefault(r, "timeframe", "hour"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := r.URL.Query().Get("node")
	guest := r.URL.Query().Get("guest")
	storage := r.URL.Query().Get("storage")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "archive"
	}

	var tasks []pve.Task
	var err error
	usedArchive := false
	if node != "" {
		tasks, err = c.NodeTasks(r.Context(), node, pve.NodeTaskQuery{
			VMID:       guest,
			Source:     source,
			Limit:      100,
			UserFilter: r.URL.Query().Get("user"),
			TypeFilter: r.URL.Query().Get("type"),
		})
		if err == nil {
			usedArchive = true
		} else if pve.IsNotFound(err) {
			err = nil
			tasks = nil
		}
		if err != nil {
			writePVEError(w, err)
			return
		}
	}
	if !usedArchive {
		tasks, err = s.poller.CachedTasks(r.Context(), s.requestConnectionID(r))
		if err != nil || len(tasks) == 0 {
			tasks, err = c.ClusterTasks(r.Context())
			if err != nil {
				writePVEError(w, err)
				return
			}
		}
	}
	if node != "" || guest != "" || storage != "" {
		filtered := make([]pve.Task, 0, len(tasks))
		for _, t := range tasks {
			if node != "" && t.Node != node {
				continue
			}
			if guest != "" && t.ID != guest {
				continue
			}
			if storage != "" && t.ID != storage {
				continue
			}
			filtered = append(filtered, t)
		}
		tasks = filtered
	}
	if acl, filter := s.requestACL(r); filter {
		tasks = pve.FilterTasks(tasks, acl)
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid := urlParam(r, "upid")
	node := r.URL.Query().Get("node")
	if node == "" {
		node = pve.ParseUPIDNode(upid)
	}
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	st, err := c.TaskStatus(r.Context(), node, upid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	lines, total, err := c.TaskLogTail(r.Context(), node, upid, 1000)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": st, "log": lines, "total": total})
}

func (s *Server) handleStopTask(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	upid := urlParam(r, "upid")
	node := r.URL.Query().Get("node")
	if node == "" {
		node = pve.ParseUPIDNode(upid)
	}
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	if err := c.StopTask(r.Context(), node, upid); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
