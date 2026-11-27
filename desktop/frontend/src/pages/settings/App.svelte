<script lang="ts">
  import { Core, Desktop, type engine, type desk } from "../../lib/api";
  import { store } from "../../lib/store.svelte";
  import { t } from "../../lib/i18n";
  import Switch from "../../components/Switch.svelte";

  // Desktop behaviour (prefs, autostart) and the engine's app options (log level, gateway route).
  let { settings }: { settings: engine.Settings } = $props();
  let prefs = $state<desk.Prefs | null>(null);
  let autostart = $state<boolean | null>(null);
  Desktop.Prefs().then((p) => (prefs = p));
  Desktop.Autostart().then((a) => (autostart = a)).catch(() => (autostart = null));

  async function setPref(f: (p: desk.Prefs) => void) {
    if (!prefs) return;
    f(prefs);
    await store.act(() => Desktop.SetPrefs($state.snapshot(prefs) as desk.Prefs));
  }
  async function setAutostart(on: boolean) {
    const ok = await store.act(() => Desktop.SetAutostart(on).then(() => true));
    if (ok) autostart = on;
  }
  async function setOption(f: (o: engine.Options) => void) {
    const ns = structuredClone($state.snapshot(settings)) as engine.Settings;
    f(ns.options);
    await store.act(() => Core.SetSettings(ns));
  }
</script>

<div class="card">
  <h3>{t("app_settings")}</h3>
  {#if prefs}
    <Switch label={t("d_autostart")} checked={!!autostart} disabled={autostart === null} hint={t("d_autostart_hint")} onchange={setAutostart} />
    <Switch label={t("d_connect_on_launch")} checked={prefs.connect_on_launch} hint={t("d_connect_on_launch_hint")} onchange={(v) => setPref((p) => (p.connect_on_launch = v))} />
    <Switch label={t("d_close_to_tray")} checked={prefs.close_to_tray} hint={t("d_close_to_tray_hint")} onchange={(v) => setPref((p) => (p.close_to_tray = v))} />
  {/if}
  {#if store.servers.length}
    <div class="field">
      <label for="a-via">{t("fetch_via")}</label>
      <select id="a-via" value={settings.options.fetch_via ?? ""} onchange={(e) => setOption((o) => (o.fetch_via = e.currentTarget.value))}>
        <option value="">{t("fetch_via_direct")}</option>
        {#each store.servers as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
      </select>
      <span class="hint">{t("fetch_via_hint")}</span>
    </div>
  {/if}
  <div class="field narrow">
    <label for="a-log">{t("log_level")}</label>
    <select id="a-log" value={settings.options.log_level} onchange={(e) => setOption((o) => (o.log_level = e.currentTarget.value))}>
      {#each ["error", "warning", "info", "debug"] as l}<option value={l}>{l}</option>{/each}
    </select>
  </div>
</div>

<style>
  .narrow { max-width: 260px; }
</style>
