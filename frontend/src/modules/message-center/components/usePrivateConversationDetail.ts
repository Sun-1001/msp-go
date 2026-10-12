import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import {
  isRequestCancelled,
  toAppError,
  type AppError,
} from '@/libs/http/appError';
import {
  conversationService,
  type ConversationDetail,
} from '@/modules/message-center/services/conversationService';
import {
  fetchStableOffsetMessageWindow,
  mergeMessagesByID,
  appendDeliveredMessage,
} from '@/modules/message-center/pageUtils';
import { useConversationMessageSearch } from '@/modules/message-center/components/useConversationMessageSearch';
import { useVisibleMessageCutoff } from '@/modules/message-center/components/useVisibleMessageCutoff';
import type { MessageAttachment } from '@/modules/message-center/attachmentTypes';

export type ConversationScrollIntent =
  | { type: 'bottom'; conversationID: string }
  | { type: 'preserve'; conversationID: string; scrollHeight: number; scrollTop: number };

function isConversationViewportNearBottom(viewport: HTMLDivElement): boolean {
  return viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= 80;
}

export interface UsePrivateConversationDetailOptions {
  initialConversationID: string;
  active: boolean;
  pageVisible: boolean;
  notifyRequestError: (error: unknown, fallback: string) => void;
  invalidateConversationList: () => void;
  onConversationMutated: () => Promise<void>;
}

export function usePrivateConversationDetail(options: UsePrivateConversationDetailOptions) {
  const {
    initialConversationID,
    active,
    pageVisible,
    notifyRequestError,
    invalidateConversationList,
    onConversationMutated,
  } = options;
  const conversationRequest = useRef(0);
  const acknowledgedConversationCutoff = useRef('');
  const acknowledgingConversationCutoff = useRef('');

  const [activeConv, setActiveConv] = useState<ConversationDetail | null>(null);
  const [activeConvId, setActiveConvId] = useState(initialConversationID);
  const activeConvIDRef = useRef(activeConvId);
  const conversationListModeRef = useRef(!activeConvId);
  const conversationDetailLoadingRef = useRef(Boolean(activeConvId));
  const [conversationDetailLoading, setConversationDetailLoading] = useState(Boolean(activeConvId));
  const [conversationDetailError, setConversationDetailError] = useState<AppError | null>(null);
  const [messageDrafts, setMessageDrafts] = useState<Record<string, string>>({});
  const messageDraft = activeConvId ? messageDrafts[activeConvId] ?? '' : '';
  const [messageAttachmentDrafts, setMessageAttachmentDrafts] = useState<Record<string, MessageAttachment[]>>({});
  const messageAttachments = useMemo(
    () => activeConvId ? messageAttachmentDrafts[activeConvId] ?? [] : [],
    [activeConvId, messageAttachmentDrafts],
  );
  const [messageUploading, setMessageUploading] = useState(false);
  const [sendingMsg, setSendingMsg] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const loadingOlderMessagesRef = useRef(false);
  const conversationViewportRef = useRef<HTMLDivElement>(null);
  const conversationScrollIntentRef = useRef<ConversationScrollIntent | null>(
    activeConvId ? { type: 'bottom', conversationID: activeConvId } : null,
  );
  const [conversationSearchOpen, setConversationSearchOpen] = useState(false);
  const [conversationMessageSearch, setConversationMessageSearch] = useState('');

  const conversationSearch = useConversationMessageSearch(activeConvId, conversationMessageSearch, conversationSearchOpen && active);
  const loadingConversationSearch = conversationSearch.loading;
  const conversationSearchError = conversationSearch.error?.message ?? '';
  const visibleConversationCutoff = useVisibleMessageCutoff(
    conversationViewportRef, activeConvId, activeConv?.messages ?? [],
    pageVisible && active && !conversationSearchOpen && !conversationDetailLoading,
  );

  const activateConversation = useCallback((id: string): boolean => {
    conversationListModeRef.current = !id;
    conversationScrollIntentRef.current = id ? { type: 'bottom', conversationID: id } : null;
    if (activeConvIDRef.current === id) return false;
    activeConvIDRef.current = id;
    conversationRequest.current++;
    loadingOlderMessagesRef.current = false;
    setActiveConvId(id);
    setActiveConv(null);
    setLoadingOlderMessages(false);
    setConversationSearchOpen(false);
    setConversationMessageSearch('');
    conversationDetailLoadingRef.current = Boolean(id);
    setConversationDetailLoading(Boolean(id));
    setConversationDetailError(null);
    return true;
  }, []);

  const loadConversationDetail = useCallback(async (id: string, preserveLoadedMessages = false): Promise<boolean> => {
    const request = ++conversationRequest.current;
    conversationDetailLoadingRef.current = true;
    if (!preserveLoadedMessages) {
      conversationScrollIntentRef.current = { type: 'bottom', conversationID: id };
      setConversationDetailLoading(true);
    }
    setConversationDetailError(null);
    try {
      const detail = await conversationService.get(id);
      if (request === conversationRequest.current && activeConvIDRef.current === id) {
        setActiveConv((current) => preserveLoadedMessages && current?.id === detail.id ? {
          ...detail,
          messages: mergeMessagesByID(current.messages, detail.messages),
          messages_page: current.messages_page,
          messages_page_size: current.messages_page_size,
        } : detail);
        conversationDetailLoadingRef.current = false;
        setConversationDetailLoading(false);
      }
      return true;
    } catch (error) {
      if (request === conversationRequest.current && activeConvIDRef.current === id) {
        if (!preserveLoadedMessages) {
          setActiveConv(null);
          if (!isRequestCancelled(error)) {
            setConversationDetailError(toAppError(error, '私信详情加载失败，请稍后重试'));
          }
        }
        conversationDetailLoadingRef.current = false;
        setConversationDetailLoading(false);
      }
      return false;
    }
  }, []);

  useEffect(() => {
    if (!active) {
      conversationRequest.current++;
      conversationDetailLoadingRef.current = false;
      setConversationDetailLoading(false);
      return;
    }
    if (!activeConvId) {
      conversationRequest.current++;
      setActiveConv(null);
      conversationDetailLoadingRef.current = false;
      setConversationDetailLoading(false);
      setConversationDetailError(null);
      return;
    }
    setActiveConv(null);
    void loadConversationDetail(activeConvId);
  }, [activeConvId, active, loadConversationDetail]);

  useLayoutEffect(() => {
    const viewport = conversationViewportRef.current;
    const intent = conversationScrollIntentRef.current;
    if (!viewport || !intent || !activeConv || activeConv.id !== activeConvId || intent.conversationID !== activeConv.id) return;

    if (intent.type === 'bottom') {
      viewport.scrollTop = viewport.scrollHeight;
    } else {
      viewport.scrollTop = Math.max(0, intent.scrollTop + viewport.scrollHeight - intent.scrollHeight);
    }
    if (conversationScrollIntentRef.current === intent) conversationScrollIntentRef.current = null;
  }, [activeConv, activeConvId]);

  useEffect(() => {
    const throughMessageID = visibleConversationCutoff;
    const cutoffKey = activeConv && throughMessageID ? `${activeConv.id}:${throughMessageID}` : '';
    if (!pageVisible || !visibleConversationCutoff || !active || !activeConv || !throughMessageID || activeConv.id !== activeConvId || acknowledgedConversationCutoff.current === cutoffKey || acknowledgingConversationCutoff.current === cutoffKey) return;
    const conversationID = activeConv.id;
    const controller = new AbortController();
    acknowledgingConversationCutoff.current = cutoffKey;
    invalidateConversationList();
    void conversationService.acknowledgeRead(conversationID, throughMessageID, controller.signal).then(async () => {
      acknowledgedConversationCutoff.current = cutoffKey;
      await onConversationMutated();
    }).catch((error) => {
      if (controller.signal.aborted) return;
      notifyRequestError(error, '私信已显示，但同步已读状态失败');
    }).finally(() => {
      if (acknowledgingConversationCutoff.current === cutoffKey) acknowledgingConversationCutoff.current = '';
    });
    return () => controller.abort();
  }, [activeConv, activeConvId, active, visibleConversationCutoff, invalidateConversationList, notifyRequestError, onConversationMutated, pageVisible]);

  const sendPrivateMessage = useCallback(async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    if (!activeConv || activeConv.id !== activeConvId || (!messageDraft.trim() && messageAttachments.length === 0) || sendingMsg || messageUploading) return;
    const conversationID = activeConv.id;
    const submittedDraft = messageDraft;
    const submittedAttachments = messageAttachments;
    conversationRequest.current++;
    invalidateConversationList();
    setSendingMsg(true);
    try {
      const message = await conversationService.sendMessage(conversationID, submittedDraft.trim(), submittedAttachments);
      if (activeConvIDRef.current === conversationID) {
        conversationRequest.current++;
        conversationScrollIntentRef.current = { type: 'bottom', conversationID };
        conversationDetailLoadingRef.current = false;
        setConversationDetailLoading(false);
        setConversationDetailError(null);
        setConversationSearchOpen(false);
        setConversationMessageSearch('');
      }
      setActiveConv((current) => activeConvIDRef.current === conversationID
        ? appendDeliveredMessage(current?.id === conversationID ? current : activeConv, message)
        : current);
      setMessageDrafts((current) => {
        if ((current[conversationID] ?? '') !== submittedDraft) return current;
        const next = { ...current };
        delete next[conversationID];
        return next;
      });
      setMessageAttachmentDrafts((current) => {
        if (current[conversationID] !== submittedAttachments) return current;
        const next = { ...current };
        delete next[conversationID];
        return next;
      });
      await onConversationMutated();
    } catch (error) {
      notifyRequestError(error, '发送私信失败，请稍后重试');
    }
    finally { setSendingMsg(false); }
  }, [activeConv, activeConvId, messageAttachments, messageDraft, messageUploading, sendingMsg, invalidateConversationList, onConversationMutated, notifyRequestError]);

  const loadOlderConversationMessages = useCallback(async () => {
    if (!activeConv || activeConv.id !== activeConvIDRef.current || loadingOlderMessagesRef.current || activeConv.messages.length >= activeConv.messages_total) return;
    const conversationID = activeConv.id;
    const request = ++conversationRequest.current;
    loadingOlderMessagesRef.current = true;
    setLoadingOlderMessages(true);
    try {
      const nextPage = activeConv.messages_page + 1;
      const detail = await conversationService.get(conversationID, { messages_page: nextPage, messages_page_size: activeConv.messages_page_size });
      if (request !== conversationRequest.current) return;
      let messages = mergeMessagesByID(activeConv.messages, detail.messages);
      let messagesTotal = detail.messages_total;
      const headDetail = await conversationService.get(conversationID, { messages_page: 1, messages_page_size: activeConv.messages_page_size });
      if (request !== conversationRequest.current) return;
      const currentHeadID = activeConv.messages.at(-1)?.id ?? '';
      const serverHeadID = headDetail.messages.at(-1)?.id ?? '';
      const headShifted = currentHeadID !== serverHeadID
        || activeConv.messages_total !== headDetail.messages_total
        || detail.messages_total !== headDetail.messages_total;
      const loadedWindowDrifted = activeConv.messages.length < headDetail.messages_total
        && activeConv.messages.length !== Math.min(
          headDetail.messages_total,
          activeConv.messages_page * activeConv.messages_page_size,
        );
      if (headShifted || loadedWindowDrifted || messages.length < Math.min(messagesTotal, nextPage * activeConv.messages_page_size)) {
        const stableWindow = await fetchStableOffsetMessageWindow(nextPage, activeConv.messages_page_size, async (messagesPage) => {
          const pageDetail = await conversationService.get(conversationID, { messages_page: messagesPage, messages_page_size: activeConv.messages_page_size });
          return { messages: pageDetail.messages, messages_total: pageDetail.messages_total };
        });
        if (request !== conversationRequest.current) return;
        messages = stableWindow.messages;
        messagesTotal = stableWindow.total;
      }
      const viewport = conversationViewportRef.current;
      if (viewport && activeConvIDRef.current === conversationID) {
        conversationScrollIntentRef.current = {
          type: 'preserve',
          conversationID,
          scrollHeight: viewport.scrollHeight,
          scrollTop: viewport.scrollTop,
        };
      }
      setActiveConv((current) => current?.id === detail.id ? {
        ...detail,
        read_through_message_id: current.read_through_message_id,
        messages,
        messages_total: messagesTotal,
        messages_page: nextPage,
      } : current);
    } catch (error) {
      if (request === conversationRequest.current && activeConvIDRef.current === conversationID) {
        notifyRequestError(error, '加载更早私信失败，请稍后重试');
      }
    } finally {
      if (activeConvIDRef.current === conversationID) {
        loadingOlderMessagesRef.current = false;
        setLoadingOlderMessages(false);
      }
    }
  }, [activeConv, notifyRequestError]);

  const refreshActiveConversation = useCallback(async (signal: AbortSignal): Promise<void> => {
    const currentConversationID = activeConvIDRef.current;
    if (!currentConversationID) return;
    const request = ++conversationRequest.current;
    try {
      const detail = await conversationService.get(currentConversationID, undefined, signal);
      if (signal.aborted || document.hidden || request !== conversationRequest.current || activeConvIDRef.current !== currentConversationID) return;
      const viewport = conversationViewportRef.current;
      if (!conversationSearchOpen && viewport && isConversationViewportNearBottom(viewport)) {
        conversationScrollIntentRef.current = { type: 'bottom', conversationID: currentConversationID };
      }
      setActiveConv((current) => current?.id === detail.id ? {
        ...detail,
        messages: mergeMessagesByID(current.messages, detail.messages),
        messages_page: current.messages_page,
        messages_page_size: current.messages_page_size,
      } : current);
    } catch { /* retain the last successfully loaded detail */ }
  }, [conversationSearchOpen]);

  const discardDrafts = useCallback((ids: Iterable<string>) => {
    const discarded = [...ids];
    setMessageDrafts((current) => {
      const nextDrafts = { ...current };
      discarded.forEach((id) => delete nextDrafts[id]);
      return nextDrafts;
    });
    setMessageAttachmentDrafts((current) => {
      const nextDrafts = { ...current };
      discarded.forEach((id) => delete nextDrafts[id]);
      return nextDrafts;
    });
  }, []);

  const toggleConversationSearch = useCallback(() => {
    setConversationSearchOpen((current) => !current);
    setConversationMessageSearch('');
  }, []);

  const setMessageDraft = useCallback((value: string) => {
    const conversationID = activeConvIDRef.current;
    if (conversationID) setMessageDrafts((current) => ({ ...current, [conversationID]: value }));
  }, []);

  const setMessageAttachments = useCallback((attachments: MessageAttachment[]) => {
    const conversationID = activeConvIDRef.current;
    if (conversationID) setMessageAttachmentDrafts((current) => ({ ...current, [conversationID]: attachments }));
  }, []);

  const visibleConversationMessages = conversationSearchOpen && conversationMessageSearch.trim()
    ? conversationSearch.messages
    : activeConv?.messages ?? [];

  return {
    activeConv,
    setActiveConv,
    activeConvId,
    activeConvIDRef,
    conversationListModeRef,
    conversationDetailLoading,
    conversationDetailLoadingRef,
    conversationDetailError,
    messageDraft,
    setMessageDraft,
    messageAttachments,
    setMessageAttachments,
    messageUploading,
    setMessageUploading,
    sendingMsg,
    loadingOlderMessages,
    loadingOlderMessagesRef,
    conversationViewportRef,
    conversationSearchOpen,
    conversationMessageSearch,
    setConversationMessageSearch,
    conversationSearch,
    loadingConversationSearch,
    conversationSearchError,
    visibleConversationMessages,
    activateConversation,
    loadConversationDetail,
    sendPrivateMessage,
    loadOlderConversationMessages,
    refreshActiveConversation,
    discardDrafts,
    toggleConversationSearch,
  };
}
