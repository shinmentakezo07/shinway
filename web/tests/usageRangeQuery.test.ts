import { describe, expect, test } from 'bun:test';
import { usageRangeQuery } from '../src/services/api/usage';

describe('usage range query mapping', () => {
  test('omits the range param for all-time so the backend does not reject it', () => {
    expect(usageRangeQuery('all')).toEqual({});
  });

  test('omits the range param when no range is given', () => {
    expect(usageRangeQuery(undefined)).toEqual({});
  });

  test('passes concrete shorthands through unchanged', () => {
    expect(usageRangeQuery('1h')).toEqual({ range: '1h' });
    expect(usageRangeQuery('24h')).toEqual({ range: '24h' });
    expect(usageRangeQuery('7d')).toEqual({ range: '7d' });
    expect(usageRangeQuery('30d')).toEqual({ range: '30d' });
  });
});
