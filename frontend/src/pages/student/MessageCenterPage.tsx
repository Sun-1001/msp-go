import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  Trash2,
} from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { formatRelativeTime } from '@/libs/utils/dateFormat';
import {
  conversationService,
  type ConversationItem,
  type ConversationDetail,
  type Contact,
} from '@/modules/message-center/services/conversationService';
import {
  noticeService,
  type StudentNoticeItem,
} from '@/modules/message-center/services/noticeService';
import {
  qaThreadService,
  type StudentThreadItem,
  type ThreadDetail,
} from '@/modules/message-center/services/qaThreadService';
import {
  fetchMistakes,
  type MistakeRecord,
} from '@/modules/mistake/services/mistakeService';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function matchKeywords(haystack: string, search: string): boolean {
  if (!search.trim()) return true;
  const keywords = search.trim().toLowerCase().split(/\s+/);
  const lower = haystack.toLowerCase();
  return keywords.every((kw) => lower.includes(kw));
}

const statusVariant = {
  '待回复': 'warning',
  '已回复': 'default',
  '已解决': 'success',
} as const;
const noticeStatuses = ['全部', '待确认', '已确认'];

const renderTabCount = (count: number) => {
  if (count <= 0) return null;
  return (
    <span className="ml-2 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-xs font-semibold leading-none text-white">
      {count > 99 ? '99+' : count}
    </span>
  );
};

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

function mapNotice(n: StudentNoticeItem) {
  return {
    id: n.id,
    className: n.class_name,
    title: n.title,
    body: n.body,
    publishedAt: formatRelativeTime(n.published_at),
    confirmed: n.confirmed,
    attachments: n.attachments,
  };
}

function mergeMessages<T extends { id: string }>(current: T[], incoming: T[]): T[] {
  const byID = new Map(current.map((item) => [item.id, item]));
  incoming.forEach((item) => byID.set(item.id, item));
  return [...byID.values()];
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export const MessageCenterPage: React.FC = () => {
  const conversationRequest = useRef(0);
  const threadRequest = useRef(0);
  const { toast } = useToast();
  // ---- state ---------------------------------------------------------
  const [searchTerm, setSearchTerm] = useState('');
  const [serverSearch, setServerSearch] = useState('');
  const [activeTab, setActiveTab] = useState('private');
  const [initialLoad, setInitialLoad] = useState(true);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');

  // conversations
  const [convItems, setConvItems] = useState<ReturnType<typeof mapConversationItem>[]>([]);
  const [activeConv, setActiveConv] = useState<ConversationDetail | null>(null);
  const [activeConvId, setActiveConvId] = useState('');
  const [messageDraft, setMessageDraft] = useState('');
  const [sendingMsg, setSendingMsg] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
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
  const [notices, setNotices] = useState<ReturnType<typeof mapNotice>[]>([]);
  const [activeNoticeId, setActiveNoticeId] = useState('');
  const [noticeStatus, setNoticeStatus] = useState('全部');
  const [confirming, setConfirming] = useState('');
  const [noticePage, setNoticePage] = useState(1);
  const [noticeTotal, setNoticeTotal] = useState(0);

  // questions
  const [questions, setQuestions] = useState<StudentThreadItem[]>([]);
  const [activeThread, setActiveThread] = useState<ThreadDetail | null>(null);
  const [activeQuestionId, setActiveQuestionId] = useState('');
  const [questionDraft, setQuestionDraft] = useState('');
  const [selectedQTeacherId, setSelectedQTeacherId] = useState('');
  const [submittingQ, setSubmittingQ] = useState(false);
  const [followUpDraft, setFollowUpDraft] = useState('');
  const [sendingFollowUp, setSendingFollowUp] = useState(false);
  const [loadingOlderThreadMessages, setLoadingOlderThreadMessages] = useState(false);
  const [deletingThread, setDeletingThread] = useState(false);
  const [questionPage, setQuestionPage] = useState(1);
  const [questionTotal, setQuestionTotal] = useState(0);
  const [loadingMoreList, setLoadingMoreList] = useState('');

  // import modal
  const [importOpen, setImportOpen] = useState(false);
  const [importTeacherId, setImportTeacherId] = useState('');
  const [importing, setImporting] = useState(false);
  const [mistakes, setMistakes] = useState<MistakeRecord[]>([]);
  const [loadingMistakes, setLoadingMistakes] = useState(false);
  const [selectedMistakeId, setSelectedMistakeId] = useState('');
  const [importQuestionText, setImportQuestionText] = useState('');

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
  const loadConversations = useCallback(async (page = 1, append = false) => {
    try {
      const response = await conversationService.list({ search: serverSearch, page, page_size: 50 });
      const items = response.items.map(mapConversationItem);
      setConvItems((current) => append ? [...current, ...items] : items);
      setConversationPage(page);
      setConversationTotal(response.total);
      return true;
    } catch { return false; }
  }, [serverSearch]);

  const loadConversationDetail = useCallback(async (id: string): Promise<boolean> => {
    const request = ++conversationRequest.current;
    try {
      const detail = await conversationService.get(id);
      if (request === conversationRequest.current) setActiveConv(detail);
      return true;
    } catch {
      return false;
    }
  }, []);

  // ---- load notices ---------------------------------------------------
  const loadNotices = useCallback(async (page = 1, append = false) => {
    try {
      const response = await noticeService.list({ search: serverSearch, status: noticeStatus, page, page_size: 50 });
      const items = response.items as StudentNoticeItem[];
      setNotices((current) => append ? [...current, ...items.map(mapNotice)] : items.map(mapNotice));
      setNoticePage(page);
      setNoticeTotal(response.total);
      if (items.length > 0 && !activeNoticeId) {
        setActiveNoticeId(items[0].id);
      }
      return true;
    } catch { return false; }
  }, [serverSearch, noticeStatus, activeNoticeId]);

  // ---- load questions -------------------------------------------------
  const loadQuestions = useCallback(async (page = 1, append = false) => {
    try {
      const response = await qaThreadService.list({ search: serverSearch, page, page_size: 50 });
      const items = response.items as StudentThreadItem[];
      setQuestions((current) => append ? [...current, ...items] : items);
      setQuestionPage(page);
      setQuestionTotal(response.total);
      if (items.length > 0 && !activeQuestionId) {
        setActiveQuestionId(items[0].id);
      }
      return true;
    } catch { return false; }
  }, [serverSearch, activeQuestionId]);

  const loadThreadDetail = useCallback(async (id: string): Promise<boolean> => {
    const request = ++threadRequest.current;
    try {
      const detail = await qaThreadService.get(id);
      if (request === threadRequest.current) setActiveThread(detail);
      return true;
    } catch {
      return false;
    }
  }, []);

  const reloadInitialData = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    const results = await Promise.all([loadContacts(), loadConversations(), loadNotices(), loadQuestions()]);
    if (results.some((success) => !success)) setLoadError('部分消息中心数据加载失败，请检查网络后重试。');
    setLoading(false);
    setInitialLoad(false);
  }, [loadContacts, loadConversations, loadNotices, loadQuestions]);

  // ---- initial load — only shows full-page spinner on first mount
  useEffect(() => { void reloadInitialData(); }, [reloadInitialData]);

  useEffect(() => {
    const refresh = async () => {
      if (document.hidden) return;
      await Promise.all([
        conversationPage === 1 ? loadConversations() : Promise.resolve(),
        noticePage === 1 ? loadNotices() : Promise.resolve(),
        questionPage === 1 ? loadQuestions() : Promise.resolve(),
      ]);
      if (activeConvId) {
        try {
          const detail = await conversationService.get(activeConvId);
          setActiveConv((current) => current?.id === detail.id ? {
            ...detail,
            messages: mergeMessages(current.messages, detail.messages),
            messages_page: current.messages_page,
            messages_page_size: current.messages_page_size,
          } : current);
        } catch { /* retain the last successfully loaded detail */ }
      }
      if (activeQuestionId) {
        try {
          const detail = await qaThreadService.get(activeQuestionId);
          setActiveThread((current) => current?.id === detail.id ? {
            ...detail,
            messages: mergeMessages(current.messages, detail.messages),
            messages_page: current.messages_page,
            messages_page_size: current.messages_page_size,
          } : current);
        } catch { /* retain the last successfully loaded detail */ }
      }
    };
    const interval = window.setInterval(() => { void refresh(); }, 30_000);
    return () => window.clearInterval(interval);
  }, [loadConversations, loadNotices, loadQuestions, conversationPage, noticePage, questionPage, activeConvId, activeQuestionId]);

  // ---- derived --------------------------------------------------------
  const activeNotice = useMemo(
    () => notices.find((n) => n.id === activeNoticeId) ?? notices[0],
    [notices, activeNoticeId],
  );

  const filteredConversations = useMemo(
    () => convItems.filter((c) =>
      !c.archived &&
      matchKeywords(`${c.teacherName} ${c.scope} ${c.lastMessage}`, searchTerm),
    ),
    [convItems, searchTerm],
  );

  const filteredNotices = useMemo(
    () => notices.filter((n) => {
      const matchesSearch = matchKeywords(`${n.className} ${n.title} ${n.body}`, searchTerm);
      const matchesStatus = noticeStatus === '全部' ||
        (noticeStatus === '待确认' && !n.confirmed) ||
        (noticeStatus === '已确认' && n.confirmed);
      return matchesSearch && matchesStatus;
    }),
    [notices, noticeStatus, searchTerm],
  );

  const filteredQuestions = useMemo(
    () => questions.filter((q) =>
      matchKeywords(`${q.title} ${q.teacher_name} ${q.source} ${q.context}`, searchTerm),
    ),
    [questions, searchTerm],
  );

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
        c.teacher_name.toLowerCase().includes(kw) ||
        c.scope.toLowerCase().includes(kw),
      );
    },
    [availableContacts, contactSearch],
  );

  // Global search — debounced, queries all users
  useEffect(() => {
    const q = contactSearch.trim();
    if (!q) { setGlobalSearchResults([]); return; }
    const timer = setTimeout(async () => {
      try {
        const { contacts: list } = await conversationService.searchUsers(q);
        setGlobalSearchResults(list.filter((c) => !availableContacts.some((a) => a.id === c.id)));
      } catch { setGlobalSearchResults([]); }
    }, 300);
    return () => clearTimeout(timer);
  }, [contactSearch, availableContacts]);

  const allSearchResults = useMemo(() => {
    const local = filteredAvailableContacts;
    const extra = globalSearchResults.filter((g) => !local.some((l) => l.id === g.id));
    return [...local, ...extra];
  }, [filteredAvailableContacts, globalSearchResults]);

  const privateUnreadCount = convItems.filter((c) => !c.archived).reduce((t, c) => t + c.unread, 0);
  const noticePendingCount = notices.filter((n) => !n.confirmed).length;
  const questionAnsweredCount = questions.filter((q) => q.status !== '待回复').length;

  // ---- actions: conversations -----------------------------------------
  const openConversation = useCallback(async (id: string) => {
    const unread = convItems.find((item) => item.id === id)?.unread ?? 0;
    setActiveConvId(id);
    setActiveConv(null);
    setConvItems((prev) => prev.map((c) => (c.id === id ? { ...c, unread: 0 } : c)));
    if (!await loadConversationDetail(id)) {
      setConvItems((prev) => prev.map((c) => (c.id === id ? { ...c, unread } : c)));
      toast({ type: 'error', title: '加载私信详情失败，请稍后重试' });
    }
  }, [convItems, loadConversationDetail, toast]);

  const sendPrivateMessage = useCallback(async (event?: React.FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    if (!activeConvId || !messageDraft.trim() || sendingMsg) return;
    setSendingMsg(true);
    try {
      await conversationService.sendMessage(activeConvId, messageDraft.trim());
      setMessageDraft('');
      await loadConversationDetail(activeConvId);
      await loadConversations(); // refresh sidebar
    } catch {
      toast({ type: 'error', title: '发送私信失败，请稍后重试' });
    }
    finally { setSendingMsg(false); }
  }, [activeConvId, messageDraft, sendingMsg, loadConversationDetail, loadConversations, toast]);

  const loadOlderConversationMessages = useCallback(async () => {
    if (!activeConv || loadingOlderMessages || activeConv.messages.length >= activeConv.messages_total) return;
    setLoadingOlderMessages(true);
    try {
      const detail = await conversationService.get(activeConv.id, { messages_page: activeConv.messages_page + 1, messages_page_size: activeConv.messages_page_size });
      setActiveConv((current) => current?.id === detail.id ? { ...detail, messages: [...detail.messages, ...current.messages] } : current);
    } catch { toast({ type: 'error', title: '加载更早私信失败，请稍后重试' }); } finally { setLoadingOlderMessages(false); }
  }, [activeConv, loadingOlderMessages, toast]);

  const loadMoreConversations = useCallback(async () => {
    if (loadingMoreList || convItems.length >= conversationTotal) return;
    setLoadingMoreList('conversations');
    await loadConversations(conversationPage + 1, true);
    setLoadingMoreList('');
  }, [loadingMoreList, convItems.length, conversationTotal, loadConversations, conversationPage]);

  const loadMoreNotices = useCallback(async () => {
    if (loadingMoreList || notices.length >= noticeTotal) return;
    setLoadingMoreList('notices');
    await loadNotices(noticePage + 1, true);
    setLoadingMoreList('');
  }, [loadingMoreList, notices.length, noticeTotal, loadNotices, noticePage]);

  const loadMoreQuestions = useCallback(async () => {
    if (loadingMoreList || questions.length >= questionTotal) return;
    setLoadingMoreList('questions');
    await loadQuestions(questionPage + 1, true);
    setLoadingMoreList('');
  }, [loadingMoreList, questions.length, questionTotal, loadQuestions, questionPage]);

  const createConversation = useCallback(async () => {
    if (!selectedTeacherId || creatingConv) return;
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
      setActiveConvId(detail.id);
      setActiveConv(detail);
    } catch {
      toast({ type: 'error', title: '创建私信失败，请稍后重试' });
    }
    finally { setCreatingConv(false); }
  }, [selectedTeacherId, newConvDraft, creatingConv, loadConversations, toast]);

  const archiveConversation = useCallback(async (id: string) => {
    try {
      await conversationService.archive(id);
      await loadConversations();
      const next = convItems.find((c) => c.id !== id && !c.archived);
      if (next) {
        setActiveConvId(next.id);
        setActiveConv(null);
        if (!await loadConversationDetail(next.id)) toast({ type: 'error', title: '加载下一条私信失败，请稍后重试' });
      } else {
        setActiveConvId('');
        setActiveConv(null);
      }
    } catch {
      toast({ type: 'error', title: '归档私信失败，请稍后重试' });
    }
  }, [loadConversations, convItems, loadConversationDetail, toast]);

  const deleteConversation = useCallback(async (id: string) => {
    try {
      await conversationService.delete(id);
      await loadConversations();
      const next = convItems.find((c) => c.id !== id && !c.archived);
      if (next) {
        setActiveConvId(next.id);
        setActiveConv(null);
        if (!await loadConversationDetail(next.id)) toast({ type: 'error', title: '加载下一条私信失败，请稍后重试' });
      } else {
        setActiveConvId('');
        setActiveConv(null);
      }
    } catch {
      toast({ type: 'error', title: '删除私信失败，请稍后重试' });
    }
  }, [loadConversations, convItems, loadConversationDetail, toast]);

  // ---- actions: notices -----------------------------------------------
  const confirmNotice = useCallback(async (id: string) => {
    if (confirming === id) return;
    setConfirming(id);
    try {
      await noticeService.confirm(id);
      setNotices((prev) => prev.map((n) => (n.id === id ? { ...n, confirmed: true } : n)));
    } catch {
      toast({ type: 'error', title: '确认通知失败，请稍后重试' });
    }
    finally { setConfirming(''); }
  }, [confirming, toast]);

  // ---- actions: questions ---------------------------------------------
  const createQuestion = useCallback(async () => {
    if (!questionDraft.trim() || submittingQ) return;
    setSubmittingQ(true);
    try {
      await qaThreadService.create({ teacher_id: selectedQTeacherId, content: questionDraft.trim() });
      setQuestionDraft('');
      await loadQuestions();
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
    } catch { /* silent */ }
    finally { setLoadingMistakes(false); }
  }, []);

  const importQuestion = useCallback(async () => {
    if (importing) return;
    const selected = mistakes.find((m) => m.id === selectedMistakeId);
    if (!selected) return;
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
      await loadQuestions();
    } catch { /* silent */ }
    finally { setImporting(false); }
  }, [importing, mistakes, selectedMistakeId, importTeacherId, importQuestionText, loadQuestions]);

  const createFollowUp = useCallback(async () => {
    if (!followUpDraft.trim() || !activeQuestionId || sendingFollowUp) return;
    setSendingFollowUp(true);
    try {
      await qaThreadService.sendMessage(activeQuestionId, followUpDraft.trim());
      setFollowUpDraft('');
      await loadThreadDetail(activeQuestionId);
      await loadQuestions();
    } catch {
      toast({ type: 'error', title: '发送追问失败，请稍后重试' });
    }
    finally { setSendingFollowUp(false); }
  }, [followUpDraft, activeQuestionId, sendingFollowUp, loadThreadDetail, loadQuestions, toast]);

  const loadOlderThreadMessages = useCallback(async () => {
    if (!activeThread || loadingOlderThreadMessages || activeThread.messages.length >= activeThread.messages_total) return;
    setLoadingOlderThreadMessages(true);
    try {
      const detail = await qaThreadService.get(activeThread.id, { messages_page: activeThread.messages_page + 1, messages_page_size: activeThread.messages_page_size });
      setActiveThread((current) => current?.id === detail.id ? { ...detail, messages: [...detail.messages, ...current.messages] } : current);
    } catch { toast({ type: 'error', title: '加载更早答疑消息失败，请稍后重试' }); } finally { setLoadingOlderThreadMessages(false); }
  }, [activeThread, loadingOlderThreadMessages, toast]);

  const deleteThread = useCallback(async () => {
    if (!activeQuestionId || deletingThread) return;
    if (!window.confirm('确定要删除这条提问吗？')) return;
    setDeletingThread(true);
    try {
      await qaThreadService.delete(activeQuestionId);
      setActiveQuestionId('');
      setActiveThread(null);
      await loadQuestions();
    } catch {
      toast({ type: 'error', title: '删除提问失败，请稍后重试' });
    } finally {
      setDeletingThread(false);
    }
  }, [activeQuestionId, deletingThread, loadQuestions, toast]);

  const selectQuestion = useCallback(async (id: string) => {
    setActiveQuestionId(id);
    setActiveThread(null);
    await loadThreadDetail(id);
  }, [loadThreadDetail]);

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
      <div className="container mx-auto max-w-7xl px-4 py-8">
        <div className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
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

        {loadError && <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200"><span>{loadError}</span><Button variant="outline" size="sm" onClick={() => void reloadInitialData()} disabled={loading}>重新加载</Button></div>}

        <Tabs defaultValue="private" keepMounted={false} onValueChange={(v) => { setActiveTab(v); setSearchTerm(''); }}>
          <TabsList className="mb-4">
            <TabsTrigger value="private">
              <MessageSquare className="mr-2 h-4 w-4" />
              私信{renderTabCount(privateUnreadCount)}
            </TabsTrigger>
            <TabsTrigger value="notices">
              <Bell className="mr-2 h-4 w-4" />
              通知{renderTabCount(noticePendingCount)}
            </TabsTrigger>
            <TabsTrigger value="questions">
              <HelpCircle className="mr-2 h-4 w-4" />
              答疑{renderTabCount(questionAnsweredCount)}
            </TabsTrigger>
          </TabsList>

          {/* ============================================================ PRIVATE */}
          <TabsContent value="private" className="mt-0">
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[340px_1fr]">
              <Card>
                <CardContent className="p-0">
                  <div className="border-b border-surface-100 p-4 dark:border-surface-800">
                    <Button
                      className="w-full"
                      onClick={() => {
                        setContactSearch('');
                        setSelectedTeacherId(availableContacts[0]?.id ?? '');
                        setNewConvOpen(true);
                      }}
                    >
                      <Plus className="mr-2 h-4 w-4" />
                      新建对话
                    </Button>
                  </div>
                  {filteredConversations.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      onClick={() => openConversation(c.id)}
                      className={cn(
                        'w-full border-b border-surface-100 p-4 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeConvId === c.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-medium text-surface-900 dark:text-surface-100">{c.teacherName}</div>
                          <div className="text-xs text-surface-500 dark:text-surface-400">{c.scope}</div>
                        </div>
                        <Badge variant={c.unread > 0 ? 'warning' : 'secondary'}>
                          {c.unread > 0 ? `未读 ${c.unread}` : '已读'}
                        </Badge>
                      </div>
                      <p className="mt-2 line-clamp-2 text-sm text-surface-600 dark:text-surface-300">{c.lastMessage}</p>
                      <div className="mt-2 text-xs text-surface-400">{c.lastTime}</div>
                    </button>
                  ))}
                  {convItems.length < conversationTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreConversations} disabled={loadingMoreList !== ''}>{loadingMoreList === 'conversations' ? '加载中…' : '加载更多对话'}</Button>}
                </CardContent>
              </Card>

              <Card>
                <CardContent className="flex h-full flex-col p-0">
                  {activeConv ? (
                    <>
                      <div className="border-b border-surface-100 p-5 dark:border-surface-800">
                        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                          <div>
                            <div className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                              {activeConv.teacher_name}
                            </div>
                            <div className="text-sm text-surface-500 dark:text-surface-400">{activeConv.scope}</div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <Button variant="outline" size="sm" onClick={() => archiveConversation(activeConv.id)}>
                              <Archive className="mr-2 h-4 w-4" />归档
                            </Button>
                            <Button variant="outline" size="sm" onClick={() => deleteConversation(activeConv.id)}>
                              <Trash2 className="mr-2 h-4 w-4" />删除
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
                          <Input value={messageDraft} onChange={(e) => setMessageDraft(e.target.value)} placeholder="输入给老师的消息" />
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
              <Card>
                <CardContent className="p-0">
                  {filteredNotices.map((n) => (
                    <button
                      key={n.id}
                      type="button"
                      onClick={() => setActiveNoticeId(n.id)}
                      className={cn(
                        'w-full border-b border-surface-100 p-4 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeNotice?.id === n.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-medium text-surface-900 dark:text-surface-100">{n.title}</div>
                          <div className="mt-1 text-xs text-surface-500 dark:text-surface-400">{n.className}</div>
                        </div>
                        <Badge variant={n.confirmed ? 'success' : 'warning'}>{n.confirmed ? '已确认' : '待确认'}</Badge>
                      </div>
                      <div className="mt-2 text-xs text-surface-400">{n.publishedAt}</div>
                    </button>
                  ))}
                  {notices.length < noticeTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreNotices} disabled={loadingMoreList !== ''}>{loadingMoreList === 'notices' ? '加载中…' : '加载更多通知'}</Button>}
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  {activeNotice && (
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
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          {/* ============================================================ QUESTIONS */}
          <TabsContent value="questions" className="mt-0">
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[360px_1fr]">
              <Card>
                <CardContent className="space-y-4 p-4">
                  <div className="space-y-2">
                    <select
                      value={selectedQTeacherId}
                      onChange={(e) => setSelectedQTeacherId(e.target.value)}
                      className="h-10 w-full rounded-md border border-surface-200 bg-white px-3 text-sm dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                    >
                      {contacts.map((c) => <option key={c.id} value={c.id}>{c.teacher_name} · {c.scope}</option>)}
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
                    {filteredQuestions.map((q) => (
                      <button
                        key={q.id}
                        type="button"
                        onClick={() => selectQuestion(q.id)}
                        className={cn(
                          'w-full py-4 text-left hover:bg-surface-50 dark:hover:bg-surface-800',
                          activeQuestionId === q.id && 'bg-primary-50 dark:bg-primary-950/30',
                        )}
                      >
                        <div className="flex items-start justify-between gap-3 px-3">
                          <div>
                            <div className="font-medium text-surface-900 dark:text-surface-100">{q.title}</div>
                            <div className="mt-1 text-xs text-surface-500 dark:text-surface-400">
                              {q.teacher_name} · {q.source} · {formatRelativeTime(q.last_update)}
                            </div>
                          </div>
                          <Badge variant={statusVariant[q.status as keyof typeof statusVariant] ?? 'secondary'}>{q.status}</Badge>
                        </div>
                      </button>
                    ))}
                    {questions.length < questionTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreQuestions} disabled={loadingMoreList !== ''}>{loadingMoreList === 'questions' ? '加载中…' : '加载更多提问'}</Button>}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  {activeThread ? (
                    <div className="space-y-5">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <div className="text-sm text-surface-500 dark:text-surface-400">
                            提问给：{activeThread.teacher_name} · 来源：{activeThread.source}
                          </div>
                          <h2 className="mt-2 text-xl font-semibold text-surface-900 dark:text-surface-100">{activeThread.title}</h2>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={statusVariant[activeThread.status as keyof typeof statusVariant] ?? 'secondary'}>{activeThread.status}</Badge>
                          <Button variant="outline" size="sm" onClick={deleteThread} disabled={deletingThread}>
                            {deletingThread ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                          </Button>
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
                          onChange={(e) => setFollowUpDraft(e.target.value)}
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
              {contacts.map((c) => <option key={c.id} value={c.id}>{c.teacher_name} · {c.scope}</option>)}
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
        <Modal isOpen={newConvOpen} onClose={() => { setNewConvOpen(false); setContactSearch(''); }} title="新建私信对话" className="max-w-lg">
          <div className="space-y-4">
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">选择联系人</label>
            <Input value={contactSearch} onChange={(e) => { setContactSearch(e.target.value); if (!e.target.value.trim()) setSelectedTeacherId(''); }}
              placeholder="搜索老师姓名或 ID…" />
            {allSearchResults.length > 0 ? (
              <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-md border border-surface-200 dark:border-surface-700">
                {allSearchResults.map((c) => (
                  <button key={c.id} type="button"
                    onClick={() => { setSelectedTeacherId(c.id); setContactSearch(''); setGlobalSearchResults([]); }}
                    className={cn('w-full px-4 py-2.5 text-left text-sm hover:bg-surface-50 dark:hover:bg-surface-800',
                      selectedTeacherId === c.id && 'bg-primary-50 ring-1 ring-inset ring-primary-200 dark:bg-primary-950/30 dark:ring-primary-800')}>
                    <div className="font-medium text-surface-900 dark:text-surface-100">{c.teacher_name}</div>
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
              <Button variant="outline" onClick={() => { setNewConvOpen(false); setContactSearch(''); }}>取消</Button>
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
