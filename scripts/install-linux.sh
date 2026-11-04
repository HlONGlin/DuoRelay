#!/usr/bin/env bash
set -euo pipefail

PREFIX="/opt/duorelay"
DATA_DIR="/var/lib/duorelay"
SERVICE_USER="duorelay"
PACKAGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
USER_UNIT=0
TRANSPARENT=0
BACKUP=0
PREFIX_SET=0
DATA_SET=0

usage() {
  echo "usage: sudo $0 [--package DIR] [--prefix DIR] [--data-dir DIR] [--transparent | --user-mode] [--backup]"
}

# Rename within the destination filesystem, including when the old ELF is running.
install_atomic() {
  local source="$1" target="$2" staging
  staging="$(mktemp "${target}.install-XXXXXX")"
  if ! install -m 0755 "$source" "$staging" || ! mv -fT "$staging" "$target"; then
    rm -f "$staging"
    return 1
  fi
}

install_kernels() {
  [[ -d "$PACKAGE_DIR/kernels" ]] || return 0
  mkdir -p "$PREFIX/kernels"
  local source
  while IFS= read -r -d '' source; do
    install_atomic "$source" "$PREFIX/kernels/$(basename "$source")"
  done < <(find "$PACKAGE_DIR/kernels" -maxdepth 1 -type f -print0)
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --package) PACKAGE_DIR="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; PREFIX_SET=1; shift 2 ;;
    --data-dir) DATA_DIR="$2"; DATA_SET=1; shift 2 ;;
    --user-mode) USER_UNIT=1; shift ;;
    --transparent) TRANSPARENT=1; shift ;;
    --backup) BACKUP=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

[[ "$USER_UNIT:$TRANSPARENT" != 1:1 ]] || { echo "--transparent requires a system service" >&2; exit 1; }
if [[ "$USER_UNIT" == 1 ]]; then
  [[ "$PREFIX_SET" == 1 ]] || PREFIX="${XDG_DATA_HOME:-$HOME/.local/share}/duorelay"
  [[ "$DATA_SET" == 1 ]] || DATA_DIR="$PREFIX/data"
fi
for directory in "$PREFIX" "$DATA_DIR"; do
  [[ "$directory" == /* && "$directory" != / && "$directory" != *[\&\|\%\$\`\\\"\'[:space:]]* ]] || { echo "installation paths must be absolute without whitespace or shell/systemd metacharacters" >&2; exit 1; }
done
if [[ "$BACKUP" == 1 && -f "$DATA_DIR/duorelay.db" ]]; then
  mkdir -p "$DATA_DIR/backups"
  "$PREFIX/duorelay" --db "$DATA_DIR/duorelay.db" --backup "$DATA_DIR/backups/$(date -u +%Y%m%dT%H%M%SZ)"
fi

if [[ "$USER_UNIT" == 1 ]]; then
  mkdir -p "$PREFIX" "$DATA_DIR"
  install_atomic "$PACKAGE_DIR/duorelay" "$PREFIX/duorelay"
  [[ -d "$PACKAGE_DIR/web" ]] && cp -a "$PACKAGE_DIR/web" "$PREFIX/"
  install_kernels
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
install_atomic "$PACKAGE_DIR/duorelay" "$PREFIX/duorelay"
[[ -d "$PACKAGE_DIR/web" ]] && cp -a "$PACKAGE_DIR/web" "$PREFIX/"
install_kernels
chown -R root:root "$PREFIX"
if [[ "$TRANSPARENT" == 1 ]]; then SERVICE_USER=root; fi
chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
sed -e "s|@DUORELAY_PREFIX@|$PREFIX|g" -e "s|@DUORELAY_DATA@|$DATA_DIR|g" -e "s|@DUORELAY_USER@|$SERVICE_USER|g" \
  "$PACKAGE_DIR/deploy/systemd/duorelay.service" > /etc/systemd/system/duorelay.service
mkdir -p /etc/systemd/system/duorelay.service.d
if [[ "$TRANSPARENT" == 1 ]]; then
  sed -e "s|@DUORELAY_PREFIX@|$PREFIX|g" "$PACKAGE_DIR/deploy/systemd/duorelay-transparent.conf" > /etc/systemd/system/duorelay.service.d/transparent.conf
else
  rm -f /etc/systemd/system/duorelay.service.d/transparent.conf
fi
if [[ "$DATA_DIR" != /var/lib/duorelay ]]; then
  printf '[Service]\nStateDirectory=\nEnvironment=STATE_DIRECTORY=%s\n' "$DATA_DIR" > /etc/systemd/system/duorelay.service.d/custom-data.conf
else
  rm -f /etc/systemd/system/duorelay.service.d/custom-data.conf
fi
if [[ "$BACKUP" == 1 || -f /etc/systemd/system/duorelay-backup.service ]]; then
  sed -e "s|@DUORELAY_PREFIX@|$PREFIX|g" -e "s|@DUORELAY_DATA@|$DATA_DIR|g" -e "s|@DUORELAY_USER@|$SERVICE_USER|g" "$PACKAGE_DIR/deploy/systemd/duorelay-backup.service" > /etc/systemd/system/duorelay-backup.service
  if [[ "$DATA_DIR" != /var/lib/duorelay ]]; then sed -i '/^StateDirectory=/d; /^StateDirectoryMode=/d' /etc/systemd/system/duorelay-backup.service; fi
  install -m 0644 "$PACKAGE_DIR/deploy/systemd/duorelay-backup.timer" /etc/systemd/system/duorelay-backup.timer
fi
systemctl daemon-reload
systemctl enable --now duorelay.service
systemctl restart duorelay.service
if [[ "$BACKUP" == 1 ]]; then systemctl enable --now duorelay-backup.timer; fi
echo "installed system service; use systemctl status duorelay"
