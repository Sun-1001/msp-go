import { describe, expect, it } from 'vitest';
import type { LearningSession, SessionMessage } from '@/types';
import sessionReducer, {
  batchDeleteSessionsAsync,
  type SessionState,
} from '@/modules/session/store/sessionSlice';

function makeSession(id: string): LearningSession {
  return {
    id,
    studentId: 'student-1',
    title: id,
    status: 'active',
    startedAt: '2026-07-10T00:00:00Z',
    messageCount: 1,
  };
}

const message: SessionMessage = {
  id: 'message-1',
  sessionId: 'session-2',
  role: 'assistant',
  content: 'content',
  timestamp: '2026-07-10T00:00:00Z',
};

function makeState(): SessionState {
  const sessions = [
    makeSession('session-1'),
    makeSession('session-2'),
    makeSession('session-3'),
  ];
  return {
    currentSession: sessions[1],
    messages: [message],
    mode: 'chat',
    loadingState: 'success',
    sendingState: 'idle',
    error: null,
    sessions,
    sessionsLoadingState: 'success',
    currentTaskId: null,
    streamStatus: 'idle',
    streamingMessageId: null,
  };
}

describe('session batch deletion', () => {
  it('removes selected sessions and clears a selected current session', () => {
    const action = batchDeleteSessionsAsync.fulfilled(
      ['session-1', 'session-2'],
      'request-1',
      ['session-1', 'session-2'],
    );

    const state = sessionReducer(makeState(), action);

    expect(state.sessions.map((session) => session.id)).toEqual(['session-3']);
    expect(state.currentSession).toBeNull();
    expect(state.messages).toEqual([]);
  });

  it('keeps current session state when it is not selected', () => {
    const action = batchDeleteSessionsAsync.fulfilled(
      ['session-1'],
      'request-2',
      ['session-1'],
    );

    const state = sessionReducer(makeState(), action);

    expect(state.sessions.map((session) => session.id)).toEqual([
      'session-2',
      'session-3',
    ]);
    expect(state.currentSession?.id).toBe('session-2');
    expect(state.messages).toEqual([message]);
  });
});
