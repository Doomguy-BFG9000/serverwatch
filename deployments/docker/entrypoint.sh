#!/bin/sh
# Entrypoint for the ServerWatch Community Edition container.
#
# Runs the Go backend (monitor scheduler + API) and Nginx (dashboard + reverse
# proxy) side by side. The backend binds loopback only; Nginx is the sole public
# listener. If either process exits, the container exits so the orchestrator can
# restart it.
set -eu

log() { echo "[serverwatch] $*"; }

# Start the backend, retrying briefly so a cold or slow Postgres does not leave
# the container in a crash-loop. Compose healthchecks usually make this
# unnecessary, but a standalone `docker run` against a cold database benefits.
start_backend() {
  attempt=1
  while :; do
    /usr/local/bin/serverwatch &
    backend_pid=$!
    sleep 2
    if kill -0 "$backend_pid" 2>/dev/null; then
      return 0
    fi
    if [ "$attempt" -ge 10 ]; then
      log "backend exited during startup after ${attempt} attempts"
      return 1
    fi
    log "backend exited during startup (attempt ${attempt}); retrying in 3s"
    attempt=$((attempt + 1))
    sleep 3
  done
}

start_backend || exit 1
log "backend started (pid ${backend_pid})"

nginx -g 'daemon off;' &
nginx_pid=$!
log "nginx started (pid ${nginx_pid})"

shutdown() {
  log "shutting down"
  kill -TERM "$backend_pid" 2>/dev/null || true
  kill -TERM "$nginx_pid" 2>/dev/null || true
}
trap shutdown TERM INT

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$nginx_pid" 2>/dev/null; do
  sleep 2
done

log "a child process exited; stopping container"
shutdown
wait "$backend_pid" 2>/dev/null || true
wait "$nginx_pid" 2>/dev/null || true
exit 1
