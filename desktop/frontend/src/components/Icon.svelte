<script lang="ts">
  // A small set of line icons (24×24, stroke = currentColor).
  let { name, size = 16 }: { name: string; size?: number } = $props();
  const paths: Record<string, string> = {
    play: "M7 5v14l11-7z",
    stop: "M6 6h12v12H6z",
    swap: "M7 7h13M16 3l4 4-4 4M17 17H4M8 13l-4 4 4 4",
    plus: "M12 5v14M5 12h14",
    edit: "M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4",
    trash: "M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3",
    share: "M8 12h.01M4 12a8 8 0 1 0 16 0M12 4v10M8 8l4-4 4 4",
    copy: "M9 9h11v11H9zM5 15H4V4h11v1",
    test: "M13 2 4 14h7l-1 8 9-12h-7z",
    more: "M12 5h.01M12 12h.01M12 19h.01",
    close: "M6 6l12 12M18 6 6 18",
    drag: "M4 8h16M4 12h16M4 16h16",
    globe: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z",
    link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
    routes: "M6 3v12a3 3 0 0 0 3 3h9M15 15l3 3-3 3M6 3 3 6M6 3l3 3",
    log: "M5 4h14v16H5zM8 8h8M8 12h8M8 16h5",
    gear: "M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z",
    home: "M3 11 12 4l9 7M5 10v10h5v-6h4v6h5V10",
    servers: "M4 5h16v5H4zM4 14h16v5H4zM8 7.5h.01M8 16.5h.01",
    key: "M15 7a4 4 0 1 1-3.5 6L4 20.5V17h3v-3h3l1.5-1.5A4 4 0 0 1 15 7zM16 9h.01",
    check: "M5 12l5 5L20 7",
    warn: "M12 4 2 20h20L12 4zM12 10v4M12 17h.01",
    import: "M12 3v12M7 10l5 5 5-5M4 21h16",
    eye: "M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12zM12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
    eyeoff: "M3 3l18 18M10.6 10.6a2 2 0 0 0 2.8 2.8M9.9 5.2A9.9 9.9 0 0 1 12 5c6 0 10 7 10 7a17 17 0 0 1-3.2 3.9M6.6 6.6C3.9 8.4 2 12 2 12s4 7 10 7a9.7 9.7 0 0 0 5.4-1.6",
    refresh: "M20 11a8 8 0 0 0-14.7-4.4L4 8M4 4v4h4M4 13a8 8 0 0 0 14.7 4.4L20 16M20 20v-4h-4",
    sort: "M7 4v16M3 8l4-4 4 4M17 20V4M13 16l4 4 4-4",
    chevron: "M9 6l6 6-6 6",
    down: "M6 9l6 6 6-6",
    up: "M6 15l6-6 6 6",
  };
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"
  stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="flex:none">
  <path d={paths[name] ?? ""} fill={name === "play" || name === "stop" ? "currentColor" : "none"} />
</svg>
