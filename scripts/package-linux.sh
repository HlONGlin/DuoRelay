#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
ARCH="${2:-$(uname -m)}"
WITH_KERNELS=1
VARIANT=mihomo
TAGS=()
KERNELS_DIR=""
shift "$(( $# >= 2 ? 2 : $# ))"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --lite) VARIANT=lite; shift ;;
    --variant) VARIANT="${2:?missing variant}"; shift 2 ;;
    --without-kernels) WITH_KERNELS=0; shift ;;
    --kernels-dir) KERNELS_DIR="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done

case "$VARIANT" in
  full) NAMES=(sing-box mihomo) ;;
  lite|mihomo) TAGS=(-tags "$VARIANT"); NAMES=(mihomo) ;;
  singbox) TAGS=(-tags singbox); NAMES=(sing-box) ;;
  *) echo "unknown variant: $VARIANT" >&2; exit 1 ;;
esac
[[ "$VERSION" =~ ^[a-zA-Z0-9][a-zA-Z0-9._+-]*$ ]] || { echo "invalid version: $VERSION" >&2; exit 1; }

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
elif [[ "$VARIANT" == mihomo || "$VARIANT" == singbox ]]; then
  STAGING="$DIST/duorelay-$VARIANT-linux-$ARCH-$VERSION"
  ARCHIVE="$STAGING.tar.gz"
fi
KERNELS_DIR="${KERNELS_DIR:-$ROOT/kernels}"
COMMIT="${COMMIT:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || printf unknown)}"
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
  for name in "${NAMES[@]}"; do
    if [[ ! -f "$KERNELS_DIR/$name" ]]; then
      echo "missing $KERNELS_DIR/$name; download kernels first or pass --without-kernels" >&2
      exit 1
    fi
    cp -a "$KERNELS_DIR/$name" "$STAGING/kernels/$name"
    chmod 0755 "$STAGING/kernels/$name"
  done
  if [[ "$VARIANT" == full || "$VARIANT" == singbox ]]; then
    if [[ ! -f "$KERNELS_DIR/libcronet.so" ]]; then
      echo "missing $KERNELS_DIR/libcronet.so; download the selected kernels first or pass --without-kernels" >&2
      exit 1
    fi
    cp -a "$KERNELS_DIR/libcronet.so" "$STAGING/kernels/libcronet.so"
  fi
fi

mkdir -p "$STAGING/deploy"
cp -a "$ROOT/deploy/systemd" "$STAGING/deploy/systemd"
cp -a "$ROOT/README.md" "$STAGING/README.md"
[[ ! -f "$ROOT/docs/single-kernel.md" ]] || cp -a "$ROOT/docs/single-kernel.md" "$STAGING/single-kernel.md"
[[ -f "$ROOT/LICENSE" ]] && cp -a "$ROOT/LICENSE" "$STAGING/LICENSE"
awk -v variant="$VARIANT" '
  /^[[:alnum:]]+:/ { section=$1; sub(/:$/, "", section) }
  variant == "full" || section == "" ||
    ((variant == "lite" || variant == "mihomo") && section == "mihomo") ||
    (variant == "singbox" && (section == "singbox" || section == "cronet"))
' "$ROOT/kernels.lock" > "$STAGING/kernels.lock"
cat > "$STAGING/build-info.json" <<EOF
{"version":"$VERSION","variant":"$VARIANT","commit":"$COMMIT","build_time":"$BUILD_TIME","platform":"linux/$ARCH","kernels_included":$([[ "$WITH_KERNELS" == 1 ]] && echo true || echo false)}
EOF

PACKAGE_NAME="$(basename "$STAGING")"
PACKAGE_PARENT="$(dirname "$STAGING")"
TAR_TMP="$(mktemp "$DIST/.${PACKAGE_NAME}.tar.XXXXXX")"
trap 'rm -f "$TAR_TMP"' EXIT

EXECUTABLES=("$PACKAGE_NAME/duorelay")
if [[ "$WITH_KERNELS" == 1 ]]; then
  for name in "${NAMES[@]}"; do EXECUTABLES+=("$PACKAGE_NAME/kernels/$name"); done
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
