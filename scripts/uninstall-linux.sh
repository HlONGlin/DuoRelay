#!/usr/bin/env bash
set -euo pipefail

PURGE=0
USER_MODE=0
for arg in "$@"; do
  case "$arg" in
    --purge) PURGE=1 ;;
    --user-mode) USER_MODE=1 ;;
    -h|--help) echo "usage: $0 [--purge] [--user-mode]"; exit 0 ;;
    *) echo "unknown option: $arg" >&2; exit 1 ;;
  esac
done

if [[ "$USER_MODE" == 1 ]]; then
  systemctl --user disable --now duorelay.service 2>/dev/null || true
  rm -f "$HOME/.config/systemd/user/duorelay.service"
  systemctl --user daemon-reload 2>/dev/null || true
  [[ "$PURGE" == 1 ]] && rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/duorelay"
  exit 0
fi

[[ "$(id -u)" == 0 ]] || { echo "system uninstall requires root" >&2; exit 1; }
systemctl disable --now duorelay.service 2>/dev/null || true
rm -f /etc/systemd/system/duorelay.service
systemctl daemon-reload
if [[ "$PURGE" == 1 ]]; then
  rm -rf /opt/duorelay /var/lib/duorelay
else
  rm -rf /opt/duorelay
fi
echo "DuoRelay Linux service removed"
