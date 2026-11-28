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
  <h1>{t("title_settings")}</h1>
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
  .page { display: grid; gap: 16px; align-content: start; max-width: 960px; }
  .tabs { display: flex; gap: 8px; flex-wrap: wrap; }
  .tabs button { height: 32px; padding: 0 14px; border-radius: 8px; color: var(--muted); background: transparent; border: 1px solid var(--outline); cursor: pointer; font-weight: 500; }
  .tabs button:hover { background: var(--state); }
  .tabs button.on { color: var(--on-secondary-c); background: var(--secondary-c); border-color: transparent; }
</style>
