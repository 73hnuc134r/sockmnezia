<script lang="ts">
  // A tiny tag: tinted box, coloured text (as small as stays readable).
  let { text, color = "var(--muted)", title = "" }: { text: string; color?: string; title?: string } = $props();
</script>

<span class="tag" style="--c:{color}" {title}>{text}</span>

<style>
  .tag { font: 500 10px/14px var(--ui); padding: 0 4px; border-radius: 4px; display: inline-block; white-space: nowrap;
    color: var(--c); background: color-mix(in srgb, var(--c) 16%, transparent); }
</style>
