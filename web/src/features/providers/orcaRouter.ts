import type { Config, ProviderKeyConfig } from '@/types';

export const ORCA_ROUTER_PROVIDER_NAME = 'orcarouter';
export const ORCA_ROUTER_DISPLAY_NAME = 'OrcaRouter';
export const ORCA_ROUTER_AFFILIATE_URL = 'https://orcarouter.ai';
export const ORCA_ROUTER_DEFAULT_BASE_URL = 'https://api.orcarouter.ai/v1';

export const extractOrcaRouterEntries = (
  config: Config | null | undefined
): Array<{ config: ProviderKeyConfig; index: number }> =>
  (config?.orcaRouterApiKeys ?? []).map((item, index) => ({ config: item, index }));
