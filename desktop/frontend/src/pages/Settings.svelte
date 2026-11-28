<script lang="ts">
  import { Core, type engine } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { t } from "../lib/i18n";
  import Amnezia from "./settings/Amnezia.svelte";
  import Proxy from "./settings/Proxy.svelte";
  import App from "./settings/App.svelte";
  import About from "./settings/About.svelte";
  import Updates from "./settings/Updates.svelte";

  let tab = $state<"proxy" | "amnezia" | "app" | "about">("proxy");
  let settings = $state<engine.Settings | null>(null);
  // Reload when saved state changes or the mode is switched (from the tray). The status object is replaced every
  // second, so depend on the derived value, not on the status; and keep the same object when nothing changed,
  // or the forms below would be rebuilt (flicker, lost focus).
  const mode = $derived(`${store.status?.system_proxy ?? ""}/${store.status?.mode ?? ""}`);
  $effect(() => {
    void store.chains;
    void mode;
    Core.Settings().then((s) => {
      if (JSON.stringify(s) !== JSON.stringify($state.snapshot(settings))) settings = s;
    });
  });
  const tabs = [["proxy", "local_proxy"], ["amnezia", "amnezia_settings"], ["app", "app_settings"], ["about", "title_about"]] as const;
</script>

<div class="page">
  <div class="tabs">
    {#each tabs as [id, label]}<button class:on={tab === id} onclick={() => (tab = id)}>{t(label)}</button>{/each}
  </div>
  {#if settings}
    {#key settings}
      {#if tab === "proxy"}<Proxy {settings} />
      {:else if tab === "amnezia"}<Amnezia />
      {:else if tab === "app"}<App {settings} /><Updates />
      {:else}<About />{/if}
    {/key}
  {/if}
</div>

<style>
  .page { display: grid; gap: 14px; align-content: start; max-width: 920px; }
  .tabs { display: flex; gap: 18px; border-bottom: 1px solid var(--line); }
  .tabs button { padding: 8px 2px; color: var(--muted); background: transparent; border: 0; border-bottom: 2px solid transparent; cursor: pointer; }
  .tabs button.on { color: var(--ink); border-bottom-color: var(--accent); }
</style>
