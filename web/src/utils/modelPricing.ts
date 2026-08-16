/**
 * Client-side model pricing for per-request cost estimates in the usage log.
 *
 * The backend does not track prices, so costs are estimated from list pricing
 * (USD per 1M tokens). Reasoning tokens are billed at the output rate.
 * Unknown models fall back to a conservative default rate and are flagged as
 * estimates; models with "free" in their id cost nothing.
 */

export interface ModelPricing {
  /** USD per 1M input tokens. */
  input: number;
  /** USD per 1M output tokens (reasoning billed at this rate). */
  output: number;
  /** True when the rate came from the fallback table, not a known model. */
  estimated: boolean;
  /** True when the model is a free tier. */
  free: boolean;
}

export interface CostEstimate extends ModelPricing {
  /** Estimated cost for one request in USD. */
  cost: number;
}

interface PricingEntry {
  match: string[];
  input: number;
  output: number;
}

// First match wins; keep more specific patterns above broader ones.
const PRICING_TABLE: PricingEntry[] = [
  // Anthropic
  { match: ['claude-opus-4', 'claude-3-opus'], input: 15, output: 75 },
  {
    match: ['claude-sonnet-4', 'claude-3-7-sonnet', 'claude-3-5-sonnet'],
    input: 3,
    output: 15,
  },
  { match: ['claude-haiku-4', 'claude-3-5-haiku'], input: 1, output: 5 },
  { match: ['claude-3-haiku'], input: 0.25, output: 1.25 },
  // Google
  { match: ['gemini-2.5-pro', 'gemini-2.0-pro', 'gemini-1.5-pro'], input: 1.25, output: 10 },
  { match: ['gemini-2.5-flash-lite', 'gemini-2.0-flash-lite'], input: 0.1, output: 0.4 },
  { match: ['gemini-2.5-flash'], input: 0.3, output: 2.5 },
  { match: ['gemini-2.0-flash'], input: 0.1, output: 0.4 },
  { match: ['gemini-1.5-flash'], input: 0.075, output: 0.3 },
  // OpenAI
  { match: ['gpt-5-nano'], input: 0.05, output: 0.4 },
  { match: ['gpt-5-mini'], input: 0.25, output: 2 },
  { match: ['gpt-5'], input: 1.25, output: 10 },
  { match: ['o3-mini', 'o4-mini'], input: 1.1, output: 4.4 },
  { match: ['o3'], input: 10, output: 40 },
  { match: ['o1'], input: 15, output: 60 },
  { match: ['gpt-4.1-nano'], input: 0.1, output: 0.4 },
  { match: ['gpt-4.1-mini'], input: 0.4, output: 1.6 },
  { match: ['gpt-4.1'], input: 2, output: 8 },
  { match: ['gpt-4o-mini'], input: 0.15, output: 0.6 },
  { match: ['gpt-4o'], input: 2.5, output: 10 },
  // DeepSeek
  { match: ['deepseek-reasoner', 'deepseek-r1'], input: 0.55, output: 2.19 },
  { match: ['deepseek-chat', 'deepseek-v3', 'deepseek-v4'], input: 0.27, output: 1.1 },
  // Zhipu GLM
  { match: ['glm-4.5', 'glm-4.6', 'glm-5', 'chatglm'], input: 0.6, output: 2.2 },
  // Moonshot
  { match: ['kimi'], input: 0.6, output: 2.5 },
  // xAI
  { match: ['grok-3-mini', 'grok-4-mini'], input: 0.3, output: 0.5 },
  { match: ['grok-4'], input: 3, output: 15 },
  { match: ['grok-3'], input: 3, output: 15 },
  { match: ['grok-2'], input: 2, output: 10 },
  // Qwen
  { match: ['qwen3-max', 'qwen-max'], input: 1.2, output: 6 },
  { match: ['qwen3-coder'], input: 0.4, output: 1.6 },
  { match: ['qwen'], input: 0.4, output: 1.2 },
];

const FREE_MARKERS = ['free'];

const FALLBACK_PRICING = { input: 0.5, output: 2 };

/** Resolve the per-million rates for a model id (case-insensitive). */
export function resolveModelPricing(model: string): ModelPricing {
  const id = (model || '').toLowerCase();
  if (FREE_MARKERS.some((marker) => id.includes(marker))) {
    return { input: 0, output: 0, estimated: false, free: true };
  }
  for (const entry of PRICING_TABLE) {
    if (entry.match.some((pattern) => id.includes(pattern))) {
      return { input: entry.input, output: entry.output, estimated: false, free: false };
    }
  }
  return { ...FALLBACK_PRICING, estimated: true, free: false };
}

/**
 * Estimate the USD cost of one request. Reasoning tokens are billed at the
 * output rate; cached tokens are not discounted (estimate stays simple).
 */
export function estimateRequestCost(
  model: string,
  inputTokens: number,
  outputTokens: number,
  reasoningTokens: number
): CostEstimate {
  const pricing = resolveModelPricing(model);
  const cost =
    (Math.max(0, inputTokens) * pricing.input +
      (Math.max(0, outputTokens) + Math.max(0, reasoningTokens)) * pricing.output) /
    1_000_000;
  return { cost, ...pricing };
}

/** Format a USD amount compactly for table cells. */
export function formatUsd(amount: number): string {
  if (!Number.isFinite(amount) || amount <= 0) return '$0.00';
  if (amount < 0.0001) return '<$0.0001';
  if (amount < 0.01) return `$${amount.toFixed(4)}`;
  if (amount < 1) return `$${amount.toFixed(3)}`;
  return `$${amount.toFixed(2)}`;
}
