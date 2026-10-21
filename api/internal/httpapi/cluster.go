package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"hypervane/internal/pve"
)

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	status, resources, err := s.poller.CachedResources(r.Context(), s.requestConnectionID(r))
	if err != nil || len(resources) == 0 {
		status, err = c.ClusterStatus(r.Context())
		if err != nil {
			writePVEError(w, err)
			return
		}
		resources, err = c.ClusterResources(r.Context())
		if err != nil {
			writePVEError(w, err)
			return
		}
	}
	if acl, filter := s.requestACL(r); filter {
		resources = pve.FilterResources(resources, acl)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    status,
		"resources": resources,
	})
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	s.handleSummary(w, r)
}

func (s *Server) handleNextID(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	id, err := c.NextID(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"vmid": id})
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	resources, err := c.ClusterResources(r.Context())
	if err != nil {
		writePVEError(w, err)
		return
	}
	var nodes []pve.Resource
	for _, res := range resources {
		if res.Type == "node" {
			nodes = append(nodes, res)
		}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := urlParam(r, "node")
	st, err := c.NodeStatus(r.Context(), node)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleNodeRRD(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	data, err := c.NodeRRD(r.Context(), urlParam(r, "node"), queryDefault(r, "timeframe", "hour"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleNodeIORRD(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := urlParam(r, "node")
	tf := queryDefault(r, "timeframe", "hour")
	data, err := c.NodeIORRD(r.Context(), node, tf)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if s.poller != nil {
		if iops, err := s.poller.NodeIOPS(r.Context(), s.requestConnectionID(r), node, tf); err == nil && len(iops) > 0 {
			data.IOPS = iops
		}
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleGuestIOPSLatest(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"samples": map[string]any{}})
		return
	}
	rows, err := s.store.LatestGuestIOPS(r.Context(), s.requestConnectionID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	samples := map[string]map[string]float64{}
	for vmid, p := range rows {
		samples[strconv.Itoa(vmid)] = map[string]float64{"rd": p.Rd, "wr": p.Wr, "wait": p.Wait}
	}
	writeJSON(w, http.StatusOK, map[string]any{"samples": samples})
}

func (s *Server) handleRing(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if len(key) < 8 || key[:8] == "http://" {
		writeError(w, http.StatusBadRequest, "invalid key")
		return
	}
	ring, err := s.poller.Ring(r.Context(), "hypervane:"+s.requestConnectionID(r)+":ring:"+key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ring)
}

func (s *Server) handleGuest(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	status, err := c.GuestStatus(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	cfg, err := c.GuestConfig(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	pending, err := c.GuestPending(r.Context(), node, kind, vmid)
	if err != nil {
		pending = nil
	}
	if kind == "qemu" && status.Status == "running" && status.Disk == 0 {
		agentCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		raw, aerr := c.AgentGet(agentCtx, node, vmid, "get-fsinfo")
		cancel()
		if aerr == nil {
			if used := pve.GuestFSUsed(raw); used > 0 {
				status.Disk = used
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"node":    node,
		"type":    kind,
		"vmid":    vmid,
		"status":  status,
		"config":  cfg,
		"pending": pending,
	})
}

func (s *Server) handleCreateGuest(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.CreateGuestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.VMID == 0 {
		id, err := c.NextID(r.Context())
		if err != nil {
			writePVEError(w, err)
			return
		}
		req.VMID = id
	}
	var (
		upid string
		err  error
	)
	if req.Type == "lxc" {
		upid, err = c.CreateLXC(r.Context(), req)
	} else {
		req.Type = "qemu"
		upid, err = c.CreateQEMU(r.Context(), req)
	}
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"upid": upid, "vmid": req.VMID})
}

func (s *Server) handleUpdateGuest(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.UpdateGuestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	if req.Node != "" {
		node = req.Node
	}
	if req.Type != "" {
		kind = kindFromType(req.Type)
	}
	if err := c.UpdateGuestConfig(r.Context(), node, kind, vmid, req.Fields, req.Delete, req.Digest); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteGuest(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	opts := pve.DeleteGuestOptions{
		Purge:                    queryFlag(r, "purge"),
		Force:                    queryFlag(r, "force"),
		DestroyUnreferencedDisks: queryFlag(r, "destroy-unreferenced-disks"),
	}
	upid, err := c.DeleteGuest(r.Context(), node, kind, vmid, opts)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleGuestPower(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	action := urlParam(r, "action")
	switch action {
	case "start", "stop", "shutdown", "reboot", "reset", "suspend", "resume", "hibernate":
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	if action == "hibernate" && kind != "qemu" {
		writeError(w, http.StatusBadRequest, "hibernate is only available for qemu")
		return
	}
	upid, err := c.GuestPower(r.Context(), node, kind, action, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleGuestRRD(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	data, err := c.GuestRRD(r.Context(), node, kind, vmid, queryDefault(r, "timeframe", "hour"))
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleGuestIORRD(w http.ResponseWriter, r *http.Request) {
	if s.pveOrError(w, r) == nil {
		return
	}
	_, _, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	tf := queryDefault(r, "timeframe", "hour")
	iops, err := s.poller.GuestIOPS(r.Context(), s.requestConnectionID(r), vmid, tf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if iops == nil {
		iops = []pve.RRDPoint{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"iops": iops})
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	snaps, err := c.Snapshots(r.Context(), node, kind, vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	var req pve.SnapshotRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	upid, err := c.CreateSnapshot(r.Context(), node, kind, vmid, req)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	upid, err := c.DeleteSnapshot(r.Context(), node, kind, urlParam(r, "name"), vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleRollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	upid, err := c.RollbackSnapshot(r.Context(), node, kind, urlParam(r, "name"), vmid)
	if err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"upid": upid})
}

func (s *Server) handleUpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	var req pve.SnapshotUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := c.UpdateSnapshot(r.Context(), node, kind, urlParam(r, "name"), vmid, req.Description); err != nil {
		writePVEError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) guestRef(w http.ResponseWriter, r *http.Request) (node, kind string, vmid int, ok bool) {
	vmid, err := strconv.Atoi(urlParam(r, "vmid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid vmid")
		return "", "", 0, false
	}
	node = r.URL.Query().Get("node")
	kind = kindFromType(r.URL.Query().Get("type"))
	if node == "" {
		c := s.pveOrError(w, r)
		if c != nil {
			resources, err := c.ClusterResources(r.Context())
			if err == nil {
				for _, res := range resources {
					if res.VMID == vmid && (res.Type == "qemu" || res.Type == "lxc") {
						node = res.Node
						kind = res.Type
						break
					}
				}
			}
		} else {
			return "", "", 0, false
		}
	}
	if node == "" {
		writeError(w, http.StatusNotFound, "guest not found")
		return "", "", 0, false
	}
	return node, kind, vmid, true
}
