import { useEffect, useRef } from 'react';

export type SerialPollingTask = (signal: AbortSignal) => Promise<void>;

/** Runs one task immediately, then schedules the next run after it settles. */
export function useSerialPolling(task: SerialPollingTask, intervalMs: number): void {
  const taskRef = useRef(task);
  const inFlightRef = useRef<{ promise: Promise<void>; controller: AbortController } | null>(null);

  useEffect(() => {
    taskRef.current = task;
    inFlightRef.current?.controller.abort();
  }, [task]);

  useEffect(() => {
    if (!Number.isFinite(intervalMs) || intervalMs <= 0) return;

    let active = true;
    let timerId: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      if (!active) return;
      const currentController = new AbortController();
      const run = (async () => {
        try {
          await taskRef.current(currentController.signal);
        } catch {
          // Polling tasks own their user-facing error state.
        }
      })();
      const inFlight = { promise: run, controller: currentController };
      inFlightRef.current = inFlight;
      await run;
      if (inFlightRef.current === inFlight) {
        inFlightRef.current = null;
      }
      if (active) {
        timerId = setTimeout(poll, intervalMs);
      }
    };

    queueMicrotask(() => {
      const previous = inFlightRef.current;
      if (previous) {
        void previous.promise.finally(() => {
          if (active) void poll();
        });
      } else {
        void poll();
      }
    });

    return () => {
      active = false;
      inFlightRef.current?.controller.abort();
      if (timerId !== undefined) {
        clearTimeout(timerId);
      }
    };
  }, [intervalMs]);
}
