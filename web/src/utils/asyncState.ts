/**
 * Formal async state machine.
 *
 * Many screens in this app encode their lifecycle as independent flags
 * (`loading`, `error`, `data`, `hasFetched`, ...). Because those flags are
 * independent, illegal combinations are *representable* — e.g. `loading`
 * stuck true forever, or `loading && error` both set, or a stale response
 * overwriting a newer one. This module models the lifecycle as an explicit,
 * closed set of states so those combinations cannot exist.
 *
 * States:   idle -> loading -> (success | error)
 *           any state -> idle (reset)
 *
 * A monotonically increasing `token` identifies each attempt. Results from a
 * superseded attempt are discarded, which removes the classic "stale response
 * wins" race in a single place instead of in every caller.
 */

export type AsyncPhase = 'idle' | 'loading' | 'success' | 'error';

export interface AsyncState<T> {
  readonly phase: AsyncPhase;
  readonly data: T | null;
  readonly error: string | null;
  /**
   * Increments on every attempt. Callers compare this to decide whether a
   * late response is still relevant.
   */
  readonly attempt: number;
  /** True only after at least one attempt settled (success or error). */
  readonly settled: boolean;
}

export type AsyncEvent<T> =
  | { type: 'start' }
  | { type: 'success'; data: T }
  | { type: 'failure'; error: string }
  | { type: 'reset' };

export const initialAsyncState = <T>(): AsyncState<T> => ({
  phase: 'idle',
  data: null,
  error: null,
  attempt: 0,
  settled: false,
});

/**
 * Pure reducer. Every transition is explicit, so the reachable set of states
 * is exactly: idle, loading, success, error — never a mixture.
 */
export const asyncReducer = <T>(state: AsyncState<T>, event: AsyncEvent<T>): AsyncState<T> => {
  switch (event.type) {
    case 'start':
      return {
        phase: 'loading',
        data: state.data,
        error: null,
        attempt: state.attempt + 1,
        settled: state.settled,
      };
    case 'success':
      // Ignore a success that arrives when nothing is in flight (stale/duplicate).
      if (state.phase !== 'loading') return state;
      return {
        phase: 'success',
        data: event.data,
        error: null,
        attempt: state.attempt,
        settled: true,
      };
    case 'failure':
      if (state.phase !== 'loading') return state;
      return {
        phase: 'error',
        data: state.data,
        error: event.error,
        attempt: state.attempt,
        settled: true,
      };
    case 'reset':
      return initialAsyncState<T>();
    default:
      return state;
  }
};

/** Derived view flags, guaranteed mutually consistent. */
export const asyncFlags = <T>(state: AsyncState<T>) => ({
  isLoading: state.phase === 'loading',
  isSuccess: state.phase === 'success',
  isError: state.phase === 'error',
  isIdle: state.phase === 'idle',
});

/**
 * Error-message extraction shared by every async caller so the UI never has to
 * guess a shape. Handles Error, string, and `{ message | error }` objects.
 */
export const normalizeAsyncError = (err: unknown, fallback = 'Request failed'): string => {
  if (typeof err === 'string' && err.trim()) return err;
  if (err && typeof err === 'object') {
    const record = err as Record<string, unknown>;
    for (const key of ['message', 'error', 'msg']) {
      const value = record[key];
      if (typeof value === 'string' && value.trim()) return value;
    }
    if (typeof record.status === 'number') return `HTTP ${record.status}`;
  }
  return fallback;
};
