#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${DUORELAY_REPO_URL:-https://github.com/HlONGlin/DuoRelay.git}"
REF="${DUORELAY_REF:-master}"
VERSION="bootstrap"
VARIANT="${DUORELAY_VARIANT:-full}"

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
usage() { printf 'usage: sudo bash %s [--full|--lite]\n' "$0"; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --full) VARIANT=full; shift ;;
    --lite) VARIANT=lite; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; fail "未知选项：$1" ;;
  esac
done
[[ "$VARIANT" == full || "$VARIANT" == lite ]] || fail 'DUORELAY_VARIANT 必须为 full 或 lite。'

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
required_go="$(awk '$1 == "go" { print $2; exit }' go.mod)"
printf '构建 %s：需要 Go %s（或可自动下载该工具链的 Go）及 Node.js 18 / 20+，推荐 Node.js 22。\n' "$VARIANT" "$required_go"
go version || fail "Go 不可用；请准备 go.mod 要求的 Go $required_go 工具链。"
go list -m >/dev/null || fail "当前 Go 无法加载此模块；请安装 Go $required_go，或配置兼容的自动工具链下载。"
node_major="$(node -p 'Number(process.versions.node.split(".")[0])')"
[[ "$node_major" == 18 || "$node_major" -ge 20 ]] || fail 'Node.js 版本过旧或不兼容；请安装 Node.js 22 后重试。'
printf '正在安装前端依赖并下载内核……\n'
(cd web && npm ci)
package_args=("$VERSION" "$ARCH")
package_dir="$WORK_DIR/source/dist/duorelay-linux-$ARCH-$VERSION"
if [[ "$VARIANT" == lite ]]; then
  go run ./deploy/docker/download-kernels.go -arch "$ARCH" -variant lite -out kernels
  package_args+=(--lite)
  package_dir="$WORK_DIR/source/dist/staging-linux/duorelay-lite-$VERSION-$ARCH"
else
  bash scripts/download-kernels.sh
fi
printf '正在构建 Linux %s 发布包……\n' "$ARCH"
bash scripts/package-linux.sh "${package_args[@]}"
printf '正在安装并启动 systemd 服务……\n'
bash scripts/install-linux.sh --package "$package_dir"
printf '\n安装完成。请在本机访问 http://127.0.0.1:8080/ 并创建管理员。\n'
printf '远程服务器可使用 SSH 端口转发访问；服务状态：systemctl status duorelay\n'
printf '数据目录：/var/lib/duorelay；部署说明：%s\n' 'https://github.com/HlONGlin/DuoRelay/blob/master/docs/linux-deployment.md'
