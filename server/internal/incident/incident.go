// Package incident contains the pure state-transition logic that turns a single
// probe outcome into a decision: what the monitor's confirmed status becomes,
// and whether an incident should be opened or resolved. It touches no database,
// so the flapping/threshold rules are trivially unit-testable.
package incident

import "time"

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
	// reminder config: while the monitor stays down, repeat the down alert every
	// ReminderInterval. Zero disables reminders. LastAlertSentAt is when the most
	// recent down/reminder alert went out (nil if none yet).
	ReminderInterval time.Duration
	LastAlertSentAt  *time.Time
}

// Decision is the computed outcome to persist.
type Decision struct {
	ResultStatus        string // status of THIS probe, written to check_results (up|down)
	NewStatus           string // monitor's confirmed status after this probe
	ConsecutiveFailures int
	OpenIncident        bool // transitioned into down -> open an incident + alert
	ResolveIncident     bool // recovered from down -> resolve incident + alert
	SendReminder        bool // still down and a reminder interval has elapsed -> re-alert
	MarkAlerted         bool // a down/reminder alert is being sent -> stamp last_alert_sent_at
	Cause               string
}

// Evaluate applies the threshold rules. `cause` describes the failure (ignored
// when up); `now` is the probe time, used to decide whether a down reminder is
// due. A monitor is only declared down after FailThreshold consecutive failures,
// which suppresses single-blip flapping.
func Evaluate(prev State, probeUp bool, cause string, now time.Time) Decision {
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
		if prev.CurrentStatus != StatusDown {
			// transition into down -> open an incident and fire the first alert
			d.OpenIncident = true
		} else if reminderDue(prev, now) {
			// already down -> repeat the alert once the reminder interval elapses
			d.SendReminder = true
		}
	} else {
		// not yet confirmed down; keep prior confirmed status
		d.NewStatus = prev.CurrentStatus
	}
	// any down alert we emit (initial or reminder) restamps the reminder clock
	d.MarkAlerted = d.OpenIncident || d.SendReminder
	return d
}

// reminderDue reports whether enough time has passed since the last down alert
// to send another reminder. Reminders are off when ReminderInterval <= 0.
func reminderDue(prev State, now time.Time) bool {
	if prev.ReminderInterval <= 0 {
		return false
	}
	if prev.LastAlertSentAt == nil {
		// confirmed down but no alert recorded yet (e.g. legacy row) -> send one
		return true
	}
	return now.Sub(*prev.LastAlertSentAt) >= prev.ReminderInterval
}
