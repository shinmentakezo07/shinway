import type { Config, ProviderKeyConfig } from '@/types';

export const ZEN_PROVIDER_NAME = 'zen';
export const ZEN_DISPLAY_NAME = 'OpenCode Zen';
export const ZEN_AFFILIATE_URL = 'https://opencode.ai/zen';
export const ZEN_DEFAULT_BASE_URL = 'https://opencode.ai/zen/v1';

export const extractZenEntries = (
  config: Config | null | undefined
): Array<{ config: ProviderKeyConfig; index: number }> =>
  (config?.zenApiKeys ?? []).map((item, index) => ({ config: item, index }));
