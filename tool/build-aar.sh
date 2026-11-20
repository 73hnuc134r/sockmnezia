#!/usr/bin/env bash
# Builds the Go core as an Android library: android/app/libs/sockmnezia-core.aar
# Needs Go (core/go.mod toolchain), ANDROID_HOME and ANDROID_NDK_HOME (or an NDK under $ANDROID_HOME/ndk).
# gomobile/gobind are installed at the version pinned in core/go.mod (tool directive).
set -euo pipefail
cd "$(dirname "$0")/../core"
: "${ANDROID_HOME:?set ANDROID_HOME}"
if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
  ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort -V | tail -1)
  export ANDROID_NDK_HOME
fi
GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"
go install golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind # versions from go.mod (tool directive)
out=../android/app/libs/sockmnezia-core.aar
mkdir -p "$(dirname "$out")"
# -checklinkname=0: a transitive Xray dependency (wlynxg/anet, Android interface listing) uses go:linkname.
gomobile bind -target android/arm64,android/arm,android/amd64 -androidapi 26 \
  -javapkg io.nuc134r.gocore -trimpath -ldflags "-s -w -buildid= -checklinkname=0" \
  -o "$out" ./mobile
rm -f "${out%.aar}-sources.jar"
ls -la "$out"
