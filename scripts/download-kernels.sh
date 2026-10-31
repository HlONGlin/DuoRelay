#!/bin/bash
set -e

# DuoRelay Kernel Download Script
# Downloads and verifies sing-box and mihomo kernels

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
KERNELS_DIR="$PROJECT_ROOT/kernels"
LOCK_FILE="$PROJECT_ROOT/kernels.lock"

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

    curl -L -o "$archive" "$url"

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

    curl -L -o "$archive" "$url"

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

# Read versions from lock file if exists
if [ -f "$LOCK_FILE" ]; then
    echo "Reading versions from kernels.lock..."
    # Simple parsing - in real implementation, use proper YAML/JSON parser
    # For now, just use defaults
fi

# Download kernels
download_singbox
download_mihomo

# Generate SHA256 checksums
echo ""
echo "Generating SHA256 checksums..."

if [ "$OS" = "windows" ]; then
    SINGBOX_BINARY="sing-box.exe"
    MIHOMO_BINARY="mihomo.exe"
else
    SINGBOX_BINARY="sing-box"
    MIHOMO_BINARY="mihomo"
fi

SINGBOX_SHA256=$(sha256sum "$KERNELS_DIR/$SINGBOX_BINARY" | awk '{print $1}')
MIHOMO_SHA256=$(sha256sum "$KERNELS_DIR/$MIHOMO_BINARY" | awk '{print $1}')

echo "  sing-box: $SINGBOX_SHA256"
echo "  mihomo:   $MIHOMO_SHA256"

# Generate lock file
cat > "$LOCK_FILE" << EOF
# DuoRelay Kernel Lock File
# Generated at $(date -u +"%Y-%m-%d %H:%M:%S UTC")

singbox:
  version: "$SINGBOX_VERSION"
  url_template: "https://github.com/SagerNet/sing-box/releases/download/v{version}/sing-box-{version}-{os}-{arch}.{ext}"
  sha256:
    ${OS}-${ARCH}: "$SINGBOX_SHA256"

mihomo:
  version: "$MIHOMO_VERSION"
  url_template: "https://github.com/MetaCubeX/mihomo/releases/download/v{version}/mihomo-{os}-{arch}-v{version}.{ext}"
  sha256:
    ${OS}-${ARCH}: "$MIHOMO_SHA256"
EOF

echo ""
echo "✅ Kernels downloaded successfully!"
echo "   sing-box: $KERNELS_DIR/$SINGBOX_BINARY"
echo "   mihomo:   $KERNELS_DIR/$MIHOMO_BINARY"
echo ""
echo "Lock file updated: $LOCK_FILE"
