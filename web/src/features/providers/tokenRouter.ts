import type { Config, ProviderKeyConfig } from '@/types';

export const TOKEN_ROUTER_PROVIDER_NAME = 'tokenrouter';
export const TOKEN_ROUTER_DISPLAY_NAME = 'TokenRouter';
export const TOKEN_ROUTER_AFFILIATE_URL = 'https://www.tokenrouter.com';
export const TOKEN_ROUTER_DEFAULT_BASE_URL = 'https://api.tokenrouter.com/v1';

export const extractTokenRouterEntries = (
  config: Config | null | undefined
): Array<{ config: ProviderKeyConfig; index: number }> =>
  (config?.tokenRouterApiKeys ?? []).map((item, index) => ({ config: item, index }));
