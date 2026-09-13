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
