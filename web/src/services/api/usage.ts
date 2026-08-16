import { apiClient } from './client';

const USAGE_TIMEOUT_MS = 30 * 1000;

export interface UsageSummary {
  from: string;
  to: string;
  total_requests: number;
  failed_requests: number;
  succeeded_requests: number;
  total_input_tokens: number;
  total_output_tokens: number;
  total_reasoning_tokens: number;
  total_cached_tokens: number;
  total_tokens: number;
  avg_latency_ms: number;
  avg_ttft_ms: number;
  unique_models: number;
  unique_api_keys: number;
  unique_auth_files: number;
}

export interface UsagePoint {
  bucket: string;
  requests: number;
  failed: number;
  input_tokens: number;
  output_tokens: number;
  reasoning_tokens: number;
  cached_tokens: number;
  total_tokens: number;
}

export interface UsageGroupStat {
  key: string;
  requests: number;
  failed: number;
  input_tokens: number;
  output_tokens: number;
  reasoning_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  avg_latency_ms: number;
}

export interface UsageStatsResponse {
  summary: UsageSummary;
  bucket_seconds: number;
  series: UsagePoint[];
  by_model: UsageGroupStat[];
  by_provider: UsageGroupStat[];
  by_api_key: UsageGroupStat[];
  by_auth: UsageGroupStat[];
}

export interface UsageRecord {
  TS: string;
  RequestID: string;
  Provider: string;
  ExecutorType: string;
  Model: string;
  Alias: string;
  Endpoint: string;
  AuthType: string;
  AuthID: string;
  AuthIndex: string;
  APIKey: string;
  Source: string;
  ReasoningEffort: string;
  ServiceTier: string;
  ResponseServiceTier: string;
  ClientIP: string;
  UserAgent: string;
  LatencyMs: number;
  TTFtMs: number;
  Failed: boolean;
  Generate: boolean;
  FailStatus: number;
  FailBody: string;
  InputTokens: number;
  OutputTokens: number;
  ReasoningTokens: number;
  CachedTokens: number;
  CacheReadTokens: number;
  CacheCreationTokens: number;
  TotalTokens: number;
}

export interface UsageRecordsResponse {
  total: number;
  limit: number;
  offset: number;
  records: UsageRecord[];
}

export type UsageRange = '1h' | '6h' | '24h' | '7d' | '30d' | 'all';

export interface UsageRecordsParams {
  range?: UsageRange;
  model?: string;
  provider?: string;
  api_key?: string;
  auth_id?: string;
  failed?: boolean;
  search?: string;
  limit?: number;
  offset?: number;
}

function buildQuery(params: Record<string, string | number | boolean | undefined>): string {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '' || value === null) continue;
    qs.set(key, String(value));
  }
  return qs.toString();
}

/**
 * Maps the UI range shorthand to query params. The backend treats a missing
 * from/to (and no `range`) as "all time", and rejects `range=all` as an
 * invalid duration, so the all-time option must omit the param entirely.
 */
export function usageRangeQuery(range: UsageRange | undefined): { range?: string } {
  if (!range || range === 'all') return {};
  return { range };
}

export const usageApi = {
  getStats: (range: UsageRange = '24h') =>
    apiClient.get<UsageStatsResponse>(`/usage-stats?${buildQuery(usageRangeQuery(range))}`, {
      timeout: USAGE_TIMEOUT_MS,
    }),
  getRecords: (params: UsageRecordsParams = {}) => {
    const { range, ...rest } = params;
    return apiClient.get<UsageRecordsResponse>(
      `/usage-records?${buildQuery({ ...usageRangeQuery(range), ...rest } as Record<string, string | number | boolean | undefined>)}`,
      {
        timeout: USAGE_TIMEOUT_MS,
      }
    );
  },
  purge: (beforeMs: number) =>
    apiClient.delete<{ deleted: number }>(`/usage-records?${buildQuery({ before: beforeMs })}`, {
      timeout: USAGE_TIMEOUT_MS,
    }),
};
