// UI strings come from tool/strings.py (shared with Android): strings.json holds en and ru.
import table from "./strings.json";

type Plural = { one?: string; few?: string; many?: string; other: string };
type Table = Record<string, string | Plural>;

const all = table as unknown as Record<string, Table>;
export const lang: "en" | "ru" = (navigator.language || "en").toLowerCase().startsWith("ru") ? "ru" : "en";
const strings: Table = all[lang];
const fallback: Table = all.en;

// Android placeholders: %1$s, %2$d, %d, %s.
function format(s: string, args: unknown[]): string {
  let next = 0;
  return s.replace(/%(?:(\d+)\$)?[sd]/g, (_m, n) => String(args[n ? Number(n) - 1 : next++] ?? ""));
}

export function t(key: string, ...args: unknown[]): string {
  const v = strings[key] ?? fallback[key];
  if (v === undefined) return key;
  if (typeof v !== "string") return format(v.other, args);
  return format(v, args);
}

const rules = new Intl.PluralRules(lang);

/** A plural string; n is also the first placeholder. */
export function tp(key: string, n: number, ...args: unknown[]): string {
  const v = strings[key] ?? fallback[key];
  if (v === undefined || typeof v === "string") return t(key, n, ...args);
  const form = rules.select(n) as keyof Plural;
  return format(v[form] ?? v.other, [n, ...args]);
}
