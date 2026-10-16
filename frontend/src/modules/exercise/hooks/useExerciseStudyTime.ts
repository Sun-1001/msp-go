import { useCallback, useEffect, useRef } from 'react';
import { exerciseService, type SubmitPayload } from '../services/exerciseService';

type Binding = Pick<SubmitPayload, 'exerciseId' | 'dailyAssignmentId' | 'reviewTaskId' | 'reviewTaskRevision' | 'originalAttemptId'>;

// Count only the visible, focused exercise. The server independently bounds
// this report by its clock and removes overlap with other submissions/devices.
export function useExerciseStudyTime(binding: Binding, enabled: boolean) {
  const { exerciseId, dailyAssignmentId, reviewTaskId, reviewTaskRevision, originalAttemptId } = binding;
  const clock = useRef({ elapsed: 0, since: null as number | null });

  const pause = useCallback(() => {
    if (clock.current.since !== null) {
      clock.current.elapsed += Math.max(0, performance.now() - clock.current.since);
      clock.current.since = null;
    }
  }, []);

  useEffect(() => {
    clock.current = { elapsed: 0, since: null };
  }, [exerciseId, dailyAssignmentId, reviewTaskId, reviewTaskRevision, originalAttemptId]);

  useEffect(() => {
    if (!exerciseId || !enabled) return;
    const controller = new AbortController();
    let ready = false;
    const sync = () => {
      pause();
      if (ready && document.visibilityState === 'visible' && document.hasFocus()) {
        clock.current.since = performance.now();
      }
    };
    void exerciseService.startExercise({ exerciseId, dailyAssignmentId, reviewTaskId, reviewTaskRevision, originalAttemptId }, controller.signal)
      .then(() => {
        if (!controller.signal.aborted) { ready = true; sync(); }
      })
      .catch(() => { /* Grading remains available; an unverified clock contributes no time. */ });
    document.addEventListener('visibilitychange', sync);
    window.addEventListener('focus', sync);
    window.addEventListener('blur', pause);
    return () => {
      pause();
      controller.abort();
      document.removeEventListener('visibilitychange', sync);
      window.removeEventListener('focus', sync);
      window.removeEventListener('blur', pause);
    };
  }, [exerciseId, dailyAssignmentId, reviewTaskId, reviewTaskRevision, originalAttemptId, enabled, pause]);

  return useCallback(() => {
    const running = clock.current.since === null ? 0 : Math.max(0, performance.now() - clock.current.since);
    return Math.min(7200, Math.floor((clock.current.elapsed + running) / 1000));
  }, []);
}
