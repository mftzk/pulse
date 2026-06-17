package incident

import "testing"

func TestEvaluate_FirstFailureThreshold1OpensIncident(t *testing.T) {
	d := Evaluate(State{CurrentStatus: StatusUnknown, ConsecutiveFailures: 0, FailThreshold: 1}, false, "boom")
	if d.NewStatus != StatusDown {
		t.Fatalf("want down, got %s", d.NewStatus)
	}
	if !d.OpenIncident {
		t.Fatal("want OpenIncident=true on first confirmed down")
	}
	if d.ResolveIncident {
		t.Fatal("should not resolve")
	}
	if d.ConsecutiveFailures != 1 {
		t.Fatalf("want 1 failure, got %d", d.ConsecutiveFailures)
	}
	if d.Cause != "boom" {
		t.Fatalf("cause not propagated: %q", d.Cause)
	}
}

func TestEvaluate_BelowThresholdDoesNotDeclareDown(t *testing.T) {
	// threshold 3, first failure from an up monitor
	d := Evaluate(State{CurrentStatus: StatusUp, ConsecutiveFailures: 0, FailThreshold: 3}, false, "x")
	if d.NewStatus != StatusUp {
		t.Fatalf("want still up below threshold, got %s", d.NewStatus)
	}
	if d.OpenIncident {
		t.Fatal("must not open incident below threshold")
	}
	if d.ConsecutiveFailures != 1 {
		t.Fatalf("want 1, got %d", d.ConsecutiveFailures)
	}
}

func TestEvaluate_ReachesThresholdOpensOnce(t *testing.T) {
	// already 2 failures, threshold 3 -> this failure confirms down
	d := Evaluate(State{CurrentStatus: StatusUp, ConsecutiveFailures: 2, FailThreshold: 3}, false, "x")
	if d.NewStatus != StatusDown || !d.OpenIncident {
		t.Fatalf("want down+open, got status=%s open=%v", d.NewStatus, d.OpenIncident)
	}
}

func TestEvaluate_AlreadyDownDoesNotReopen(t *testing.T) {
	d := Evaluate(State{CurrentStatus: StatusDown, ConsecutiveFailures: 5, FailThreshold: 1}, false, "x")
	if d.NewStatus != StatusDown {
		t.Fatalf("want down, got %s", d.NewStatus)
	}
	if d.OpenIncident {
		t.Fatal("must not reopen an already-open incident")
	}
}

func TestEvaluate_RecoveryResolves(t *testing.T) {
	d := Evaluate(State{CurrentStatus: StatusDown, ConsecutiveFailures: 4, FailThreshold: 1}, true, "")
	if d.NewStatus != StatusUp {
		t.Fatalf("want up, got %s", d.NewStatus)
	}
	if !d.ResolveIncident {
		t.Fatal("want ResolveIncident=true on recovery")
	}
	if d.ConsecutiveFailures != 0 {
		t.Fatalf("want 0 failures after recovery, got %d", d.ConsecutiveFailures)
	}
}

func TestEvaluate_UpStaysUpNoResolve(t *testing.T) {
	d := Evaluate(State{CurrentStatus: StatusUp, ConsecutiveFailures: 0, FailThreshold: 1}, true, "")
	if d.ResolveIncident {
		t.Fatal("no incident to resolve when already up")
	}
}
