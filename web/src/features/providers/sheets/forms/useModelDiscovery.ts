import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';
import { modelsApi } from '@/services/api';
import { buildHeaderObject } from '@/utils/headers';
import { getErrorMessage } from '@/utils/helpers';
import { asyncFlags, asyncReducer, initialAsyncState } from '@/utils/asyncState';
import { withZenIdentityHeaders } from '../../zen';
import type { ModelInfo } from '@/utils/models';
import type { ApiKeyEntryInput, ProviderBrand } from '../../types';

export const MODEL_DISCOVERY_BRANDS: ReadonlyArray<ProviderBrand> = [
  'gemini',
  'codex',
  'xai',
  'nvidiaNim',
  'zen',
  'tokenRouter',
  'orcaRouter',
  'claude',
  'claudeApi',
  'openaiCompatibility',
];

export const isModelDiscoveryBrand = (brand: ProviderBrand): boolean =>
  MODEL_DISCOVERY_BRANDS.includes(brand);

export interface UseModelDiscoveryArgs {
  brand: ProviderBrand;
  baseUrl: string;
  formHeaders: Array<{ key: string; value: string }>;
  apiKeyEntries?: ApiKeyEntryInput[];
  apiKey?: string;
  fallbackApiKey?: string;
  authIndex?: string;
}

export interface UseModelDiscoveryResult {
  available: boolean;
  loading: boolean;
  error: string | null;
  models: ModelInfo[];
  hasFetched: boolean;
  fetch: () => Promise<void>;
  reset: () => void;
}

/**
 * Model discovery lifecycle.
 *
 * Previously this was four independent flags (`loading`, `error`, `models`,
 * `hasFetched`) set in scattered branches. Illegal combinations were
 * representable — notably a superseded request writing a stale `success` or
 * `error` over a newer attempt (`loading && error`, or stale models shown
 * for changed credentials). It is now a single explicit state machine via
 * `asyncReducer`, with an attempt token that discards late responses.
 *
 * The returned shape is unchanged so callers need no migration.
 */
export function useModelDiscovery(args: UseModelDiscoveryArgs): UseModelDiscoveryResult {
  const { brand, baseUrl, formHeaders, apiKeyEntries, apiKey, fallbackApiKey, authIndex } = args;

  const available = isModelDiscoveryBrand(brand);
  const [state, dispatch] = useReducer(
    asyncReducer<ModelInfo[]>,
    undefined,
    initialAsyncState<ModelInfo[]>
  );
  const attemptRef = useRef(0);

  const fetchModels = useCallback(async () => {
    if (!available) return;
    const attempt = (attemptRef.current += 1);
    dispatch({ type: 'start' });
    try {
      const baseHeaders =
        brand === 'zen'
          ? withZenIdentityHeaders(buildHeaderObject(formHeaders))
          : buildHeaderObject(formHeaders);
      const resolvedAuthIndex = (authIndex ?? '').trim() || undefined;
      let next: ModelInfo[] = [];
      if (brand === 'gemini') {
        const key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        next = await modelsApi.fetchGeminiModelsViaApiCall(
          baseUrl,
          key,
          baseHeaders,
          resolvedAuthIndex
        );
      } else if (
        brand === 'codex' ||
        brand === 'xai' ||
        brand === 'nvidiaNim' ||
        brand === 'zen' ||
        brand === 'tokenRouter' ||
        brand === 'orcaRouter'
      ) {
        let key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        let entryAuthIndex = resolvedAuthIndex;
        if (
          brand === 'nvidiaNim' ||
          brand === 'zen' ||
          brand === 'tokenRouter' ||
          brand === 'orcaRouter'
        ) {
          const firstEntry = (apiKeyEntries ?? []).find(
            (entry) => (entry.apiKey ?? '').trim() || (entry.existingApiKey ?? '').trim()
          );
          const entryKey =
            (firstEntry?.apiKey ?? '').trim() || (firstEntry?.existingApiKey ?? '').trim();
          if (entryKey) {
            key = entryKey;
          }
          const entryAuthIdx = (firstEntry?.authIndex ?? '').trim();
          if (entryAuthIdx) {
            entryAuthIndex = entryAuthIdx;
          }
        }
        next = await modelsApi.fetchV1ModelsViaApiCall(baseUrl, key, baseHeaders, entryAuthIndex);
      } else if (brand === 'claude' || brand === 'claudeApi') {
        const key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        next = await modelsApi.fetchClaudeModelsViaApiCall(
          baseUrl,
          key,
          baseHeaders,
          resolvedAuthIndex
        );
      } else if (brand === 'openaiCompatibility') {
        const firstEntry = (apiKeyEntries ?? []).find(
          (e) =>
            (e.apiKey ?? '').trim() || (e.existingApiKey ?? '').trim() || (e.authIndex ?? '').trim()
        );
        const entryKey =
          (firstEntry?.apiKey ?? '').trim() || (firstEntry?.existingApiKey ?? '').trim();
        const entryAuthIndex = (firstEntry?.authIndex ?? '').trim() || resolvedAuthIndex;
        try {
          next = await modelsApi.fetchModelsViaApiCall(
            baseUrl,
            entryKey,
            baseHeaders,
            entryAuthIndex
          );
        } catch (firstErr) {
          // Some OpenAI-compatible endpoints expose /models without auth, or
          // reject the configured key for the discovery route. Retry once
          // without any auth/headers before surfacing the original error.
          try {
            next = await modelsApi.fetchModelsViaApiCall(baseUrl);
          } catch {
            throw firstErr;
          }
        }
      }
      // A newer attempt (or a reset from changed inputs) superseded this one.
      // Discard the result rather than writing stale data over fresh state.
      if (attempt !== attemptRef.current) return;
      dispatch({ type: 'success', data: next ?? [] });
    } catch (err) {
      if (attempt !== attemptRef.current) return;
      dispatch({
        type: 'failure',
        error: getErrorMessage(err) || 'Failed to fetch models',
      });
    }
  }, [available, apiKey, apiKeyEntries, authIndex, baseUrl, brand, fallbackApiKey, formHeaders]);

  const reset = useCallback(() => {
    attemptRef.current += 1;
    dispatch({ type: 'reset' });
  }, []);

  const inputSignature = useMemo(() => {
    const headerSig = formHeaders.map((h) => `${h.key}:${h.value}`).join('|');
    const entriesSig = (apiKeyEntries ?? [])
      .map((e) => `${e.apiKey ?? ''}::${e.existingApiKey ?? ''}::${e.authIndex ?? ''}`)
      .join('|');
    return [
      baseUrl,
      apiKey ?? '',
      fallbackApiKey ?? '',
      authIndex ?? '',
      headerSig,
      entriesSig,
    ].join('||');
  }, [apiKey, apiKeyEntries, authIndex, baseUrl, fallbackApiKey, formHeaders]);

  const lastSignatureRef = useRef(inputSignature);
  useEffect(() => {
    if (lastSignatureRef.current === inputSignature) return;
    lastSignatureRef.current = inputSignature;
    reset();
  }, [inputSignature, reset]);

  const flags = asyncFlags(state);
  return {
    available,
    loading: flags.isLoading,
    error: state.error,
    models: state.data ?? [],
    hasFetched: state.settled,
    fetch: fetchModels,
    reset,
  };
}
