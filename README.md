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
