#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
ARCH="${2:-$(uname -m)}"
WITH_KERNELS=1
VARIANT=full
TAGS=()
KERNELS_DIR=""
shift "$(( $# >= 2 ? 2 : $# ))"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --lite) VARIANT=lite; TAGS=(-tags lite); shift ;;
    --without-kernels) WITH_KERNELS=0; shift ;;
    --kernels-dir) KERNELS_DIR="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done

case "$ARCH" in
  x86_64|amd64) GOARCH=amd64; ARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64; ARCH=arm64 ;;
  *) echo "unsupported Linux architecture: $ARCH" >&2; exit 1 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
STAGING="$DIST/duorelay-linux-$ARCH-$VERSION"
ARCHIVE="$DIST/duorelay-linux-$ARCH-$VERSION.tar.gz"
if [[ "$VARIANT" == lite ]]; then
  STAGING="$DIST/staging-linux/duorelay-lite-$VERSION-$ARCH"
  ARCHIVE="$DIST/duorelay-lite-$VERSION-$ARCH.tar.gz"
fi
KERNELS_DIR="${KERNELS_DIR:-$ROOT/kernels}"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || printf unknown)"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

rm -rf "$STAGING" "$ARCHIVE"
mkdir -p "$STAGING/kernels" "$DIST"

command -v go >/dev/null || { echo "Go is required" >&2; exit 1; }
command -v npm >/dev/null || { echo "Node.js/npm is required" >&2; exit 1; }

(cd "$ROOT/web" && npm run build)
GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -trimpath "${TAGS[@]}" \
  -ldflags "-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME" \
  -o "$STAGING/duorelay" "$ROOT/cmd/duorelay"
chmod 0755 "$STAGING/duorelay"

if [[ "$WITH_KERNELS" == 1 ]]; then
  NAMES=(sing-box mihomo)
  [[ "$VARIANT" == lite ]] && NAMES=(mihomo)
  for name in "${NAMES[@]}"; do
    if [[ ! -f "$KERNELS_DIR/$name" ]]; then
      echo "missing $KERNELS_DIR/$name; download kernels first or pass --without-kernels" >&2
      exit 1
    fi
    cp -a "$KERNELS_DIR/$name" "$STAGING/kernels/$name"
    chmod 0755 "$STAGING/kernels/$name"
  done
  if [[ "$VARIANT" == full ]]; then
    if [[ ! -f "$KERNELS_DIR/libcronet.so" ]]; then
      echo "missing $KERNELS_DIR/libcronet.so; download full kernels first or pass --without-kernels" >&2
      exit 1
    fi
    cp -a "$KERNELS_DIR/libcronet.so" "$STAGING/kernels/libcronet.so"
  fi
fi

cp -a "$ROOT/deploy" "$STAGING/deploy"
cp -a "$ROOT/README.md" "$STAGING/README.md"
[[ -f "$ROOT/LICENSE" ]] && cp -a "$ROOT/LICENSE" "$STAGING/LICENSE"
[[ -f "$ROOT/kernels.lock" ]] && cp -a "$ROOT/kernels.lock" "$STAGING/kernels.lock"
cat > "$STAGING/build-info.json" <<EOF
{"version":"$VERSION","variant":"$VARIANT","commit":"$COMMIT","build_time":"$BUILD_TIME","platform":"linux/$ARCH","kernels_included":$([[ "$WITH_KERNELS" == 1 ]] && echo true || echo false)}
EOF

PACKAGE_NAME="$(basename "$STAGING")"
PACKAGE_PARENT="$(dirname "$STAGING")"
TAR_TMP="$(mktemp "$DIST/.${PACKAGE_NAME}.tar.XXXXXX")"
trap 'rm -f "$TAR_TMP"' EXIT

EXECUTABLES=("$PACKAGE_NAME/duorelay")
if [[ "$WITH_KERNELS" == 1 ]]; then
  EXECUTABLES+=("$PACKAGE_NAME/kernels/mihomo")
  [[ "$VARIANT" == full ]] && EXECUTABLES+=("$PACKAGE_NAME/kernels/sing-box")
fi
while IFS= read -r -d '' path; do
  EXECUTABLES+=("$PACKAGE_NAME/$path")
done < <(cd "$STAGING" && find deploy -type f -name '*.sh' -print0)

(
  cd "$PACKAGE_PARENT"
  find "$PACKAGE_NAME" -type d -print0 |
    tar --null --no-recursion --mode=0755 -cf "$TAR_TMP" --files-from=-
  find "$PACKAGE_NAME" -type f \
    ! -path "$PACKAGE_NAME/duorelay" \
    ! -path "$PACKAGE_NAME/kernels/mihomo" \
    ! -path "$PACKAGE_NAME/kernels/sing-box" \
    ! -name '*.sh' -print0 |
    tar --null --no-recursion --mode=0644 -rf "$TAR_TMP" --files-from=-
  printf '%s\0' "${EXECUTABLES[@]}" |
    tar --null --no-recursion --mode=0755 -rf "$TAR_TMP" --files-from=-
)
gzip -n -c "$TAR_TMP" > "$ARCHIVE"
echo "created $ARCHIVE"
