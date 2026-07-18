#!/usr/bin/env bash
set -euo pipefail

SSH_ALIAS="${SSH_ALIAS:-tencent-sg}"
API_SERVICE="${API_SERVICE:-new-api}"
WEB_SERVICE="${WEB_SERVICE:-nginx}"
API_PORT="${API_PORT:-3000}"
PUBLIC_HOST="${PUBLIC_HOST:-}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-}"

log() {
  printf '[qapi-up] %s\n' "$*"
}

remote() {
  ssh "$SSH_ALIAS" "$@"
}

log "target: ${SSH_ALIAS}"
log "services: ${API_SERVICE}, ${WEB_SERVICE}"

log "enabling services on boot"
remote "set -eu
systemctl enable '${API_SERVICE}' '${WEB_SERVICE}' >/dev/null
systemctl start '${API_SERVICE}' '${WEB_SERVICE}'"

log "checking service state"
remote "set -eu
systemctl is-enabled '${API_SERVICE}' '${WEB_SERVICE}'
systemctl is-active '${API_SERVICE}' '${WEB_SERVICE}'
systemctl show '${API_SERVICE}' '${WEB_SERVICE}' \
  -p Id -p ActiveState -p SubState -p Result -p NRestarts -p MainPID --no-pager"

log "checking local API health on server"
remote "set -eu
curl -fsS -m 8 -o /dev/null -w 'local_api_status=%{http_code} time=%{time_total}\n' \
  'http://127.0.0.1:${API_PORT}/api/status'
curl -fsS -m 8 -o /dev/null -w 'local_home_status=%{http_code} time=%{time_total}\n' \
  'http://127.0.0.1:${API_PORT}/'"

if [ -z "$PUBLIC_BASE_URL" ]; then
  if [ -z "$PUBLIC_HOST" ]; then
    PUBLIC_HOST="$(remote "curl -4 -fsS -m 5 https://api.ipify.org || true")"
  fi
  if [ -n "$PUBLIC_HOST" ]; then
    PUBLIC_BASE_URL="http://${PUBLIC_HOST}:${API_PORT}"
  fi
fi

if [ -n "$PUBLIC_BASE_URL" ]; then
  log "checking public API health: ${PUBLIC_BASE_URL}"
  curl -fsS -m 12 -o /dev/null -w 'public_api_status=%{http_code} time=%{time_total}\n' \
    "${PUBLIC_BASE_URL}/api/status"
  curl -fsS -m 12 -o /dev/null -w 'public_home_status=%{http_code} time=%{time_total}\n' \
    "${PUBLIC_BASE_URL}/"
else
  log "skip public check: set PUBLIC_HOST or PUBLIC_BASE_URL to enable it"
fi

log "memory snapshot"
remote "free -h
systemctl show '${API_SERVICE}' -p MemoryCurrent -p MemorySwapCurrent -p MemoryHigh -p MemoryMax -p MemorySwapMax --no-pager"

log "done"
