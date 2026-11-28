<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import { countryName } from "../lib/format";
  import { errorText } from "../lib/errors";
  import Icon from "../components/Icon.svelte";
  import Ping from "../components/Ping.svelte";
  import ServerTags from "../components/ServerTags.svelte";
  import ServerEditor from "../components/ServerEditor.svelte";
  import ChainEditor from "../components/ChainEditor.svelte";
  import ShareDialog from "../components/ShareDialog.svelte";
  import Modal from "../components/Modal.svelte";

  // Servers & chains: a Chains island, then one island per host with its servers (like the Android list); the
  // selected row's editor on the right. Click selects, double-click connects through it.
  let selKind = $state<"server" | "chain">("server");
  let selected = $state<string | null>(null); // id, or "" for a new item
  let filter = $state("");
  let sort = $state<"name" | "ping">("name");
  let addOpen = $state(false);
  let sharing = $state<{ kind: "server" | "chain"; id: string; name: string; withAmnezia: boolean } | null>(null);
  let deleting = $state<{ kind: "server" | "chain"; id: string; name: string } | null>(null);
  let pasteOpen = $state(false);
  let sharingAll = $state(false);
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

  const selServer = $derived(selKind === "server" && selected ? store.servers.find((s) => s.id === selected) : undefined);
  const amneziaReady = $derived(!!st?.has_key && !!st?.has_gateway_key);

  function pick(kind: "server" | "chain", id: string) {
    selKind = kind;
    selected = id;
  }
  const initial = (name: string) => (name.trim()[0] ?? "?").toUpperCase();
  /** The chain's protocols, e.g. "VLESS · Trojan". */
  const protos = (c: engine.ChainInfo) =>
    [...new Set((c.hops ?? []).filter((h) => h.protocol).map((h) => (h.protocol === "shadowsocks" ? "SS" : h.protocol!.toUpperCase())))].join(" · ");
  const exitCC = (l?: engine.TestResult | null) => (l && !l.error ? (l.country ?? "") : "");

  async function connectServer(id: string) {
    await store.act(() => Core.SetActiveServer(id));
  }
  async function connectChain(id: string) {
    await store.act(() => Core.SetActiveChain(id));
  }

  function testAll() {
    void store.testAll([
      ...chains.filter((c) => !c.error).map((c) => ({ chain: c.id, server: "" })),
      ...servers.map((s) => ({ chain: "", server: s.id })),
    ]);
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

</script>

<div class="page">
  <div class="row nowrap head">
    <h1 class="grow">{t("d_nav_servers")}</h1>
    <div class="seg" title={t("d_sort")}>
      <button class:on={sort === "name"} onclick={() => (sort = "name")}>{t("sort_name")}</button>
      <button class:on={sort === "ping"} onclick={() => (sort = "ping")}>{t("sort_ping")}</button>
    </div>
    <input class="filter" placeholder={t("d_filter")} bind:value={filter} />
  </div>

  <div class="split">
    <div class="lists">
      <section class="island">
        <div class="row nowrap"><span class="cap grow">{t("d_chains_title")}</span><span class="dim small">{chains.length}</span></div>
        {#if !store.chains.length}
          <div class="hint">{t("chain_none")}</div>
        {:else}
          <div class="rows">
            {#each chains as c (c.id)}
              {@const on = c.active && !st?.server_id}
              <button class="li" class:active={on} class:sel={selKind === "chain" && selected === c.id}
                onclick={() => pick("chain", c.id)} ondblclick={() => !c.error && connectChain(c.id)}>
                <span class="av" class:avon={on}>{#if on}<Icon name="check" size={18} />{:else}{initial(c.name)}{/if}</span>
                <span class="grow txt">
                  <span class="ellipsis nm">{c.name}</span>
                  <span class="sub">
                    {#each c.hops ?? [] as h, i}
                      {#if i > 0}<span class="dim">→</span>{/if}
                      {#if h.missing}<span class="warnc">{t("hop_missing")}</span>
                      {:else if h.country}<span class="cc">{h.country.toUpperCase()}</span>
                      {:else}<span class="ellipsis hn">{h.kind === "amnezia" ? "Amnezia" : h.name}</span>{/if}
                    {/each}
                    {#if protos(c)}<span class="dim pr">{protos(c)}</span>{/if}
                    {#if c.error}<span class="warnc">· {t("chain_cannot_run")}</span>{/if}
                  </span>
                </span>
                <Ping cc={exitCC(c.last)} ms={c.last?.ms ?? 0} error={c.last?.error ?? ""} testing={!!store.testing[`c:${c.id}`]} title={c.last?.ip ?? ""} />
              </button>
            {/each}
          </div>
        {/if}
      </section>

      {#if !store.servers.length}
        <section class="island"><div class="hint">{t("selector_no_servers")} {t("d_paste_hint")}</div></section>
      {/if}
      {#each groups as [host, rows] (host)}
        <section class="island">
          <div class="row nowrap hosthead">
            <span class="hicon"><Icon name="servers" size={18} /></span>
            <span class="grow row nowrap"><span class="mono ellipsis hostname selectable">{host}</span><span class="pill">{rows.length}</span></span>
            {#if rows[0].host_country}<span class="ccpill"><span class="cc">{rows[0].host_country.toUpperCase()}</span>{countryName(rows[0].host_country)}</span>{/if}
          </div>
          <div class="group">
            {#each rows as s (s.id)}
              {@const on = st?.server_id === s.id}
              <button class="item li2" class:active={on} class:sel={selKind === "server" && selected === s.id}
                onclick={() => pick("server", s.id)} ondblclick={() => connectServer(s.id)}>
                <span class="grow txt">
                  <span class="row nowrap"><span class="ellipsis nm">{s.name}</span>{#if on}<Icon name="check" size={16} />{/if}</span>
                  <ServerTags protocol={s.protocol} security={s.security} network={s.network} port={s.port} />
                </span>
                <Ping cc={s.last && !s.last.error && s.last.country && s.last.country !== s.host_country ? s.last.country : ""} ms={s.last?.ms ?? 0} error={s.last?.error ?? ""} testing={!!store.testing[`s:${s.id}`]} title={s.last?.ip ?? ""} />
              </button>
            {/each}
          </div>
        </section>
      {/each}

      {#if amneziaReady}
        <section class="island">
          <div class="row nowrap hosthead">
            <span class="hicon amz"><Icon name="globe" size={18} /></span>
            <span class="grow nm">Amnezia Premium</span>
            {#if st?.country}<span class="ccpill"><span class="cc">{st.country.toUpperCase()}</span>{st.country_name || countryName(st.country)}</span>{/if}
            <button class="btn flat" onclick={() => (store.page = "settings")}>{t("amnezia_settings")}</button>
          </div>
        </section>
      {/if}

      <div class="fabs">
        <button class="fab sq" title={t("test_all")} disabled={anyTesting || (!store.servers.length && !store.chains.length)} onclick={testAll}><Icon name="refresh" size={22} /></button>
        <button class="fab sq" title={t("share_all")} disabled={!store.servers.length && !store.chains.length} onclick={() => (sharingAll = true)}><Icon name="share" size={22} /></button>
        <button class="fab" onclick={() => pick("chain", "")}><Icon name="plus" size={20} />{t("chain_new")}</button>
        <div class="menuwrap">
          <button class="fab p" onclick={() => (addOpen = !addOpen)}><Icon name="plus" size={20} />{t("add_server")}</button>
          {#if addOpen}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="scrim" onclick={() => (addOpen = false)}></div>
            <div class="menu up">
              <button onclick={() => { addOpen = false; pick("server", ""); }}><Icon name="edit" />{t("d_new_server")}</button>
              <button onclick={importClipboard}><Icon name="import" />{t("import_clipboard")}</button>
              <button onclick={() => { addOpen = false; pasteText = ""; pasteOpen = true; }}><Icon name="link" />{t("import_links")}</button>
            </div>
          {/if}
        </div>
      </div>
    </div>

    <aside class="panel island">
      {#if selected === null}
        <div class="hint">{t("d_servers_panel_hint")} {t("d_chains_panel_hint")}</div>
      {:else if selKind === "server"}
        <div class="row nowrap">
          <h2 class="grow ellipsis">{selServer?.name ?? t("add_server")}</h2>
          {#if selServer}
            <button class="btn icon flat" title={t("test")} disabled={!!store.testing[`s:${selServer.id}`]} onclick={() => store.test("", selServer.id)}><Icon name="test" size={18} /></button>
            <button class="btn icon flat" title={t("share")} onclick={() => (sharing = { kind: "server", id: selServer.id, name: selServer.name, withAmnezia: false })}><Icon name="share" size={18} /></button>
            <button class="btn icon flat danger" title={t("delete")} onclick={() => (deleting = { kind: "server", id: selServer.id, name: selServer.name })}><Icon name="trash" size={18} /></button>
          {/if}
        </div>
        {#if selServer?.used_in?.length}<div class="hint">{t("d_used_in", selServer.used_in.join(", "))}</div>{/if}
        {#if selServer && st?.server_id !== selServer.id}
          <button class="btn" onclick={() => connectServer(selServer.id)}><Icon name="swap" size={18} />{t("d_connect_through_this")}</button>
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
{#if sharingAll}<ShareDialog kind="all" onclose={() => (sharingAll = false)} />{/if}
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
  .page { display: grid; gap: 16px; grid-template-rows: auto 1fr; height: 100%; min-height: 0; }
  .head { gap: 12px; }
  .filter { width: 220px; border-radius: 20px; padding: 0 16px; background: var(--s); border-color: transparent; }
  .split { display: grid; grid-template-columns: minmax(0, 1fr) minmax(300px, 380px); gap: 16px; min-height: 0; }
  .lists { overflow: auto; min-width: 0; display: flex; flex-direction: column; gap: 12px; padding-right: 2px; }
  .lists > .island { background: var(--s); padding: 14px 10px 10px; gap: 8px; }
  .lists > .island > .row, .hosthead { padding: 0 8px; }
  .small { font-size: 12px; }
  .rows { display: grid; gap: 2px; }
  .li, .li2 { border: 0; color: inherit; text-align: left; cursor: pointer; display: flex; gap: 12px; align-items: center; width: 100%; font: inherit; }
  .li { background: transparent; border-radius: var(--r-item); padding: 10px 12px; }
  .li:hover, .li2:hover { box-shadow: inset 0 0 0 100px var(--state); }
  .li.active { background: var(--primary-c); color: var(--on-primary-c); }
  .li2.active { color: var(--primary); }
  .sel { outline: 2px solid var(--primary); outline-offset: -2px; }
  .av { width: 36px; height: 36px; flex: none; border-radius: 18px; background: var(--secondary-c); color: var(--on-secondary-c); display: grid; place-items: center; font-weight: 600; }
  .av.avon { background: var(--primary); color: var(--on-primary); }
  .txt { display: grid; gap: 4px; min-width: 0; }
  .nm { font-size: 15px; font-weight: 500; }
  .sub { display: flex; gap: 4px; align-items: center; font-size: 12px; flex-wrap: wrap; min-width: 0; }
  .hn { max-width: 140px; }
  .pr { margin-left: 4px; }
  .hicon { width: 32px; height: 32px; flex: none; border-radius: 10px; background: var(--s-high); color: var(--muted); display: grid; place-items: center; }
  .hicon.amz { background: var(--tertiary-c); color: var(--on-tertiary-c); }
  .hostname { font-size: 13.5px; font-weight: 500; }
  .ccpill { display: inline-flex; gap: 5px; align-items: center; font: 500 12px/16px var(--ui); background: var(--s-high); color: var(--muted); border-radius: 8px; padding: 3px 8px 3px 4px; white-space: nowrap; }
  .panel { overflow: auto; min-height: 0; align-content: start; }
  .fabs { margin-top: auto; }
</style>
