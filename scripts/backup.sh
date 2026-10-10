#!/usr/bin/env bash
#
# ServerWatch Community Edition — database backup.
#
# Dumps the Postgres database (monitors, check history, alert state, and user
# accounts) to a timestamped, gzip-compressed custom-format archive. Custom
# format is restorable with pg_restore and compresses well.
#
# Usage:
#   scripts/backup.sh [BACKUP_DIR]
#
# Configuration is read from an environment file, in this order:
#   1. $SERVERWATCH_ENV_FILE
#   2. /opt/serverwatch/serverwatch.env
#   3. ./serverwatch.env
#   4. ./.env
# It uses POSTGRES_DB, POSTGRES_USER, and the `serverwatch-postgres` container.
#
# Schedule it with cron or a systemd timer for regular backups.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BACKUP_DIR="${1:-${SERVERWATCH_BACKUP_DIR:-/var/backups/serverwatch}}"
CONTAINER="${SERVERWATCH_POSTGRES_CONTAINER:-serverwatch-postgres}"
KEEP="${SERVERWATCH_BACKUP_KEEP:-7}"

env_file="${SERVERWATCH_ENV_FILE:-}"
if [[ -z "${env_file}" ]]; then
  for candidate in /opt/serverwatch/serverwatch.env "${REPO_DIR}/serverwatch.env" "${REPO_DIR}/.env"; do
    if [[ -f "${candidate}" ]]; then
      env_file="${candidate}"
      break
    fi
  done
fi

if [[ -z "${env_file}" || ! -f "${env_file}" ]]; then
  echo "error: no environment file found; set SERVERWATCH_ENV_FILE" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
# The env file is trusted (it is the same one systemd loads); disable sourcing
# warnings about unset optional variables.
source "${env_file}"
set +a

: "${POSTGRES_DB:?POSTGRES_DB must be set in ${env_file}}"
: "${POSTGRES_USER:?POSTGRES_USER must be set in ${env_file}}"

if ! docker inspect "${CONTAINER}" >/dev/null 2>&1; then
  echo "error: container ${CONTAINER} not found; is the stack running?" >&2
  exit 1
fi

mkdir -p "${BACKUP_DIR}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="${BACKUP_DIR}/serverwatch-${timestamp}.dump.gz"

echo "Backing up ${POSTGRES_DB} from ${CONTAINER} to ${file}"
docker exec "${CONTAINER}" pg_dump \
  --username "${POSTGRES_USER}" \
  --dbname "${POSTGRES_DB}" \
  --format custom \
  --no-owner \
  | gzip -c > "${file}"

if [[ ! -s "${file}" ]]; then
  echo "error: backup file is empty; removing" >&2
  rm -f "${file}"
  exit 1
fi

echo "Backup complete: ${file} ($(du -h "${file}" | cut -f1))"

# Prune old backups, keeping the most recent ${KEEP}.
if [[ "${KEEP}" =~ ^[0-9]+$ ]] && (( KEEP > 0 )); then
  mapfile -t old < <(ls -1t "${BACKUP_DIR}"/serverwatch-*.dump.gz 2>/dev/null | tail -n "+$((KEEP + 1))")
  for stale in "${old[@]:-}"; do
    [[ -n "${stale}" ]] || continue
    echo "Removing old backup: ${stale}"
    rm -f "${stale}"
  done
fi
