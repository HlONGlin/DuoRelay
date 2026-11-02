#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
ARCH="${2:-$(uname -m)}"
WITH_KERNELS=1
if [[ "${3:-}" == "--without-kernels" ]]; then
  WITH_KERNELS=0
fi

case "$ARCH" in
  x86_64|amd64) GOARCH=amd64; ARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64; ARCH=arm64 ;;
  *) echo "unsupported Linux architecture: $ARCH" >&2; exit 1 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
STAGING="$DIST/duorelay-linux-$ARCH-$VERSION"
ARCHIVE="$DIST/duorelay-linux-$ARCH-$VERSION.tar.gz"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || printf unknown)"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

rm -rf "$STAGING" "$ARCHIVE"
mkdir -p "$STAGING/kernels" "$DIST"

command -v go >/dev/null || { echo "Go is required" >&2; exit 1; }
command -v npm >/dev/null || { echo "Node.js/npm is required" >&2; exit 1; }

(cd "$ROOT/web" && npm run build)
GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -trimpath \
  -ldflags "-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME" \
  -o "$STAGING/duorelay" "$ROOT/cmd/duorelay"
chmod 0755 "$STAGING/duorelay"

if [[ "$WITH_KERNELS" == 1 ]]; then
  for name in sing-box mihomo; do
    if [[ ! -f "$ROOT/kernels/$name" ]]; then
      echo "missing kernels/$name; download kernels first or pass --without-kernels" >&2
      exit 1
    fi
    cp -a "$ROOT/kernels/$name" "$STAGING/kernels/$name"
    chmod 0755 "$STAGING/kernels/$name"
  done
fi

cp -a "$ROOT/deploy" "$STAGING/deploy"
cp -a "$ROOT/README.md" "$STAGING/README.md"
[[ -f "$ROOT/LICENSE" ]] && cp -a "$ROOT/LICENSE" "$STAGING/LICENSE"
[[ -f "$ROOT/kernels.lock" ]] && cp -a "$ROOT/kernels.lock" "$STAGING/kernels.lock"
cat > "$STAGING/build-info.json" <<EOF
{"version":"$VERSION","commit":"$COMMIT","build_time":"$BUILD_TIME","platform":"linux/$ARCH","kernels_included":$([[ "$WITH_KERNELS" == 1 ]] && echo true || echo false)}
EOF

tar -C "$DIST" -czf "$ARCHIVE" "$(basename "$STAGING")"
echo "created $ARCHIVE"
