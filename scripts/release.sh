#!/usr/bin/env bash
#
# Build a ServerWatch Community Edition release bundle.
#
# Usage: scripts/release.sh [version]
#
# The version defaults to the contents of VERSION. The script cross-compiles
# the backend and builds the dashboard with the same version string, then packs
# everything an operator needs into dist/serverwatch-<version>-<os>-<arch>.tar.gz
# plus a SHA256SUMS file.
#
# This is what the GitHub release workflow runs. It only needs Go, Node/pnpm and
# standard coreutils; it does not touch the running service or the database.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

VERSION="${1:-$(cat VERSION 2>/dev/null | tr -d '[:space:]')}"
VERSION="${VERSION:-dev}"

OS="${GOOS:-linux}"
ARCH="${GOARCH:-amd64}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BUNDLE="serverwatch-${VERSION}-${OS}-${ARCH}"

LDFLAGS="-s -w \
  -X github.com/serverwatch/serverwatch/server/internal/version.Version=${VERSION} \
  -X github.com/serverwatch/serverwatch/server/internal/version.Commit=${COMMIT} \
  -X github.com/serverwatch/serverwatch/server/internal/version.Date=${DATE}"

echo "==> Building ServerWatch ${VERSION} (${COMMIT}) for ${OS}/${ARCH}"

rm -rf dist
STAGE="dist/${BUNDLE}"
mkdir -p "${STAGE}/bin" "${STAGE}/web" "${STAGE}/deployments" \
         "${STAGE}/database" "${STAGE}/docs" "${STAGE}/scripts"

echo "==> Backend binary"
( cd server && CGO_ENABLED=0 GOOS="${OS}" GOARCH="${ARCH}" \
  go build -trimpath -ldflags "${LDFLAGS}" -o "${REPO_ROOT}/${STAGE}/bin/serverwatch" ./cmd/serverwatch )

echo "==> Dashboard bundle"
( cd web && VITE_APP_VERSION="${VERSION}" pnpm install --frozen-lockfile && pnpm build )
cp -R web/dist "${STAGE}/web/dist"

echo "==> Packaging support files"
cp -R deployments/* "${STAGE}/deployments/"
cp -R database/migrations "${STAGE}/database/"
cp -R docs/* "${STAGE}/docs/"
cp scripts/backup.sh scripts/restore.sh scripts/lib.sh "${STAGE}/scripts/"
cp .env.example README.md CHANGELOG.md LICENSE VERSION "${STAGE}/"

# Runs on any host with Docker; the operator edits serverwatch.env first.
cat > "${STAGE}/docker-compose.yml" <<'YAML'
# ServerWatch Community Edition release bundle.
# 1. cp .env.example serverwatch.env && edit it
# 2. docker compose --env-file serverwatch.env up -d postgres
# 3. ./bin/serverwatch
services:
  postgres:
    image: postgres:16-alpine
    container_name: serverwatch-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - serverwatch_postgres_data:/var/lib/postgresql/data
    ports:
      - "127.0.0.1:${POSTGRES_PORT:-5432}:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  serverwatch_postgres_data:
YAML

echo "==> Compressing"
( cd dist && tar czf "${BUNDLE}.tar.gz" "${BUNDLE}" )
( cd dist && sha256sum "${BUNDLE}.tar.gz" > SHA256SUMS )

echo ""
echo "Release bundle: dist/${BUNDLE}.tar.gz"
echo "Checksums:      dist/SHA256SUMS"
cat dist/SHA256SUMS
