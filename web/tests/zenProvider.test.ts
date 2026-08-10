import { afterEach, describe, expect, test } from 'bun:test';
import { PROVIDER_LOGOS } from '../src/features/providers/brandLogos';
import { PROVIDER_BRAND_ORDER, PROVIDER_DESCRIPTORS } from '../src/features/providers/descriptors';
import {
  ZEN_AFFILIATE_URL,
  ZEN_DEFAULT_BASE_URL,
  ZEN_DISPLAY_NAME,
  ZEN_PROVIDER_NAME,
  extractZenEntries,
} from '../src/features/providers/zen';
import { groupZenEntries, zenToResource } from '../src/features/providers/adapters';
import { apiCallApi } from '../src/services/api/apiCall';
import { apiClient } from '../src/services/api/client';
import { modelsApi } from '../src/services/api/models';
import { providersApi } from '../src/services/api/providers';
import { useConfigStore } from '../src/stores/useConfigStore';
import type { Config, ProviderKeyConfig } from '../src/types';

const originalApiCallRequest = apiCallApi.request;
const originalGet = apiClient.get;
const originalPut = apiClient.put;

afterEach(() => {
  apiCallApi.request = originalApiCallRequest;
  apiClient.get = originalGet;
  apiClient.put = originalPut;
  useConfigStore.setState({ config: null });
});

describe('OpenCode Zen provider', () => {
  test('exposes the correct constants', () => {
    expect(ZEN_PROVIDER_NAME).toBe('zen');
    expect(ZEN_DISPLAY_NAME).toBe('OpenCode Zen');
    expect(ZEN_AFFILIATE_URL).toBe('https://opencode.ai/zen');
    expect(ZEN_DEFAULT_BASE_URL).toBe('https://opencode.ai/zen/v1');
  });

  test('is registered as a Quick Fill brand with a logo', () => {
    expect(PROVIDER_BRAND_ORDER).toContain('zen');
    expect(PROVIDER_DESCRIPTORS.zen).toBeDefined();
    expect(PROVIDER_DESCRIPTORS.zen.supportsApiKey).toBeFalse();
    expect(PROVIDER_DESCRIPTORS.zen.supportsApiKeyEntries).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.zen.supportsModels).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.zen.supportsTestModel).toBeTrue();
    expect(PROVIDER_LOGOS.zen.src).toBeDefined();
  });

  test('extracts Zen entries from config.zenApiKeys', () => {
    const config: Config = {
      zenApiKeys: [
        { apiKey: 'oc-1', baseUrl: ZEN_DEFAULT_BASE_URL },
        { apiKey: 'oc-2', models: [{ name: 'glm-5.2', alias: 'glm' }] },
      ],
    };
    const entries = extractZenEntries(config);
    expect(entries).toHaveLength(2);
    expect(entries[0].config.apiKey).toBe('oc-1');
    expect(entries[0].index).toBe(0);
    expect(entries[1].config.models).toHaveLength(1);
  });

  test('returns empty list when no Zen entries exist', () => {
    expect(extractZenEntries(undefined)).toEqual([]);
    expect(extractZenEntries({})).toEqual([]);
  });

  test('zenToResource produces a usable descriptor', () => {
    const keyConfig: ProviderKeyConfig = {
      apiKey: 'oc-abcdef123456',
      baseUrl: ZEN_DEFAULT_BASE_URL,
      models: [{ name: 'deepseek-v4-pro', alias: 'dsv4-pro' }],
    };
    const resource = zenToResource(keyConfig, 3);
    expect(resource.brand).toBe('zen');
    expect(resource.originalIndex).toBe(3);
    expect(resource.name).toBe(ZEN_DISPLAY_NAME);
    expect(resource.apiKeyPreview).toBe('oc******56');
    expect(resource.models).toEqual(['deepseek-v4-pro']);
    expect(resource.modelCount).toBe(1);
    const sel = resource.selector;
    if (sel.brand !== 'zen') {
      throw new Error(`unexpected selector brand ${sel.brand}`);
    }
    expect(sel.apiKey).toBe('oc-abcdef123456');
    expect(sel.baseUrl).toBe(ZEN_DEFAULT_BASE_URL);
    expect(sel.index).toBe(3);
    expect(resource.apiKeyEntryCount).toBe(1);
    expect((resource.raw as ProviderKeyConfig).apiKeyEntries?.map((entry) => entry.apiKey)).toEqual(
      ['oc-abcdef123456']
    );
  });

  test('groupZenEntries aggregates keys with matching config into one entry', () => {
    const config: Config = {
      zenApiKeys: [
        { apiKey: 'oc-1', baseUrl: ZEN_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'oc-2', baseUrl: ZEN_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'oc-3', baseUrl: 'https://other.zen.example' },
      ],
    };
    const groups = groupZenEntries(config);
    expect(groups).toHaveLength(2);

    expect(groups[0].indices).toEqual([0, 1]);
    expect(groups[0].raw.apiKeyEntries).toEqual([
      { apiKey: 'oc-1', proxyUrl: undefined, authIndex: undefined },
      { apiKey: 'oc-2', proxyUrl: undefined, authIndex: undefined },
    ]);
    expect(groups[0].raw.prefix).toBe('team');
    expect(groups[0].raw.baseUrl).toBe(ZEN_DEFAULT_BASE_URL);

    expect(groups[1].indices).toEqual([2]);
    expect(groups[1].raw.apiKeyEntries).toEqual([
      { apiKey: 'oc-3', proxyUrl: undefined, authIndex: undefined },
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
        body: { data: [{ id: 'glm-5.2', object: 'model' }] },
      };
    }) as typeof apiCallApi.request;

    const list = await modelsApi.fetchV1ModelsViaApiCall(ZEN_DEFAULT_BASE_URL, 'oc-test-key');

    expect(requestedUrl).toBe('https://opencode.ai/zen/v1/models');
    expect(list.some((m) => m.name === 'glm-5.2')).toBeTrue();
  });

  test('preserves legacy websocket settings when updating Zen configuration', async () => {
    let saved: unknown;
    apiClient.get = (async () => ({
      'zen-api-key': [
        {
          'api-key': 'oc-test-key',
          'base-url': ZEN_DEFAULT_BASE_URL,
          websockets: true,
        },
      ],
    })) as typeof apiClient.get;
    apiClient.put = (async (_url: string, data?: unknown) => {
      saved = data;
    }) as typeof apiClient.put;

    await providersApi.updateZenConfig('oc-test-key', ZEN_DEFAULT_BASE_URL, {
      apiKey: 'oc-test-key',
      baseUrl: ZEN_DEFAULT_BASE_URL,
      prefix: 'team',
    });

    expect(saved).toEqual([
      {
        'api-key': 'oc-test-key',
        'base-url': ZEN_DEFAULT_BASE_URL,
        prefix: 'team',
        websockets: true,
      },
    ]);
  });

  test('updates the in-memory Zen configuration section', () => {
    useConfigStore.setState({
      config: {
        zenApiKeys: [{ apiKey: 'oc-old', baseUrl: ZEN_DEFAULT_BASE_URL }],
      },
    });

    useConfigStore.getState().updateConfigValue('zen-api-key', [
      { apiKey: 'oc-new', baseUrl: ZEN_DEFAULT_BASE_URL },
    ]);

    expect(useConfigStore.getState().config?.zenApiKeys).toEqual([
      { apiKey: 'oc-new', baseUrl: ZEN_DEFAULT_BASE_URL },
    ]);
  });
});
