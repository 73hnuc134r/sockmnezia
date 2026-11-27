<script lang="ts">
  import { onMount } from "svelte";
  import { store, type Page } from "./lib/store.svelte";
  import { t } from "./lib/i18n";
  import { rate } from "./lib/format";
  import Icon from "./components/Icon.svelte";
  import Toasts from "./components/Toasts.svelte";
  import ImportDialog from "./components/ImportDialog.svelte";
  import { Clipboard } from "@wailsio/runtime";
  import Connect from "./pages/Connect.svelte";
  import Servers from "./pages/Servers.svelte";
  import Routing from "./pages/Routing.svelte";
  import Logs from "./pages/Logs.svelte";
  import Settings from "./pages/Settings.svelte";

  onMount(() => void store.init());

  const nav: { id: Page; icon: string; label: string }[] = [
    { id: "connect", icon: "home", label: "d_nav_connect" },
    { id: "servers", icon: "servers", label: "d_nav_servers" },
    { id: "routing", icon: "routes", label: "title_routing" },
    { id: "logs", icon: "log", label: "title_logs" },
    { id: "settings", icon: "gear", label: "title_settings" },
  ];
  // Ctrl+V anywhere outside a text field imports links, a shared chain or an outbound JSON (with a preview).
  async function onkey(e: KeyboardEvent) {
    if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== "v" || store.importText !== null) return;
    const el = e.target as HTMLElement | null;
    if (el && (el.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName))) return;
    e.preventDefault();
    const text = (await Clipboard.Text()) ?? "";
    if (text.trim()) store.importText = text;
  }

  const st = $derived(store.status);
  const current = $derived(st?.server_id ? (store.servers.find((s) => s.id === st.server_id)?.name ?? "") : (st?.chain_name ?? ""));
</script>

<svelte:window onkeydown={onkey} />
<div class="shell">
  <nav class="side">
    <div class="brand">sockmnezia</div>
    {#each nav as n}
      <button class="nav" class:on={store.page === n.id} onclick={() => (store.page = n.id)}>
        <Icon name={n.icon} />{t(n.label)}
      </button>
    {/each}
  </nav>
  <main class="main">
    {#if store.page === "connect"}<Connect />
    {:else if store.page === "servers"}<Servers />
    {:else if store.page === "routing"}<Routing />
    {:else if store.page === "logs"}<Logs />
    {:else}<Settings />{/if}
  </main>
  <footer class="statusbar">
    <span class="row nowrap"><span class="led" class:on={st?.running} class:bad={!!st?.last_error && !st?.running}></span>{st?.running ? t("d_running") : t("conn_stopped")}</span>
    {#if current}<span class="ellipsis">{current}{st?.hops && st.hops.length > 1 ? ` · ${t("d_hops", st.hops.length)}` : ""}</span>{/if}
    {#if st?.running && st.vpn}<span>TUN</span>{/if}
    {#if st?.running}<span class="mono">SOCKS {st.listen}:{st.socks_port}</span>{/if}
    <span class="grow"></span>
    {#if st?.running}<span class="mono">↓ {rate(store.downRate.at(-1) ?? 0)} ↑ {rate(store.upRate.at(-1) ?? 0)}</span>{/if}
  </footer>
</div>
<Toasts />
{#if store.importText !== null}
  <ImportDialog text={store.importText} onclose={() => (store.importText = null)} ondone={() => (store.page = "servers")} />
{/if}

<style>
  .shell { display: grid; grid-template-columns: 196px 1fr; grid-template-rows: 1fr auto; height: 100vh; }
  .side { grid-row: 1; background: var(--side); border-right: 1px solid var(--line); padding: 14px 10px; display: grid; align-content: start; gap: 2px; }
  .brand { font-weight: 600; font-size: 15px; padding: 4px 10px 16px; }
  .nav { display: flex; gap: 10px; align-items: center; padding: 9px 10px; border-radius: 8px; color: var(--muted); background: transparent; border: 0; cursor: pointer; text-align: left; }
  .nav:hover { background: var(--hover); }
  .nav.on { background: var(--sel); color: var(--ink); }
  .main { grid-row: 1; padding: 18px 20px; overflow: auto; min-width: 0; }
  .statusbar { grid-column: 1 / span 2; display: flex; gap: 18px; align-items: center; padding: 6px 14px; background: var(--side); border-top: 1px solid var(--line); color: var(--muted); font-size: 12px; white-space: nowrap; min-width: 0; }
  .led { width: 8px; height: 8px; border-radius: 50%; background: var(--dim); display: inline-block; }
  .led.on { background: var(--ok); }
  .led.bad { background: var(--warn); }
</style>
