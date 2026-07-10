import { createSlice, createAsyncThunk, type PayloadAction } from '@reduxjs/toolkit';
import type { RootState } from '@/store';
import type { LearningSession, SessionMessage, LoadingState } from '@/types';
import { createFieldSelector } from '@/store/utils/sliceFactory';
import { sessionService, type SessionMode } from '@/modules/session/services/sessionService';

/**
 * 聊天模式
 */
export type ChatMode = 'study' | 'chat' | 'practice' | 'explain';

/**
 * 流式状态
 */
export type StreamStatus = 'idle' | 'streaming' | 'cancelled' | 'error';

/**
 * 会话状态
 */
export interface SessionState {
  currentSession: LearningSession | null;
  messages: SessionMessage[];
  mode: ChatMode;
  loadingState: LoadingState;
  sendingState: LoadingState;
  error: string | null;
  sessions: LearningSession[];
  sessionsLoadingState: LoadingState;
  // 流式响应相关
  currentTaskId: string | null;
  streamStatus: StreamStatus;
  streamingMessageId: string | null;
}

const initialState: SessionState = {
  currentSession: null,
  messages: [],
  mode: 'chat',
  loadingState: 'idle',
  sendingState: 'idle',
  error: null,
  sessions: [],
  sessionsLoadingState: 'idle',
  // 流式响应相关
  currentTaskId: null,
  streamStatus: 'idle',
  streamingMessageId: null,
};

// ============ Async Thunks ============

/**
 * 创建会话
 */
export const createSessionAsync = createAsyncThunk(
  'session/createSession',
  async (
    { topic, mode }: { topic?: string; mode?: SessionMode },
    { rejectWithValue }
  ) => {
    try {
      const response = await sessionService.createSession(topic, mode);

      // 转换为前端格式
      const session: LearningSession = {
        id: response.session_id,
        studentId: response.user_id,
        title: response.topic || '新会话',
        status: response.status as 'active' | 'completed' | 'paused',
        startedAt: response.created_at,
        messageCount: 1,
      };

      const welcomeMessage: SessionMessage = {
        id: response.welcome_message.id,
        sessionId: response.session_id,
        role: response.welcome_message.role as 'user' | 'assistant' | 'system',
        content: response.welcome_message.content,
        timestamp: response.welcome_message.timestamp,
        metadata: {
          agent: response.welcome_message.agent,
        },
      };

      return { session, welcomeMessage, mode: response.mode as ChatMode };
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '创建会话失败'
      );
    }
  }
);

/**
 * 获取会话历史
 */
export const fetchHistoryAsync = createAsyncThunk(
  'session/fetchHistory',
  async (
    { sessionId, limit, offset }: { sessionId: string; limit?: number; offset?: number },
    { rejectWithValue }
  ) => {
    try {
      const response = await sessionService.getHistory(sessionId, limit, offset);

      // 转换为前端格式
      const messages: SessionMessage[] = response.messages.map((msg) => ({
        id: msg.id,
        sessionId,
        role: msg.role as 'user' | 'assistant' | 'system',
        content: msg.content,
        timestamp: msg.timestamp,
        metadata: {
          agent: msg.agent,
          attachments: msg.attachments,
        },
      }));

      return { messages, total: response.total, hasMore: response.has_more };
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '获取历史失败'
      );
    }
  }
);

/**
 * 获取会话列表
 */
export const fetchSessionsAsync = createAsyncThunk(
  'session/fetchSessions',
  async (
    { limit, offset }: { limit?: number; offset?: number } = {},
    { rejectWithValue }
  ) => {
    try {
      const response = await sessionService.getSessions(limit, offset);

      // 转换为前端格式
      const sessions: LearningSession[] = response.sessions.map((s) => ({
        id: s.session_id,
        studentId: s.user_id,
        title: s.topic || '会话',
        status: s.status,
        startedAt: s.started_at,
        endedAt: s.ended_at || undefined,
        messageCount: s.message_count,
      }));

      return { sessions, total: response.total };
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '获取会话列表失败'
      );
    }
  },
  {
    condition: (_, { getState }) => {
      const { sessionsLoadingState } = (getState() as RootState).session;
      return sessionsLoadingState !== 'loading';
    },
  }
);

/**
 * 结束会话
 */
export const endSessionAsync = createAsyncThunk(
  'session/endSession',
  async (sessionId: string, { rejectWithValue }) => {
    try {
      await sessionService.endSession(sessionId);
      return sessionId;
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '结束会话失败'
      );
    }
  }
);

/**
 * 更新会话模式
 */
export const updateSessionModeAsync = createAsyncThunk(
  'session/updateSessionMode',
  async (
    { sessionId, mode }: { sessionId: string; mode: ChatMode },
    { rejectWithValue }
  ) => {
    try {
      const response = await sessionService.updateSessionMode(sessionId, mode);
      return { sessionId: response.session_id, mode: response.mode as ChatMode };
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '更新模式失败'
      );
    }
  }
);

/**
 * 删除会话
 */
export const deleteSessionAsync = createAsyncThunk(
  'session/deleteSession',
  async (sessionId: string, { rejectWithValue }) => {
    try {
      const response = await sessionService.deleteSession(sessionId);
      if (!response.success) {
        return rejectWithValue(response.message);
      }
      return sessionId;
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '删除会话失败'
      );
    }
  }
);

/**
 * 批量删除会话
 */
export const batchDeleteSessionsAsync = createAsyncThunk(
  'session/batchDeleteSessions',
  async (sessionIds: string[], { rejectWithValue }) => {
    try {
      const response = await sessionService.batchDeleteSessions(sessionIds);
      if (!response.success) {
        return rejectWithValue(response.message);
      }
      return sessionIds;
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '批量删除会话失败'
      );
    }
  }
);

/**
 * 取消当前任务
 */
export const cancelCurrentTaskAsync = createAsyncThunk(
  'session/cancelCurrentTask',
  async (_, { getState, rejectWithValue }) => {
    const state = getState() as { session: SessionState };
    const taskId = state.session.currentTaskId;

    if (!taskId) {
      return rejectWithValue('没有正在进行的任务');
    }

    try {
      const success = await sessionService.cancelTask(taskId);
      if (!success) {
        return rejectWithValue('取消任务失败');
      }
      return taskId;
    } catch (error) {
      return rejectWithValue(
        error instanceof Error ? error.message : '取消任务失败'
      );
    }
  }
);

// ============ Slice ============

const sessionSlice = createSlice({
  name: 'session',
  initialState,
  reducers: {
    // 设置当前会话
    setCurrentSession(state, action: PayloadAction<LearningSession>) {
      state.currentSession = action.payload;
      state.error = null;
    },

    // 设置消息列表
    setMessages(state, action: PayloadAction<SessionMessage[]>) {
      state.messages = action.payload;
    },

    // 添加消息
    addMessage(state, action: PayloadAction<SessionMessage>) {
      state.messages.push(action.payload);

      // 更新会话消息计数
      if (state.currentSession) {
        state.currentSession.messageCount = state.messages.length;
      }
    },

    // 更新最后一条消息（用于流式响应）
    updateLastMessage(state, action: PayloadAction<Partial<SessionMessage>>) {
      if (state.messages.length > 0) {
        const lastMessage = state.messages[state.messages.length - 1];
        state.messages[state.messages.length - 1] = {
          ...lastMessage,
          ...action.payload,
        };
      }
    },

    // 追加内容到流式消息（用于流式响应）
    // 优先按 streamingMessageId 精确定位目标消息，fallback 到最后一条
    // 利用 Immer Proxy 机制，O(1) 修改，未变更的消息保持原引用
    appendToLastMessage(state, action: PayloadAction<string>) {
      const targetId = state.streamingMessageId;
      const target = targetId
        ? state.messages.find((m) => m.id === targetId)
        : state.messages[state.messages.length - 1];
      if (target) {
        target.content += action.payload;
      }
    },

    // 设置聊天模式
    setMode(state, action: PayloadAction<ChatMode>) {
      state.mode = action.payload;
    },

    // 清除当前会话
    clearCurrentSession(state) {
      state.currentSession = null;
      state.messages = [];
      state.error = null;
      state.currentTaskId = null;
      state.streamStatus = 'idle';
      state.streamingMessageId = null;
    },

    // 设置加载状态
    setLoadingState(state, action: PayloadAction<LoadingState>) {
      state.loadingState = action.payload;
      if (action.payload === 'loading') {
        state.error = null;
      }
    },

    // 设置发送状态
    setSendingState(state, action: PayloadAction<LoadingState>) {
      state.sendingState = action.payload;
      if (action.payload === 'loading') {
        state.error = null;
      }
    },

    // 设置错误信息
    setError(state, action: PayloadAction<string>) {
      state.error = action.payload;
      state.loadingState = 'error';
    },

    // 设置会话列表
    setSessions(state, action: PayloadAction<LearningSession[]>) {
      state.sessions = action.payload;
    },

    // 添加会话到列表
    addSession(state, action: PayloadAction<LearningSession>) {
      state.sessions.unshift(action.payload);
    },

    // 更新会话状态
    updateSessionStatus(
      state,
      action: PayloadAction<{ sessionId: string; status: LearningSession['status'] }>
    ) {
      const { sessionId, status } = action.payload;

      // 更新当前会话
      if (state.currentSession?.id === sessionId) {
        state.currentSession.status = status;
        if (status === 'completed') {
          state.currentSession.endedAt = new Date().toISOString();
        }
      }

      // 更新会话列表
      const session = state.sessions.find(s => s.id === sessionId);
      if (session) {
        session.status = status;
        if (status === 'completed') {
          session.endedAt = new Date().toISOString();
        }
      }
    },

    // 设置会话列表加载状态
    setSessionsLoadingState(state, action: PayloadAction<LoadingState>) {
      state.sessionsLoadingState = action.payload;
    },

    // 设置当前任务 ID
    setCurrentTaskId(state, action: PayloadAction<string | null>) {
      state.currentTaskId = action.payload;
    },

    // 设置流式状态
    setStreamStatus(state, action: PayloadAction<StreamStatus>) {
      state.streamStatus = action.payload;
    },

    // 设置正在流式接收的消息 ID
    setStreamingMessageId(state, action: PayloadAction<string | null>) {
      state.streamingMessageId = action.payload;
    },

    // 重置状态
    resetSessionState() {
      return initialState;
    },
  },
  extraReducers: (builder) => {
    // 创建会话
    builder
      .addCase(createSessionAsync.pending, (state) => {
        state.loadingState = 'loading';
        state.error = null;
      })
      .addCase(createSessionAsync.fulfilled, (state, action) => {
        state.loadingState = 'success';
        state.currentSession = action.payload.session;
        state.messages = [action.payload.welcomeMessage];
        state.mode = action.payload.mode;
      })
      .addCase(createSessionAsync.rejected, (state, action) => {
        state.loadingState = 'error';
        state.error = action.payload as string;
      });

    // 获取历史
    builder
      .addCase(fetchHistoryAsync.pending, (state) => {
        state.loadingState = 'loading';
      })
      .addCase(fetchHistoryAsync.fulfilled, (state, action) => {
        state.loadingState = 'success';
        state.messages = action.payload.messages;
      })
      .addCase(fetchHistoryAsync.rejected, (state, action) => {
        state.loadingState = 'error';
        state.error = action.payload as string;
      });

    // 获取会话列表
    builder
      .addCase(fetchSessionsAsync.pending, (state) => {
        state.sessionsLoadingState = 'loading';
      })
      .addCase(fetchSessionsAsync.fulfilled, (state, action) => {
        state.sessionsLoadingState = 'success';
        state.sessions = action.payload.sessions;
      })
      .addCase(fetchSessionsAsync.rejected, (state, action) => {
        state.sessionsLoadingState = 'error';
        state.error = action.payload as string;
      });

    // 结束会话
    builder
      .addCase(endSessionAsync.fulfilled, (state, action) => {
        const sessionId = action.payload;
        if (state.currentSession?.id === sessionId) {
          state.currentSession.status = 'completed';
          state.currentSession.endedAt = new Date().toISOString();
        }
        const session = state.sessions.find(s => s.id === sessionId);
        if (session) {
          session.status = 'completed';
          session.endedAt = new Date().toISOString();
        }
      });

    // 取消任务
    builder
      .addCase(cancelCurrentTaskAsync.fulfilled, (state) => {
        state.streamStatus = 'cancelled';
        state.currentTaskId = null;
      })
      .addCase(cancelCurrentTaskAsync.rejected, (state) => {
        // 即使取消失败，也重置状态
        state.streamStatus = 'idle';
      });

    // 更新会话模式
    builder.addCase(updateSessionModeAsync.fulfilled, (state, action) => {
      state.mode = action.payload.mode;
      // 更新当前会话的标题
      if (state.currentSession?.id === action.payload.sessionId) {
        const modeNames: Record<ChatMode, string> = {
          study: '学习模式',
          chat: '聊天模式',
          practice: '练习模式',
          explain: '讲解模式',
        };
        state.currentSession.title = modeNames[action.payload.mode];
      }
    });

    // 删除会话
    builder.addCase(deleteSessionAsync.fulfilled, (state, action) => {
      const sessionId = action.payload;
      // 从列表中移除
      state.sessions = state.sessions.filter((s) => s.id !== sessionId);
      // 如果删除的是当前会话，清空当前会话
      if (state.currentSession?.id === sessionId) {
        state.currentSession = null;
        state.messages = [];
      }
    });

    // 批量删除会话
    builder.addCase(batchDeleteSessionsAsync.fulfilled, (state, action) => {
      const sessionIds = new Set(action.payload);
      // 从列表中移除
      state.sessions = state.sessions.filter((s) => !sessionIds.has(s.id));
      // 如果删除的包含当前会话，清空当前会话
      if (state.currentSession && sessionIds.has(state.currentSession.id)) {
        state.currentSession = null;
        state.messages = [];
      }
    });
  },
});

export const {
  setCurrentSession,
  setMessages,
  addMessage,
  updateLastMessage,
  appendToLastMessage,
  setMode,
  clearCurrentSession,
  setLoadingState,
  setSendingState,
  setError,
  setSessions,
  addSession,
  updateSessionStatus,
  setSessionsLoadingState,
  setCurrentTaskId,
  setStreamStatus,
  setStreamingMessageId,
  resetSessionState,
} = sessionSlice.actions;

// ============ Selectors ============
// 使用工厂函数生成字段 selectors
export const selectCurrentSession = createFieldSelector<SessionState, 'session', 'currentSession'>('session', 'currentSession');
export const selectMessages = createFieldSelector<SessionState, 'session', 'messages'>('session', 'messages');
export const selectMode = createFieldSelector<SessionState, 'session', 'mode'>('session', 'mode');
export const selectSessionLoadingState = createFieldSelector<SessionState, 'session', 'loadingState'>('session', 'loadingState');
export const selectSessionSendingState = createFieldSelector<SessionState, 'session', 'sendingState'>('session', 'sendingState');
export const selectSessionError = createFieldSelector<SessionState, 'session', 'error'>('session', 'error');
export const selectSessions = createFieldSelector<SessionState, 'session', 'sessions'>('session', 'sessions');
export const selectSessionsLoadingState = createFieldSelector<SessionState, 'session', 'sessionsLoadingState'>('session', 'sessionsLoadingState');
export const selectCurrentTaskId = createFieldSelector<SessionState, 'session', 'currentTaskId'>('session', 'currentTaskId');
export const selectStreamStatus = createFieldSelector<SessionState, 'session', 'streamStatus'>('session', 'streamStatus');
export const selectStreamingMessageId = createFieldSelector<SessionState, 'session', 'streamingMessageId'>('session', 'streamingMessageId');

// 派生 selectors
export const selectIsSessionLoading = (state: { session: SessionState }) =>
  state.session.loadingState === 'loading';

export const selectIsMessageSending = (state: { session: SessionState }) =>
  state.session.sendingState === 'loading';

export const selectIsStreaming = (state: { session: SessionState }) =>
  state.session.streamStatus === 'streaming';

export default sessionSlice.reducer;
