#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${DUORELAY_REPO_URL:-https://github.com/HlONGlin/DuoRelay.git}"
REF="${DUORELAY_REF:-master}"
VERSION="bootstrap"

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }

[[ "$(uname -s)" == Linux ]] || fail '此安装入口仅支持 Linux。'
[[ "$(id -u)" == 0 ]] || fail '安装系统服务需要 root，请使用 sudo bash 运行。'
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail '仅支持 Linux amd64 和 arm64。' ;;
esac

command -v systemctl >/dev/null || fail '需要使用 systemd 的系统。'
systemctl show --property=Version --value >/dev/null 2>&1 || fail 'systemd 未运行；容器请使用其他部署方式。'
if [[ -e /etc/systemd/system/duorelay.service || -e /opt/duorelay/duorelay ]]; then
  fail '检测到已有 DuoRelay 系统安装；请备份数据并参考 Linux 部署文档手动升级。'
fi

missing=()
for dependency in git curl go node npm tar gzip sha256sum; do
  command -v "$dependency" >/dev/null || missing+=("$dependency")
done
if (( ${#missing[@]} )); then
  printf '缺少依赖：%s\n' "${missing[*]}"
  if command -v apt-get >/dev/null; then
    printf '正在使用 apt-get 安装构建依赖……\n'
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y git curl golang-go nodejs npm tar gzip coreutils ca-certificates
  else
    fail '请先安装 git、curl、Go、Node.js/npm、tar、gzip 和 coreutils，再重新运行。'
  fi
fi
for dependency in git curl go node npm tar gzip sha256sum; do
  command -v "$dependency" >/dev/null || fail "依赖安装后仍找不到：$dependency"
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/duorelay-bootstrap.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
printf '正在获取 DuoRelay 源码（%s）……\n' "$REF"
git clone --depth 1 --branch "$REF" -- "$REPO_URL" "$WORK_DIR/source"
cd "$WORK_DIR/source"
printf '正在安装前端依赖并下载内核……\n'
(cd web && npm ci)
bash scripts/download-kernels.sh
printf '正在构建 Linux %s 发布包……\n' "$ARCH"
bash scripts/package-linux.sh "$VERSION" "$ARCH"
printf '正在安装并启动 systemd 服务……\n'
bash scripts/install-linux.sh --package "$WORK_DIR/source/dist/duorelay-linux-$ARCH-$VERSION"
printf '\n安装完成。请在本机访问 http://127.0.0.1:8080/ 并创建管理员。\n'
printf '远程服务器可使用 SSH 端口转发访问；服务状态：systemctl status duorelay\n'
printf '数据目录：/var/lib/duorelay；部署说明：%s\n' 'https://github.com/HlONGlin/DuoRelay/blob/master/docs/linux-deployment.md'
