# sockmnezia desktop

The desktop app (Windows 10/11, Linux): the shared Go core (`../core`) with a web UI in the OS webview, built
with [Wails v3](https://v3.wails.io). Plan and decisions: `docs/10-desktop-plan.md`, D30, D33–D40.

| Path | What |
|---|---|
| `main.go`, `desk.go` | app setup, window, status loop, close-to-tray, tray actions |
| `core.go` | `Core` service bound to the UI: the engine's operations with typed values |
| `desktop.go` | `Desktop` service: build info, prefs, autostart, update check, file dialogs, QR codes |
| `sysproxy*.go` | OS proxy settings (Windows WinINET, Linux GSettings/KDE), saved and restored |
| `secret*.go` | state encryption (DPAPI, Secret Service keyring) |
| `tun/`, `tunmode.go` | TUN mode: the system service (`--tun-service`, `--tun-install`, `--tun-uninstall`), its pipe/socket, the Linux kill switch (D40) |
| `tray.go`, `prefs.go` | tray menu and icons; the app's own preferences (`desktop.json`) |
| `frontend/` | Svelte 5 + TypeScript UI; `bindings/` is generated, `src/lib/strings.json` comes from `tool/strings.py` |
| `build/` | app icon, Windows manifest/version info/installer, Linux `.desktop` and nfpm config |

## Build

Linux needs GTK 4 and WebKitGTK 6 (`sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev`); Windows builds need no
cgo and cross-compile from Linux.

```bash
cd desktop/frontend && npm ci && cd ../..
bash tool/desktop-build.sh                      # -> desktop/build/bin/sockmnezia
bash tool/desktop-build.sh windows/amd64        # -> desktop/build/bin/sockmnezia.exe
VERSION=0.1.0 bash tool/desktop-package.sh all  # release files -> desktop/dist (needs nsis, zip)
```

After changing a bound method or a type it returns: `bash tool/desktop-bindings.sh`. UI strings live in
`tool/strings.py` (`D` holds the desktop-only ones); run `python3 tool/strings.py` after editing.

UI development: `cd frontend && npm run dev` serves the UI on 127.0.0.1:9245 (the bound calls need the app).

Data lives in `%LOCALAPPDATA%\sockmnezia\desktop` (Windows) or `$XDG_DATA_HOME/sockmnezia` (Linux, default
`~/.local/share/sockmnezia`); `SOCKMNEZIA_DESKTOP_DIR` overrides it.
