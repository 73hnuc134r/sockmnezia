# Plan: desktop app (Windows, Linux; macOS later)

Status: decided, not started (2026-11-26): owner answered §8; implementation later. Mockups: `docs/mockups/desktop.html`. Updated 2026-11-27 for TUN mode (§3a) and the Android 1.1.x server list (host groups, tags, ping pills);
open it in a browser. A new session starting the desktop app: read this file, `docs/09` (chains, sharing), D30/D31
in `docs/DECISIONS.md`, then the mockup.

## 0. Goal

The same product as the Android app on the desktop: a hardened local SOCKS5/HTTP proxy running Amnezia and own
servers as chains, on the **same Go core** (`engine`, `xrayconf`, `runner`, `links`). The desktop adds what
the platform needs: a tray icon, the system proxy, autostart, OS secret storage, and a denser
"servers grid" UI for people with many servers.

Non-goals for v1: subscriptions (owner: not wanted), sing-box, policy groups / load balancing. TUN mode
(system-wide capture) is in the plan as its own milestone (§3a, D6).

## 1. What we take from v2rayN (and what we don't)

v2rayN (2dust/v2rayN, .NET 8, Windows/Linux/macOS) is the reference desktop client. Read from its README and
wiki (Nov 2026):

| v2rayN | We do |
|---|---|
| Server **grid**: sortable columns (delay/speed sort with failures last), drag/resize columns, layout saved, fuzzy filter | Same grid for servers and chains, fewer columns (§4.2), saved layout |
| **Ctrl+V anywhere** imports: subscription URL, share links, SIP008, WireGuard, custom configs | Ctrl+V anywhere imports links / shared chains / outbound JSON through our existing import (preview first) |
| **Pre-proxy / landing proxy** per subscription group, by alias; missing alias silently skipped at config time | Our chains are explicit and **fail-closed**: a missing hop is an error, never a skip |
| Latency + "real connection" + speed tests in batches | Test (exit IP + ping through a temporary instance, D28) per row and "Test all" with bounded concurrency |
| System proxy: clear / set / don't change / **PAC**; Windows can't use SOCKS5 as system proxy | Off / On (HTTP) / PAC (§3) and TUN (§3a); restore after crash (v2rayN doesn't reliably) |
| TUN (gvisor for Xray), admin rights, complex | TUN mode via Xray's own tun inbound, elevated service on Windows (§3a) |
| Tray: switch profile (limited count), proxy mode | Tray: start/stop, switch chain/server (recent 8 + "More…"), system proxy mode, copy address |
| Routing: ordered rules, import/export JSON with `enabled`/`remarks`; DNS separate | We already have this model (D-routing); same editor as Android |
| Settings page full of options that say "read the docs first" | Keep settings few; defaults safe; advanced in one collapsed section |
| Update check had a MITM issue (6.10–7.23.2) | Updates over HTTPS only, verify `SHA256SUMS` signed by the release key before installing |
| Close = hide to tray (Windows) / quit (others), configurable | Same, configurable, first-close hint |

## 2. Architecture

```
┌──────────── desktop app (one process) ─────────────┐
│  UI (webview)  ⇄  bindings  ⇄  core/desktop (Go)   │
│                                   │                 │
│            engine · xrayconf · runner · links       │  ← unchanged, shared with Android/CLI
│                                   │                 │
│  tray · system proxy · autostart · secret store     │  ← core/desktop/platform_{windows,linux}.go
└─────────────────────────────────────────────────────┘
```

**Recommendation: Wails (Go backend + web UI in the OS webview).** The core is Go, so the engine runs
in-process with no FFI, the bindings are generated from Go methods (we already expose JSON-in/JSON-out in
`mobile`, reusable almost 1:1), Wails has tray and native dialogs, and the app is one ~40 MB binary (most of it
Xray). Windows uses WebView2 (preinstalled on 10/11), Linux WebKitGTK.

Alternatives considered:
- **Compose Multiplatform Desktop**: would reuse the Android screens, but needs the JVM (+~80 MB) and a
  JNI/c-shared bridge to Go; the desktop wants a different layout (grid, side panel) anyway.
- **Fyne / Gio (pure Go UI)**: no webview dependency, but weaker widgets (tables, text editing) and look.
- **Tauri + Go sidecar**: two processes and IPC for no gain over Wails.

UI strings: a desktop copy of the string table generated from `tool/strings.py` (JSON for the web UI), so
English and Russian stay in one place.

## 3. System proxy and authentication (needs a decision)

Our inbounds always require a username and password (security policy, PLAN §4). Operating-system proxy
settings **cannot carry credentials**: Windows (WinINET) and GNOME/KDE store only host:port, and Windows can't
use SOCKS5 there at all. Browsers prompt for HTTP proxy credentials (407), many other apps just fail.

Options:
- **A. Separate system-proxy HTTP inbound without auth, loopback only, only while the mode is on**
  (recommended). The authenticated SOCKS/HTTP ports stay for apps configured by hand. Risk: any local process
  of any user on the machine can use that port while it is on; the UI says so, and it is off by default.
- **B. Keep auth everywhere**: browsers ask once per session; apps that don't handle 407 can't use it.
- **C. PAC file only** (served on loopback): same credential problem as B for the proxy it points to.

Whatever is chosen lives in `xrayconf` with a test (CLAUDE.md rule). Restore: before changing the system proxy
the app saves the previous settings to the state dir and restores them on stop, quit, and on the next start
after a crash.

## 3a. TUN mode (owner's addition, 2026-11-26)

A third connection mode next to Off / System proxy / PAC: **TUN** captures all traffic of the computer, for
apps without proxy support. Same approach as Android VPN mode (`docs/11`): Xray's own `tun` inbound in the
same instance (no tun2socks, no second process), the same chains and fail-closed outbounds.

- **Windows:** Xray's TUN uses Wintun (driver DLL shipped with the app) and needs administrator rights.
  `autoSystemRoutingTable` adds the routes; `autoOutboundsInterface: "auto"` binds Xray's own sockets to the
  physical interface so they don't loop back; `autoSystemWfpBlockLeak` uses WFP to block traffic and DNS that
  would bypass the tunnel. Elevation: a small Windows service (installed once by the installer) runs the
  engine as SYSTEM; the UI talks to it over a local named pipe whose ACL admits only the installing user, so
  the UI itself never runs elevated.
- **Linux:** needs `CAP_NET_ADMIN`: the package installs a helper with that capability (setcap) or uses a
  polkit action; `autoSystemRoutingTable` for routes, `autoSystemDnsToGateway` (systemd-resolved) for DNS
  where available, otherwise the UI says how DNS is handled.
- **DNS, LAN, apps:** the same settings as Android VPN mode (configurable DNS, configurable LAN bypass);
  per-process selection on Windows later (Xray routing can match process names).
- **Fail-closed:** while TUN mode runs, a broken chain blocks traffic instead of leaking; stopping the app
  removes routes and WFP filters, and the next start cleans up after a crash (like the system proxy restore).
- Security: the tun inbound is allowed by `xrayconf.Audit` only in TUN mode (shared with Android, D31).

## 4. UI

### 4.1 Main window (`Connect`)
Left navigation: **Connect · Servers & chains · Routing · Logs · Settings**. Connect shows what runs: the active
chain as a horizontal stepper (phone→… becomes "This PC → hop → hop → Internet"), the big Start/Stop, the
connection mode (Off / System proxy / PAC / TUN), exit address and ping (click to test), live up/down and a 60 s traffic
sparkline, and the local addresses with copy buttons (`socks5://user:pass@127.0.0.1:port`).

### 4.2 Servers & chains
Two tabs, each a grid. Servers: **Name, Protocol, Address, Transport, Security, Ping, Exit** (+ selected
marker); chains: **Name, Hops, Ping, Exit, Status**. Click a row to see/edit it in the right panel (the same
endpoint editor as Android, fields or link/JSON); double-click = connect through it. Toolbar: Add ▾ (new server,
import links, from file, scan QR from screen), Test all, Share, Delete; filter box. Ctrl+V anywhere imports.
Chain editor in the panel with drag-and-drop hops.

### 4.3 Tray
Icon shows state (stopped / running / error). Menu: Start/Stop · current chain ▸ (recent 8 + "More…") · System
proxy ▸ Off / System / PAC / TUN · Copy SOCKS address · Open sockmnezia · Quit.

### 4.4 Status bar (bottom of the window)
`● Running · Work (3 hops) · SOCKS 127.0.0.1:41377 · System proxy: On · ↓ 1.2 MB/s ↑ 84 KB/s`.

## 5. Platform layer (`core/desktop`)

| | Windows | Linux |
|---|---|---|
| System proxy | WinINET registry + `InternetSetOption` refresh | GNOME `gsettings org.gnome.system.proxy`, KDE `kwriteconfig` + D-Bus reload; others: show env lines |
| Autostart | `HKCU\…\Run` | `~/.config/autostart/*.desktop` |
| Secrets at rest | DPAPI (`CryptProtectData`, user scope) as the `SecretBox` | Secret Service (libsecret) key; fallback 0600 file with a warning |
| Single instance | named mutex | lock file + D-Bus/loopback activation |
| Packages | NSIS installer + portable zip | AppImage + .deb (+ Flatpak later) |

The `engine.SecretBox` interface already exists (Android Keystore, CLI PlainBox).

## 6. Security notes
Same policy as Android (`xrayconf.Audit`). Desktop specifics: the listen address stays loopback unless the user
shares on LAN; the state dir is per user with 0700/ACL; logs are redacted as in `mobile`; updates verified
(§1); the system-proxy port (if option A) is never on a LAN address.

## 7. Milestones
| # | Content |
|---|---|
| D1 | `core/desktop` bindings (port of `mobile`), Wails skeleton, Connect page, start/stop, Windows + Linux builds in CI |
| D2 | Servers & chains grids, editor panel, import/share, Test / Test all |
| D3 | Tray, system proxy (with restore), autostart, single instance |
| D4 | Secret store (DPAPI / libsecret), routing editor, logs, settings |
| D5 | Installers, signed update check, release workflow, docs |
| D6 | TUN mode: Windows service + Wintun + WFP leak block, Linux capability helper, DNS/LAN settings |

## 8. Owner's decisions (2026-11-26)
1. UI technology: **Wails** (Go core in-process, OS webview).
2. System proxy auth: **option A** — a separate unauthenticated HTTP port on 127.0.0.1, open only while the
   system-proxy mode is on, with the warning shown in the mockup; the SOCKS/HTTP ports keep their password.
3. **macOS later**, not in v1 (Windows and Linux first).
4. **No subscriptions.**

## 9. Later
Per-process selection in TUN mode on Windows, QR from screen on Linux Wayland
(portal), per-app routing on Windows (process name rules exist in Xray routing).
