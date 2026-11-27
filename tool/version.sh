#!/usr/bin/env bash
# Prints the app version for CI builds (needs full git history: checkout with fetch-depth: 0).
#   tool/version.sh            Android: tags v1.2.0
#   tool/version.sh desktop    desktop: tags desktop-v0.1.0 (the apps are versioned separately, D33)
#   on a release tag v1.2.0       -> 1.2.0
#   5 commits after it            -> 1.2.0-5-g1a2b3c4   (git describe style; sorts after 1.2.0)
#   no release tagged yet         -> 0.0.0-<commit count>-g<sha>
set -euo pipefail
prefix=v
[[ "${1:-}" == desktop ]] && prefix=desktop-v
if tag=$(git describe --tags --match "${prefix}[0-9]*" 2>/dev/null); then
  echo "${tag#"$prefix"}"
else
  echo "0.0.0-$(git rev-list --count HEAD)-g$(git rev-parse --short=7 HEAD)"
fi
