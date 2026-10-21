package httpapi

import (
	"net/http"

	"hypervane/internal/pve"
)

func (s *Server) handleHAStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	current, err := c.HACurrent(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	manager, err := c.HAManagerStatus(r.Context())
	if err != nil {
		manager = nil
	}
	writeJSON(w, http.StatusOK, pve.HAStatus{Current: current, Manager: manager})
}

func (s *Server) handleArmHA(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.ArmHA(r.Context()); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDisarmHA(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		ResourceMode string `json:"resource-mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		body.ResourceMode = r.URL.Query().Get("resource-mode")
	}
	if body.ResourceMode != "freeze" && body.ResourceMode != "ignore" {
		writeError(w, http.StatusBadRequest, "resource-mode must be freeze or ignore")
		return
	}
	if err := c.DisarmHA(r.Context(), body.ResourceMode); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHAResources(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.HAResources(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateHAResource(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HAResourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.SID == "" && req.VMID == 0 {
		writeError(w, http.StatusBadRequest, "sid or vmid is required")
		return
	}
	if req.State == "" {
		req.State = "started"
	}
	if err := c.CreateHAResource(r.Context(), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleHAResource(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.HAResource(r.Context(), urlParam(r, "sid"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateHAResource(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HAResourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateHAResource(r.Context(), urlParam(r, "sid"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteHAResource(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	purge := true
	if r.URL.Query().Has("purge") {
		purge = queryFlag(r, "purge")
	}
	if err := c.DeleteHAResource(r.Context(), urlParam(r, "sid"), purge); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHAMigrate(w http.ResponseWriter, r *http.Request) {
	s.handleHAMove(w, r, "migrate")
}

func (s *Server) handleHARelocate(w http.ResponseWriter, r *http.Request) {
	s.handleHAMove(w, r, "relocate")
}

func (s *Server) handleHAMove(w http.ResponseWriter, r *http.Request, action string) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HAMoveRequest
	if err := decodeJSON(r, &req); err != nil || req.Node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	var (
		upid string
		err  error
	)
	if action == "migrate" {
		upid, err = c.HAMigrate(r.Context(), urlParam(r, "sid"), req.Node)
	} else {
		upid, err = c.HARelocate(r.Context(), urlParam(r, "sid"), req.Node)
	}
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid, "queued": "1"})
}

func (s *Server) handleHAGroups(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.HAGroups(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateHAGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HAGroupRequest
	if err := decodeJSON(r, &req); err != nil || req.Group == "" || req.Nodes == "" {
		writeError(w, http.StatusBadRequest, "group and nodes are required")
		return
	}
	if err := c.CreateHAGroup(r.Context(), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleHAGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.HAGroup(r.Context(), urlParam(r, "group"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateHAGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HAGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateHAGroup(r.Context(), urlParam(r, "group"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteHAGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteHAGroup(r.Context(), urlParam(r, "group")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHARules(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.HARules(r.Context())
	if err != nil {
		if apiErr, ok := pve.IsAPIError(err); ok && apiErr.Status == 404 {
			writeJSON(w, http.StatusOK, pve.HARulesResponse{Supported: false, Rules: []pve.HARule{}})
			return
		}
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pve.HARulesResponse{Supported: true, Rules: items})
}

func (s *Server) handleCreateHARule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HARuleRequest
	if err := decodeJSON(r, &req); err != nil || req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}
	if err := c.CreateHARule(r.Context(), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateHARule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.HARuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateHARule(r.Context(), urlParam(r, "rule"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteHARule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteHARule(r.Context(), urlParam(r, "rule")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
