# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately rather than in a public issue.
Open a GitHub security advisory for the repository, or contact the maintainers
directly. Include the affected version (`GET /health` reports it), a description,
and reproduction steps. We will acknowledge the report and coordinate a fix and
disclosure.

## Supported versions

The latest released Community Edition version receives security fixes.

## Security model

- **Authentication is optional.** A fresh install is open on the local network
  until you enable account authentication or the shared API token.
- **The API token is a single shared secret** for the whole installation. It
  grants full access and is not a per-user credential.
- **Alerts and secrets live in `serverwatch.env`.** Keep it `chmod 600` and never
  commit it. `SMTP_PASSWORD` and `TELEGRAM_BOT_TOKEN` are the most sensitive
  values.
- **Health and readiness probes (`/health`, `/ready`) stay open** so systemd,
  Docker, and Nginx can check the service without credentials. They expose no
  secrets.
- **`/metrics` is opt-in** and should only be reachable from your monitoring
  network (restrict it in Nginx or bind it privately).

## Hardening recommendations

- Serve the dashboard over HTTPS and set `SERVERWATCH_COOKIE_SECURE=true`.
- Enable `SERVERWATCH_AUTH_ENABLED=true` and/or set a strong `SERVERWATCH_API_TOKEN`.
- Keep Postgres bound to `127.0.0.1` (the bundled compose file does).
- Run the backend as the unprivileged `serverwatch` user; the bundled systemd
  unit does and adds sandboxing directives.
