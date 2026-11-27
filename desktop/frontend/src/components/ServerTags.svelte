<script lang="ts">
  import Tag from "./Tag.svelte";
  // Protocol, security, transport (not tcp) and port: the same tags as the Android app.
  let { protocol = "", security = "", network = "", port = 0 }: { protocol?: string; security?: string; network?: string; port?: number } = $props();
  const protoColor = (p: string) =>
    ({ vless: "var(--t-vless)", vmess: "var(--t-vmess)", trojan: "var(--t-trojan)", shadowsocks: "var(--t-ss)", socks: "var(--t-socks)", http: "var(--t-http)" })[p] ?? "var(--muted)";
  const secColor = (s: string) => ({ reality: "var(--t-reality)", tls: "var(--t-tls)" })[s] ?? "var(--warn)";
</script>

<span class="tags">
  {#if protocol}<Tag text={protocol === "shadowsocks" ? "ss" : protocol} color={protoColor(protocol)} />{/if}
  {#if security}<Tag text={security} color={secColor(security)} />{/if}
  {#if network && network !== "tcp" && network !== "raw"}<Tag text={network} />{/if}
  {#if port > 0}<Tag text={`:${port}`} />{/if}
</span>

<style>
  .tags { display: inline-flex; flex-wrap: wrap; gap: 4px; vertical-align: middle; }
</style>
