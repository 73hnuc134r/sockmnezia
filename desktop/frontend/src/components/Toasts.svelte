<script lang="ts">
  import { store } from "../lib/store.svelte";
</script>

<div class="toasts" role="status" aria-live="polite">
  {#each store.toasts as m (m.id)}
    <div class="toast" class:error={m.error}>{m.text}</div>
  {/each}
</div>

<style>
  .toasts { position: fixed; bottom: 44px; left: 50%; transform: translateX(-50%); display: grid; gap: 8px; z-index: 50; width: min(520px, 90vw); }
  .toast { background: var(--on-surface); color: var(--s); border-radius: 8px; padding: 12px 16px; box-shadow: var(--shadow-dlg); white-space: pre-line; }
  .toast.error { background: var(--error-c); color: var(--on-error-c); }
</style>
