<script lang="ts">
  import type { Snippet } from "svelte";
  // A dialog over the window; Escape or a click outside closes it.
  let { title, onclose, children, actions, width = 480 }: { title: string; onclose: () => void; children: Snippet; actions?: Snippet; width?: number } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="back" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={title} style="width:min({width}px, 92vw)">
    <h2>{title}</h2>
    <div class="body">{@render children()}</div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </div>
</div>

<style>
  .back { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.45); display: grid; place-items: center; z-index: 40; }
  .dlg { background: var(--card); border: 1px solid var(--line); border-radius: 14px; padding: 18px 20px; box-shadow: var(--shadow); display: grid; gap: 14px; max-height: 88vh; }
  .body { overflow: auto; display: grid; gap: 10px; min-height: 0; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; }
</style>
