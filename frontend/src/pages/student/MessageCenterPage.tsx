import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { MainLayout } from '@/components/layout/MainLayout';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs';
import { useToast } from '@/components/ui/Toast';
import {
  Archive,
  ArrowLeft,
  Bell,
  CheckCircle2,
  HelpCircle,
  Import,
  Loader2,
  MessageSquare,
  Paperclip,
  Plus,
  Search,
  Send,
  UserRound,
} from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { formatRelativeTime } from '@/libs/utils/dateFormat';
import { useSerialPolling } from '@/hooks/useSerialPolling';
import {
  conversationService,
  type ConversationItem,
  type ConversationDetail,
  type Contact,
} from '@/modules/message-center/services/conversationService';
import {
  noticeService,
  type StudentNoticeItem,
  type StudentNoticeListItem,
} from '@/modules/message-center/services/noticeService';
import {
  qaThreadService,
  type StudentThreadItem,
  type ThreadDetail,
} from '@/modules/message-center/services/qaThreadService';
import {
  refreshMessageCenterSummaryAfterMutation,
  useMessageCenterSummary,
} from '@/modules/message-center/components/useMessageCenterSummary';
import {
  useObservedVisibility,
  usePageVisibility,
} from '@/modules/message-center/components/useObservedVisibility';
import {
  fetchLoadedPageRange,
  fetchStableOffsetMessageWindow,
  hasMinimumGlobalSearchCharacters,
  latestPageChanged,
  mergeByID,
  mergeLatestPageByID,
  mergeMessagesByID,
  selectListItemID,
} from '@/modules/message-center/pageUtils';
import { TabCount } from '@/modules/message-center/TabCount';
import {
  fetchMistakes,
  type MistakeRecord,
} from '@/modules/mistake/services/mistakeService';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
const statusVariant = {
  '待回复': 'warning',
  '已回复': 'default',
  '已解决': 'success',
} as const;
const noticeStatuses = ['全部', '待确认', '已确认'];
const listPageSize = 50;

interface ListLoadOptions {
  refreshLoadedPages?: boolean;
  signal?: AbortSignal;
}

const studentTabs = new Set(['private', 'notices', 'questions']);

function parseStudentTab(value: string | null): string {
  return value && studentTabs.has(value) ? value : 'private';
}

function mapConversationItem(c: ConversationItem) {
  return {
    id: c.id,
    teacherId: c.teacher_id ?? '',
    teacherName: c.teacher_name ?? '',
    scope: c.scope ?? '',
    lastMessage: c.last_message,
    lastTime: formatRelativeTime(c.last_time),
    unread: c.unread,
    archived: c.archived,
    messages: [] as Array<{ id: string; from: string; text: string; time: string; readByRecipient?: boolean }>,
  };
}

function mapNoticeListItem(n: StudentNoticeListItem) {
  return {
    id: n.id,
    className: n.class_name,
    title: n.title,
    publishedAt: formatRelativeTime(n.published_at),
    confirmed: n.confirmed,
  };
}

function mapNoticeDetail(n: StudentNoticeItem) {
  return {
    ...mapNoticeListItem(n),
    body: n.body,
    attachments: n.attachments,
  };
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export const MessageCenterPage: React.FC = () => {
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const initialTab = parseStudentTab(searchParams.get('tab'));
  const initialItemID = searchParams.get('id') ?? '';
  const conversationRequest = useRef(0);
  const noticeRequest = useRef(0);
  const threadRequest = useRef(0);
  const conversationListRequest = useRef(0);
  const noticeListRequest = useRef(0);
  const questionListRequest = useRef(0);
  const reloadRequest = useRef(0);
  const initialLoadStarted = useRef(false);
  const lastListQuery = useRef(`${initialTab}\u0000\u0000全部`);
  const handledLocationKey = useRef(location.key);
  const pendingDeepLink = useRef(initialItemID ? { tab: initialTab, id: initialItemID } : null);
  const contactSearchRequest = useRef(0);
  const acknowledgedConversationCutoff = useRef('');
  const acknowledgedThreadCutoff = useRef('');
  const acknowledgingConversationCutoff = useRef('');
  const acknowledgingThreadCutoff = useRef('');
  const { toast } = useToast();
  const {
    summary,
    error: summaryError,
    isRefreshing: summaryRefreshing,
    refresh: refreshSummary,
  } = useMessageCenterSummary();
  const pageVisible = usePageVisibility();
  const { ref: conversationDetailRef, isVisible: conversationDetailVisible } = useObservedVisibility<HTMLDivElement>();
  const { ref: threadDetailRef, isVisible: threadDetailVisible } = useObservedVisibility<HTMLDivElement>();
  // ---- state ---------------------------------------------------------
  const [searchTerm, setSearchTerm] = useState('');
  const [serverSearch, setServerSearch] = useState('');
  const [activeTab, setActiveTab] = useState(initialTab);
  const [initialLoad, setInitialLoad] = useState(true);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');

  // conversations
  const [convItems, setConvItems] = useState<ReturnType<typeof mapConversationItem>[]>([]);
  const [activeConv, setActiveConv] = useState<ConversationDetail | null>(null);
  const [activeConvId, setActiveConvId] = useState(initialTab === 'private' ? initialItemID : '');
  const activeConvIDRef = useRef(activeConvId);
  const conversationListModeRef = useRef(false);
  const conversationDetailLoadingRef = useRef(Boolean(activeConvId));
  const [conversationDetailLoading, setConversationDetailLoading] = useState(Boolean(activeConvId));
  const [conversationDetailError, setConversationDetailError] = useState('');
  const [messageDrafts, setMessageDrafts] = useState<Record<string, string>>({});
  const messageDraft = activeConvId ? messageDrafts[activeConvId] ?? '' : '';
  const [sendingMsg, setSendingMsg] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const loadingOlderMessagesRef = useRef(false);
  const [conversationPage, setConversationPage] = useState(1);
  const [conversationTotal, setConversationTotal] = useState(0);

  // new conversation modal
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [newConvOpen, setNewConvOpen] = useState(false);
  const [selectedTeacherId, setSelectedTeacherId] = useState('');
  const [contactSearch, setContactSearch] = useState('');
  const [globalSearchResults, setGlobalSearchResults] = useState<Contact[]>([]);
  const [newConvDraft, setNewConvDraft] = useState('');
  const [creatingConv, setCreatingConv] = useState(false);

  // notices
  const [notices, setNotices] = useState<ReturnType<typeof mapNoticeListItem>[]>([]);
  const [activeNotice, setActiveNotice] = useState<ReturnType<typeof mapNoticeDetail> | null>(null);
  const [activeNoticeId, setActiveNoticeId] = useState(initialTab === 'notices' ? initialItemID : '');
  const activeNoticeIDRef = useRef(activeNoticeId);
  const [noticeDetailLoading, setNoticeDetailLoading] = useState(false);
  const [noticeDetailError, setNoticeDetailError] = useState('');
  const [noticeStatus, setNoticeStatus] = useState('全部');
  const [confirming, setConfirming] = useState('');
  const [noticePage, setNoticePage] = useState(1);
  const [noticeTotal, setNoticeTotal] = useState(0);

  // questions
  const [questions, setQuestions] = useState<StudentThreadItem[]>([]);
  const [activeThread, setActiveThread] = useState<ThreadDetail | null>(null);
  const [activeQuestionId, setActiveQuestionId] = useState(initialTab === 'questions' ? initialItemID : '');
  const activeQuestionIDRef = useRef(activeQuestionId);
  const threadDetailLoadingRef = useRef(Boolean(activeQuestionId));
  const [threadDetailLoading, setThreadDetailLoading] = useState(Boolean(activeQuestionId));
  const [threadDetailError, setThreadDetailError] = useState('');
  const [questionDraft, setQuestionDraft] = useState('');
  const [selectedQTeacherId, setSelectedQTeacherId] = useState('');
  const [submittingQ, setSubmittingQ] = useState(false);
  const [followUpDrafts, setFollowUpDrafts] = useState<Record<string, string>>({});
  const followUpDraft = activeQuestionId ? followUpDrafts[activeQuestionId] ?? '' : '';
  const [sendingFollowUp, setSendingFollowUp] = useState(false);
  const [loadingOlderThreadMessages, setLoadingOlderThreadMessages] = useState(false);
  const loadingOlderThreadMessagesRef = useRef(false);
  const [questionPage, setQuestionPage] = useState(1);
  const [questionTotal, setQuestionTotal] = useState(0);
  const [loadingMoreList, setLoadingMoreList] = useState('');
  const loadingMoreListRef = useRef(false);
  const [listLoadError, setListLoadError] = useState('');
  const tabCounts = {
    private: summary?.conversation_count ?? 0,
    notices: summary?.notice_count ?? 0,
    questions: summary?.thread_count ?? 0,
  };

  // import modal
  const [importOpen, setImportOpen] = useState(false);
  const [importTeacherId, setImportTeacherId] = useState('');
  const [importing, setImporting] = useState(false);
  const [mistakes, setMistakes] = useState<MistakeRecord[]>([]);
  const [loadingMistakes, setLoadingMistakes] = useState(false);
  const [selectedMistakeId, setSelectedMistakeId] = useState('');
  const [importQuestionText, setImportQuestionText] = useState('');
  const convItemsRef = useRef(convItems);
  const noticesRef = useRef(notices);
  const questionsRef = useRef(questions);
  const conversationTotalRef = useRef(conversationTotal);
  const noticeTotalRef = useRef(noticeTotal);
  const questionTotalRef = useRef(questionTotal);
  const conversationQueryRef = useRef(serverSearch);
  const noticeQueryRef = useRef(`${serverSearch}\u0000${noticeStatus}`);
  const questionQueryRef = useRef(serverSearch);
  conversationQueryRef.current = serverSearch;
  noticeQueryRef.current = `${serverSearch}\u0000${noticeStatus}`;
  questionQueryRef.current = serverSearch;
  convItemsRef.current = convItems;
  noticesRef.current = notices;
  questionsRef.current = questions;
  conversationTotalRef.current = conversationTotal;
  noticeTotalRef.current = noticeTotal;
  questionTotalRef.current = questionTotal;

  const consumePendingDeepLink = useCallback((tab: string): string => {
    const pending = pendingDeepLink.current;
    if (!pending || pending.tab !== tab) return '';
    pendingDeepLink.current = null;
    return pending.id;
  }, []);

  const activateConversation = useCallback((id: string): boolean => {
    if (id) conversationListModeRef.current = false;
    if (activeConvIDRef.current === id) return false;
    activeConvIDRef.current = id;
    conversationRequest.current++;
    loadingOlderMessagesRef.current = false;
    setActiveConvId(id);
    setActiveConv(null);
    setLoadingOlderMessages(false);
    conversationDetailLoadingRef.current = Boolean(id);
    setConversationDetailLoading(Boolean(id));
    setConversationDetailError('');
    return true;
  }, []);

  const activateQuestion = useCallback((id: string): boolean => {
    if (activeQuestionIDRef.current === id) return false;
    activeQuestionIDRef.current = id;
    threadRequest.current++;
    loadingOlderThreadMessagesRef.current = false;
    setActiveQuestionId(id);
    setActiveThread(null);
    setLoadingOlderThreadMessages(false);
    threadDetailLoadingRef.current = Boolean(id);
    setThreadDetailLoading(Boolean(id));
    setThreadDetailError('');
    return true;
  }, []);

  const activateNotice = useCallback((id: string): boolean => {
    if (activeNoticeIDRef.current === id) return false;
    activeNoticeIDRef.current = id;
    noticeRequest.current++;
    setActiveNoticeId(id);
    setActiveNotice(null);
    setNoticeDetailLoading(Boolean(id));
    setNoticeDetailError('');
    return true;
  }, []);

  const clearItemDeepLink = useCallback((tab: string) => {
    pendingDeepLink.current = null;
    if (!searchParams.has('id')) return;
    setSearchParams({ tab }, { replace: true });
  }, [searchParams, setSearchParams]);

  const showConversationList = useCallback(() => {
    conversationListModeRef.current = true;
    clearItemDeepLink('private');
    activateConversation('');
  }, [activateConversation, clearItemDeepLink]);

  useEffect(() => {
    const timer = window.setTimeout(() => setServerSearch(searchTerm.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [searchTerm]);

  // ---- load contacts --------------------------------------------------
  const loadContacts = useCallback(async () => {
    try {
      const { contacts: list } = await conversationService.contacts();
      setContacts(list);
      if (list.length > 0) {
        setSelectedTeacherId(list[0].id);
        setSelectedQTeacherId(list[0].id);
        setImportTeacherId(list[0].id);
      }
      return true;
    } catch { return false; }
  }, []);

  // ---- load conversations ---------------------------------------------
  const loadConversations = useCallback(async (page = 1, append = false, preserveLoadedPages = false, options: ListLoadOptions = {}) => {
    const queryKey = serverSearch;
    if (queryKey !== conversationQueryRef.current) return true;
    const request = ++conversationListRequest.current;
    try {
      let response = options.refreshLoadedPages
        ? await fetchLoadedPageRange(conversationPage, listPageSize, (loadedPage) => conversationService.list({ search: serverSearch, page: loadedPage, page_size: listPageSize }, options.signal))
        : await conversationService.list({ search: serverSearch, page, page_size: listPageSize }, options.signal);
      if (options.signal?.aborted || request !== conversationListRequest.current || queryKey !== conversationQueryRef.current) return true;
      const refreshShiftedWindow = !options.refreshLoadedPages
        && preserveLoadedPages
        && conversationPage > 1
        && latestPageChanged(convItemsRef.current, response.items, conversationTotalRef.current, response.total);
      if (refreshShiftedWindow) {
        response = await fetchLoadedPageRange(conversationPage, listPageSize, (loadedPage) => conversationService.list({ search: serverSearch, page: loadedPage, page_size: listPageSize }, options.signal));
        if (options.signal?.aborted || request !== conversationListRequest.current || queryKey !== conversationQueryRef.current) return true;
      }
      const replaceLoadedPages = options.refreshLoadedPages || refreshShiftedWindow;
      const items = response.items.map(mapConversationItem);
      setConvItems((current) => append
        ? mergeByID(current, items)
        : preserveLoadedPages && !replaceLoadedPages
          ? mergeLatestPageByID(current, items, response.total)
          : items);
      if (replaceLoadedPages) {
        setConversationPage(Math.max(1, Math.min(conversationPage, Math.ceil(response.total / listPageSize) || 1)));
      } else if (!preserveLoadedPages) {
        setConversationPage(page);
      }
      setConversationTotal(response.total);
      if (!append) {
        const deepLinkID = consumePendingDeepLink('private');
        if (deepLinkID) activateConversation(deepLinkID);
        else if (!conversationListModeRef.current) {
          if (!preserveLoadedPages && !replaceLoadedPages) activateConversation(selectListItemID(activeConvIDRef.current, items.map((item) => item.id), ''));
          else if (!activeConvIDRef.current) activateConversation(items[0]?.id ?? '');
        }
      }
      return true;
    } catch { return options.signal?.aborted || request !== conversationListRequest.current || queryKey !== conversationQueryRef.current; }
  }, [activateConversation, consumePendingDeepLink, conversationPage, serverSearch]);

  const loadConversationDetail = useCallback(async (id: string, preserveLoadedMessages = false): Promise<boolean> => {
    const request = ++conversationRequest.current;
    conversationDetailLoadingRef.current = true;
    setConversationDetailLoading(true);
    setConversationDetailError('');
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
    } catch {
      if (request === conversationRequest.current && activeConvIDRef.current === id) {
        setActiveConv(null);
        setConversationDetailError('私信详情加载失败，请稍后重试。');
        conversationDetailLoadingRef.current = false;
        setConversationDetailLoading(false);
      }
      return false;
    }
  }, []);

  // ---- load notices ---------------------------------------------------
  const loadNotices = useCallback(async (page = 1, append = false, preserveLoadedPages = false, options: ListLoadOptions = {}) => {
    const queryKey = `${serverSearch}\u0000${noticeStatus}`;
    if (queryKey !== noticeQueryRef.current) return true;
    const request = ++noticeListRequest.current;
    try {
      let response = options.refreshLoadedPages
        ? await fetchLoadedPageRange(noticePage, listPageSize, (loadedPage) => noticeService.list<StudentNoticeListItem>({ search: serverSearch, status: noticeStatus, page: loadedPage, page_size: listPageSize }, options.signal))
        : await noticeService.list<StudentNoticeListItem>({ search: serverSearch, status: noticeStatus, page, page_size: listPageSize }, options.signal);
      if (options.signal?.aborted || request !== noticeListRequest.current || queryKey !== noticeQueryRef.current) return true;
      const refreshShiftedWindow = !options.refreshLoadedPages
        && preserveLoadedPages
        && noticePage > 1
        && latestPageChanged(noticesRef.current, response.items, noticeTotalRef.current, response.total);
      if (refreshShiftedWindow) {
        response = await fetchLoadedPageRange(noticePage, listPageSize, (loadedPage) => noticeService.list<StudentNoticeListItem>({ search: serverSearch, status: noticeStatus, page: loadedPage, page_size: listPageSize }, options.signal));
        if (options.signal?.aborted || request !== noticeListRequest.current || queryKey !== noticeQueryRef.current) return true;
      }
      const replaceLoadedPages = options.refreshLoadedPages || refreshShiftedWindow;
      const items = response.items;
      const mappedItems = items.map(mapNoticeListItem);
      setNotices((current) => append
        ? mergeByID(current, mappedItems)
        : preserveLoadedPages && !replaceLoadedPages
          ? mergeLatestPageByID(current, mappedItems, response.total)
          : mappedItems);
      if (replaceLoadedPages) {
        setNoticePage(Math.max(1, Math.min(noticePage, Math.ceil(response.total / listPageSize) || 1)));
      } else if (!preserveLoadedPages) {
        setNoticePage(page);
      }
      setNoticeTotal(response.total);
      if (!append) {
        const deepLinkID = consumePendingDeepLink('notices');
        if (deepLinkID) activateNotice(deepLinkID);
        else if (!preserveLoadedPages && !replaceLoadedPages) activateNotice(selectListItemID(activeNoticeIDRef.current, items.map((item) => item.id), ''));
        else if (!activeNoticeIDRef.current) activateNotice(items[0]?.id ?? '');
      }
      return true;
    } catch { return options.signal?.aborted || request !== noticeListRequest.current || queryKey !== noticeQueryRef.current; }
  }, [activateNotice, consumePendingDeepLink, noticePage, serverSearch, noticeStatus]);

  const loadNoticeDetail = useCallback(async (id: string): Promise<boolean> => {
    const request = ++noticeRequest.current;
    setNoticeDetailLoading(true);
    setNoticeDetailError('');
    try {
      const detail = await noticeService.get(id);
      if (!('confirmed' in detail)) throw new Error('unexpected teacher notice detail');
      if (request === noticeRequest.current && activeNoticeIDRef.current === id) {
        setActiveNotice(mapNoticeDetail(detail));
        setNoticeDetailLoading(false);
      }
      return true;
    } catch {
      if (request === noticeRequest.current && activeNoticeIDRef.current === id) {
        setActiveNotice(null);
        setNoticeDetailError('通知详情加载失败，请稍后重试。');
        setNoticeDetailLoading(false);
      }
      return false;
    }
  }, []);

  // ---- load questions -------------------------------------------------
  const loadQuestions = useCallback(async (page = 1, append = false, preserveLoadedPages = false, options: ListLoadOptions = {}) => {
    const queryKey = serverSearch;
    if (queryKey !== questionQueryRef.current) return true;
    const request = ++questionListRequest.current;
    try {
      let response = options.refreshLoadedPages
        ? await fetchLoadedPageRange(questionPage, listPageSize, (loadedPage) => qaThreadService.list<StudentThreadItem>({ search: serverSearch, page: loadedPage, page_size: listPageSize }, options.signal))
        : await qaThreadService.list<StudentThreadItem>({ search: serverSearch, page, page_size: listPageSize }, options.signal);
      if (options.signal?.aborted || request !== questionListRequest.current || queryKey !== questionQueryRef.current) return true;
      const refreshShiftedWindow = !options.refreshLoadedPages
        && preserveLoadedPages
        && questionPage > 1
        && latestPageChanged(questionsRef.current, response.items, questionTotalRef.current, response.total);
      if (refreshShiftedWindow) {
        response = await fetchLoadedPageRange(questionPage, listPageSize, (loadedPage) => qaThreadService.list<StudentThreadItem>({ search: serverSearch, page: loadedPage, page_size: listPageSize }, options.signal));
        if (options.signal?.aborted || request !== questionListRequest.current || queryKey !== questionQueryRef.current) return true;
      }
      const replaceLoadedPages = options.refreshLoadedPages || refreshShiftedWindow;
      const items = response.items;
      setQuestions((current) => append
        ? mergeByID(current, items)
        : preserveLoadedPages && !replaceLoadedPages
          ? mergeLatestPageByID(current, items, response.total)
          : items);
      if (replaceLoadedPages) {
        setQuestionPage(Math.max(1, Math.min(questionPage, Math.ceil(response.total / listPageSize) || 1)));
      } else if (!preserveLoadedPages) {
        setQuestionPage(page);
      }
      setQuestionTotal(response.total);
      if (!append) {
        const deepLinkID = consumePendingDeepLink('questions');
        if (deepLinkID) activateQuestion(deepLinkID);
        else if (!preserveLoadedPages && !replaceLoadedPages) activateQuestion(selectListItemID(activeQuestionIDRef.current, items.map((item) => item.id), ''));
        else if (!activeQuestionIDRef.current) activateQuestion(items[0]?.id ?? '');
      }
      return true;
    } catch { return options.signal?.aborted || request !== questionListRequest.current || queryKey !== questionQueryRef.current; }
  }, [activateQuestion, consumePendingDeepLink, questionPage, serverSearch]);

  const loadThreadDetail = useCallback(async (id: string, preserveLoadedMessages = false): Promise<boolean> => {
    const request = ++threadRequest.current;
    threadDetailLoadingRef.current = true;
    setThreadDetailLoading(true);
    setThreadDetailError('');
    try {
      const detail = await qaThreadService.get(id);
      if (request === threadRequest.current && activeQuestionIDRef.current === id) {
        setActiveThread((current) => preserveLoadedMessages && current?.id === detail.id ? {
          ...detail,
          messages: mergeMessagesByID(current.messages, detail.messages),
          messages_page: current.messages_page,
          messages_page_size: current.messages_page_size,
        } : detail);
        threadDetailLoadingRef.current = false;
        setThreadDetailLoading(false);
      }
      return true;
    } catch {
      if (request === threadRequest.current && activeQuestionIDRef.current === id) {
        setActiveThread(null);
        setThreadDetailError('答疑详情加载失败，请稍后重试。');
        threadDetailLoadingRef.current = false;
        setThreadDetailLoading(false);
      }
      return false;
    }
  }, []);

  useEffect(() => {
    if (activeTab !== 'private') {
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
      setConversationDetailError('');
      return;
    }
    setActiveConv(null);
    void loadConversationDetail(activeConvId);
  }, [activeConvId, activeTab, loadConversationDetail]);

  useEffect(() => {
    const throughMessageID = activeConv?.read_through_message_id;
    const cutoffKey = activeConv && throughMessageID ? `${activeConv.id}:${throughMessageID}` : '';
    if (!pageVisible || !conversationDetailVisible || activeTab !== 'private' || !activeConv || !throughMessageID || activeConv.id !== activeConvId || acknowledgedConversationCutoff.current === cutoffKey || acknowledgingConversationCutoff.current === cutoffKey) return;
    const conversationID = activeConv.id;
    acknowledgingConversationCutoff.current = cutoffKey;
    conversationListRequest.current++;
    void conversationService.acknowledgeRead(conversationID, throughMessageID).then(async () => {
      acknowledgedConversationCutoff.current = cutoffKey;
      const loaded = await loadConversations(1, false, false, { refreshLoadedPages: true });
      if (!loaded) setListLoadError('私信列表刷新失败，请稍后重试。');
      refreshMessageCenterSummaryAfterMutation();
    }).catch(() => {
      toast({ type: 'error', title: '私信已显示，但同步已读状态失败' });
    }).finally(() => {
      if (acknowledgingConversationCutoff.current === cutoffKey) acknowledgingConversationCutoff.current = '';
    });
  }, [activeConv, activeConvId, activeTab, conversationDetailVisible, loadConversations, pageVisible, toast]);

  useEffect(() => {
    if (activeTab !== 'notices') {
      noticeRequest.current++;
      return;
    }
    if (!activeNoticeId) {
      noticeRequest.current++;
      setActiveNotice(null);
      setNoticeDetailLoading(false);
      setNoticeDetailError('');
      return;
    }
    setActiveNotice(null);
    void loadNoticeDetail(activeNoticeId);
  }, [activeNoticeId, activeTab, loadNoticeDetail]);

  useEffect(() => {
    if (activeTab !== 'questions') {
      threadRequest.current++;
      threadDetailLoadingRef.current = false;
      setThreadDetailLoading(false);
      return;
    }
    if (!activeQuestionId) {
      threadRequest.current++;
      setActiveThread(null);
      threadDetailLoadingRef.current = false;
      setThreadDetailLoading(false);
      setThreadDetailError('');
      return;
    }
    setActiveThread(null);
    void loadThreadDetail(activeQuestionId);
  }, [activeQuestionId, activeTab, loadThreadDetail]);

  useEffect(() => {
    const throughMessageID = activeThread?.read_through_message_id;
    const cutoffKey = activeThread && throughMessageID ? `${activeThread.id}:${throughMessageID}` : '';
    if (!pageVisible || !threadDetailVisible || activeTab !== 'questions' || !activeThread || !throughMessageID || activeThread.id !== activeQuestionId || acknowledgedThreadCutoff.current === cutoffKey || acknowledgingThreadCutoff.current === cutoffKey) return;
    const threadID = activeThread.id;
    acknowledgingThreadCutoff.current = cutoffKey;
    questionListRequest.current++;
    void qaThreadService.acknowledgeRead(threadID, throughMessageID).then(async () => {
      acknowledgedThreadCutoff.current = cutoffKey;
      const loaded = await loadQuestions(1, false, false, { refreshLoadedPages: true });
      if (!loaded) setListLoadError('答疑列表刷新失败，请稍后重试。');
      refreshMessageCenterSummaryAfterMutation();
    }).catch(() => {
      toast({ type: 'error', title: '答疑已显示，但同步已读状态失败' });
    }).finally(() => {
      if (acknowledgingThreadCutoff.current === cutoffKey) acknowledgingThreadCutoff.current = '';
    });
  }, [activeQuestionId, activeTab, activeThread, loadQuestions, pageVisible, threadDetailVisible, toast]);

  const reloadInitialData = useCallback(async (preserveCurrent = false) => {
    const request = ++reloadRequest.current;
    setLoading(true);
    setLoadError('');
    const refreshOptions: ListLoadOptions = preserveCurrent ? { refreshLoadedPages: true } : {};
    const results = await Promise.all([
      loadContacts(),
      loadConversations(1, false, false, refreshOptions),
      loadNotices(1, false, false, refreshOptions),
      loadQuestions(1, false, false, refreshOptions),
    ]);
    if (request !== reloadRequest.current) return;
    if (results.some((success) => !success)) setLoadError('部分消息中心数据加载失败，请检查网络后重试。');
    else setListLoadError('');
    setLoading(false);
    setInitialLoad(false);
  }, [loadContacts, loadConversations, loadNotices, loadQuestions]);

  // ---- initial load — only shows full-page spinner on first mount
  useEffect(() => {
    if (initialLoadStarted.current) return;
    initialLoadStarted.current = true;
    void reloadInitialData();
  }, [reloadInitialData]);

  useEffect(() => {
    if (initialLoad) return;
    const queryKey = `${activeTab}\u0000${serverSearch}\u0000${noticeStatus}`;
    if (lastListQuery.current === queryKey) return;
    lastListQuery.current = queryKey;
    let active = true;
    const load = async () => {
      let loaded = true;
      if (activeTab === 'private') loaded = await loadConversations();
      if (activeTab === 'notices') loaded = await loadNotices();
      if (activeTab === 'questions') loaded = await loadQuestions();
      if (active) setListLoadError(loaded ? '' : '当前筛选结果加载失败，正在显示上次结果。');
    };
    void load();
    return () => { active = false; };
  }, [activeTab, initialLoad, loadConversations, loadNotices, loadQuestions, noticeStatus, serverSearch]);

  const pollMessageCenter = useCallback(async (signal: AbortSignal) => {
    if (signal.aborted || initialLoad || document.hidden || loadingMoreListRef.current || loadingOlderMessagesRef.current || loadingOlderThreadMessagesRef.current || conversationDetailLoadingRef.current || threadDetailLoadingRef.current) return;
    if (activeTab === 'private') await loadConversations(1, false, true, { signal });
    if (activeTab === 'notices') await loadNotices(1, false, true, { signal });
    if (activeTab === 'questions') await loadQuestions(1, false, true, { signal });
    if (signal.aborted) return;
    if (document.hidden || loadingMoreListRef.current || loadingOlderMessagesRef.current || loadingOlderThreadMessagesRef.current || conversationDetailLoadingRef.current || threadDetailLoadingRef.current) return;
    const currentConversationID = activeConvIDRef.current;
    const currentQuestionID = activeQuestionIDRef.current;
    if (activeTab === 'private' && currentConversationID) {
      const request = ++conversationRequest.current;
      try {
        const detail = await conversationService.get(currentConversationID, undefined, signal);
        if (signal.aborted || document.hidden || request !== conversationRequest.current || activeConvIDRef.current !== currentConversationID) return;
        setActiveConv((current) => current?.id === detail.id ? {
          ...detail,
          messages: mergeMessagesByID(current.messages, detail.messages),
          messages_page: current.messages_page,
          messages_page_size: current.messages_page_size,
        } : current);
      } catch { /* retain the last successfully loaded detail */ }
    }
    if (activeTab === 'questions' && currentQuestionID) {
      const request = ++threadRequest.current;
      try {
        const detail = await qaThreadService.get(currentQuestionID, undefined, signal);
        if (signal.aborted || document.hidden || request !== threadRequest.current || activeQuestionIDRef.current !== currentQuestionID) return;
        setActiveThread((current) => current?.id === detail.id ? {
          ...detail,
          messages: mergeMessagesByID(current.messages, detail.messages),
          messages_page: current.messages_page,
          messages_page_size: current.messages_page_size,
        } : current);
      } catch { /* retain the last successfully loaded detail */ }
    }
  }, [activeTab, initialLoad, loadConversations, loadNotices, loadQuestions]);

  useSerialPolling(pollMessageCenter, 30_000);

  // ---- derived --------------------------------------------------------
  const availableContacts = useMemo(
    () => contacts.filter((c) => !convItems.some((conv) => conv.teacherId === c.id)),
    [contacts, convItems],
  );

  const filteredAvailableContacts = useMemo(
    () => {
      if (!contactSearch.trim()) return availableContacts;
      const kw = contactSearch.trim().toLowerCase();
      return availableContacts.filter((c) =>
        c.id.toLowerCase().includes(kw) ||
        c.display_name.toLowerCase().includes(kw) ||
        c.scope.toLowerCase().includes(kw),
      );
    },
    [availableContacts, contactSearch],
  );

  // Global search — debounced, queries all users
  useEffect(() => {
    const request = ++contactSearchRequest.current;
    const q = contactSearch.trim();
    if (!hasMinimumGlobalSearchCharacters(q)) { setGlobalSearchResults([]); return; }
    const timer = setTimeout(async () => {
      try {
        const { contacts: list } = await conversationService.searchUsers(q);
        if (request === contactSearchRequest.current) {
          setGlobalSearchResults(list.filter((c) => !availableContacts.some((a) => a.id === c.id)));
        }
      } catch {
        if (request === contactSearchRequest.current) setGlobalSearchResults([]);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [contactSearch, availableContacts]);

  const allSearchResults = useMemo(() => {
    const local = filteredAvailableContacts;
    const extra = globalSearchResults.filter((g) => !local.some((l) => l.id === g.id));
    return [...local, ...extra];
  }, [filteredAvailableContacts, globalSearchResults]);

  // ---- actions: conversations -----------------------------------------
  const openConversation = useCallback((id: string) => {
    clearItemDeepLink('private');
    if (!activateConversation(id)) {
      setActiveConv(null);
      void loadConversationDetail(id);
    }
  }, [activateConversation, clearItemDeepLink, loadConversationDetail]);

  const sendPrivateMessage = useCallback(async (event?: React.FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    if (!activeConv || activeConv.id !== activeConvId || !messageDraft.trim() || sendingMsg) return;
    const conversationID = activeConv.id;
    const submittedDraft = messageDraft;
    conversationRequest.current++;
    conversationListRequest.current++;
    setSendingMsg(true);
    try {
      await conversationService.sendMessage(conversationID, submittedDraft.trim());
      setMessageDrafts((current) => {
        if ((current[conversationID] ?? '') !== submittedDraft) return current;
        const next = { ...current };
        delete next[conversationID];
        return next;
      });
      if (activeConvIDRef.current === conversationID) await loadConversationDetail(conversationID, true);
      await loadConversations(1, false, false, { refreshLoadedPages: true });
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '发送私信失败，请稍后重试' });
    }
    finally { setSendingMsg(false); }
  }, [activeConv, activeConvId, messageDraft, sendingMsg, loadConversationDetail, loadConversations, toast]);

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
      setActiveConv((current) => current?.id === detail.id ? {
        ...detail,
        read_through_message_id: current.read_through_message_id,
        messages,
        messages_total: messagesTotal,
        messages_page: nextPage,
      } : current);
    } catch {
      if (request === conversationRequest.current && activeConvIDRef.current === conversationID) toast({ type: 'error', title: '加载更早私信失败，请稍后重试' });
    } finally {
      if (activeConvIDRef.current === conversationID) {
        loadingOlderMessagesRef.current = false;
        setLoadingOlderMessages(false);
      }
    }
  }, [activeConv, toast]);

  const loadMoreConversations = useCallback(async () => {
    if (loadingMoreListRef.current || convItems.length >= conversationTotal) return;
    loadingMoreListRef.current = true;
    setLoadingMoreList('conversations');
    try {
      const loaded = await loadConversations(conversationPage + 1, true);
      if (!loaded) toast({ type: 'error', title: '加载更多私信失败，请稍后重试' });
    } finally {
      loadingMoreListRef.current = false;
      setLoadingMoreList('');
    }
  }, [convItems.length, conversationTotal, loadConversations, conversationPage, toast]);

  const loadMoreNotices = useCallback(async () => {
    if (loadingMoreListRef.current || notices.length >= noticeTotal) return;
    loadingMoreListRef.current = true;
    setLoadingMoreList('notices');
    try {
      const loaded = await loadNotices(noticePage + 1, true);
      if (!loaded) toast({ type: 'error', title: '加载更多通知失败，请稍后重试' });
    } finally {
      loadingMoreListRef.current = false;
      setLoadingMoreList('');
    }
  }, [notices.length, noticeTotal, loadNotices, noticePage, toast]);

  const loadMoreQuestions = useCallback(async () => {
    if (loadingMoreListRef.current || questions.length >= questionTotal) return;
    loadingMoreListRef.current = true;
    setLoadingMoreList('questions');
    try {
      const loaded = await loadQuestions(questionPage + 1, true);
      if (!loaded) toast({ type: 'error', title: '加载更多提问失败，请稍后重试' });
    } finally {
      loadingMoreListRef.current = false;
      setLoadingMoreList('');
    }
  }, [questions.length, questionTotal, loadQuestions, questionPage, toast]);

  const createConversation = useCallback(async () => {
    if (!selectedTeacherId || creatingConv) return;
    conversationListRequest.current++;
    setCreatingConv(true);
    try {
      const teacher = contacts.find((c) => c.id === selectedTeacherId);
      const detail = await conversationService.create({
        target_id: selectedTeacherId,
        subject: teacher?.scope ?? '',
        initial_message: newConvDraft.trim(),
      });
      setNewConvDraft('');
      setNewConvOpen(false);
      await loadConversations();
      activateConversation(detail.id);
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '创建私信失败，请稍后重试' });
    }
    finally { setCreatingConv(false); }
  }, [activateConversation, selectedTeacherId, newConvDraft, creatingConv, loadConversations, contacts, toast]);

  const archiveConversation = useCallback(async (id: string) => {
    conversationListRequest.current++;
    try {
      await conversationService.archive(id);
      setMessageDrafts((current) => {
        if (!(id in current)) return current;
        const nextDrafts = { ...current };
        delete nextDrafts[id];
        return nextDrafts;
      });
      clearItemDeepLink('private');
      await loadConversations();
      const next = convItems.find((c) => c.id !== id && !c.archived);
      if (next) {
        activateConversation(next.id);
      } else {
        activateConversation('');
      }
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '归档私信失败，请稍后重试' });
    }
  }, [activateConversation, clearItemDeepLink, loadConversations, convItems, toast]);

  // ---- actions: notices -----------------------------------------------
  const confirmNotice = useCallback(async (id: string) => {
    if (confirming === id) return;
    noticeRequest.current++;
    noticeListRequest.current++;
    setConfirming(id);
    try {
      await noticeService.confirm(id);
      setActiveNotice((current) => current?.id === id ? { ...current, confirmed: true } : current);
      const loaded = await loadNotices(1, false, false, { refreshLoadedPages: true });
      if (!loaded) setListLoadError('通知列表刷新失败，请稍后重试。');
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '确认通知失败，请稍后重试' });
    }
    finally { setConfirming(''); }
  }, [confirming, loadNotices, toast]);

  // ---- actions: questions ---------------------------------------------
  const createQuestion = useCallback(async () => {
    if (!questionDraft.trim() || submittingQ) return;
    questionListRequest.current++;
    setSubmittingQ(true);
    try {
      await qaThreadService.create({ teacher_id: selectedQTeacherId, content: questionDraft.trim() });
      setQuestionDraft('');
      await loadQuestions(1, false, false, { refreshLoadedPages: true });
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '提交提问失败，请稍后重试' });
    }
    finally { setSubmittingQ(false); }
  }, [questionDraft, selectedQTeacherId, submittingQ, loadQuestions, toast]);

  const loadMistakesForImport = useCallback(async () => {
    setLoadingMistakes(true);
    try {
      const res = await fetchMistakes({ page: 1, pageSize: 50 });
      setMistakes(res.items);
    } catch {
      toast({ type: 'error', title: '错题列表加载失败，请稍后重试' });
    }
    finally { setLoadingMistakes(false); }
  }, [toast]);

  const importQuestion = useCallback(async () => {
    if (importing) return;
    const selected = mistakes.find((m) => m.id === selectedMistakeId);
    if (!selected) return;
    questionListRequest.current++;
    setImporting(true);
    try {
      const fullContext = [
        `【原题】${selected.exercise.title}`,
        `【题目内容】${selected.exercise.content}`,
        `【我的答案】${selected.attempt.studentAnswer}`,
        `【正确答案】${selected.attempt.correctAnswer}`,
        selected.diagnosis.explanation ? `【错因分析】${selected.diagnosis.explanation}` : '',
        selected.diagnosis.suggestion ? `【建议】${selected.diagnosis.suggestion}` : '',
      ].filter(Boolean).join('\n\n');
      const question = importQuestionText.trim();
      const content = question
        ? `${question}\n\n---\n\n${fullContext}`
        : fullContext;
      await qaThreadService.importQuestion({
        teacher_id: importTeacherId,
        source: '错题本',
        content,
      });
      setSelectedMistakeId('');
      setImportQuestionText('');
      setImportOpen(false);
      await loadQuestions(1, false, false, { refreshLoadedPages: true });
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '导入提问失败，请稍后重试' });
    }
    finally { setImporting(false); }
  }, [importing, mistakes, selectedMistakeId, importTeacherId, importQuestionText, loadQuestions, toast]);

  const createFollowUp = useCallback(async () => {
    if (!followUpDraft.trim() || !activeThread || activeThread.id !== activeQuestionId || sendingFollowUp) return;
    const threadID = activeThread.id;
    const submittedDraft = followUpDraft;
    threadRequest.current++;
    questionListRequest.current++;
    setSendingFollowUp(true);
    try {
      await qaThreadService.sendMessage(threadID, submittedDraft.trim());
      setFollowUpDrafts((current) => {
        if ((current[threadID] ?? '') !== submittedDraft) return current;
        const next = { ...current };
        delete next[threadID];
        return next;
      });
      if (activeQuestionIDRef.current === threadID) await loadThreadDetail(threadID, true);
      await loadQuestions(1, false, false, { refreshLoadedPages: true });
      refreshMessageCenterSummaryAfterMutation();
    } catch {
      toast({ type: 'error', title: '发送追问失败，请稍后重试' });
    }
    finally { setSendingFollowUp(false); }
  }, [followUpDraft, activeQuestionId, activeThread, sendingFollowUp, loadThreadDetail, loadQuestions, toast]);

  const loadOlderThreadMessages = useCallback(async () => {
    if (!activeThread || activeThread.id !== activeQuestionIDRef.current || loadingOlderThreadMessagesRef.current || activeThread.messages.length >= activeThread.messages_total) return;
    const threadID = activeThread.id;
    const request = ++threadRequest.current;
    loadingOlderThreadMessagesRef.current = true;
    setLoadingOlderThreadMessages(true);
    try {
      const nextPage = activeThread.messages_page + 1;
      const detail = await qaThreadService.get(threadID, { messages_page: nextPage, messages_page_size: activeThread.messages_page_size });
      if (request !== threadRequest.current) return;
      let messages = mergeMessagesByID(activeThread.messages, detail.messages);
      let messagesTotal = detail.messages_total;
      const headDetail = await qaThreadService.get(threadID, { messages_page: 1, messages_page_size: activeThread.messages_page_size });
      if (request !== threadRequest.current) return;
      const currentHeadID = activeThread.messages.at(-1)?.id ?? '';
      const serverHeadID = headDetail.messages.at(-1)?.id ?? '';
      const headShifted = currentHeadID !== serverHeadID
        || activeThread.messages_total !== headDetail.messages_total
        || detail.messages_total !== headDetail.messages_total;
      const loadedWindowDrifted = activeThread.messages.length < headDetail.messages_total
        && activeThread.messages.length !== Math.min(
          headDetail.messages_total,
          activeThread.messages_page * activeThread.messages_page_size,
        );
      if (headShifted || loadedWindowDrifted || messages.length < Math.min(messagesTotal, nextPage * activeThread.messages_page_size)) {
        const stableWindow = await fetchStableOffsetMessageWindow(nextPage, activeThread.messages_page_size, async (messagesPage) => {
          const pageDetail = await qaThreadService.get(threadID, { messages_page: messagesPage, messages_page_size: activeThread.messages_page_size });
          return { messages: pageDetail.messages, messages_total: pageDetail.messages_total };
        });
        if (request !== threadRequest.current) return;
        messages = stableWindow.messages;
        messagesTotal = stableWindow.total;
      }
      setActiveThread((current) => current?.id === detail.id ? {
        ...detail,
        read_through_message_id: current.read_through_message_id,
        messages,
        messages_total: messagesTotal,
        messages_page: nextPage,
      } : current);
    } catch {
      if (request === threadRequest.current && activeQuestionIDRef.current === threadID) toast({ type: 'error', title: '加载更早答疑消息失败，请稍后重试' });
    } finally {
      if (activeQuestionIDRef.current === threadID) {
        loadingOlderThreadMessagesRef.current = false;
        setLoadingOlderThreadMessages(false);
      }
    }
  }, [activeThread, toast]);

  const selectQuestion = useCallback((id: string) => {
    clearItemDeepLink('questions');
    if (!activateQuestion(id)) {
      setActiveThread(null);
      void loadThreadDetail(id);
    }
  }, [activateQuestion, clearItemDeepLink, loadThreadDetail]);

  useEffect(() => {
    if (handledLocationKey.current === location.key) return;
    handledLocationKey.current = location.key;
    const tab = parseStudentTab(searchParams.get('tab'));
    const id = searchParams.get('id') ?? '';
    setActiveTab(tab);
    if (!id) {
      pendingDeepLink.current = null;
      return;
    }
    pendingDeepLink.current = { tab, id };
    if (tab === 'private' && !activateConversation(id)) {
      setActiveConv(null);
      void loadConversationDetail(id);
    }
    if (tab === 'notices') {
      if (!activateNotice(id)) {
        setActiveNotice(null);
        void loadNoticeDetail(id);
      }
    }
    if (tab === 'questions' && !activateQuestion(id)) {
      setActiveThread(null);
      void loadThreadDetail(id);
    }
  }, [activateConversation, activateNotice, activateQuestion, loadConversationDetail, loadNoticeDetail, loadThreadDetail, location.key, searchParams]);

  const retryActiveList = useCallback(async () => {
    let loaded = true;
    if (activeTab === 'private') loaded = await loadConversations(1, false, false, { refreshLoadedPages: true });
    if (activeTab === 'notices') loaded = await loadNotices(1, false, false, { refreshLoadedPages: true });
    if (activeTab === 'questions') loaded = await loadQuestions(1, false, false, { refreshLoadedPages: true });
    setListLoadError(loaded ? '' : '当前列表加载失败，请稍后重试。');
  }, [activeTab, loadConversations, loadNotices, loadQuestions]);

  // ---- render ---------------------------------------------------------
  if (initialLoad && loading) {
    return (
      <MainLayout>
        <div className="container mx-auto flex max-w-7xl items-center justify-center px-4 py-24">
          <Loader2 className="h-8 w-8 animate-spin text-primary-500" />
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <div className="container mx-auto max-w-7xl px-4 py-5 sm:py-8">
        <div className="mb-5 flex flex-col gap-4 rounded-2xl border border-surface-200/80 bg-white/85 px-5 py-4 shadow-sm backdrop-blur dark:border-surface-700 dark:bg-surface-900/85 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">消息中心</h1>
            <p className="mt-1 text-sm text-surface-500 dark:text-surface-400">
              管理和老师的私信、班级通知与答疑线程
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="relative w-full sm:w-72">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
              <Input
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder={activeTab === 'private' ? '搜索老师、班级…' : activeTab === 'notices' ? '搜索通知标题、内容…' : '搜索问题、知识点…'}
                className="pl-10"
              />
            </div>
          </div>
        </div>

        {loadError && <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200"><span>{loadError}</span><Button variant="outline" size="sm" onClick={() => void reloadInitialData(true)} disabled={loading}>重新加载</Button></div>}
        {listLoadError && <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200"><span>{listLoadError}</span><Button variant="outline" size="sm" onClick={() => void retryActiveList()}>重试</Button></div>}
        {summaryError && <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200"><span>{summary ? '消息计数刷新失败，当前显示上次结果。' : '消息计数加载失败，页签角标暂不可用。'}</span><Button variant="outline" size="sm" onClick={() => void refreshSummary().catch(() => undefined)} disabled={summaryRefreshing}>重试</Button></div>}

        <Tabs
          defaultValue="private"
          value={activeTab}
          keepMounted={false}
          onValueChange={(value) => {
            setActiveTab(value);
            setSearchTerm('');
            setServerSearch('');
            setSearchParams({ tab: value });
          }}
        >
          <TabsList className="mb-5 h-auto rounded-xl border border-surface-200 bg-white p-1.5 shadow-sm dark:border-surface-700 dark:bg-surface-900">
            <TabsTrigger value="private" className="rounded-lg px-4 py-2">
              <MessageSquare className="mr-2 h-4 w-4" />
              私信<TabCount count={tabCounts.private} />
            </TabsTrigger>
            <TabsTrigger value="notices" className="rounded-lg px-4 py-2">
              <Bell className="mr-2 h-4 w-4" />
              通知<TabCount count={tabCounts.notices} />
            </TabsTrigger>
            <TabsTrigger value="questions" className="rounded-lg px-4 py-2">
              <HelpCircle className="mr-2 h-4 w-4" />
              答疑<TabCount count={tabCounts.questions} />
            </TabsTrigger>
          </TabsList>

          {/* ============================================================ PRIVATE */}
          <TabsContent value="private" className="mt-0">
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[300px_1fr]">
              <Card className={cn('overflow-hidden rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700', activeConvId ? 'hidden lg:block' : '')}>
                <CardContent className="p-0">
                  <div className="border-b border-surface-100 p-3 dark:border-surface-800">
                    <Button
                      className="w-full"
                      onClick={() => {
                        setContactSearch('');
                        setSelectedTeacherId(availableContacts[0]?.id ?? '');
                        setNewConvDraft('');
                        setNewConvOpen(true);
                      }}
                    >
                      <Plus className="mr-2 h-4 w-4" />
                      新建对话
                    </Button>
                  </div>
                  {convItems.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      onClick={() => openConversation(c.id)}
                      className={cn(
                        'w-full border-b border-surface-100 px-3 py-2 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeConvId === c.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}
                    >
                      <div className="flex items-center gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-full border border-surface-300 bg-white text-surface-800 shadow-sm dark:border-surface-600 dark:bg-surface-900 dark:text-surface-100"><UserRound className="h-5 w-5" /></span><div className="min-w-0 flex-1"><div className="truncate text-sm font-medium text-surface-900 dark:text-surface-100">{c.teacherName}</div><div className="mt-0.5 truncate text-xs text-surface-500 dark:text-surface-400">{c.lastMessage || c.scope}</div></div><div className="flex shrink-0 flex-col items-end gap-1 text-xs"><span className="text-surface-400">{c.lastTime}</span><span className={c.unread > 0 ? 'text-red-500' : 'text-emerald-600 dark:text-emerald-400'}>{c.unread > 0 ? '未读' : '已回复'}</span></div></div>
                    </button>
                  ))}
                  {convItems.length < conversationTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreConversations} disabled={loadingMoreList !== ''}>{loadingMoreList === 'conversations' ? '加载中…' : '加载更多对话'}</Button>}
                </CardContent>
              </Card>

              <Card className={cn('overflow-hidden rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700', !activeConvId ? 'hidden lg:block' : '')}>
                <CardContent className="flex h-full flex-col p-0">
                  {conversationDetailLoading ? (
                    <div className="flex min-h-64 flex-col items-center justify-center gap-3 p-6">
                      <Loader2 className="h-6 w-6 animate-spin text-primary-500" />
                      <Button variant="ghost" size="sm" className="lg:hidden" onClick={showConversationList}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
                    </div>
                  ) : conversationDetailError ? (
                    <div className="flex min-h-64 flex-col items-center justify-center gap-3 p-6 text-center text-sm text-surface-500 dark:text-surface-400">
                      <span>{conversationDetailError}</span>
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => activeConvId && void loadConversationDetail(activeConvId)}>重新加载</Button>
                        <Button variant="ghost" size="sm" className="lg:hidden" onClick={showConversationList}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
                      </div>
                    </div>
                  ) : activeConv && activeConv.id === activeConvId ? (
                    <>
                      <div ref={conversationDetailRef} className="border-b border-surface-100 p-3 sm:p-4 dark:border-surface-800">
                        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                          <div>
                            <Button variant="ghost" size="sm" className="mb-2 -ml-2 lg:hidden" onClick={showConversationList}><ArrowLeft className="mr-1 h-4 w-4" />返回列表</Button>
                            <div className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                              {activeConv.teacher_name}
                            </div>
                            <div className="text-sm text-surface-500 dark:text-surface-400">{activeConv.scope}</div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => archiveConversation(activeConv.id)}>
                              <Archive className="mr-2 h-4 w-4" />归档
                            </Button>
                          </div>
                        </div>
                      </div>
                      <div className="flex-1 space-y-4 overflow-y-auto p-5">
                        {activeConv.messages.length < activeConv.messages_total && <Button variant="outline" size="sm" className="w-full" onClick={loadOlderConversationMessages} disabled={loadingOlderMessages}>{loadingOlderMessages ? '加载中…' : '加载更早消息'}</Button>}
                        {activeConv.messages.map((msg) => (
                          <div key={msg.id} className="flex w-full">
                            <div className={cn('max-w-[80%]', msg.from === 'student' ? 'ml-auto text-right' : 'mr-auto')}>
                              <div className={cn(
                                'inline-block rounded-lg px-4 py-3 text-sm',
                                msg.from === 'student'
                                  ? 'bg-primary-600 text-white'
                                  : 'bg-surface-100 text-surface-800 dark:bg-surface-800 dark:text-surface-100',
                              )}>
                                {msg.text}
                              </div>
                              <div className={cn('mt-1 flex gap-2 text-xs text-surface-400', msg.from === 'student' ? 'justify-end' : 'justify-start')}>
                                <span>{formatRelativeTime(msg.time)}</span>
                                {msg.from === 'student' && <span>{msg.read_by_recipient ? '老师已读' : '老师未读'}</span>}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                      <div className="border-t border-surface-100 p-4 dark:border-surface-800">
                        <form className="flex gap-2" onSubmit={sendPrivateMessage}>
                          <Input
                            value={messageDraft}
                            onChange={(e) => {
                              const conversationID = activeConvIDRef.current;
                              if (!conversationID) return;
                              const value = e.target.value;
                              setMessageDrafts((current) => ({ ...current, [conversationID]: value }));
                            }}
                            placeholder="输入给老师的消息"
                          />
                          <Button type="submit" size="icon" aria-label="发送私信" disabled={sendingMsg}>
                            {sendingMsg ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                          </Button>
                        </form>
                      </div>
                    </>
                  ) : (
                    <div className="flex h-full items-center justify-center p-8 text-sm text-surface-500 dark:text-surface-400">
                      暂无可显示的私信对话
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          {/* ============================================================ NOTICES */}
          <TabsContent value="notices" className="mt-0">
            <div className="mb-4 flex flex-wrap gap-2">
              {noticeStatuses.map((s) => (
                <Button key={s} variant={noticeStatus === s ? 'primary' : 'outline'} size="sm" onClick={() => setNoticeStatus(s)}>{s}</Button>
              ))}
            </div>
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[360px_1fr]">
              <Card className="overflow-hidden rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700">
                <CardContent className="p-0">
                  {notices.map((n) => (
                    <button
                      key={n.id}
                      type="button"
                      onClick={() => {
                        clearItemDeepLink('notices');
                        if (!activateNotice(n.id)) {
                          setActiveNotice(null);
                          void loadNoticeDetail(n.id);
                        }
                      }}
                      className={cn(
                        'w-full border-b border-surface-100 px-3 py-2 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeNoticeId === n.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}
                    >
                      <div className="flex items-center gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-full border border-surface-300 bg-white text-surface-800 dark:border-surface-600 dark:bg-surface-900 dark:text-surface-100"><Bell className="h-5 w-5" /></span><div className="min-w-0 flex-1"><div className="truncate text-sm font-medium text-surface-900 dark:text-surface-100">{n.title}</div><div className="mt-0.5 truncate text-xs text-surface-500 dark:text-surface-400">通知 · {n.className}</div></div><div className="flex shrink-0 flex-col items-end gap-1 text-xs"><span className="text-surface-400">{n.publishedAt}</span><span className={n.confirmed ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}>{n.confirmed ? '已确认' : '待确认'}</span></div></div>
                    </button>
                  ))}
                  {notices.length < noticeTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreNotices} disabled={loadingMoreList !== ''}>{loadingMoreList === 'notices' ? '加载中…' : '加载更多通知'}</Button>}
                </CardContent>
              </Card>

              <Card className="rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700">
                <CardContent className="p-6">
                  {noticeDetailLoading ? (
                    <div className="flex min-h-48 items-center justify-center">
                      <Loader2 className="h-6 w-6 animate-spin text-primary-500" />
                    </div>
                  ) : noticeDetailError ? (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-center text-sm text-surface-500 dark:text-surface-400">
                      <span>{noticeDetailError}</span>
                      <Button variant="outline" size="sm" onClick={() => activeNoticeId && loadNoticeDetail(activeNoticeId)}>重新加载</Button>
                    </div>
                  ) : activeNotice ? (
                    <div className="space-y-5">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <div className="text-sm text-surface-500 dark:text-surface-400">{activeNotice.className} · {activeNotice.publishedAt}</div>
                          <h2 className="mt-2 text-xl font-semibold text-surface-900 dark:text-surface-100">{activeNotice.title}</h2>
                        </div>
                        <Badge variant={activeNotice.confirmed ? 'success' : 'warning'}>{activeNotice.confirmed ? '已确认收到' : '待确认'}</Badge>
                      </div>
                      <p className="leading-7 text-surface-700 dark:text-surface-300">{activeNotice.body}</p>
                      {activeNotice.attachments.length > 0 && (
                        <div className="space-y-2">
                          {activeNotice.attachments.map((a) => (
                            <div key={a} className="flex items-center gap-2 rounded-md border border-surface-200 p-3 text-sm dark:border-surface-700">
                              <Paperclip className="h-4 w-4 text-surface-400" />{a}
                            </div>
                          ))}
                        </div>
                      )}
                      <Button onClick={() => confirmNotice(activeNotice.id)} disabled={activeNotice.confirmed || confirming === activeNotice.id}>
                        {confirming === activeNotice.id
                          ? <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          : <CheckCircle2 className="mr-2 h-4 w-4" />}
                        {activeNotice.confirmed ? '已确认收到' : '确认收到'}
                      </Button>
                    </div>
                  ) : (
                    <div className="flex min-h-48 items-center justify-center text-sm text-surface-500 dark:text-surface-400">暂无通知详情</div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          {/* ============================================================ QUESTIONS */}
          <TabsContent value="questions" className="mt-0">
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[360px_1fr]">
              <Card className="overflow-hidden rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700">
                <CardContent className="space-y-4 p-4">
                  <div className="space-y-2">
                    <select
                      value={selectedQTeacherId}
                      onChange={(e) => setSelectedQTeacherId(e.target.value)}
                      className="h-10 w-full rounded-md border border-surface-200 bg-white px-3 text-sm dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                    >
                      {contacts.map((c) => <option key={c.id} value={c.id}>{c.display_name} · {c.scope}</option>)}
                    </select>
                    <textarea
                      value={questionDraft}
                      onChange={(e) => setQuestionDraft(e.target.value)}
                      placeholder="新建一个要问老师的问题"
                      className="min-h-24 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                    />
                    <Button className="w-full" onClick={createQuestion} disabled={submittingQ}>
                      {submittingQ ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <HelpCircle className="mr-2 h-4 w-4" />}
                      提交问题
                    </Button>
                    <Button className="w-full" variant="outline" onClick={() => { setImportOpen(true); loadMistakesForImport(); }}>
                      <Import className="mr-2 h-4 w-4" />导入问题
                    </Button>
                  </div>
                  <div className="divide-y divide-surface-100 dark:divide-surface-800">
                    {questions.map((q) => (
                      <button
                        key={q.id}
                        type="button"
                        onClick={() => selectQuestion(q.id)}
                        className={cn(
                          'w-full px-3 py-2 text-left hover:bg-surface-50 dark:hover:bg-surface-800',
                          activeQuestionId === q.id && 'bg-primary-50 dark:bg-primary-950/30',
                        )}
                      >
                      <div className="flex items-center gap-3"><span className="relative grid h-10 w-10 shrink-0 place-items-center rounded-full border border-surface-300 bg-white text-surface-800 dark:border-surface-600 dark:bg-surface-900 dark:text-surface-100"><HelpCircle className="h-5 w-5" />{q.unread && <span className="absolute right-0 top-0 h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-white dark:ring-surface-900" aria-label="有新回复" />}</span><div className="min-w-0 flex-1"><div className="truncate text-sm font-medium text-surface-900 dark:text-surface-100">{q.title}</div><div className="mt-0.5 truncate text-xs text-surface-500 dark:text-surface-400">{q.teacher_name} · {q.source}</div></div><div className="flex shrink-0 flex-col items-end gap-1 text-xs"><span className="text-surface-400">{formatRelativeTime(q.last_update)}</span><span className={q.status === '待回复' ? 'text-red-500' : q.status === '已回复' || q.status === '已解决' ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}>{q.status}</span></div></div>
                      </button>
                    ))}
                    {questions.length < questionTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreQuestions} disabled={loadingMoreList !== ''}>{loadingMoreList === 'questions' ? '加载中…' : '加载更多提问'}</Button>}
                  </div>
                </CardContent>
              </Card>

              <Card className="rounded-2xl border-surface-200/80 shadow-sm dark:border-surface-700">
                <CardContent className="p-6">
                  {threadDetailLoading ? (
                    <div className="flex min-h-48 items-center justify-center">
                      <Loader2 className="h-6 w-6 animate-spin text-primary-500" />
                    </div>
                  ) : threadDetailError ? (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-center text-sm text-surface-500 dark:text-surface-400">
                      <span>{threadDetailError}</span>
                      <Button variant="outline" size="sm" onClick={() => activeQuestionId && void loadThreadDetail(activeQuestionId)}>重新加载</Button>
                    </div>
                  ) : activeThread && activeThread.id === activeQuestionId ? (
                    <div className="space-y-5">
                      <div ref={threadDetailRef} className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <div className="text-sm text-surface-500 dark:text-surface-400">
                            提问给：{activeThread.teacher_name} · 来源：{activeThread.source}
                          </div>
                          <h2 className="mt-2 text-xl font-semibold text-surface-900 dark:text-surface-100">{activeThread.title}</h2>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={statusVariant[activeThread.status as keyof typeof statusVariant] ?? 'secondary'}>{activeThread.status}</Badge>
                        </div>
                      </div>
                      {activeThread.source !== '消息中心' && (
                        <div className="rounded-md bg-surface-50 p-4 text-sm leading-6 text-surface-700 dark:bg-surface-800 dark:text-surface-300">
                          {activeThread.context}
                        </div>
                      )}
                      <div className="space-y-3">
                        {activeThread.messages.length < activeThread.messages_total && <Button variant="outline" size="sm" className="w-full" onClick={loadOlderThreadMessages} disabled={loadingOlderThreadMessages}>{loadingOlderThreadMessages ? '加载中…' : '加载更早消息'}</Button>}
                        {activeThread.messages.map((msg) => (
                          <div key={msg.id} className={cn('rounded-md border p-3', msg.from === 'student' ? 'border-primary-200 bg-primary-50/30 dark:border-primary-800 dark:bg-primary-950/20' : 'border-surface-200 dark:border-surface-700')}>
                            <div className="mb-1 flex items-center gap-2">
                              <span className={cn('text-xs font-medium', msg.from === 'student' ? 'text-primary-600 dark:text-primary-400' : 'text-emerald-600 dark:text-emerald-400')}>
                                {msg.from === 'student' ? '我' : activeThread.teacher_name}
                              </span>
                            </div>
                            <div className="text-sm text-surface-700 dark:text-surface-300">{msg.text}</div>
                            <div className="mt-2 text-xs text-surface-400">{formatRelativeTime(msg.time)}</div>
                          </div>
                        ))}
                      </div>
                      <div className="space-y-2 border-t border-surface-100 pt-4 dark:border-surface-800">
                        <textarea
                          value={followUpDraft}
                          onChange={(e) => {
                            const threadID = activeQuestionIDRef.current;
                            if (!threadID) return;
                            const value = e.target.value;
                            setFollowUpDrafts((current) => ({ ...current, [threadID]: value }));
                          }}
                          placeholder="继续追问这个问题"
                          className="min-h-24 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                        />
                        <div className="flex justify-end">
                          <Button onClick={createFollowUp} disabled={sendingFollowUp}>
                            {sendingFollowUp ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Send className="mr-2 h-4 w-4" />}
                            追问
                          </Button>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="flex h-full items-center justify-center p-8 text-sm text-surface-500 dark:text-surface-400">
                      暂无提问详情
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>

        {/* Import modal — browse mistakes */}
        <Modal isOpen={importOpen} onClose={() => { setImportOpen(false); setSelectedMistakeId(''); setImportQuestionText(''); }} title="从错题本导入" className="max-w-xl">
          <div className="space-y-4">
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">选择老师</label>
            <select value={importTeacherId} onChange={(e) => setImportTeacherId(e.target.value)}
              className="h-10 w-full rounded-md border border-surface-200 bg-white px-3 text-sm dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100">
              {contacts.map((c) => <option key={c.id} value={c.id}>{c.display_name} · {c.scope}</option>)}
            </select>

            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">选择错题</label>
            {loadingMistakes ? (
              <div className="flex justify-center py-8"><Loader2 className="h-6 w-6 animate-spin text-primary-500" /></div>
            ) : mistakes.length === 0 ? (
              <div className="rounded-md border border-surface-200 p-8 text-center text-sm text-surface-500 dark:border-surface-700 dark:text-surface-400">
                暂无错题记录，完成练习后错题会自动收集到这里
              </div>
            ) : (
              <div className="max-h-80 space-y-1 overflow-y-auto rounded-md border border-surface-200 dark:border-surface-700">
                {mistakes.map((m) => (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => setSelectedMistakeId(m.id)}
                    className={cn(
                      'w-full px-4 py-3 text-left transition-colors hover:bg-surface-50 dark:hover:bg-surface-800',
                      selectedMistakeId === m.id
                        ? 'bg-primary-50 ring-1 ring-inset ring-primary-200 dark:bg-primary-950/30 dark:ring-primary-800'
                        : 'border-b border-surface-100 last:border-b-0 dark:border-surface-800',
                    )}
                  >
                    <div className="text-sm font-medium text-surface-900 dark:text-surface-100">
                      {m.exercise.title}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-surface-500 dark:text-surface-400">
                      {m.exercise.knowledgePoints.slice(0, 3).map((kp) => (
                        <span key={kp} className="rounded bg-surface-100 px-1.5 py-0.5 dark:bg-surface-800">{kp}</span>
                      ))}
                      {m.diagnosis.errorType && (
                        <span className="text-red-500">{m.diagnosis.errorType}</span>
                      )}
                      <span>错误 {m.errorCount} 次</span>
                    </div>
                  </button>
                ))}
              </div>
            )}

            {selectedMistakeId && (
              <div className="space-y-1">
                <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">
                  你的疑问（可选）
                </label>
                <textarea
                  value={importQuestionText}
                  onChange={(e) => setImportQuestionText(e.target.value)}
                  placeholder="例如：我不明白为什么这里要用洛必达法则，什么时候该用？"
                  className="min-h-20 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                />
              </div>
            )}

            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => { setImportOpen(false); setSelectedMistakeId(''); setImportQuestionText(''); }}>取消</Button>
              <Button onClick={importQuestion} disabled={importing || !selectedMistakeId}>
                {importing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}导入选中
              </Button>
            </div>
          </div>
        </Modal>

        {/* New conversation modal */}
        <Modal isOpen={newConvOpen} onClose={() => { setNewConvOpen(false); setContactSearch(''); setNewConvDraft(''); }} title="新建私信对话" className="max-w-lg">
          <div className="space-y-4">
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">选择联系人</label>
            <Input value={contactSearch} onChange={(e) => {
              setContactSearch(e.target.value);
              if (selectedTeacherId) {
                setSelectedTeacherId('');
                setNewConvDraft('');
              }
            }}
              placeholder="搜索老师姓名或 ID…" />
            {allSearchResults.length > 0 ? (
              <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-md border border-surface-200 dark:border-surface-700">
                {allSearchResults.map((c) => (
                  <button key={c.id} type="button"
                    onClick={() => {
                      if (selectedTeacherId && selectedTeacherId !== c.id) setNewConvDraft('');
                      setSelectedTeacherId(c.id);
                      setContactSearch('');
                      setGlobalSearchResults([]);
                    }}
                    className={cn('w-full px-4 py-2.5 text-left text-sm hover:bg-surface-50 dark:hover:bg-surface-800',
                      selectedTeacherId === c.id && 'bg-primary-50 ring-1 ring-inset ring-primary-200 dark:bg-primary-950/30 dark:ring-primary-800')}>
                    <div className="font-medium text-surface-900 dark:text-surface-100">{c.display_name}</div>
                    <div className="flex items-center justify-between text-xs text-surface-500 dark:text-surface-400">
                      <span>{c.scope || '全校'}</span>
                      <span className="font-mono text-surface-400">{c.id}</span>
                    </div>
                  </button>
                ))}
              </div>
            ) : contactSearch.trim() ? (
              <p className="text-sm text-surface-500 dark:text-surface-400">未找到匹配的教师。</p>
            ) : availableContacts.length === 0 ? (
              <p className="text-sm text-surface-500 dark:text-surface-400">输入教师姓名或 ID 搜索并建立对话。</p>
            ) : null}
            <textarea value={newConvDraft} onChange={(e) => setNewConvDraft(e.target.value)}
              placeholder="可以先写一句要发给老师的消息"
              className="min-h-28 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
            />
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => { setNewConvOpen(false); setContactSearch(''); setNewConvDraft(''); }}>取消</Button>
              <Button onClick={createConversation} disabled={!selectedTeacherId || creatingConv}>
                {creatingConv ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}创建对话
              </Button>
            </div>
          </div>
        </Modal>
      </div>
    </MainLayout>
  );
};
