package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hypervane/internal/pve"
	"hypervane/internal/store"

	"github.com/go-chi/chi/v5"
)

func upgradeCall(h http.HandlerFunc, method, path, body string, params map[string]string, user store.User) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, userKey, user)
	h(rec, req.WithContext(ctx))
	return rec
}

func upgradeServer(t *testing.T) (*Server, *int) {
	t.Helper()
	var posts int
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		posts++
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("ticket call must not send the API token, got %q", r.Header.Get("Authorization"))
		}
		if r.Form.Get("username") != "root@pam" {
			t.Errorf("username %q", r.Form.Get("username"))
		}
		write := func(status int, data any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		}
		switch {
		case r.Form.Get("password") != "secret" && r.Form.Get("tfa-challenge") == "":
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "authentication failure"})
		case r.Form.Get("tfa-challenge") == "" && r.Form.Get("password") == "secret":
			write(http.StatusOK, map[string]any{"username": "root@pam", "ticket": "tfa-challenge-value", "NeedTFA": true})
		case r.Form.Get("tfa-challenge") == "tfa-challenge-value" && r.Form.Get("password") == "123456":
			write(http.StatusOK, map[string]any{
				"username":            "root@pam",
				"ticket":              "PVE:root@pam:ticket",
				"CSRFPreventionToken": "csrf",
			})
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "authentication failure"})
		}
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	c, err := pve.New(pve.Options{BaseURL: srv.URL, Token: "root@pam!hv=abc", TLSMode: pve.TLSSkipVerify, Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.putClient("c1", c)
	return s, &posts
}

func TestUpgradeSessionStartsRootTerminalWithoutKeepingThePassword(t *testing.T) {
	s, posts := upgradeServer(t)
	alice := store.User{ID: "alice", Kind: store.UserKindLocal}

	rec := upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session", `{"password":"secret"}`, map[string]string{"node": "pve1"}, alice)
	var tfa struct {
		NeedTfa      bool   `json:"needTfa"`
		TFAChallenge string `json:"tfaChallenge"`
		Grant        string `json:"grant"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &tfa)
	if rec.Code != http.StatusOK || !tfa.NeedTfa || tfa.TFAChallenge != "tfa-challenge-value" || tfa.Grant != "" {
		t.Fatalf("tfa: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response contains the password: %s", rec.Body.String())
	}

	rec = upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session",
		`{"password":"secret","otp":"123456","tfaChallenge":"tfa-challenge-value"}`, map[string]string{"node": "pve1"}, alice)
	var okBody struct {
		Grant string `json:"grant"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &okBody)
	if rec.Code != http.StatusOK || okBody.Grant == "" || strings.Contains(rec.Body.String(), "PVE:root@pam") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}

	root, err := s.takeUpgradeGrant(okBody.Grant, alice.ID, "pve1", "c1")
	if err != nil || root == nil || root.Ticket() != "PVE:root@pam:ticket" || root.Token() != "" || root.CSRF() != "csrf" {
		gotTicket, gotToken, gotCSRF := "", "", ""
		if root != nil {
			gotTicket, gotToken, gotCSRF = root.Ticket(), root.Token(), root.CSRF()
		}
		t.Fatalf("client ticket %q token %q csrf %q err %v", gotTicket, gotToken, gotCSRF, err)
	}
	if _, err := s.takeUpgradeGrant(okBody.Grant, alice.ID, "pve1", "c1"); err == nil {
		t.Fatal("grant was reusable")
	}
	if *posts != 2 {
		t.Fatalf("ticket posts %d", *posts)
	}
}

func TestUpgradeSessionRejectsABadPasswordAndStopsTrying(t *testing.T) {
	s, posts := upgradeServer(t)
	alice := store.User{ID: "alice", Kind: store.UserKindLocal}
	for i := 0; i < upgradeFailMax; i++ {
		rec := upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session", `{"password":"nope"}`, map[string]string{"node": "pve1"}, alice)
		if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "nope") {
			t.Fatalf("attempt %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session", `{"password":"secret"}`, map[string]string{"node": "pve1"}, alice)
	if rec.Code != http.StatusTooManyRequests || *posts != upgradeFailMax {
		t.Fatalf("lock: %d posts %d body %s", rec.Code, *posts, rec.Body.String())
	}
}

func TestUpgradeGrantMatchesTheUserAndNode(t *testing.T) {
	s, _ := upgradeServer(t)
	alice := store.User{ID: "alice", Kind: store.UserKindLocal}
	// Skip TFA for this case by answering it in one server that already requires TFA.
	// Drive the two-step exchange, then check the console refuses a mismatched grant
	// without spending it.
	rec := upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session", `{"password":"secret"}`, map[string]string{"node": "pve1"}, alice)
	rec = upgradeCall(s.handleAPTUpgradeSession, http.MethodPost, "/nodes/pve1/apt/upgrade-session",
		`{"password":"secret","otp":"123456","tfaChallenge":"tfa-challenge-value"}`, map[string]string{"node": "pve1"}, alice)
	var body struct {
		Grant string `json:"grant"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Grant == "" {
		t.Fatalf("no grant: %d %s", rec.Code, rec.Body.String())
	}

	bob := store.User{ID: "bob", Kind: store.UserKindLocal}
	rec = upgradeCall(s.handleNodeConsole, http.MethodGet, "/api/nodes/pve1/console?cmd=upgrade&grant="+body.Grant, "", map[string]string{"node": "pve1"}, bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("bob: %d %s", rec.Code, rec.Body.String())
	}
	rec = upgradeCall(s.handleNodeConsole, http.MethodGet, "/api/nodes/pve2/console?cmd=upgrade&grant="+body.Grant, "", map[string]string{"node": "pve2"}, alice)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other node: %d %s", rec.Code, rec.Body.String())
	}
	if !s.upgradeGrantOK(body.Grant, alice.ID, "pve1", "c1") {
		t.Fatal("mismatched console calls spent the grant")
	}

	s.upgradeMu.Lock()
	g := s.upgradeGrants[body.Grant]
	g.expires = time.Now().Add(-time.Second)
	s.upgradeGrants[body.Grant] = g
	s.upgradeMu.Unlock()
	if s.upgradeGrantOK(body.Grant, alice.ID, "pve1", "c1") {
		t.Fatal("expired grant still valid")
	}
	rec = upgradeCall(s.handleNodeConsole, http.MethodGet, "/api/nodes/pve1/console?cmd=upgrade&grant="+body.Grant, "", map[string]string{"node": "pve1"}, alice)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expired: %d %s", rec.Code, rec.Body.String())
	}
}
