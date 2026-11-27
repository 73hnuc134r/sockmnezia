<script lang="ts">
  import { untrack } from "svelte";
  import { Core, type routing } from "../lib/api";
  import { t } from "../lib/i18n";
  import { coreError, errorText } from "../lib/errors";

  // One routing rule: name, target, domains, IPs and the advanced conditions. Validated by the core.
  let { rule, onsave, ondelete, oncancel }: { rule: routing.Rule; onsave: (r: routing.Rule) => void; ondelete?: () => void; oncancel: () => void } = $props();

  // The form starts from the rule (the parent re-creates the editor for another rule).
  const init = structuredClone(untrack(() => $state.snapshot(rule))) as routing.Rule;
  let remarks = $state(init.remarks ?? "");
  let out = $state(init.outboundTag || "proxy");
  let domains = $state((init.domain ?? []).join("\n"));
  let ips = $state((init.ip ?? []).join("\n"));
  let port = $state(init.port ?? "");
  let network = $state(init.network ?? "");
  let protocols = $state<string[]>(init.protocol ?? []);
  let enabled = $state(init.enabled !== false);
  let more = $state(!!(init.port || init.network || init.protocol?.length));
  let error = $state("");

  const lines = (s: string) => s.split(/[\n,]/).map((x) => x.trim()).filter(Boolean);

  async function save() {
    const r: routing.Rule = {
      ...init, remarks: remarks.trim(), outboundTag: out, domain: lines(domains), ip: lines(ips), port: port.trim(),
      network, protocol: protocols, enabled: enabled ? null : false,
    };
    try {
      await Core.ValidateRule(r);
      onsave(r);
    } catch (e) {
      error = errorText(coreError(e));
    }
  }
  function toggleProto(p: string, on: boolean) {
    protocols = on ? [...protocols, p] : protocols.filter((x) => x !== p);
  }
</script>

<div class="col">
  <div class="field"><label for="r-name">{t("rule_name")}</label><input id="r-name" bind:value={remarks} /></div>
  <div class="field">
    <span class="label">{t("rule_outbound")}</span>
    <div class="seg">
      {#each [["proxy", "out_proxy"], ["direct", "out_direct"], ["block", "out_block"]] as [v, l]}
        <button class:on={out === v} onclick={() => (out = v)}>{t(l)}</button>
      {/each}
    </div>
    {#if out === "direct"}<span class="hint">{t("d_direct_hint")}</span>{/if}
  </div>
  <div class="field"><label for="r-dom">{t("rule_domains")}</label><textarea id="r-dom" rows="5" bind:value={domains} placeholder="domain:example.com"></textarea>
    <span class="hint">{t("rule_domains_hint")}</span></div>
  <div class="field"><label for="r-ip">{t("rule_ips")}</label><textarea id="r-ip" rows="3" bind:value={ips} placeholder="198.51.100.0/24, geoip:ru"></textarea></div>
  <button class="btn flat small" style="justify-self:start" onclick={() => (more = !more)}>{t("rule_more")} {more ? "▴" : "▾"}</button>
  {#if more}
    <div class="field"><label for="r-port">{t("rule_port")}</label><input id="r-port" class="mono" bind:value={port} placeholder="443, 1000-2000" /></div>
    <div class="field">
      <span class="label">{t("rule_network")}</span>
      <div class="seg">{#each [["", "any"], ["tcp", ""], ["udp", ""], ["tcp,udp", ""]] as [v, l]}<button class:on={network === v} onclick={() => (network = v)}>{l ? t(l) : v}</button>{/each}</div>
    </div>
    <div class="field">
      <span class="label">{t("rule_protocol")}</span>
      <div class="row">{#each ["http", "tls", "quic", "bittorrent"] as p}
        <label class="check"><input type="checkbox" checked={protocols.includes(p)} onchange={(e) => toggleProto(p, e.currentTarget.checked)} /><span>{p}</span></label>
      {/each}</div>
    </div>
  {/if}
  <label class="check"><input type="checkbox" bind:checked={enabled} /><span>{t("rule_enabled")}</span></label>
  {#if error}<div class="hint warn" style="white-space:pre-line">{error}</div>{/if}
  <div class="row">
    {#if ondelete}<button class="btn flat danger" onclick={ondelete}>{t("delete")}</button>{/if}
    <span class="grow"></span>
    <button class="btn flat" onclick={oncancel}>{t("cancel")}</button>
    <button class="btn p" onclick={save}>{t("save")}</button>
  </div>
</div>
