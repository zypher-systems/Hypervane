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
	"hypervane/internal/scripts"
	"hypervane/internal/store"

	"github.com/go-chi/chi/v5"
)

// scriptServer is a Server wired to a minimal Proxmox that has two nodes and
// finishes every task at once.
func scriptServer(t *testing.T) (*Server, *[]string) {
	t.Helper()
	var writes []string
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, data any) { _ = json.NewEncoder(w).Encode(map[string]any{"data": data}) }
	mux.HandleFunc("/api2/json/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api2/json")
		if r.Method != http.MethodGet {
			writes = append(writes, r.Method+" "+path)
		}
		switch {
		case path == "/cluster/resources":
			reply(w, []pve.Resource{{Type: "node", Node: "pve1", Status: "online"}, {Type: "node", Node: "pve2", Status: "online"}})
		case path == "/storage":
			reply(w, []pve.Storage{})
		case strings.HasSuffix(path, "/apt/update") && r.Method == http.MethodPost:
			reply(w, "UPID:"+strings.Split(path, "/")[2]+":1:1:1:aptupdate::root@pam:")
		case strings.HasSuffix(path, "/apt/update"):
			reply(w, []map[string]any{})
		case strings.HasSuffix(path, "/status"):
			reply(w, map[string]any{"status": "stopped", "exitstatus": "OK"})
		default:
			http.Error(w, "not faked", http.StatusNotImplemented)
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
	return s, &writes
}

func scriptCall(s *Server, h http.HandlerFunc, method, path, body string, params map[string]string, user store.User) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	if path == "" {
		path = "/scripts"
	}
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

func TestRecipePlanRunAndFollow(t *testing.T) {
	s, writes := scriptServer(t)
	alice := store.User{ID: "alice", Kind: store.UserKindLocal}

	// Planning reads only, and a bad input is a 400 with the reason.
	rec := scriptCall(s, s.handleScriptPlan, http.MethodPost, "/scripts/snapshot-node/plan", `{"node":"nope"}`, map[string]string{"id": "snapshot-node"}, alice)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "isn't in this cluster") {
		t.Fatalf("bad input: %d %s", rec.Code, rec.Body.String())
	}
	rec = scriptCall(s, s.handleScriptPlan, http.MethodPost, "/scripts/check-updates/plan", `{}`, map[string]string{"id": "check-updates"}, alice)
	var plan scripts.Plan
	_ = json.Unmarshal(rec.Body.Bytes(), &plan)
	if rec.Code != http.StatusOK || len(plan.Steps) != 2 || len(*writes) != 0 {
		t.Fatalf("plan: %d %s (writes %v)", rec.Code, rec.Body.String(), *writes)
	}

	// Running returns the job at once; the job then finishes in the background.
	rec = scriptCall(s, s.handleScriptRun, http.MethodPost, "/scripts/check-updates/run", `{}`, map[string]string{"id": "check-updates"}, alice)
	var job scripts.Job
	_ = json.Unmarshal(rec.Body.Bytes(), &job)
	if rec.Code != http.StatusAccepted || job.ID == "" || len(job.Steps) != 2 {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	for i := 0; i < 200 && job.Status == scripts.JobRunning; i++ {
		time.Sleep(10 * time.Millisecond)
		rec = scriptCall(s, s.handleScriptJob, http.MethodGet, "/scripts/jobs/"+job.ID, "", map[string]string{"job": job.ID}, alice)
		_ = json.Unmarshal(rec.Body.Bytes(), &job)
	}
	if job.Status != scripts.JobDone || job.Steps[0].Note != "Up to date." || len(job.Steps[0].UPIDs) != 1 {
		t.Fatalf("job %+v", job)
	}

	// A Proxmox user sees only their own runs; a local admin sees every run.
	bob := store.User{ID: "bob", Kind: store.UserKindPVE}
	if rec := scriptCall(s, s.handleScriptJob, http.MethodGet, "", "", map[string]string{"job": job.ID}, bob); rec.Code != http.StatusNotFound {
		t.Fatalf("bob sees alice's job: %d", rec.Code)
	}
	admin := store.User{ID: "another-admin", Kind: store.UserKindLocal}
	if rec := scriptCall(s, s.handleScriptJob, http.MethodGet, "", "", map[string]string{"job": job.ID}, admin); rec.Code != http.StatusOK {
		t.Fatalf("admin: %d", rec.Code)
	}
	rec = scriptCall(s, s.handleScriptJobs, http.MethodGet, "/scripts/jobs", "", nil, bob)
	if !strings.Contains(rec.Body.String(), `"jobs":[]`) {
		t.Fatalf("bob's list %s", rec.Body.String())
	}
}

// Recipes that used to require a Pro license run like the rest of the library.
func TestFormerlyProRecipesAreNotGated(t *testing.T) {
	s, _ := scriptServer(t)
	u := store.User{ID: "u", Kind: store.UserKindLocal}
	body := `{"node":"pve1","target":"pve2"}`
	for _, id := range []string{"evacuate-node", "almalinux-9-template", "rocky-9-template"} {
		rec := scriptCall(s, s.handleScriptRun, http.MethodPost, "", body, map[string]string{"id": id}, u)
		if rec.Code == http.StatusPaymentRequired {
			t.Fatalf("%s is still gated: %s", id, rec.Body.String())
		}
	}
}
