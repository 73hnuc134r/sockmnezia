<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import { flag } from "../lib/format";
  import { errorText } from "../lib/errors";
  import Icon from "../components/Icon.svelte";
  import Ping from "../components/Ping.svelte";
  import Tag from "../components/Tag.svelte";
  import ServerTags from "../components/ServerTags.svelte";
  import ServerEditor from "../components/ServerEditor.svelte";
  import ChainEditor from "../components/ChainEditor.svelte";
  import ShareDialog from "../components/ShareDialog.svelte";
  import Modal from "../components/Modal.svelte";

  // Servers & chains (docs/10 §4.2): two grids, the selected row's editor on the right. Click selects,
  // double-click connects through it. Servers are grouped by host, like the Android list.
  let tab = $state<"servers" | "chains">("servers");
  let selected = $state<string | null>(null); // id, or "" for a new item
  let filter = $state("");
  let sort = $state<"name" | "ping">("name");
  let addOpen = $state(false);
  let sharing = $state<{ kind: "server" | "chain"; id: string; name: string; withAmnezia: boolean } | null>(null);
  let deleting = $state<{ kind: "server" | "chain"; id: string; name: string } | null>(null);
  let pasteOpen = $state(false);
  let pasteText = $state("");

  const st = $derived(store.status);
  const q = $derived(filter.trim().toLowerCase());

  function pingKey(l?: engine.TestResult | null): number {
    if (!l) return 1e9 - 1; // untested after tested, failures last
    return l.error ? 1e9 : (l.ms ?? 0);
  }

  const servers = $derived.by(() => {
    const list = store.servers.filter((s) => !q || `${s.name} ${s.address} ${s.protocol}`.toLowerCase().includes(q));
    return [...list].sort((a, b) => (sort === "ping" ? pingKey(a.last) - pingKey(b.last) : 0) || a.name.localeCompare(b.name));
  });
  // Groups by host in the order of their best row, so sorting by ping keeps fast hosts on top.
  const groups = $derived.by(() => {
    const m = new Map<string, engine.ServerView[]>();
    for (const s of servers) {
      const k = s.address || "—";
      if (!m.has(k)) m.set(k, []);
      m.get(k)!.push(s);
    }
    return [...m.entries()];
  });
  const chains = $derived.by(() => {
    const list = store.chains.filter((c) => !q || `${c.name} ${(c.hops ?? []).map((h) => h.name).join(" ")}`.toLowerCase().includes(q));
    return [...list].sort((a, b) => (sort === "ping" ? pingKey(a.last) - pingKey(b.last) : 0) || a.name.localeCompare(b.name));
  });

  const selServer = $derived(tab === "servers" && selected ? store.servers.find((s) => s.id === selected) : undefined);
  const selChain = $derived(tab === "chains" && selected ? store.chains.find((c) => c.id === selected) : undefined);

  function switchTab(to: "servers" | "chains") {
    tab = to;
    selected = null;
  }

  async function connectServer(id: string) {
    await store.act(() => Core.SetActiveServer(id));
  }
  async function connectChain(id: string) {
    await store.act(() => Core.SetActiveChain(id));
  }

  function testAll() {
    if (tab === "servers") void store.testAll(servers.map((s) => ({ chain: "", server: s.id })));
    else void store.testAll(chains.filter((c) => !c.error).map((c) => ({ chain: c.id, server: "" })));
  }
  const anyTesting = $derived(Object.keys(store.testing).length > 0);

  async function confirmDelete() {
    const d = deleting!;
    deleting = null;
    const ok = await store.act(() => (d.kind === "server" ? Core.RemoveServer(d.id) : Core.DeleteChain(d.id)).then(() => true));
    if (ok && selected === d.id) selected = null;
  }

  async function importClipboard() {
    addOpen = false;
    const text = (await Clipboard.Text()) ?? "";
    if (text.trim()) store.importText = text;
    else store.toast(t("d_clipboard_empty"), true);
  }

  function exitOf(l?: engine.TestResult | null): string {
    return l && !l.error && l.country ? `${flag(l.country)} ${l.country}` : "";
  }
</script>

<div class="page">
  <div class="row tabs">
    <button class:on={tab === "servers"} onclick={() => switchTab("servers")}>{t("d_tab_servers", store.servers.length)}</button>
    <button class:on={tab === "chains"} onclick={() => switchTab("chains")}>{t("d_tab_chains", store.chains.length)}</button>
  </div>

  <div class="row toolbar">
    <div class="menuwrap">
      {#if tab === "servers"}
        <button class="btn p" onclick={() => (addOpen = !addOpen)}><Icon name="plus" />{t("add")}<Icon name="down" size={12} /></button>
        {#if addOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div class="scrim" onclick={() => (addOpen = false)}></div>
          <div class="menu">
            <button onclick={() => { addOpen = false; selected = ""; }}><Icon name="edit" />{t("d_new_server")}</button>
            <button onclick={importClipboard}><Icon name="import" />{t("import_clipboard")}</button>
            <button onclick={() => { addOpen = false; pasteText = ""; pasteOpen = true; }}><Icon name="link" />{t("import_links")}</button>
          </div>
        {/if}
      {:else}
        <button class="btn p" onclick={() => (selected = "")}><Icon name="plus" />{t("chain_new")}</button>
      {/if}
    </div>
    <button class="btn" disabled={anyTesting} onclick={testAll}><Icon name="test" />{t("test_all")}</button>
    <span class="grow"></span>
    <div class="seg" title={t("d_sort")}>
      <button class:on={sort === "name"} onclick={() => (sort = "name")}>{t("sort_name")}</button>
      <button class:on={sort === "ping"} onclick={() => (sort = "ping")}>{t("sort_ping")}</button>
    </div>
    <input class="filter" placeholder={t("d_filter")} bind:value={filter} />
  </div>

  <div class="split">
    <div class="gridwrap">
      {#if tab === "servers"}
        {#if !store.servers.length}
          <div class="empty">{t("selector_no_servers")} {t("d_paste_hint")}</div>
        {:else}
          <table>
            <thead><tr><th class="mark"></th><th>{t("srv_name")}</th><th>{t("d_col_type")}</th><th>{t("sort_ping")}</th><th>{t("d_col_exit")}</th></tr></thead>
            <tbody>
              {#each groups as [host, rows] (host)}
                <tr class="grp"><td colspan="5"><span class="mono">{host}</span>{#if rows[0].host_country}<span> · {flag(rows[0].host_country)} {rows[0].host_country}</span>{/if}</td></tr>
                {#each rows as s (s.id)}
                  <tr class:sel={selected === s.id} onclick={() => (selected = s.id)} ondblclick={() => connectServer(s.id)}>
                    <td class="mark">{#if st?.server_id === s.id}<span class="dot" title={t("d_selected")}></span>{/if}</td>
                    <td class="name ellipsis" title={s.name}>{s.name}</td>
                    <td><ServerTags protocol={s.protocol} security={s.security} network={s.network} port={s.port} /></td>
                    <td><Ping ms={s.last?.ms ?? 0} error={s.last?.error ?? ""} testing={!!store.testing[`s:${s.id}`]} /></td>
                    <td class="exit" title={s.last?.ip ?? ""}>{exitOf(s.last)}</td>
                  </tr>
                {/each}
              {/each}
            </tbody>
          </table>
        {/if}
      {:else}
        {#if !store.chains.length}
          <div class="empty">{t("chain_none")}</div>
        {:else}
          <table>
            <thead><tr><th class="mark"></th><th>{t("chain_name")}</th><th>{t("d_col_hops")}</th><th>{t("sort_ping")}</th><th>{t("d_col_exit")}</th></tr></thead>
            <tbody>
              {#each chains as c (c.id)}
                <tr class:sel={selected === c.id} onclick={() => (selected = c.id)} ondblclick={() => !c.error && connectChain(c.id)}>
                  <td class="mark">{#if c.active && !st?.server_id}<span class="dot" title={t("d_selected")}></span>{/if}</td>
                  <td class="name ellipsis" title={c.name}>{c.name}</td>
                  <td><div class="hops">
                    {#each c.hops ?? [] as h, i}
                      {#if i > 0}<span class="dim">→</span>{/if}
                      {#if h.kind === "amnezia"}<Tag text="amnezia" color="var(--t-amz)" />{:else}<span class:warnc={h.missing}>{h.missing ? t("hop_missing") : h.name}</span>{/if}
                    {/each}
                    {#if c.error}<span class="warnc"> · {t("chain_cannot_run")}</span>{/if}
                  </div></td>
                  <td><Ping ms={c.last?.ms ?? 0} error={c.last?.error ?? ""} testing={!!store.testing[`c:${c.id}`]} /></td>
                  <td class="exit" title={c.last?.ip ?? ""}>{exitOf(c.last)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    </div>

    <aside class="panel">
      {#if selected === null}
        <div class="hint">{tab === "servers" ? t("d_servers_panel_hint") : t("d_chains_panel_hint")}</div>
      {:else if tab === "servers"}
        <div class="row nowrap">
          <h2 class="grow ellipsis">{selServer?.name ?? t("add_server")}</h2>
          {#if selServer}
            <button class="btn icon flat" title={t("test")} disabled={!!store.testing[`s:${selServer.id}`]} onclick={() => store.test("", selServer.id)}><Icon name="test" /></button>
            <button class="btn icon flat" title={t("share")} onclick={() => (sharing = { kind: "server", id: selServer.id, name: selServer.name, withAmnezia: false })}><Icon name="share" /></button>
            <button class="btn icon flat danger" title={t("delete")} onclick={() => (deleting = { kind: "server", id: selServer.id, name: selServer.name })}><Icon name="trash" /></button>
          {/if}
        </div>
        {#if selServer?.used_in?.length}<div class="hint">{t("d_used_in", selServer.used_in.join(", "))}</div>{/if}
        {#if selServer && st?.server_id !== selServer.id}
          <button class="btn" onclick={() => connectServer(selServer.id)}><Icon name="swap" />{t("d_connect_through_this")}</button>
        {/if}
        {#key selected}
          <ServerEditor id={selected} onsaved={(id) => { store.toast(t("srv_saved")); selected = id; }} oncancel={selected ? undefined : () => (selected = null)} />
        {/key}
      {:else}
        {#key selected}
          <ChainEditor id={selected}
            onsaved={(id) => (selected = id)}
            onshare={(c) => (sharing = { kind: "chain", id: c.id, name: c.name, withAmnezia: (c.hops ?? []).some((h) => h.kind === "amnezia") })}
            ondelete={(c) => (deleting = { kind: "chain", id: c.id, name: c.name })}
            oncancel={() => (selected = null)} />
        {/key}
      {/if}
    </aside>
  </div>
</div>

{#if sharing}<ShareDialog {...sharing} onclose={() => (sharing = null)} />{/if}
{#if deleting}
  <Modal title={deleting.kind === "server" ? t("delete_server_q", deleting.name) : t("chain_delete_q", deleting.name)} onclose={() => (deleting = null)}>
    <div class="muted">{t("d_delete_text")}</div>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (deleting = null)}>{t("cancel")}</button>
      <button class="btn p" onclick={confirmDelete}>{t("delete")}</button>
    {/snippet}
  </Modal>
{/if}
{#if pasteOpen}
  <Modal title={t("import_links")} onclose={() => (pasteOpen = false)} width={600}>
    <textarea rows="10" bind:value={pasteText} placeholder={t("server_link")}></textarea>
    <div class="hint">{t("srv_link_hint")}</div>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (pasteOpen = false)}>{t("cancel")}</button>
      <button class="btn p" disabled={!pasteText.trim()} onclick={() => { pasteOpen = false; store.importText = pasteText; }}>{t("d_next")}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .page { display: grid; gap: 12px; grid-template-rows: auto auto 1fr; height: 100%; min-height: 0; }
  .tabs { gap: 18px; border-bottom: 1px solid var(--line); }
  .tabs button { padding: 8px 2px; color: var(--muted); background: transparent; border: 0; border-bottom: 2px solid transparent; cursor: pointer; }
  .tabs button.on { color: var(--ink); border-bottom-color: var(--accent); }
  .toolbar { gap: 8px; }
  .filter { width: 200px; }
  .menuwrap { position: relative; }
  .scrim { position: fixed; inset: 0; z-index: 9; }
  .menu { position: absolute; top: 38px; left: 0; z-index: 10; background: var(--card); border: 1px solid var(--line); border-radius: 10px; padding: 5px; box-shadow: var(--shadow); display: grid; min-width: 230px; }
  .menu button { display: flex; gap: 10px; align-items: center; padding: 8px 10px; border-radius: 6px; background: transparent; border: 0; cursor: pointer; text-align: left; }
  .menu button:hover { background: var(--hover); }
  .split { display: grid; grid-template-columns: 1fr 320px; gap: 14px; min-height: 0; }
  .gridwrap { overflow: auto; min-width: 0; border: 1px solid var(--line); border-radius: 12px; background: var(--card); }
  table { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; table-layout: fixed; }
  th { text-align: left; font: 500 11px/1 var(--ui); color: var(--dim); text-transform: uppercase; letter-spacing: 0.06em; padding: 10px; border-bottom: 1px solid var(--line); position: sticky; top: 0; background: var(--card); z-index: 1; }
  th:nth-child(2) { width: 30%; } th:nth-child(4) { width: 92px; } th:nth-child(5) { width: 64px; }
  td { padding: 8px 10px; border-bottom: 1px solid var(--line2); vertical-align: middle; }
  tbody tr:not(.grp) { cursor: default; }
  tbody tr:not(.grp):hover td { background: var(--hover); }
  tr.sel td, tr.sel:hover td { background: var(--sel) !important; }
  tr.grp td { padding: 10px 10px 3px; font-size: 11px; color: var(--dim); border-bottom: 0; }
  .mark { width: 26px; padding-right: 0; }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--ok); }
  .name { font-weight: 500; }
  .hops { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
  .exit { white-space: nowrap; }
  .empty { padding: 30px; text-align: center; color: var(--dim); }
  .panel { background: var(--card); border: 1px solid var(--line); border-radius: 12px; padding: 14px; display: grid; gap: 10px; align-content: start; overflow: auto; min-height: 0; }
</style>
