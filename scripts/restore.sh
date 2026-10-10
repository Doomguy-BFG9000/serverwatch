#!/usr/bin/env bash
#
# ServerWatch Community Edition — database restore.
#
# Restores a backup produced by scripts/backup.sh. This OVERWRITES the current
# database contents, so the service should be stopped first.
#
# Usage:
#   scripts/restore.sh <backup-file> [--yes]
#
# Without --yes the script prompts for confirmation. Configuration is read from
# an environment file exactly as in scripts/backup.sh.
#
# Restore procedure:
#   sudo systemctl stop serverwatch
#   scripts/restore.sh /var/backups/serverwatch/serverwatch-YYYYMMDDTHHMMSSZ.dump.gz
#   sudo systemctl start serverwatch

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/lib.sh
source "${SCRIPT_DIR}/lib.sh"

if [[ $# -lt 1 ]]; then
  echo "usage: scripts/restore.sh <backup-file> [--yes]" >&2
  exit 1
fi

backup_file="$1"
assume_yes="false"
if [[ "${2:-}" == "--yes" ]]; then
  assume_yes="true"
fi

if [[ ! -f "${backup_file}" ]]; then
  echo "error: backup file not found: ${backup_file}" >&2
  exit 1
fi

CONTAINER="${SERVERWATCH_POSTGRES_CONTAINER:-serverwatch-postgres}"

if ! env_file="$(find_env_file "${REPO_DIR}")"; then
  echo "error: no environment file found; set SERVERWATCH_ENV_FILE" >&2
  exit 1
fi

read -r POSTGRES_DB POSTGRES_USER <<<"$(resolve_postgres_identity "${env_file}")"

: "${POSTGRES_DB:?POSTGRES_DB must be set in ${env_file}}"
: "${POSTGRES_USER:?POSTGRES_USER must be set in ${env_file}}"

if ! docker inspect "${CONTAINER}" >/dev/null 2>&1; then
  echo "error: container ${CONTAINER} not found; is the stack running?" >&2
  exit 1
fi

if [[ "${assume_yes}" != "true" ]]; then
  echo "This will OVERWRITE database '${POSTGRES_DB}' in ${CONTAINER} with:"
  echo "  ${backup_file}"
  read -r -p "Continue? [y/N] " reply
  case "${reply}" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; exit 1 ;;
  esac
fi

echo "Stopping service (best effort)..."
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop serverwatch 2>/dev/null || true
fi

echo "Restoring into ${POSTGRES_DB}..."
# --clean --if-exists drops existing objects before recreating them, so the
# restore is idempotent against a database that already has data.
gunzip -c "${backup_file}" \
  | docker exec -i "${CONTAINER}" pg_restore \
      --username "${POSTGRES_USER}" \
      --dbname "${POSTGRES_DB}" \
      --clean \
      --if-exists \
      --no-owner

echo "Restore complete. Start the service with: sudo systemctl start serverwatch"
