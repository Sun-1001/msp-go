import React, { useState, useRef, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import { MainLayout } from '../../../components/layout/MainLayout';
import { useAppDispatch, useAppSelector } from '../../../store';
import {
  createSessionAsync,
  fetchHistoryAsync,
  fetchSessionsAsync,
  deleteSessionAsync,
  batchDeleteSessionsAsync,
  updateSessionModeAsync,
  cancelCurrentTaskAsync,
  setCurrentSession,
  setMode,
  selectCurrentSession,
  selectMessages,
  selectMode,
  selectStreamStatus,
  selectStreamingMessageId,
  selectSessionLoadingState,
  selectSessionError,
  selectSessions,
  selectSessionsLoadingState,
  type ChatMode,
} from '@/modules/session/store/sessionSlice';
import type { SSEController } from '../../../libs/http/sseClient';
import { ChatHeader } from './components/ChatHeader';
import { ChatSidebar } from './components/ChatSidebar';
import { ChatMessages } from './components/ChatMessages';
import { ChatInput } from './components/ChatInput';
import { ModeSelector } from './components/ModeSelector';
import { QuickActions } from './components/QuickActions';
import { useChatStream } from './hooks/useChatStream';
import { useImageUpload } from './hooks/useImageUpload';
import { useFileUpload } from './hooks/useFileUpload';
import { CHAT_MODES, QUICK_ACTIONS } from './constants.tsx';
import type { ExerciseTutorLaunchState } from '../exerciseTutorLaunch';

export const SessionChatPage: React.FC = () => {
  const { sessionId } = useParams<{ sessionId?: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const dispatch = useAppDispatch();

  // 从刷题页面跳转时携带的初始消息
  const locationState = location.state as Partial<ExerciseTutorLaunchState> | null;
  const initialMessageHandled = useRef(false);

  // Redux state
  const currentSession = useAppSelector(selectCurrentSession);
  const messages = useAppSelector(selectMessages);
  const currentMode = useAppSelector(selectMode);
  const streamStatus = useAppSelector(selectStreamStatus);
  const streamingMessageId = useAppSelector(selectStreamingMessageId);
  const loadingState = useAppSelector(selectSessionLoadingState);
  const error = useAppSelector(selectSessionError);
  const sessions = useAppSelector(selectSessions);
  const sessionsLoadingState = useAppSelector(selectSessionsLoadingState);

  // Local state
  const [inputValue, setInputValue] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [deletingSessionId, setDeletingSessionId] = useState<string | null>(null);
  const [isSelectMode, setIsSelectMode] = useState(false);
  const [selectedSessionIds, setSelectedSessionIds] = useState<string[]>([]);
  const [isBatchDeleting, setIsBatchDeleting] = useState(false);

  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const sseControllerRef = useRef<SSEController | null>(null);
  const initStarted = useRef(false);
  const scrollRafRef = useRef<number | null>(null);

  const currentModeConfig = CHAT_MODES.find((m) => m.id === currentMode)!;
  const isStreaming = streamStatus === 'streaming';
  const isLoading = loadingState === 'loading';

  // 自定义 hooks
  const { selectedImages, previewUrls, isUploading, handleImageSelect, handleRemoveImage, clearImages } =
    useImageUpload();

  const {
    files: uploadedFiles,
    isParsing: isFileParsing,
    handleFileSelect,
    handleRemoveFile,
    clearFiles,
    getParsedDocuments,
  } = useFileUpload();

  const { handleSendMessage: sendMessage } = useChatStream({
    currentSession,
    isStreaming,
    isUploading,
    selectedImages,
    sseControllerRef,
    onClearImages: clearImages,
    getParsedDocuments,
    onClearFiles: clearFiles,
  });

  // 加载历史会话列表
  useEffect(() => {
    dispatch(fetchSessionsAsync({}));
  }, [dispatch]);

  // 滚动到底部 — 流式时即时滚动，非流式时 smooth 动画
  const scrollToBottom = useCallback((smooth = false) => {
    if (scrollRafRef.current !== null) {
      cancelAnimationFrame(scrollRafRef.current);
    }
    scrollRafRef.current = requestAnimationFrame(() => {
      scrollRafRef.current = null;
      const el = messagesContainerRef.current;
      if (!el) return;
      if (smooth) {
        el.scrollTo({
          top: el.scrollHeight - el.clientHeight,
          behavior: 'smooth',
        });
      } else {
        el.scrollTop = el.scrollHeight - el.clientHeight;
      }
    });
  }, []);

  // 消息变化时滚动 — 流式过程用即时滚动避免动画排队
  useEffect(() => {
    scrollToBottom(streamStatus !== 'streaming');
  }, [messages, scrollToBottom, streamStatus]);

  // 初始化会话
  useEffect(() => {
    const initSession = async () => {
      if (sessionId && sessionId !== 'new') {
        // 如果当前会话已匹配（如刚创建完 navigate 过来），跳过历史拉取，
        // 避免与自动发送初始消息的 useEffect 产生竞态条件
        if (currentSession?.id === sessionId) return;

        dispatch(fetchHistoryAsync({ sessionId }));
        const existingSession = sessions.find((s) => s.id === sessionId);
        if (existingSession) {
          dispatch(setCurrentSession(existingSession));
        }
      } else {
        if (initStarted.current) return;
        initStarted.current = true;

        const launchMode = locationState?.mode ?? currentMode;
        const result = await dispatch(createSessionAsync({
          topic: locationState?.topic,
          mode: launchMode,
        }));
        if (createSessionAsync.fulfilled.match(result)) {
          navigate(`/session/${result.payload.session.id}`, {
            replace: true,
            state: locationState,
          });
          dispatch(fetchSessionsAsync({}));
        }
      }
    };

    initSession();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // 清理 SSE 连接和 rAF
  useEffect(() => {
    const controller = sseControllerRef.current;
    return () => {
      controller?.close();
      if (scrollRafRef.current !== null) {
        cancelAnimationFrame(scrollRafRef.current);
      }
    };
  }, []);

  // 发送消息
  const handleSendMessage = useCallback(
    async (customMessage?: string) => {
      const messageContent = customMessage || inputValue;
      await sendMessage(messageContent);
      setInputValue('');
    },
    [inputValue, sendMessage]
  );

  // 从刷题页面跳转时，自动发送初始消息
  useEffect(() => {
    if (
      sessionId &&
      sessionId !== 'new' &&
      currentSession?.id === sessionId &&
      locationState?.initialMessage &&
      !initialMessageHandled.current &&
      !isStreaming
    ) {
      initialMessageHandled.current = true;
      handleSendMessage(locationState.initialMessage);
      // 清除 state 防止刷新后重复发送（用 replaceState 避免触发 React Router 重渲染）
      window.history.replaceState(null, '', location.pathname);
    }
  }, [currentSession, locationState, isStreaming, handleSendMessage, location.pathname, sessionId]);

  // 取消响应
  const handleCancelResponse = useCallback(() => {
    sseControllerRef.current?.close();
    dispatch(cancelCurrentTaskAsync());
  }, [dispatch]);

  // 切换模式
  const handleModeChange = useCallback(
    async (mode: ChatMode) => {
      if (isStreaming) return;
      dispatch(setMode(mode));
      if (currentSession) {
        dispatch(updateSessionModeAsync({ sessionId: currentSession.id, mode }));
      }
    },
    [isStreaming, currentSession, dispatch]
  );

  // 新建会话
  const handleNewSession = useCallback(async () => {
    if (isStreaming) return;
    initStarted.current = false;

    const result = await dispatch(createSessionAsync({ mode: currentMode }));
    if (createSessionAsync.fulfilled.match(result)) {
      initStarted.current = true;
      navigate(`/session/${result.payload.session.id}`, { replace: true });
      dispatch(fetchSessionsAsync({}));
    }
  }, [isStreaming, currentMode, dispatch, navigate]);

  // 切换到历史会话
  const handleSelectSession = useCallback(
    (sessionId: string) => {
      if (isStreaming) return;
      navigate(`/session/${sessionId}`);
    },
    [isStreaming, navigate]
  );

  // 删除会话
  const handleDeleteSession = useCallback(
    async (sessionId: string) => {
      setDeletingSessionId(sessionId);
      await dispatch(deleteSessionAsync(sessionId));
      setDeletingSessionId(null);

      if (currentSession?.id === sessionId) {
        handleNewSession();
      }
    },
    [dispatch, currentSession, handleNewSession]
  );

  // 批量删除会话
  const handleBatchDeleteSessions = useCallback(async () => {
    if (selectedSessionIds.length === 0) return;

    setIsBatchDeleting(true);
    await dispatch(batchDeleteSessionsAsync(selectedSessionIds));
    setIsBatchDeleting(false);
    setSelectedSessionIds([]);
    setIsSelectMode(false);

    if (currentSession && selectedSessionIds.includes(currentSession.id)) {
      handleNewSession();
    }
  }, [selectedSessionIds, dispatch, currentSession, handleNewSession]);

  // 切换选择模式
  const handleToggleSelectMode = useCallback(() => {
    setIsSelectMode((prev) => !prev);
    setSelectedSessionIds([]);
  }, []);

  // 切换会话选中状态
  const handleToggleSessionSelection = useCallback((sessionId: string) => {
    setSelectedSessionIds((prev) =>
      prev.includes(sessionId) ? prev.filter((id) => id !== sessionId) : [...prev, sessionId]
    );
  }, []);

  // 全选/取消全选
  const handleSelectAllSessions = useCallback(() => {
    if (selectedSessionIds.length === sessions.length) {
      setSelectedSessionIds([]);
    } else {
      setSelectedSessionIds(sessions.map((s) => s.id));
    }
  }, [selectedSessionIds, sessions]);

  return (
    <MainLayout>
      <div className="flex h-[calc(100vh-4rem)] bg-surface-50 dark:bg-surface-900">
        {/* 侧边栏 */}
        <ChatSidebar
          isOpen={sidebarOpen}
          sessions={sessions}
          currentSessionId={currentSession?.id}
          isSelectMode={isSelectMode}
          selectedSessionIds={selectedSessionIds}
          deletingSessionId={deletingSessionId}
          isBatchDeleting={isBatchDeleting}
          loading={sessionsLoadingState === 'loading'}
          onToggleSidebar={() => setSidebarOpen((prev) => !prev)}
          onNewSession={handleNewSession}
          onSelectSession={handleSelectSession}
          onDeleteSession={handleDeleteSession}
          onToggleSelectMode={handleToggleSelectMode}
          onToggleSessionSelection={handleToggleSessionSelection}
          onSelectAll={handleSelectAllSessions}
          onBatchDelete={handleBatchDeleteSessions}
        />

        {/* 主聊天区域 */}
        <div className="flex-1 flex flex-col">
          {/* 顶部栏 + 同一行的模式选择器 */}
          <ChatHeader
            currentMode={currentModeConfig}
            sidebarOpen={sidebarOpen}
            onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
            rightSlot={
              <ModeSelector
                modes={CHAT_MODES}
                currentMode={currentMode}
                onModeChange={handleModeChange}
                disabled={isStreaming}
              />
            }
          />

          {/* 消息列表 */}
          <ChatMessages
            messages={messages}
            streamingMessageId={streamingMessageId}
            isLoading={isLoading}
            error={error}
            messagesContainerRef={messagesContainerRef}
          />

          {/* 快捷操作 */}
          {messages.length === 0 && !isLoading && (
            <QuickActions actions={QUICK_ACTIONS} onActionClick={handleSendMessage} />
          )}

          {/* 输入区域 */}
          <ChatInput
            value={inputValue}
            selectedImages={selectedImages}
            previewUrls={previewUrls}
            isStreaming={isStreaming}
            isUploading={isUploading}
            disabled={!currentSession}
            files={uploadedFiles}
            isFileParsing={isFileParsing}
            onChange={setInputValue}
            onSend={handleSendMessage}
            onCancel={handleCancelResponse}
            onImageSelect={handleImageSelect}
            onRemoveImage={handleRemoveImage}
            onFileSelect={handleFileSelect}
            onRemoveFile={handleRemoveFile}
          />
        </div>
      </div>
    </MainLayout>
  );
};

export default SessionChatPage;
