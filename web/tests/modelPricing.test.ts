import { describe, expect, test } from 'bun:test';
import {
  estimateRequestCost,
  formatUsd,
  resolveModelPricing,
} from '../src/utils/modelPricing';

describe('model pricing resolution', () => {
  test('matches known models case-insensitively', () => {
    const pricing = resolveModelPricing('Claude-Sonnet-4-5-20250929');
    expect(pricing.input).toBe(3);
    expect(pricing.output).toBe(15);
    expect(pricing.estimated).toBe(false);
    expect(pricing.free).toBe(false);
  });

  test('flags free models as zero cost', () => {
    const pricing = resolveModelPricing('deepseek-v4-flash-free');
    expect(pricing.free).toBe(true);
    expect(pricing.input).toBe(0);
    expect(pricing.output).toBe(0);
  });

  test('falls back to default rates for unknown models', () => {
    const pricing = resolveModelPricing('some-brand-new-model');
    expect(pricing.estimated).toBe(true);
    expect(pricing.input).toBeGreaterThan(0);
    expect(pricing.output).toBeGreaterThan(0);
  });
});

describe('request cost estimation', () => {
  test('bills reasoning tokens at the output rate', () => {
    const estimate = estimateRequestCost('gpt-4o', 1_000_000, 0, 500_000);
    // 1M input @ $2.5 + 0.5M reasoning @ $10 = 2.5 + 5 = 7.5
    expect(estimate.cost).toBeCloseTo(7.5, 6);
  });

  test('free models cost nothing regardless of tokens', () => {
    const estimate = estimateRequestCost('qwen3-free', 1_000_000, 1_000_000, 1_000_000);
    expect(estimate.cost).toBe(0);
    expect(estimate.free).toBe(true);
  });

  test('handles zero and negative token counts safely', () => {
    const estimate = estimateRequestCost('claude-haiku-4-5', 0, -5, 0);
    expect(estimate.cost).toBe(0);
  });
});

describe('usd formatting', () => {
  test('formats zero and tiny amounts', () => {
    expect(formatUsd(0)).toBe('$0.00');
    expect(formatUsd(0.00001)).toBe('<$0.0001');
  });

  test('uses more precision for small amounts', () => {
    expect(formatUsd(0.004)).toBe('$0.0040');
    expect(formatUsd(0.5)).toBe('$0.500');
    expect(formatUsd(12.3)).toBe('$12.30');
  });
});
