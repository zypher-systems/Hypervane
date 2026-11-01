package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"hypervane/internal/crypto"
	"hypervane/internal/pve"
	"hypervane/internal/store"
)

// restrictedRequest builds a request from a Proxmox user who holds VM.Audit on
// VM 100 and nothing else.
func restrictedRequest(t *testing.T, s *Server, path string) *http.Request {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any = []any{}
		switch r.URL.Path {
		case "/api2/json/access/permissions":
			data = map[string]any{"/vms/100": map[string]any{"VM.Audit": 1}}
		case "/api2/json/cluster/resources":
			data = []map[string]any{{"id": "qemu/100", "type": "qemu", "vmid": 100}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	return sessionRequest(t, s, srv.URL, path)
}

// sessionRequest builds a request from a Proxmox user whose session is on the
// cluster at url.
func sessionRequest(t *testing.T, s *Server, url, path string) *http.Request {
	t.Helper()
	box, err := crypto.NewBox("test-only-master-key")
	if err != nil {
		t.Fatal(err)
	}
	s.box = box
	s.putClient("lab-a", testClient(t, url))
	sess := store.Session{ConnectionID: "lab-a"}
	if err := s.sealTicket(&sess, "PVE:alice@pve:ticket", "csrf"); err != nil {
		t.Fatal(err)
	}
	return withViewer(httptest.NewRequest(http.MethodGet, path, nil), store.User{Kind: store.UserKindPVE, PVEUserID: "alice@pve"}, sess)
}

func withVMID(r *http.Request, vmid string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("vmid", vmid)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// Both of these come from Proxmox's real answers: a NoAccess entry leaves the
// path out of the permission list, and 0 means held but not propagated.
func TestGuestIOPSFollowsProxmoxsOwnAnswer(t *testing.T) {
	cases := []struct {
		name      string
		perms     map[string]any
		resources []map[string]any
		want      int
	}{
		{
			name:      "NoAccess below a grant on /",
			perms:     map[string]any{"/": map[string]any{"VM.Audit": 1}, "/vms": map[string]any{"VM.Audit": 1}},
			resources: []map[string]any{{"id": "qemu/100", "type": "qemu", "vmid": 100}},
			want:      http.StatusForbidden,
		},
		{
			name:      "a grant that does not propagate",
			perms:     map[string]any{"/vms/200": map[string]any{"VM.Audit": 0}},
			resources: []map[string]any{{"id": "qemu/200", "type": "qemu", "vmid": 200}},
			want:      http.StatusOK,
		},
	}
	for _, c := range cases {
		s := &Server{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var data any = []any{}
			switch r.URL.Path {
			case "/api2/json/access/permissions":
				data = c.perms
			case "/api2/json/cluster/resources":
				data = c.resources
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		}))
		t.Cleanup(srv.Close)
		rec := httptest.NewRecorder()
		s.handleGuestIORRD(rec, withVMID(sessionRequest(t, s, srv.URL, "/guests/200/io-rrd?node=pve1&timeframe=hour"), "200"))
		if rec.Code != c.want {
			t.Errorf("%s: %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

func TestGuestIOPSNeedsAccessToThatGuest(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleGuestIORRD(rec, withVMID(restrictedRequest(t, s, "/guests/200/io-rrd?node=pve1&timeframe=hour"), "200"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a named node must not open another guest's series: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	s.handleGuestIORRD(rec, withVMID(restrictedRequest(t, s, "/guests/100/io-rrd?node=pve1&timeframe=hour"), "100"))
	if rec.Code != http.StatusOK {
		t.Fatalf("own guest: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		IOPS []pve.RRDPoint `json:"iops"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
}
