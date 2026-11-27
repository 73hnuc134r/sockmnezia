<script lang="ts">
  // A 60-second sparkline (values oldest first).
  let { values, color }: { values: number[]; color: string } = $props();
  const W = 200, H = 38;
  const pts = $derived.by(() => {
    if (values.length < 2) return "";
    const max = Math.max(1, ...values);
    const step = W / (values.length - 1);
    return values.map((v, i) => `${(i * step).toFixed(1)},${(H - 3 - (v / max) * (H - 8)).toFixed(1)}`).join(" ");
  });
</script>

<svg viewBox="0 0 {W} {H}" preserveAspectRatio="none" aria-hidden="true">
  {#if pts}
    <polygon points="0,{H} {pts} {W},{H}" fill={color} opacity="0.16" />
    <polyline points={pts} fill="none" stroke={color} stroke-width="1.5" vector-effect="non-scaling-stroke" />
  {:else}
    <line x1="0" y1={H - 3} x2={W} y2={H - 3} stroke="var(--line)" stroke-width="1" />
  {/if}
</svg>

<style>
  svg { width: 100%; height: 38px; display: block; }
</style>
