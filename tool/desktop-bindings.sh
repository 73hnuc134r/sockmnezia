#!/usr/bin/env bash
# Regenerates the desktop UI's TypeScript bindings for the Go services (desktop/frontend/bindings) after
# changing a bound method or a type it uses. CI checks they are up to date.
set -euo pipefail
cd "$(dirname "$0")/../desktop"
WAILS="go run github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0"
command -v wails3 >/dev/null && wails3 version 2>/dev/null | grep -q 'v3.1.0' && WAILS=wails3
$WAILS generate bindings -ts -i -silent .
