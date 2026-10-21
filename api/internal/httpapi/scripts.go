package httpapi

import (
	"net/http"

	"hypervane/internal/pve"
	"hypervane/internal/scripts"
	"hypervane/internal/store"
)

func (s *Server) handleScriptCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"recipes": scripts.Public()})
}

// scriptInputs reads a recipe and its form.
func (s *Server) scriptInputs(w http.ResponseWriter, r *http.Request) (scripts.Recipe, scripts.Params, bool) {
	recipe, ok := scripts.Lookup(urlParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "unknown recipe")
		return recipe, scripts.Params{}, false
	}
	var p scripts.Params
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return recipe, p, false
	}
	return recipe, p, true
}

func (s *Server) planScript(w http.ResponseWriter, r *http.Request, c *pve.Client, recipe scripts.Recipe, p scripts.Params) *scripts.Plan {
	s.initScripts()
	plan, err := scripts.Prepare(r.Context(), c, s.images, recipe, p)
	if err != nil {
		if scripts.IsInputError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writePVEError(w, err)
		}
		return nil
	}
	return plan
}

// handleScriptPlan answers with the exact steps a run would take. It only
// reads from Proxmox.
func (s *Server) handleScriptPlan(w http.ResponseWriter, r *http.Request) {
	recipe, p, ok := s.scriptInputs(w, r)
	if !ok {
		return
	}
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	if plan := s.planScript(w, r, c, recipe, p); plan != nil {
		writeJSON(w, http.StatusOK, plan)
	}
}

func (s *Server) handleScriptRun(w http.ResponseWriter, r *http.Request) {
	recipe, p, ok := s.scriptInputs(w, r)
	if !ok {
		return
	}
	c := s.pveOrError(w, r)
	if c == nil {
		return
	}
	plan := s.planScript(w, r, c, recipe, p)
	if plan == nil {
		return
	}
	user, _ := r.Context().Value(userKey).(store.User)
	writeJSON(w, http.StatusAccepted, s.scriptJobs.Start(c, plan, user.ID))
}

func (s *Server) handleScriptJobs(w http.ResponseWriter, r *http.Request) {
	s.initScripts()
	user, _ := r.Context().Value(userKey).(store.User)
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.scriptJobs.List(user.ID, user.Kind == store.UserKindLocal)})
}

func (s *Server) handleScriptJob(w http.ResponseWriter, r *http.Request) {
	s.initScripts()
	user, _ := r.Context().Value(userKey).(store.User)
	job, ok := s.scriptJobs.Get(urlParam(r, "job"), user.ID, user.Kind == store.UserKindLocal)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) initScripts() {
	s.scriptsOnce.Do(func() {
		s.scriptJobs = scripts.NewJobs()
		s.images = scripts.NewImages()
	})
}
