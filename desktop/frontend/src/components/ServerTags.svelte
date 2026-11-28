<script lang="ts">
  import Tag from "./Tag.svelte";
  // Protocol, security, transport (not tcp) and port as pills, the same as the Android app.
  let { protocol = "", security = "", network = "", port = 0 }: { protocol?: string; security?: string; network?: string; port?: number } = $props();
  const net = (n: string) => (n === "ws" || n === "grpc" || n === "xhttp" ? n.toUpperCase() : n);
</script>

<span class="tags">
  {#if protocol}<Tag text={protocol === "shadowsocks" ? "SS" : protocol.toUpperCase()} tone="tonal" />{/if}
  {#if security === "none"}<Tag text="NONE" tone="red" title="no TLS / Reality" />
  {:else if security}<Tag text={security === "reality" ? "Reality" : security.toUpperCase()} tone="tonal" />{/if}
  {#if network && network !== "tcp" && network !== "raw"}<Tag text={net(network)} tone="tonal" />{/if}
  {#if port > 0}<Tag text={String(port)} />{/if}
</span>

<style>
  .tags { display: inline-flex; flex-wrap: wrap; gap: 4px; vertical-align: middle; }
</style>
