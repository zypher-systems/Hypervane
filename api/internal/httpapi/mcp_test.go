package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleMCPListsReadOnlyTools(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleMCP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Tools []struct{ Name string `json:"name"` }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range body.Tools {
		names[tool.Name] = true
		if strings.Contains(tool.Name, "write") || strings.Contains(tool.Name, "apply") {
			t.Fatalf("write tool leaked: %s", tool.Name)
		}
	}
	for _, need := range []string{"hypervane_summary", "hypervane_guests", "hypervane_guest", "hypervane_history"} {
		if !names[need] {
			t.Fatalf("missing %s", need)
		}
	}
}

func TestHandleMCPToolsListJSONRPC(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	s.handleMCP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["jsonrpc"] != "2.0" || body["error"] != nil {
		t.Fatalf("%v", body)
	}
}
