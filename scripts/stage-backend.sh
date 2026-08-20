#!/usr/bin/env bash
# Build and stage the Go backend binary electron-builder expects:
#   resources/bin/rss-backend      (unix)
#   resources/bin/rss-backend.exe  (windows)
#
# Usage:
#   ./scripts/stage-backend.sh <goos> <goarch>
# Examples:
#   ./scripts/stage-backend.sh darwin arm64
#   ./scripts/stage-backend.sh linux amd64
#   ./scripts/stage-backend.sh windows amd64
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/apps/desktop/resources/bin"
GOOS="${1:?goos required}"
GOARCH="${2:?goarch required}"

mkdir -p "$OUT"
rm -f "$OUT/rss-backend" "$OUT/rss-backend.exe"

if [[ "$GOOS" == "windows" ]]; then
  NAME="rss-backend.exe"
else
  NAME="rss-backend"
fi

echo "building $NAME ($GOOS/$GOARCH)…"
(
  cd "$ROOT/backend"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="-s -w" -o "$OUT/$NAME" ./cmd/desktop
)

ls -la "$OUT/$NAME"
