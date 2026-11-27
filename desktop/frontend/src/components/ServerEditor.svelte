<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import { Core, type links } from "../lib/api";
  import { t } from "../lib/i18n";
  import { coreError, errorText, type CoreError } from "../lib/errors";

  // Endpoint editor for a saved server (id) or a new one (id = ""): fields, or the link / Xray outbound JSON as
  // text. Both go through the core's parser; editing keeps the ID, so chains using the server follow.
  let { id, onsaved, oncancel }: { id: string; onsaved: (id: string) => void; oncancel?: () => void } = $props();

  const Protocols = ["vless", "vmess", "trojan", "shadowsocks", "socks", "http"];
  const Transports = ["tcp", "ws", "grpc", "httpupgrade", "xhttp"];
  const Fingerprints = ["", "chrome", "firefox", "safari", "ios", "android", "edge", "qq", "random", "randomized"];
  const SsMethods = ["2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305", "aes-128-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305"];
  const VmessSecurity = ["auto", "aes-128-gcm", "chacha20-poly1305", "none", "zero"];
  const XhttpModes = ["auto", "packet-up", "stream-up", "stream-one"];

  const empty = (): links.Endpoint => ({
    name: "", protocol: "vless", address: "", port: 443, id: "", password: "", user: "", method: "", flow: "", encryption: "",
    network: "tcp", security: "tls", sni: "", fp: "", alpn: "", pbk: "", sid: "", spx: "", path: "", host: "",
    service_name: "", mode: "", header_type: "", authority: "", extra: "",
  });

  let ep = $state<links.Endpoint>(empty());
  let text = $state("");
  let fields = $state(true);
  let jsonOnly = $state(false);
  let error = $state<CoreError | null>(null);
  let busy = $state(false);

  $effect(() => {
    const cur = id;
    error = null;
    jsonOnly = false;
    fields = true;
    if (!cur) {
      ep = empty();
      text = "";
      return;
    }
    Core.ServerEdit(cur).then((e) => {
      if (cur !== id) return;
      text = e.link;
      if (e.endpoint) ep = e.endpoint;
      else {
        jsonOnly = true;
        fields = false;
      }
    });
  });

  async function switchMode(toFields: boolean) {
    if (toFields === fields) return;
    error = null;
    try {
      if (toFields) {
        ep = await Core.LinkToEndpoint(text);
        jsonOnly = false;
      } else text = await Core.EndpointToLink(ep);
      fields = toFields;
    } catch (e) {
      const ce = coreError(e);
      if (ce.code === "json_only") jsonOnly = true;
      error = ce;
    }
  }

  async function save() {
    error = null;
    busy = true;
    try {
      const link = fields ? await Core.EndpointToLink(ep) : text.trim();
      // The name travels in the link (fields: the Name field; text: the link's #fragment).
      if (id) {
        await Core.UpdateServer(id, link, "");
        onsaved(id);
      } else onsaved(await Core.AddServer(link, ""));
    } catch (e) {
      error = coreError(e);
    } finally {
      busy = false;
    }
  }

  function setProtocol(p: string) {
    // Reality is vless-only in practice; socks/http/ss have no transport settings here.
    ep.protocol = p;
    if (["socks", "http", "shadowsocks"].includes(p)) ep.security = "none";
    if (p === "vmess" && ep.security === "reality") ep.security = "tls";
  }
  const net = $derived(ep.network || "tcp");
  const hasTransport = $derived(["vless", "vmess", "trojan"].includes(ep.protocol));
</script>

<div class="ed">
  <div class="seg">
    <button class:on={fields} onclick={() => switchMode(true)}>{t("srv_mode_fields")}</button>
    <button class:on={!fields} onclick={() => switchMode(false)}>{t("srv_mode_link")}</button>
  </div>
  {#if jsonOnly}<div class="hint warn">{t("srv_json_only")}</div>{/if}

  {#if fields}
    <div class="field"><label for="f-name">{t("srv_name")}</label><input id="f-name" bind:value={ep.name} /></div>
    <div class="two">
      <div class="field"><label for="f-proto">{t("srv_protocol")}</label>
        <select id="f-proto" value={ep.protocol} onchange={(e) => setProtocol(e.currentTarget.value)}>
          {#each Protocols as p}<option value={p}>{p}</option>{/each}
        </select></div>
      <div></div>
    </div>
    <div class="two">
      <div class="field"><label for="f-addr">{t("srv_address")}</label><input id="f-addr" class="mono" bind:value={ep.address} /></div>
      <div class="field"><label for="f-port">{t("srv_port")}</label><input id="f-port" class="mono" type="number" min="1" max="65535" bind:value={ep.port} /></div>
    </div>
    {#if ep.protocol === "vless" || ep.protocol === "vmess"}
      <div class="field"><label for="f-uuid">{t("srv_uuid")}</label><input id="f-uuid" class="mono" bind:value={ep.id} /></div>
    {/if}
    {#if ep.protocol === "vless"}
      <div class="field"><label for="f-flow">{t("srv_flow")}</label>
        <select id="f-flow" bind:value={ep.flow}><option value="">{t("srv_none")}</option><option value="xtls-rprx-vision">xtls-rprx-vision</option></select></div>
    {:else if ep.protocol === "vmess"}
      <div class="field"><label for="f-enc">{t("srv_vmess_security")}</label>
        <select id="f-enc" value={ep.encryption || "auto"} onchange={(e) => (ep.encryption = e.currentTarget.value)}>
          {#each VmessSecurity as v}<option value={v}>{v}</option>{/each}</select></div>
    {:else if ep.protocol === "shadowsocks"}
      <div class="field"><label for="f-method">{t("srv_method")}</label>
        <select id="f-method" bind:value={ep.method}>
          {#each [...SsMethods, ...(ep.method && !SsMethods.includes(ep.method) ? [ep.method] : [])] as m}<option value={m}>{m}</option>{/each}
        </select></div>
    {:else if ep.protocol === "socks" || ep.protocol === "http"}
      <div class="field"><label for="f-user">{t("username")}</label><input id="f-user" bind:value={ep.user} /></div>
    {/if}
    {#if ["trojan", "shadowsocks", "socks", "http"].includes(ep.protocol)}
      <div class="field"><label for="f-pass">{t("password")}</label><input id="f-pass" class="mono" bind:value={ep.password} /></div>
    {/if}
    {#if ep.protocol === "http"}
      <label class="check"><input type="checkbox" checked={ep.security === "tls"} onchange={(e) => (ep.security = e.currentTarget.checked ? "tls" : "none")} /><span>{t("srv_tls")}</span></label>
    {/if}

    {#if hasTransport}
      <div class="sect cap">{t("srv_section_transport")}</div>
      <div class="two">
        <div class="field"><label for="f-net">{t("srv_transport")}</label>
          <select id="f-net" value={net} onchange={(e) => (ep.network = e.currentTarget.value)}>{#each Transports as n}<option value={n}>{n}</option>{/each}</select></div>
        <div class="field"><label for="f-sec">{t("srv_security")}</label>
          <select id="f-sec" value={ep.security || "none"} onchange={(e) => (ep.security = e.currentTarget.value)}>
            <option value="none">{t("srv_none")}</option><option value="tls">{t("srv_tls")}</option>
            {#if ep.protocol !== "vmess"}<option value="reality">{t("srv_reality")}</option>{/if}
          </select></div>
      </div>
      {#if net === "ws" || net === "httpupgrade" || net === "xhttp"}
        <div class="two">
          <div class="field"><label for="f-path">{t("srv_path")}</label><input id="f-path" class="mono" bind:value={ep.path} /></div>
          <div class="field"><label for="f-host">{t("srv_host")}</label><input id="f-host" class="mono" bind:value={ep.host} /></div>
        </div>
      {/if}
      {#if net === "grpc"}
        <div class="two">
          <div class="field"><label for="f-svc">{t("srv_service")}</label><input id="f-svc" class="mono" bind:value={ep.service_name} /></div>
          <div class="field"><label for="f-auth">{t("srv_authority")}</label><input id="f-auth" class="mono" bind:value={ep.authority} /></div>
        </div>
        <label class="check"><input type="checkbox" checked={ep.mode === "multi"} onchange={(e) => (ep.mode = e.currentTarget.checked ? "multi" : "")} /><span>{t("srv_grpc_multi")}</span></label>
      {/if}
      {#if net === "xhttp"}
        <div class="field"><label for="f-mode">{t("srv_xhttp_mode")}</label>
          <select id="f-mode" value={ep.mode || "auto"} onchange={(e) => (ep.mode = e.currentTarget.value)}>{#each XhttpModes as m}<option value={m}>{m}</option>{/each}</select></div>
        <div class="field"><label for="f-extra">{t("srv_extra")}</label><textarea id="f-extra" rows="3" bind:value={ep.extra}></textarea></div>
      {/if}
      {#if net === "tcp"}
        <label class="check"><input type="checkbox" checked={ep.header_type === "http"} onchange={(e) => (ep.header_type = e.currentTarget.checked ? "http" : "")} /><span>{t("srv_http_header")}</span></label>
        {#if ep.header_type === "http"}
          <div class="two">
            <div class="field"><label for="f-hpath">{t("srv_path")}</label><input id="f-hpath" class="mono" bind:value={ep.path} /></div>
            <div class="field"><label for="f-hhost">{t("srv_host")}</label><input id="f-hhost" class="mono" bind:value={ep.host} /></div>
          </div>
        {/if}
      {/if}
      {#if ep.security === "tls" || ep.security === "reality"}
        <div class="two">
          <div class="field"><label for="f-sni">{t("srv_sni")}</label><input id="f-sni" class="mono" bind:value={ep.sni} /></div>
          <div class="field"><label for="f-fp">{t("srv_fp")}</label>
            <select id="f-fp" bind:value={ep.fp}>{#each Fingerprints as f}<option value={f}>{f || t("srv_default")}</option>{/each}</select></div>
        </div>
      {/if}
      {#if ep.security === "tls"}
        <div class="field"><label for="f-alpn">{t("srv_alpn")}</label><input id="f-alpn" class="mono" bind:value={ep.alpn} /></div>
      {/if}
      {#if ep.security === "reality"}
        <div class="field"><label for="f-pbk">{t("srv_pbk")}</label><input id="f-pbk" class="mono" bind:value={ep.pbk} /></div>
        <div class="two">
          <div class="field"><label for="f-sid">{t("srv_sid")}</label><input id="f-sid" class="mono" bind:value={ep.sid} /></div>
          <div class="field"><label for="f-spx">{t("srv_spx")}</label><input id="f-spx" class="mono" bind:value={ep.spx} /></div>
        </div>
      {/if}
    {/if}
  {:else}
    <div class="field">
      <label for="f-link">{t("server_link")}</label>
      <textarea id="f-link" rows="9" bind:value={text} oninput={() => (error = null)}></textarea>
    </div>
    <div class="row"><button class="btn small" onclick={async () => (text = (await Clipboard.Text()) ?? "")}>{t("paste")}</button></div>
    <div class="hint">{t("srv_link_hint")}</div>
    <div class="hint">{t("d_server_link_hint")}</div>
  {/if}

  {#if error}<div class="hint warn" style="white-space:pre-line">{errorText(error)}</div>{/if}
  <div class="row">
    <span class="grow"></span>
    {#if oncancel}<button class="btn flat" onclick={oncancel}>{t("cancel")}</button>{/if}
    <button class="btn p" disabled={busy} onclick={save}>{id ? t("save") : t("add")}</button>
  </div>
</div>

<style>
  .ed { display: grid; gap: 10px; align-content: start; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .two:has(#f-port) { grid-template-columns: 1fr 90px; }
  .sect { margin-top: 6px; }
</style>
