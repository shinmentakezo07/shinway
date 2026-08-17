import { afterEach, describe, expect, test } from 'bun:test';
import { PROVIDER_LOGOS } from '../src/features/providers/brandLogos';
import { PROVIDER_BRAND_ORDER, PROVIDER_DESCRIPTORS } from '../src/features/providers/descriptors';
import {
  TOKEN_ROUTER_AFFILIATE_URL,
  TOKEN_ROUTER_DEFAULT_BASE_URL,
  TOKEN_ROUTER_DISPLAY_NAME,
  TOKEN_ROUTER_PROVIDER_NAME,
  extractTokenRouterEntries,
} from '../src/features/providers/tokenRouter';
import { groupTokenRouterEntries, tokenRouterToResource } from '../src/features/providers/adapters';
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

describe('TokenRouter provider', () => {
  test('exposes the correct constants', () => {
    expect(TOKEN_ROUTER_PROVIDER_NAME).toBe('tokenrouter');
    expect(TOKEN_ROUTER_DISPLAY_NAME).toBe('TokenRouter');
    expect(TOKEN_ROUTER_AFFILIATE_URL).toBe('https://www.tokenrouter.com');
    expect(TOKEN_ROUTER_DEFAULT_BASE_URL).toBe('https://api.tokenrouter.com/v1');
  });

  test('is registered as a Quick Fill brand with a logo', () => {
    expect(PROVIDER_BRAND_ORDER).toContain('tokenRouter');
    expect(PROVIDER_DESCRIPTORS.tokenRouter).toBeDefined();
    expect(PROVIDER_DESCRIPTORS.tokenRouter.supportsApiKey).toBeFalse();
    expect(PROVIDER_DESCRIPTORS.tokenRouter.supportsApiKeyEntries).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.tokenRouter.supportsModels).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.tokenRouter.supportsTestModel).toBeTrue();
    expect(PROVIDER_LOGOS.tokenRouter.src).toBeDefined();
  });

  test('extracts TokenRouter entries from config.tokenRouterApiKeys', () => {
    const config: Config = {
      tokenRouterApiKeys: [
        { apiKey: 'tr-1', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL },
        { apiKey: 'tr-2', models: [{ name: 'anthropic/claude-sonnet-4.6', alias: 'claude' }] },
      ],
    };
    const entries = extractTokenRouterEntries(config);
    expect(entries).toHaveLength(2);
    expect(entries[0].config.apiKey).toBe('tr-1');
    expect(entries[0].index).toBe(0);
    expect(entries[1].config.models).toHaveLength(1);
  });

  test('returns empty list when no TokenRouter entries exist', () => {
    expect(extractTokenRouterEntries(undefined)).toEqual([]);
    expect(extractTokenRouterEntries({})).toEqual([]);
  });

  test('tokenRouterToResource produces a usable descriptor', () => {
    const keyConfig: ProviderKeyConfig = {
      apiKey: 'tr-abcdef123456',
      baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL,
      models: [{ name: 'deepseek/deepseek-v4-pro-0813', alias: 'dsv4-pro' }],
    };
    const resource = tokenRouterToResource(keyConfig, 3);
    expect(resource.brand).toBe('tokenRouter');
    expect(resource.originalIndex).toBe(3);
    expect(resource.name).toBe(TOKEN_ROUTER_DISPLAY_NAME);
    expect(resource.models).toEqual(['deepseek/deepseek-v4-pro-0813']);
    expect(resource.modelCount).toBe(1);
    const sel = resource.selector;
    if (sel.brand !== 'tokenRouter') {
      throw new Error(`unexpected selector brand ${sel.brand}`);
    }
    expect(sel.apiKey).toBe('tr-abcdef123456');
    expect(sel.baseUrl).toBe(TOKEN_ROUTER_DEFAULT_BASE_URL);
    expect(sel.index).toBe(3);
    expect(resource.apiKeyEntryCount).toBe(1);
    expect(
      (resource.raw as ProviderKeyConfig).apiKeyEntries?.map((entry) => entry.apiKey)
    ).toEqual(['tr-abcdef123456']);
  });

  test('groupTokenRouterEntries aggregates keys with matching config into one entry', () => {
    const config: Config = {
      tokenRouterApiKeys: [
        { apiKey: 'tr-1', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'tr-2', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'tr-3', baseUrl: 'https://other.tokenrouter.example' },
      ],
    };
    const groups = groupTokenRouterEntries(config);
    expect(groups).toHaveLength(2);

    expect(groups[0].indices).toEqual([0, 1]);
    expect(groups[0].raw.apiKeyEntries).toEqual([
      { apiKey: 'tr-1', proxyUrl: undefined, authIndex: undefined },
      { apiKey: 'tr-2', proxyUrl: undefined, authIndex: undefined },
    ]);
    expect(groups[0].raw.prefix).toBe('team');
    expect(groups[0].raw.baseUrl).toBe(TOKEN_ROUTER_DEFAULT_BASE_URL);

    expect(groups[1].indices).toEqual([2]);
    expect(groups[1].raw.apiKeyEntries).toEqual([
      { apiKey: 'tr-3', proxyUrl: undefined, authIndex: undefined },
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
        body: { data: [{ id: 'anthropic/claude-sonnet-4.6', object: 'model' }] },
      };
    }) as typeof apiCallApi.request;

    const list = await modelsApi.fetchV1ModelsViaApiCall(
      TOKEN_ROUTER_DEFAULT_BASE_URL,
      'tr-test-key'
    );

    expect(requestedUrl).toBe('https://api.tokenrouter.com/v1/models');
    expect(list.some((m) => m.name === 'anthropic/claude-sonnet-4.6')).toBeTrue();
  });

  test('preserves legacy websocket settings when updating TokenRouter configuration', async () => {
    let saved: unknown;
    apiClient.get = (async () => ({
      'tokenrouter-api-key': [
        {
          'api-key': 'tr-test-key',
          'base-url': TOKEN_ROUTER_DEFAULT_BASE_URL,
          websockets: true,
        },
      ],
    })) as typeof apiClient.get;
    apiClient.put = (async (_url: string, data?: unknown) => {
      saved = data;
    }) as typeof apiClient.put;

    await providersApi.updateTokenRouterConfig('tr-test-key', TOKEN_ROUTER_DEFAULT_BASE_URL, {
      apiKey: 'tr-test-key',
      baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL,
      prefix: 'team',
    });

    expect(saved).toEqual([
      {
        'api-key': 'tr-test-key',
        'base-url': TOKEN_ROUTER_DEFAULT_BASE_URL,
        prefix: 'team',
        websockets: true,
      },
    ]);
  });

  test('updates the in-memory TokenRouter configuration section', () => {
    useConfigStore.setState({
      config: {
        tokenRouterApiKeys: [{ apiKey: 'tr-old', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL }],
      },
    });

    useConfigStore.getState().updateConfigValue('tokenrouter-api-key', [
      { apiKey: 'tr-new', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL },
    ]);

    expect(useConfigStore.getState().config?.tokenRouterApiKeys).toEqual([
      { apiKey: 'tr-new', baseUrl: TOKEN_ROUTER_DEFAULT_BASE_URL },
    ]);
  });
});
