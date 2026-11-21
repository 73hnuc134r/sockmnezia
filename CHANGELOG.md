# Changelog

Each `## <version>` section becomes the GitHub release notes for that tag (tool/changelog.sh) and is
shown in the app's update card. Write for people using the app, not for developers.

## 0.3.0

- Home screen: modes as a list with the own server picker inside; a bottom bar with connection state, the full route (every server address), exit address, ping and the start/stop button.
- Quick Settings tile and launcher shortcut to toggle the proxy.
- Notification shows live speed and total traffic, without time, addresses or ports.
- Amnezia screen shows the device ID listed in Amnezia's device list; the socks5:// link is copied from Settings.

## 0.1.0

- First version: a local SOCKS5/HTTP proxy for Android that runs your Amnezia Premium subscription's VLESS config, optionally exiting through your own server.
- Password-protected proxy on a random port (both editable), UDP off by default, private addresses always blocked.
- Routing rules with geosite/geoip, presets and a rule tester; background health checks; update check.
