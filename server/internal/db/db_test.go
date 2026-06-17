package db_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"

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

	user, err := s.CreateUser(ctx, "claim-"+randSuffix(), "x")
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

	user, _ := s.CreateUser(ctx, "inc-"+randSuffix(), "x")
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

func randSuffix() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
