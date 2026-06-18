// Package httpapi exposes the REST API: auth, organizations (many-to-many
// membership), monitors, results, incidents and Discord channels. All business
// data is scoped to an organization the caller belongs to.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aji/pulse/internal/auth"
	"github.com/aji/pulse/internal/config"
	"github.com/aji/pulse/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const sessionCookie = "pulse_session"

type ctxKey string

const (
	ctxUser ctxKey = "user"
	ctxOrg  ctxKey = "org"
)

// Server holds dependencies for the HTTP handlers.
type Server struct {
	store        *db.Store
	cfg          config.Config
	cookieSecure bool
	allowOrigin  string
}

func New(store *db.Store, cfg config.Config, cookieSecure bool, allowOrigin string) *Server {
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	return &Server{store: store, cfg: cfg, cookieSecure: cookieSecure, allowOrigin: allowOrigin}
}

// Router builds the chi router with all routes and middleware.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.cors)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		// public auth endpoints
		r.Get("/auth/config", s.handleAuthConfig)
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)

		// authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/me", s.handleMe)
			r.Get("/orgs", s.handleListOrgs)
			r.Post("/orgs", s.handleCreateOrg)

			// org-scoped endpoints
			r.Route("/orgs/{slug}", func(r chi.Router) {
				r.Use(s.orgScope)
				r.Get("/", s.handleGetOrg)

				r.Get("/members", s.handleListMembers)
				r.Post("/members", s.handleAddMember)
				r.Delete("/members/{userID}", s.handleRemoveMember)

				r.Get("/monitors", s.handleListMonitors)
				r.Post("/monitors", s.handleCreateMonitor)
				r.Get("/monitors/{id}", s.handleGetMonitor)
				r.Put("/monitors/{id}", s.handleUpdateMonitor)
				r.Delete("/monitors/{id}", s.handleDeleteMonitor)
				r.Get("/monitors/{id}/results", s.handleMonitorResults)

				r.Get("/incidents", s.handleListIncidents)

				r.Get("/channels", s.handleListChannels)
				r.Post("/channels", s.handleCreateChannel)
				r.Delete("/channels/{id}", s.handleDeleteChannel)
			})
		})
	})

	return r
}

// --- middleware ---

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (s.allowOrigin == "*" || s.allowOrigin == origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			writeErr(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		user, err := s.store.UserBySessionToken(r.Context(), auth.HashToken(c.Value))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid session")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) orgScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFrom(r.Context())
		slug := chi.URLParam(r, "slug")
		org, err := s.store.OrgMembershipBySlug(r.Context(), slug, user.ID)
		if err != nil {
			// not found OR not a member -> 404, don't leak existence
			writeErr(w, http.StatusNotFound, "organization not found")
			return
		}
		ctx := context.WithValue(r.Context(), ctxOrg, org)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFrom(ctx context.Context) db.User { u, _ := ctx.Value(ctxUser).(db.User); return u }
func orgFrom(ctx context.Context) db.Organization {
	o, _ := ctx.Value(ctxOrg).(db.Organization)
	return o
}

// --- session cookie ---

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func notFoundOr500(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, "internal error")
}

// slugify produces a URL-safe slug from arbitrary text.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "org"
	}
	return out
}
