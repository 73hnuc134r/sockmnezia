# Changelog

Each `## <version>` section becomes the GitHub release notes for that tag (tool/changelog.sh) and is
shown in the app's update card. Write for people using the app, not for developers.

## Unreleased

- Chains replace the three modes: build a chain of hops (Amnezia and your own servers, in any order, up to
  5), save several and switch between them on the Home screen. Each hop is reached through the one before
  it; the last one is the exit. Your current mode and server are converted into chains automatically.
- Servers tab: servers are no longer "selected"; each shows the chains it is used in, and a server that a
  chain uses cannot be deleted until it is removed from that chain.
- Settings: "Reach the Amnezia gateway through" picks a server (or direct). When the chain has hops before
  Amnezia, the gateway request always goes through them.
- Fixed: "Find fastest" while connected could leave the running proxy broken until it was restarted.
- Home screen: the connection-check block is more compact (state and ping on one line, the full route below,
  wrapped instead of cut) and the start/stop button overlaps its corner.
- The exit address and ping are cleared when you switch server, location or settings, and a failed check
  shows "check failed" instead of the previous result.
- Home screen: a "Find fastest" button opens a dialog that pings every available location (up to 3 tries
  each) and lists them sorted by the lowest ping found.
- Snackbars can now be dismissed.
- Home screen: the start/stop button now floats half over the connection-check block instead of sitting
  fully inside it.
- Home screen: the start/stop button no longer gets cropped where it overlaps the connection-check block.
- Notification: title and subtitle are swapped (title now shows the exit country or your own server name
  instead of a generic mode label), and shows download speed first, then upload.

## 0.4.0

- Premium country picker is now sorted alphabetically.
- Route hops and the exit address show a country hint (e.g. `[US]`) when the geo database is available.
- Disabled the predictive back gesture.
- Home screen: the start/stop button is now above the connection status, route and exit address.

## 0.3.1

- Long-pressing the Quick Settings tile opens the app.
- No fade animation when switching tabs.
- Notification shows upload speed first, then download.
- Settings: the last block is now tiles.
- Update check now finds the latest GitHub releases.
- Sharper, centred app icon and a tidier notification icon.

## 0.3.0

- Home screen: modes as a list with the own server picker inside; a bottom bar with connection state, the full route (every server address), exit address, ping and the start/stop button.
- Quick Settings tile and launcher shortcut to toggle the proxy.
- Notification shows live speed and total traffic, without time, addresses or ports.
- Amnezia screen shows the device ID listed in Amnezia's device list; the socks5:// link is copied from Settings.

## 0.1.0

- First version: a local SOCKS5/HTTP proxy for Android that runs your Amnezia Premium subscription's VLESS config, optionally exiting through your own server.
- Password-protected proxy on a random port (both editable), UDP off by default, private addresses always blocked.
- Routing rules with geosite/geoip, presets and a rule tester; background health checks; update check.
