package httpapi

import (
	"net/http"
	"strconv"

	"hypervane/internal/pve"
)

func (s *Server) handleClusterOptions(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ClusterOptions(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateClusterOptions(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateClusterOptions(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePools(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	items, err := c.Pools(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.Pool(r.Context(), urlParam(r, "poolid"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreatePool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body struct {
		PoolID  string `json:"poolid"`
		Comment string `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil || body.PoolID == "" {
		writeError(w, http.StatusBadRequest, "poolid is required")
		return
	}
	if err := c.CreatePool(r.Context(), body.PoolID, body.Comment); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdatePool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdatePool(r.Context(), urlParam(r, "poolid"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeletePool(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeletePool(r.Context(), urlParam(r, "poolid")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleACMEAccounts(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ACMEAccounts(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateACMEAccount(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	upid, err := c.CreateACMEAccount(r.Context(), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleUpdateACMEAccount(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	upid, err := c.UpdateACMEAccount(r.Context(), urlParam(r, "name"), body)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleDeleteACMEAccount(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteACMEAccount(r.Context(), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleACMEPlugins(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ACMEPlugins(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateACMEPlugin(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateACMEPlugin(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateACMEPlugin(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateACMEPlugin(r.Context(), urlParam(r, "id"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteACMEPlugin(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteACMEPlugin(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNotificationEndpoints(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NotificationEndpoints(r.Context(), urlParam(r, "kind"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateNotificationEndpoint(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateNotificationEndpoint(r.Context(), urlParam(r, "kind"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateNotificationEndpoint(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNotificationEndpoint(r.Context(), urlParam(r, "kind"), urlParam(r, "name"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteNotificationEndpoint(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteNotificationEndpoint(r.Context(), urlParam(r, "kind"), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNotificationMatchers(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NotificationMatchers(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateNotificationMatcher(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateNotificationMatcher(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateNotificationMatcher(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateNotificationMatcher(r.Context(), urlParam(r, "name"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteNotificationMatcher(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteNotificationMatcher(r.Context(), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNotificationTargets(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.NotificationTargets(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleClusterConfig(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ClusterConfig(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleClusterRemotes(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.ClusterRemotes(r.Context()) })
}

func (s *Server) handleCreateClusterRemote(w http.ResponseWriter, r *http.Request) {
	s.handleMapCreate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.CreateClusterRemote(r.Context(), body)
	})
}

func (s *Server) handleUpdateClusterRemote(w http.ResponseWriter, r *http.Request) {
	s.handleMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateClusterRemote(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteClusterRemote(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteClusterRemote(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateClusterTotem(w http.ResponseWriter, r *http.Request) {
	s.handleMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateClusterTotem(r.Context(), body)
	})
}

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateCluster(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleClusterJoinInfo(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ClusterJoinInfo(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleJoinCluster(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.JoinCluster(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleClusterConfigNodes(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ClusterConfigNodes(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAddClusterNode(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	node, _ := body["node"].(string)
	if node == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	delete(body, "node")
	if err := c.AddClusterNode(r.Context(), node, body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteClusterNode(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteClusterNode(r.Context(), urlParam(r, "node")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleClusterQDevice(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ClusterQDevice(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateClusterQDevice(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateClusterQDevice(r.Context(), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteClusterQDevice(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteClusterQDevice(r.Context()); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMappings(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.Mappings(r.Context(), urlParam(r, "kind"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateMapping(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.CreateMapping(r.Context(), urlParam(r, "kind"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateMapping(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateMapping(r.Context(), urlParam(r, "kind"), urlParam(r, "id"), body); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteMapping(r.Context(), urlParam(r, "kind"), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMetricServers(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.MetricServers(r.Context()) })
}

func (s *Server) handleCreateMetricServer(w http.ResponseWriter, r *http.Request) {
	s.handleMapCreate(w, r, func(c *pve.Client, body map[string]any) error { return c.CreateMetricServer(r.Context(), body) })
}

func (s *Server) handleUpdateMetricServer(w http.ResponseWriter, r *http.Request) {
	s.handleMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateMetricServer(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteMetricServer(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteMetricServer(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleClusterLog(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	max := 50
	if v := r.URL.Query().Get("max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			max = n
		}
	}
	item, err := c.ClusterLog(r.Context(), max)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleClusterJobs(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.ClusterJobs(r.Context()) })
}

func (s *Server) handleRealmSyncJobs(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.RealmSyncJobs(r.Context()) })
}

func (s *Server) handleCreateRealmSyncJob(w http.ResponseWriter, r *http.Request) {
	s.handleMapCreate(w, r, func(c *pve.Client, body map[string]any) error { return c.CreateRealmSyncJob(r.Context(), body) })
}

func (s *Server) handleUpdateRealmSyncJob(w http.ResponseWriter, r *http.Request) {
	s.handleMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateRealmSyncJob(r.Context(), urlParam(r, "id"), body)
	})
}

func (s *Server) handleDeleteRealmSyncJob(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteRealmSyncJob(r.Context(), urlParam(r, "id")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleScheduleAnalyze(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ScheduleAnalyze(r.Context(), r.URL.Query().Get("schedule"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleACMEDirectories(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.ACMEDirectories(r.Context()) })
}

func (s *Server) handleACMEMeta(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ACMEMeta(r.Context(), r.URL.Query().Get("directory"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleACMETOS(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ACMETOS(r.Context(), r.URL.Query().Get("directory"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleACMEChallengeSchema(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.ACMEChallengeSchema(r.Context(), r.URL.Query().Get("plugin"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleTestNotificationTarget(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	res, err := c.TestNotificationTarget(r.Context(), urlParam(r, "name"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCustomCPUModels(w http.ResponseWriter, r *http.Request) {
	s.handleFeatureList(w, r, func(c *pve.Client) (pve.FeatureList, error) { return c.CustomCPUModels(r.Context()) })
}

func (s *Server) handleCreateCustomCPUModel(w http.ResponseWriter, r *http.Request) {
	s.handleMapCreate(w, r, func(c *pve.Client, body map[string]any) error { return c.CreateCustomCPUModel(r.Context(), body) })
}

func (s *Server) handleUpdateCustomCPUModel(w http.ResponseWriter, r *http.Request) {
	s.handleMapUpdate(w, r, func(c *pve.Client, body map[string]any) error {
		return c.UpdateCustomCPUModel(r.Context(), urlParam(r, "name"), body)
	})
}

func (s *Server) handleDeleteCustomCPUModel(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if err := c.DeleteCustomCPUModel(r.Context(), urlParam(r, "name")); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleOpenID(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	item, err := c.OpenID(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleMapCreate(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, map[string]any) error) {
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

func (s *Server) handleMapUpdate(w http.ResponseWriter, r *http.Request, fn func(*pve.Client, map[string]any) error) {
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
