package db

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// monitorCols is the canonical column list / order used by scanMonitor.
const monitorCols = `id, organization_id, name, url, method, expected_status,
	interval_seconds, timeout_ms, follow_redirects, headers, fail_threshold,
	reminder_interval_seconds, enabled, current_status, consecutive_failures,
	last_checked_at, last_alert_sent_at, next_run_at, created_at`

func scanMonitor(row pgx.Row) (Monitor, error) {
	var m Monitor
	var headers []byte
	err := row.Scan(
		&m.ID, &m.OrganizationID, &m.Name, &m.URL, &m.Method, &m.ExpectedStatus,
		&m.IntervalSeconds, &m.TimeoutMs, &m.FollowRedirects, &headers, &m.FailThreshold,
		&m.ReminderIntervalSeconds, &m.Enabled, &m.CurrentStatus, &m.ConsecutiveFailures,
		&m.LastCheckedAt, &m.LastAlertSentAt, &m.NextRunAt, &m.CreatedAt,
	)
	if err != nil {
		return m, err
	}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &m.Headers)
	}
	if m.Headers == nil {
		m.Headers = map[string]any{}
	}
	return m, nil
}

// CreateMonitor inserts a new monitor; next_run_at defaults to now() so it is
// immediately claimable.
func (s *Store) CreateMonitor(ctx context.Context, m Monitor) (Monitor, error) {
	headers, _ := json.Marshal(m.Headers)
	if len(headers) == 0 {
		headers = []byte(`{}`)
	}
	row := s.Pool.QueryRow(ctx,
		`INSERT INTO monitors
		   (organization_id, name, url, method, expected_status, interval_seconds,
		    timeout_ms, follow_redirects, headers, fail_threshold,
		    reminder_interval_seconds, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING `+monitorCols,
		m.OrganizationID, m.Name, m.URL, m.Method, m.ExpectedStatus, m.IntervalSeconds,
		m.TimeoutMs, m.FollowRedirects, headers, m.FailThreshold,
		m.ReminderIntervalSeconds, m.Enabled,
	)
	return scanMonitor(row)
}

func (s *Store) ListMonitors(ctx context.Context, orgID string) ([]Monitor, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+monitorCols+` FROM monitors WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors := []Monitor{}
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}

func (s *Store) GetMonitor(ctx context.Context, orgID, id string) (Monitor, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+monitorCols+` FROM monitors WHERE id = $1 AND organization_id = $2`, id, orgID)
	m, err := scanMonitor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// UpdateMonitor updates user-editable fields. It also resets next_run_at to now()
// so interval changes take effect promptly.
func (s *Store) UpdateMonitor(ctx context.Context, m Monitor) (Monitor, error) {
	headers, _ := json.Marshal(m.Headers)
	if len(headers) == 0 {
		headers = []byte(`{}`)
	}
	row := s.Pool.QueryRow(ctx,
		`UPDATE monitors SET
		   name=$3, url=$4, method=$5, expected_status=$6, interval_seconds=$7,
		   timeout_ms=$8, follow_redirects=$9, headers=$10, fail_threshold=$11,
		   reminder_interval_seconds=$12, enabled=$13, next_run_at = now()
		 WHERE id=$1 AND organization_id=$2
		 RETURNING `+monitorCols,
		m.ID, m.OrganizationID, m.Name, m.URL, m.Method, m.ExpectedStatus, m.IntervalSeconds,
		m.TimeoutMs, m.FollowRedirects, headers, m.FailThreshold,
		m.ReminderIntervalSeconds, m.Enabled,
	)
	res, err := scanMonitor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrNotFound
	}
	return res, err
}

func (s *Store) DeleteMonitor(ctx context.Context, orgID, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM monitors WHERE id = $1 AND organization_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClaimDueMonitors atomically leases up to `limit` due, unleased monitors to the
// given worker using FOR UPDATE SKIP LOCKED. Two workers can never claim the same
// row, which is what spreads (shards) the load across the worker fleet.
func (s *Store) ClaimDueMonitors(ctx context.Context, workerID string, leaseSeconds, limit int) ([]Monitor, error) {
	rows, err := s.Pool.Query(ctx,
		`UPDATE monitors
		    SET leased_until = now() + make_interval(secs => $2), leased_by = $1
		  WHERE id IN (
		      SELECT id FROM monitors
		       WHERE enabled AND next_run_at <= now()
		         AND (leased_until IS NULL OR leased_until < now())
		       ORDER BY next_run_at
		       FOR UPDATE SKIP LOCKED
		       LIMIT $3
		  )
		  RETURNING `+monitorCols,
		workerID, leaseSeconds, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimed := []Monitor{}
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, m)
	}
	return claimed, rows.Err()
}

// CheckOutcome is the row written to check_results for a single probe.
type CheckOutcome struct {
	WorkerID       string
	Status         string // "up" | "down"
	StatusCode     *int
	ResponseTimeMs *int
	Error          *string
}

// MonitorUpdate describes the state change to apply to the monitor row plus any
// incident open/resolve decided by the caller (incident package).
type MonitorUpdate struct {
	NewStatus           string
	ConsecutiveFailures int
	OpenIncident        bool
	ResolveIncident     bool
	MarkAlerted         bool // stamp last_alert_sent_at = now() (initial down or reminder)
	Cause               string
}

// ApplyCheckResult records a probe result and advances the monitor's schedule in
// one transaction: insert the result row, update status + next_run_at + clear the
// lease, and open/resolve an incident as decided. When an incident is resolved it
// is returned (with its StartedAt) so the caller can report downtime duration.
func (s *Store) ApplyCheckResult(ctx context.Context, m Monitor, out CheckOutcome, upd MonitorUpdate) (*Incident, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO check_results
		   (monitor_id, organization_id, worker_id, status, status_code, response_time_ms, error)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		m.ID, m.OrganizationID, out.WorkerID, out.Status, out.StatusCode, out.ResponseTimeMs, out.Error,
	); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE monitors
		    SET current_status = $2, consecutive_failures = $3,
		        last_checked_at = now(),
		        last_alert_sent_at = CASE WHEN $4 THEN now() ELSE last_alert_sent_at END,
		        next_run_at = now() + make_interval(secs => interval_seconds),
		        leased_until = NULL, leased_by = NULL
		  WHERE id = $1`,
		m.ID, upd.NewStatus, upd.ConsecutiveFailures, upd.MarkAlerted,
	); err != nil {
		return nil, err
	}

	if upd.OpenIncident {
		// the partial unique index keeps at most one open incident per monitor
		if _, err := tx.Exec(ctx,
			`INSERT INTO incidents (monitor_id, organization_id, cause) VALUES ($1, $2, $3)
			 ON CONFLICT (monitor_id) WHERE resolved_at IS NULL DO NOTHING`,
			m.ID, m.OrganizationID, upd.Cause,
		); err != nil {
			return nil, err
		}
	}

	var resolved *Incident
	if upd.ResolveIncident {
		var inc Incident
		err := tx.QueryRow(ctx,
			`UPDATE incidents SET resolved_at = now()
			  WHERE monitor_id = $1 AND resolved_at IS NULL
			  RETURNING id, monitor_id, organization_id, started_at, resolved_at, cause`,
			m.ID,
		).Scan(&inc.ID, &inc.MonitorID, &inc.OrganizationID, &inc.StartedAt, &inc.ResolvedAt, &inc.Cause)
		if err == nil {
			resolved = &inc
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return resolved, nil
}

// RecentResults returns the most recent check results for a monitor (newest first).
func (s *Store) RecentResults(ctx context.Context, orgID, monitorID string, limit int) ([]CheckResult, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, monitor_id, organization_id, worker_id, checked_at, status, status_code, response_time_ms, error
		   FROM check_results
		  WHERE monitor_id = $1 AND organization_id = $2
		  ORDER BY checked_at DESC
		  LIMIT $3`,
		monitorID, orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []CheckResult{}
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.ID, &r.MonitorID, &r.OrganizationID, &r.WorkerID, &r.CheckedAt,
			&r.Status, &r.StatusCode, &r.ResponseTimeMs, &r.Error); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// MonthlySLA returns count-based uptime per calendar month for a monitor,
// covering the current month plus the previous (months-1) months, newest first.
func (s *Store) MonthlySLA(ctx context.Context, orgID, monitorID string, months int) ([]MonthlySLA, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT to_char(date_trunc('month', checked_at), 'YYYY-MM') AS month,
		        count(*) AS total,
		        count(*) FILTER (WHERE status = 'up') AS up,
		        (avg(response_time_ms) FILTER (WHERE response_time_ms IS NOT NULL))::float8 AS avg_ms
		   FROM check_results
		  WHERE monitor_id = $1 AND organization_id = $2
		    AND checked_at >= date_trunc('month', now()) - make_interval(months => $3 - 1)
		  GROUP BY 1
		  ORDER BY 1 DESC`,
		monitorID, orgID, months,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []MonthlySLA{}
	for rows.Next() {
		var m MonthlySLA
		var avg *float64
		if err := rows.Scan(&m.Month, &m.Total, &m.Up, &avg); err != nil {
			return nil, err
		}
		m.UptimePct = uptimePct(m.Up, m.Total)
		m.AvgMs = roundAvg(avg)
		out = append(out, m)
	}
	return out, rows.Err()
}

// DailySLA returns count-based uptime per day within [from, to), oldest first.
func (s *Store) DailySLA(ctx context.Context, orgID, monitorID string, from, to time.Time) ([]DailySLA, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT to_char(date_trunc('day', checked_at), 'YYYY-MM-DD') AS day,
		        count(*) AS total,
		        count(*) FILTER (WHERE status = 'up') AS up,
		        (avg(response_time_ms) FILTER (WHERE response_time_ms IS NOT NULL))::float8 AS avg_ms
		   FROM check_results
		  WHERE monitor_id = $1 AND organization_id = $2
		    AND checked_at >= $3 AND checked_at < $4
		  GROUP BY 1
		  ORDER BY 1 ASC`,
		monitorID, orgID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DailySLA{}
	for rows.Next() {
		var d DailySLA
		var avg *float64
		if err := rows.Scan(&d.Day, &d.Total, &d.Up, &avg); err != nil {
			return nil, err
		}
		d.UptimePct = uptimePct(d.Up, d.Total)
		d.AvgMs = roundAvg(avg)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ResultsInRange returns raw check results within [from, to) newest-first,
// paginated by limit/offset, plus the total count for the range.
func (s *Store) ResultsInRange(ctx context.Context, orgID, monitorID string, from, to time.Time, limit, offset int) ([]CheckResult, int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM check_results
		  WHERE monitor_id = $1 AND organization_id = $2
		    AND checked_at >= $3 AND checked_at < $4`,
		monitorID, orgID, from, to,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT id, monitor_id, organization_id, worker_id, checked_at, status, status_code, response_time_ms, error
		   FROM check_results
		  WHERE monitor_id = $1 AND organization_id = $2
		    AND checked_at >= $3 AND checked_at < $4
		  ORDER BY checked_at DESC
		  LIMIT $5 OFFSET $6`,
		monitorID, orgID, from, to, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []CheckResult{}
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.ID, &r.MonitorID, &r.OrganizationID, &r.WorkerID, &r.CheckedAt,
			&r.Status, &r.StatusCode, &r.ResponseTimeMs, &r.Error); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}
	return results, total, rows.Err()
}

// uptimePct returns up/total*100 rounded to 2 decimals (0 when no checks).
func uptimePct(up, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(up)/float64(total)*10000) / 100
}

// roundAvg rounds a nullable average milliseconds value to the nearest int.
func roundAvg(avg *float64) *int {
	if avg == nil {
		return nil
	}
	n := int(math.Round(*avg))
	return &n
}

// ListIncidents returns incidents for an org, newest first, joined with monitor name.
func (s *Store) ListIncidents(ctx context.Context, orgID string, limit int) ([]Incident, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT i.id, i.monitor_id, i.organization_id, m.name, i.started_at, i.resolved_at, i.cause
		   FROM incidents i JOIN monitors m ON m.id = i.monitor_id
		  WHERE i.organization_id = $1
		  ORDER BY i.started_at DESC
		  LIMIT $2`,
		orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	incidents := []Incident{}
	for rows.Next() {
		var i Incident
		if err := rows.Scan(&i.ID, &i.MonitorID, &i.OrganizationID, &i.MonitorName, &i.StartedAt, &i.ResolvedAt, &i.Cause); err != nil {
			return nil, err
		}
		incidents = append(incidents, i)
	}
	return incidents, rows.Err()
}
