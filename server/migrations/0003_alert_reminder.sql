-- +goose Up
-- +goose StatementBegin
-- Re-alert reminder: while a monitor stays down, the down alert is repeated
-- every reminder_interval_seconds. 0 disables reminders (alert only on the
-- initial down + recovery transitions). Default 600s (10 minutes).
ALTER TABLE monitors ADD COLUMN reminder_interval_seconds INT NOT NULL DEFAULT 600;
-- when the last down/reminder alert was sent, so the worker knows when the next
-- reminder is due. NULL until the first down alert fires.
ALTER TABLE monitors ADD COLUMN last_alert_sent_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE monitors DROP COLUMN last_alert_sent_at;
ALTER TABLE monitors DROP COLUMN reminder_interval_seconds;
-- +goose StatementEnd
