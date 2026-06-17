// Package probe performs a single HTTP/HTTPS health check against a target.
// It has no knowledge of the database and is safe to unit test in isolation.
package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Target describes what to probe and how.
type Target struct {
	URL             string
	Method          string // defaults to GET
	ExpectedStatus  int    // 0 => any 2xx counts as up
	TimeoutMs       int    // defaults to 10000
	FollowRedirects bool
	Headers         map[string]string
}

// Result is the outcome of a single probe.
type Result struct {
	Up           bool
	StatusCode   int           // 0 when the request never completed
	ResponseTime time.Duration // time to first response (or until failure)
	Error        string        // transport/HTTP error description, empty when up
}

// Do executes the probe. It never returns an error value; failures are encoded
// in Result (Up=false, Error set) so callers have one uniform path.
func Do(ctx context.Context, t Target) Result {
	method := t.Method
	if method == "" {
		method = http.MethodGet
	}
	timeout := time.Duration(t.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	if !t.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(reqCtx, method, t.URL, nil)
	if err != nil {
		return Result{Up: false, ResponseTime: time.Since(start), Error: fmt.Sprintf("bad request: %v", err)}
	}
	req.Header.Set("User-Agent", "Pulse-Uptime-Checker/1.0")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return Result{Up: false, ResponseTime: elapsed, Error: err.Error()}
	}
	defer resp.Body.Close()
	// drain a little so the connection can be reused; ignore body content
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)

	up := statusIsUp(resp.StatusCode, t.ExpectedStatus)
	res := Result{Up: up, StatusCode: resp.StatusCode, ResponseTime: elapsed}
	if !up {
		res.Error = fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	return res
}

// statusIsUp decides up/down from the response code. When expected is non-zero
// the code must match exactly; otherwise any 2xx is considered up.
func statusIsUp(code, expected int) bool {
	if expected > 0 {
		return code == expected
	}
	return code >= 200 && code < 300
}
