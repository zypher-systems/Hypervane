package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"hypervane/internal/auth"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

type connectionWriteRequest struct {
	Name    string `json:"name"`
	PVEURL  string `json:"pveUrl"`
	Token   string `json:"token"`
	TLSMode string `json:"tlsMode"`
	TLSPin  string `json:"tlsPin"`
	Enabled *bool  `json:"enabled"`
}

type connectionView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	PVEURL      string   `json:"pveUrl"`
	TLSMode     string   `json:"tlsMode"`
	Enabled     bool     `json:"enabled"`
	Online      *bool    `json:"online,omitempty"`
	Error       string   `json:"error,omitempty"`
	Nodes       []string `json:"nodes,omitempty"`
	ClusterName string   `json:"clusterName,omitempty"`
}

type probeResult struct {
	Client      *pve.Client
	Version     pve.Version
	ClusterName string
	Nodes       []string
}

func (s *Server) clientFromStore(cfg store.Connection) (*pve.Client, error) {
	token, err := s.box.Open(cfg.TokenCiphertext, cfg.TokenNonce)
	if err != nil {
		return nil, err
	}
	return pve.New(pve.Options{
		BaseURL: cfg.PVEURL,
		Token:   string(token),
		TLSMode: pve.TLSMode(cfg.TLSMode),
		TLSPin:  cfg.TLSPin,
	})
}

func (s *Server) publicConnections(ctx context.Context, user store.User, authed bool, connectionID string) []connectionView {
	conns, err := s.store.ListConnections(ctx)
	if err != nil {
		return []connectionView{}
	}
	out := make([]connectionView, 0, len(conns))
	for _, c := range conns {
		if authed && user.Kind == store.UserKindPVE && c.ID != connectionID {
			continue
		}
		out = append(out, connectionView{
			ID:      c.ID,
			Name:    c.Name,
			PVEURL:  c.PVEURL,
			TLSMode: c.TLSMode,
			Enabled: c.Enabled,
		})
	}
	return out
}

func (s *Server) handleListVisibleConnections(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(store.User)
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	writeJSON(w, http.StatusOK, s.publicConnections(r.Context(), user, true, sess.ConnectionID))
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(store.User)
	sess, _ := r.Context().Value(sessionKey).(store.Session)
	conns, err := s.store.ListConnections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	type group struct {
		ID        string              `json:"id"`
		Name      string              `json:"name"`
		Status    []pve.ClusterStatus `json:"status"`
		Resources []pve.Resource      `json:"resources"`
	}
	groups := make([]group, 0, len(conns))
	for _, cfg := range conns {
		if !cfg.Enabled {
			continue
		}
		if user.Kind == store.UserKindPVE && sess.ConnectionID != "" && cfg.ID != sess.ConnectionID {
			continue
		}
		var status []pve.ClusterStatus
		var resources []pve.Resource
		if s.poller != nil {
			var err error
			status, resources, err = s.poller.CachedResources(r.Context(), cfg.ID)
			if err != nil {
				status, resources = nil, nil
			}
		}
		if len(resources) == 0 {
			if c := s.ClientByID(cfg.ID); c != nil {
				if st, e := c.ClusterStatus(r.Context()); e == nil {
					status = st
				}
				if res, e := c.ClusterResources(r.Context()); e == nil {
					resources = res
				}
			}
		}
		if acl, filter := s.requestACL(r); filter {
			resources = pve.FilterResources(resources, acl)
		}
		groups = append(groups, group{ID: cfg.ID, Name: cfg.Name, Status: status, Resources: resources})
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": groups})
}

func (s *Server) handleAdminListConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := s.store.ListConnections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	views := make([]connectionView, len(conns))
	var wg sync.WaitGroup
	for i, cfg := range conns {
		views[i] = connectionView{
			ID:      cfg.ID,
			Name:    cfg.Name,
			PVEURL:  cfg.PVEURL,
			TLSMode: cfg.TLSMode,
			Enabled: cfg.Enabled,
		}
		if !cfg.Enabled {
			continue
		}
		wg.Add(1)
		go func(i int, cfg store.Connection) {
			defer wg.Done()
			c := s.ClientByID(cfg.ID)
			if c == nil {
				var err error
				c, err = s.clientFromStore(cfg)
				if err != nil {
					offline := false
					views[i].Online = &offline
					views[i].Error = err.Error()
					return
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			probe, err := probeExisting(ctx, c)
			if err != nil {
				offline := false
				views[i].Online = &offline
				views[i].Error = err.Error()
				return
			}
			online := true
			views[i].Online = &online
			views[i].Nodes = probe.Nodes
			views[i].ClusterName = probe.ClusterName
		}(i, cfg)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	var req connectionWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.PVEURL = strings.TrimSpace(req.PVEURL)
	req.Token = strings.TrimSpace(req.Token)
	if req.PVEURL == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "url and token are required")
		return
	}
	tlsMode, tlsPin := normalizeTLS(req.TLSMode, req.TLSPin)
	probe, err := probeConnection(r.Context(), req.PVEURL, req.Token, tlsMode, tlsPin)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if req.Name == "" {
		req.Name = defaultConnectionName(probe, req.PVEURL)
	}
	ct, nonce, err := s.box.Seal([]byte(req.Token))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt token")
		return
	}
	id, err := auth.NewUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "id")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cfg := store.Connection{
		ID:              id,
		Name:            req.Name,
		PVEURL:          probe.Client.BaseURL(),
		TLSMode:         string(tlsMode),
		TLSPin:          tlsPin,
		TokenCiphertext: ct,
		TokenNonce:      nonce,
		Enabled:         enabled,
	}
	if err := s.store.CreateConnection(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "save connection")
		return
	}
	if enabled {
		s.putClient(id, probe.Client)
	}
	online := true
	writeJSON(w, http.StatusOK, connectionView{
		ID: id, Name: cfg.Name, PVEURL: cfg.PVEURL, TLSMode: cfg.TLSMode, Enabled: enabled,
		Online: &online, Nodes: probe.Nodes, ClusterName: probe.ClusterName,
	})
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	existing, err := s.store.GetConnection(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	var req connectionWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		existing.Name = name
	}
	if u := strings.TrimSpace(req.PVEURL); u != "" {
		existing.PVEURL = u
	}
	tlsMode, tlsPin := normalizeTLS(req.TLSMode, req.TLSPin)
	if req.TLSMode != "" {
		existing.TLSMode = string(tlsMode)
		existing.TLSPin = tlsPin
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	token := strings.TrimSpace(req.Token)
	needsProbe := token != "" || strings.TrimSpace(req.PVEURL) != "" || req.TLSMode != ""
	var probe probeResult
	if token != "" {
		ct, nonce, err := s.box.Seal([]byte(token))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt token")
			return
		}
		existing.TokenCiphertext = ct
		existing.TokenNonce = nonce
	}
	if needsProbe {
		if token == "" {
			opened, err := s.box.Open(existing.TokenCiphertext, existing.TokenNonce)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "decrypt token")
				return
			}
			token = string(opened)
		}
		var err error
		probe, err = probeConnection(r.Context(), existing.PVEURL, token, pve.TLSMode(existing.TLSMode), existing.TLSPin)
		if err != nil {
			writePVEError(w, err)
			return
		}
		existing.PVEURL = probe.Client.BaseURL()
	}
	if err := s.store.UpdateConnection(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "save connection")
		return
	}
	if !existing.Enabled {
		s.removeClient(id)
	} else if probe.Client != nil {
		s.putClient(id, probe.Client)
	} else if s.ClientByID(id) == nil {
		if c, err := s.clientFromStore(existing); err == nil {
			s.putClient(id, c)
		}
	}
	view := connectionView{ID: id, Name: existing.Name, PVEURL: existing.PVEURL, TLSMode: existing.TLSMode, Enabled: existing.Enabled}
	if probe.Client != nil {
		online := true
		view.Online = &online
		view.Nodes = probe.Nodes
		view.ClusterName = probe.ClusterName
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if err := s.store.DeleteConnection(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	s.removeClient(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	var req connectionWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.PVEURL = strings.TrimSpace(req.PVEURL)
	req.Token = strings.TrimSpace(req.Token)
	if req.PVEURL == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "url and token are required")
		return
	}
	tlsMode, tlsPin := normalizeTLS(req.TLSMode, req.TLSPin)
	probe, err := probeConnection(r.Context(), req.PVEURL, req.Token, tlsMode, tlsPin)
	if err != nil {
		writePVEError(w, err)
		return
	}
	online := true
	writeJSON(w, http.StatusOK, connectionView{
		Name: defaultConnectionName(probe, req.PVEURL), PVEURL: probe.Client.BaseURL(),
		TLSMode: string(tlsMode), Enabled: true, Online: &online, Nodes: probe.Nodes, ClusterName: probe.ClusterName,
	})
}

func (s *Server) handleTestSavedConnection(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	cfg, err := s.store.GetConnection(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	c := s.ClientByID(id)
	if c == nil {
		c, err = s.clientFromStore(cfg)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	probe, err := probeExisting(r.Context(), c)
	if err != nil {
		writePVEError(w, err)
		return
	}
	online := true
	writeJSON(w, http.StatusOK, connectionView{
		ID: cfg.ID, Name: cfg.Name, PVEURL: cfg.PVEURL, TLSMode: cfg.TLSMode, Enabled: cfg.Enabled,
		Online: &online, Nodes: probe.Nodes, ClusterName: probe.ClusterName,
	})
}

func normalizeTLS(mode, pin string) (pve.TLSMode, string) {
	if mode == string(pve.TLSPin) {
		return pve.TLSPin, pin
	}
	return pve.TLSSkipVerify, pin
}

func probeConnection(ctx context.Context, baseURL, token string, tlsMode pve.TLSMode, tlsPin string) (probeResult, error) {
	client, err := pve.New(pve.Options{BaseURL: baseURL, Token: token, TLSMode: tlsMode, TLSPin: tlsPin})
	if err != nil {
		return probeResult{}, err
	}
	probe, err := probeExisting(ctx, client)
	if err != nil {
		return probeResult{}, err
	}
	probe.Client = client
	return probe, nil
}

func probeExisting(ctx context.Context, c *pve.Client) (probeResult, error) {
	ver, err := c.Version(ctx)
	if err != nil {
		return probeResult{}, err
	}
	if _, err := c.Permissions(ctx); err != nil {
		return probeResult{}, err
	}
	out := probeResult{Client: c, Version: ver}
	if status, err := c.ClusterStatus(ctx); err == nil {
		for _, st := range status {
			if st.Type == "cluster" && st.Name != "" {
				out.ClusterName = st.Name
			}
			if st.Type == "node" && st.Name != "" {
				out.Nodes = append(out.Nodes, st.Name)
			}
		}
	}
	if len(out.Nodes) == 0 {
		if resources, err := c.ClusterResources(ctx); err == nil {
			seen := map[string]bool{}
			for _, res := range resources {
				if res.Type == "node" && res.Node != "" && !seen[res.Node] {
					seen[res.Node] = true
					out.Nodes = append(out.Nodes, res.Node)
				}
			}
		}
	}
	return out, nil
}

func defaultConnectionName(probe probeResult, rawURL string) string {
	if probe.ClusterName != "" {
		return probe.ClusterName
	}
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "Proxmox"
}
