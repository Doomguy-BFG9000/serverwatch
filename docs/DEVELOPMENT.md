# Development

ServerWatch Community Edition is a Go backend (`server/`), a Go agent
(`agent/`), and a React/Vite dashboard (`web/`). Postgres stores configuration
and check history.

## Prerequisites

- Go 1.26+
- Node.js 20+ and pnpm
- Docker (for the Postgres container) or a reachable Postgres 16 instance
- `make` (optional; every target has a direct equivalent)

## First run

```bash
cp .env.example serverwatch.env     # then edit DATABASE_URL and secrets
docker compose --env-file serverwatch.env up -d postgres

make server-run                     # or: cd server && go run ./cmd/serverwatch
make web-build                      # or: cd web && pnpm install && pnpm build
```

For dashboard development, `cd web && pnpm dev` starts Vite on
`127.0.0.1:5173` and proxies `/api` to the backend on `127.0.0.1:8080`.

## Build, test, check

```bash
make build        # versioned backend binary -> bin/serverwatch
make test         # Go (server + agent) and TypeScript checks
make check        # merge gate: vet, tests, backend build, web build
```

`make test` runs `go test ./...` for each Go module plus `pnpm lint` and
`tsc -b` for the dashboard. Go tests are table-driven and use real code paths;
persistence tests exercise SQL against a scratch database.

## Database migrations

Migrations live in two mirrored directories that must stay byte-for-byte
identical:

- `database/migrations/`
- `server/internal/storage/migrations/`

The backend embeds and applies `server/internal/storage/migrations/` on
startup; `database/migrations/` is the human-facing copy used by backup and
restore tooling. A guard test fails the build if the two diverge. To add a
migration, create the next numbered file in both directories with the same
contents.

## Versioning and releases

Version lives in `VERSION`. It is injected into the backend via `-ldflags -X`
and into the dashboard via the `VITE_APP_VERSION` build variable, so `GET
/health` and the dashboard header always show the build you are running.

```bash
make release VERSION=v0.1.0     # -> dist/serverwatch-v0.1.0-linux-amd64.tar.gz + SHA256SUMS
```

Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds the same
bundle and attaches it to a GitHub release. Every push and pull request runs
`.github/workflows/ci.yml` (`make check`).

## Code style

- Go: `gofmt`, `go vet`. Keep changes small and guarded by tests.
- TypeScript: `pnpm lint` (oxlint) and `tsc -b`.
- Do not commit secrets. `.env.example` documents the variables; real values
  live in `serverwatch.env`, which is git-ignored.
