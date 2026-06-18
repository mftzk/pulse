// Command worker continuously claims due monitors (sharing load with every
// other worker via Postgres FOR UPDATE SKIP LOCKED), probes them, records
// results, and fires Discord alerts on status transitions.
package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aji/pulse/internal/config"
	"github.com/aji/pulse/internal/db"
	"github.com/aji/pulse/internal/incident"
	"github.com/aji/pulse/internal/notify"
	"github.com/aji/pulse/internal/probe"
)

type worker struct {
	cfg     config.Config
	store   *db.Store
	discord *notify.Discord
	slack   *notify.Slack
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	w := &worker{cfg: cfg, store: store, discord: notify.NewDiscord(), slack: notify.NewSlack()}
	log.Printf("worker %q started (batch=%d, lease=%s, poll=%s)",
		cfg.WorkerID, cfg.ClaimBatch, cfg.LeaseDuration, cfg.PollInterval)
	w.run(ctx)
	log.Printf("worker %q stopped", cfg.WorkerID)
}

func (w *worker) run(ctx context.Context) {
	leaseSecs := int(w.cfg.LeaseDuration.Seconds())
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := w.store.ClaimDueMonitors(ctx, w.cfg.WorkerID, leaseSecs, w.cfg.ClaimBatch)
		if err != nil {
			log.Printf("claim error: %v", err)
			sleep(ctx, w.cfg.PollInterval)
			continue
		}
		if len(claimed) == 0 {
			sleep(ctx, w.cfg.PollInterval)
			continue
		}

		var wg sync.WaitGroup
		for _, m := range claimed {
			wg.Add(1)
			go func(m db.Monitor) {
				defer wg.Done()
				w.process(ctx, m)
			}(m)
		}
		wg.Wait()
	}
}

// process probes a single monitor and persists the outcome + any alert.
func (w *worker) process(ctx context.Context, m db.Monitor) {
	res := probe.Do(ctx, probe.Target{
		URL:             m.URL,
		Method:          m.Method,
		ExpectedStatus:  m.ExpectedStatus,
		TimeoutMs:       m.TimeoutMs,
		FollowRedirects: m.FollowRedirects,
		Headers:         headersToStrings(m.Headers),
	})

	dec := incident.Evaluate(
		incident.State{
			CurrentStatus:       m.CurrentStatus,
			ConsecutiveFailures: m.ConsecutiveFailures,
			FailThreshold:       m.FailThreshold,
		},
		res.Up, res.Error,
	)

	out := db.CheckOutcome{
		WorkerID: w.cfg.WorkerID,
		Status:   dec.ResultStatus,
	}
	if res.StatusCode > 0 {
		out.StatusCode = &res.StatusCode
	}
	rt := int(res.ResponseTime.Milliseconds())
	out.ResponseTimeMs = &rt
	if res.Error != "" {
		out.Error = &res.Error
	}

	resolved, err := w.store.ApplyCheckResult(ctx, m, out, db.MonitorUpdate{
		NewStatus:           dec.NewStatus,
		ConsecutiveFailures: dec.ConsecutiveFailures,
		OpenIncident:        dec.OpenIncident,
		ResolveIncident:     dec.ResolveIncident,
		Cause:               dec.Cause,
	})
	if err != nil {
		log.Printf("[%s] apply result for %s failed: %v", w.cfg.WorkerID, m.Name, err)
		return
	}

	log.Printf("[%s] %s -> %s (code=%d, %dms)", w.cfg.WorkerID, m.Name, dec.ResultStatus, res.StatusCode, rt)

	if dec.OpenIncident {
		w.alertDown(ctx, m, dec.Cause)
	}
	if resolved != nil && resolved.ResolvedAt != nil {
		w.alertRecovered(ctx, m, resolved.ResolvedAt.Sub(resolved.StartedAt))
	}
}

func (w *worker) alertDown(ctx context.Context, m db.Monitor, cause string) {
	byType, err := w.store.EnabledWebhooksByType(ctx, m.OrganizationID)
	if err != nil || len(byType) == 0 {
		return
	}
	if cause == "" {
		cause = "no response"
	}
	if hooks := byType["discord"]; len(hooks) > 0 {
		if err := w.discord.NotifyDown(ctx, hooks, m.Name, m.URL, cause); err != nil {
			log.Printf("[%s] discord down alert failed: %v", w.cfg.WorkerID, err)
		}
	}
	if hooks := byType["slack"]; len(hooks) > 0 {
		if err := w.slack.NotifyDown(ctx, hooks, m.Name, m.URL, cause); err != nil {
			log.Printf("[%s] slack down alert failed: %v", w.cfg.WorkerID, err)
		}
	}
}

func (w *worker) alertRecovered(ctx context.Context, m db.Monitor, downtime time.Duration) {
	byType, err := w.store.EnabledWebhooksByType(ctx, m.OrganizationID)
	if err != nil || len(byType) == 0 {
		return
	}
	if hooks := byType["discord"]; len(hooks) > 0 {
		if err := w.discord.NotifyRecovered(ctx, hooks, m.Name, m.URL, downtime); err != nil {
			log.Printf("[%s] discord recovered alert failed: %v", w.cfg.WorkerID, err)
		}
	}
	if hooks := byType["slack"]; len(hooks) > 0 {
		if err := w.slack.NotifyRecovered(ctx, hooks, m.Name, m.URL, downtime); err != nil {
			log.Printf("[%s] slack recovered alert failed: %v", w.cfg.WorkerID, err)
		}
	}
}

// headersToStrings flattens the jsonb headers map into string headers for probing.
func headersToStrings(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// sleep waits for d or until ctx is cancelled, whichever comes first.
func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
