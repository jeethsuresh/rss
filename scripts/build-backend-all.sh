#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/apps/desktop/resources/bin"
mkdir -p "$OUT"

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "building $name ($goos/$goarch)…"
  (cd "$ROOT/backend" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -o "$OUT/$name" ./cmd/desktop)
}

build darwin arm64 rss-backend-darwin-arm64
build darwin amd64 rss-backend-darwin-amd64
build linux amd64 rss-backend-linux-amd64
build windows amd64 rss-backend-windows-amd64.exe

# default local binary for current platform
cp "$OUT/rss-backend-darwin-arm64" "$OUT/rss-backend" 2>/dev/null || \
  cp "$OUT/rss-backend-linux-amd64" "$OUT/rss-backend" 2>/dev/null || true

echo "done → $OUT"
ls -la "$OUT"
