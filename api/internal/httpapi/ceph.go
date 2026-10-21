package httpapi

import (
	"net/http"
	"strconv"

	"hypervane/internal/pve"
)

func (s *Server) resolveCephNode(w http.ResponseWriter, r *http.Request, c *pve.Client) string {
	if n := urlParam(r, "node"); n != "" {
		return n
	}
	if n := r.URL.Query().Get("node"); n != "" {
		return n
	}
	resources, err := c.ClusterResources(r.Context())
	if err != nil {
		writePVEError(w, err)
		return ""
	}
	var fallback string
	for _, res := range resources {
		if res.Type != "node" {
			continue
		}
		name := res.Node
		if name == "" {
			name = res.Name
		}
		if name == "" {
			continue
		}
		if res.Status == "online" {
			return name
		}
		if fallback == "" {
			fallback = name
		}
	}
	if fallback == "" {
		writeError(w, http.StatusBadRequest, "no node available for ceph")
	}
	return fallback
}

func writeUPID(w http.ResponseWriter, upid string, err error) {
	if err != nil {
		writePVEError(w, err)
		return
	}
	if upid == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func doUPID(w http.ResponseWriter, fn func() (string, error)) {
	upid, err := fn()
	writeUPID(w, upid, err)
}

func (s *Server) handleCephStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephStatus(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephFlags(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephFlags(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateCephFlags(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateCephFlags(r.Context(), node, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCephConfig(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephConfig(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateCephCrush(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateCephCrush(r.Context(), node, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCreateCephRule(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateCephRule(r.Context(), node, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleCephCrush(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephCrush(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephRules(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephRules(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephLog(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephLog(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCephReleases(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephReleases(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephRestartBulk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		body = map[string]any{}
	}
	doUPID(w, func() (string, error) { return c.CephRestartBulk(r.Context(), body) })
}

func (s *Server) handleNodeCephRestartBulk(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		body = map[string]any{}
	}
	doUPID(w, func() (string, error) { return c.NodeCephRestartBulk(r.Context(), urlParam(r, "node"), body) })
}

func (s *Server) handleRestartCeph(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.RestartCeph(r.Context(), node) })
}

func (s *Server) handleStartCeph(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.StartCeph(r.Context(), node, r.URL.Query().Get("service")) })
}

func (s *Server) handleStopCeph(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.StopCeph(r.Context(), node, r.URL.Query().Get("service")) })
}

func (s *Server) handleCephCmdSafety(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephCmdSafety(r.Context(), node, r.URL.Query().Get("service"), r.URL.Query().Get("id"), r.URL.Query().Get("action"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephMetadata(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephMetadata(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephRGW(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephRGW(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateCephRGW(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, body, ok := decodeNamedCeph(w, r)
	if !ok {
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephRGW(r.Context(), node, id, body) })
}

func (s *Server) handleDeleteCephRGW(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.DeleteCephRGW(r.Context(), node, urlParam(r, "id")) })
}

func (s *Server) handleCephRGWUsers(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephRGWUsers(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateCephRGWUser(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateCephRGWUser(r.Context(), node, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteCephRGWUser(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	if err := c.DeleteCephRGWUser(r.Context(), node, urlParam(r, "uid")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCephOSDMetadata(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDObject(w, r, func(c *pve.Client, node string, id int) (pve.FeatureObject, error) {
		return c.CephOSDMetadata(r.Context(), node, id)
	})
}

func (s *Server) handleCephOSDLVInfo(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDObject(w, r, func(c *pve.Client, node string, id int) (pve.FeatureObject, error) {
		return c.CephOSDLVInfo(r.Context(), node, id)
	})
}

func (s *Server) handleCephOSDObject(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, string, int) (pve.FeatureObject, error)) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, err := strconv.Atoi(urlParam(r, "osdid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid osdid")
		return
	}
	item, err := fn(c, node, id)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephPoolStatus(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephPoolStatus(r.Context(), node, urlParam(r, "pool"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCephMons(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephMons(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCephMon(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, body, ok := decodeNamedCeph(w, r)
	if !ok {
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephMon(r.Context(), node, id, body) })
}

func (s *Server) handleDeleteCephMon(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.DeleteCephMon(r.Context(), node, urlParam(r, "id")) })
}

func (s *Server) handleCephMgrs(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephMgrs(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCephMgr(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, body, ok := decodeNamedCeph(w, r)
	if !ok {
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephMgr(r.Context(), node, id, body) })
}

func (s *Server) handleDeleteCephMgr(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.DeleteCephMgr(r.Context(), node, urlParam(r, "id")) })
}

func (s *Server) handleCephMDS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephMDS(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCephMDS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, body, ok := decodeNamedCeph(w, r)
	if !ok {
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephMDS(r.Context(), node, id, body) })
}

func (s *Server) handleDeleteCephMDS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.DeleteCephMDS(r.Context(), node, urlParam(r, "id")) })
}

func (s *Server) handleCephOSDs(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephOSDs(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCephOSDIn(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDAction(w, r, func(c *pve.Client, node string, id int) (string, error) {
		return c.CephOSDIn(r.Context(), node, id)
	})
}

func (s *Server) handleCephOSDOut(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDAction(w, r, func(c *pve.Client, node string, id int) (string, error) {
		return c.CephOSDOut(r.Context(), node, id)
	})
}

func (s *Server) handleCephOSDScrub(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDAction(w, r, func(c *pve.Client, node string, id int) (string, error) {
		deep := r.URL.Query().Get("deep") == "1" || r.URL.Query().Get("deep") == "true"
		return c.CephOSDScrub(r.Context(), node, id, deep)
	})
}

func (s *Server) handleDestroyCephOSD(w http.ResponseWriter, r *http.Request) {
	s.handleCephOSDAction(w, r, func(c *pve.Client, node string, id int) (string, error) {
		return c.DestroyCephOSD(r.Context(), node, id, true)
	})
}

func (s *Server) handleCephOSDAction(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, string, int) (string, error)) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, err := strconv.Atoi(urlParam(r, "osdid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid osdid")
		return
	}
	doUPID(w, func() (string, error) { return fn(c, node, id) })
}

func (s *Server) handleCephPools(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	items, err := c.CephPools(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateCephPool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephPool(r.Context(), node, body) })
}

func (s *Server) handleUpdateCephPool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateCephPool(r.Context(), node, urlParam(r, "pool"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteCephPool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	doUPID(w, func() (string, error) { return c.DeleteCephPool(r.Context(), node, urlParam(r, "pool")) })
}

func (s *Server) handleCephFS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	item, err := c.CephFSList(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateCephFS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := s.resolveCephNode(w, r, c)
	if node == "" {
		return
	}
	id, body, ok := decodeNamedCeph(w, r)
	if !ok {
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephFS(r.Context(), node, id, body) })
}

func (s *Server) handleInstallCeph(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		body = map[string]any{}
	}
	doUPID(w, func() (string, error) { return c.InstallCeph(r.Context(), urlParam(r, "node"), body) })
}

func (s *Server) handleInitCeph(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		body = map[string]any{}
	}
	doUPID(w, func() (string, error) { return c.InitCeph(r.Context(), urlParam(r, "node"), body) })
}

func (s *Server) handleCreateNodeCephOSD(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	doUPID(w, func() (string, error) { return c.CreateCephOSD(r.Context(), urlParam(r, "node"), body) })
}

func decodeNamedCeph(w http.ResponseWriter, r *http.Request) (string, map[string]any, bool) {
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return "", nil, false
	}
	id := stringVal(body["name"])
	if id == "" {
		id = stringVal(body["id"])
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return "", nil, false
	}
	return id, body, true
}

func stringVal(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
