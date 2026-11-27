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
  .toast { background: var(--card2); border: 1px solid var(--line); border-radius: 10px; padding: 10px 14px; box-shadow: var(--shadow); white-space: pre-line; }
  .toast.error { border-color: color-mix(in srgb, var(--warn) 60%, var(--line)); }
</style>
