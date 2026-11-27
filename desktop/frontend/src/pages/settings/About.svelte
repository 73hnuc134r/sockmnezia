<script lang="ts">
  import { Core, Desktop } from "../../lib/api";
  import { store } from "../../lib/store.svelte";
  import { t } from "../../lib/i18n";
  import Modal from "../../components/Modal.svelte";

  let forget = $state(false);
  const info = $derived(store.info);
  const storeText: Record<string, string> = { dpapi: "d_secret_dpapi", keyring: "d_secret_keyring", file: "d_secret_file" };
</script>

<div class="card">
  <h3>{t("title_about")}</h3>
  <div class="muted">sockmnezia {info?.version} · Xray-core {info?.xray} · {info?.os}</div>
  <div class="hint">{t("unofficial")}</div>
  <div class="row">
    <button class="btn" onclick={() => Desktop.OpenURL("https://github.com/73hnuc134r/sockmnezia")}>GitHub</button>
  </div>
  <div class="hint" style="white-space:pre-line">{t("licenses_text")}</div>
</div>

<div class="card">
  <h3>{t("d_data")}</h3>
  <div class="muted">{t("d_data_dir")}: <span class="mono selectable">{info?.data_dir}</span></div>
  {#if info}<div class="hint" class:warn={info.secret_store === "file"}>{t(storeText[info.secret_store] ?? "d_secret_file")}</div>{/if}
  <div class="row"><button class="btn danger" onclick={() => (forget = true)}>{t("forget")}</button></div>
</div>

{#if forget}
  <Modal title={t("forget_q")} onclose={() => (forget = false)}>
    <div class="muted">{t("d_forget_text")}</div>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (forget = false)}>{t("cancel")}</button>
      <button class="btn p" onclick={async () => { forget = false; await store.act(() => Core.Forget(), t("forgotten")); }}>{t("delete")}</button>
    {/snippet}
  </Modal>
{/if}
