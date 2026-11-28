<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, Desktop, type engine, type routing } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import { errorText } from "../lib/errors";
  import { bytes, date } from "../lib/format";
  import Icon from "../components/Icon.svelte";
  import Tag from "../components/Tag.svelte";
  import Modal from "../components/Modal.svelte";
  import Switch from "../components/Switch.svelte";
  import RuleEditor from "../components/RuleEditor.svelte";

  // Routing (plan §5.3): rules top to bottom, first match wins; locked security rules first. The rule
  // list is saved as a whole (Core.SetSettings), like on Android.
  type RoutingSettings = { domainStrategy: string; rules: routing.Rule[] };
  let settings = $state<engine.Settings | null>(null);
  let locked = $state<routing.Rule[]>([]);
  let presets = $state<{ id: string; rules: routing.Rule[] | null }[]>([]);
  let editing = $state<number | null>(null); // index, or -1 for a new rule
  let presetFor = $state<string | null>(null);
  let presetsOpen = $state(false);
  let dragFrom = $state<number | null>(null);
  let dragOver = $state<number | null>(null);
  let geo = $state<{ has_geoip: boolean; has_geosite: boolean; updated: number } | null>(null);
  let sources = $state<{ id: string; name: string }[]>([]);
  let progress = $state<{ file: string; index: number; count: number; done: number; total: number; verifying: boolean } | null>(null);
  let geoBusy = $state(false);

  async function load() {
    settings = await Core.Settings();
    geo = await Core.GeoStatus();
  }
  $effect(() => {
    void store.chains;
    void load();
  });
  Core.LockedRules().then((l) => (locked = l ?? []));
  Core.Presets().then((p) => (presets = p ?? []));
  Core.GeoSources().then((s) => (sources = s ?? []));

  const rt = $derived((settings?.routing ?? { domainStrategy: "AsIs", rules: [] }) as RoutingSettings);
  const rules = $derived(rt.rules ?? []);

  async function saveRouting(next: Partial<RoutingSettings>) {
    if (!settings) return false;
    const ns = structuredClone($state.snapshot(settings)) as engine.Settings;
    ns.routing = { ...rt, ...next };
    const ok = await store.act(() => Core.SetSettings(ns).then(() => true));
    await load();
    return !!ok;
  }
  const saveRules = (l: routing.Rule[]) => saveRouting({ rules: l });

  function move(from: number, to: number) {
    if (from === to) return;
    const l = [...rules];
    const [x] = l.splice(from, 1);
    l.splice(to, 0, x);
    void saveRules(l);
  }

  const presetName = (id: string) =>
    t(({ global: "preset_global", bypass_ru: "preset_bypass_ru", block_ads: "preset_block_ads", bypass_bittorrent: "preset_bypass_bt", block_quic: "preset_block_quic" } as Record<string, string>)[id] ?? "preset_unknown");
  async function applyPreset(replace: boolean) {
    const p = presets.find((x) => x.id === presetFor)?.rules ?? [];
    presetFor = null;
    await saveRules(replace ? p : [...rules, ...p.filter((n) => !rules.some((r) => r.id && r.id === n.id))]);
  }

  async function importClipboard() {
    const text = (await Clipboard.Text()) ?? "";
    const r = await store.act(() => Core.ParseRouting(text));
    if (r && (await saveRouting(r as RoutingSettings))) store.toast(t("rules_imported"));
  }
  async function exportRules() {
    await Clipboard.SetText(JSON.stringify(rt, null, 2));
    store.toast(t("rules_copied"));
  }

  /** A rule's conditions as short chips (domains, IPs, port, network, detected protocol). */
  const conds = (r: routing.Rule) => {
    const l = [...(r.domain ?? []), ...(r.ip ?? []), r.port ? String(r.port) : "", r.network ?? "", ...(r.protocol ?? [])].filter(Boolean);
    return l.length > 6 ? [...l.slice(0, 6), `+${l.length - 6}`] : l;
  };
  const outTone = (o: string) => (o === "direct" ? "blue" : o === "block" ? "red" : "teal") as "blue" | "red" | "teal";

  // Route tester.
  let host = $state("");
  let tport = $state("443");
  let udp = $state(false);
  let verdict = $state<{ out: string; text: string } | null>(null);
  async function testRoute() {
    try {
      const v = await Core.TestRoute(host.trim(), Number(tport) || 443, udp ? "udp" : "tcp");
      const by = v.locked ? t("verdict_locked") : v.rule_index < 0 ? t("verdict_default") : v.remarks || t("rule_n", v.rule_index - locked.length + 1);
      verdict = { out: v.outbound, text: by };
    } catch (e) {
      verdict = { out: "", text: errorText(e) };
    }
  }

  // Geo files.
  async function updateGeo() {
    geoBusy = true;
    const timer = setInterval(async () => (progress = await Core.GeoProgress()), 400);
    await store.act(() => Core.UpdateGeo(), t("geo_updated"));
    clearInterval(timer);
    progress = null;
    geoBusy = false;
    await load();
  }
  async function setGeo(f: (o: engine.Options) => void) {
    if (!settings) return;
    const ns = structuredClone($state.snapshot(settings)) as engine.Settings;
    f(ns.options);
    await store.act(() => Core.SetSettings(ns));
    await load();
  }
</script>

<div class="page">
  <div class="main col">
    <div class="row nowrap">
      <h1 class="grow">{t("title_routing")}</h1>
      <button class="btn icon flat" title={t("import_clipboard")} onclick={importClipboard}><Icon name="import" size={20} /></button>
      <button class="btn icon flat" title={t("export_clipboard")} onclick={exportRules}><Icon name="copy" size={20} /></button>
    </div>

    <section class="island tester">
      <span class="cap">{t("d_where_goes")}</span>
      <div class="row nowrap">
        <input class="grow q" placeholder={t("host_or_ip")} bind:value={host} onkeydown={(e) => e.key === "Enter" && host.trim() && testRoute()} />
        <input class="mono portin" placeholder={t("port")} bind:value={tport} />
        <label class="check"><input type="checkbox" bind:checked={udp} /><span>UDP</span></label>
        <button class="btn" disabled={!host.trim()} onclick={testRoute}>{t("test")}</button>
      </div>
      {#if verdict}
        <div class="row nowrap verdict">
          {#if verdict.out}<Tag text={t(`out_${verdict.out}`)} tone={outTone(verdict.out)} />{/if}
          <span class="muted">{verdict.text}</span>
        </div>
      {/if}
    </section>

    <section class="island list">
      <span class="cap">{t("d_your_rules")}</span>
      <span class="hint pad">{t("d_routing_hint")}</span>
      {#if rules.length}
        <div class="group">
          {#each rules as r, i (i)}
            <div class="item rule" class:off={r.enabled === false} class:over={dragOver === i && dragFrom !== null && dragFrom !== i} class:sel={editing === i}
              draggable="true" role="listitem"
              ondragstart={(e) => { dragFrom = i; e.dataTransfer?.setData("text/plain", String(i)); }}
              ondragover={(e) => { e.preventDefault(); dragOver = i; }}
              ondrop={(e) => { e.preventDefault(); if (dragFrom !== null) move(dragFrom, i); dragFrom = null; dragOver = null; }}
              ondragend={() => { dragFrom = null; dragOver = null; }}>
              <span class="handle" title={t("d_drag")}><Icon name="drag" size={16} /></span>
              <input type="checkbox" checked={r.enabled !== false} title={t("rule_enabled")}
                onchange={(e) => { const l = structuredClone($state.snapshot(rules)) as routing.Rule[]; l[i].enabled = e.currentTarget.checked ? null : false; void saveRules(l); }} />
              <button class="body" onclick={() => (editing = i)}>
                <span class="ellipsis rn">{r.remarks || t("rule_n", i + 1)}</span>
                <span class="conds">{#each conds(r) as c}<span class="cond">{c}</span>{/each}</span>
              </button>
              <Tag text={t(`out_${r.outboundTag}`)} tone={outTone(r.outboundTag)} />
            </div>
          {/each}
        </div>
      {/if}
    </section>

    <section class="island list">
      <span class="cap">{t("d_everything_else")}</span>
      <div class="group">
        <div class="item"><span class="grow txt"><span>{t("default_route")}</span><span class="dim small">{t("default_route_hint")}</span></span><Tag text={t("out_proxy")} tone="teal" /></div>
        {#each locked as r}
          <div class="item"><Icon name="key" size={18} /><span class="grow">{r.domain?.length ? t("locked_domain") : t("locked_ip")}</span><span class="dim small">{t("locked_rules")}</span></div>
        {/each}
      </div>
    </section>

    <div class="fabs">
      <div class="menuwrap">
        <button class="fab" onclick={() => (presetsOpen = !presetsOpen)}><Icon name="sort" size={20} />{t("presets")}</button>
        {#if presetsOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div class="scrim" onclick={() => (presetsOpen = false)}></div>
          <div class="menu up">{#each presets as p}<button onclick={() => { presetsOpen = false; presetFor = p.id; }}>{presetName(p.id)}</button>{/each}</div>
        {/if}
      </div>
      <button class="fab p" onclick={() => (editing = -1)}><Icon name="plus" size={20} />{t("add_rule")}</button>
    </div>
  </div>

  <aside class="side col">
    {#if editing !== null}
      <div class="card">
        <h3>{editing < 0 ? t("add_rule") : t("title_rule")}</h3>
        {#key editing}
          <RuleEditor rule={editing >= 0 ? rules[editing] : { outboundTag: "direct" }}
            onsave={async (r) => { const l = structuredClone($state.snapshot(rules)) as routing.Rule[]; if (editing! < 0) l.push(r); else l[editing!] = r; if (await saveRules(l)) editing = null; }}
            ondelete={editing >= 0 ? async () => { const l = rules.filter((_, k) => k !== editing); if (await saveRules(l)) editing = null; } : undefined}
            oncancel={() => (editing = null)} />
        {/key}
      </div>
    {/if}

    <div class="card">
      <h3>{t("domain_strategy")}</h3>
      <select value={rt.domainStrategy || "AsIs"} onchange={(e) => saveRouting({ domainStrategy: e.currentTarget.value })}>
        <option value="AsIs">{t("ds_asis")}</option><option value="IPIfNonMatch">{t("ds_ipifnonmatch")}</option><option value="IPOnDemand">{t("ds_ipondemand")}</option>
      </select>
      <div class="hint">{t("d_domain_strategy_hint")}</div>
    </div>

    <div class="card">
      <h3>{t("geo_files")}</h3>
      {#if geo}<div class="hint">{geo.has_geoip && geo.has_geosite && geo.updated ? t("geo_status_ok", date(geo.updated)) : t("geo_status_missing")}</div>{/if}
      {#if settings}
        <select value={settings.options.geo_source} onchange={(e) => setGeo((o) => (o.geo_source = e.currentTarget.value))}>
          {#each sources as s}<option value={s.id}>{s.name}</option>{/each}<option value="custom">{t("geo_custom")}</option>
        </select>
        {#if settings.options.geo_source === "custom"}
          <input class="mono" placeholder="https://…" value={settings.options.geo_custom_url} onchange={(e) => setGeo((o) => (o.geo_custom_url = e.currentTarget.value.trim()))} />
        {/if}
        <Switch label={t("geo_auto")} checked={settings.options.geo_auto_update} hint={t("d_geo_auto_hint")} onchange={(v) => setGeo((o) => (o.geo_auto_update = v))} />
      {/if}
      <div class="row">
        <button class="btn" disabled={geoBusy} onclick={updateGeo}><Icon name="refresh" />{t("geo_update_now")}</button>
        {#if geoBusy}
          <span class="muted small">
            {#if !progress}{t("geo_connecting")}
            {:else if progress.verifying}{t("geo_progress", progress.file, progress.index, progress.count, t("geo_verifying"))}
            {:else}{t("geo_progress", progress.file, progress.index, progress.count, progress.total > 0 ? t("geo_progress_of", bytes(progress.done), bytes(progress.total)) : bytes(progress.done))}{/if}
          </span>
        {/if}
      </div>
    </div>
  </aside>
</div>

{#if presetFor}
  <Modal title={presetName(presetFor)} onclose={() => (presetFor = null)}>
    <div>{t("preset_apply_q")}</div>
    {#snippet actions()}
      <button class="btn flat" onclick={() => (presetFor = null)}>{t("cancel")}</button>
      <button class="btn" onclick={() => applyPreset(false)}>{t("preset_append")}</button>
      <button class="btn p" onclick={() => applyPreset(true)}>{t("preset_replace")}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .page { display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 16px; align-items: start; min-height: 100%; }
  .main { gap: 12px; min-height: 100%; display: flex; flex-direction: column; }
  .side { gap: 12px; }
  .list, .tester { background: var(--s); padding: 14px 10px 10px; gap: 8px; }
  .list > .cap, .tester > .cap { padding: 0 8px; }
  .tester { padding: 14px 14px 14px; }
  .q { border-radius: 20px; background: var(--s-highest); border-color: transparent; padding: 0 16px; }
  .portin { width: 80px; }
  .verdict { padding: 0 4px; }
  .rule.over { box-shadow: inset 0 3px 0 var(--primary); }
  .rule.sel { outline: 2px solid var(--primary); outline-offset: -2px; }
  .rule.off .body, .rule.off :global(.pill) { opacity: 0.5; }
  .handle { width: 18px; color: var(--outline); cursor: grab; display: grid; place-items: center; }
  .body { flex: 1; min-width: 0; display: grid; gap: 4px; text-align: left; background: transparent; border: 0; padding: 0; cursor: pointer; }
  .rn { font-size: 15px; }
  .conds { display: flex; flex-wrap: wrap; gap: 4px; }
  .cond { font: 11.5px/16px var(--mono); background: var(--s-high); color: var(--muted); border-radius: 6px; padding: 1px 6px; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .txt { display: grid; gap: 1px; }
  .small { font-size: 12px; }
  .pad { padding: 0 8px; }
  .fabs { margin-top: auto; }
</style>
