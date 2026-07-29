import type { Config, ProviderKeyConfig } from '@/types';

export const NVIDIA_NIM_PROVIDER_NAME = 'nvidiaNim';
export const NVIDIA_NIM_DISPLAY_NAME = 'NVIDIA NIM';
export const NVIDIA_NIM_AFFILIATE_URL = 'https://build.nvidia.com/';
export const NVIDIA_NIM_DEFAULT_BASE_URL = 'https://integrate.api.nvidia.com/v1';

export const extractNvidiaNimEntries = (
  config: Config | null | undefined
): Array<{ config: ProviderKeyConfig; index: number }> =>
  (config?.nvidiaApiKeys ?? []).map((item, index) => ({ config: item, index }));
