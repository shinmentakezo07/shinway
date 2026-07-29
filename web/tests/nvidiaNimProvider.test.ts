import { afterEach, describe, expect, test } from 'bun:test';
import { PROVIDER_LOGOS } from '../src/features/providers/brandLogos';
import { PROVIDER_BRAND_ORDER, PROVIDER_DESCRIPTORS } from '../src/features/providers/descriptors';
import {
  NVIDIA_NIM_AFFILIATE_URL,
  NVIDIA_NIM_DEFAULT_BASE_URL,
  NVIDIA_NIM_DISPLAY_NAME,
  NVIDIA_NIM_PROVIDER_NAME,
  extractNvidiaNimEntries,
} from '../src/features/providers/nvidiaNim';
import { groupNvidiaEntries, nvidiaToResource } from '../src/features/providers/adapters';
import { apiCallApi } from '../src/services/api/apiCall';
import { modelsApi } from '../src/services/api/models';
import type { Config, ProviderKeyConfig } from '../src/types';

const originalApiCallRequest = apiCallApi.request;

afterEach(() => {
  apiCallApi.request = originalApiCallRequest;
});

describe('NVIDIA NIM provider', () => {
  test('exposes the correct constants', () => {
    expect(NVIDIA_NIM_PROVIDER_NAME).toBe('nvidiaNim');
    expect(NVIDIA_NIM_DISPLAY_NAME).toBe('NVIDIA NIM');
    expect(NVIDIA_NIM_AFFILIATE_URL).toBe('https://build.nvidia.com/');
    expect(NVIDIA_NIM_DEFAULT_BASE_URL).toBe('https://integrate.api.nvidia.com/v1');
  });

  test('is registered as a Quick Fill brand with a logo', () => {
    expect(PROVIDER_BRAND_ORDER).toContain('nvidiaNim');
    expect(PROVIDER_DESCRIPTORS.nvidiaNim).toBeDefined();
    expect(PROVIDER_DESCRIPTORS.nvidiaNim.supportsApiKey).toBeFalse();
    expect(PROVIDER_DESCRIPTORS.nvidiaNim.supportsApiKeyEntries).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.nvidiaNim.supportsModels).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.nvidiaNim.supportsTestModel).toBeTrue();
    expect(PROVIDER_LOGOS.nvidiaNim.src).toBeDefined();
  });

  test('extracts NVIDIA entries from config.nvidiaApiKeys', () => {
    const config: Config = {
      nvidiaApiKeys: [
        { apiKey: 'nvapi-1', baseUrl: NVIDIA_NIM_DEFAULT_BASE_URL },
        { apiKey: 'nvapi-2', models: [{ name: 'deepseek-ai/deepseek-v4-flash', alias: 'ds' }] },
      ],
    };
    const entries = extractNvidiaNimEntries(config);
    expect(entries).toHaveLength(2);
    expect(entries[0].config.apiKey).toBe('nvapi-1');
    expect(entries[0].index).toBe(0);
    expect(entries[1].config.models).toHaveLength(1);
  });

  test('returns empty list when no NVIDIA entries exist', () => {
    expect(extractNvidiaNimEntries(undefined)).toEqual([]);
    expect(extractNvidiaNimEntries({})).toEqual([]);
  });

  test('nvidiaToResource produces a usable descriptor', () => {
    const keyConfig: ProviderKeyConfig = {
      apiKey: 'nvapi-abcdef123456',
      baseUrl: NVIDIA_NIM_DEFAULT_BASE_URL,
      models: [{ name: 'nvidia/llama-3.3-nemotron-super-49b-v1', alias: 'nemotron' }],
    };
    const resource = nvidiaToResource(keyConfig, 3);
    expect(resource.brand).toBe('nvidiaNim');
    expect(resource.originalIndex).toBe(3);
    expect(resource.name).toBe(NVIDIA_NIM_DISPLAY_NAME);
    expect(resource.apiKeyPreview).toBe('nv******56');
    expect(resource.models).toEqual(['nvidia/llama-3.3-nemotron-super-49b-v1']);
    expect(resource.modelCount).toBe(1);
    const sel = resource.selector;
    if (sel.brand !== 'nvidiaNim') {
      throw new Error(`unexpected selector brand ${sel.brand}`);
    }
    expect(sel.apiKey).toBe('nvapi-abcdef123456');
    expect(sel.baseUrl).toBe(NVIDIA_NIM_DEFAULT_BASE_URL);
    expect(sel.index).toBe(3);
    expect(resource.apiKeyEntryCount).toBe(1);
    expect(
      (resource.raw as ProviderKeyConfig).apiKeyEntries?.map((entry) => entry.apiKey)
    ).toEqual(['nvapi-abcdef123456']);
  });

  test('groupNvidiaEntries aggregates keys with matching config into one entry', () => {
    const config: Config = {
      nvidiaApiKeys: [
        { apiKey: 'nvapi-1', baseUrl: NVIDIA_NIM_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'nvapi-2', baseUrl: NVIDIA_NIM_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'nvapi-3', baseUrl: 'https://other.nvidia.example' },
      ],
    };
    const groups = groupNvidiaEntries(config);
    expect(groups).toHaveLength(2);

    expect(groups[0].indices).toEqual([0, 1]);
    expect(groups[0].raw.apiKeyEntries).toEqual([
      { apiKey: 'nvapi-1', proxyUrl: undefined, authIndex: undefined },
      { apiKey: 'nvapi-2', proxyUrl: undefined, authIndex: undefined },
    ]);
    expect(groups[0].raw.prefix).toBe('team');
    expect(groups[0].raw.baseUrl).toBe(NVIDIA_NIM_DEFAULT_BASE_URL);

    expect(groups[1].indices).toEqual([2]);
    expect(groups[1].raw.apiKeyEntries).toEqual([
      { apiKey: 'nvapi-3', proxyUrl: undefined, authIndex: undefined },
    ]);
  });

  test('fetches the catalog through the versioned OpenAI endpoint', async () => {
    let requestedUrl = '';
    apiCallApi.request = (async (payload) => {
      requestedUrl = payload.url;
      return {
        statusCode: 200,
        header: {},
        bodyText: '',
        body: { data: [{ id: 'nvidia/llama-3.3-nemotron-super-49b-v1', object: 'model' }] },
      };
    }) as typeof apiCallApi.request;

    const list = await modelsApi.fetchV1ModelsViaApiCall(
      NVIDIA_NIM_DEFAULT_BASE_URL,
      'nvapi-test-key'
    );

    expect(requestedUrl).toBe('https://integrate.api.nvidia.com/v1/models');
    expect(list.some((m) => m.name === 'nvidia/llama-3.3-nemotron-super-49b-v1')).toBeTrue();
  });
});
