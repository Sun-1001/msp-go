import { StrictMode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useSerialPolling, type SerialPollingTask } from '../useSerialPolling';

describe('useSerialPolling', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('never overlaps slow polling tasks', async () => {
    vi.useFakeTimers();
    const resolvers: Array<() => void> = [];
    const task = vi.fn<SerialPollingTask>(() => new Promise<void>((resolve) => {
      resolvers.push(resolve);
    }));

    renderHook(() => useSerialPolling(task, 1_000));
    await act(async () => Promise.resolve());
    expect(task).toHaveBeenCalledOnce();

    act(() => vi.advanceTimersByTime(10_000));
    expect(task).toHaveBeenCalledOnce();

    await act(async () => {
      resolvers[0]();
      await Promise.resolve();
    });
    act(() => vi.advanceTimersByTime(999));
    expect(task).toHaveBeenCalledOnce();
    act(() => vi.advanceTimersByTime(1));
    expect(task).toHaveBeenCalledTimes(2);
  });

  it('aborts the active task and clears timers on unmount', async () => {
    vi.useFakeTimers();
    let signal: AbortSignal | undefined;
    const task = vi.fn<SerialPollingTask>((currentSignal) => {
      signal = currentSignal;
      return new Promise<void>(() => undefined);
    });

    const { unmount } = renderHook(() => useSerialPolling(task, 1_000));
    await act(async () => Promise.resolve());
    expect(task).toHaveBeenCalledOnce();
    unmount();

    expect(signal?.aborted).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('starts only one initial task when StrictMode replays effects', async () => {
    const task = vi.fn<SerialPollingTask>(() => Promise.resolve());

    renderHook(() => useSerialPolling(task, 1_000), { wrapper: StrictMode });
    await act(async () => Promise.resolve());

    expect(task).toHaveBeenCalledOnce();
  });

  it('waits for the previous task when the interval changes', async () => {
    let resolveTask!: () => void;
    const task = vi.fn<SerialPollingTask>(() => new Promise<void>((resolve) => {
      resolveTask = resolve;
    }));
    const view = renderHook(
      ({ interval }) => useSerialPolling(task, interval),
      { initialProps: { interval: 1_000 } },
    );
    await act(async () => Promise.resolve());
    expect(task).toHaveBeenCalledOnce();

    view.rerender({ interval: 500 });
    await act(async () => Promise.resolve());
    expect(task).toHaveBeenCalledOnce();

    await act(async () => {
      resolveTask();
      await Promise.resolve();
    });
    expect(task).toHaveBeenCalledTimes(2);
  });
});
