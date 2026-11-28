<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, Desktop, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t, tp } from "../lib/i18n";
  import { bytes, countryName, duration, rate } from "../lib/format";
  import { coreError, errorText } from "../lib/errors";
  import Icon from "../components/Icon.svelte";
  import ServerTags from "../components/ServerTags.svelte";
  import Tag from "../components/Tag.svelte";
  import Spark from "../components/Spark.svelte";

  const st = $derived(store.status);
  let busy = $state(false);
  let settings = $state<engine.Settings | null>(null);
  let showPass = $state(false);

  // The exit test belongs to one selection: it is cleared when the chain or server changes or the proxy stops,
  // so an old result never shows next to a new selection.
  type Probe = { state: "idle" | "testing" | "failed"; key: string } | { state: "done"; key: string; ip: string; ms: number; country: string };
  let probe = $state<Probe>({ state: "idle", key: "" });
  const key = $derived(`${st?.chain_id ?? ""}/${st?.server_id ?? ""}/${st?.running ? st.started_at : "off"}`);
  $effect(() => {
    if (probe.key !== key) probe = { state: "idle", key };
  });

  $effect(() => {
    void store.chains; // reload with the lists
    Core.Settings().then((s) => (settings = s));
  });

  async function test() {
    if (!st?.running) return;
    const k = key;
    probe = { state: "testing", key: k };
    try {
      const r = await Core.TestChain();
      if (probe.key === k) probe = { state: "done", key: k, ...r };
    } catch (e) {
      if (probe.key === k) probe = { state: "failed", key: k };
      store.toast(errorText(e), true);
    }
  }

  async function toggle() {
    busy = true;
    try {
      if (st?.running) await Core.Stop();
      else {
        await Core.Start();
        await store.reload();
        void test();
      }
    } catch (e) {
      store.toast(errorText(e), true);
    } finally {
      busy = false;
      await store.reload();
    }
  }

  const title = $derived(st?.server_id ? (store.servers.find((s) => s.id === st.server_id)?.name ?? "") : (st?.chain_name ?? ""));
  const socks = $derived(settings ? `socks5://${settings.inbound.user}:${showPass ? settings.inbound.pass : "••••••••"}@${settings.inbound.listen}:${settings.inbound.socks_port}` : "");
  const socksFull = $derived(settings ? `socks5://${settings.inbound.user}:${settings.inbound.pass}@${settings.inbound.listen}:${settings.inbound.socks_port}` : "");
  const http = $derived(settings && settings.inbound.http_port ? `http://${settings.inbound.listen}:${settings.inbound.http_port}` : "");
  const lastDown = $derived(store.downRate.at(-1) ?? 0);
  const lastUp = $derived(store.upRate.at(-1) ?? 0);

  const sys = $derived(store.desktop?.sys_proxy);
  const mode = $derived(st?.mode === "vpn" ? "tun" : (st?.system_proxy ?? ""));
  let modeBusy = $state(false);
  let dnsWarning = $state(false);
  Desktop.TUNDNSWarning().then((w) => (dnsWarning = w));
  async function setMode(v: string) {
    modeBusy = true; // switching to or from TUN restarts the proxy (and may ask for administrator rights)
    await store.act(() => Core.SetConnectionMode(v));
    modeBusy = false;
    await store.reload();
  }

  // Uptime, refreshed with the status (every second).
  const uptime = $derived(st?.running && st.started_at ? duration(Date.now() - new Date(st.started_at).getTime()) : "");

  async function copy(text: string) {
    await Clipboard.SetText(text);
    store.toast(t("share_copied"));
  }
</script>

<div class="page">
  {#if store.update}
    <div class="banner row nowrap">
      <span class="grow">{t("update_available", store.update.version)}</span>
      <button class="btn small" onclick={() => Desktop.OpenURL(store.update!.url)}>{t("open_release")}</button>
      <button class="btn small flat" onclick={() => (store.update = null)}>{t("d_close")}</button>
    </div>
  {/if}
  <h1>{t("d_nav_connect")}</h1>

  <div class="grid">
    <section class="island chain">
      <div class="row nowrap">
        <span class="cap grow">{st?.server_id ? t("d_active_server") : t("d_active_chain")}</span>
        {#if st?.chain_id && !st.server_id}
          <button class="btn" onclick={() => (store.page = "servers")}><Icon name="edit" size={18} />{t("chain_edit")}</button>
        {/if}
      </div>
      {#if st?.hops?.length}
        <h2 class="ellipsis">{title}</h2>
        <div class="hops">
          {#each st.hops as h, i}
            {#if i > 0}<div class="rail"></div>{/if}
            <div class="hop">
              <span class="num" class:amz={h.kind === "amnezia"}>{i + 1}</span>
              <span class="grow hinfo">
                <span class="ellipsis" class:warnc={h.missing}>{h.missing ? t("hop_missing") : h.name}</span>
                {#if h.kind === "amnezia"}
                  <span class="tags"><Tag text="Amnezia" tone="blue" /></span>
                {:else if !h.missing}
                  <ServerTags protocol={h.protocol} security={h.security} network={h.network} port={h.port ?? 0} />
                {/if}
              </span>
              {#if h.country}<span class="ccpill"><span class="cc">{h.country.toUpperCase()}</span>{countryName(h.country)}</span>{/if}
            </div>
          {/each}
        </div>
      {:else}
        <div class="hint">{t("chain_none")}</div>
      {/if}
      {#if st?.chain_error}<div class="hint warn">{errorText(st.chain_error)}</div>{/if}
      <span class="grow"></span>
      <div class="tiles">
        <div class="tile"><span class="tl">↓ {t("d_download")}</span><b>{st?.running ? rate(lastDown) : "—"}</b><Spark values={store.downRate} color="var(--primary)" /></div>
        <div class="tile"><span class="tl">↑ {t("d_upload")}</span><b>{st?.running ? rate(lastUp) : "—"}</b><Spark values={store.upRate} color="var(--tertiary)" /></div>
        <div class="tile"><span class="tl">{t("d_session")}</span><b>{st?.running ? bytes(st.up + st.down) : "—"}</b><span class="tl">{uptime ? t("d_running_for", uptime) : ""}</span></div>
      </div>
    </section>

    <section class="island">
      <span class="cap">{t("d_how_apps")}</span>
      <div class="radios">
        {#each [["", "d_mode_off"], ["system", "d_mode_system"], ["tun", "d_mode_tun"]] as [v, label]}
          <label class="radio" class:on={mode === v} class:disabled={modeBusy}>
            <input type="radio" name="mode" checked={mode === v} disabled={modeBusy} onchange={() => setMode(v)} />
            <span>{t(label)}</span>
          </label>
        {/each}
      </div>
      {#if st}
        <div class="modeinfo">
          <div class="hint">{t(mode === "tun" ? "d_mode_tun_hint" : mode === "system" ? "d_mode_system_hint" : "d_mode_off_hint")}</div>
          {#if mode === "tun"}
            <div class="hint">{t("d_mode_tun_service")}</div>
            {#if dnsWarning}<div class="hint warn">{t("d_tun_dns_warning")}</div>{/if}
            {#if !st.running}<div class="hint warn">{t("d_tun_blocking")}</div>{/if}
          {:else}
            <div class="hint">{t("d_mode_tun_admin")}</div>
          {/if}
          {#if mode === "system"}
            <div class="hint warn">{t("d_mode_warning")}</div>
            {#if sys && !sys.supported}
              <div class="hint warn selectable">{t("d_mode_unsupported", st.system_proxy_port)}</div>
            {:else if sys?.error}
              <div class="hint warn">{t("d_mode_failed", sys.error)}</div>
            {:else if sys?.applied}
              <div class="hint okc">✓ {t("d_mode_applied")} · <span class="mono">127.0.0.1:{st.system_proxy_port}</span></div>
            {/if}
            <div class="hint">{t("d_mode_restore")}</div>
          {/if}
        </div>
      {/if}
    </section>

    {#if settings}
      <section class="island">
        <span class="cap">{t("d_local_proxy")}</span>
        <div class="kv"><span>SOCKS5</span><span class="mono selectable">{settings.inbound.listen}:{settings.inbound.socks_port}</span></div>
        {#if http}<div class="kv"><span>HTTP</span><span class="mono selectable">{settings.inbound.listen}:{settings.inbound.http_port}</span></div>{/if}
        <div class="row nowrap">
          <span class="mono grow ellipsis selectable dim">{socks}</span>
          <button class="btn icon flat" title={showPass ? t("hide") : t("show")} onclick={() => (showPass = !showPass)}><Icon name={showPass ? "eyeoff" : "eye"} size={18} /></button>
        </div>
        <div class="row">
          <button class="btn outline small" onclick={() => copy(socksFull)}><Icon name="copy" size={16} />SOCKS5</button>
          {#if http}<button class="btn outline small" title={t("d_same_credentials")} onclick={() => copy(http)}><Icon name="copy" size={16} />HTTP</button>{/if}
        </div>
        {#if st && st.auth_failures > 0}<div class="hint warn">{tp("auth_failures", st.auth_failures)}</div>{/if}
      </section>
    {/if}
  </div>

  <div class="bar">
    <button class="status" class:on={st?.running && probe.state === "done"} class:bad={probe.state === "failed"}
      disabled={!st?.running || probe.state === "testing"} onclick={test} title={t("d_click_to_check")}>
      <span class="led" class:lit={st?.running}></span>
      {#if !st?.running}
        <span class="sl">{t("d_not_running")}</span>
      {:else if probe.state === "done"}
        <span class="sl">{t("conn_connected")} · {t("conn_ping", probe.ms)}</span>
        <span class="sub">{#if probe.country}<span class="cc">{probe.country.toUpperCase()}</span>{countryName(probe.country)} ·{/if} <span class="selectable">{probe.ip}</span></span>
        {#if uptime}<span class="sub">· {uptime}</span>{/if}
      {:else if probe.state === "testing"}
        <span class="sl">{t("d_running")}</span><span class="sub">{t("conn_testing")}</span>
      {:else if probe.state === "failed"}
        <span class="sl">{t("conn_failed")}</span><span class="sub">{t("d_click_to_check")}</span>
      {:else}
        <span class="sl">{t("d_running")}</span><span class="sub">{t("d_click_to_check")}</span>
      {/if}
      {#if st?.running}<Icon name="refresh" size={18} />{/if}
    </button>
    <span class="grow"></span>
    <button class="fab" onclick={() => (store.page = "servers")}><Icon name="swap" size={20} />{t("chain_switch")}</button>
    <button class="fab p" disabled={busy || (!st?.running && !!st?.chain_error)} onclick={toggle}>
      <Icon name={st?.running ? "stop" : "play"} size={18} />{st?.running ? t("action_stop") : t("action_start")}
    </button>
  </div>
</div>

<style>
  .page { display: flex; flex-direction: column; gap: 20px; min-height: 100%; }
  .banner { background: var(--secondary-c); color: var(--on-secondary-c); border-radius: 16px; padding: 8px 8px 8px 16px; }
  .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; flex: 1; align-content: start; }
  .chain { grid-column: span 2; grid-row: span 2; }
  @media (max-width: 900px) { .grid { grid-template-columns: minmax(0, 1fr); } .chain { grid-column: auto; grid-row: auto; } }
  .hops { display: grid; }
  .hop { display: flex; gap: 14px; align-items: center; padding: 6px 0; min-width: 0; }
  .rail { width: 2px; height: 10px; background: var(--outline-v); margin-left: 19px; }
  .num { width: 40px; height: 40px; flex: none; border-radius: 20px; background: var(--secondary-c); color: var(--on-secondary-c); display: grid; place-items: center; font-weight: 600; }
  .num.amz { background: var(--tertiary-c); color: var(--on-tertiary-c); }
  .hinfo { display: grid; gap: 4px; min-width: 0; font-size: 16px; }
  .tags { display: inline-flex; gap: 4px; }
  .ccpill { display: inline-flex; gap: 5px; align-items: center; font: 500 12px/16px var(--ui); background: var(--s-high); color: var(--muted); border-radius: 8px; padding: 3px 8px 3px 4px; white-space: nowrap; }
  .tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .tile { background: var(--s); border-radius: 16px; padding: 10px 14px; display: grid; gap: 2px; min-width: 0; }
  .tile b { font-size: 18px; font-weight: 500; font-variant-numeric: tabular-nums; }
  .tl { font-size: 12px; color: var(--muted); }
  .radios { display: grid; gap: 4px; }
  .radio { display: flex; gap: 12px; align-items: center; min-height: 44px; padding: 0 12px; border-radius: 12px; cursor: pointer; }
  .radio:hover { background: var(--state); }
  .radio.on { background: var(--secondary-c); color: var(--on-secondary-c); font-weight: 500; }
  .radio.disabled { opacity: 0.5; cursor: default; }
  .modeinfo { display: grid; gap: 6px; }
  .kv { display: flex; justify-content: space-between; gap: 12px; }
  .bar { position: sticky; bottom: 0; display: flex; gap: 12px; align-items: center; padding: 14px 0 4px; background: var(--surface); z-index: 5; }
  .status { height: 56px; border: 0; border-radius: 16px; background: var(--s-high); color: var(--ink); display: flex; gap: 10px; align-items: center; padding: 0 18px;
    cursor: pointer; min-width: 0; max-width: 62%; white-space: nowrap; overflow: hidden; }
  .status:hover:not(:disabled) { box-shadow: inset 0 0 0 100px var(--state); }
  .status:disabled { cursor: default; }
  .status.on { background: var(--primary-c); color: var(--on-primary-c); }
  .status.bad { background: var(--error-c); color: var(--on-error-c); }
  .sl { font-weight: 500; }
  .sub { display: inline-flex; gap: 6px; align-items: center; opacity: 0.85; overflow: hidden; text-overflow: ellipsis; }
  .led { width: 10px; height: 10px; border-radius: 50%; border: 2px solid var(--outline); flex: none; }
  .led.lit { background: var(--primary); border-color: var(--primary); }
</style>
