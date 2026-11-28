<script lang="ts">
  import { Core, type engine } from "../lib/api";
  import { t } from "../lib/i18n";
  import { store } from "../lib/store.svelte";
  import { errorText } from "../lib/errors";
  import Modal from "./Modal.svelte";

  // Import preview: what the shared text adds (a chain, or servers); nothing is saved until confirmed.
  let { text, onclose, ondone }: { text: string; onclose: () => void; ondone?: (r: engine.ImportResult) => void } = $props();

  let preview = $state<engine.ImportResult | null>(null);
  let error = $state("");
  let busy = $state(false);

  $effect(() => {
    Core.Import(text, true).then((r) => (preview = r)).catch((e) => (error = errorText(e)));
  });

  async function doImport() {
    busy = true;
    const r = await store.act(() => Core.Import(text, false), t("imported"));
    busy = false;
    if (r) {
      ondone?.(r);
      onclose();
    }
  }
</script>

<Modal title={(preview?.chains?.length ?? 0) > 1 ? t("import_all_q", preview!.chains!.length) : preview?.chains?.length ? t("import_chain_q", preview.chain) : t("import_servers_q")} {onclose}>
  {#if error}
    <div class="hint warn" style="white-space:pre-line">{error}</div>
  {:else if preview}
    {#if preview.chains?.length === 1}<div>{t("import_hops", preview.hops ?? 0)}</div>{/if}
    {#if (preview.chains?.length ?? 0) > 1}<div>{t("import_chains", preview.chains!.join(", "))}</div>{/if}
    {#if preview.known_chains?.length}<div class="muted">{t("import_known", preview.known_chains.join(", "))}</div>{/if}
    {#if preview.new_servers?.length}<div>{t("import_new", preview.new_servers.join(", "))}</div>{/if}
    {#if preview.known_servers?.length}<div class="muted">{t("import_known", preview.known_servers.join(", "))}</div>{/if}
    {#if preview.needs_amnezia}<div class="hint warn">{t("import_needs_amnezia")}</div>{/if}
  {:else}
    <div class="dim">…</div>
  {/if}
  {#snippet actions()}
    <button class="btn flat" onclick={onclose}>{t("cancel")}</button>
    <button class="btn p" disabled={!preview || busy} onclick={doImport}>{t("import_do")}</button>
  {/snippet}
</Modal>
