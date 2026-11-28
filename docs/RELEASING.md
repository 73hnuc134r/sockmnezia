# Releasing

| Workflow | Trigger | Output |
|---|---|---|
| `tests.yml` | push to `main`, pull requests | Go vet + tests (incl. loopback Xray tests), desktop CLI builds, AAR + Android lint/unit tests/debug APK |
| `secrets.yml` | every push and pull request | gitleaks scan of the whole history |
| `release.yml` | pushing a `v*` tag, or "Run workflow" with a version | Android: signed APKs + SHA256SUMS on a GitHub release |
| `release-desktop.yml` | pushing a `desktop-v*` tag, or "Run workflow" with a version | desktop: Windows installer and zip, Linux .deb/.rpm/.tar.gz + SHA256SUMS |
| `security.yml` | weekly, dependency changes | govulncheck (core, desktop), npm audit, Xray release/advisory watch (opens issues) |
| `release-notes.yml` | manual | re-syncs release notes from CHANGELOG.md / desktop/CHANGELOG.md |

The Android and desktop apps are released **separately** from this one repository (D33): Android tags `v1.2.3`
with `CHANGELOG.md`, desktop tags `desktop-v0.1.0` with `desktop/CHANGELOG.md`. Each app's update check sees
only its own tags. A desktop-only change needs no Android release and vice versa; a core change that matters
to both is released in both. Desktop releases are not marked "latest".

## Versions

- The version comes from the git tag (`tool/version.sh`): `v1.2.0` → `1.2.0`. Builds between tags are
  `1.2.0-5-g1a2b3c4` and sort after `1.2.0`. Local builds are `0.1.0-dev` unless `-PversionName=` is passed.
- A tag with a suffix (`v1.3.0-beta.1`) is published as a **pre-release**. 
- Before tagging, add a `## 1.2.0` section to `CHANGELOG.md`. It becomes the release notes.

## One-time setup

### 1. Create an upload keystore

Android only installs an update if it is signed with the **same key** as the installed app, so create one
key and keep it forever. Back up the `.jks` file and its passwords (password manager). If they are lost,
everyone has to uninstall to get new versions.

Run this **outside the repository** (keystores are git-ignored and gitleaks would flag them, but don't risk it):

```bash
keytool -genkeypair -v -keystore ~/keys/sockmnezia-upload.jks -keyalg RSA -keysize 4096 -validity 10000 -alias sockmnezia
```

`keytool` comes with any JDK. It asks for a keystore password, a name (anything) and a key password
(Enter reuses the keystore password).

### 2. Add repository secrets

Repo **Settings → Secrets and variables → Actions → New repository secret**:

| Secret | Value |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | the `.jks` file, base64-encoded |
| `ANDROID_KEYSTORE_PASSWORD` | keystore password |
| `ANDROID_KEY_ALIAS` | `sockmnezia` (the `-alias` above) |
| `ANDROID_KEY_PASSWORD` | key password |

With the GitHub CLI:

```bash
gh secret set ANDROID_KEYSTORE_BASE64 --body "$(base64 -w0 ~/keys/sockmnezia-upload.jks)"
gh secret set ANDROID_KEYSTORE_PASSWORD
gh secret set ANDROID_KEY_ALIAS --body sockmnezia
gh secret set ANDROID_KEY_PASSWORD
```

PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("$HOME\keys\sockmnezia-upload.jks")) | Set-Clipboard`.

Without these secrets the workflow still runs but signs with a throwaway debug key and warns. That is
fine for testing, not for releases: debug-signed APKs cannot update each other.

### 3. Local release builds (optional)

Create `android/key.properties` (git-ignored):

```properties
storeFile=/absolute/path/to/sockmnezia-upload.jks
storePassword=…
keyAlias=sockmnezia
keyPassword=…
```

## Publishing

```bash
git tag v0.1.0 && git push origin v0.1.0
```

or Actions → **Release APK** → Run workflow → version `0.1.0`. The workflow runs the tests and the secret scan,
builds a universal APK plus one per ABI (`arm64-v8a`, `armeabi-v7a`, `x86_64`) and publishes them with
`SHA256SUMS`. Most phones want `arm64-v8a`.

CI caches: Go modules/build cache (`setup-go`), Gradle (`setup-gradle`), the NDK, and the built Go core AAR,
keyed on the core's non-test Go sources, `go.mod`/`go.sum`, `tool/build-aar.sh` and the NDK version. When only
app code changed, the AAR and the NDK are reused and the gomobile build is skipped. All APKs come from one
Gradle pass (`-PsplitAbi` with a universal APK).

## Local builds

```bash
export ANDROID_HOME=~/Android/Sdk            # with an NDK installed (sdkmanager "ndk;29.0.14206865")
bash tool/build-aar.sh                       # Go core -> android/app/libs/sockmnezia-core.aar
cd android && ./gradlew assembleDebug        # or assembleRelease with android/key.properties
```

## Security scanning

- `security.yml` runs every Monday and when `go.mod`/`go.sum`/`package-lock.json` change. govulncheck only reports
  vulnerabilities in code that is actually called. A failing scheduled run emails the owner.
- The Xray watch opens an issue when Xray-core publishes a newer release than the pinned commit, or a security
  advisory that isn't listed in `.github/xray-advisories-reviewed.txt`. Update Xray-core in `core/` and `desktop/`,
  run the tests, release both apps; list reviewed advisories in that file.
- Dependabot (`.github/dependabot.yml`) opens grouped weekly update PRs (Go, npm, Gradle; Actions monthly).
  Xray-core is left to the watch. Turn on **Settings → Code security → Dependabot alerts** and **Dependabot
  security updates** so known vulnerabilities also raise alerts and fix PRs.

## Desktop releases

Add a `## 0.1.0` section to `desktop/CHANGELOG.md`, then Actions → **Release desktop** → Run workflow → version
`0.1.0` (or push the tag `desktop-v0.1.0`). It runs the core and desktop tests and the secret scan, builds:

| File | For |
|---|---|
| `sockmnezia-desktop-<v>-windows-x64-setup.exe` | Windows 10/11: per-user install (no administrator rights), Start menu entry, uninstaller |
| `sockmnezia-desktop-<v>-windows-x64.zip` | Windows, portable |
| `sockmnezia-desktop-<v>-linux-x64.deb` / `.rpm` | Ubuntu 24.04+, Debian 13+, Fedora 39+ (GTK 4, WebKitGTK 6) |
| `sockmnezia-desktop-<v>-linux-x64.tar.gz` | other distributions with GTK 4 and WebKitGTK 6 |

and publishes them with `SHA256SUMS`. The binaries are not code-signed: Windows SmartScreen warns on the first
run ("More info → Run anyway") until the app has a signing certificate.

Local build (`tool/desktop-package.sh` needs Go, Node, `nsis`, `zip` and `libgtk-4-dev libwebkitgtk-6.0-dev`):

```bash
bash tool/desktop-build.sh                            # this machine -> desktop/build/bin/sockmnezia
VERSION=0.1.0 bash tool/desktop-package.sh all        # release files -> desktop/dist
```

**Don't push changes to `.github/workflows/` while a release runs.** The publish step creates the tag
with the workflow's `GITHUB_TOKEN`, and GitHub refuses (`HTTP 403: Resource not accessible by
integration`) a token without the `workflows` permission when the tagged commit's workflow files differ
from `main`. App changes are fine: the release tags the commit it built. If it happened, run the
release again from `main` (its workflows match), or push the tag yourself.

## F-Droid (later)

Build with `--dart-define=NUX_UPDATER=false`: the GitHub update check, the beta switch and the changelog
download disappear (the bundled changelog is still shown). See docs/PLAN.md §13.
