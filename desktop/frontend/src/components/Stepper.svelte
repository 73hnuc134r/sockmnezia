<script lang="ts">
  import { t } from "../lib/i18n";
  import { flag } from "../lib/format";
  import type { engine } from "../lib/api";
  import ServerTags from "./ServerTags.svelte";
  import Tag from "./Tag.svelte";
  // This PC → hops → Internet, left to right; the last hop is the exit.
  let { hops, exit = "" }: { hops: engine.HopInfo[]; exit?: string } = $props();
</script>

<div class="steps">
  <div class="node"><span class="end"></span><span class="t">{t("d_this_pc")}</span></div>
  {#each hops as h, i}
    <div class="link"></div>
    <div class="node">
      <span class="dot" class:amz={h.kind === "amnezia"}>{h.kind === "amnezia" ? "A" : i + 1}</span>
      <span class="t ellipsis" title={h.name}>{h.missing ? t("hop_missing") : h.name}</span>
      {#if h.kind === "amnezia"}
        <span class="tags"><Tag text="amnezia" color="var(--t-amz)" />{#if h.country}<Tag text={h.country.toUpperCase()} />{/if}</span>
      {:else if !h.missing}
        <ServerTags protocol={h.protocol} security={h.security} network={h.network} />
      {/if}
    </div>
  {/each}
  <div class="link"></div>
  <div class="node">
    <span class="end"></span><span class="t">{t("chain_internet")}</span>
    {#if exit}<span class="exit mono">{exit}</span>{/if}
  </div>
</div>

<style>
  .steps { display: flex; align-items: flex-start; overflow-x: auto; padding-bottom: 2px; }
  .node { display: grid; gap: 4px; justify-items: center; min-width: 96px; max-width: 170px; text-align: center; flex: none; }
  .dot { width: 26px; height: 26px; border-radius: 50%; display: grid; place-items: center; font: 600 11px/1 var(--ui);
    border: 2px solid var(--accent); color: var(--accent); background: var(--card2); }
  .dot.amz { border-color: var(--amz); color: var(--amz); }
  .end { width: 12px; height: 12px; border-radius: 50%; background: var(--dim); margin: 7px; }
  .link { flex: 1; min-width: 20px; height: 2px; background: var(--line); margin-top: 13px; }
  .t { font-size: 12.5px; max-width: 100%; }
  .tags { display: inline-flex; gap: 4px; }
  .exit { font-size: 11px; color: var(--ok); }
</style>
