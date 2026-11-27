<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, Desktop } from "../lib/api";
  import { t } from "../lib/i18n";
  import { store } from "../lib/store.svelte";
  import { errorText } from "../lib/errors";
  import Modal from "./Modal.svelte";

  // Shares a saved server (its link) or chain (links, or Xray JSON for chains without Amnezia), with a QR code.
  let { kind, id, name, withAmnezia = false, onclose }: { kind: "server" | "chain"; id: string; name: string; withAmnezia?: boolean; onclose: () => void } = $props();

  let format = $state<"links" | "xray">("links");
  let text = $state("");
  let qr = $state("");
  let error = $state("");

  $effect(() => {
    const f = format;
    error = "";
    qr = "";
    (kind === "server" ? Core.ShareServer(id) : Core.ShareChain(id, f))
      .then(async (s) => {
        text = s;
        qr = await Desktop.QR(s.trim()).catch(() => "");
      })
      .catch((e) => (error = errorText(e)));
  });

  async function copy() {
    await Clipboard.SetText(text);
    store.toast(t("share_copied"));
  }
</script>

<Modal title={t("share_title", name)} {onclose} width={640}>
  {#if kind === "chain"}
    <div class="seg" style="justify-self:start">
      <button class:on={format === "links"} onclick={() => (format = "links")}>{t("share_format_links")}</button>
      <button class:on={format === "xray"} disabled={withAmnezia} onclick={() => (format = "xray")}>{t("share_format_xray")}</button>
    </div>
    <div class="hint">{withAmnezia ? `${t("share_amnezia_note")} ${t("share_xray_amnezia")}` : t("share_links_hint")}</div>
  {/if}
  <div class="hint warn">{t("share_warning")}</div>
  {#if error}
    <div class="hint warn">{error}</div>
  {:else}
    <div class="body">
      <textarea readonly rows="8" value={text}></textarea>
      {#if format === "links"}
        {#if qr}<img src={qr} alt={t("share_qr")} />{:else if text}<div class="hint qrno">{t("share_qr_too_long")}</div>{/if}
      {/if}
    </div>
  {/if}
  {#snippet actions()}
    <button class="btn flat" onclick={onclose}>{t("d_close")}</button>
    <button class="btn p" disabled={!text} onclick={copy}>{t("share_copy")}</button>
  {/snippet}
</Modal>

<style>
  .body { display: grid; grid-template-columns: 1fr auto; gap: 12px; align-items: start; }
  img { width: 200px; height: 200px; image-rendering: pixelated; background: #fff; border-radius: 8px; padding: 6px; }
  .qrno { width: 200px; }
</style>
