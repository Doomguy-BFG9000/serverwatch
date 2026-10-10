# ServerWatch Community Edition

ServerWatch is a lightweight self-hosted monitoring platform for servers, websites, APIs, SSL certificates, cron jobs, and infrastructure health.

## Project Status

Early development / Community Edition.

## Initial Goals

- Central dashboard for multiple servers
- HTTP/HTTPS uptime monitoring
- SSL certificate expiry monitoring
- TCP port monitoring
- Ping/ICMP host reachability monitoring
- Cron heartbeat monitoring
- Lightweight ServerWatch agent
- CPU, RAM, disk, load, and uptime metrics
- Email and Telegram alerts
- Docker-based deployment
- Simple self-hosted installation

## Architecture

ServerWatch consists of:

- Go backend/API
- Go monitoring agent
- React + TypeScript frontend
- PostgreSQL database
- Docker Compose deployment

## Repository Structure

- `server/` - central ServerWatch backend
- `agent/` - lightweight monitored-server agent
- `web/` - React frontend
- `database/` - migrations and database-related files
- `deployments/` - deployment configuration
- `docs/` - project documentation
- `scripts/` - helper/install scripts
- `tests/` - integration and system tests

## License

Apache License 2.0

## Ping monitors

Ping targets are hostnames or IP addresses only (for example `server.example.com`, `192.168.1.1`, or `::1`). Do not include a protocol, path, or port. ServerWatch invokes the system `ping` executable, so Linux/Ubuntu installations need `iputils-ping`:

```sh
sudo apt install iputils-ping
```

The ServerWatch service does not need to run as root. Its systemd service
retains `NoNewPrivileges=true`; configure Linux unprivileged ICMP with
`net.ipv4.ping_group_range` for the local `serverwatch` GID. See the
[Ping monitoring architecture decision](docs/ARCHITECTURE.md#ping-monitoring)
and [Ping troubleshooting](docs/TROUBLESHOOTING.md#ping-monitor-is-down-but-manual-ping-works)
for validation and persistent host configuration.

## Port monitors

Multi-port monitors check several explicitly configured TCP ports on one host.
The target is a hostname or IP address only (for example `server.example.com`,
`192.168.1.10`, or `2001:db8::10`); ports are configured separately, for
example `22, 80, 443`. Every configured port must accept a TCP connection for
the monitor to report `UP`.

This is monitoring, not scanning. ServerWatch never scans ranges, never
discovers ports automatically, and only dials ports the user configured, with a
maximum of 20 ports per monitor. See
[Port monitoring](docs/CONFIGURATION.md#multi-port-monitoring) and
[Port troubleshooting](docs/TROUBLESHOOTING.md#port-monitor-shows-down).

## Monitor groups

Monitor groups are an optional organizational feature. A monitor may belong to
zero or one group, and groups have no effect on scheduling, checking, or
alerting.

Group names are trimmed, must be non-empty, are limited to 100 characters, and
must be unique case-insensitively. Deleting a group never deletes its monitors;
those monitors become ungrouped. The dashboard can filter the visible monitor
list by group, Ungrouped, or All Groups without pausing or changing monitors.

## Monitor search and sorting

The dashboard lets you search the loaded monitor list and reorder it. Search is
client-side and case-insensitive, ignores leading and trailing whitespace, and
matches monitor names and targets. An empty search shows every monitor. Search
composes with the existing group filter: monitors are first limited by the
selected group, then by the search text, and then sorted.

Sorting is also client-side and offers Name A → Z, Name Z → A, Status, Monitor
type, Newest, and Oldest, plus a Default option that preserves the existing
incoming order. Status sorting reuses the dashboard presentation state, so
Disabled ranks after Maintenance, and both rank after the normal monitoring
statuses. Search and sorting only change which monitors are visible and in what
order; they never modify a monitor, change monitoring behavior, or affect
scheduling, checkers, alerting, or maintenance. Selections are not persisted
across a browser reload.

## Dashboard health summary

The dashboard shows a compact health summary above the monitor controls:
Total Monitors, Up, Down, Maintenance, Disabled, and Pending / No Data. The
summary is frontend-only and derived from the monitor list the browser has
already loaded; it adds no endpoint, query, or migration.

Every configured monitor is counted in exactly one bucket. A disabled monitor
is always counted as Disabled, even if it is also in maintenance or its last
result was DOWN. Otherwise a monitor in maintenance is counted as Maintenance,
regardless of its last result. Remaining monitors use their current monitoring
status: Down, Up, or Pending / No Data when no result exists yet. The buckets
partition the monitors, so Up + Down + Maintenance + Disabled + Pending always
equals Total Monitors.

The summary is a global overview: it counts every configured monitor and is not
affected by the Group filter, Search, or Sort controls, which only change the
monitor cards shown below it. The summary cards are informational and are not
clickable. See [Health summary](docs/CONFIGURATION.md#dashboard-health-summary)
and [Health summary troubleshooting](docs/TROUBLESHOOTING.md#dashboard-health-summary).

## Monitor maintenance mode

Maintenance mode is a manual, per-monitor switch for planned work. While it is
active, ServerWatch stops running scheduled checks for that monitor, so no new
scheduled result is written and no DOWN, recovery, or SSL-expiry alert is sent
from scheduled checks. The monitor, its configuration, its group, its check
history, and its alert state are all preserved.

Maintenance mode is independent of the enabled flag. A monitor is normally one
of three states:

- Enabled and normal: scheduled monitoring is active.
- Enabled and in maintenance: configured and enabled, but scheduled monitoring
  is temporarily paused.
- Disabled: administratively turned off.

Exiting maintenance does not run a check immediately and does not fabricate an
UP result or recovery event. The monitor becomes eligible for the scheduler
again, and the first real scheduled check is processed with the existing alert
state. The dashboard shows a distinct `Maintenance` badge and provides
`Enter Maintenance` / `Exit Maintenance` actions with a confirmation. See
[Maintenance mode](docs/CONFIGURATION.md#maintenance-mode) and
[Maintenance troubleshooting](docs/TROUBLESHOOTING.md#maintenance-mode).

## API access token

ServerWatch Community Edition can require a bearer token on its API surface.
This is opt-in and off by default: when `SERVERWATCH_API_TOKEN` is unset, the
API behaves exactly as before, so enabling the feature can never break an
existing installation.

Set `SERVERWATCH_API_TOKEN` (for example `openssl rand -hex 32`) in the
environment file and restart the service. While it is set:

- every `/api/v1/...` request must send `Authorization: Bearer <token>`;
  requests without it receive `401 Unauthorized`;
- the `/health` and `/ready` probes stay open so systemd, Docker, and Nginx
  health checks keep working;
- the dashboard prompts for the token and stores it in the browser only, then
  sends it on every request.

The token is a single shared secret for the whole installation. It is not a
per-user account system. See
[API access token](docs/CONFIGURATION.md#api-access-token) for configuration
and operational details.

## Account authentication

Setting `SERVERWATCH_AUTH_ENABLED=true` turns on single-user account
authentication for the dashboard and API. Like the API token, this is opt-in
and off by default, so an existing open installation is unaffected until the
operator enables it.

- On first visit, ServerWatch asks you to create the administrator account
  (email and password). This first-run setup is only offered while no account
  exists, so it cannot be used to add users or take over an initialized
  deployment.
- After that, visitors must sign in. Sessions are tracked with an HttpOnly
  cookie and can be ended with **Sign out**.
- `/health`, `/ready`, and the sign-in endpoints stay open; every other
  `/api/v1/...` route requires a valid session. The `SERVERWATCH_API_TOKEN`
  bearer token keeps working for scripts and health tooling.

Passwords are stored as PBKDF2-HMAC-SHA256 hashes with a random per-password
salt. This release provides a single administrator account, not per-user roles
or multi-tenant permissions. See
[Account authentication](docs/CONFIGURATION.md#account-authentication).

## Security hardening

Phase 0.50 adds hardening that is on by default plus opt-in features:

- Strict security headers (CSP, `X-Frame-Options: DENY`, `nosniff`, referrer and
  cross-origin policies) on every response; `Strict-Transport-Security` only
  over TLS.
- Per-client API rate limiting (`429` over limit) with edge `limit_req` in the
  Nginx template.
- An audit log of every mutating API call, attributed to the user, the shared
  token, or anonymous.
- An optional Prometheus `/metrics` endpoint
  (`SERVERWATCH_METRICS_ENABLED=true`).
- Ready-to-use `deployments/nginx/` and `deployments/systemd/` templates with
  TLS guidance and service sandboxing.

See [Security hardening](docs/CONFIGURATION.md#security-hardening) for details.

## Data retention

Check history (`monitor_results`) is the fastest-growing table, so a built-in job
keeps it bounded:

```dotenv
SERVERWATCH_RESULT_RETENTION_DAYS=30
```

Once at startup and then daily, it deletes results older than the window while
always keeping the newest result for every monitor (so no monitor's last status
is ever blanked). Set it to `0` to keep history forever. See
[Data retention](docs/CONFIGURATION.md#data-retention) and
[Backup and restore](docs/BACKUP_RESTORE.md) (`scripts/backup.sh`,
`scripts/restore.sh`).

## Alert channels

DOWN, recovery, and SSL-expiry alerts are delivered to every enabled channel.
Email (SMTP) and Telegram can be used independently or together:

```dotenv
SERVERWATCH_ALERT_EMAIL_ENABLED=false
SMTP_HOST=smtp.example.com
SMTP_PORT=587
...

SERVERWATCH_ALERT_TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=123456:ABC-DEF...
TELEGRAM_CHAT_ID=123456789
```

A failure on one channel does not prevent the others from delivering. See
[Telegram alerts](docs/CONFIGURATION.md#telegram-alerts) and
[SMTP email alerts](docs/CONFIGURATION.md#smtp-email-alerts).
