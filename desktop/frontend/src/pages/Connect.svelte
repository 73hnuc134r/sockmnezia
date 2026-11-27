<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, Desktop, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t, tp } from "../lib/i18n";
  import { bytes, duration, flag, rate } from "../lib/format";
  import { coreError, errorText } from "../lib/errors";
  import Icon from "../components/Icon.svelte";
  import Stepper from "../components/Stepper.svelte";
  import Spark from "../components/Spark.svelte";
  import Ping from "../components/Ping.svelte";

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
  async function setMode(v: string) {
    await store.act(() => Core.SetSystemProxy(v));
  }

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
  <div class="card">
    <div class="row nowrap">
      <span class="cap grow ellipsis">{st?.hops?.length ? t("d_connected_through", title) : t("chain_none")}</span>
      <button class="btn" onclick={() => (store.page = "servers")}><Icon name="swap" />{t("chain_switch")}</button>
    </div>
    {#if st?.hops?.length}
      <Stepper hops={st.hops} exit={probe.state === "done" ? `${probe.ip}${probe.country ? " " + flag(probe.country) : ""}` : ""} />
    {/if}
    {#if st?.chain_error}
      <div class="hint warn">{errorText(st.chain_error)}</div>
    {/if}
  </div>

  <div class="row controls">
    <button class="btn p big" disabled={busy || (!st?.running && !!st?.chain_error)} onclick={toggle}>
      <Icon name={st?.running ? "stop" : "play"} size={14} />{st?.running ? t("action_stop") : t("action_start")}
    </button>
    <div class="modebox">
      <span class="cap">{t("d_mode")}</span>
      <div class="seg">
        {#each [["", "d_mode_off"], ["system", "d_mode_system"], ["pac", "d_mode_pac"]] as [v, label]}
          <button class:on={(st?.system_proxy ?? "") === v} onclick={() => setMode(v)}>{t(label)}</button>
        {/each}
      </div>
    </div>
    <span class="grow"></span>
    <button class="exitbox" disabled={!st?.running || probe.state === "testing"} onclick={test} title={t("d_click_to_test")}>
      <span class="cap">{t("d_exit_click")}</span>
      {#if !st?.running}
        <span class="dim">{t("conn_stopped")}</span>
      {:else if probe.state === "done"}
        <span class="row nowrap"><span class="mono okc selectable">{probe.ip}</span>{#if probe.country}<span>{flag(probe.country)} {probe.country}</span>{/if}<Ping ms={probe.ms} /></span>
      {:else if probe.state === "testing"}
        <Ping testing />
      {:else if probe.state === "failed"}
        <span class="warnc">{t("conn_failed")}</span>
      {:else}
        <span class="dim">{t("d_click_to_test")}</span>
      {/if}
    </button>
  </div>

  {#if st}
    <div class="modeinfo">
      <div class="hint">{t(st.system_proxy === "pac" ? "d_mode_pac_hint" : st.system_proxy === "system" ? "d_mode_system_hint" : "d_mode_off_hint")}</div>
      {#if st.system_proxy}
        <div class="hint warn">{t("d_mode_warning")}</div>
        {#if sys && !sys.supported}
          <div class="hint warn selectable">{t("d_mode_unsupported", st.system_proxy_port)}</div>
        {:else if sys?.error}
          <div class="hint warn">{t("d_mode_failed", sys.error)}</div>
        {:else if sys?.applied}
          <div class="hint okc">✓ {t("d_mode_applied")} · <span class="mono">127.0.0.1:{st.system_proxy_port}</span> · {t("d_mode_restore")}</div>
        {/if}
      {/if}
    </div>
  {/if}

  <div class="stats">
    <div class="card stat"><span class="cap">{t("d_download")}</span><b>{st?.running ? rate(lastDown) : "—"}</b><Spark values={store.downRate} color="var(--accent)" /></div>
    <div class="card stat"><span class="cap">{t("d_upload")}</span><b>{st?.running ? rate(lastUp) : "—"}</b><Spark values={store.upRate} color="var(--ok)" /></div>
    <div class="card stat">
      <span class="cap">{t("d_session")}</span>
      <b>{st?.running ? bytes(st.up + st.down) : "—"}</b>
      <span class="muted">{st?.running && st.started_at ? t("d_running_for", duration(Date.now() - new Date(st.started_at).getTime())) : ""}</span>
    </div>
  </div>

  {#if settings}
    <div class="card">
      <span class="cap">{t("d_local_proxy")}</span>
      <div class="row nowrap">
        <span class="mono grow ellipsis selectable">{socks}</span>
        <button class="btn icon flat" title={showPass ? t("hide") : t("show")} onclick={() => (showPass = !showPass)}><Icon name={showPass ? "eyeoff" : "eye"} /></button>
        <button class="btn small" onclick={() => copy(socksFull)}><Icon name="copy" size={14} />{t("copy")}</button>
      </div>
      {#if http}
        <div class="row nowrap">
          <span class="mono grow ellipsis selectable">{http} <span class="dim">{t("d_same_credentials")}</span></span>
          <button class="btn small" onclick={() => copy(http)}><Icon name="copy" size={14} />{t("copy")}</button>
        </div>
      {/if}
      {#if st && st.auth_failures > 0}<div class="hint warn">{tp("auth_failures", st.auth_failures)}</div>{/if}
    </div>
  {/if}
</div>

<style>
  .page { display: grid; gap: 14px; align-content: start; }
  .controls { gap: 16px; align-items: stretch; }
  .big { padding: 14px 26px; font-size: 15px; border-radius: 12px; min-width: 130px; justify-content: center; }
  .exitbox { display: grid; gap: 6px; justify-items: end; align-content: center; background: transparent; border: 1px solid transparent; border-radius: 10px; padding: 6px 10px; cursor: pointer; text-align: right; }
  .exitbox:hover:not(:disabled) { border-color: var(--line); background: var(--hover); }
  .exitbox:disabled { cursor: default; }
  .banner { background: var(--sel); border-radius: 10px; padding: 8px 12px; }
  .modebox { display: grid; gap: 6px; align-content: center; }
  .modeinfo { display: grid; gap: 4px; margin-top: -4px; }
  .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
  .stat { gap: 6px; padding: 12px 14px; }
  .stat b { font-size: 20px; font-weight: 600; font-variant-numeric: tabular-nums; }
</style>
