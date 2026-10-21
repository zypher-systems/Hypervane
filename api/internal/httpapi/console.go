package httpapi

import (
	"log"
	"net/http"
	"strings"

	"hypervane/internal/console"
	"hypervane/internal/store"
)

func (s *Server) handleGuestConsole(w http.ResponseWriter, r *http.Request) {
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node, kind, vmid, ok := s.guestRef(w, r)
	if !ok {
		return
	}
	up := upgrader()
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("guest console upgrade: %v", err)
		return
	}
	defer conn.Close()
	if err := console.Proxy(r.Context(), c, conn, node, kind, vmid, console.NormalizeKind(r.URL.Query().Get("kind")), ""); err != nil {
		log.Printf("console proxy: %v", err)
	}
}

func (s *Server) handleNodeConsole(w http.ResponseWriter, r *http.Request) {
	// Only Proxmox's upgrade command may be run this way; it needs a terminal.
	cmd := r.URL.Query().Get("cmd")
	if cmd != "" && cmd != "upgrade" {
		writeError(w, http.StatusBadRequest, "unsupported command")
		return
	}
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	node := urlParam(r, "node")
	grantID := strings.TrimSpace(r.URL.Query().Get("grant"))
	if grantID != "" {
		// A grant is a one-time root@pam ticket. Checked before the socket
		// opens so a bad one is an HTTP error, and spent only once the
		// browser is actually connected.
		if cmd != "upgrade" {
			writeError(w, http.StatusBadRequest, "unsupported command")
			return
		}
		user, _ := r.Context().Value(userKey).(store.User)
		if user.ID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !s.upgradeGrantOK(grantID, user.ID, node, s.requestConnectionID(r)) {
			writeError(w, http.StatusForbidden, "upgrade session expired")
			return
		}
	}
	up := upgrader()
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("node console upgrade: %v", err)
		return
	}
	defer conn.Close()
	if grantID != "" {
		user, _ := r.Context().Value(userKey).(store.User)
		root, err := s.takeUpgradeGrant(grantID, user.ID, node, s.requestConnectionID(r))
		if err != nil {
			log.Printf("node console: %v", err)
			return
		}
		c = root
	}
	kind := console.NormalizeKind(r.URL.Query().Get("kind"))
	if r.URL.Query().Get("kind") == "" || cmd != "" {
		kind = console.KindTerm
	}
	if err := console.Proxy(r.Context(), c, conn, node, "node", 0, kind, cmd); err != nil {
		log.Printf("node console: %v", err)
	}
}
