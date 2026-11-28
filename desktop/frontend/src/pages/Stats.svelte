<script lang="ts">
  import { onDestroy } from "svelte";
  import { Core, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import { bytes } from "../lib/format";
  import Modal from "../components/Modal.svelte";

  // Traffic per day through each chain or server and directly; recorded by the core, kept on this computer.
  let period = $state(7);
  let days = $state<engine.StatsDay[]>([]);
  let resetting = $state(false);
  let hovered = $state<number | null>(null);
  const dayLabel = (date: string) =>
    new Date(date + "T12:00:00").toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });

  async function load() {
    days = (await Core.Stats(period)) ?? [];
  }
  $effect(() => {
    void period;
    void load();
  });
  const timer = setInterval(load, 5000);
  onDestroy(() => clearInterval(timer));

  const all = $derived(days.flatMap((d) => d.entries ?? []));
  const sent = $derived(all.reduce((a, e) => a + e.up, 0));
  const received = $derived(all.reduce((a, e) => a + e.down, 0));
  const direct = $derived(all.filter((e) => e.kind === "direct").reduce((a, e) => a + e.up + e.down, 0));
  const byKey = $derived.by(() => {
    const m = new Map<string, { e: engine.StatsEntry; up: number; down: number }>();
    for (const e of all) {
      const cur = m.get(e.key) ?? { e, up: 0, down: 0 };
      cur.e = e;
      cur.up += e.up;
      cur.down += e.down;
      m.set(e.key, cur);
    }
    return [...m.values()].sort((a, b) => b.up + b.down - (a.up + a.down));
  });
  const totals = $derived(days.map((d) => {
    const es = d.entries ?? [];
    const total = es.reduce((a, e) => a + e.up + e.down, 0);
    const dir = es.filter((e) => e.kind === "direct").reduce((a, e) => a + e.up + e.down, 0);
    const up = es.reduce((a, e) => a + e.up, 0);
    const down = es.reduce((a, e) => a + e.down, 0);
    return { date: d.date, total, dir, up, down };
  }));
  const max = $derived(Math.max(1, ...totals.map((x) => x.total)));
  const name = (e: engine.StatsEntry) =>
    e.kind === "direct" ? t("stats_direct_name") : e.deleted ? t("stats_deleted", e.name) : e.name;

  async function reset() {
    resetting = false;
    await store.act(() => Core.ResetStats());
    await load();
  }
</script>

<div class="page">
  <div class="row">
    <h1 class="grow">{t("title_stats")}</h1>
    <div class="seg">
      {#each [[1, "stats_today"], [7, "stats_7d"], [30, "stats_30d"]] as [d, label]}
        <button class:on={period === d} onclick={() => (period = d as number)}>{t(label as string)}</button>
      {/each}
    </div>
  </div>

  <div class="card">
    <div class="totals">
      <div><span class="cap">{t("stats_received")}</span><b>{bytes(received)}</b></div>
      <div><span class="cap">{t("stats_sent")}</span><b>{bytes(sent)}</b></div>
    </div>
    <div class="row legend">
      <span><i class="dot chain"></i>{t("stats_through_chain", bytes(sent + received - direct))}</span>
      <span><i class="dot direct"></i>{t("stats_direct", bytes(direct))}</span>
    </div>
    {#if period > 1}
      <div class="detail">
        {#if hovered !== null && totals[hovered]}
          {@const x = totals[hovered]}
          <div class="row"><b>{dayLabel(x.date)}</b><span class="mono">↓ {bytes(x.down)} ↑ {bytes(x.up)}</span></div>
          <div class="row legend">
            <span><i class="dot chain"></i>{t("stats_through_chain", bytes(x.total - x.dir))}</span>
            <span><i class="dot direct"></i>{t("stats_direct", bytes(x.dir))}</span>
          </div>
        {:else}
          <span class="hint">{t("d_stats_hover_hint")}</span>
        {/if}
      </div>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <svg class="chart" onmouseleave={() => (hovered = null)} viewBox="0 0 {totals.length * 10} 100" preserveAspectRatio="none" aria-hidden="true">
        {#each totals as x, i}
          {@const h = (x.total / max) * 96}
          {@const hd = (x.dir / max) * 96}
          <g class:dim={hovered !== null && hovered !== i}>
            {#if x.total > 0}
              <rect x={i * 10 + 1.5} y={100 - h} width="7" height={h - hd} rx="1" class="chain" />
              {#if hd > 0}<rect x={i * 10 + 1.5} y={100 - hd} width="7" height={hd} rx="1" class="direct" />{/if}
            {:else}
              <rect x={i * 10 + 1.5} y="99" width="7" height="1" class="none" />
            {/if}
          </g>
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <rect x={i * 10} y="0" width="10" height="100" class="hit" onmouseenter={() => (hovered = i)} />
        {/each}
      </svg>
      <div class="row axis"><span class="grow">{totals[0]?.date.slice(5)}</span><span>{totals.at(-1)?.date.slice(5)}</span></div>
    {/if}
  </div>

  <div class="card">
    <h3>{t("stats_by_endpoint")}</h3>
    {#if !byKey.length}<div class="hint">{t("stats_empty")}</div>{/if}
    {#each byKey as x (x.e.key)}
      <div class="row nowrap item">
        <i class="dot" class:direct={x.e.kind === "direct"} class:chain={x.e.kind !== "direct"}></i>
        <span class="grow ellipsis">{name(x.e)}</span>
        <span class="mono">↓ {bytes(x.down)}</span>
        <span class="mono">↑ {bytes(x.up)}</span>
      </div>
    {/each}
  </div>
  <div class="row">
    <span class="hint grow">{t("stats_hint")}</span>
    <button class="btn flat danger" onclick={() => (resetting = true)}>{t("stats_reset")}</button>
  </div>
</div>

{#if resetting}
  <Modal title={t("stats_reset_q")} onclose={() => (resetting = false)}>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (resetting = false)}>{t("cancel")}</button>
      <button class="btn p" onclick={reset}>{t("delete")}</button>
    {/snippet}
    <div></div>
  </Modal>
{/if}

<style>
  .page { display: grid; gap: 14px; align-content: start; max-width: 920px; }
  .totals { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .totals div { display: grid; gap: 4px; }
  .totals b { font-size: 22px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .legend { gap: 18px; color: var(--muted); font-size: 12px; }
  .dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 6px; flex: none; }
  .dot.chain, rect.chain { background: var(--accent); fill: var(--accent); }
  .dot.direct, rect.direct { background: var(--ok); fill: var(--ok); }
  rect.none { fill: var(--line); }
  rect.hit { fill: transparent; cursor: default; }
  g.dim { opacity: 0.35; }
  .detail { min-height: 48px; display: grid; gap: 2px; align-content: center; background: var(--card2); border-radius: 8px; padding: 6px 10px; }
  .detail .row { gap: 12px; }
  .chart { width: 100%; height: 140px; display: block; }
  .axis { font-size: 11px; color: var(--dim); }
  .item { padding: 4px 0; border-bottom: 1px solid var(--line2); }
  .item .mono { min-width: 90px; text-align: right; }
</style>
