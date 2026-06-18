package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aji/pulse/internal/auth"
	"github.com/aji/pulse/internal/db"
	"github.com/jackc/pgx/v5/pgconn"
)

const sessionTTL = 30 * 24 * time.Hour

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerInput struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token"`
}

type authResponse struct {
	User db.User           `json:"user"`
	Orgs []db.Organization `json:"orgs"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in registerInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if len(in.Username) < 3 || len(in.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "username must be >=3 chars and password >=6 chars")
		return
	}
	if !validEmail(in.Email) {
		writeErr(w, http.StatusBadRequest, "a valid email is required")
		return
	}

	// Bot protection: only enforced when a Turnstile secret is configured, so
	// local/dev environments can sign up without Cloudflare keys.
	if s.turnstileEnabled() {
		if err := s.verifyTurnstile(r.Context(), in.TurnstileToken, clientIP(r)); err != nil {
			writeErr(w, http.StatusBadRequest, "captcha verification failed")
			return
		}
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	user, err := s.store.CreateUser(r.Context(), in.Username, in.Email, hash)
	if err != nil {
		if isUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "username or email already taken")
			return
		}
		writeErr(w, http.StatusInternalServerError, "could not create user")
		return
	}

	// every new user gets a personal organization they own
	org, err := s.createOrgUnique(r.Context(), in.Username+"'s org", user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create org")
		return
	}

	if err := s.issueSession(w, r.Context(), user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{User: user, Orgs: []db.Organization{org}})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := s.store.GetUserByUsername(r.Context(), strings.TrimSpace(in.Username))
	if err != nil || !auth.CheckPassword(user.PasswordHash, in.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err := s.issueSession(w, r.Context(), user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	orgs, _ := s.store.ListOrgsForUser(r.Context(), user.ID)
	writeJSON(w, http.StatusOK, authResponse{User: user, Orgs: orgs})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(r.Context(), auth.HashToken(c.Value))
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	orgs, err := s.store.ListOrgsForUser(r.Context(), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user, Orgs: orgs})
}

// issueSession creates a session row and sets the cookie.
func (s *Server) issueSession(w http.ResponseWriter, ctx context.Context, userID string) error {
	token, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionTTL)
	if err := s.store.CreateSession(ctx, userID, auth.HashToken(token), expires); err != nil {
		return err
	}
	s.setSessionCookie(w, token, expires)
	return nil
}

// createOrgUnique slugifies the name and retries with a numeric suffix on slug
// collision, so org creation never fails just because a slug is taken.
func (s *Server) createOrgUnique(ctx context.Context, name, userID string) (db.Organization, error) {
	base := slugify(name)
	for attempt := 0; attempt < 6; attempt++ {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", base, attempt+1)
		}
		org, err := s.store.CreateOrgWithOwner(ctx, name, slug, userID)
		if err == nil {
			return org, nil
		}
		if !isUniqueViolation(err) {
			return db.Organization{}, err
		}
	}
	return db.Organization{}, errors.New("could not allocate unique slug")
}

// handleAuthConfig exposes non-secret client config (the Turnstile site key) so
// the browser can render the widget. The site key is public by design; the
// secret stays server-side. Returns an empty key when captcha is disabled.
func (s *Server) handleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"turnstile_site_key": s.cfg.TurnstileSiteKey,
	})
}

// validEmail is a lightweight, non-validating sanity check (we have no SMTP to
// confirm deliverability): exactly one @, with non-empty local and domain parts
// and a dot in the domain.
func validEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at != strings.LastIndexByte(email, '@') {
		return false
	}
	local, domain := email[:at], email[at+1:]
	if local == "" || domain == "" || strings.IndexByte(domain, '.') < 1 {
		return false
	}
	return !strings.HasSuffix(domain, ".")
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
