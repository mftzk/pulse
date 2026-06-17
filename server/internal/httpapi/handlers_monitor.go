package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/aji/pulse/internal/db"
	"github.com/go-chi/chi/v5"
)

type monitorInput struct {
	Name            string         `json:"name"`
	URL             string         `json:"url"`
	Method          string         `json:"method"`
	ExpectedStatus  int            `json:"expected_status"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutMs       int            `json:"timeout_ms"`
	FollowRedirects *bool          `json:"follow_redirects"`
	Headers         map[string]any `json:"headers"`
	FailThreshold   int            `json:"fail_threshold"`
	Enabled         *bool          `json:"enabled"`
}

// toMonitor validates input and applies defaults, producing a db.Monitor.
func (in monitorInput) toMonitor(orgID string) (db.Monitor, string) {
	name := strings.TrimSpace(in.Name)
	url := strings.TrimSpace(in.URL)
	if name == "" {
		return db.Monitor{}, "name is required"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return db.Monitor{}, "url must start with http:// or https://"
	}

	m := db.Monitor{
		OrganizationID:  orgID,
		Name:            name,
		URL:             url,
		Method:          strings.ToUpper(strings.TrimSpace(in.Method)),
		ExpectedStatus:  in.ExpectedStatus,
		IntervalSeconds: in.IntervalSeconds,
		TimeoutMs:       in.TimeoutMs,
		FollowRedirects: true,
		Headers:         in.Headers,
		FailThreshold:   in.FailThreshold,
		Enabled:         true,
	}
	if m.Method == "" {
		m.Method = "GET"
	}
	if m.IntervalSeconds < 10 {
		m.IntervalSeconds = 60
	}
	if m.TimeoutMs <= 0 {
		m.TimeoutMs = 10000
	}
	if m.FailThreshold < 1 {
		m.FailThreshold = 1
	}
	if in.FollowRedirects != nil {
		m.FollowRedirects = *in.FollowRedirects
	}
	if in.Enabled != nil {
		m.Enabled = *in.Enabled
	}
	return m, ""
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	monitors, err := s.store.ListMonitors(r.Context(), org.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, monitors)
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	var in monitorInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	m, verr := in.toMonitor(org.ID)
	if verr != "" {
		writeErr(w, http.StatusBadRequest, verr)
		return
	}
	created, err := s.store.CreateMonitor(r.Context(), m)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create monitor")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	m, err := s.store.GetMonitor(r.Context(), org.ID, chi.URLParam(r, "id"))
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	var in monitorInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	m, verr := in.toMonitor(org.ID)
	if verr != "" {
		writeErr(w, http.StatusBadRequest, verr)
		return
	}
	m.ID = chi.URLParam(r, "id")
	updated, err := s.store.UpdateMonitor(r.Context(), m)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	if err := s.store.DeleteMonitor(r.Context(), org.ID, chi.URLParam(r, "id")); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMonitorResults(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	limit := parseLimit(r, 50, 200)
	results, err := s.store.RecentResults(r.Context(), org.ID, chi.URLParam(r, "id"), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	limit := parseLimit(r, 50, 200)
	incidents, err := s.store.ListIncidents(r.Context(), org.ID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, incidents)
}

func parseLimit(r *http.Request, def, max int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > max {
				return max
			}
			return n
		}
	}
	return def
}
