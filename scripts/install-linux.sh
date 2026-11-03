#!/usr/bin/env bash
set -euo pipefail

PREFIX="/opt/duorelay"
DATA_DIR="/var/lib/duorelay"
SERVICE_USER="duorelay"
PACKAGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
USER_UNIT=0

usage() {
  echo "usage: sudo $0 [--package DIR] [--prefix DIR] [--data-dir DIR] [--user-mode]"
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --package) PACKAGE_DIR="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    --user-mode) USER_UNIT=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ "$USER_UNIT" == 1 ]]; then
  PREFIX="${XDG_DATA_HOME:-$HOME/.local/share}/duorelay"
  DATA_DIR="$PREFIX/data"
  mkdir -p "$PREFIX" "$DATA_DIR"
  install -m 0755 "$PACKAGE_DIR/duorelay" "$PREFIX/duorelay"
  [[ -d "$PACKAGE_DIR/web" ]] && cp -a "$PACKAGE_DIR/web" "$PREFIX/"
  [[ -d "$PACKAGE_DIR/kernels" ]] && cp -a "$PACKAGE_DIR/kernels" "$PREFIX/"
  mkdir -p "$HOME/.config/systemd/user"
  sed -e "s|@DUORELAY_PREFIX@|$PREFIX|g" -e "s|@DUORELAY_DATA@|$DATA_DIR|g" \
    "$PACKAGE_DIR/deploy/systemd/duorelay-user.service" > "$HOME/.config/systemd/user/duorelay.service"
  systemctl --user daemon-reload
  systemctl --user enable --now duorelay.service
  echo "installed user service; use systemctl --user status duorelay"
  exit 0
fi

[[ "$(id -u)" == 0 ]] || { echo "system installation requires root; use --user-mode for a desktop user service" >&2; exit 1; }
NOLOGIN="$(command -v nologin || true)"
if [[ -z "$NOLOGIN" ]]; then
  for candidate in /usr/sbin/nologin /sbin/nologin /usr/bin/nologin /bin/false; do
    if [[ -x "$candidate" ]]; then NOLOGIN="$candidate"; break; fi
  done
fi
[[ -n "$NOLOGIN" ]] || { echo "no nologin or false executable found" >&2; exit 1; }
# 部分发行版默认不为 system 用户创建同名组，显式准备服务使用的组。
getent group "$SERVICE_USER" >/dev/null || groupadd --system "$SERVICE_USER"
id "$SERVICE_USER" >/dev/null 2>&1 || useradd --system --gid "$SERVICE_USER" --home-dir "$DATA_DIR" --shell "$NOLOGIN" "$SERVICE_USER"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$DATA_DIR" "$DATA_DIR/configs"
install -d -m 0755 "$PREFIX" "$PREFIX/kernels"
install -m 0755 "$PACKAGE_DIR/duorelay" "$PREFIX/duorelay"
[[ -d "$PACKAGE_DIR/web" ]] && cp -a "$PACKAGE_DIR/web" "$PREFIX/"
[[ -d "$PACKAGE_DIR/kernels" ]] && cp -a "$PACKAGE_DIR/kernels/." "$PREFIX/kernels/"
chown -R root:root "$PREFIX"
chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
sed -e "s|@DUORELAY_PREFIX@|$PREFIX|g" -e "s|@DUORELAY_DATA@|$DATA_DIR|g" -e "s|@DUORELAY_USER@|$SERVICE_USER|g" \
  "$PACKAGE_DIR/deploy/systemd/duorelay.service" > /etc/systemd/system/duorelay.service
systemctl daemon-reload
systemctl enable --now duorelay.service
echo "installed system service; use systemctl status duorelay"
