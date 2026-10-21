package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"hypervane/internal/pve"
	"hypervane/internal/store"
)

const (
	upgradeGrantTTL   = 2 * time.Minute
	upgradeFailWindow = 10 * time.Minute
	upgradeFailMax    = 8
)

// upgradeGrant is a one-time root@pam ticket used only to open Proxmox's
// upgrade terminal. Proxmox runs pveupgrade for that user alone; an API token
// gets a login shell instead. The node's root password is exchanged for the
// ticket and is not kept.
type upgradeGrant struct {
	client  *pve.Client
	node    string
	userID  string
	connID  string
	expires time.Time
}

var errUpgradeGrant = errors.New("upgrade session expired")

func (s *Server) handleAPTUpgradeSession(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	user, _ := r.Context().Value(userKey).(store.User)
	if user.ID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Password     string `json:"password"`
		OTP          string `json:"otp"`
		TFAChallenge string `json:"tfaChallenge"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	node := urlParam(r, "node")
	if strings.TrimSpace(node) == "" {
		writeError(w, http.StatusBadRequest, "node is required")
		return
	}
	if strings.TrimSpace(body.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if body.TFAChallenge != "" && strings.TrimSpace(body.OTP) == "" {
		writeError(w, http.StatusBadRequest, "one-time code is required")
		return
	}
	key := user.ID + "\x00" + s.requestConnectionID(r)
	if s.upgradeLocked(key) {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts; wait a few minutes and try again")
		return
	}
	ticket, err := c.CreateTicket(r.Context(), "root@pam", body.Password, strings.TrimSpace(body.OTP), body.TFAChallenge)
	if err != nil {
		if apiErr, ok := pve.IsAPIError(err); ok && (apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden) {
			s.noteUpgradeFailure(key)
		}
		writePVEError(w, err)
		return
	}
	if ticket.NeedTFA {
		if ticket.Ticket == "" {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"needTfa":      true,
			"tfaChallenge": ticket.Ticket,
		})
		return
	}
	if ticket.Ticket == "" || (ticket.Username != "" && ticket.Username != "root@pam") {
		s.noteUpgradeFailure(key)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	id, err := s.putUpgradeGrant(c.WithTicket(ticket.Ticket, ticket.CSRF), node, user.ID, s.requestConnectionID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upgrade session")
		return
	}
	s.clearUpgradeFailures(key)
	writeJSON(w, http.StatusOK, map[string]string{"grant": id})
}

func (s *Server) putUpgradeGrant(client *pve.Client, node, userID, connID string) (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf[:])
	now := time.Now()
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	if s.upgradeGrants == nil {
		s.upgradeGrants = map[string]upgradeGrant{}
	}
	for k, g := range s.upgradeGrants {
		if now.After(g.expires) {
			delete(s.upgradeGrants, k)
		}
	}
	s.upgradeGrants[id] = upgradeGrant{
		client:  client,
		node:    node,
		userID:  userID,
		connID:  connID,
		expires: now.Add(upgradeGrantTTL),
	}
	return id, nil
}

func (s *Server) upgradeGrantOK(id, userID, node, connID string) bool {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	g, ok := s.upgradeGrants[id]
	if !ok || time.Now().After(g.expires) {
		return false
	}
	return g.userID == userID && g.node == node && g.connID == connID && g.client != nil
}

func (s *Server) takeUpgradeGrant(id, userID, node, connID string) (*pve.Client, error) {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	g, ok := s.upgradeGrants[id]
	if ok {
		delete(s.upgradeGrants, id)
	}
	if !ok || time.Now().After(g.expires) || g.userID != userID || g.node != node || g.connID != connID || g.client == nil {
		return nil, errUpgradeGrant
	}
	return g.client, nil
}

func (s *Server) upgradeLocked(key string) bool {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	return len(s.recentUpgradeFails(key, time.Now())) >= upgradeFailMax
}

func (s *Server) noteUpgradeFailure(key string) {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	if s.upgradeFails == nil {
		s.upgradeFails = map[string][]time.Time{}
	}
	now := time.Now()
	s.upgradeFails[key] = append(s.recentUpgradeFails(key, now), now)
}

func (s *Server) clearUpgradeFailures(key string) {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()
	delete(s.upgradeFails, key)
}

func (s *Server) recentUpgradeFails(key string, now time.Time) []time.Time {
	cut := now.Add(-upgradeFailWindow)
	kept := make([]time.Time, 0, len(s.upgradeFails[key]))
	for _, t := range s.upgradeFails[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	return kept
}
