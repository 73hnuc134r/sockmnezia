<script lang="ts">
  import { Desktop, type desk } from "../../lib/api";
  import { store } from "../../lib/store.svelte";
  import { t } from "../../lib/i18n";
  import Switch from "../../components/Switch.svelte";

  // Update check (desktop-v… releases only). The app opens the release page; it does not install updates.
  let prefs = $state<desk.Prefs | null>(null);
  let busy = $state(false);
  Desktop.Prefs().then((p) => (prefs = p));
  async function set(f: (p: desk.Prefs) => void) {
    if (!prefs) return;
    f(prefs);
    await store.act(() => Desktop.SetPrefs($state.snapshot(prefs) as desk.Prefs));
  }
</script>

<div class="card">
  <h3>{t("updates")}</h3>
  {#if prefs}
    <Switch label={t("check_updates")} checked={prefs.check_updates} hint={t("d_check_updates_hint")} onchange={(v) => set((p) => (p.check_updates = v))} />
    <Switch label={t("beta_updates")} checked={prefs.beta_updates} onchange={(v) => set((p) => (p.beta_updates = v))} />
  {/if}
  <div class="row">
    <button class="btn" disabled={busy} onclick={async () => { busy = true; await store.checkUpdate(!!prefs?.beta_updates, true); busy = false; }}>{t("check_now")}</button>
    <span class="muted">{t("version", store.info?.version ?? "")}</span>
  </div>
  {#if store.update}
    <div class="okc">{t("update_available", store.update.version)}</div>
    <button class="btn p" style="justify-self:start" onclick={() => Desktop.OpenURL(store.update!.url)}>{t("open_release")}</button>
  {/if}
</div>
