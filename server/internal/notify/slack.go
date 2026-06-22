package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Slack colors (attachment bar). Slack accepts the named values "danger"/"good"
// or hex; we use hex to match the Discord palette.
const (
	slackRed   = "#ED4245"
	slackGreen = "#57F287"
)

// Slack posts attachments to Slack-compatible incoming webhook URLs.
type Slack struct {
	client *http.Client
}

func NewSlack() *Slack {
	return &Slack{client: &http.Client{Timeout: 10 * time.Second}}
}

type slackAttachment struct {
	Color string `json:"color"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type slackPayload struct {
	Username    string            `json:"username,omitempty"`
	Attachments []slackAttachment `json:"attachments"`
}

func (s *Slack) send(ctx context.Context, webhookURL string, att slackAttachment) error {
	body, err := json.Marshal(slackPayload{Username: "Pulse", Attachments: []slackAttachment{att}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

// NotifyDown sends a "down" alert to every webhook, returning the first error.
// When reminder is true this is a periodic re-alert for a monitor that is still
// down rather than the initial transition.
func (s *Slack) NotifyDown(ctx context.Context, webhooks []string, monitorName, url, cause string, reminder bool) error {
	title := "🔴 " + monitorName + " is DOWN"
	if reminder {
		title = "🔴 " + monitorName + " is STILL DOWN"
	}
	att := slackAttachment{
		Color: slackRed,
		Title: title,
		Text:  fmt.Sprintf("*URL:* %s\n*Reason:* %s", url, cause),
	}
	return s.fanout(ctx, webhooks, att)
}

// NotifyRecovered sends a "recovered" alert including how long it was down.
func (s *Slack) NotifyRecovered(ctx context.Context, webhooks []string, monitorName, url string, downtime time.Duration) error {
	att := slackAttachment{
		Color: slackGreen,
		Title: "🟢 " + monitorName + " has RECOVERED",
		Text:  fmt.Sprintf("*URL:* %s\n*Downtime:* %s", url, downtime.Round(time.Second)),
	}
	return s.fanout(ctx, webhooks, att)
}

func (s *Slack) fanout(ctx context.Context, webhooks []string, att slackAttachment) error {
	var firstErr error
	for _, w := range webhooks {
		if err := s.send(ctx, w, att); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
