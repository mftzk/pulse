-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- many-to-many: a user belongs to many orgs, an org has many users
CREATE TABLE organization_users (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member',  -- owner | member
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX idx_org_users_user ON organization_users(user_id);

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

-- a "monitor" is a monitored domain/endpoint owned by an org.
-- this table doubles as the schedule + work queue (next_run_at + lease).
CREATE TABLE monitors (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                 TEXT NOT NULL,
    url                  TEXT NOT NULL,
    method               TEXT NOT NULL DEFAULT 'GET',
    expected_status      INT  NOT NULL DEFAULT 0,      -- 0 => any 2xx is "up"
    interval_seconds     INT  NOT NULL DEFAULT 60,
    timeout_ms           INT  NOT NULL DEFAULT 10000,
    follow_redirects     BOOLEAN NOT NULL DEFAULT true,
    headers              JSONB NOT NULL DEFAULT '{}'::jsonb,
    fail_threshold       INT  NOT NULL DEFAULT 1,      -- consecutive fails before declaring down
    enabled              BOOLEAN NOT NULL DEFAULT true,
    current_status       TEXT NOT NULL DEFAULT 'unknown', -- up | down | unknown
    consecutive_failures INT  NOT NULL DEFAULT 0,
    last_checked_at      TIMESTAMPTZ,
    next_run_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    leased_until         TIMESTAMPTZ,
    leased_by            TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- the claim query orders by next_run_at over due+unleased rows
CREATE INDEX idx_monitors_claim ON monitors(next_run_at) WHERE enabled;
CREATE INDEX idx_monitors_org ON monitors(organization_id);

CREATE TABLE check_results (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    monitor_id       UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    organization_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id        TEXT NOT NULL,
    checked_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    status           TEXT NOT NULL,        -- up | down
    status_code      INT,
    response_time_ms INT,
    error            TEXT
);
CREATE INDEX idx_results_monitor ON check_results(monitor_id, checked_at DESC);

CREATE TABLE incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id      UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    cause           TEXT
);
CREATE INDEX idx_incidents_monitor ON incidents(monitor_id, started_at DESC);
-- at most one ongoing incident per monitor
CREATE UNIQUE INDEX idx_incidents_open ON incidents(monitor_id) WHERE resolved_at IS NULL;

CREATE TABLE notification_channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type            TEXT NOT NULL DEFAULT 'discord',
    name            TEXT NOT NULL DEFAULT 'Discord',
    webhook_url     TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_channels_org ON notification_channels(organization_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS check_results;
DROP TABLE IF EXISTS monitors;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS organization_users;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
