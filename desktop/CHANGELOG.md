# Desktop changelog

Each `## <version>` section becomes the GitHub release notes for the `desktop-v<version>` tag
(`tool/changelog.sh <version> desktop`). The Android app has its own changelog (`CHANGELOG.md`). Write for people
using the app.

## 0.1.1

- New Statistics page: traffic per day (today, 7 or 30 days), sent and received, through each chain or server
  and directly (routing rules), also in TUN mode. Kept on this computer for a year; can be reset.

## 0.1.0

First desktop version (Windows 10/11, Linux with GTK 4), the same core as the Android app.

- Connect: the active chain left to right (this PC → hops → internet), Start/Stop, exit address and ping on
  click, live download/upload graphs, the local SOCKS5/HTTP address with copy.
- TUN mode: all traffic of the computer through the chain, also from apps without proxy settings; DNS over
  HTTPS through the chain, the local network keeps working. Choosing TUN installs a small system service
  (one administrator prompt; choosing another mode removes it), which keeps TUN up when sockmnezia is closed
  and starts with the computer. Nothing goes around the chain: when the proxy is stopped, internet traffic is
  blocked until you start it again or choose another mode.
- System proxy: Windows / GNOME / KDE settings point at a loopback port and stay so while the mode is chosen,
  also when the proxy is stopped or sockmnezia is closed: traffic then fails instead of going direct. Choosing
  Off puts the previous settings back.
- Servers & chains: grids with host groups, colour tags, ping and exit country; sort by name or ping, filter;
  Test / Test all without switching; double-click to connect. Server editor with fields or link/JSON; chain
  editor with drag-and-drop hops.
- Import with a preview from Ctrl+V anywhere, the Add menu or pasted text; share servers and chains as links,
  Xray JSON or a QR code.
- Tray: start/stop, switch chain or server (recent first), system proxy mode, copy the SOCKS address.
  Closing the window can keep sockmnezia in the tray; start with the system; start the proxy on launch.
- Amnezia subscription setup (key, gateway key from the official APK, CAPTCHA, location with ping), routing
  rules with presets and a route tester, geo files, log, update check.
- Saved keys and passwords are encrypted for your account (Windows DPAPI, the Linux keyring).
