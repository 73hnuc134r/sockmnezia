#!/usr/bin/env bash
# Builds the desktop app: the web UI, then the Go binary with it embedded.
#   tool/desktop-build.sh [GOOS/GOARCH] [output]   (default: this machine, desktop/build/bin/sockmnezia[.exe])
# Linux builds need cgo with GTK 4 and WebKitGTK 6 (Debian/Ubuntu: libgtk-4-dev libwebkitgtk-6.0-dev);
# Windows builds need no cgo and cross-compile from anywhere.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
target="${1:-$(go env GOOS)/$(go env GOARCH)}"
goos="${target%/*}" goarch="${target#*/}"
ext=""; [[ "$goos" == windows ]] && ext=".exe"
out="${2:-$root/desktop/build/bin/sockmnezia$ext}"
version="${VERSION:-$(bash "$root/tool/version.sh" desktop)}"

(cd "$root/desktop/frontend" && { [[ -d node_modules ]] || npm ci --no-audit --no-fund; } && npm run build)

ldflags="-s -w -checklinkname=0 -X main.Version=$version"
[[ "$goos" == windows ]] && ldflags+=" -H windowsgui"
cd "$root/desktop"
GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags "$ldflags" -o "$out" .
echo "$out ($version)"
