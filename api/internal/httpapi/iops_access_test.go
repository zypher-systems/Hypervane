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
		data := any([]any{})
		if r.URL.Path == "/api2/json/access/permissions" {
			data = map[string]any{"/vms/100": map[string]any{"VM.Audit": 1}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	box, err := crypto.NewBox("test-only-master-key")
	if err != nil {
		t.Fatal(err)
	}
	s.box = box
	s.putClient("lab-a", testClient(t, srv.URL))
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
