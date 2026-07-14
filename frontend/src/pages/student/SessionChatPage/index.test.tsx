import type { ReactNode } from 'react';
import { act, render, waitFor } from '@testing-library/react';
import { Provider } from 'react-redux';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CreateSessionResponse } from '@/modules/session/services/sessionService';
import type { LearningSession } from '@/types';

const mocks = vi.hoisted(() => ({
  createSession: vi.fn(),
  getSessions: vi.fn(),
  sendMessage: vi.fn().mockResolvedValue(undefined),
  closeSSE: vi.fn(),
}));

vi.mock('@/modules/session/services/sessionService', () => ({
  sessionService: {
    createSession: mocks.createSession,
    getSessions: mocks.getSessions,
  },
}));

vi.mock('@/components/layout/MainLayout', () => ({
  MainLayout: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock('./components/ChatHeader', () => ({ ChatHeader: () => null }));
vi.mock('./components/ChatSidebar', () => ({ ChatSidebar: () => null }));
vi.mock('./components/ChatMessages', () => ({ ChatMessages: () => null }));
vi.mock('./components/ChatInput', () => ({ ChatInput: () => null }));
vi.mock('./components/ModeSelector', () => ({ ModeSelector: () => null }));
vi.mock('./components/QuickActions', () => ({ QuickActions: () => null }));
vi.mock('./hooks/useImageUpload', () => ({
  useImageUpload: () => ({
    selectedImages: [],
    previewUrls: [],
    isUploading: false,
    handleImageSelect: vi.fn(),
    handleRemoveImage: vi.fn(),
    clearImages: vi.fn(),
  }),
}));
vi.mock('./hooks/useFileUpload', () => ({
  useFileUpload: () => ({
    files: [],
    isParsing: false,
    handleFileSelect: vi.fn(),
    handleRemoveFile: vi.fn(),
    clearFiles: vi.fn(),
    getParsedDocuments: () => [],
  }),
}));
vi.mock('./hooks/useChatStream', () => ({
  useChatStream: ({
    currentSession,
    sseControllerRef,
  }: {
    currentSession: LearningSession | null;
    sseControllerRef: { current: { close: () => void; getTaskId: () => null } | null };
  }) => {
    sseControllerRef.current = { close: mocks.closeSSE, getTaskId: () => null };
    return {
      handleSendMessage: (message: string) => mocks.sendMessage(currentSession?.id, message),
    };
  },
}));

import { store } from '@/store';
import {
  resetSessionState,
  setCurrentSession,
} from '@/modules/session/store/sessionSlice';
import { SessionChatPage } from './index';

const createSessionResponse: CreateSessionResponse = {
  session_id: 'tutor-session',
  user_id: 'student-1',
  topic: 'AI 自主练习辅导 · 函数极限',
  mode: 'explain',
  status: 'active',
  created_at: '2026-07-12T01:00:00Z',
  welcome_message: {
    id: 'welcome-1',
    role: 'assistant',
    content: '欢迎',
    agent: 'tutor',
    timestamp: '2026-07-12T01:00:00Z',
    attachments: [],
  },
};

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
};

describe('SessionChatPage exercise tutor launch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.dispatch(resetSessionState());
    store.dispatch(setCurrentSession({
      id: 'old-session',
      studentId: 'student-1',
      title: '旧会话',
      status: 'active',
      startedAt: '2026-07-12T00:00:00Z',
      messageCount: 3,
    }));
    mocks.getSessions.mockResolvedValue({ sessions: [], total: 0 });
    mocks.createSession.mockResolvedValue(createSessionResponse);
  });

  it('waits for the fresh explain session before sending the tutor prompt', async () => {
    const initialMessage = '【辅导场景：AI 自主练习】\n请先给我提示。';
    const pendingSession = deferred<CreateSessionResponse>();
    mocks.createSession.mockReturnValueOnce(pendingSession.promise);

    render(
      <Provider store={store}>
        <MemoryRouter initialEntries={[{
          pathname: '/session/new',
          state: {
            initialMessage,
            mode: 'explain',
            source: 'ai_generated',
            topic: 'AI 自主练习辅导 · 函数极限',
          },
        }]}>
          <Routes>
            <Route path="/session/:sessionId" element={<SessionChatPage />} />
          </Routes>
        </MemoryRouter>
      </Provider>,
    );

    await waitFor(() => expect(mocks.createSession).toHaveBeenCalledWith(
      'AI 自主练习辅导 · 函数极限',
      'explain',
    ));
    expect(mocks.sendMessage).not.toHaveBeenCalled();

    await act(async () => {
      pendingSession.resolve(createSessionResponse);
      await pendingSession.promise;
    });

    await waitFor(() => expect(mocks.sendMessage).toHaveBeenCalledWith(
      'tutor-session',
      initialMessage,
    ));
    expect(mocks.sendMessage).not.toHaveBeenCalledWith('old-session', initialMessage);
    expect(store.getState().session.currentSession?.id).toBe('tutor-session');
  });

  it('closes the current SSE controller when the page unmounts', () => {
    const view = render(
      <Provider store={store}>
        <MemoryRouter initialEntries={['/session/old-session']}>
          <Routes>
            <Route path="/session/:sessionId" element={<SessionChatPage />} />
          </Routes>
        </MemoryRouter>
      </Provider>,
    );

    view.unmount();

    expect(mocks.closeSSE).toHaveBeenCalledOnce();
  });
});
