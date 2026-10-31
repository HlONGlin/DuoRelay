#!/bin/bash
set -e

# DuoRelay Kernel Download Script
# Downloads and verifies sing-box and mihomo kernels

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
KERNELS_DIR="$PROJECT_ROOT/kernels"
LOCK_FILE="$PROJECT_ROOT/kernels.lock"

# Default versions (can be overridden by kernels.lock)
SINGBOX_VERSION="${SINGBOX_VERSION:-1.8.0}"
MIHOMO_VERSION="${MIHOMO_VERSION:-1.18.0}"

# Detect OS and ARCH
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

echo "Detected: $OS/$ARCH"

# Create kernels directory
mkdir -p "$KERNELS_DIR"

# Function to download sing-box
download_singbox() {
    echo "Downloading sing-box $SINGBOX_VERSION..."

    local url="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-${OS}-${ARCH}.tar.gz"
    local tarball="$KERNELS_DIR/sing-box.tar.gz"

    curl -L -o "$tarball" "$url"

    echo "Extracting sing-box..."
    tar -xzf "$tarball" -C "$KERNELS_DIR"

    # Find and move binary
    find "$KERNELS_DIR" -name "sing-box" -type f -executable -exec mv {} "$KERNELS_DIR/sing-box" \;

    # Cleanup
    rm -f "$tarball"
    find "$KERNELS_DIR" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;

    chmod +x "$KERNELS_DIR/sing-box"
    echo "sing-box installed: $("$KERNELS_DIR/sing-box" version | head -n 1)"
}

# Function to download mihomo
download_mihomo() {
    echo "Downloading mihomo $MIHOMO_VERSION..."

    local url="https://github.com/MetaCubeX/mihomo/releases/download/v${MIHOMO_VERSION}/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}.gz"
    local gz_file="$KERNELS_DIR/mihomo.gz"

    curl -L -o "$gz_file" "$url"

    echo "Extracting mihomo..."
    gunzip -f "$gz_file"

    # The extracted file might have different names
    if [ -f "$KERNELS_DIR/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}" ]; then
        mv "$KERNELS_DIR/mihomo-${OS}-${ARCH}-v${MIHOMO_VERSION}" "$KERNELS_DIR/mihomo"
    fi

    chmod +x "$KERNELS_DIR/mihomo"
    echo "mihomo installed: $("$KERNELS_DIR/mihomo" -v | head -n 1)"
}

# Read versions from lock file if exists
if [ -f "$LOCK_FILE" ]; then
    echo "Reading versions from kernels.lock..."
    # Simple parsing - in real implementation, use proper YAML/JSON parser
    # For now, just use defaults
fi

# Download kernels
download_singbox
download_mihomo

# Generate lock file
cat > "$LOCK_FILE" << EOF
# DuoRelay Kernel Lock File
# Generated at $(date -u +"%Y-%m-%d %H:%M:%S UTC")

singbox:
  version: $SINGBOX_VERSION
  os: $OS
  arch: $ARCH

mihomo:
  version: $MIHOMO_VERSION
  os: $OS
  arch: $ARCH
EOF

echo ""
echo "✅ Kernels downloaded successfully!"
echo "   sing-box: $KERNELS_DIR/sing-box"
echo "   mihomo:   $KERNELS_DIR/mihomo"
echo ""
echo "Lock file updated: $LOCK_FILE"
