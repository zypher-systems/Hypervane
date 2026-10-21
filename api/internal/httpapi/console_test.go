package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The node terminal runs Proxmox's upgrade command and nothing else.
func TestNodeConsoleRefusesOtherCommands(t *testing.T) {
	s := &Server{}
	for _, cmd := range []string{"login", "ceph_install", "bash", "upgrade;rm"} {
		rec := httptest.NewRecorder()
		s.handleNodeConsole(rec, httptest.NewRequest(http.MethodGet, "/api/nodes/pve1/console?cmd="+url.QueryEscape(cmd), nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("cmd %q: %d", cmd, rec.Code)
		}
	}
	// upgrade gets past the check (and then needs a cluster this test doesn't have).
	rec := httptest.NewRecorder()
	s.handleNodeConsole(rec, httptest.NewRequest(http.MethodGet, "/api/nodes/pve1/console?cmd=upgrade", nil))
	if rec.Code == http.StatusBadRequest {
		t.Fatal("upgrade must be allowed")
	}
}
