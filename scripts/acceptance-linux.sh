#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${DUORELAY_BASE_URL:-http://127.0.0.1:8080}"
CHECK_TUN=0
[[ "${1:-}" == "--tun" ]] && CHECK_TUN=1

pass() { printf 'PASS  %s\n' "$1"; }
warn() { printf 'BLOCK %s\n' "$1"; }

command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
curl -fsS "$BASE_URL/api/config" >/dev/null
pass "API reachable at $BASE_URL"
curl -fsS "$BASE_URL/api/kernel/status" >/dev/null
pass "kernel status endpoint"

if [[ -e /dev/net/tun && -r /dev/net/tun && -w /dev/net/tun ]]; then
  pass "/dev/net/tun is available"
else
  warn "/dev/net/tun is unavailable or inaccessible; TUN acceptance requires CAP_NET_ADMIN/root"
fi

if command -v systemctl >/dev/null; then
  if systemctl is-active --quiet duorelay.service; then pass "systemd service active"; else warn "duorelay.service is not active"; fi
fi

if [[ "$CHECK_TUN" == 1 ]]; then
  if [[ "$(id -u)" != 0 ]] && ! capsh --print 2>/dev/null | grep -q cap_net_admin; then
    warn "TUN start was skipped because CAP_NET_ADMIN is unavailable"
  else
    echo "TUN prerequisites are present. Start a selected node with enable_tun=true and inspect /api/kernel/status and logs."
  fi
fi
