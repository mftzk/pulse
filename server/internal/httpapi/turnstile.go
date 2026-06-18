package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// errTurnstileFailed marks a captcha that Cloudflare did not validate.
var errTurnstileFailed = errors.New("turnstile verification failed")

// turnstileVerifyURL is Cloudflare's siteverify endpoint.
const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var turnstileClient = &http.Client{Timeout: 10 * time.Second}

// turnstileEnabled reports whether captcha enforcement is configured.
func (s *Server) turnstileEnabled() bool {
	return s.cfg.TurnstileSecret != ""
}

// verifyTurnstile validates a Turnstile token against Cloudflare's siteverify
// API. remoteIP is best-effort (Cloudflare uses it for risk scoring). It returns
// nil only when verification succeeds.
func (s *Server) verifyTurnstile(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return errTurnstileFailed
	}
	form := url.Values{
		"secret":   {s.cfg.TurnstileSecret},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := turnstileClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return errTurnstileFailed
	}
	return nil
}

// clientIP extracts the caller's IP, preferring proxy headers since the API
// usually sits behind the Next.js proxy / a load balancer.
func clientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}
