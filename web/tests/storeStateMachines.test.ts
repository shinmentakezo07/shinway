import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { modelsApi } from '../src/services/api/models';
import { useModelsStore } from '../src/stores/useModelsStore';

/**
 * Regression tests for implicit state machines that previously held illegal
 * combinations (see the implicit-state-machine audit).
 *
 * We save and restore `modelsApi.fetchModels` directly rather than using
 * `mock.module`, because a module-level mock leaks into every test file in
 * Bun's shared runtime. This follows the convention used by the provider
 * tests (save originals in `afterEach`).
 */
const originalFetchModels = modelsApi.fetchModels;

type Pending = {
  resolve: (value: unknown[]) => void;
  reject: (error: Error) => void;
};

let pending: Pending[] = [];

const stubFetch = () => {
  modelsApi.fetchModels = (() =>
    new Promise((resolve, reject) => {
      pending.push({ resolve, reject });
    })) as typeof modelsApi.fetchModels;
};

afterEach(() => {
  modelsApi.fetchModels = originalFetchModels;
});

describe('models store lifecycle', () => {
  beforeEach(() => {
    pending = [];
    useModelsStore.getState().clearCache();
    stubFetch();
  });

  test('clearCache resets loading and error, not just data', () => {
    useModelsStore.setState({ loading: true, error: 'boom', models: [{ name: 'm' }] as never });
    useModelsStore.getState().clearCache();

    const state = useModelsStore.getState();
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
    expect(state.models).toEqual([]);
    expect(state.cache).toBeNull();
  });

  test('a response superseded by clearCache does not write stale data', async () => {
    const request = useModelsStore.getState().fetchModels('http://a');
    expect(useModelsStore.getState().loading).toBe(true);

    // Simulate logout/reconnect while the request is in flight.
    useModelsStore.getState().clearCache();

    pending[0].resolve([{ name: 'stale' }]);
    await request;

    const state = useModelsStore.getState();
    expect(state.models).toEqual([]);
    expect(state.cache).toBeNull();
    expect(state.loading).toBe(false);
  });

  test('a stale error does not resurrect an error after the cache was cleared', async () => {
    const request = useModelsStore.getState().fetchModels('http://a');
    useModelsStore.getState().clearCache();
    pending[0].reject(new Error('stale failure'));
    await expect(request).rejects.toThrow('stale failure');

    expect(useModelsStore.getState().error).toBeNull();
    expect(useModelsStore.getState().loading).toBe(false);
  });

  test('a current response still commits normally', async () => {
    const request = useModelsStore.getState().fetchModels('http://a');
    pending[0].resolve([{ name: 'fresh' }]);
    await request;

    const state = useModelsStore.getState();
    expect(state.models).toEqual([{ name: 'fresh' }] as never);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });
});
