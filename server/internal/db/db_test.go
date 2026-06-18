package db_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aji/pulse/internal/db"
)

// These tests require a throwaway Postgres. Set TEST_DATABASE_URL to run them;
// otherwise they are skipped (so plain `go test ./...` stays hermetic).
func testStore(t *testing.T) *db.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping db integration test")
	}
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestClaimSkipLocked_NoDoubleClaim(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	user, err := s.CreateUser(ctx, "claim-"+randSuffix(), "claim-"+randSuffix()+"@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	org, err := s.CreateOrgWithOwner(ctx, "claim org", "claim-"+randSuffix(), user.ID)
	if err != nil {
		t.Fatal(err)
	}

	const n = 8
	for i := 0; i < n; i++ {
		if _, err := s.CreateMonitor(ctx, db.Monitor{
			OrganizationID: org.ID, Name: "m", URL: "http://example.com",
			Method: "GET", IntervalSeconds: 60, TimeoutMs: 1000, FailThreshold: 1, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// two workers claim concurrently; SKIP LOCKED must partition the rows
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			claimed, err := s.ClaimDueMonitors(ctx, worker, 30, n)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			mu.Lock()
			for _, m := range claimed {
				seen[m.ID]++
			}
			mu.Unlock()
		}("worker-" + string(rune('A'+w)))
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("expected %d distinct monitors claimed, got %d", n, len(seen))
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("monitor %s claimed %d times (double-claim!)", id, count)
		}
	}
}

func TestApplyCheckResult_OpensAndResolvesIncident(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	user, _ := s.CreateUser(ctx, "inc-"+randSuffix(), "inc-"+randSuffix()+"@example.com", "x")
	org, _ := s.CreateOrgWithOwner(ctx, "inc org", "inc-"+randSuffix(), user.ID)
	m, err := s.CreateMonitor(ctx, db.Monitor{
		OrganizationID: org.ID, Name: "site", URL: "http://example.com",
		Method: "GET", IntervalSeconds: 60, TimeoutMs: 1000, FailThreshold: 1, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// go down -> opens incident
	_, err = s.ApplyCheckResult(ctx, m, db.CheckOutcome{WorkerID: "w", Status: "down"},
		db.MonitorUpdate{NewStatus: "down", ConsecutiveFailures: 1, OpenIncident: true, Cause: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	incs, _ := s.ListIncidents(ctx, org.ID, 10)
	if len(incs) != 1 || incs[0].ResolvedAt != nil {
		t.Fatalf("expected 1 open incident, got %+v", incs)
	}

	// recover -> resolves incident
	m.CurrentStatus = "down"
	resolved, err := s.ApplyCheckResult(ctx, m, db.CheckOutcome{WorkerID: "w", Status: "up"},
		db.MonitorUpdate{NewStatus: "up", ConsecutiveFailures: 0, ResolveIncident: true})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || resolved.ResolvedAt == nil {
		t.Fatalf("expected resolved incident, got %+v", resolved)
	}
}

func TestMonthlySLAAndRangedHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	user, _ := s.CreateUser(ctx, "sla-"+randSuffix(), "sla-"+randSuffix()+"@example.com", "x")
	org, _ := s.CreateOrgWithOwner(ctx, "sla org", "sla-"+randSuffix(), user.ID)
	m, err := s.CreateMonitor(ctx, db.Monitor{
		OrganizationID: org.ID, Name: "site", URL: "http://example.com",
		Method: "GET", IntervalSeconds: 60, TimeoutMs: 1000, FailThreshold: 1, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	insert := func(at time.Time, status string) {
		t.Helper()
		rt := 100
		if _, err := s.Pool.Exec(ctx,
			`INSERT INTO check_results (monitor_id, organization_id, worker_id, checked_at, status, response_time_ms)
			 VALUES ($1, $2, 'w', $3, $4, $5)`,
			m.ID, org.ID, at, status, rt); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC()
	// current month: 4 up + 1 down => 80%
	for i := 0; i < 4; i++ {
		insert(now, "up")
	}
	insert(now, "down")
	// 4 months ago: must be excluded from a 3-month window
	insert(now.AddDate(0, -4, 0), "down")

	sla, err := s.MonthlySLA(ctx, org.ID, m.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sla) == 0 {
		t.Fatalf("expected at least the current month, got none")
	}
	cur := sla[0] // newest first
	if cur.Total != 5 || cur.Up != 4 {
		t.Fatalf("current month: want total=5 up=4, got total=%d up=%d", cur.Total, cur.Up)
	}
	if cur.UptimePct != 80 {
		t.Fatalf("current month uptime: want 80, got %v", cur.UptimePct)
	}
	for _, mo := range sla {
		if mo.Month < now.AddDate(0, -3, 0).Format("2006-01") {
			t.Fatalf("month %s is older than the 3-month window", mo.Month)
		}
	}

	// Ranged raw history with pagination over the current month's 5 rows.
	from := now.AddDate(0, 0, -1)
	to := now.Add(time.Hour)
	page1, total, err := s.ResultsInRange(ctx, org.ID, m.ID, from, to, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("range total: want 5, got %d", total)
	}
	if len(page1) != 2 {
		t.Fatalf("range page1: want 2 rows, got %d", len(page1))
	}
	page3, _, err := s.ResultsInRange(ctx, org.ID, m.ID, from, to, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(page3) != 1 {
		t.Fatalf("range page3 (offset 4): want 1 row, got %d", len(page3))
	}

	// Daily rollup within the same range.
	daily, err := s.DailySLA(ctx, org.ID, m.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	var dTotal, dUp int
	for _, d := range daily {
		dTotal += d.Total
		dUp += d.Up
	}
	if dTotal != 5 || dUp != 4 {
		t.Fatalf("daily rollup: want total=5 up=4, got total=%d up=%d", dTotal, dUp)
	}
}

func randSuffix() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
