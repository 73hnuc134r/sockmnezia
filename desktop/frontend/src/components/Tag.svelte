<script lang="ts">
  // A small pill: neutral, tonal (teal), blue, red or teal (primary container).
  let { text, tone = "neutral", title = "" }: { text: string; tone?: "neutral" | "tonal" | "blue" | "red" | "teal"; title?: string } = $props();
</script>

<span class="pill {tone === 'neutral' ? '' : tone}" {title}>{text}</span>
