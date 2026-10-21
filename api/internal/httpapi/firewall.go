package httpapi

import (
	"net/http"
	"strconv"

	"hypervane/internal/pve"
)

func (s *Server) handleDCFWRules(w http.ResponseWriter, r *http.Request) {
	s.handleFWRules(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleCreateDCFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWRule(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleUpdateDCFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWRule(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDeleteDCFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWRule(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleFWOptions(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleUpdateDCFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWOptions(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWLog(w http.ResponseWriter, r *http.Request) {
	s.handleFWLog(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWAliases(w http.ResponseWriter, r *http.Request) {
	s.handleFWAliases(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleCreateDCFWAlias(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWAlias(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleUpdateDCFWAlias(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWAlias(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDeleteDCFWAlias(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWAlias(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWIPSets(w http.ResponseWriter, r *http.Request) {
	s.handleFWIPSets(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleCreateDCFWIPSet(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWIPSet(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDeleteDCFWIPSet(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWIPSet(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWIPSetCIDRs(w http.ResponseWriter, r *http.Request) {
	s.handleFWIPSetCIDRs(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleCreateDCFWIPSetCIDR(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWIPSetCIDR(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDeleteDCFWIPSetCIDR(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWIPSetCIDR(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleDCFWMacros(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWMacros(r.Context())
	if err != nil {
		if pve.IsNotFound(err) {
			writeJSON(w, http.StatusOK, []pve.FWMacro{})
			return
		}
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDCFWRefs(w http.ResponseWriter, r *http.Request) {
	s.handleFWRefs(w, r, pve.FWScope{Level: "dc"})
}

func (s *Server) handleNodeFWRules(w http.ResponseWriter, r *http.Request) {
	s.handleFWRules(w, r, nodeFWScope(r))
}

func (s *Server) handleCreateNodeFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWRule(w, r, nodeFWScope(r))
}

func (s *Server) handleUpdateNodeFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWRule(w, r, nodeFWScope(r))
}

func (s *Server) handleDeleteNodeFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWRule(w, r, nodeFWScope(r))
}

func (s *Server) handleNodeFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleFWOptions(w, r, nodeFWScope(r))
}

func (s *Server) handleUpdateNodeFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWOptions(w, r, nodeFWScope(r))
}

func (s *Server) handleNodeFWLog(w http.ResponseWriter, r *http.Request) {
	s.handleFWLog(w, r, nodeFWScope(r))
}

func (s *Server) handleGuestFWRules(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWRules(w, r, scope)
}

func (s *Server) handleCreateGuestFWRule(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleCreateFWRule(w, r, scope)
}

func (s *Server) handleUpdateGuestFWRule(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleUpdateFWRule(w, r, scope)
}

func (s *Server) handleDeleteGuestFWRule(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleDeleteFWRule(w, r, scope)
}

func (s *Server) handleGuestFWOptions(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWOptions(w, r, scope)
}

func (s *Server) handleUpdateGuestFWOptions(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleUpdateFWOptions(w, r, scope)
}

func (s *Server) handleGuestFWLog(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWLog(w, r, scope)
}

func (s *Server) handleGuestFWAliases(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWAliases(w, r, scope)
}

func (s *Server) handleCreateGuestFWAlias(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleCreateFWAlias(w, r, scope)
}

func (s *Server) handleUpdateGuestFWAlias(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleUpdateFWAlias(w, r, scope)
}

func (s *Server) handleDeleteGuestFWAlias(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleDeleteFWAlias(w, r, scope)
}

func (s *Server) handleGuestFWIPSets(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWIPSets(w, r, scope)
}

func (s *Server) handleCreateGuestFWIPSet(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleCreateFWIPSet(w, r, scope)
}

func (s *Server) handleDeleteGuestFWIPSet(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleDeleteFWIPSet(w, r, scope)
}

func (s *Server) handleGuestFWIPSetCIDRs(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWIPSetCIDRs(w, r, scope)
}

func (s *Server) handleCreateGuestFWIPSetCIDR(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleCreateFWIPSetCIDR(w, r, scope)
}

func (s *Server) handleDeleteGuestFWIPSetCIDR(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleDeleteFWIPSetCIDR(w, r, scope)
}

func (s *Server) handleGuestFWRefs(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.guestFWScope(w, r)
	if !ok {
		return
	}
	s.handleFWRefs(w, r, scope)
}

func (s *Server) handleFWGroups(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWGroups(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWGroup
	if err := decodeJSON(r, &req); err != nil || req.Group == "" {
		writeError(w, http.StatusBadRequest, "group is required")
		return
	}
	if err := c.CreateFWGroup(r.Context(), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWGroup(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteFWGroup(r.Context(), urlParam(r, "group")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWGroupRules(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWGroupRules(r.Context(), urlParam(r, "group"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWGroupRule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateFWGroupRule(r.Context(), urlParam(r, "group"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateFWGroupRule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	pos, err := pve.ParseFWPos(urlParam(r, "pos"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req pve.FWRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateFWGroupRule(r.Context(), urlParam(r, "group"), pos, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWGroupRule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	pos, err := pve.ParseFWPos(urlParam(r, "pos"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := c.DeleteFWGroupRule(r.Context(), urlParam(r, "group"), pos, r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWRules(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWRules(r.Context(), scope)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWRule(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateFWRule(r.Context(), scope, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateFWRule(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	pos, err := pve.ParseFWPos(urlParam(r, "pos"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req pve.FWRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateFWRule(r.Context(), scope, pos, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWRule(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	pos, err := pve.ParseFWPos(urlParam(r, "pos"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := c.DeleteFWRule(r.Context(), scope, pos, r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWOptions(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	opts, err := c.FWOptions(r.Context(), scope)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, opts)
}

func (s *Server) handleUpdateFWOptions(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWOptions
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateFWOptions(r.Context(), scope, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWLog(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	lines, err := c.FWLog(r.Context(), scope, start)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) handleFWAliases(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWAliases(r.Context(), scope)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWAlias(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWAlias
	if err := decodeJSON(r, &req); err != nil || req.Name == "" || req.CIDR == "" {
		writeError(w, http.StatusBadRequest, "name and cidr are required")
		return
	}
	if err := c.CreateFWAlias(r.Context(), scope, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateFWAlias(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWAlias
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateFWAlias(r.Context(), scope, urlParam(r, "name"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWAlias(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteFWAlias(r.Context(), scope, urlParam(r, "name"), r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWIPSets(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWIPSets(r.Context(), scope)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWIPSet(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWIPSet
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := c.CreateFWIPSet(r.Context(), scope, req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWIPSet(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteFWIPSet(r.Context(), scope, urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWIPSetCIDRs(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.FWIPSetCIDRs(r.Context(), scope, urlParam(r, "name"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateFWIPSetCIDR(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.FWIPSetCIDR
	if err := decodeJSON(r, &req); err != nil || req.CIDR == "" {
		writeError(w, http.StatusBadRequest, "cidr is required")
		return
	}
	if err := c.CreateFWIPSetCIDR(r.Context(), scope, urlParam(r, "name"), req); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFWIPSetCIDR(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	cidr := r.URL.Query().Get("cidr")
	if cidr == "" {
		writeError(w, http.StatusBadRequest, "cidr is required")
		return
	}
	if err := c.DeleteFWIPSetCIDR(r.Context(), scope, urlParam(r, "name"), cidr, r.URL.Query().Get("digest")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFWRefs(w http.ResponseWriter, r *http.Request, scope pve.FWScope) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	refs, err := c.FWRefs(r.Context(), scope)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

func nodeFWScope(r *http.Request) pve.FWScope {
	return pve.FWScope{Level: "node", Node: urlParam(r, "node")}
}

func (s *Server) guestFWScope(w http.ResponseWriter, r *http.Request) (pve.FWScope, bool) {
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return pve.FWScope{}, false
	}
	return pve.FWScope{Level: "guest", Node: node, Kind: kind, VMID: vmid}, true
}
