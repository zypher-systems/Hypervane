package httpapi

import (
	"net/http"

	"hypervane/internal/pve"
)

func (s *Server) handleSDNStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.SDNStatus(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleApplySDN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	doUPID(w, func() (string, error) { return c.ApplySDN(r.Context(), sdnLockBody(r)) })
}

func (s *Server) handleRollbackSDN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.RollbackSDN(r.Context(), sdnLockBody(r)); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func sdnLockBody(r *http.Request) map[string]any {
	body := map[string]any{}
	_ = decodeJSON(r, &body)
	if body == nil {
		body = map[string]any{}
	}
	if t := r.URL.Query().Get("lock-token"); t != "" {
		body["lock-token"] = t
	}
	if r.URL.Query().Get("force") == "1" {
		body["force"] = 1
	}
	if r.URL.Query().Get("allow-pending") == "1" {
		body["allow-pending"] = 1
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func (s *Server) handleSDNDryRun(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.SDNDryRun(r.Context(), r.URL.Query().Get("node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleLockSDN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	body := sdnLockBody(r)
	allow := r.URL.Query().Get("allow-pending") == "1"
	if v, ok := body["allow-pending"]; ok {
		allow = v == 1 || v == true || v == "1"
	}
	item, err := c.LockSDN(r.Context(), allow)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUnlockSDN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	body := sdnLockBody(r)
	token, _ := body["lock-token"].(string)
	force := r.URL.Query().Get("force") == "1"
	if v, ok := body["force"]; ok {
		force = v == 1 || v == true || v == "1"
	}
	item, err := c.UnlockSDN(r.Context(), token, force)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSDNIPAMStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.SDNIPAMStatus(r.Context(), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSDNFabricInterfaces(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.SDNFabricInterfaces(r.Context(), urlParam(r, "node"), urlParam(r, "id"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateSDNVNetIP(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := c.CreateSDNVNetIP(r.Context(), urlParam(r, "vnet"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteSDNVNetIP(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	body := map[string]any{}
	if ip := r.URL.Query().Get("ip"); ip != "" {
		body["ip"] = ip
	}
	if mac := r.URL.Query().Get("mac"); mac != "" {
		body["mac"] = mac
	}
	item, err := c.DeleteSDNVNetIP(r.Context(), urlParam(r, "vnet"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSDNZones(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.SDNZones(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateSDNZone(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNZone(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNZone(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNZone(r.Context(), urlParam(r, "zone"), body)
	})
}

func (s *Server) handleDeleteSDNZone(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNZone(r.Context(), urlParam(r, "zone")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNVNets(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.SDNVNets(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateSDNVNet(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNVNet(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNVNet(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNVNet(r.Context(), urlParam(r, "vnet"), body)
	})
}

func (s *Server) handleDeleteSDNVNet(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNVNet(r.Context(), urlParam(r, "vnet")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNSubnets(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.SDNSubnets(r.Context(), urlParam(r, "vnet"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateSDNSubnet(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNSubnet(r.Context(), urlParam(r, "vnet"), body)
	})
}

func (s *Server) handleUpdateSDNSubnet(w http.ResponseWriter, r *http.Request) {
	subnet := r.URL.Query().Get("subnet")
	if subnet == "" {
		writeError(w, http.StatusBadRequest, "subnet is required")
		return
	}
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNSubnet(r.Context(), urlParam(r, "vnet"), subnet, body)
	})
}

func (s *Server) handleDeleteSDNSubnet(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	subnet := r.URL.Query().Get("subnet")
	if subnet == "" {
		writeError(w, http.StatusBadRequest, "subnet is required")
		return
	}
	if err := c.DeleteSDNSubnet(r.Context(), urlParam(r, "vnet"), subnet); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNVNetIPs(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.SDNVNetIPs(r.Context(), urlParam(r, "vnet"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSDNControllers(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.SDNControllers(r.Context()) })
}

func (s *Server) handleCreateSDNController(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNController(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNController(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNController(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNController(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNController(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNIPAMs(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.SDNIPAMs(r.Context()) })
}

func (s *Server) handleCreateSDNIPAM(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNIPAM(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNIPAM(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNIPAM(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNIPAM(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNIPAM(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNDNS(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.SDNDNS(r.Context()) })
}

func (s *Server) handleCreateSDNDNS(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNDNS(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNDNS(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNDNS(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNDNS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNDNS(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNFabrics(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.SDNFabrics(r.Context()) })
}

func (s *Server) handleCreateSDNFabric(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNFabric(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNFabric(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNFabric(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNFabric(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNFabric(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNFabricNodes(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNFabricNodes(r.Context(), urlParam(r, "id"))
	})
}

func (s *Server) handleCreateSDNFabricNode(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNFabricNode(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleUpdateSDNFabricNode(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNFabricNode(r.Context(), urlParam(r, "id"), urlParam(r, "node"), body)
	})
}

func (s *Server) handleDeleteSDNFabricNode(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNFabricNode(r.Context(), urlParam(r, "id"), urlParam(r, "node")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNRouteMaps(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNRouteMaps(r.Context())
	})
}

func (s *Server) handleCreateSDNRouteMap(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNRouteMap(r.Context(), body)
	})
}

func (s *Server) handleDeleteSDNRouteMap(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNRouteMap(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNRouteMapEntries(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNRouteMapEntries(r.Context(), urlParam(r, "id"))
	})
}

func (s *Server) handleCreateSDNRouteMapEntry(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNRouteMapEntry(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleUpdateSDNRouteMapEntry(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNRouteMapEntry(r.Context(), urlParam(r, "id"), urlParam(r, "order"), body)
	})
}

func (s *Server) handleDeleteSDNRouteMapEntry(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNRouteMapEntry(r.Context(), urlParam(r, "id"), urlParam(r, "order")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNPrefixLists(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNPrefixLists(r.Context())
	})
}

func (s *Server) handleCreateSDNPrefixList(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNPrefixList(r.Context(), body)
	})
}

func (s *Server) handleUpdateSDNPrefixList(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNPrefixList(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNPrefixList(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNPrefixList(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNPrefixListEntries(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNPrefixListEntries(r.Context(), urlParam(r, "id"))
	})
}

func (s *Server) handleCreateSDNPrefixListEntry(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNPrefixListEntry(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleUpdateSDNPrefixListEntry(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateSDNPrefixListEntry(r.Context(), urlParam(r, "id"), urlParam(r, "seq"), body)
	})
}

func (s *Server) handleDeleteSDNPrefixListEntry(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNPrefixListEntry(r.Context(), urlParam(r, "id"), urlParam(r, "seq")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNFabricRouteMaps(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNFabricRouteMaps(r.Context(), urlParam(r, "id"))
	})
}

func (s *Server) handleCreateSDNFabricRouteMap(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNFabricRouteMap(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNFabricRouteMap(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNFabricRouteMap(r.Context(), urlParam(r, "id"), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSDNFabricPrefixLists(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) {
		return c.SDNFabricPrefixLists(r.Context(), urlParam(r, "id"))
	})
}

func (s *Server) handleCreateSDNFabricPrefixList(w http.ResponseWriter, r *http.Request) {
	s.handleSDNMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateSDNFabricPrefixList(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteSDNFabricPrefixList(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteSDNFabricPrefixList(r.Context(), urlParam(r, "id"), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNodeSDN(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NodeSDN(r.Context(), urlParam(r, "node"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func vnetFWScope(r *http.Request) pve.FWScope {
	return pve.FWScope{Level: "vnet", VNet: urlParam(r, "vnet")}
}

func (s *Server) handleVNetFWRules(w http.ResponseWriter, r *http.Request) {
	s.handleFWRules(w, r, vnetFWScope(r))
}

func (s *Server) handleCreateVNetFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleCreateFWRule(w, r, vnetFWScope(r))
}

func (s *Server) handleUpdateVNetFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWRule(w, r, vnetFWScope(r))
}

func (s *Server) handleDeleteVNetFWRule(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteFWRule(w, r, vnetFWScope(r))
}

func (s *Server) handleVNetFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleFWOptions(w, r, vnetFWScope(r))
}

func (s *Server) handleUpdateVNetFWOptions(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateFWOptions(w, r, vnetFWScope(r))
}

func (s *Server) handleVNetFWLog(w http.ResponseWriter, r *http.Request) {
	s.handleFWLog(w, r, vnetFWScope(r))
}

func (s *Server) handleSDNMapCreate(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, map[string]any) error) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := fn(c, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleSDNMapUpdate(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, map[string]any) error) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := fn(c, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFeatureList(w http.ResponseWriter, r *http.Request, fn func(*pve.Client) (pve.FeatureList, error)) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := fn(c)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
