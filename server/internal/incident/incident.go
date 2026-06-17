// Package incident contains the pure state-transition logic that turns a single
// probe outcome into a decision: what the monitor's confirmed status becomes,
// and whether an incident should be opened or resolved. It touches no database,
// so the flapping/threshold rules are trivially unit-testable.
package incident

const (
	StatusUp      = "up"
	StatusDown    = "down"
	StatusUnknown = "unknown"
)

// State is the relevant prior state of a monitor plus its configured threshold.
type State struct {
	CurrentStatus       string
	ConsecutiveFailures int
	FailThreshold       int // number of consecutive failures required to declare down
}

// Decision is the computed outcome to persist.
type Decision struct {
	ResultStatus        string // status of THIS probe, written to check_results (up|down)
	NewStatus           string // monitor's confirmed status after this probe
	ConsecutiveFailures int
	OpenIncident        bool // transitioned into down -> open an incident + alert
	ResolveIncident     bool // recovered from down -> resolve incident + alert
	Cause               string
}

// Evaluate applies the threshold rules. `cause` describes the failure (ignored
// when up). A monitor is only declared down after FailThreshold consecutive
// failures, which suppresses single-blip flapping.
func Evaluate(prev State, probeUp bool, cause string) Decision {
	threshold := prev.FailThreshold
	if threshold < 1 {
		threshold = 1
	}

	if probeUp {
		return Decision{
			ResultStatus:        StatusUp,
			NewStatus:           StatusUp,
			ConsecutiveFailures: 0,
			// recovering from a confirmed-down state resolves the open incident
			ResolveIncident: prev.CurrentStatus == StatusDown,
		}
	}

	fails := prev.ConsecutiveFailures + 1
	d := Decision{
		ResultStatus:        StatusDown,
		ConsecutiveFailures: fails,
		Cause:               cause,
	}
	if fails >= threshold {
		d.NewStatus = StatusDown
		// only open a new incident on the transition into down
		d.OpenIncident = prev.CurrentStatus != StatusDown
	} else {
		// not yet confirmed down; keep prior confirmed status
		d.NewStatus = prev.CurrentStatus
	}
	return d
}
