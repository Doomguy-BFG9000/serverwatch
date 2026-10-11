# AGENTS.md

Guidance for AI agents and contributors working in this repository. See
[DEVELOPMENT.md](docs/DEVELOPMENT.md) for the human-facing setup guide.

## What this project is

ServerWatch Community Edition is a self-hosted **uptime monitoring** platform:
a Go backend performs network checks (HTTP/HTTPS, TCP, ping/ICMP, DNS, SSL
expiry, multi-port), stores results in PostgreSQL, alerts via SMTP and Telegram,
and serves a React dashboard. There is **no server-side agent or host-metrics
support yet** — `agent/` is a placeholder (`docs/AGENT_INSTALLATION.md`). Do not
document cron heartbeats, CPU/RAM/disk metrics, or an agent protocol as shipped
features; they are on the roadmap.

## Build, test, and run

Requires Go (see `go.mod`), Node (see the `engines` field in
`web/package.json`; Vite 8 needs `^20.19.0 || >=22.12.0`), and pnpm 11.

```bash
make check        # vet + Go tests + web lint/bundle build — run before a PR
make test         # Go (server + agent) tests and web lint/typecheck
make build        # backend binary -> bin/serverwatch (injects version/commit)
make web-build    # dashboard -> web/dist
make release VERSION=v0.1.0   # versioned tarball + SHA256SUMS in dist/
```

The reference deployment lives at `/opt/serverwatch` on the monitoring host,
running the backend via systemd as the unprivileged `serverwatch` user with
Postgres in the `serverwatch-postgres` Docker container. Restarting that service
needs interactive sudo; ask the user instead of attempting it.

The dev workspace and the deployment share one tree: the checkout here is the
same directory as `/opt/serverwatch` (a local edit is visible on the host and
vice versa). Nginx serves the dashboard straight from `web/dist`, so rebuilding
the web bundle (`make web-build`) takes effect immediately with no restart; the
Go binary at `bin/serverwatch` only changes for clients after a service restart.

## Conventions

- **Migrations**: add the next numbered file to **both**
  `database/migrations/` and `server/internal/storage/migrations/` with identical
  contents. `server/internal/storage/migrations_test.go` fails the build if the
  copies diverge.
- **Config**: every setting is an environment variable documented in
  `.env.example` and `docs/CONFIGURATION.md`. Keep the two in sync, and never
  add a variable that no code reads.
- **Scripts**: `scripts/*.sh` must parse the env file with the helpers in
  `scripts/lib.sh` (`read_env_var`, `resolve_postgres_identity`) rather than
  `source`-ing it; sourcing executes shell metacharacters in values such as
  `SMTP_PASSWORD`. Bundle any new script in `scripts/release.sh`.
- **Tests**: exercise real code paths; avoid mocks unless a dependency cannot be
  reached. New HTTP/config behavior should have a Go test.
- **Go**: run `gofmt`; CI fails on unformatted files.
- **Web**: `pnpm lint` (oxlint) must stay warning-free.
- **Web styling**: all colors, radii, shadows and font sizes resolve to the CSS
  variables in `web/src/index.css`; do not hard-code hex values in `App.css`.
  Light and dark themes are selected by `data-theme` on `<html>` (toggled in
  `App.tsx`, defaulting to `prefers-color-scheme`). The CSP is `style-src
  'self'`, so no inline styles and no remote fonts — Inter is self-hosted via
  `@fontsource-variable/inter`.

## Containers

- `Dockerfile` builds one self-contained image: Nginx serves `web/dist` and
  proxies `/api`, `/health`, `/ready`, `/status`, `/metrics` to the Go backend on
  loopback. `deployments/docker/nginx.conf` is the in-container config and
  `deployments/docker/entrypoint.sh` runs both processes.
- `deployments/docker/docker-compose.yml` is the full stack (app + Postgres).
  The top-level `docker-compose.yml` stays Postgres-only for native installs.
- `.github/workflows/docker.yml` publishes multi-arch (amd64/arm64) images to
  GHCR on `v*` tags (`:X.Y.Z`, `:X.Y`, `:latest`) and `:edge` from `main`.
- `.dockerignore` must keep `.env`/`serverwatch.env` out of the build context so
  secrets are never baked into an image.

## GitHub publishing

- Canonical repository is `github.com/smartphpfreelancers-sys/serverwatch`,
  published image `ghcr.io/smartphpfreelancers-sys/serverwatch`.
- The repository `/opt/serverwatch` inside this sandbox is mounted over
  `fuse.sshfs` (from `serverwatch-target`), so git operations that read many
  objects (`git status`, `pack-objects`, a full `git push`) are extremely slow.
  **Push from the remote host, not from the mount**: hold the token in a
  0600 temp file on the remote and use a `credential.helper` that cats it, e.g.
  `ssh serverwatch-target 'cd /opt/serverwatch; git -c credential.helper="!f(){ echo username=x-access-token; echo password=$(cat /tmp/.swtok); }; f" push origin main'`.
- `GITHUB_TOKEN` holds a classic PAT with `repo`, `workflow`, `write:packages`.
  Access a secret's value at runtime with
  `curl -H "X-Expose-Secrets: encrypted" .../api/settings/secrets/<NAME>`
  (never print it). A secret registered *after* a conversation starts is not
  injected into that conversation's env — fetch it via the API instead.
- Release flow: push `main` → the `docker.yml` workflow publishes `:edge`; push a
  `vX.Y.Z` tag → it publishes `:X.Y.Z`, `:X.Y`, `:latest`. Then create the GitHub
  Release (the tag alone does not create one).
- Funding is configured in `.github/FUNDING.yml` via the PayPal link for
  `kirupa.business@gmail.com`.

## Building and deploying the backend

- The Go toolchain lives on the remote at `/usr/local/go/bin` (not on PATH).
  The sandbox has neither Go nor Docker, so **build on the remote** (native FS):
  `ssh serverwatch-target 'export PATH=$PATH:/usr/local/go/bin; cd /opt/serverwatch/server && go build -ldflags "$(make -n build | sed ...)" -o ../bin/serverwatch.new ./cmd/serverwatch'`
  — or use the remote's `make build`.
- The Go binary embeds the public status page (`server/internal/http/status_template.go`
  → `/status.css`), so a **status-page styling change needs a binary rebuild**;
  the dashboard bundle (`web/dist`, git-ignored) is served by Nginx and is
  already current on the shared mount.
- Swapping a running binary: build to `bin/serverwatch.new`, `cp -a` the old one
  to `bin/serverwatch.bak.<ts>`, then `mv -f` the new one into place (writing
  over the live file fails with `ETXTBSY`). The running process keeps the old
  image until it is restarted.
- Restart needs root: `ssh serverwatch-target 'sudo systemctl restart serverwatch'`
  (the `serverwatch` user's sudo requires a password, so the agent cannot do it).
  Verify afterwards with `curl -s localhost:8080/health` — it should report the
  new commit.

## Docs map

`README.md` (overview, configuration) • `docs/INSTALLATION.md` •
`docs/CONFIGURATION.md` • `docs/ARCHITECTURE.md` • `docs/BACKUP_RESTORE.md` •
`docs/TROUBLESHOOTING.md` • `docs/DEVELOPMENT.md` • `docs/ROADMAP.md` •
`docs/AGENT_INSTALLATION.md` • `CHANGELOG.md`.
