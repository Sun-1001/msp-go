import { useCallback, useEffect, useRef } from 'react';
import { useAppDispatch } from '@/store';
import {
  addMessage,
  appendToLastMessage,
  setCurrentTaskId,
  setStreamStatus,
  setStreamingMessageId,
} from '@/modules/session/store/sessionSlice';
import { sessionService } from '@/modules/session/services/sessionService';
import { uploadService } from '@/modules/upload/services/uploadService';
import { formatDocumentAsContext, type ParsedDocument } from '@/libs/utils/documentParser';
import type { LearningSession } from '@/types';
import type { SSEController } from '@/libs/http/sseClient';

interface UseChatStreamProps {
  currentSession: LearningSession | null;
  isStreaming: boolean;
  isUploading: boolean;
  selectedImages: File[];
  sseControllerRef: React.MutableRefObject<SSEController | null>;
  onClearImages: () => void;
  /** 获取已解析的文档列表 */
  getParsedDocuments: () => ParsedDocument[];
  /** 清空文件 */
  onClearFiles: () => void;
}

export const useChatStream = ({
  currentSession,
  isStreaming,
  isUploading,
  selectedImages,
  sseControllerRef,
  onClearImages,
  getParsedDocuments,
  onClearFiles,
}: UseChatStreamProps) => {
  const dispatch = useAppDispatch();

  // 流式更新：rAF 节流相关 refs
  const contentBufferRef = useRef<string>('');
  const rafIdRef = useRef<number | null>(null);

  // 取消待执行的 rAF 刷新
  const cancelPendingFlush = useCallback(() => {
    if (rafIdRef.current !== null) {
      cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = null;
    }
  }, []);

  // 刷新缓冲区中的剩余内容到 Redux
  const flushBuffer = useCallback(() => {
    cancelPendingFlush();
    if (contentBufferRef.current) {
      dispatch(appendToLastMessage(contentBufferRef.current));
      contentBufferRef.current = '';
    }
  }, [dispatch, cancelPendingFlush]);

  useEffect(() => () => {
    flushBuffer();
  }, [flushBuffer]);

  // 发送消息
  const handleSendMessage = useCallback(
    async (messageContent: string) => {
      if ((!messageContent.trim() && selectedImages.length === 0 && getParsedDocuments().length === 0) || !currentSession || isStreaming || isUploading)
        return;

      const userMessageId = crypto.randomUUID();
      const aiMessageId = crypto.randomUUID();

      // 上传图片
      let uploadedImageUrls: string[] = [];
      if (selectedImages.length > 0) {
        try {
          const uploadPromises = selectedImages.map((file) => uploadService.uploadImage(file));
          const results = await Promise.all(uploadPromises);
          uploadedImageUrls = results.map((r) => r.url);
        } catch (error) {
          console.error('图片上传失败:', error);
          return;
        }
      }

      // 拼接文档内容到消息
      const parsedDocs = getParsedDocuments();
      const docContext = formatDocumentAsContext(parsedDocs);
      const fullMessage = docContext
        ? `${docContext}\n\n---\n\n${messageContent}`
        : messageContent;

      // 用户消息中显示原始输入 + 文件名提示
      const displayMessage = parsedDocs.length > 0
        ? `${messageContent}\n\n📎 ${parsedDocs.map((d) => d.filename).join(', ')}`
        : messageContent;

      // 1. 添加用户消息到 UI
      dispatch(
        addMessage({
          id: userMessageId,
          sessionId: currentSession.id,
          role: 'user',
          content: displayMessage,
          timestamp: new Date().toISOString(),
          attachments: uploadedImageUrls,
        })
      );

      // 2. 创建 AI 消息占位
      dispatch(
        addMessage({
          id: aiMessageId,
          sessionId: currentSession.id,
          role: 'assistant',
          content: '',
          timestamp: new Date().toISOString(),
          metadata: { agent: null },
        })
      );

      // 3. 设置流式状态
      dispatch(setStreamStatus('streaming'));
      dispatch(setStreamingMessageId(aiMessageId));

      // 清理图片和文件状态
      onClearImages();
      onClearFiles();

      // 4. 建立 SSE 连接
      sseControllerRef.current = sessionService.chatStream(
        currentSession.id,
        fullMessage,
        {
          onTaskInfo: (taskId: string) => {
            dispatch(setCurrentTaskId(taskId));
          },
          onChunk: (content: string) => {
            // rAF 节流：缓冲内容，每帧最多 dispatch 一次
            contentBufferRef.current += content;
            if (rafIdRef.current === null) {
              rafIdRef.current = requestAnimationFrame(() => {
                if (contentBufferRef.current) {
                  dispatch(appendToLastMessage(contentBufferRef.current));
                  contentBufferRef.current = '';
                }
                rafIdRef.current = null;
              });
            }
          },
          onDone: () => {
            flushBuffer();
            dispatch(setStreamStatus('idle'));
            dispatch(setStreamingMessageId(null));
            dispatch(setCurrentTaskId(null));
            sseControllerRef.current = null;
          },
          onError: (error: { code: string; message: string }) => {
            console.error('SSE error:', error);
            flushBuffer();
            dispatch(setStreamStatus('error'));
            dispatch(setStreamingMessageId(null));
            dispatch(setCurrentTaskId(null));
            dispatch(appendToLastMessage(`\n\n[错误: ${error.message}]`));
            sseControllerRef.current = null;
          },
          onCancelled: () => {
            flushBuffer();
            dispatch(setStreamStatus('cancelled'));
            dispatch(setStreamingMessageId(null));
            dispatch(setCurrentTaskId(null));
            dispatch(appendToLastMessage('\n\n[响应已取消]'));
            sseControllerRef.current = null;
          },
          onClose: () => {
            flushBuffer();
            dispatch(setStreamStatus('idle'));
            dispatch(setStreamingMessageId(null));
            dispatch(setCurrentTaskId(null));
            sseControllerRef.current = null;
          },
        },
        uploadedImageUrls.length > 0 ? uploadedImageUrls : undefined
      );
    },
    [
      currentSession,
      isStreaming,
      isUploading,
      selectedImages,
      sseControllerRef,
      onClearImages,
      onClearFiles,
      getParsedDocuments,
      dispatch,
      flushBuffer,
    ]
  );

  return {
    handleSendMessage,
  };
};
