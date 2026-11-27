<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, Desktop, type engine } from "../../lib/api";
  import { store } from "../../lib/store.svelte";
  import { t, tp } from "../../lib/i18n";
  import { coreError, errorText } from "../../lib/errors";
  import { flag } from "../../lib/format";
  import Modal from "../../components/Modal.svelte";

  // Amnezia subscription: key, gateway key (from the official APK), device ID, the config and its location.
  const st = $derived(store.status);
  let key = $state("");
  let busy = $state<string | null>(null);
  let captcha = $state<{ id: string; image: string; hint: string } | null>(null);
  let answer = $state("");
  let pemOpen = $state(false);
  let pem = $state("");
  let countries = $state<{ server_country_code: string; server_country_name: string }[]>([]);
  let pings = $state<Record<string, number | "fail" | "testing">>({});

  $effect(() => {
    void st?.fetched_at;
    Core.Countries().then((c) => (countries = c ?? []));
  });

  async function fetch(req: Partial<engine.FetchRequest> = {}) {
    busy = t("fetching");
    try {
      await Core.Fetch({ country: req.country ?? "", captcha_id: req.captcha_id ?? "", captcha_solution: req.captcha_solution ?? "" });
      store.toast(t("config_updated"));
    } catch (e) {
      const ce = coreError(e);
      if ((ce.code === "captcha_required" || ce.code === "captcha_invalid") && ce.captcha) {
        captcha = { id: ce.captcha.captcha_id, image: ce.captcha.captcha_image, hint: ce.captcha.hint ?? "" };
        answer = "";
        if (ce.code === "captcha_invalid") store.toast(errorText(ce), true);
      } else store.toast(errorText(ce), true);
    } finally {
      busy = null;
      await store.reload();
    }
  }

  async function saveKey() {
    if (await store.act(() => Core.SetKey(key.trim()).then(() => true), t("key_saved"))) key = "";
  }

  async function pickApk() {
    const path = await Desktop.PickFile(t("pick_official_apk"));
    if (!path) return;
    busy = t("scanning_apk");
    const n = await store.act(() => Core.ImportGatewayKeysFromFile(path));
    busy = null;
    if (n !== undefined) store.toast(tp("gateway_keys_found", n));
  }

  async function download() {
    busy = t("downloading_apk");
    const r = await store.act(() => Core.ImportGatewayKeysFromOfficialRelease());
    busy = null;
    if (r) store.toast(tp("gateway_keys_found_release", r.count, r.release));
  }

  async function savePem() {
    pemOpen = false;
    const n = await store.act(() => Core.SetGatewayKeysPEM(pem));
    if (n !== undefined) store.toast(tp("gateway_keys_found", n));
    pem = "";
  }

  async function findFastest() {
    for (const c of countries) pings[c.server_country_code] = "testing";
    await Promise.all(
      countries.map(async (c) => {
        try {
          pings[c.server_country_code] = await Core.PingCountry(c.server_country_code);
        } catch {
          pings[c.server_country_code] = "fail";
        }
      }),
    );
  }
  const imgSrc = $derived(captcha ? (captcha.image.startsWith("data:") ? captcha.image : `data:image/png;base64,${captcha.image}`) : "");
</script>

<div class="col">
  <div class="card">
    <h3>{t("subscription_key")}</h3>
    <div class="hint">{st?.has_key ? t("key_saved") : t("key_hint")}</div>
    <div class="row nowrap">
      <input class="grow" type="password" placeholder="vpn://…" bind:value={key} />
      <button class="btn" onclick={async () => (key = ((await Clipboard.Text()) ?? "").trim())}>{t("paste")}</button>
      <button class="btn p" disabled={!key.trim()} onclick={saveKey}>{t("save")}</button>
      {#if st?.has_key}<button class="btn flat danger" onclick={() => store.act(() => Core.SetKey(""))}>{t("remove")}</button>{/if}
    </div>
  </div>

  <div class="card">
    <h3>{t("gateway_key")}</h3>
    <div class="hint">{st?.has_gateway_key ? t("gateway_key_saved") : t("gateway_key_hint")}</div>
    <div class="row">
      <button class="btn" disabled={!!busy} onclick={pickApk}>{t("pick_official_apk")}</button>
      <button class="btn" disabled={!!busy} onclick={download}>{t("download_official_apk")}</button>
      <button class="btn flat" onclick={() => (pemOpen = true)}>{t("paste_pem")}</button>
    </div>
  </div>

  {#if st?.device_id}
    <div class="card">
      <h3>{t("device_id")}</h3>
      <div class="row nowrap"><span class="mono selectable grow">{st.device_id}</span>
        <button class="btn small" onclick={async () => { await Clipboard.SetText(st.device_id ?? ""); store.toast(t("share_copied")); }}>{t("copy")}</button></div>
      <div class="hint">{t("device_id_hint")}</div>
    </div>
  {/if}

  <div class="card">
    <h3>{t("config")}</h3>
    <div class="hint">{t("d_device_slot_warning")}</div>
    {#if st?.has_config}
      <div class="muted">{t("config_status", st.country_name || st.country, st.fetched_at ? new Date(st.fetched_at).toLocaleString() : "—")}</div>
      {#if st.subscription_to}<div class="muted">{t("subscription_until", new Date(st.subscription_to).toLocaleDateString())}</div>{/if}
    {/if}
    <div class="row">
      <button class="btn p" disabled={!st?.has_key || !st?.has_gateway_key || !!busy} onclick={() => fetch()}>{st?.has_config ? t("refresh_config") : t("fetch_config")}</button>
      {#if busy}<span class="muted">{busy}</span>{/if}
    </div>
    {#if countries.length}
      <div class="row">
        <span class="cap grow">{t("d_location")}</span>
        <button class="btn small" disabled={!!busy} onclick={findFastest}>{t("location_find_fastest")}</button>
      </div>
      <div class="locs">
        {#each countries as c (c.server_country_code)}
          {@const p = pings[c.server_country_code]}
          <button class="loc" class:on={c.server_country_code === st?.country} disabled={!!busy} onclick={() => fetch({ country: c.server_country_code })}>
            <span>{flag(c.server_country_code)}</span><span class="grow ellipsis">{c.server_country_name}</span>
            <span class="dim">{p === "testing" ? t("location_ping_testing") : p === "fail" ? t("location_ping_failed") : p ? t("conn_ping", p) : ""}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>
  <div class="hint">{t("unofficial")}</div>
</div>

{#if captcha}
  <Modal title={t("captcha_title")} onclose={() => (captcha = null)}>
    <img src={imgSrc} alt="" class="cap-img" />
    {#if captcha.hint}<div class="hint">{captcha.hint}</div>{/if}
    <div class="field"><label for="cap">{t("captcha_answer")}</label><input id="cap" bind:value={answer} /></div>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (captcha = null)}>{t("cancel")}</button>
      <button class="btn p" disabled={!answer.trim()} onclick={() => { const c = captcha!; captcha = null; void fetch({ captcha_id: c.id, captcha_solution: answer.trim() }); }}>{t("ok")}</button>
    {/snippet}
  </Modal>
{/if}
{#if pemOpen}
  <Modal title={t("paste_pem")} onclose={() => (pemOpen = false)} width={560}>
    <textarea rows="8" bind:value={pem}></textarea>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (pemOpen = false)}>{t("cancel")}</button>
      <button class="btn p" disabled={!pem.trim()} onclick={savePem}>{t("save")}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .locs { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 6px; }
  .loc { display: flex; gap: 8px; align-items: center; padding: 7px 10px; border-radius: 8px; border: 1px solid var(--line); background: var(--card2); cursor: pointer; text-align: left; min-width: 0; }
  .loc.on { border-color: var(--accent); background: var(--sel); }
  .loc:disabled { opacity: 0.6; cursor: default; }
  .cap-img { max-width: 100%; height: 100px; object-fit: contain; background: #fff; border-radius: 6px; }
</style>
