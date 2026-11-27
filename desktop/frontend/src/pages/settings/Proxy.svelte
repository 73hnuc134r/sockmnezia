<script lang="ts">
  import { untrack } from "svelte";
  import { Core, Desktop, type engine } from "../../lib/api";
  import { store } from "../../lib/store.svelte";
  import { t } from "../../lib/i18n";
  import Switch from "../../components/Switch.svelte";
  import Icon from "../../components/Icon.svelte";

  // The local proxy: ports, credentials, UDP, the HTTP port, sharing on the local network.
  let { settings }: { settings: engine.Settings } = $props();
  const credentialOk = /^[\x21-\x7e]{1,255}$/;

  // The form starts from the saved values (the parent re-creates it when they change).
  const init = untrack(() => settings.inbound);
  let port = $state(String(init.socks_port));
  let user = $state(init.user);
  let pass = $state(init.pass);
  let udp = $state(init.udp);
  let httpOn = $state(init.http_port !== 0);
  let httpPort = $state(init.http_port ? String(init.http_port) : "");
  let listen = $state(init.listen);
  let showPass = $state(false);
  let lan = $state<string[]>([]);
  Desktop.LANAddresses().then((l) => (lan = l ?? []));

  const dirty = $derived(
    port !== String(settings.inbound.socks_port) || user !== settings.inbound.user || pass !== settings.inbound.pass ||
      udp !== settings.inbound.udp || listen !== settings.inbound.listen ||
      (httpOn ? httpPort : "") !== (settings.inbound.http_port ? String(settings.inbound.http_port) : ""),
  );
  const credBad = $derived(!credentialOk.test(user) || !credentialOk.test(pass));

  async function save() {
    const ns: engine.Settings = {
      ...settings,
      inbound: { ...settings.inbound, socks_port: Number(port) || 0, user, pass, udp, listen, http_port: httpOn ? Number(httpPort) || 0 : 0 },
    };
    await store.act(() => Core.SetSettings(ns), t("saved"));
  }
  async function toggleHttp(on: boolean) {
    httpOn = on;
    if (on && !httpPort) httpPort = String(await Core.RandomPort(Number(port) || 0));
  }
</script>

<div class="card">
  <h3>{t("local_proxy")}</h3>
  <div class="grid">
    <div class="field"><label for="p-port">{t("socks_port")}</label>
      <div class="row nowrap"><input id="p-port" class="mono" bind:value={port} />
        <button class="btn" onclick={async () => (port = String(await Core.RandomPort(settings.inbound.http_port)))}>{t("random")}</button></div></div>
    <div class="field"><label for="p-user">{t("username")}</label>
      <div class="row nowrap"><input id="p-user" class="mono" class:bad={!credentialOk.test(user)} bind:value={user} />
        <button class="btn" onclick={async () => (user = await Core.RandomUser())}>{t("random")}</button></div></div>
    <div class="field"><label for="p-pass">{t("password")}</label>
      <div class="row nowrap"><input id="p-pass" class="mono" class:bad={!credentialOk.test(pass)} type={showPass ? "text" : "password"} bind:value={pass} />
        <button class="btn icon flat" title={showPass ? t("hide") : t("show")} onclick={() => (showPass = !showPass)}><Icon name={showPass ? "eyeoff" : "eye"} /></button>
        <button class="btn" onclick={async () => (pass = await Core.RandomPassword())}>{t("random")}</button></div></div>
  </div>
  {#if credBad}<div class="hint warn">{t("credential_rule")}</div>
  {:else if pass.length < 12}<div class="hint warn">{t("d_password_weak")}</div>{/if}
  <div class="hint">{t("credentials_hint")}</div>
  <Switch label={t("udp")} checked={udp} hint={t("udp_hint")} onchange={(v) => (udp = v)} />
  <Switch label={t("http_proxy")} checked={httpOn} hint={t("http_hint")} onchange={toggleHttp} />
  {#if httpOn}
    <div class="field narrow"><label for="p-http">{t("http_port")}</label><input id="p-http" class="mono" bind:value={httpPort} /></div>
  {/if}
  <Switch label={t("share_lan")} checked={listen !== "127.0.0.1"} hint={lan.length ? t("d_share_lan_hint") : t("d_share_lan_none")}
    disabled={!lan.length && listen === "127.0.0.1"} onchange={(on) => (listen = on ? lan[0] : "127.0.0.1")} />
  {#if listen !== "127.0.0.1"}
    <div class="field narrow"><select bind:value={listen}>{#each lan.length ? lan : [listen] as a}<option value={a}>{a}</option>{/each}</select></div>
    <div class="hint warn">{t("share_lan_warning")}</div>
  {/if}
  <div class="row"><span class="grow"></span><button class="btn p" disabled={!dirty || credBad} onclick={save}>{t("save")}</button></div>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 10px; }
  .narrow { max-width: 260px; }
</style>
