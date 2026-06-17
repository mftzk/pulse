# Pulse — Distributed HTTP/HTTPS Uptime Monitoring

Uptime-Kuma–style checker focused on **HTTP/HTTPS** endpoints, with **distributed
workers** that shard the check load, a **lightweight polling UI** (no websockets),
**username/password auth**, **organizations** (users ↔ orgs are many-to-many,
domains belong to an org), and **Discord** alerting. Warm, claude.ai-inspired theme.

## Architecture

```
            ┌─────────────┐      proxy /api/*        ┌──────────────┐
  browser ──▶   web (Next) ├──────────────────────────▶   api (Go)   │
            └─────────────┘   (same-origin cookie)    └──────┬───────┘
                                                             │ SQL
   ┌──────────┐  ┌──────────┐  ┌──────────┐           ┌──────▼───────┐
   │ worker 1 │  │ worker 2 │  │ worker N │ ──────────▶│  Postgres    │
   └──────────┘  └──────────┘  └──────────┘  claim via └──────────────┘
        probe HTTP/HTTPS, write results,   FOR UPDATE SKIP LOCKED
        fire Discord on transition         (no extra queue/broker)
```

- **`server/`** — one Go module, two binaries:
  - `cmd/api` — REST API, auth (bcrypt + httpOnly session cookie), org-scoping,
    monitor/incident/channel CRUD. Runs migrations on boot.
  - `cmd/worker` — loop that **claims due monitors** with `FOR UPDATE SKIP LOCKED`
    (so two workers never grab the same check → automatic sharding), probes them,
    records results, and fires Discord alerts on down/recover transitions.
- **`web/`** — Next.js 15 + Tailwind + Radix. Polls the API (~15–20s). Proxies
  `/api/*` to the API via `next.config` rewrites, so the session cookie is
  same-origin (no CORS, no websockets).
- **Postgres** is the database *and* the work queue — the `monitors` table
  doubles as the schedule (`next_run_at` + a `leased_until`/`leased_by` lease).

## Run it (Docker Compose)

```bash
cp .env.example .env          # adjust secrets
docker compose up --build     # web :3000, api :8080, postgres

# add more workers — they auto-shard the load:
docker compose up -d --scale worker=3
```

Open http://localhost:3000, register, add a monitor. Add a Discord webhook under
**Settings → Discord alerts** to get notified on down/recover.

## Local development (without Docker)

```bash
# Postgres must be running; export its URL
export DATABASE_URL='postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable'

# API (migrates on boot)
cd server && go run ./cmd/api

# one or more workers (each picks a unique id from its hostname)
WORKER_ID=w1 go run ./cmd/worker

# web (proxies to the API)
cd web && API_URL=http://localhost:8080 bun run dev
```

## Tests

```bash
cd server
go test ./...                                  # pure unit tests (probe, incident, auth)
TEST_DATABASE_URL='postgres://…' go test ./...  # also runs DB integration tests
```

## Configuration (env)

| Var | Used by | Default | Notes |
|-----|---------|---------|-------|
| `DATABASE_URL` | api, worker | — | **required** |
| `SESSION_SECRET` | api | dev value | set in production |
| `COOKIE_SECURE` | api | `false` | set `true` behind HTTPS |
| `ALLOW_ORIGIN` | api | `*` | CORS origin (unused when proxied) |
| `HTTP_ADDR` | api | `:8080` | |
| `WORKER_ID` | worker | hostname | unique per worker |
| `CLAIM_BATCH` | worker | `10` | monitors claimed per tick |
| `LEASE_DURATION` | worker | `30s` | claim lease length |
| `POLL_INTERVAL` | worker | `2s` | idle poll cadence |
| `API_URL` | web | `http://localhost:8080` | upstream for `/api/*` |

## Roadmap (not in v1)

Public status pages, heavier uptime charts, and org-scoped API keys are
deliberately out of scope. `check_results` and `incidents` are already stored,
so those are additive views — no data-model change required.
