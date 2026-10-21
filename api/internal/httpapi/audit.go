package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"hypervane/internal/store"
)

type auditCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *auditCapture) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *auditCapture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	if c.body.Len() < 4096 {
		c.body.Write(p)
	}
	return c.ResponseWriter.Write(p)
}

func (s *Server) auditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		cap := &auditCapture{ResponseWriter: w, status: 0}
		next.ServeHTTP(cap, r)
		user, _ := s.userFromRequest(r)
		upid := ""
		var body map[string]any
		if json.Unmarshal(cap.body.Bytes(), &body) == nil {
			if v, ok := body["upid"].(string); ok {
				upid = v
			}
		}
		_ = s.store.InsertAudit(r.Context(), store.AuditEvent{
			At:           time.Now(),
			Username:     user.Username,
			UserKind:     user.Kind,
			ConnectionID: s.requestConnectionID(r),
			Method:       r.Method,
			Path:         r.URL.Path,
			Status:       cap.status,
			UPID:         upid,
		})
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListAudit(r.Context(), 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []store.AuditEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": rows})
}

func (s *Server) handleMetricHistory(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	from, _ := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	rows, err := s.store.QueryMetricHistory(r.Context(), s.requestConnectionID(r), key, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []store.MetricSample{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": rows})
}
