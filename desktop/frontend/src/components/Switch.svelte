<script lang="ts">
  // A labelled checkbox row with an optional hint.
  let { label, checked, hint = "", disabled = false, onchange }: { label: string; checked: boolean; hint?: string; disabled?: boolean; onchange: (v: boolean) => void } = $props();
</script>

<label class="check" class:disabled>
  <input type="checkbox" {checked} {disabled} onchange={(e) => onchange(e.currentTarget.checked)} />
  <span><span>{label}</span>{#if hint}<span class="hint">{hint}</span>{/if}</span>
</label>

<style>
  .disabled { opacity: 0.5; cursor: default; }
</style>
