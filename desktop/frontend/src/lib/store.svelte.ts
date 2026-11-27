// App-wide state: the core's status (pushed every second), the saved servers and chains (reloaded when the
// core says something changed), transient messages.
import { Events } from "@wailsio/runtime";
import { Core, Desktop, type engine, type desk } from "./api";
import { lang, t } from "./i18n";
import { errorText } from "./errors";

const HISTORY = 60; // seconds of traffic kept for the sparklines

export type Page = "connect" | "servers" | "stats" | "routing" | "logs" | "settings";

class Store {
  page = $state<Page>("connect");
  status = $state<engine.Status | null>(null);
  servers = $state<engine.ServerView[]>([]);
  chains = $state<engine.ChainInfo[]>([]);
  info = $state<desk.Info | null>(null);
  desktop = $state<desk.DesktopState | null>(null);
  /** A newer desktop release, when the check found one. */
  update = $state<engine.Update | null>(null);
  /** Bytes per second, oldest first. */
  downRate = $state<number[]>([]);
  upRate = $state<number[]>([]);
  toasts = $state<{ id: number; text: string; error: boolean }[]>([]);
  /** Shared text waiting for the import preview (Ctrl+V anywhere, "Import" actions). */
  importText = $state<string | null>(null);
  /** Saved chains/servers being tested ("c:<id>" / "s:<id>"). */
  testing = $state<Record<string, boolean>>({});

  #last: { up: number; down: number; t: number } | null = null;
  #toastId = 0;

  async init() {
    Events.On("status", (ev) => this.#onStatus(ev.data as engine.Status));
    Events.On("changed", () => void this.reload());
    Events.On("desktop", (ev) => (this.desktop = ev.data as desk.DesktopState));
    Events.On("navigate", (ev) => (this.page = ev.data as Page));
    void Desktop.SetLanguage(lang);
    this.info = await Desktop.Info();
    this.desktop = await Desktop.State();
    const prefs = await Desktop.Prefs();
    if (prefs.check_updates) void this.checkUpdate(prefs.beta_updates, false);
    this.#onStatus(await Core.Status());
    await this.reload();
  }

  async reload() {
    const [s, c, st] = await Promise.all([Core.Servers(), Core.Chains(), Core.Status()]);
    this.servers = s ?? [];
    this.chains = c ?? [];
    this.#onStatus(st);
  }

  #onStatus(s: engine.Status) {
    this.status = s;
    const now = Date.now();
    if (s.running && this.#last) {
      const dt = Math.max(0.2, (now - this.#last.t) / 1000);
      this.downRate = [...this.downRate, Math.max(0, (s.down - this.#last.down) / dt)].slice(-HISTORY);
      this.upRate = [...this.upRate, Math.max(0, (s.up - this.#last.up) / dt)].slice(-HISTORY);
    } else if (!s.running) {
      this.downRate = [];
      this.upRate = [];
    }
    this.#last = s.running ? { up: s.up, down: s.down, t: now } : null;
  }

  /** Tests a saved chain or server without selecting it; the result is kept by the core (item.last). */
  async test(chainID: string, serverID: string) {
    const k = chainID ? `c:${chainID}` : `s:${serverID}`;
    this.testing = { ...this.testing, [k]: true };
    try {
      await Core.TestSaved(chainID, serverID);
    } catch {
      // kept as a failed result by the core
    } finally {
      const { [k]: _, ...rest } = this.testing;
      this.testing = rest;
      await this.reload();
    }
  }

  /** Clears every remembered result, then tests all items, a few at a time. */
  async testAll(items: { chain: string; server: string }[]) {
    await Core.ClearTests();
    await this.reload();
    const queue = [...items];
    const worker = async () => {
      for (let it = queue.shift(); it; it = queue.shift()) await this.test(it.chain, it.server);
    };
    await Promise.all(Array.from({ length: Math.min(4, queue.length) }, worker));
  }

  /** Asks GitHub for a newer desktop release; loud only when asked by the user. */
  async checkUpdate(beta: boolean, loud: boolean) {
    try {
      const u = await Desktop.CheckUpdate(beta);
      this.update = u?.newer ? u : null;
      if (loud && !u?.newer) this.toast(t("up_to_date"));
    } catch (e) {
      if (loud) this.toast(errorText(e), true);
    }
  }

  toast(text: string, error = false) {
    const id = ++this.#toastId;
    this.toasts = [...this.toasts, { id, text, error }];
    setTimeout(() => (this.toasts = this.toasts.filter((x) => x.id !== id)), error ? 8000 : 3000);
  }

  /** Runs an action; shows its error (and an optional success message). Returns false on error. */
  async act<T>(f: () => Promise<T>, ok?: string): Promise<T | undefined> {
    try {
      const r = await f();
      if (ok) this.toast(ok);
      return r;
    } catch (e) {
      this.toast(errorText(e), true);
      return undefined;
    }
  }
}

export const store = new Store();
