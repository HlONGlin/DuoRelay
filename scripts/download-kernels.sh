#!/bin/bash
set -e

# DuoRelay Kernel Download Script
# Downloads and verifies sing-box and mihomo kernels

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
KERNELS_DIR="$PROJECT_ROOT/kernels"
LOCK_FILE="$PROJECT_ROOT/kernels.lock"
VARIANT=mihomo
while [[ $# -gt 0 ]]; do
  case "$1" in
    --variant) VARIANT="${2:?missing variant}"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done
case "$VARIANT" in full|lite|mihomo|singbox) ;; *) echo "unknown variant: $VARIANT" >&2; exit 1 ;; esac

# Default versions (can be overridden by kernels.lock)
SINGBOX_VERSION="${SINGBOX_VERSION:-1.14.4}"
MIHOMO_VERSION="${MIHOMO_VERSION:-1.19.33}"

# Detect OS and ARCH
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Handle Windows (Git Bash/MSYS2/MinGW)
case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

echo "Detected: $OS/$ARCH"
if [[ "$OS" == linux ]]; then
    temp_kernels="$(mktemp -d)"
    trap 'rm -rf -- "$temp_kernels"' EXIT
    cd "$PROJECT_ROOT"
    go run ./deploy/docker/download-kernels.go -arch "$ARCH" -variant "$VARIANT" -out "$temp_kernels"
    mkdir -p "$KERNELS_DIR"
    cp -a "$temp_kernels/". "$KERNELS_DIR/"
    exit 0
fi

# Create kernels directory
mkdir -p "$KERNELS_DIR"

# Function to download sing-box
download_singbox() {
    echo "Downloading sing-box $SINGBOX_VERSION..."

    if [ "$OS" = "windows" ]; then
        local url="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-${OS}-${ARCH}.zip"
        local archive="$KERNELS_DIR/sing-box.zip"
        local binary="sing-box.exe"
    else
        local url="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-${OS}-${ARCH}.tar.gz"
        local archive="$KERNELS_DIR/sing-box.tar.gz"
        local binary="sing-box"
    fi

    curl --fail -L -o "$archive" "$url"

    echo "Extracting sing-box..."
    if [ "$OS" = "windows" ]; then
        unzip -q -o "$archive" -d "$KERNELS_DIR"
        # Find and move binary
        find "$KERNELS_DIR" -name "$binary" -type f -exec mv {} "$KERNELS_DIR/$binary" \;
    else
        tar -xzf "$archive" -C "$KERNELS_DIR"
        # Find and move binary
        find "$KERNELS_DIR" -name "$binary" -type f -executable -exec mv {} "$KERNELS_DIR/$binary" \;
    fi

    # Cleanup
    rm -f "$archive"
    find "$KERNELS_DIR" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;

    chmod +x "$KERNELS_DIR/$binary"
    echo "sing-box installed: $("$KERNELS_DIR/$binary" version | head -n 1)"
}

# Function to download mihomo
download_mihomo() {
    echo "Downloading mihomo $MIHOMO_VERSION..."

    if [ "$OS" = "windows" ]; then
        local url="https://github.com/MetaCubeX/mihomo/releases/download/v${MIHOMO_VERSION}/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}.zip"
        local archive="$KERNELS_DIR/mihomo.zip"
        local binary="mihomo.exe"
    else
        local url="https://github.com/MetaCubeX/mihomo/releases/download/v${MIHOMO_VERSION}/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}.gz"
        local archive="$KERNELS_DIR/mihomo.gz"
        local binary="mihomo"
    fi

    curl --fail -L -o "$archive" "$url"

    echo "Extracting mihomo..."
    if [ "$OS" = "windows" ]; then
        unzip -q -o "$archive" -d "$KERNELS_DIR"
        # The extracted file might have the pattern mihomo-{os}-{arch}.exe or mihomo.exe
        if [ -f "$KERNELS_DIR/mihomo-${OS}-${ARCH}.exe" ]; then
            mv "$KERNELS_DIR/mihomo-${OS}-${ARCH}.exe" "$KERNELS_DIR/$binary"
        elif [ ! -f "$KERNELS_DIR/$binary" ]; then
            find "$KERNELS_DIR" -name "mihomo*.exe" -type f -exec mv {} "$KERNELS_DIR/$binary" \; -quit
        fi
    else
        gunzip -f "$archive"
        # The extracted file might have different names
        if [ -f "$KERNELS_DIR/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}" ]; then
            mv "$KERNELS_DIR/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}" "$KERNELS_DIR/$binary"
        fi
    fi

    chmod +x "$KERNELS_DIR/$binary"
    echo "mihomo installed: $("$KERNELS_DIR/$binary" -v | head -n 1)"
}

lock_value() {
    awk -v section="$1" -v key="$2" '
      /^[[:alnum:]]+:/ { selected=($1 == section ":") }
      selected && $1 == key ":" { gsub(/"/, "", $2); print $2; exit }
    ' "$LOCK_FILE"
}
verify_kernel() {
    local expected actual
    expected="$(lock_value "$1" "$OS-$ARCH")"
    actual="$(sha256sum "$KERNELS_DIR/$2" | cut -d ' ' -f1)"
    [[ -n "$expected" && "$actual" == "$expected" ]] || { echo "Missing or mismatched checksum for $2" >&2; exit 1; }
}
[[ "$OS/$ARCH" == windows/amd64 ]] || { echo "No pinned kernels for $OS/$ARCH" >&2; exit 1; }
if [[ "$VARIANT" == full || "$VARIANT" == singbox ]]; then
    SINGBOX_VERSION="$(lock_value singbox version)"
    download_singbox
    verify_kernel singbox sing-box.exe
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$SCRIPT_DIR/install-cronet-windows.ps1"
fi
if [[ "$VARIANT" != singbox ]]; then
    MIHOMO_VERSION="$(lock_value mihomo version)"
    download_mihomo
    verify_kernel mihomo mihomo.exe
fi
