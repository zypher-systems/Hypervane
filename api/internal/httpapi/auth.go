package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"hypervane/internal/auth"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Realm        string `json:"realm"`
	OTP          string `json:"otp"`
	TFAChallenge string `json:"tfaChallenge"`
	Connection   string `json:"connection"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	connCount, err := s.store.ConnectionCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	user, sess, userErr := s.sessionUserFromRequest(r)
	resp := map[string]any{
		"setupRequired":     users == 0,
		"clusterConfigured": connCount > 0,
		"user":              nil,
		"permissions":       map[string]any{},
		"connections":       s.publicConnections(r.Context(), user, userErr == nil, sess.ConnectionID),
	}
	if userErr != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	u := map[string]any{"id": user.ID, "username": user.Username, "kind": user.Kind}
	if user.PVEUserID != "" {
		u["pveUserid"] = user.PVEUserID
	}
	resp["user"] = u
	if user.Kind == store.UserKindPVE {
		c, err := s.ticketClient(r.Context(), sess, user)
		if err != nil {
			resp["user"] = nil
			writeJSON(w, http.StatusOK, resp)
			return
		}
		perms, err := c.Permissions(r.Context())
		if err == nil {
			resp["permissions"] = perms
		}
	} else if c := s.ClientByID(s.requestConnectionID(r)); c != nil {
		// A local admin acts through the connection's API token, so the UI
		// should offer only what that token may do (a PVEAuditor token is
		// read-only). The token's effective permissions come from Proxmox.
		if perms, err := c.Permissions(r.Context()); err == nil {
			resp["permissions"] = perms
			resp["permissionsFrom"] = "token"
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthRealms(w http.ResponseWriter, r *http.Request) {
	realms := []pve.Realm{{Realm: "local", Type: "local", Comment: "Hypervane admin"}}
	c := s.ClientByID(strings.TrimSpace(r.URL.Query().Get("connection")))
	if c == nil {
		c = s.Client()
	}
	if c == nil {
		writeJSON(w, http.StatusOK, realms)
		return
	}
	domains, err := c.ListDomains(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, realms)
		return
	}
	writeJSON(w, http.StatusOK, append(realms, domains...))
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.UserCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if users > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}
	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username and password (8+) are required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password")
		return
	}
	uid, err := auth.NewUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "id")
		return
	}
	if err := s.store.CreateUser(r.Context(), uid, req.Username, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "create user")
		return
	}
	if err := s.issueSession(w, r, uid, nil, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{"id": uid, "username": req.Username, "kind": store.UserKindLocal},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Realm = strings.TrimSpace(req.Realm)
	if req.Realm != "" && req.Realm != "local" {
		s.handlePVELogin(w, r, req)
		return
	}
	user, err := s.store.UserByUsername(r.Context(), req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if user.Kind == store.UserKindPVE || !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := s.issueSession(w, r, user.ID, nil, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{"id": user.ID, "username": user.Username, "kind": user.Kind},
	})
}

func (s *Server) handlePVELogin(w http.ResponseWriter, r *http.Request, req loginRequest) {
	connID := strings.TrimSpace(req.Connection)
	c := s.ClientByID(connID)
	if c == nil {
		c = s.Client()
		connID = s.DefaultConnectionID()
	}
	if c == nil {
		writeError(w, http.StatusServiceUnavailable, "cluster is not configured")
		return
	}
	userid := req.Username
	if !strings.Contains(userid, "@") {
		userid = userid + "@" + req.Realm
	}
	ticket, err := c.CreateTicket(r.Context(), userid, req.Password, req.OTP, req.TFAChallenge)
	if err != nil {
		writePVEError(w, err)
		return
	}
	if ticket.NeedTFA {
		writeJSON(w, http.StatusOK, map[string]any{
			"needTfa":      true,
			"tfaChallenge": ticket.Ticket,
		})
		return
	}
	if ticket.Ticket == "" {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if ticket.Username != "" {
		userid = ticket.Username
	}
	uid, err := auth.NewUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "id")
		return
	}
	user, err := s.store.UpsertPVEUser(r.Context(), uid, userid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create user")
		return
	}
	if err := s.issueSession(w, r, user.ID, &ticket, connID); err != nil {
		writeError(w, http.StatusInternalServerError, "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{"id": user.ID, "username": user.Username, "kind": user.Kind, "pveUserid": user.PVEUserID},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteSessionByTokenHash(r.Context(), store.HashToken(c.Value))
	}
	s.clearSession(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, userID string, ticket *pve.TicketAuth, connectionID string) error {
	token, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	sid, err := auth.NewUUID()
	if err != nil {
		return err
	}
	expires := time.Now().Add(auth.SessionTTL)
	sess := store.Session{
		ID:           sid,
		UserID:       userID,
		TokenHash:    store.HashToken(token),
		ExpiresAt:    expires,
		ConnectionID: connectionID,
	}
	if ticket != nil && ticket.Ticket != "" {
		if err := s.sealTicket(&sess, ticket.Ticket, ticket.CSRF); err != nil {
			return err
		}
	}
	if err := s.store.CreateSession(r.Context(), sess); err != nil {
		return err
	}
	s.setSession(w, token, expires)
	return nil
}

func (s *Server) sealTicket(sess *store.Session, ticket, csrf string) error {
	ct, nonce, err := s.box.Seal([]byte(ticket))
	if err != nil {
		return err
	}
	sess.TicketCiphertext = ct
	sess.TicketNonce = nonce
	if csrf != "" {
		cct, cnonce, err := s.box.Seal([]byte(csrf))
		if err != nil {
			return err
		}
		sess.CSRFCiphertext = cct
		sess.CSRFNonce = cnonce
	}
	exp := time.Now().Add(pve.TicketTTL)
	sess.TicketExpiresAt = &exp
	return nil
}
