import { useEffect, useRef } from 'react';

export type SerialPollingTask = (signal: AbortSignal) => Promise<void>;

/** Runs one task immediately, then schedules the next run after it settles. */
export function useSerialPolling(task: SerialPollingTask, intervalMs: number): void {
  const taskRef = useRef(task);
  const inFlightRef = useRef<Promise<void> | null>(null);

  useEffect(() => {
    taskRef.current = task;
  }, [task]);

  useEffect(() => {
    if (!Number.isFinite(intervalMs) || intervalMs <= 0) return;

    let active = true;
    let timerId: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | null = null;

    const poll = async () => {
      if (!active) return;
      const currentController = new AbortController();
      controller = currentController;
      const run = (async () => {
        try {
          await taskRef.current(currentController.signal);
        } catch {
          // Polling tasks own their user-facing error state.
        }
      })();
      inFlightRef.current = run;
      await run;
      if (inFlightRef.current === run) {
        inFlightRef.current = null;
      }
      if (controller === currentController) {
        controller = null;
      }
      if (active) {
        timerId = setTimeout(poll, intervalMs);
      }
    };

    queueMicrotask(() => {
      const previous = inFlightRef.current;
      if (previous) {
        void previous.finally(() => {
          if (active) void poll();
        });
      } else {
        void poll();
      }
    });

    return () => {
      active = false;
      const currentController = controller;
      controller = null;
      currentController?.abort();
      if (timerId !== undefined) {
        clearTimeout(timerId);
      }
    };
  }, [intervalMs]);
}
