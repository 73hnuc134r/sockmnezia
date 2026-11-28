import { lang } from "./i18n";

export function bytes(b: number): string {
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
  if (b < 1024 ** 3) return `${(b / 1048576).toFixed(1)} MB`;
  return `${(b / 1024 ** 3).toFixed(2)} GB`;
}

export function rate(b: number): string {
  return `${bytes(Math.max(0, Math.round(b)))}/s`;
}

export function duration(ms: number): string {
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0) return `${h} h ${m} min`;
  if (m > 0) return `${m} min`;
  return `${s} s`;
}

export function date(unixSecondsOrIso: number | string): string {
  const d = typeof unixSecondsOrIso === "number" ? new Date(unixSecondsOrIso * 1000) : new Date(unixSecondsOrIso);
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

/** Country code → flag emoji ("" for unknown). */
export function flag(cc?: string): string {
  if (!cc || cc.length !== 2) return "";
  const A = 0x1f1e6;
  const up = cc.toUpperCase();
  return String.fromCodePoint(A + up.charCodeAt(0) - 65, A + up.charCodeAt(1) - 65);
}

let regions: Intl.DisplayNames | null = null;
/** Country code → its name in the UI language (the code itself when unknown). */
export function countryName(cc?: string): string {
  if (!cc) return "";
  try {
    regions ??= new Intl.DisplayNames([lang], { type: "region" });
    return regions.of(cc.toUpperCase()) ?? cc.toUpperCase();
  } catch {
    return cc.toUpperCase();
  }
}
