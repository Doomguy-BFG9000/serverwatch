#!/usr/bin/env bash
#
# Build ServerWatch Community Edition release bundles.
#
# Usage: scripts/release.sh [version]
#
# The version defaults to the contents of VERSION. The dashboard is built once,
# the backend is cross-compiled for the supported platforms, and each bundle is
# packed with everything an operator needs as
# dist/serverwatch-<version>-<os>-<arch>.tar.gz, plus a single SHA256SUMS file.
#
# Targets:
#   linux/amd64  linux/arm64  darwin/amd64  darwin/arm64  windows/amd64
#
# Set GOOS/GOARCH to build a single target (used by local testing):
#   GOOS=linux GOARCH=arm64 scripts/release.sh 0.1.0
#
# This is what the GitHub release workflow runs. It only needs Go, Node/pnpm and
# standard coreutils; it does not touch the running service or the database.
#
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

VERSION="${1:-$(cat VERSION 2>/dev/null | tr -d '[:space:]')}"
VERSION="${VERSION:-dev}"

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [ -n "${GOOS:-}" ] || [ -n "${GOARCH:-}" ]; then
  TARGETS="${GOOS:-linux}/${GOARCH:-amd64}"
else
  TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"
fi

echo "==> ServerWatch ${VERSION} (${COMMIT})"
echo "==> Targets: ${TARGETS}"

rm -rf dist
mkdir -p dist

echo "==> Dashboard bundle (built once, shared by every platform)"
( cd web && VITE_APP_VERSION="${VERSION}" pnpm install --frozen-lockfile && pnpm build )
DASHBOARD="$(mktemp -d)"
cp -R web/dist "${DASHBOARD}/dist"

for TARGET in ${TARGETS}; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  BINARY="serverwatch"
  if [ "${OS}" = "windows" ]; then
    BINARY="serverwatch.exe"
  fi
  BUNDLE="serverwatch-${VERSION}-${OS}-${ARCH}"
  STAGE="dist/${BUNDLE}"

  echo ""
  echo "==> ${OS}/${ARCH}"
  mkdir -p "${STAGE}/bin" "${STAGE}/web" "${STAGE}/deployments" \
           "${STAGE}/database" "${STAGE}/docs" "${STAGE}/scripts"

  LDFLAGS="-s -w \
    -X github.com/serverwatch/serverwatch/server/internal/version.Version=${VERSION} \
    -X github.com/serverwatch/serverwatch/server/internal/version.Commit=${COMMIT} \
    -X github.com/serverwatch/serverwatch/server/internal/version.Date=${DATE}"

  ( cd server && CGO_ENABLED=0 GOOS="${OS}" GOARCH="${ARCH}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${REPO_ROOT}/${STAGE}/bin/${BINARY}" ./cmd/serverwatch )

  cp -R "${DASHBOARD}/dist" "${STAGE}/web/dist"
  cp -R deployments/* "${STAGE}/deployments/"
  cp -R database/migrations "${STAGE}/database/"
  cp -R docs/* "${STAGE}/docs/"
  cp scripts/backup.sh scripts/restore.sh scripts/lib.sh scripts/harden-server.sh "${STAGE}/scripts/"
  cp .env.example README.md CHANGELOG.md LICENSE VERSION "${STAGE}/"

  # Runs on any host with Docker; the operator edits serverwatch.env first.
  cat > "${STAGE}/docker-compose.yml" <<'YAML'
# ServerWatch Community Edition release bundle.
# 1. cp .env.example serverwatch.env && edit it
# 2. docker compose --env-file serverwatch.env up -d
# 3. open http://localhost:8080
services:
  postgres:
    image: postgres:16-alpine
    container_name: serverwatch-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-serverwatch}
      POSTGRES_USER: ${POSTGRES_USER:-serverwatch}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in your env file}
    volumes:
      - serverwatch_postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-serverwatch} -d ${POSTGRES_DB:-serverwatch}"]
      interval: 10s
      timeout: 5s
      retries: 5

  app:
    image: ${SERVERWATCH_IMAGE:-ghcr.io/smartphpfreelancers-sys/serverwatch:latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER:-serverwatch}:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in your env file}@postgres:5432/${POSTGRES_DB:-serverwatch}?sslmode=disable
      SERVERWATCH_HTTP_ADDR: 127.0.0.1:8080
      SERVERWATCH_API_TOKEN: ${SERVERWATCH_API_TOKEN:-}
      SERVERWATCH_AUTH_ENABLED: ${SERVERWATCH_AUTH_ENABLED:-false}
      SERVERWATCH_SESSION_TTL: ${SERVERWATCH_SESSION_TTL:-720h}
      SERVERWATCH_COOKIE_SECURE: ${SERVERWATCH_COOKIE_SECURE:-false}
      SERVERWATCH_STATUS_PAGE_ENABLED: ${SERVERWATCH_STATUS_PAGE_ENABLED:-false}
      SERVERWATCH_METRICS_ENABLED: ${SERVERWATCH_METRICS_ENABLED:-false}
      SERVERWATCH_RESULT_RETENTION_DAYS: ${SERVERWATCH_RESULT_RETENTION_DAYS:-30}
      SERVERWATCH_ALERT_EMAIL_ENABLED: ${SERVERWATCH_ALERT_EMAIL_ENABLED:-false}
      SMTP_HOST: ${SMTP_HOST:-}
      SMTP_PORT: ${SMTP_PORT:-587}
      SMTP_TLS: ${SMTP_TLS:-true}
      SMTP_USERNAME: ${SMTP_USERNAME:-}
      SMTP_PASSWORD: ${SMTP_PASSWORD:-}
      SMTP_FROM: ${SMTP_FROM:-}
      SMTP_TO: ${SMTP_TO:-}
      SERVERWATCH_ALERT_TELEGRAM_ENABLED: ${SERVERWATCH_ALERT_TELEGRAM_ENABLED:-false}
      TELEGRAM_BOT_TOKEN: ${TELEGRAM_BOT_TOKEN:-}
      TELEGRAM_CHAT_ID: ${TELEGRAM_CHAT_ID:-}
      TELEGRAM_API_BASE: ${TELEGRAM_API_BASE:-}
    ports:
      - "${SERVERWATCH_HTTP_PORT:-8080}:80"

volumes:
  serverwatch_postgres_data:
YAML

  ( cd dist && tar czf "${BUNDLE}.tar.gz" "${BUNDLE}" )
  rm -rf "${STAGE}"
done

rm -rf "${DASHBOARD}"

echo ""
echo "==> Checksums"
( cd dist && sha256sum serverwatch-*.tar.gz > SHA256SUMS && cat SHA256SUMS )

echo ""
echo "Release bundles: dist/serverwatch-*.tar.gz"
echo "Checksums:       dist/SHA256SUMS"
