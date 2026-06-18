package db

import "time"

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        *string   `json:"email,omitempty"` // nullable: legacy accounts predate the email field
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Role      string    `json:"role,omitempty"` // current user's role, when listed per-user
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type Monitor struct {
	ID                  string         `json:"id"`
	OrganizationID      string         `json:"organization_id"`
	Name                string         `json:"name"`
	URL                 string         `json:"url"`
	Method              string         `json:"method"`
	ExpectedStatus      int            `json:"expected_status"`
	IntervalSeconds     int            `json:"interval_seconds"`
	TimeoutMs           int            `json:"timeout_ms"`
	FollowRedirects     bool           `json:"follow_redirects"`
	Headers             map[string]any `json:"headers"`
	FailThreshold       int            `json:"fail_threshold"`
	Enabled             bool           `json:"enabled"`
	CurrentStatus       string         `json:"current_status"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	LastCheckedAt       *time.Time     `json:"last_checked_at"`
	NextRunAt           time.Time      `json:"next_run_at"`
	CreatedAt           time.Time      `json:"created_at"`
}

type CheckResult struct {
	ID             int64     `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	OrganizationID string    `json:"organization_id"`
	WorkerID       string    `json:"worker_id"`
	CheckedAt      time.Time `json:"checked_at"`
	Status         string    `json:"status"`
	StatusCode     *int      `json:"status_code"`
	ResponseTimeMs *int      `json:"response_time_ms"`
	Error          *string   `json:"error"`
}

type Incident struct {
	ID             string     `json:"id"`
	MonitorID      string     `json:"monitor_id"`
	OrganizationID string     `json:"organization_id"`
	MonitorName    string     `json:"monitor_name,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	Cause          *string    `json:"cause"`
}

type NotificationChannel struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Type           string    `json:"type"`
	Name           string    `json:"name"`
	WebhookURL     string    `json:"webhook_url"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}
