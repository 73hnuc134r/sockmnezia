<script lang="ts">
  import { t } from "../lib/i18n";
  // The last test as a pill: green < 300 ms, amber < 800 ms, red slower or failed.
  let { ms = 0, error = "", testing = false, title = "" }: { ms?: number; error?: string; testing?: boolean; title?: string } = $props();
  const cls = $derived(testing ? "run" : error ? "bad" : ms <= 0 ? "none" : ms < 300 ? "ok" : ms < 800 ? "mid" : "bad");
</script>

<span class="ping {cls}" {title}>
  {#if testing}{t("conn_testing")}{:else if error}{t("ping_failed")}{:else if ms > 0}{t("conn_ping", ms)}{:else}{t("ping_untested")}{/if}
</span>

<style>
  .ping { font: 500 11px/1 var(--ui); padding: 4px 8px; border-radius: 10px; white-space: nowrap; display: inline-block; }
  .ok { color: var(--ok); background: color-mix(in srgb, var(--ok) 14%, transparent); }
  .mid { color: var(--amz); background: color-mix(in srgb, var(--amz) 14%, transparent); }
  .bad { color: var(--warn); background: color-mix(in srgb, var(--warn) 14%, transparent); }
  .run, .none { color: var(--dim); background: color-mix(in srgb, var(--dim) 14%, transparent); }
</style>
