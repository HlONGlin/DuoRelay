#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${DUORELAY_BASE_URL:-http://127.0.0.1:8080}"
TEST_URL="${DUORELAY_TEST_URL:-}"
COOKIE_ARGS=()
if [[ -n "${DUORELAY_AUTH_COOKIE:-}" ]]; then COOKIE_ARGS=(-H "Cookie: ${DUORELAY_AUTH_COOKIE}"); fi

pass() { printf 'PASS  %s\n' "$1"; }
warn() { printf 'UNVERIFIED  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1" >&2; exit 1; }
api() { curl -fsS "${COOKIE_ARGS[@]}" "$@"; }

command -v curl >/dev/null || fail "curl is required"
command -v jq >/dev/null || fail "jq is required for settings round trips"
api "$BASE_URL/api/config" >/dev/null
pass "API reachable at $BASE_URL"
api "$BASE_URL/api/kernel/status" >/dev/null
pass "kernel status endpoint"

DISTRO="unknown"
if [[ -r /etc/os-release ]]; then . /etc/os-release; DISTRO="${ID:-unknown}:${VERSION_ID:-unknown}"; fi
printf 'Linux acceptance matrix (host=%s)\n' "$DISTRO"
printf 'mode\tresult\tevidence\n'

if [[ -r /dev/net/tun && -w /dev/net/tun ]]; then TUN_READY=1; else TUN_READY=0; fi
if command -v nft >/dev/null; then FIREWALL=nft; elif command -v iptables >/dev/null; then FIREWALL=iptables; else FIREWALL=none; fi
if command -v ip >/dev/null; then IP_READY=1; else IP_READY=0; fi

printf 'redirect\t'
if [[ "$FIREWALL" != none && "$IP_READY" == 1 && -n "$TEST_URL" && "${DUORELAY_ACCEPTANCE_REAL:-0}" == 1 ]]; then
  printf 'RUNNING\t'
else
  printf 'UNVERIFIED\t'
  warn "redirect requires DUORELAY_ACCEPTANCE_REAL=1, DUORELAY_TEST_URL, iproute2 and nftables/iptables"
fi
printf 'firewall=%s\n' "$FIREWALL"

printf 'tproxy\t'
if [[ "$FIREWALL" != none && "$IP_READY" == 1 && -n "$TEST_URL" && "${DUORELAY_ACCEPTANCE_REAL:-0}" == 1 ]]; then
  printf 'RUNNING\t'
else
  printf 'UNVERIFIED\t'
  warn "tproxy requires DUORELAY_ACCEPTANCE_REAL=1, DUORELAY_TEST_URL, policy routing and TPROXY kernel support"
fi
printf 'policy-routing=%s\n' "$IP_READY"

printf 'tun\t'
if [[ "$TUN_READY" == 1 && "$IP_READY" == 1 && -n "$TEST_URL" && "${DUORELAY_ACCEPTANCE_REAL:-0}" == 1 ]]; then
  printf 'RUNNING\t'
else
  printf 'UNVERIFIED\t'
  warn "tun requires DUORELAY_ACCEPTANCE_REAL=1, DUORELAY_TEST_URL, writable /dev/net/tun and CAP_NET_ADMIN"
fi
printf '/dev/net/tun=%s\n' "$TUN_READY"

if [[ "${DUORELAY_ACCEPTANCE_REAL:-0}" != 1 || -z "$TEST_URL" ]]; then
  cat <<'EOF'
Real mode execution was not claimed. Set DUORELAY_ACCEPTANCE_REAL=1 and DUORELAY_TEST_URL to run each mode.
For every mode the real path must be: snapshot nft/iptables + ip rule/route + interfaces + TUN devices -> PUT settings -> POST /api/config/apply -> request TEST_URL -> POST /api/kernel/stop -> compare snapshots.
The script intentionally does not call an HTTP raw-config apply endpoint.
EOF
  exit 0
fi

WORK="${TMPDIR:-/tmp}/duorelay-acceptance-linux"
mkdir -p "$WORK"
snapshot() {
  ip -details rule show >"$1.rule"
  ip -details route show table all >"$1.route"
  if [[ "$FIREWALL" == nft ]]; then nft list ruleset >"$1.firewall"; else iptables-save >"$1.firewall"; fi
  find /sys/class/net -maxdepth 1 -mindepth 1 -type l -printf '%f\n' | sort >"$1.interfaces"
  if [[ -e /dev/net/tun ]]; then ip -details link show type tun >"$1.tun" 2>&1 || true; else : >"$1.tun"; fi
}

for mode in redirect tproxy tun; do
  before="$WORK/$mode-before"
  after="$WORK/$mode-after"
  snapshot "$before"
  settings="$(api "$BASE_URL/api/config")"
  case "$mode" in
    tun) settings="$(jq '.data.transparent_mode="tun" | .data.enable_tun=true' <<<"$settings")" ;;
    *) settings="$(jq --arg mode "$mode" '.data.transparent_mode=$mode | .data.enable_tun=false' <<<"$settings")" ;;
  esac
  api -X PUT "$BASE_URL/api/config" -H 'Content-Type: application/json' --data "$(jq -c '.data' <<<"$settings")" >/dev/null
  api -X POST "$BASE_URL/api/config/apply" -H 'Content-Type: application/json' --data '{"kernel":"sing-box"}' >/dev/null
  curl -fsS "$TEST_URL" >/dev/null
  api -X POST "$BASE_URL/api/kernel/stop" >/dev/null
  snapshot "$after"
  if ! cmp -s "$before.rule" "$after.rule" || ! cmp -s "$before.route" "$after.route" || ! cmp -s "$before.firewall" "$after.firewall" || ! cmp -s "$before.interfaces" "$after.interfaces" || ! cmp -s "$before.tun" "$after.tun"; then
    fail "$mode did not restore firewall, policy-routing, interface, or TUN state"
  fi
  pass "$mode: start -> real request -> stop -> complete network-state restoration"
done
