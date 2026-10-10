#!/usr/bin/env bash
#
# Shared helpers for ServerWatch Community Edition scripts.

# read_env_var KEY FILE
#
# Reads KEY=VALUE from an environment file without executing it. The value is
# taken literally to the end of the line (the same rule systemd's
# EnvironmentFile uses), so passwords or tokens containing spaces or shell
# metacharacters are read correctly and cannot run commands. A single pair of
# surrounding quotes is stripped if present.
read_env_var() {
  local key="$1" file="$2" line value
  line="$(grep -E "^[[:space:]]*${key}=" "${file}" | tail -n1)"
  if [[ -z "${line}" ]]; then
    return 1
  fi
  value="${line#*=}"
  value="${value%$'\r'}"
  if [[ ${#value} -ge 2 && "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
    value="${value:1:${#value}-2}"
  elif [[ ${#value} -ge 2 && "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
    value="${value:1:${#value}-2}"
  fi
  printf '%s' "${value}"
}

# find_env_file
#
# Locates the environment file in the documented order and prints its path, or
# returns non-zero if none is found.
find_env_file() {
  local repo_dir="$1" candidate
  if [[ -n "${SERVERWATCH_ENV_FILE:-}" ]]; then
    [[ -f "${SERVERWATCH_ENV_FILE}" ]] && printf '%s' "${SERVERWATCH_ENV_FILE}" && return 0
    return 1
  fi
  for candidate in /opt/serverwatch/serverwatch.env \
                   "${repo_dir}/serverwatch.env" \
                   "${repo_dir}/.env"; do
    if [[ -f "${candidate}" ]]; then
      printf '%s' "${candidate}"
      return 0
    fi
  done
  return 1
}

# db_name_from_url URL / db_user_from_url URL
#
# Extract the database name or user from a libpq URL such as
# postgres://user:pass@host:5432/dbname?sslmode=disable.
db_name_from_url() {
  local url="${1%%\?*}"
  printf '%s' "${url##*/}"
}

db_user_from_url() {
  local rest="${1#*://}"
  rest="${rest%%@*}"
  printf '%s' "${rest%%:*}"
}

# resolve_postgres_identity FILE
#
# Prints "<db> <user>" for the Postgres database to back up. It prefers the
# explicit POSTGRES_DB / POSTGRES_USER variables and otherwise derives them from
# DATABASE_URL, so the scripts work against an install whose environment file
# only sets DATABASE_URL (as the reference deployment does).
resolve_postgres_identity() {
  local file="$1" db user url
  db="$(read_env_var POSTGRES_DB "${file}" 2>/dev/null || true)"
  user="$(read_env_var POSTGRES_USER "${file}" 2>/dev/null || true)"
  url="$(read_env_var DATABASE_URL "${file}" 2>/dev/null || true)"
  if [[ -z "${db}" && -n "${url}" ]]; then
    db="$(db_name_from_url "${url}")"
  fi
  if [[ -z "${user}" && -n "${url}" ]]; then
    user="$(db_user_from_url "${url}")"
  fi
  printf '%s %s' "${db:-serverwatch}" "${user:-serverwatch}"
}
