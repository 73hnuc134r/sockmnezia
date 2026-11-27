#!/usr/bin/env bash
# Builds the desktop release files into desktop/dist (docs/RELEASING.md):
#   sockmnezia-desktop-<v>-windows-x64-setup.exe   per-user installer (NSIS)
#   sockmnezia-desktop-<v>-windows-x64.zip         portable
#   sockmnezia-desktop-<v>-linux-x64.deb / .rpm     GTK 4 + WebKitGTK 6 distributions
#   sockmnezia-desktop-<v>-linux-x64.tar.gz         binary, icon, .desktop file
# Needs: Go, Node, makensis (nsis), zip, curl, and GTK 4 / WebKitGTK 6 dev packages for the Linux build.
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
WINTUN_VERSION=0.14.1
WINTUN_SHA256=07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51

if [[ "$what" == windows || "$what" == all ]]; then
  # Icon, version info and manifest (DPI awareness, never elevated) go into the exe as a resource object.
  sed -e "s/@VERSION@/$v/g" -e "s/@NUMERIC@/$numeric/g" build/windows/info.json.in > "$tmp/info.json"
  $WAILS generate syso -arch amd64 -icon build/appicon.ico -manifest build/windows/sockmnezia.exe.manifest \
    -info "$tmp/info.json" -out rsrc_windows_amd64.syso
  VERSION="$v" bash "$root/tool/desktop-build.sh" windows/amd64 "$tmp/sockmnezia.exe"
  rm -f rsrc_windows_amd64.syso
  # Wintun (WireGuard LLC, signed driver DLL, redistributable) for TUN mode, checked against a pinned hash.
  curl -sSfL -o "$tmp/wintun.zip" "https://www.wintun.net/builds/wintun-$WINTUN_VERSION.zip"
  echo "$WINTUN_SHA256  $tmp/wintun.zip" | sha256sum -c --quiet
  unzip -q -j -o "$tmp/wintun.zip" wintun/bin/amd64/wintun.dll wintun/LICENSE.txt -d "$tmp/wintun"
  cp "$tmp/wintun/wintun.dll" "$tmp/wintun.dll"
  cp "$tmp/wintun/LICENSE.txt" "$tmp/wintun-LICENSE.txt"
  (cd "$tmp" && zip -q -9 "$dist/$name-windows-x64.zip" sockmnezia.exe wintun.dll wintun-LICENSE.txt)
  makensis -V2 -DVERSION="$v" -DNUMERIC="$numeric" -DEXE="$tmp/sockmnezia.exe" -DWINTUN="$tmp/wintun.dll" \
    -DWINTUNLICENSE="$tmp/wintun-LICENSE.txt" -DICON="$root/desktop/build/appicon.ico" \
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
