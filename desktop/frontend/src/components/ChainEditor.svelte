<script lang="ts">
  import { Core, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import { errorText } from "../lib/errors";
  import Icon from "./Icon.svelte";
  import Tag from "./Tag.svelte";
  import ServerTags from "./ServerTags.svelte";

  // Builds or edits a chain (id = "" for a new one): ordered hops, this PC first, exit last; drag to reorder.
  // Validated live by the core. An empty name follows the hops (the core keeps it current).
  let { id, onsaved, onshare, ondelete, oncancel }: {
    id: string;
    onsaved: (id: string) => void;
    onshare: (c: engine.ChainInfo) => void;
    ondelete: (c: engine.ChainInfo) => void;
    oncancel: () => void;
  } = $props();

  const MaxHops = 5; // xrayconf.MaxHops
  const saved = $derived(store.chains.find((c) => c.id === id));
  let name = $state("");
  let hops = $state<engine.ChainHop[]>([]);
  let error = $state<engine.Error | null>(null);
  let picking = $state(false);
  let dragFrom = $state<number | null>(null);
  let dragOver = $state<number | null>(null);

  // Loaded once per chain (the parent re-creates the editor for another id).
  const initial = store.chains.find((c) => c.id === id);
  if (initial) {
    name = initial.auto_name ? "" : initial.name;
    hops = (initial.hops ?? []).map((h) => ({ kind: h.kind, server: h.server ?? "" }));
  }

  $effect(() => {
    const c: engine.Chain = { id, name, hops: $state.snapshot(hops) as engine.ChainHop[] };
    void store.servers; // revalidate when servers change
    Core.ValidateChain(c).then((e) => (error = e ?? null));
  });

  const st = $derived(store.status);
  const amneziaReady = $derived(!!st?.has_key && !!st?.has_gateway_key);

  function info(h: engine.ChainHop): engine.HopInfo {
    if (h.kind === "amnezia") return { kind: "amnezia", name: st?.country_name || "Amnezia", country: st?.country ?? "", protocol: "", network: "", security: "" } as engine.HopInfo;
    const s = store.servers.find((x) => x.id === h.server);
    return { kind: "server", server: h.server, name: s?.name ?? "", protocol: s?.protocol ?? "", network: s?.network ?? "", security: s?.security ?? "", port: s?.port ?? 0, missing: !s } as engine.HopInfo;
  }
  const autoName = $derived(hops.map((h) => { const i = info(h); return i.missing ? t("hop_missing") : i.name; }).join(" → "));

  function move(from: number, to: number) {
    if (from === to || to < 0 || to >= hops.length) return;
    const l = [...hops];
    const [x] = l.splice(from, 1);
    l.splice(to, 0, x);
    hops = l;
  }

  function add(h: engine.ChainHop) {
    hops = [...hops, h];
    picking = false;
  }

  async function save(use: boolean) {
    const c: engine.Chain = { id, name, hops: $state.snapshot(hops) as engine.ChainHop[] };
    const newID = await store.act(() => Core.SaveChain(c), t("chain_saved"));
    if (!newID) return;
    if (use || !id) await store.act(() => Core.SetActiveChain(newID)); // a chain just built is the one wanted
    onsaved(newID);
  }

  const hasAmnezia = $derived(hops.some((h) => h.kind === "amnezia"));
  const isActive = $derived(!!saved && saved.active && !st?.server_id);
</script>

<div class="ce">
  <div class="row nowrap">
    <h2 class="grow ellipsis">{id ? (saved?.name ?? "") : t("chain_new")}</h2>
    {#if saved}
      <button class="btn icon flat" title={t("test")} disabled={!!store.testing[`c:${id}`] || !!saved.error} onclick={() => store.test(id, "")}><Icon name="test" /></button>
      <button class="btn icon flat" title={t("share")} onclick={() => onshare(saved)}><Icon name="share" /></button>
      <button class="btn icon flat danger" title={t("delete")} onclick={() => ondelete(saved)}><Icon name="trash" /></button>
    {/if}
  </div>

  <div class="field">
    <label for="c-name">{t("chain_name")}</label>
    <input id="c-name" bind:value={name} placeholder={autoName} />
    <span class="hint">{t("chain_name_hint")}</span>
  </div>

  <div class="hint">{t("d_chain_editor_hint")}</div>
  <div class="hops">
    <div class="end"><span class="enddot"></span>{t("d_this_pc")}</div>
    {#each hops as h, i (`${h.kind}:${h.server}:${i}`)}
      {@const hi = info(h)}
      <div class="hop" class:over={dragOver === i && dragFrom !== null && dragFrom !== i} class:dragging={dragFrom === i}
        draggable="true" role="listitem"
        ondragstart={(e) => { dragFrom = i; e.dataTransfer?.setData("text/plain", String(i)); if (e.dataTransfer) e.dataTransfer.effectAllowed = "move"; }}
        ondragover={(e) => { e.preventDefault(); dragOver = i; }}
        ondragleave={() => dragOver === i && (dragOver = null)}
        ondrop={(e) => { e.preventDefault(); if (dragFrom !== null) move(dragFrom, i); dragFrom = null; dragOver = null; }}
        ondragend={() => { dragFrom = null; dragOver = null; }}>
        <span class="handle" title={t("d_drag")}><Icon name="drag" size={14} /></span>
        <span class="dot" class:amz={hi.kind === "amnezia"}>{hi.kind === "amnezia" ? "A" : i + 1}</span>
        <span class="grow info">
          <span class="ellipsis" class:warnc={hi.missing}>{hi.missing ? t("hop_missing") : hi.name}</span>
          {#if hi.kind === "amnezia"}<span><Tag text="amnezia" color="var(--t-amz)" />{#if hi.country}&nbsp;<Tag text={hi.country.toUpperCase()} />{/if}</span>
          {:else if !hi.missing}<ServerTags protocol={hi.protocol} security={hi.security} network={hi.network} port={hi.port ?? 0} />{/if}
        </span>
        <span class="moves">
          <button class="btn icon flat" title={t("chain_move_up")} disabled={i === 0} onclick={() => move(i, i - 1)}><Icon name="up" size={14} /></button>
          <button class="btn icon flat" title={t("chain_move_down")} disabled={i === hops.length - 1} onclick={() => move(i, i + 1)}><Icon name="down" size={14} /></button>
        </span>
        <button class="btn icon flat" title={t("chain_remove_hop")} onclick={() => (hops = hops.filter((_, k) => k !== i))}><Icon name="close" size={14} /></button>
      </div>
      {#if error?.field === `hop${i + 1}`}<div class="hint warn indent">{errorText(error)}</div>{/if}
    {/each}
    <div class="end"><span class="enddot"></span>{t("chain_internet")}</div>
  </div>
  {#if error && hops.length && !error.field?.startsWith("hop")}<div class="hint warn">{errorText(error)}</div>{/if}

  <div class="pickwrap">
    <button class="btn" disabled={hops.length >= MaxHops} onclick={() => (picking = !picking)}><Icon name="plus" />{t("chain_add_hop")}</button>
    {#if picking}
      <div class="picker">
        <button disabled={hasAmnezia || !amneziaReady} onclick={() => add({ kind: "amnezia", server: "" })}>
          <Tag text="amnezia" color="var(--t-amz)" /><span class="grow ellipsis">{t("amnezia")}</span>
          {#if hasAmnezia}<span class="dim note">{t("hop_amnezia_used")}</span>{:else if !amneziaReady}<span class="dim note">{t("hop_amnezia_not_set_up")}</span>{/if}
        </button>
        {#each [...store.servers].sort((a, b) => a.name.localeCompare(b.name)) as s (s.id)}
          <button onclick={() => add({ kind: "server", server: s.id })}>
            <span class="grow ellipsis">{s.name}</span><ServerTags protocol={s.protocol} security={s.security} network={s.network} port={s.port} />
          </button>
        {/each}
        {#if !store.servers.length}<div class="dim pad">{t("selector_no_servers")}</div>{/if}
      </div>
    {/if}
  </div>

  <div class="row">
    {#if !id}<button class="btn flat" onclick={oncancel}>{t("cancel")}</button>{/if}
    <span class="grow"></span>
    {#if id && !isActive}<button class="btn" disabled={!!error || !hops.length} onclick={() => save(true)}>{t("chain_use")}</button>{/if}
    <button class="btn p" disabled={!!error || !hops.length} onclick={() => save(false)}>{t("save")}</button>
  </div>
</div>

<style>
  .ce { display: grid; gap: 10px; align-content: start; }
  .hops { display: grid; gap: 2px; }
  .end { display: flex; gap: 10px; align-items: center; color: var(--dim); font-size: 12px; padding: 4px 6px; }
  .enddot { width: 8px; height: 8px; border-radius: 50%; background: var(--dim); margin: 0 9px 0 31px; }
  .hop { display: flex; gap: 8px; align-items: center; padding: 6px; border-radius: 8px; border: 1px solid transparent; background: var(--card2); }
  .hop.over { border-color: var(--accent); }
  .hop.dragging { opacity: 0.4; }
  .handle { cursor: grab; color: var(--dim); display: grid; place-items: center; width: 18px; }
  .dot { width: 24px; height: 24px; flex: none; border-radius: 50%; display: grid; place-items: center; font: 600 11px/1 var(--ui); border: 2px solid var(--accent); color: var(--accent); }
  .dot.amz { border-color: var(--amz); color: var(--amz); }
  .info { display: grid; gap: 2px; min-width: 0; }
  .moves { display: flex; }
  .indent { padding-left: 58px; }
  .pickwrap { position: relative; }
  .picker { margin-top: 6px; background: var(--card); border: 1px solid var(--line); border-radius: 10px; padding: 5px; box-shadow: var(--shadow); display: grid; max-height: 300px; overflow: auto; }
  .picker button { display: flex; gap: 8px; align-items: center; padding: 7px 9px; border-radius: 6px; background: transparent; border: 0; cursor: pointer; text-align: left; min-width: 0; }
  .picker button:hover:not(:disabled) { background: var(--hover); }
  .picker button:disabled { opacity: 0.5; cursor: default; }
  .pad { padding: 8px; }
  .note { font-size: 11px; white-space: nowrap; }
</style>
