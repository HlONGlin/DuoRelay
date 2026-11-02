#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${TMPDIR:-/tmp}/duorelay-smoke"
PORT="${DUORELAY_SMOKE_PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"

rm -rf "$WORK"
mkdir -p "$WORK"
/root/go/bin/go build -trimpath -o "$WORK/duorelay" "$ROOT/cmd/duorelay"

start_panel() {
  "$WORK/duorelay" -listen "127.0.0.1:${PORT}" -db "$WORK/duorelay.db" -config-dir "$WORK/configs" -kernels "$ROOT/kernels" >"$WORK/panel.log" 2>&1 &
  PANEL_PID=$!
  for _ in $(seq 1 100); do
    if curl -fsS "$BASE/api/auth/status" >"$WORK/status.json"; then return; fi
    sleep 0.1
  done
  cat "$WORK/panel.log"
  exit 1
}

stop_panel() {
  if [[ -n "${PANEL_PID:-}" ]]; then
    kill "$PANEL_PID" 2>/dev/null || true
    wait "$PANEL_PID" 2>/dev/null || true
    PANEL_PID=""
  fi
}

trap stop_panel EXIT
start_panel

curl -fsS -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' \
  --data '{"username":"admin","password":"password123"}' >"$WORK/register.json"
curl -fsS -c "$WORK/cookies" -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  --data '{"username":"admin","password":"password123"}' >"$WORK/login.json"

curl -fsS -b "$WORK/cookies" -X POST "$BASE/api/config/apply" -H 'Content-Type: application/json' \
  --data '{"kernel":"sing-box","config":"{\"log\":{\"level\":\"info\"},\"inbounds\":[{\"type\":\"mixed\",\"tag\":\"mixed-in\",\"listen\":\"127.0.0.1\",\"listen_port\":18082}],\"outbounds\":[{\"type\":\"direct\",\"tag\":\"direct\"}],\"route\":{\"rules\":[{\"outbound\":\"direct\"}]}}"}' >"$WORK/applied.json"
curl -fsS -b "$WORK/cookies" "$BASE/api/kernel/status" >"$WORK/running.json"
grep -q '"running":true' "$WORK/running.json"

stop_panel
start_panel
curl -fsS -b "$WORK/cookies" "$BASE/api/kernel/status" >"$WORK/resumed.json"
grep -q '"running":true' "$WORK/resumed.json"

echo "Linux smoke passed: configuration applied and resumed after panel restart"
