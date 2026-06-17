// Package notify sends alerts to Discord webhooks. It is intentionally tiny and
// dependency-free (stdlib only).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	colorRed   = 0xED4245 // down
	colorGreen = 0x57F287 // recovered
)

// Discord posts embeds to Discord-compatible webhook URLs.
type Discord struct {
	client *http.Client
}

func NewDiscord() *Discord {
	return &Discord{client: &http.Client{Timeout: 10 * time.Second}}
}

type embed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
}

type payload struct {
	Username string  `json:"username,omitempty"`
	Embeds   []embed `json:"embeds"`
}

func (d *Discord) send(ctx context.Context, webhookURL string, e embed) error {
	body, err := json.Marshal(payload{Username: "Pulse", Embeds: []embed{e}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned %d", resp.StatusCode)
	}
	return nil
}

// NotifyDown sends a "down" alert to every webhook, returning the first error.
func (d *Discord) NotifyDown(ctx context.Context, webhooks []string, monitorName, url, cause string) error {
	e := embed{
		Title:       "🔴 " + monitorName + " is DOWN",
		Description: fmt.Sprintf("**URL:** %s\n**Reason:** %s", url, cause),
		Color:       colorRed,
	}
	return d.fanout(ctx, webhooks, e)
}

// NotifyRecovered sends a "recovered" alert including how long it was down.
func (d *Discord) NotifyRecovered(ctx context.Context, webhooks []string, monitorName, url string, downtime time.Duration) error {
	e := embed{
		Title:       "🟢 " + monitorName + " has RECOVERED",
		Description: fmt.Sprintf("**URL:** %s\n**Downtime:** %s", url, downtime.Round(time.Second)),
		Color:       colorGreen,
	}
	return d.fanout(ctx, webhooks, e)
}

func (d *Discord) fanout(ctx context.Context, webhooks []string, e embed) error {
	var firstErr error
	for _, w := range webhooks {
		if err := d.send(ctx, w, e); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
