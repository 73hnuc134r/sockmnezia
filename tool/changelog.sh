#!/usr/bin/env bash
# Prints a changelog section for a version (without its heading), for release notes.
# Usage: tool/changelog.sh 2.3.3            — Android (CHANGELOG.md)
#        tool/changelog.sh 0.1.0 desktop    — desktop (desktop/CHANGELOG.md)
# Prints nothing if the version has no section.
set -euo pipefail
file="$(dirname "$0")/../CHANGELOG.md"
[[ "${2:-}" == desktop ]] && file="$(dirname "$0")/../desktop/CHANGELOG.md"
awk -v v="$1" '
  /^## / { on = ($2 == v); next }
  on
' "$file" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
