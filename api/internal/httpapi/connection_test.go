package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"hypervane/internal/config"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

func testClient(t *testing.T, url string) *pve.Client {
	t.Helper()
	c, err := pve.New(pve.Options{BaseURL: url, Token: "user@pam!t=secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func withViewer(r *http.Request, user store.User, sess store.Session) *http.Request {
	ctx := context.WithValue(r.Context(), userKey, user)
	return r.WithContext(context.WithValue(ctx, sessionKey, sess))
}

func TestPVEOrErrorNeverFallsBackToAnotherCluster(t *testing.T) {
	s := &Server{}
	def := testClient(t, "http://127.0.0.1:1")
	s.putClient("default", def)
	admin := store.User{Kind: store.UserKindLocal}

	rec := httptest.NewRecorder()
	req := withViewer(httptest.NewRequest(http.MethodPost, "/guests/100/status/stop", nil), admin, store.Session{})
	req.Header.Set("X-Hypervane-Connection", "deleted")
	if c := s.pveOrError(rec, req); c != nil {
		t.Fatal("a named, missing connection must not resolve to the default")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = withViewer(httptest.NewRequest(http.MethodGet, "/nodes?connection=deleted", nil), admin, store.Session{})
	if c := s.pveOrError(rec, req); c != nil {
		t.Fatal("the query form must not fall back either")
	}

	rec = httptest.NewRecorder()
	req = withViewer(httptest.NewRequest(http.MethodGet, "/nodes", nil), admin, store.Session{})
	if c := s.pveOrError(rec, req); c != def {
		t.Fatalf("no target should use the default: %d", rec.Code)
	}
}

func TestDisabledConnectionStopsResolving(t *testing.T) {
	s := &Server{}
	s.putClient("a", testClient(t, "http://127.0.0.1:1"))
	s.putClient("b", testClient(t, "http://127.0.0.1:2"))
	s.removeClient("b")

	rec := httptest.NewRecorder()
	req := withViewer(httptest.NewRequest(http.MethodGet, "/nodes", nil), store.User{Kind: store.UserKindLocal}, store.Session{})
	req.Header.Set("X-Hypervane-Connection", "b")
	if c := s.pveOrError(rec, req); c != nil {
		t.Fatal("disabled connection resolved")
	}
}

func TestProxmoxSessionStaysOnItsConnection(t *testing.T) {
	s := &Server{}
	s.putClient("default", testClient(t, "http://127.0.0.1:1"))
	s.putClient("other", testClient(t, "http://127.0.0.1:2"))
	user := store.User{Kind: store.UserKindPVE, PVEUserID: "alice@pve"}

	req := withViewer(httptest.NewRequest(http.MethodGet, "/nodes", nil), user, store.Session{ConnectionID: "other"})
	req.Header.Set("X-Hypervane-Connection", "default")
	if got := s.requestConnectionID(req); got != "other" {
		t.Fatalf("header moved the session to %q", got)
	}

	// The cluster that issued the ticket is gone. The ticket must not be
	// presented to whichever cluster is the default now.
	if _, err := s.ticketClient(context.Background(), store.Session{ConnectionID: "deleted", TicketCiphertext: []byte{1}}, user); err == nil {
		t.Fatal("expected no client for a deleted connection")
	}
	rec := httptest.NewRecorder()
	req = withViewer(httptest.NewRequest(http.MethodGet, "/nodes", nil), user, store.Session{ConnectionID: "deleted"})
	if c := s.pveOrError(rec, req); c != nil {
		t.Fatal("resolved a deleted connection")
	}
	if got := strings.TrimSpace(rec.Header().Get("Set-Cookie")); got != "" {
		t.Fatalf("an outage must not end the session: %s", got)
	}

	// Sessions with no connection have nothing to bind them to, so they get
	// no choice of cluster.
	req = withViewer(httptest.NewRequest(http.MethodGet, "/nodes", nil), user, store.Session{})
	req.Header.Set("X-Hypervane-Connection", "default")
	if got := s.requestConnectionID(req); got != "" {
		t.Fatalf("unbound session reached %q", got)
	}
}

// fakeTicketServer stands in for a Proxmox ticket endpoint that refuses every
// password, and counts how many reached it.
func fakeTicketServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "authentication failure", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func loginRequestFor(t *testing.T, body map[string]string, addr string) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(raw)))
	req.RemoteAddr = addr
	return req
}

func TestLoginNeverSendsPasswordToAnotherCluster(t *testing.T) {
	srv, hits := fakeTicketServer(t)
	s := &Server{}
	s.putClient("default", testClient(t, srv.URL))

	rec := httptest.NewRecorder()
	s.handleLogin(rec, loginRequestFor(t, map[string]string{
		"username": "alice", "password": "pw", "realm": "pve", "connection": "deleted",
	}, "192.0.2.1:1000"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if hits.Load() != 0 {
		t.Fatal("the password went to the default cluster")
	}
}

func TestAuthRealmsWithUnavailableConnection(t *testing.T) {
	srv, hits := fakeTicketServer(t)
	s := &Server{}
	s.putClient("default", testClient(t, srv.URL))
	rec := httptest.NewRecorder()
	s.handleAuthRealms(rec, httptest.NewRequest(http.MethodGet, "/api/auth/realms?connection=deleted", nil))
	if hits.Load() != 0 {
		t.Fatal("asked the default cluster for another connection's realms")
	}
	var realms []pve.Realm
	if err := json.Unmarshal(rec.Body.Bytes(), &realms); err != nil {
		t.Fatal(err)
	}
	if len(realms) != 1 || realms[0].Realm != "local" {
		t.Fatalf("realms %+v", realms)
	}
}

func TestLoginThrottlesRepeatedGuesses(t *testing.T) {
	srv, hits := fakeTicketServer(t)
	s := &Server{}
	s.putClient("default", testClient(t, srv.URL))
	body := map[string]string{"username": "alice", "password": "wrong", "realm": "pve"}
	for i := 0; i < loginMaxPair; i++ {
		rec := httptest.NewRecorder()
		s.handleLogin(rec, loginRequestFor(t, body, "192.0.2.1:1000"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	s.handleLogin(rec, loginRequestFor(t, body, "192.0.2.1:1001"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	if int(hits.Load()) != loginMaxPair {
		t.Fatalf("%d guesses reached Proxmox", hits.Load())
	}
	rec = httptest.NewRecorder()
	s.handleLogin(rec, loginRequestFor(t, body, "192.0.2.2:1000"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("another address was blocked: %d", rec.Code)
	}
}

func TestLoginBodyIsLimited(t *testing.T) {
	s := New(config.Config{}, nil, nil, nil, nil)
	big := `{"username":"` + strings.Repeat("a", authBodyLimit) + `"}`
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(big)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}
