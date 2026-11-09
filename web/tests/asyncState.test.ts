import { describe, expect, test } from 'bun:test';
import {
  asyncFlags,
  asyncReducer,
  initialAsyncState,
  normalizeAsyncError,
  type AsyncState,
  type AsyncEvent,
} from '../src/utils/asyncState';

const run = <T>(events: AsyncEvent<T>[], from: AsyncState<T> = initialAsyncState<T>()) =>
  events.reduce<AsyncState<T>>(asyncReducer, from);

describe('async state machine', () => {
  test('walks the documented lifecycle idle -> loading -> success', () => {
    expect(initialAsyncState<string>().phase).toBe('idle');

    const loading = run<string>([{ type: 'start' }]);
    expect(loading.phase).toBe('loading');
    expect(loading.settled).toBe(false);

    const done = run<string>([{ type: 'start' }, { type: 'success', data: 'ok' }]);
    expect(done.phase).toBe('success');
    expect(done.data).toBe('ok');
    expect(done.settled).toBe(true);
  });

  test('walks idle -> loading -> error and clears any stale data error', () => {
    const failed = run<string>([
      { type: 'start' },
      { type: 'failure', error: 'boom' },
    ]);
    expect(failed.phase).toBe('error');
    expect(failed.error).toBe('boom');
    expect(failed.settled).toBe(true);
  });

  test('illegal state loading+error is unreachable', () => {
    const failed = run<string>([{ type: 'start' }, { type: 'failure', error: 'x' }]);
    const flags = asyncFlags(failed);
    expect(flags.isLoading && flags.isError).toBe(false);
  });

  test('starting a new attempt clears the previous error', () => {
    const retried = run<string>([
      { type: 'start' },
      { type: 'failure', error: 'boom' },
      { type: 'start' },
    ]);
    expect(retried.phase).toBe('loading');
    expect(retried.error).toBeNull();
  });

  test('ignores a stale success that arrives after the attempt was reset', () => {
    const state = run<string>([
      { type: 'start' },
      { type: 'reset' },
      { type: 'success', data: 'late' },
    ]);
    expect(state.phase).toBe('idle');
    expect(state.data).toBeNull();
  });

  test('ignores a duplicate success once settled', () => {
    const settled = run<string>([
      { type: 'start' },
      { type: 'success', data: 'first' },
      { type: 'success', data: 'second' },
    ]);
    expect(settled.data).toBe('first');
  });

  test('ignores a failure after success', () => {
    const settled = run<string>([
      { type: 'start' },
      { type: 'success', data: 'ok' },
      { type: 'failure', error: 'late' },
    ]);
    expect(settled.phase).toBe('success');
    expect(settled.error).toBeNull();
  });

  test('attempt counter advances so callers can detect supersession', () => {
    const one = run<string>([{ type: 'start' }]);
    const two = run<string>([{ type: 'start' }, { type: 'start' }]);
    expect(two.attempt).toBeGreaterThan(one.attempt);
  });

  test('reset returns to the initial state', () => {
    const reset = run<string>([
      { type: 'start' },
      { type: 'success', data: 'x' },
      { type: 'reset' },
    ]);
    expect(reset).toEqual(initialAsyncState<string>());
  });

  test('derived flags are always mutually exclusive', () => {
    const states: AsyncState<string>[] = [
      initialAsyncState<string>(),
      run<string>([{ type: 'start' }]),
      run<string>([{ type: 'start' }, { type: 'success', data: 'x' }]),
      run<string>([{ type: 'start' }, { type: 'failure', error: 'e' }]),
    ];
    for (const state of states) {
      const flags = asyncFlags(state);
      const active = [flags.isLoading, flags.isSuccess, flags.isError, flags.isIdle].filter(
        Boolean
      ).length;
      expect(active).toBe(1);
    }
  });

  describe('normalizeAsyncError', () => {
    test('extracts Error messages', () => {
      expect(normalizeAsyncError(new Error('nope'))).toBe('nope');
    });
    test('extracts strings', () => {
      expect(normalizeAsyncError('plain')).toBe('plain');
    });
    test('extracts nested error fields', () => {
      expect(normalizeAsyncError({ error: 'nested' })).toBe('nested');
    });
    test('falls back for unknown shapes', () => {
      expect(normalizeAsyncError(null, 'fallback')).toBe('fallback');
      expect(normalizeAsyncError({}, 'fallback')).toBe('fallback');
    });
  });
});
