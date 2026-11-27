// App-wide state: the core's status (pushed every second), the saved servers and chains (reloaded when the
// core says something changed), transient messages.
import { Events } from "@wailsio/runtime";
import { Core, Desktop, type engine, type desk } from "./api";
import { errorText } from "./errors";

const HISTORY = 60; // seconds of traffic kept for the sparklines

export type Page = "connect" | "servers" | "routing" | "logs" | "settings";

class Store {
  page = $state<Page>("connect");
  status = $state<engine.Status | null>(null);
  servers = $state<engine.ServerView[]>([]);
  chains = $state<engine.ChainInfo[]>([]);
  info = $state<desk.Info | null>(null);
  /** Bytes per second, oldest first. */
  downRate = $state<number[]>([]);
  upRate = $state<number[]>([]);
  toasts = $state<{ id: number; text: string; error: boolean }[]>([]);

  #last: { up: number; down: number; t: number } | null = null;
  #toastId = 0;

  async init() {
    Events.On("status", (ev) => this.#onStatus(ev.data as engine.Status));
    Events.On("changed", () => void this.reload());
    this.info = await Desktop.Info();
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
