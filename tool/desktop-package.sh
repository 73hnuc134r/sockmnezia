#!/usr/bin/env bash
# Builds the desktop release files into desktop/dist (docs/RELEASING.md):
#   sockmnezia-desktop-<v>-windows-x64-setup.exe   per-user installer (NSIS)
#   sockmnezia-desktop-<v>-windows-x64.zip         portable
#   sockmnezia-desktop-<v>-linux-x64.deb / .rpm     GTK 4 + WebKitGTK 6 distributions
#   sockmnezia-desktop-<v>-linux-x64.tar.gz         binary, icon, .desktop file
# Needs: Go, Node, makensis (nsis), zip, and GTK 4 / WebKitGTK 6 dev packages for the Linux build.
#   VERSION=0.1.0 bash tool/desktop-package.sh [windows|linux|all]
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
what="${1:-all}"
v="${VERSION:-$(bash "$root/tool/version.sh" desktop)}"
numeric="$(echo "${v%%-*}" | awk -F. '{printf "%d.%d.%d.0", $1, $2, $3}')"
dist="$root/desktop/dist"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"; rm -f "$root"/desktop/rsrc_windows_*.syso' EXIT
mkdir -p "$dist"
cd "$root/desktop"
WAILS="go run github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0"
NFPM="go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.49.0"
name="sockmnezia-desktop-$v"

if [[ "$what" == windows || "$what" == all ]]; then
  # Icon, version info and manifest (DPI awareness, never elevated) go into the exe as a resource object.
  sed -e "s/@VERSION@/$v/g" -e "s/@NUMERIC@/$numeric/g" build/windows/info.json.in > "$tmp/info.json"
  $WAILS generate syso -arch amd64 -icon build/appicon.ico -manifest build/windows/sockmnezia.exe.manifest \
    -info "$tmp/info.json" -out rsrc_windows_amd64.syso
  VERSION="$v" bash "$root/tool/desktop-build.sh" windows/amd64 "$tmp/sockmnezia.exe"
  rm -f rsrc_windows_amd64.syso
  (cd "$tmp" && zip -q -9 "$dist/$name-windows-x64.zip" sockmnezia.exe)
  makensis -V2 -DVERSION="$v" -DNUMERIC="$numeric" -DEXE="$tmp/sockmnezia.exe" -DICON="$root/desktop/build/appicon.ico" \
    -DOUTFILE="$dist/$name-windows-x64-setup.exe" build/windows/installer.nsi
fi

if [[ "$what" == linux || "$what" == all ]]; then
  VERSION="$v" bash "$root/tool/desktop-build.sh" linux/amd64 "$tmp/sockmnezia"
  mkdir -p "$tmp/$name"
  cp "$tmp/sockmnezia" build/appicon.png build/linux/sockmnezia.desktop "$tmp/$name/"
  tar -C "$tmp" -czf "$dist/$name-linux-x64.tar.gz" "$name"
  mkdir -p build/bin && cp "$tmp/sockmnezia" build/bin/sockmnezia
  for fmt in deb rpm; do
    VERSION="$v" ARCH=amd64 $NFPM package -f build/linux/nfpm.yaml -p $fmt -t "$dist/$name-linux-x64.$fmt" >/dev/null
  done
fi
ls -la "$dist"
