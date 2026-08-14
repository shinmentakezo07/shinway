import type { Config, ProviderKeyConfig } from '@/types';
import { hasHeader } from '@/utils/headers';

export const ZEN_PROVIDER_NAME = 'zen';
export const ZEN_DISPLAY_NAME = 'OpenCode Zen';
export const ZEN_AFFILIATE_URL = 'https://opencode.ai/zen';
export const ZEN_DEFAULT_BASE_URL = 'https://opencode.ai/zen/v1';

// Default request-identity headers matching what the real opencode client sends
// to OpenCode Zen (mirrors internal/runtime/executor/zen_executor.go). The web
// panel sends these on Zen connectivity tests and model discovery so they
// exercise the same wire identity as proxied requests.
export const ZEN_DEFAULT_HTTP_REFERER = 'https://opencode.ai/';
export const ZEN_DEFAULT_X_TITLE = 'opencode';
export const ZEN_DEFAULT_USER_AGENT = 'opencode/1.18.18';

/**
 * Fills the opencode request-identity defaults (HTTP-Referer, X-Title,
 * User-Agent) into the given headers, only where the caller has not already set
 * a value (case-insensitive match). Mirrors the backend's "??=" semantics:
 * explicitly configured headers always win over the defaults.
 */
export const withZenIdentityHeaders = (headers: Record<string, string>): Record<string, string> => {
  const out = { ...headers };
  const identityDefaults: Array<[string, string]> = [
    ['HTTP-Referer', ZEN_DEFAULT_HTTP_REFERER],
    ['X-Title', ZEN_DEFAULT_X_TITLE],
    ['User-Agent', ZEN_DEFAULT_USER_AGENT],
  ];
  for (const [name, value] of identityDefaults) {
    if (!hasHeader(out, name)) {
      out[name] = value;
    }
  }
  return out;
};

export const extractZenEntries = (
  config: Config | null | undefined
): Array<{ config: ProviderKeyConfig; index: number }> =>
  (config?.zenApiKeys ?? []).map((item, index) => ({ config: item, index }));
