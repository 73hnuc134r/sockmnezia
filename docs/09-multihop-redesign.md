# Plan: multihop config builder

Status: proposal (2026-11-25). Supersedes the three fixed modes of `PLAN.md` §5.1 once implemented.

## 0. Goal

Today the app has three hard-coded modes (`xrayconf.Mode`): **Amnezia**, **Amnezia → own**, **Own**. The app
becomes a **chain builder**: the user assembles an ordered list of hops, and Amnezia is just one kind of hop
that can sit anywhere in it (entry, intermediate or exit). The three old modes become three ready-made chains.

```
phone ─► hop 1 (entry) ─► hop 2 ─► … ─► hop N (exit) ─► internet
         each hop is dialed THROUGH the previous one (Xray sockopt.dialerProxy)
```

Unchanged principles: fail-closed (a broken hop never falls back to a shorter chain or direct), security
policy enforced in `xrayconf` (`Audit`), no personal data in the repo, gateway never contacted from tests.

Two small fixes ship first, independent of the redesign (§6): stale exit IP/ping after switching config, and a
compact connection-check block.

## 1. Concepts

| Term | Meaning |
|---|---|
| **Hop** | One element of a chain: `amnezia` (the subscription; its gateway config may itself contain an internal relay chain, treated as one hop) or `server` (a saved own server, by ID). |
| **Chain** | Named, ordered list of hops (1…`MaxHops`, 5). Several chains are saved; exactly one is **active**. |
| **Entry / exit** | First hop (dialed directly from the phone) / last hop (its IP is the exit address). |

### Rules (enforced in `xrayconf`, each with a test)

1. 1 ≤ hops ≤ 5. Empty chain → `chain: no hops` error, the proxy does not start.
2. **Amnezia at most once per chain** (one subscription, one config, one device slot). Its country is the
   global Amnezia location, as today.
3. A hop that is not the entry must be TCP-dialable: `links.CheckChainable` applies to every hop except hop 1
   (kcp / QUIC / h3 transports are allowed **only as entry**). For Amnezia as a non-entry hop, the outbound
   at the *tail* of its internal relay chain (the one without `dialerProxy`) gets the previous hop's tag.
4. The same own server may appear twice only if not adjacent (useless loop); warn in UI, allow in core.
5. Tags: Amnezia outbounds keep `amz-*`; own servers become `hop1`, `hop2`, … (by position, not ID, so the
   tag never leaks a name). `direct` / `block` unchanged. Head (default outbound) = exit hop's head.
6. Routing rules keep `proxy` = the whole chain; `direct` and `block` are unchanged.

## 2. Core changes (Go)

### `xrayconf`
- Replace `Mode` + `Spec.Amnezia`/`Spec.Own` with `Spec.Hops []Hop` where
  `Hop{Amnezia json.RawMessage; Own links.Outbound}` (exactly one set).
- `Outbounds(hops)`: build per hop, then wire `dialerProxy` hop i → head of hop i-1; reuse `amneziaOutbounds`
  and `ownOutbound` (generalised `via`). Cycle check stays.
- `Hops()` (route display) walks the same structure; now returns `[]HopInfo{Kind, Addr, Name}`.
- `BuildDialer(prefix)` for the gateway fetch takes the hops **before** the Amnezia hop (see below).
- Tests: 1/2/3/5 hops, Amnezia at each position, UDP transport rejected as non-entry, accepted as entry,
  Amnezia twice rejected, 6 hops rejected, `Audit` still passes on every shape.

### `engine` / state (version 1 → 2)
```go
type Hop   struct { Kind string `json:"kind"`; Server string `json:"server,omitempty"` } // "amnezia" | "server"
type Chain struct { ID, Name string; Hops []Hop }
State.Chains []Chain; State.ActiveChain string   // replaces Mode + ActiveOwn
```
- **Migration** on load of a v1 state: create the three preset chains from the old modes using the old
  active own server ("Amnezia", "Amnezia → <server>", "<server>"), mark the one matching `Mode` active.
  Unit test with a v1 fixture.
- API: `Chains()`, `SaveChain(c)` (validates by building), `DeleteChain(id)`, `SetActiveChain(id)`,
  `ValidateChain(c)` (for the editor's live errors). Deleting an own server used in chains → error listing the
  chains (or remove the hop; decide in UI, see §7).
- **Gateway fetch through the prefix**: `Options.FetchViaOwn` becomes "fetch the Amnezia config through the
  hops before Amnezia" (no-op when Amnezia is the entry). Same in-process dialer as today, no listening port.
- `TestChain` / `PingCountry`: use the active chain; `PingCountry` substitutes the candidate country's config
  into the Amnezia hop. A chain without Amnezia hides "find fastest".
- `NeedsRefresh` only when the active chain contains Amnezia.
- `Status`: `chain` becomes `[]{kind, name, addr, country}`; adds `chain_id`, `chain_name` and a
  **`config_rev`** counter bumped on every change that alters the outbounds (active chain switch, chain edit,
  Amnezia location/refresh, own server edit). The app uses it for §6.1.

### `mobile`, `cmd/sockmnezia`
- JSON wrappers for the chain API. CLI: `sockmnezia chain list|add|rm|use|show`.
- Remove `mode` from settings JSON (accept and ignore it for one release).

### Integration test
- `runner/integration_test.go`: 3-hop loopback chain (own → mock "amnezia" → own) and a UDP-associate
  through 3 hops; fail-closed: kill the middle upstream → requests fail, no direct leak.

## 3. App changes (Kotlin / Compose)

Mockups: see the HTML mockups published with this plan (link in the PR / chat) — Home, chain editor,
hop picker and the compact connection bar.

### Home
- **Mode card → Chain card.** Title row: chain name + dropdown to switch saved chains + "Edit".
  Body: vertical stepper `Phone → hop → hop → Internet`; each hop row shows icon (Amnezia / server), name,
  `protocol · transport` and the country flag code if known. The exit row is marked "exit".
- **Amnezia card** shown only when the active chain contains Amnezia (location picker, find fastest,
  refresh, subscription date — unchanged), with a note of its position ("hop 1 of 3").
- Background work / update cards unchanged.

### Chain editor (new screen `Routes.CHAIN_EDIT`)
- Name field; ordered hop list with drag handle (plus ↑/↓ buttons for accessibility) and remove.
- "Add hop" → bottom sheet: Amnezia (disabled if already used, or "set up first" if no key) + saved servers
  + "Add new server…" (existing import flow).
- Live validation from `ValidateChain`: per-hop error text under the offending hop (e.g. "QUIC can only be
  the first hop"). Save disabled while invalid.
- Presets on "New chain": Amnezia only / Amnezia → server / Server only / Server → Amnezia → server.

### Servers tab
- Stays the library of own servers (import, rename, delete), but **no "active" radio** any more; each row
  shows "used in: Work, Home" chips. Tab may be renamed "Hops" later; not needed now.

### Strings
- New strings in `tool/strings.py` (en + ru): chain, hop, entry, exit, add hop, editor errors; core error codes
  for chain validation in `ui/Common.kt` `errorText`. Remove `mode_*` strings.

### Sharing chains as links (new requirement)

A standard `vless://` link describes **one** outbound; there is no field for "dial me through X". So:

| Chain shape | Export | Who can import it |
|---|---|---|
| 1 own hop | the plain standard link (`vless://`, `vmess://`, `trojan://`, `ss://`, …), as imported | any client |
| N own hops | **chain text**: one standard link per line, entry first, each named `#<chain>·<i>/<N>·<hop>` | sockmnezia rebuilds the chain; other clients import N separate servers |
| any | **Xray JSON** (outbounds + `dialerProxy`, no inbounds) | Xray-based clients with custom config (v2rayNG, v2rayN) keep the chaining |
| any | QR of `sockmnezia://chain?v=1#<base64url(chain text)>` | sockmnezia (single scan, fits ~5 hops) |

- **Amnezia is never exported.** Its config holds the subscription's per-device UUID and is tied to the
  gateway/device slot. In chain text it appears as a placeholder line `sockmnezia://amnezia`; on import the
  receiver's own subscription fills that position, or the chain is saved as invalid ("set up Amnezia")
  if they have none. Xray JSON export is offered only for chains without Amnezia.
- Import: the existing "Add server" paste/QR flow detects multi-line or `sockmnezia://chain` input → preview
  ("Import chain *Work*: 3 hops, 2 new servers, 1 already saved") → saves new servers (de-dup by
  protocol+address+port+id) and the chain.
- Share sheet warns that the text contains server credentials. The link goes only to Android's share
  sheet / clipboard (with `ClipDescription.EXTRA_IS_SENSITIVE`), never to logs.
- Core: `links.Format(outbound)` (the reverse of the parser, round-trip tested for every protocol/transport
  the parser accepts; servers imported as raw JSON fall back to JSON-only export), `engine.ExportChain(id,
  format)`, `engine.ImportChain(text)`; mobile + CLI (`sockmnezia chain export|import`).

## 4. Docs
- `DECISIONS.md`: D25 "Chains replace modes" (why Amnezia once, why UDP only as entry, tag naming, migration).
- `PLAN.md` §5.1 rewritten to point here; `CHANGELOG.md` entries for each user-visible step.

## 5. Milestones (each one a PR, main stays releasable)

| # | Content | Visible change |
|---|---|---|
| M0 | §6 fixes: stale probe, compact bar | yes, small |
| M1 | `xrayconf` hops + tests + integration test; engine keeps `Mode` and maps it to hops internally | none |
| M2 | State v2, chains API, migration, mobile + CLI, fetch-via-prefix | CLI only |
| M3 | Home chain card, chain editor, hop picker, Servers tab without active radio | the redesign |
| M4 | Chain share/import: `links.Format`, chain text, `sockmnezia://chain` QR, Xray JSON export | yes |
| M5 | Polish: presets, "used in" chips, per-hop ping in editor, docs/decisions | yes |

## 6. Fixes to ship first (M0)

### 6.1 Clear exit IP and ping when the selected config changes

Problem: `Core.probe` is cleared only when the proxy starts/stops (`LaunchedEffect(status.running)`). After
switching server/mode/location the previous exit IP and ping stay visible, and `runProbe()` keeps the old
value when the new test throws — so a failing new config looks like it works.

Fix:
- `Core.runProbe()`: set `_probe.value = null` **before** testing; keep a generation counter and drop a
  result whose generation is stale (a test started before the switch must not land after it).
- Clear on config change: `Core.clearProbe()` from every mutation that changes outbounds
  (`setSettings` with another mode, `setActiveOwnServer`, `fetch`, later `setActiveChain`/`saveChain`).
  Once M2 lands, key it on `status.configRev` instead: `LaunchedEffect(status.running, status.configRev)`.
- A failed test shows "Check failed — tap to retry" in place of the exit line (not the old value).

### 6.2 Compact connection-check block

Today: band padding 28 dp top (half the FAB) + 12 dp bottom, card padding 12 dp all round, title in
`titleMedium`, then route and exit/ping on separate lines (≈140 dp). Target ≈ 84 dp, button still
overlapping the top edge by half:
- Status as a coloured dot + `labelLarge` text on the same line as the ping: `● Connected · 437 ms`.
  First line gets `end` padding = FAB width so it never runs under the button.
- One monospace line for the path: `203.0.113.9 [US]`; when the exit IP equals the last route hop, show it
  once with "exit ✓" instead of repeating the address; otherwise `route → exit 203.0.113.9 [NL]`.
- Card padding 12 dp horizontal / 8 dp vertical, `spacedBy(0.dp)`, band bottom padding 8 dp.
- Keep: tap = test, `StartStopButtonHeight / 2` overlap, FAB outside the `Surface` (cropping fix from #5).

## 7. Open questions for the owner

1. Should Amnezia be allowed as the **exit** (today's "Amnezia only")? Plan assumes yes.
2. Deleting a server that chains use: block with a list of chains (assumed), or drop the hop silently?
3. Chain text with a `sockmnezia://amnezia` placeholder line: OK, or should Amnezia chains not be shareable
   at all?
4. Keep a quick "switch chain" in the notification / QS tile (cycles through chains)? Not planned for M3.
