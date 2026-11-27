import { t } from "./i18n";
import type { Error as CoreError } from "../../bindings/github.com/nuc134r/sockmnezia/core/engine/models";

export type { CoreError };

/** The core's error from a failed call (its cause), or an internal one. */
export function coreError(e: unknown): CoreError {
  const cause = (e as { cause?: unknown })?.cause;
  if (cause && typeof cause === "object" && "code" in (cause as object)) return cause as CoreError;
  return { code: "internal", detail: String((e as { message?: string })?.message ?? e) };
}

const codes = new Set([
  "invalid_key", "no_gateway_key", "gateway_wrong_key", "gateway_unreachable", "device_limit", "trial_used",
  "subscription_expired", "subscription_not_active", "captcha_required", "captcha_invalid", "rate_limit",
  "update_required", "gateway_error", "no_config", "no_own_server", "port_in_use", "invalid_settings", "geo_missing",
  "unsupported_link", "invalid_link", "not_running", "network", "geo_busy", "json_only", "update_invalid", "no_chain",
  "server_in_use", "chain_empty", "chain_too_long", "chain_amnezia_twice", "hop_not_chainable", "sysproxy",
  "tun_declined", "tun_failed",
]);
// Codes whose text says everything; others add the technical detail.
const plain = new Set([
  "device_limit", "captcha_required", "not_running", "no_chain", "geo_busy", "json_only", "chain_empty",
  "chain_too_long", "chain_amnezia_twice", "hop_not_chainable", "tun_declined",
]);

export function errorText(e: CoreError | unknown): string {
  const ce = e && typeof e === "object" && "code" in (e as object) ? (e as CoreError) : coreError(e);
  const base = t(codes.has(ce.code) ? `err_${ce.code}` : "err_internal");
  return !ce.detail || plain.has(ce.code) ? base : `${base}\n${ce.detail}`;
}
