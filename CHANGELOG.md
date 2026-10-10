# Changelog

All notable changes to ServerWatch Community Edition are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-10-10

First Community Edition release: a lightweight, self-hosted monitoring platform
for servers, websites, APIs, SSL certificates, cron jobs, and infrastructure
health.

### Added

- **Monitoring** - HTTP/HTTPS, TCP port, ping/ICMP, DNS, SSL certificate expiry,
  multi-port, and cron heartbeat checks, each with configurable interval and
  timeout.
- **Monitor management** - groups, maintenance mode, favorites, search, status
  filtering, sorting, and pagination, plus bulk actions (group assignment,
  enable/disable, favorite).
- **Dashboard** - live status summary, per-monitor history, ranged analytics with
  aggregated chart data, and status shortcuts.
- **Alerting** - plain-text email alerts (SMTP) and Telegram alerts, delivered on
  DOWN, recovery, and SSL-expiry transitions. Channels are independent: a
  failure on one does not block the other.
- **Authentication** - optional single-admin account authentication with session
  cookies and an optional shared API access token for automation. Health and
  readiness probes stay open.
- **Security hardening** - strict security headers (CSP, X-Frame-Options,
  nosniff, referrer and cross-origin policies), per-client API rate limiting, an
  audit log of mutating calls, and an opt-in Prometheus /metrics endpoint.
- **Operations** - a data retention job that bounds monitor_results growth while
  keeping the newest result per monitor, scripts/backup.sh and
  scripts/restore.sh, readiness/health probes, and a systemd unit plus Nginx
  reverse-proxy template under deployments/.
- **Release engineering** - a Makefile (build, test, check, release), a VERSION
  file, build metadata injected into the binary and dashboard and exposed via
  GET /health and the dashboard header, scripts/release.sh to produce a
  versioned tarball with checksums, and CI/release GitHub Actions workflows.

[Unreleased]: https://github.com/serverwatch/serverwatch/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/serverwatch/serverwatch/releases/tag/v0.1.0
