import { afterEach, describe, expect, test } from 'bun:test';
import { PROVIDER_LOGOS } from '../src/features/providers/brandLogos';
import { PROVIDER_BRAND_ORDER, PROVIDER_DESCRIPTORS } from '../src/features/providers/descriptors';
import {
  ORCA_ROUTER_AFFILIATE_URL,
  ORCA_ROUTER_DEFAULT_BASE_URL,
  ORCA_ROUTER_DISPLAY_NAME,
  ORCA_ROUTER_PROVIDER_NAME,
  extractOrcaRouterEntries,
} from '../src/features/providers/orcaRouter';
import { groupOrcaRouterEntries, orcaRouterToResource } from '../src/features/providers/adapters';
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

describe('OrcaRouter provider', () => {
  test('exposes the correct constants', () => {
    expect(ORCA_ROUTER_PROVIDER_NAME).toBe('orcarouter');
    expect(ORCA_ROUTER_DISPLAY_NAME).toBe('OrcaRouter');
    expect(ORCA_ROUTER_AFFILIATE_URL).toBe('https://orcarouter.ai');
    expect(ORCA_ROUTER_DEFAULT_BASE_URL).toBe('https://api.orcarouter.ai/v1');
  });

  test('is registered as a Quick Fill brand with a logo', () => {
    expect(PROVIDER_BRAND_ORDER).toContain('orcaRouter');
    expect(PROVIDER_DESCRIPTORS.orcaRouter).toBeDefined();
    expect(PROVIDER_DESCRIPTORS.orcaRouter.supportsApiKey).toBeFalse();
    expect(PROVIDER_DESCRIPTORS.orcaRouter.supportsApiKeyEntries).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.orcaRouter.supportsModels).toBeTrue();
    expect(PROVIDER_DESCRIPTORS.orcaRouter.supportsTestModel).toBeTrue();
    expect(PROVIDER_LOGOS.orcaRouter.src).toBeDefined();
  });

  test('extracts OrcaRouter entries from config.orcaRouterApiKeys', () => {
    const config: Config = {
      orcaRouterApiKeys: [
        { apiKey: 'sk-orca-1', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL },
        { apiKey: 'sk-orca-2', models: [{ name: 'openai/gpt-4o-mini', alias: 'gpt-mini' }] },
      ],
    };
    const entries = extractOrcaRouterEntries(config);
    expect(entries).toHaveLength(2);
    expect(entries[0].config.apiKey).toBe('sk-orca-1');
    expect(entries[0].index).toBe(0);
    expect(entries[1].config.models).toHaveLength(1);
  });

  test('returns empty list when no OrcaRouter entries exist', () => {
    expect(extractOrcaRouterEntries(undefined)).toEqual([]);
    expect(extractOrcaRouterEntries({})).toEqual([]);
  });

  test('orcaRouterToResource produces a usable descriptor', () => {
    const keyConfig: ProviderKeyConfig = {
      apiKey: 'sk-orca-abcdef123456',
      baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL,
      models: [{ name: 'deepseek/deepseek-chat', alias: 'ds-chat' }],
    };
    const resource = orcaRouterToResource(keyConfig, 3);
    expect(resource.brand).toBe('orcaRouter');
    expect(resource.originalIndex).toBe(3);
    expect(resource.name).toBe(ORCA_ROUTER_DISPLAY_NAME);
    expect(resource.models).toEqual(['deepseek/deepseek-chat']);
    expect(resource.modelCount).toBe(1);
    const sel = resource.selector;
    if (sel.brand !== 'orcaRouter') {
      throw new Error(`unexpected selector brand ${sel.brand}`);
    }
    expect(sel.apiKey).toBe('sk-orca-abcdef123456');
    expect(sel.baseUrl).toBe(ORCA_ROUTER_DEFAULT_BASE_URL);
    expect(sel.index).toBe(3);
    expect(resource.apiKeyEntryCount).toBe(1);
    expect(
      (resource.raw as ProviderKeyConfig).apiKeyEntries?.map((entry) => entry.apiKey)
    ).toEqual(['sk-orca-abcdef123456']);
  });

  test('groupOrcaRouterEntries aggregates keys with matching config into one entry', () => {
    const config: Config = {
      orcaRouterApiKeys: [
        { apiKey: 'sk-orca-1', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'sk-orca-2', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL, prefix: 'team' },
        { apiKey: 'sk-orca-3', baseUrl: 'https://other.orcarouter.example' },
      ],
    };
    const groups = groupOrcaRouterEntries(config);
    expect(groups).toHaveLength(2);

    expect(groups[0].indices).toEqual([0, 1]);
    expect(groups[0].raw.apiKeyEntries).toEqual([
      { apiKey: 'sk-orca-1', proxyUrl: undefined, authIndex: undefined },
      { apiKey: 'sk-orca-2', proxyUrl: undefined, authIndex: undefined },
    ]);
    expect(groups[0].raw.prefix).toBe('team');
    expect(groups[0].raw.baseUrl).toBe(ORCA_ROUTER_DEFAULT_BASE_URL);

    expect(groups[1].indices).toEqual([2]);
    expect(groups[1].raw.apiKeyEntries).toEqual([
      { apiKey: 'sk-orca-3', proxyUrl: undefined, authIndex: undefined },
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
        body: { data: [{ id: 'openai/gpt-4o-mini', object: 'model' }] },
      };
    }) as typeof apiCallApi.request;

    const list = await modelsApi.fetchV1ModelsViaApiCall(
      ORCA_ROUTER_DEFAULT_BASE_URL,
      'sk-orca-test-key'
    );

    expect(requestedUrl).toBe('https://api.orcarouter.ai/v1/models');
    expect(list.some((m) => m.name === 'openai/gpt-4o-mini')).toBeTrue();
  });

  test('preserves legacy websocket settings when updating OrcaRouter configuration', async () => {
    let saved: unknown;
    apiClient.get = (async () => ({
      'orcarouter-api-key': [
        {
          'api-key': 'sk-orca-test-key',
          'base-url': ORCA_ROUTER_DEFAULT_BASE_URL,
          websockets: true,
        },
      ],
    })) as typeof apiClient.get;
    apiClient.put = (async (_url: string, data?: unknown) => {
      saved = data;
    }) as typeof apiClient.put;

    await providersApi.updateOrcaRouterConfig('sk-orca-test-key', ORCA_ROUTER_DEFAULT_BASE_URL, {
      apiKey: 'sk-orca-test-key',
      baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL,
      prefix: 'team',
    });

    expect(saved).toEqual([
      {
        'api-key': 'sk-orca-test-key',
        'base-url': ORCA_ROUTER_DEFAULT_BASE_URL,
        prefix: 'team',
        websockets: true,
      },
    ]);
  });

  test('updates the in-memory OrcaRouter configuration section', () => {
    useConfigStore.setState({
      config: {
        orcaRouterApiKeys: [{ apiKey: 'sk-orca-old', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL }],
      },
    });

    useConfigStore.getState().updateConfigValue('orcarouter-api-key', [
      { apiKey: 'sk-orca-new', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL },
    ]);

    expect(useConfigStore.getState().config?.orcaRouterApiKeys).toEqual([
      { apiKey: 'sk-orca-new', baseUrl: ORCA_ROUTER_DEFAULT_BASE_URL },
    ]);
  });
});
