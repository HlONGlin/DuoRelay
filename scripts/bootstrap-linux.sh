#!/usr/bin/env bash
set -Eeuo pipefail

REPO_URL="${DUORELAY_REPO_URL:-https://github.com/HlONGlin/DuoRelay.git}"
REF="${DUORELAY_REF:-master}"
VERSION="bootstrap"
VARIANT="${DUORELAY_VARIANT:-full}"
STAGE="检查环境"

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
trap 'status=$?; printf "错误：%s失败（退出码 %s），未完成安装。\n" "$STAGE" "$status" >&2; exit "$status"' ERR
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
if [[ -e /etc/systemd/system/duorelay.service || -L /etc/systemd/system/duorelay.service || -e /opt/duorelay/duorelay ]] ||
   systemctl cat duorelay.service >/dev/null 2>&1; then
  fail '检测到已有 DuoRelay 系统安装；请备份数据并参考 Linux 部署文档手动升级。'
fi

version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | sed -n '1p')" == "$2" ]]; }
command -v getconf >/dev/null || fail '缺少 getconf；请先安装发行版的 glibc 工具。'
libc_version="$(getconf GNU_LIBC_VERSION 2>/dev/null || true)"
[[ "$libc_version" == 'glibc '* ]] || fail '此入口需要 glibc，不支持 Alpine/musl 或 OpenWrt；请使用 Docker 部署。'
[[ "$libc_version" =~ ^glibc\ ([0-9]+)\.([0-9]+)$ ]] || fail '无法识别 glibc 版本。'
(( BASH_REMATCH[1] > 2 || (BASH_REMATCH[1] == 2 && BASH_REMATCH[2] >= 28) )) ||
  fail '此入口需要 glibc 2.28+；请升级系统或使用 Docker 部署。'
# os-release 也定义 VERSION；在子 shell 中读取，避免覆盖构建版本等变量。
distro_data="$(
  ID=unknown
  ID_LIKE=""
  PRETTY_NAME=Linux
  [[ ! -r /etc/os-release ]] || source /etc/os-release
  printf '%s\n%s\n%s\n' "$ID" "$ID_LIKE" "$PRETTY_NAME"
)"
mapfile -t distro_fields <<< "$distro_data"
ID="${distro_fields[0]}"
ID_LIKE="${distro_fields[1]}"
PRETTY_NAME="${distro_fields[2]}"
printf '系统：%s，架构：%s，变体：%s\n' "$PRETTY_NAME" "$ARCH" "$VARIANT"

STAGE="准备基础依赖"
missing=()
for dependency in git curl tar gzip sha256sum sort awk sed install useradd groupadd getent find; do
  command -v "$dependency" >/dev/null || missing+=("$dependency")
done
# 官方 Node.js 二进制需要 libstdc++；一些 minimal 镜像不预装它。
case "$(ldconfig -p 2>/dev/null || true)" in
  *libstdc++.so.6*) ;;
  *) missing+=(libstdc++) ;;
esac
ca_available=0
for ca_bundle in /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt /etc/ssl/ca-bundle.pem; do
  [[ ! -s "$ca_bundle" ]] || ca_available=1
done
[[ "$ca_available" == 1 ]] || missing+=(ca-certificates)
if (( ${#missing[@]} )); then
  printf '缺少依赖：%s\n' "${missing[*]}"
  manager=""
  for distro in "$ID" $ID_LIKE; do
    case "$distro" in
      debian|ubuntu|raspbian|linuxmint) candidates=(apt-get) ;;
      fedora|rhel|centos|rocky|almalinux|ol|amzn) candidates=(dnf yum) ;;
      arch|manjaro) candidates=(pacman) ;;
      opensuse*|sles|suse) candidates=(zypper) ;;
      *) candidates=() ;;
    esac
    for candidate in "${candidates[@]}"; do
      if command -v "$candidate" >/dev/null; then manager="$candidate"; break; fi
    done
    [[ -z "$manager" ]] || break
  done
  [[ -n "$manager" ]] || fail "未找到适用于 $PRETTY_NAME 的包管理器；请手动安装上述基础依赖后重试。"
  packages=(git ca-certificates tar gzip coreutils findutils sed gawk)
  # Fedora/RHEL 的 curl-minimal 已提供 curl，避免强装 curl 造成软件包冲突。
  command -v curl >/dev/null || packages+=(curl)
  printf '正在使用 %s 安装基础依赖……\n' "$manager"
  case "$manager" in
    apt-get)
      apt-get update
      DEBIAN_FRONTEND=noninteractive apt-get install -y "${packages[@]}" passwd libc-bin libstdc++6 ;;
    dnf|yum) "$manager" install -y "${packages[@]}" shadow-utils glibc-common libstdc++ ;;
    # 不单独刷新数据库，避免 Arch 部分升级；如本地数据库过旧，先手动 pacman -Syu。
    pacman) pacman -S --needed --noconfirm "${packages[@]}" shadow glibc gcc-libs ;;
    zypper) zypper --non-interactive install "${packages[@]}" shadow glibc libstdc++6 ;;
  esac
fi
for dependency in git curl tar gzip sha256sum sort awk sed install useradd groupadd getent find; do
  command -v "$dependency" >/dev/null || fail "依赖安装后仍找不到：$dependency"
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/duorelay-bootstrap.XXXXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
# 某些 VPS 将 /tmp 挂载为 noexec。提前报错，可用 TMPDIR 选择允许执行的目录。
printf '#!/bin/sh\nexit 0\n' > "$WORK_DIR/exec-check"
chmod 0700 "$WORK_DIR/exec-check"
"$WORK_DIR/exec-check" || fail '临时目录不允许执行程序；请设置 TMPDIR 为允许执行的目录后重试。'
STAGE="获取源码"
printf '正在获取 DuoRelay 源码（%s）……\n' "$REF"
git clone --depth 1 --branch "$REF" -- "$REPO_URL" "$WORK_DIR/source"
cd "$WORK_DIR/source"
required_go="$(awk '$1 == "go" { print $2; exit }' go.mod)"
[[ "$required_go" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail '无法识别 go.mod 中的 Go 版本。'

download() {
  curl --fail --show-error --location --retry 3 --connect-timeout 20 --max-time 900 \
    --proto '=https' --proto-redir '=https' --tlsv1.2 --output "$2" "$1"
}
verify_archive() {
  [[ "$2" =~ ^[a-fA-F0-9]{64}$ ]] || fail '官方下载摘要格式无效，拒绝解压。'
  printf '%s  %s\n' "$2" "$1" | sha256sum --check --status || fail '工具链 SHA-256 校验失败，拒绝解压。'
}
STAGE="准备 Go 工具链"
mkdir -p "$WORK_DIR/toolchains"
installed_go="$(GOTOOLCHAIN=local go version 2>/dev/null | awk '{sub(/^go/, "", $3); print $3}' || true)"
if [[ ! "$installed_go" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || ! version_ge "$installed_go" "$required_go"; then
  go_archive="go$required_go.linux-$ARCH.tar.gz"
  printf '正在下载临时 Go %s（不替换系统 Go）……\n' "$required_go"
  download "https://dl.google.com/go/$go_archive.sha256" "$WORK_DIR/go.sha256"
  go_hash="$(cat "$WORK_DIR/go.sha256")"
  download "https://dl.google.com/go/$go_archive" "$WORK_DIR/$go_archive"
  verify_archive "$WORK_DIR/$go_archive" "$go_hash"
  tar -xzf "$WORK_DIR/$go_archive" -C "$WORK_DIR/toolchains"
  export GOROOT="$WORK_DIR/toolchains/go"
  export PATH="$GOROOT/bin:$PATH"
fi
export GOTOOLCHAIN=local
go version
go list -m >/dev/null || fail "Go 无法加载模块（需要 $required_go）；请检查工具链及模块下载网络。"

STAGE="准备 Node.js 工具链"
node_major="$(node -p 'Number(process.versions.node.split(".")[0])' 2>/dev/null || true)"
if [[ ! "$node_major" =~ ^[0-9]+$ ]] || [[ "$node_major" != 18 && "$node_major" -lt 20 ]] || ! npm --version >/dev/null 2>&1; then
  NODE_ARCH="$ARCH"
  [[ "$ARCH" != amd64 ]] || NODE_ARCH=x64
  printf '正在下载临时 Node.js 22 LTS（不替换系统 Node.js/npm）……\n'
  download 'https://nodejs.org/dist/latest-v22.x/SHASUMS256.txt' "$WORK_DIR/node.sha256"
  node_entry="$(awk -v arch="$NODE_ARCH" '$2 ~ "^node-v22\\.[0-9]+\\.[0-9]+-linux-" arch "\\.tar\\.gz$" {print $1, $2}' "$WORK_DIR/node.sha256")"
  [[ "$node_entry" =~ ^([a-fA-F0-9]{64})\ (node-v22\.[0-9]+\.[0-9]+-linux-$NODE_ARCH\.tar\.gz)$ ]] || fail 'Node.js 官方清单中没有唯一的匹配归档。'
  node_hash="${BASH_REMATCH[1]}"
  node_archive="${BASH_REMATCH[2]}"
  node_version="${node_archive%-linux-*}"
  download "https://nodejs.org/dist/${node_version#node-}/$node_archive" "$WORK_DIR/$node_archive"
  verify_archive "$WORK_DIR/$node_archive" "$node_hash"
  tar -xzf "$WORK_DIR/$node_archive" -C "$WORK_DIR/toolchains"
  export PATH="$WORK_DIR/toolchains/${node_archive%.tar.gz}/bin:$PATH"
fi
node --version
npm --version
STAGE="安装前端依赖和下载内核"
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
STAGE="构建发布包"
bash scripts/package-linux.sh "${package_args[@]}"
printf '正在安装并启动 systemd 服务……\n'
STAGE="安装并启动 systemd 服务"
bash scripts/install-linux.sh --package "$package_dir"
printf '\n安装完成。请在本机访问 http://127.0.0.1:8080/ 并创建管理员。\n'
printf '远程服务器可使用 SSH 端口转发访问；服务状态：systemctl status duorelay\n'
printf '数据目录：/var/lib/duorelay；部署说明：%s\n' 'https://github.com/HlONGlin/DuoRelay/blob/master/docs/linux-deployment.md'
