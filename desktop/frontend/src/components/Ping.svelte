<script lang="ts">
  import { t } from "../lib/i18n";
  // Country code + the last test as a pill; failures are red.
  let { ms = 0, error = "", testing = false, title = "", cc = "" }: { ms?: number; error?: string; testing?: boolean; title?: string; cc?: string } = $props();
  const bad = $derived(!testing && !!error);
</script>

<span class="ping" class:bad class:withcc={!!cc} {title}>
  {#if cc}<span class="cc">{cc.toUpperCase()}</span>{/if}
  {#if testing}{t("conn_testing")}{:else if error}{t("ping_failed")}{:else if ms > 0}{t("conn_ping", ms)}{:else}{t("ping_untested")}{/if}
</span>

<style>
  .ping { font: 500 12px/16px var(--ui); padding: 3px 8px; border-radius: 8px; white-space: nowrap; display: inline-flex; gap: 5px; align-items: center;
    background: var(--s-high); color: var(--muted); font-variant-numeric: tabular-nums; }
  .withcc { padding-left: 4px; }
  .bad { background: var(--error-c); color: var(--on-error-c); }
</style>
