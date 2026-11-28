<script lang="ts">
  import { onDestroy } from "svelte";
  import { Clipboard } from "@wailsio/runtime";
  import { Core, type desk } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import Icon from "../components/Icon.svelte";

  // The core's log (Xray and app lines), newest last; secrets and addresses are masked by the core.
  let lines = $state<desk.LogLine[]>([]);
  let filter = $state("");
  let follow = $state(true);
  let box: HTMLDivElement | undefined = $state();

  async function load() {
    lines = (await Core.Logs()) ?? [];
    if (follow && box) requestAnimationFrame(() => box && (box.scrollTop = box.scrollHeight));
  }
  void load();
  const timer = setInterval(load, 2000);
  onDestroy(() => clearInterval(timer));

  const shown = $derived(filter ? lines.filter((l) => l.msg.toLowerCase().includes(filter.toLowerCase())) : lines);
  const time = (ms: number) => new Date(ms).toLocaleTimeString();
  const text = () => shown.map((l) => `${new Date(l.t).toISOString()} ${l.level} ${l.msg}`).join("\n");
</script>

<div class="page">
  <div class="row">
    <h1 class="grow">{t("title_logs")}</h1>
    <input class="filter" placeholder={t("search")} bind:value={filter} />
    <label class="check"><input type="checkbox" bind:checked={follow} /><span>{t("d_follow")}</span></label>
    <button class="btn" onclick={async () => { await Clipboard.SetText(text()); store.toast(t("share_copied")); }}><Icon name="copy" />{t("copy")}</button>
    <button class="btn" onclick={async () => { await Core.ClearLogs(); await load(); }}><Icon name="trash" />{t("clear")}</button>
  </div>
  <div class="hint">{t("logs_hint")}</div>
  <div class="log selectable" bind:this={box}>
    {#each shown as l}
      <div class="line {l.level}"><span class="dim">{time(l.t)}</span> <span class="lvl">{l.level}</span> {l.msg}</div>
    {/each}
  </div>
</div>

<style>
  .page { display: grid; grid-template-rows: auto auto 1fr; gap: 10px; height: 100%; min-height: 0; }
  .filter { width: 220px; }
  .log { overflow: auto; background: var(--s); border-radius: var(--r-island); padding: 16px 20px; font: 12px/1.55 var(--mono); min-height: 0; }
  .line { white-space: pre-wrap; word-break: break-word; }
  .lvl { display: inline-block; width: 60px; color: var(--dim); }
  .error .lvl, .Error .lvl { color: var(--warn); }
  .warning .lvl, .Warning .lvl { color: var(--amz); }
</style>
