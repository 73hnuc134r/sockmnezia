<script lang="ts">
  import { onMount } from "svelte";
  import { store, type Page } from "./lib/store.svelte";
  import { t } from "./lib/i18n";
  import Icon from "./components/Icon.svelte";
  import Toasts from "./components/Toasts.svelte";
  import ImportDialog from "./components/ImportDialog.svelte";
  import { Clipboard } from "@wailsio/runtime";
  import Connect from "./pages/Connect.svelte";
  import Servers from "./pages/Servers.svelte";
  import Routing from "./pages/Routing.svelte";
  import Logs from "./pages/Logs.svelte";
  import Settings from "./pages/Settings.svelte";
  import Stats from "./pages/Stats.svelte";

  onMount(() => void store.init());

  const nav: { id: Page; icon: string; label: string }[] = [
    { id: "connect", icon: "home", label: "d_nav_connect" },
    { id: "servers", icon: "servers", label: "d_nav_servers" },
    { id: "routing", icon: "routes", label: "title_routing" },
    { id: "stats", icon: "chart", label: "title_stats" },
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
</script>

<svelte:window onkeydown={onkey} />
<div class="shell">
  <nav class="rail">
    <div class="brand" title="sockmnezia">
      <svg viewBox="0 0 24 24" width="30" height="30" aria-hidden="true"><path fill="currentColor" d="M8,2h7v9.5l4.2,4.2a3.5,3.5 0,0 1,0 4.95l-0.35,0.35a3.5,3.5 0,0 1,-4.95 0L8,15.1V2z" /><path fill="var(--primary)" d="M10,5h3v1.5h-3z" /></svg>
    </div>
    {#each nav as n}
      {#if n.id === "settings"}<span class="grow"></span>{/if}
      <button class="nav" class:on={store.page === n.id} onclick={() => (store.page = n.id)}>
        <span class="ind"><Icon name={n.icon} size={22} />{#if n.id === "connect" && st?.running}<span class="led"></span>{/if}</span>
        <span class="lbl">{t(n.label)}</span>
      </button>
    {/each}
  </nav>
  <main class="main" class:fill={store.page === "servers"}>
    {#if store.page === "connect"}<Connect />
    {:else if store.page === "servers"}<Servers />
    {:else if store.page === "stats"}<Stats />
    {:else if store.page === "routing"}<Routing />
    {:else if store.page === "logs"}<Logs />
    {:else}<Settings />{/if}
  </main>
</div>
<Toasts />
{#if store.importText !== null}
  <ImportDialog text={store.importText} onclose={() => (store.importText = null)} ondone={() => (store.page = "servers")} />
{/if}

<style>
  .shell { display: grid; grid-template-columns: 88px 1fr; height: 100vh; background: var(--s); }
  .rail { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 16px 0 16px; min-height: 0; overflow: auto; }
  .brand { width: 52px; height: 52px; border-radius: 16px; background: var(--primary); color: var(--on-primary); display: grid; place-items: center; margin-bottom: 20px; flex: none; }
  .nav { display: flex; flex-direction: column; align-items: center; gap: 4px; background: transparent; border: 0; cursor: pointer; color: var(--muted); padding: 4px 0 6px; width: 80px; }
  .ind { position: relative; width: 56px; height: 32px; border-radius: 16px; display: grid; place-items: center; }
  .nav:hover .ind { background: var(--state); }
  .nav.on { color: var(--ink); }
  .nav.on .ind { background: var(--secondary-c); color: var(--on-secondary-c); }
  .lbl { font: 500 12px/16px var(--ui); text-align: center; max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .nav.on .lbl { font-weight: 600; }
  .led { position: absolute; top: 4px; right: 14px; width: 8px; height: 8px; border-radius: 50%; background: var(--primary); box-shadow: 0 0 0 2px var(--s); }
  .grow { flex: 1; }
  .main { margin: 12px 12px 12px 0; background: var(--surface); border-radius: var(--r-island); padding: 28px 32px 20px; overflow: auto; min-width: 0; min-height: 0; }
  .main.fill { display: grid; }
</style>
