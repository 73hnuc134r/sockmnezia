# Changelog

Each `## <version>` section becomes the GitHub release notes for that tag (tool/changelog.sh) and is
shown in the app's update card. Write for people using the app, not for developers.

## 1.1.4

- "Test all" first clears every remembered ping, so old results never sit next to new ones.

## 1.1.3

- Chain editor: "Add hop" opens the same server list as Switch (host groups, tags, ping, Test all, sort,
  swipe), with Amnezia at the top; tapping a server adds it as a hop. Unsaved edits to the chain are kept
  while you pick.

## 1.1.2

- Server list: servers are grouped by host (with the country of an IP host), and each shows small coloured
  tags for its protocol, security (no encryption stands out in red), transport and port.
- Every chain and server shows its last test as a small ping pill (green, amber or red, "failed", or untested)
  that runs a test when tapped; results are kept. "Test all" tests everything a few at a time, and the list
  sorts by name or by ping.
- Swipe a row right to test it, left to delete it (asks first).
- The same tags in the chain editor, the "Add hop" list and the chain on the Home screen.

## 1.1.1

- In-app updates: when a newer release is available, "Update" downloads the APK for this phone, checks it
  against the release's checksums and installs it (Android asks you to confirm; the first time, allow
  sockmnezia to install apps). The proxy or VPN starts again after the update if it was running.
- Chain editor: the "Add hop" list scrolls when you have many servers.
- Home screen: the connection-check block is at least four lines tall.

## 1.1.0

- VPN mode: switch Home between Proxy and VPN. In VPN mode the whole phone (or the apps you choose) goes
  through the selected chain or server, also apps without proxy settings. The proxy ports keep working
  beside it. Android's Always-on VPN is supported, and a broken hop blocks traffic instead of letting it
  around the chain.
- Settings → VPN mode: DNS (public resolvers through the chain, DNS-over-HTTPS in the app, or your own
  servers), local network (bypasses the VPN, or blocked), and apps (all, only chosen, all except chosen,
  with an app list and search).
- Home screen: the connection-check block is always at least three lines tall, so it is easy to tap.
- Home screen: the Background work card only shows while a setting needs changing (battery optimization,
  background restriction, notifications, data saver, pausing unused apps). Advice such as the phone-maker
  note or unexpected stops stays on the Background work screen.

## 1.0.4

- "Switch" opens a full screen instead of a sheet: chains and single servers, each with Test and a ⋮ menu.
  Server rows no longer list the chains they are used in.
- Server editor: add or edit a server field by field (protocol, address, port, credentials, transport,
  TLS/Reality and their settings) or as a link / Xray JSON, and switch between the two. Editing keeps the
  server in the chains that use it, and an edit that would break one of them is refused. "Import links"
  next to "Add server" takes several links or a shared chain.

## 1.0.3

- Home screen: Switch and start/stop are now two same-size round icon buttons side by side, where start/stop
  used to be (start/stop on the right).

## 1.0.2

- The button above start/stop now reads "Switch" with a left-right arrows icon.
- Sheets (chain/server switcher, sharing, adding a hop) open fully instead of half-way.

## 1.0.1

- One sheet to choose what to connect through, opened from the Home card or the new button above
  start/stop: saved chains and, in a separate list, single servers. A single server can be used directly, no
  one-hop chain needed. Each row has Test (exit address and ping through it, without switching or touching
  the running proxy) and a ⋮ menu: edit or rename, share, delete (with confirmation). "New chain" and "Add
  server" are in the same sheet, so the Servers tab is gone.
- Routing → Geo files: updating shows progress (which file, downloaded of total size, then verifying),
  also when the weekly update runs while the app is open. Only one update runs at a time.

## 1.0.0

Multihop: sockmnezia is now a chain builder. Amnezia is one possible hop, alongside your own servers.

- Chains replace the three modes: build a chain of hops (Amnezia and your own servers, in any order, up to
  5), save several and switch between them on the Home screen. Each hop is reached through the one before
  it; the last one is the exit. Your current mode and server are converted into chains automatically.
- Servers tab: servers are no longer "selected"; each shows the chains it is used in, and a server that a
  chain uses cannot be deleted until it is removed from that chain.
- Settings: "Reach the Amnezia gateway through" picks a server (or direct). When the chain has hops before
  Amnezia, the gateway request always goes through them.
- Share a saved server (link, QR code) or a whole chain (links or Xray JSON). Amnezia is shared as a
  placeholder: whoever imports the chain uses their own subscription. "Add server" also accepts several links
  or a shared chain, shows what will be imported, and reuses servers you already have.
- Fixed: "Find fastest" while connected could leave the running proxy broken until it was restarted.
- Home screen: the connection-check block is more compact (state and ping on one line, the full route below,
  wrapped instead of cut) and the start/stop button overlaps its corner.
- The exit address and ping are cleared when you switch server, location or settings, and a failed check
  shows "check failed" instead of the previous result.
- The notification title lists the hops of the active chain.

## 0.5.1

- Home screen: the start/stop button no longer gets cropped where it overlaps the connection-check block.
- Notification: title and subtitle are swapped (title now shows the exit country or your own server name
  instead of a generic mode label), and shows download speed first, then upload.

## 0.5.0

- Home screen: a "Find fastest" button opens a dialog that pings every available location (up to 3 tries
  each) and lists them sorted by the lowest ping found.
- Snackbars can now be dismissed.
- Home screen: the start/stop button now floats half over the connection-check block instead of sitting
  fully inside it.

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
