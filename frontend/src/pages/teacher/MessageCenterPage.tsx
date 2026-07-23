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
  Bell,
  CheckCircle2,
  HelpCircle,
  Loader2,
  Megaphone,
  MessageSquare,
  Plus,
  Search,
  Send,
  Users,
} from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { formatRelativeTime } from '@/libs/utils/dateFormat';
import { classService } from '@/modules/classroom/services/classService';
import {
  conversationService,
  type ConversationDetail,
  type Contact,
} from '@/modules/message-center/services/conversationService';
import {
  noticeService,
  type TeacherNoticeItem,
} from '@/modules/message-center/services/noticeService';
import {
  qaThreadService,
  type TeacherThreadItem,
  type ThreadDetail,
} from '@/modules/message-center/services/qaThreadService';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function matchKeywords(haystack: string, search: string): boolean {
  if (!search.trim()) return true;
  const keywords = search.trim().toLowerCase().split(/\s+/);
  const lower = haystack.toLowerCase();
  return keywords.every((kw) => lower.includes(kw));
}

const privateStatuses = ['全部', '未读', '待回复'];
const noticeStatuses = ['全部', '有未确认', '全部确认'];
const answerStatuses = ['全部', '待回复', '已回复', '已解决', '需跟进'];
const threadStatusVariant: Record<string, 'warning' | 'default' | 'success' | 'secondary'> = {
  '待回复': 'warning',
  '已回复': 'default',
  '已解决': 'success',
  '需跟进': 'secondary',
};

const renderTabCount = (count: number) => {
  if (count <= 0) return null;
  return (
    <span className="ml-2 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-xs font-semibold leading-none text-white">
      {count > 99 ? '99+' : count}
    </span>
  );
};

interface ConvItem {
  id: string;
  studentName: string;
  className: string;
  lastMessage: string;
  lastTime: string;
  unread: boolean;
  pendingReply: boolean;
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
  const [convItems, setConvItems] = useState<ConvItem[]>([]);
  const [activeConv, setActiveConv] = useState<ConversationDetail | null>(null);
  const [activeConvId, setActiveConvId] = useState('');
  const [messageDraft, setMessageDraft] = useState('');
  const [sendingMsg, setSendingMsg] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const [conversationPage, setConversationPage] = useState(1);
  const [conversationTotal, setConversationTotal] = useState(0);
  const [privateStatus, setPrivateStatus] = useState('全部');

  // new conversation
  const [studentContacts, setStudentContacts] = useState<Contact[]>([]);
  const [newConvOpen, setNewConvOpen] = useState(false);
  const [selectedStudentId, setSelectedStudentId] = useState('');
  const [contactSearch, setContactSearch] = useState('');
  const [globalSearchResults, setGlobalSearchResults] = useState<Contact[]>([]);
  const [newConvDraft, setNewConvDraft] = useState('');
  const [creatingConv, setCreatingConv] = useState(false);

  // notices
  const [notices, setNotices] = useState<TeacherNoticeItem[]>([]);
  const [activeNoticeId, setActiveNoticeId] = useState('');
  const [noticeStatus, setNoticeStatus] = useState('全部');
  const [noticeModalOpen, setNoticeModalOpen] = useState(false);
  const [noticeTitle, setNoticeTitle] = useState('');
  const [noticeBody, setNoticeBody] = useState('');
  const [noticeClassID, setNoticeClassID] = useState('');
  const [publishing, setPublishing] = useState(false);
  const [noticePage, setNoticePage] = useState(1);
  const [noticeTotal, setNoticeTotal] = useState(0);

  // threads
  const [threads, setThreads] = useState<TeacherThreadItem[]>([]);
  const [activeThread, setActiveThread] = useState<ThreadDetail | null>(null);
  const [activeThreadId, setActiveThreadId] = useState('');
  const [answerStatus, setAnswerStatus] = useState('全部');
  const [answerDraft, setAnswerDraft] = useState('');
  const [sendingAnswer, setSendingAnswer] = useState(false);
  const [loadingOlderThreadMessages, setLoadingOlderThreadMessages] = useState(false);
  const [threadPage, setThreadPage] = useState(1);
  const [threadTotal, setThreadTotal] = useState(0);
  const [loadingMoreList, setLoadingMoreList] = useState('');

  const [noticeClasses, setNoticeClasses] = useState<Array<{ id: string; name: string }>>([]);

  useEffect(() => {
    const timer = window.setTimeout(() => setServerSearch(searchTerm.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [searchTerm]);

  // load real class names from class service on mount
  useEffect(() => {
    let active = true;
    classService.listTeacherClasses().then((res) => {
      if (active && res.items?.length > 0) {
        const classes = res.items.map((c) => ({ id: c.id, name: c.name }));
        setNoticeClasses(classes);
        setNoticeClassID(classes[0].id);
      }
    }).catch(() => {
      if (active) toast({ type: 'error', title: '班级列表加载失败，暂不能发布通知' });
    });
    return () => { active = false; };
  }, [toast]);

  // ---- load data ------------------------------------------------------
  const loadConversations = useCallback(async (page = 1, append = false) => {
    try {
      const status = privateStatus === '全部' ? '' : privateStatus;
      const response = await conversationService.list({ search: serverSearch, status, page, page_size: 50 });
      const items = response.items.map((c) => ({
        id: c.id,
        studentName: c.student_name ?? '',
        className: c.class_name ?? '',
        lastMessage: c.last_message,
        lastTime: formatRelativeTime(c.last_time),
        unread: c.unread > 0,
        pendingReply: c.pending_reply ?? false,
      }));
      setConvItems((current) => append ? [...current, ...items] : items);
      setConversationPage(page);
      setConversationTotal(response.total);
      return true;
    } catch { return false; }
  }, [serverSearch, privateStatus]);

	const loadConvDetail = useCallback(async (id: string) => {
		const request = ++conversationRequest.current;
		try {
			const d = await conversationService.get(id);
			if (request === conversationRequest.current) setActiveConv(d);
      return true;
    } catch { return false; }
  }, []);

  const loadStudentContacts = useCallback(async () => {
    try {
      const { contacts: list } = await conversationService.studentContacts();
      setStudentContacts(list);
      if (list.length > 0) setSelectedStudentId(list[0].id);
      return true;
    } catch { return false; }
  }, []);

  const filteredStudentContacts = useMemo(
    () => {
      if (!contactSearch.trim()) return studentContacts;
      const kw = contactSearch.trim().toLowerCase();
      return studentContacts.filter((c) =>
        c.id.toLowerCase().includes(kw) ||
        c.teacher_name.toLowerCase().includes(kw) ||
        c.scope.toLowerCase().includes(kw),
      );
    },
    [studentContacts, contactSearch],
  );

  useEffect(() => {
    const q = contactSearch.trim();
    if (!q) { setGlobalSearchResults([]); return; }
    const timer = setTimeout(async () => {
      try {
        const { contacts: list } = await conversationService.searchUsers(q);
        setGlobalSearchResults(list.filter((c) => !studentContacts.some((s) => s.id === c.id)));
      } catch { setGlobalSearchResults([]); }
    }, 300);
    return () => clearTimeout(timer);
  }, [contactSearch, studentContacts]);

  const allStudentSearchResults = useMemo(() => {
    const local = filteredStudentContacts;
    const extra = globalSearchResults.filter((g) => !local.some((l) => l.id === g.id));
    return [...local, ...extra];
  }, [filteredStudentContacts, globalSearchResults]);

  const createConversation = useCallback(async () => {
    if (!selectedStudentId || creatingConv) return;
    setCreatingConv(true);
    try {
      const student = studentContacts.find((s) => s.id === selectedStudentId);
      const detail = await conversationService.create({
        target_id: selectedStudentId,
        subject: student?.scope ?? '',
        initial_message: newConvDraft.trim(),
      });
      setNewConvDraft('');
      setNewConvOpen(false);
      setContactSearch('');
      await loadConversations();
      setActiveConvId(detail.id);
      setActiveConv(detail);
    } catch {
      toast({ type: 'error', title: '创建私信失败，请稍后重试' });
    }
    finally { setCreatingConv(false); }
  }, [selectedStudentId, studentContacts, newConvDraft, creatingConv, loadConversations, toast]);

  const loadNotices = useCallback(async (page = 1, append = false) => {
    try {
      const status = noticeStatus === '全部' ? '' : noticeStatus === '有未确认' ? '有未确认' : '全部确认';
      const response = await noticeService.list({ search: serverSearch, status, page, page_size: 50 });
      const items = response.items as TeacherNoticeItem[];
      setNotices((current) => append ? [...current, ...items] : items);
      setNoticePage(page);
      setNoticeTotal(response.total);
      if (items.length > 0 && !activeNoticeId) setActiveNoticeId(items[0].id);
      return true;
    } catch { return false; }
  }, [serverSearch, noticeStatus, activeNoticeId]);

  const loadThreads = useCallback(async (page = 1, append = false) => {
    try {
      const status = answerStatus === '全部' ? '' : answerStatus;
      const response = await qaThreadService.list({ search: serverSearch, status, page, page_size: 50 });
      const items = response.items as TeacherThreadItem[];
      setThreads((current) => append ? [...current, ...items] : items);
      setThreadPage(page);
      setThreadTotal(response.total);
      if (items.length > 0 && !activeThreadId) setActiveThreadId(items[0].id);
      return true;
    } catch { return false; }
  }, [serverSearch, answerStatus, activeThreadId]);

	const loadThreadDetail = useCallback(async (id: string) => {
		const request = ++threadRequest.current;
		try {
			const d = await qaThreadService.get(id);
			if (request === threadRequest.current) setActiveThread(d);
    } catch { /* silent */ }
  }, []);

  const reloadInitialData = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    const results = await Promise.all([loadConversations(), loadNotices(), loadThreads()]);
    if (results.some((success) => !success)) setLoadError('部分消息中心数据加载失败，请检查网络后重试。');
    setLoading(false);
    setInitialLoad(false);
  }, [loadConversations, loadNotices, loadThreads]);

  // initial load — only shows full-page spinner on first mount
  useEffect(() => { void reloadInitialData(); }, [reloadInitialData]);

  useEffect(() => {
    const refresh = async () => {
      if (document.hidden) return;
      await Promise.all([
        conversationPage === 1 ? loadConversations() : Promise.resolve(),
        noticePage === 1 ? loadNotices() : Promise.resolve(),
        threadPage === 1 ? loadThreads() : Promise.resolve(),
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
      if (activeThreadId) {
        try {
          const detail = await qaThreadService.get(activeThreadId);
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
  }, [loadConversations, loadNotices, loadThreads, conversationPage, noticePage, threadPage, activeConvId, activeThreadId]);

  // ---- derived --------------------------------------------------------
  const activeNotice = useMemo(() => notices.find((n) => n.id === activeNoticeId) ?? notices[0], [notices, activeNoticeId]);

  const filteredConversations = useMemo(
    () => convItems.filter((c) =>
      matchKeywords(`${c.studentName} ${c.className} ${c.lastMessage}`, searchTerm),
    ),
    [convItems, searchTerm],
  );

  const filteredNotices = useMemo(
    () => notices.filter((n) => {
      const matchesSearch = matchKeywords(`${n.class_name} ${n.title} ${n.body}`, searchTerm);
      const matchesStatus = noticeStatus === '全部' ||
        (noticeStatus === '有未确认' && n.confirmed_count < n.total_count) ||
        (noticeStatus === '全部确认' && n.confirmed_count >= n.total_count);
      return matchesSearch && matchesStatus;
    }),
    [notices, noticeStatus, searchTerm],
  );

  const filteredThreads = useMemo(
    () => threads.filter((t) => {
      const matchesSearch = matchKeywords(`${t.student_name} ${t.class_name} ${t.title} ${t.source} ${t.knowledge_point} ${t.resource_name ?? ''}`, searchTerm);
      const matchesStatus = answerStatus === '全部' || t.status === answerStatus;
      return matchesSearch && matchesStatus;
    }),
    [threads, answerStatus, searchTerm],
  );

  const privatePendingCount = convItems.filter((c) => c.unread || c.pendingReply).length;
  const noticePendingCount = notices.filter((n) => n.confirmed_count < n.total_count).length;
  const answerPendingCount = threads.filter((t) => t.status === '待回复' || t.status === '需跟进').length;

  // ---- actions: conversations -----------------------------------------
  const openConversation = useCallback(async (id: string) => {
    setActiveConvId(id);
    setActiveConv(null);
    await loadConvDetail(id);
  }, [loadConvDetail]);

  const sendPrivateMessage = useCallback(async (e?: React.FormEvent<HTMLFormElement>) => {
    e?.preventDefault();
    if (!activeConvId || !messageDraft.trim() || sendingMsg) return;
    setSendingMsg(true);
    try {
      await conversationService.sendMessage(activeConvId, messageDraft.trim());
      setMessageDraft('');
      await loadConvDetail(activeConvId);
      await loadConversations();
    } catch { /* silent */ }
    finally { setSendingMsg(false); }
  }, [activeConvId, messageDraft, sendingMsg, loadConvDetail, loadConversations]);

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

  // ---- actions: notices -----------------------------------------------
  const publishNotice = useCallback(async () => {
    if (!noticeClassID) {
      toast({ type: 'error', title: '请先创建或选择一个班级' });
      return;
    }
    if (!noticeTitle.trim() || publishing) return;
    setPublishing(true);
    try {
      await noticeService.create({ class_id: noticeClassID, title: noticeTitle.trim(), body: noticeBody.trim() });
      setNoticeTitle('');
      setNoticeBody('');
      setNoticeModalOpen(false);
      await loadNotices();
    } catch {
      toast({ type: 'error', title: '发布通知失败，请稍后重试' });
    }
    finally { setPublishing(false); }
  }, [noticeTitle, noticeBody, noticeClassID, publishing, loadNotices, toast]);

  // ---- actions: threads -----------------------------------------------
  const replyThread = useCallback(async () => {
    if (!answerDraft.trim() || !activeThreadId || sendingAnswer) return;
    setSendingAnswer(true);
    try {
      await qaThreadService.sendMessage(activeThreadId, answerDraft.trim());
      setAnswerDraft('');
      await loadThreadDetail(activeThreadId);
      await loadThreads();
    } catch {
      toast({ type: 'error', title: '发送答复失败，请稍后重试' });
    }
    finally { setSendingAnswer(false); }
  }, [answerDraft, activeThreadId, sendingAnswer, loadThreadDetail, loadThreads, toast]);

  const loadOlderThreadMessages = useCallback(async () => {
    if (!activeThread || loadingOlderThreadMessages || activeThread.messages.length >= activeThread.messages_total) return;
    setLoadingOlderThreadMessages(true);
    try {
      const detail = await qaThreadService.get(activeThread.id, { messages_page: activeThread.messages_page + 1, messages_page_size: activeThread.messages_page_size });
      setActiveThread((current) => current?.id === detail.id ? { ...detail, messages: [...detail.messages, ...current.messages] } : current);
    } catch { toast({ type: 'error', title: '加载更早答疑消息失败，请稍后重试' }); } finally { setLoadingOlderThreadMessages(false); }
  }, [activeThread, loadingOlderThreadMessages, toast]);

  const loadMoreNotices = useCallback(async () => {
    if (loadingMoreList || notices.length >= noticeTotal) return;
    setLoadingMoreList('notices');
    await loadNotices(noticePage + 1, true);
    setLoadingMoreList('');
  }, [loadingMoreList, notices.length, noticeTotal, loadNotices, noticePage]);

  const loadMoreThreads = useCallback(async () => {
    if (loadingMoreList || threads.length >= threadTotal) return;
    setLoadingMoreList('threads');
    await loadThreads(threadPage + 1, true);
    setLoadingMoreList('');
  }, [loadingMoreList, threads.length, threadTotal, loadThreads, threadPage]);

  const updateThreadStatus = useCallback(async (id: string, status: string) => {
    try {
      await qaThreadService.updateStatus(id, status);
      setThreads((prev) => prev.map((t) => (t.id === id ? { ...t, status } : t)));
      setActiveThread((prev) => prev?.id === id ? { ...prev, status } : prev);
    } catch {
      toast({ type: 'error', title: '更新答疑状态失败，请稍后重试' });
    }
  }, [toast]);

  const selectThread = useCallback(async (id: string) => {
    setActiveThreadId(id);
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
            <p className="mt-1 text-sm text-surface-500 dark:text-surface-400">管理学生私信、班级通知与答疑</p>
          </div>
          <div className="relative w-full sm:w-72">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
            <Input value={searchTerm} onChange={(e) => setSearchTerm(e.target.value)} placeholder={activeTab === 'private' ? '搜索学生、班级…' : activeTab === 'notices' ? '搜索通知标题、内容…' : '搜索问题、知识点…'} className="pl-10" />
          </div>
        </div>

        {loadError && <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200"><span>{loadError}</span><Button variant="outline" size="sm" onClick={() => void reloadInitialData()} disabled={loading}>重新加载</Button></div>}

        <Tabs defaultValue="private" keepMounted={false} onValueChange={(v) => { setActiveTab(v); setSearchTerm(''); }}>
          <TabsList className="mb-4">
            <TabsTrigger value="private"><MessageSquare className="mr-2 h-4 w-4" />私信{renderTabCount(privatePendingCount)}</TabsTrigger>
            <TabsTrigger value="notices"><Bell className="mr-2 h-4 w-4" />通知{renderTabCount(noticePendingCount)}</TabsTrigger>
            <TabsTrigger value="answers"><HelpCircle className="mr-2 h-4 w-4" />答疑{renderTabCount(answerPendingCount)}</TabsTrigger>
          </TabsList>

          {/* ============================================================ PRIVATE */}
          <TabsContent value="private" className="mt-0">
            <div className="mb-4 flex flex-wrap gap-2">
              {privateStatuses.map((s) => (
                <Button key={s} variant={privateStatus === s ? 'primary' : 'outline'} size="sm" onClick={() => setPrivateStatus(s)}>{s}</Button>
              ))}
            </div>
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[340px_1fr]">
              <Card>
                <CardContent className="p-0">
                  <div className="border-b border-surface-100 p-4 dark:border-surface-800">
                    <Button className="w-full" onClick={() => { loadStudentContacts(); setContactSearch(''); setSelectedStudentId(''); setNewConvOpen(true); }}>
                      <Plus className="mr-2 h-4 w-4" />新建对话
                    </Button>
                  </div>
                  {filteredConversations.map((c) => (
                    <button key={c.id} type="button" onClick={() => openConversation(c.id)}
                      className={cn(
                        'w-full border-b border-surface-100 p-4 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeConvId === c.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}>
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-medium text-surface-900 dark:text-surface-100">{c.studentName}</div>
                          <div className="text-xs text-surface-500 dark:text-surface-400">{c.className}</div>
                        </div>
                        <Badge variant={c.pendingReply ? 'warning' : c.unread ? 'warning' : 'secondary'}>
                          {c.pendingReply ? '待回复' : c.unread ? '未读' : '已回复'}
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
                        <div className="flex items-start justify-between">
                          <div>
                            <div className="text-lg font-semibold text-surface-900 dark:text-surface-100">{activeConv.student_name}</div>
                            <div className="text-sm text-surface-500 dark:text-surface-400">{activeConv.class_name}</div>
                          </div>
                        </div>
                      </div>
                      <div className="flex-1 space-y-4 overflow-y-auto p-5">
                        {activeConv.messages.length < activeConv.messages_total && <Button variant="outline" size="sm" className="w-full" onClick={loadOlderConversationMessages} disabled={loadingOlderMessages}>{loadingOlderMessages ? '加载中…' : '加载更早消息'}</Button>}
                        {activeConv.messages.map((msg) => (
                          <div key={msg.id} className="flex w-full">
                            <div className={cn('max-w-[80%]', msg.from === 'teacher' ? 'ml-auto text-right' : 'mr-auto')}>
                              <div className={cn(
                                'inline-block rounded-lg px-4 py-3 text-sm',
                                msg.from === 'teacher' ? 'bg-primary-600 text-white' : 'bg-surface-100 text-surface-800 dark:bg-surface-800 dark:text-surface-100',
                              )}>{msg.text}</div>
                              <div className={cn('mt-1 flex gap-2 text-xs text-surface-400', msg.from === 'teacher' ? 'justify-end' : 'justify-start')}>
                                <span>{formatRelativeTime(msg.time)}</span>
                                {msg.from === 'teacher' && <span>{msg.read_by_recipient ? '学生已读' : '学生未读'}</span>}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                      <div className="border-t border-surface-100 p-4 dark:border-surface-800">
                        <form className="flex gap-2" onSubmit={sendPrivateMessage}>
                          <Input value={messageDraft} onChange={(e) => setMessageDraft(e.target.value)} placeholder="输入给学生的回复" />
                          <Button type="submit" size="icon" aria-label="发送" disabled={sendingMsg}>
                            {sendingMsg ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                          </Button>
                        </form>
                      </div>
                    </>
                  ) : (
                    <div className="flex h-full items-center justify-center p-8 text-sm text-surface-500 dark:text-surface-400">暂无可显示的私信对话</div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          {/* ============================================================ NOTICES */}
          <TabsContent value="notices" className="mt-0">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap gap-2">
                {noticeStatuses.map((s) => (
                  <Button key={s} variant={noticeStatus === s ? 'primary' : 'outline'} size="sm" onClick={() => setNoticeStatus(s)}>{s}</Button>
                ))}
              </div>
              <Button onClick={() => setNoticeModalOpen(true)}>
                <Megaphone className="mr-2 h-4 w-4" />发布通知
              </Button>
            </div>
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[360px_1fr]">
              <Card>
                <CardContent className="p-0">
                  {filteredNotices.map((n) => (
                    <button key={n.id} type="button" onClick={() => setActiveNoticeId(n.id)}
                      className={cn(
                        'w-full border-b border-surface-100 p-4 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeNotice?.id === n.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}>
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-medium text-surface-900 dark:text-surface-100">{n.title}</div>
                          <div className="mt-1 text-xs text-surface-500 dark:text-surface-400">{n.class_name}</div>
                        </div>
                        <Badge variant={n.confirmed_count >= n.total_count ? 'success' : 'warning'}>
                          {n.confirmed_count}/{n.total_count}
                        </Badge>
                      </div>
                      <div className="mt-2 text-xs text-surface-400">{formatRelativeTime(n.published_at)}</div>
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
                          <div className="text-sm text-surface-500 dark:text-surface-400">{activeNotice.class_name} · {formatRelativeTime(activeNotice.published_at)}</div>
                          <h2 className="mt-2 text-xl font-semibold text-surface-900 dark:text-surface-100">{activeNotice.title}</h2>
                        </div>
                        <Badge variant={activeNotice.confirmed_count >= activeNotice.total_count ? 'success' : 'warning'}>
                          {activeNotice.confirmed_count}/{activeNotice.total_count} 已确认
                        </Badge>
                      </div>
                      <p className="leading-7 text-surface-700 dark:text-surface-300">{activeNotice.body}</p>
                      {activeNotice.unconfirmed_students.length > 0 && (
                        <div className="space-y-2">
                          <div className="text-sm font-medium text-surface-700 dark:text-surface-300">未确认学生</div>
                          <div className="flex flex-wrap gap-2">
                            {activeNotice.unconfirmed_students.map((name) => (
                              <span key={name} className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2.5 py-1 text-xs text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
                                <Users className="h-3 w-3" />{name}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          {/* ============================================================ ANSWERS */}
          <TabsContent value="answers" className="mt-0">
            <div className="mb-4 flex flex-wrap gap-2">
              {answerStatuses.map((s) => (
                <Button key={s} variant={answerStatus === s ? 'primary' : 'outline'} size="sm" onClick={() => setAnswerStatus(s)}>{s}</Button>
              ))}
            </div>
            <div className="grid min-h-[620px] grid-cols-1 gap-4 lg:grid-cols-[360px_1fr]">
              <Card>
                <CardContent className="p-0">
                  {filteredThreads.map((t) => (
                    <button key={t.id} type="button" onClick={() => selectThread(t.id)}
                      className={cn(
                        'w-full border-b border-surface-100 p-4 text-left last:border-b-0 hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-800',
                        activeThreadId === t.id && 'bg-primary-50 dark:bg-primary-950/30',
                      )}>
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="font-medium text-surface-900 dark:text-surface-100">{t.title}</div>
                          <div className="mt-1 text-xs text-surface-500 dark:text-surface-400">{t.student_name} · {t.class_name} · {t.source}</div>
                        </div>
                        <Badge variant={threadStatusVariant[t.status] ?? 'secondary'}>{t.status}</Badge>
                      </div>
                    </button>
                  ))}
                  {threads.length < threadTotal && <Button variant="outline" size="sm" className="m-3 w-[calc(100%-1.5rem)]" onClick={loadMoreThreads} disabled={loadingMoreList !== ''}>{loadingMoreList === 'threads' ? '加载中…' : '加载更多答疑'}</Button>}
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  {activeThread ? (
                    <div className="space-y-5">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <div className="text-sm text-surface-500 dark:text-surface-400">
                            {activeThread.student_name} · {activeThread.class_name} · {activeThread.source}
                          </div>
                          <h2 className="mt-2 text-xl font-semibold text-surface-900 dark:text-surface-100">{activeThread.title}</h2>
                          {activeThread.knowledge_point && <div className="mt-1 text-xs text-surface-500">知识点：{activeThread.knowledge_point}</div>}
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={threadStatusVariant[activeThread.status] ?? 'secondary'}>{activeThread.status}</Badge>
                          <select
                            value={activeThread.status}
                            onChange={(e) => updateThreadStatus(activeThread.id, e.target.value)}
                            className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                          >
                            <option value="待回复">待回复</option>
                            <option value="已回复">已回复</option>
                            <option value="已解决">已解决</option>
                            <option value="需跟进">需跟进</option>
                          </select>
                        </div>
                      </div>
                      <div className="rounded-md bg-surface-50 p-4 text-sm leading-6 text-surface-700 dark:bg-surface-800 dark:text-surface-300">
                        {activeThread.context}
                      </div>
                      <div className="space-y-3">
                        {activeThread.messages.length < activeThread.messages_total && <Button variant="outline" size="sm" className="w-full" onClick={loadOlderThreadMessages} disabled={loadingOlderThreadMessages}>{loadingOlderThreadMessages ? '加载中…' : '加载更早消息'}</Button>}
                        {activeThread.messages.map((msg) => (
                          <div key={msg.id} className={cn('rounded-md border p-3', msg.from === 'teacher' ? 'border-primary-200 bg-primary-50/30 dark:border-primary-800 dark:bg-primary-950/20' : 'border-surface-200 dark:border-surface-700')}>
                            <div className="mb-1 flex items-center gap-2">
                              <span className={cn('text-xs font-medium', msg.from === 'teacher' ? 'text-primary-600 dark:text-primary-400' : 'text-emerald-600 dark:text-emerald-400')}>
                                {msg.from === 'teacher' ? '我' : activeThread.student_name}
                              </span>
                            </div>
                            <div className="text-sm text-surface-700 dark:text-surface-300">{msg.text}</div>
                            <div className="mt-2 text-xs text-surface-400">{formatRelativeTime(msg.time)}</div>
                          </div>
                        ))}
                      </div>
                      <div className="space-y-2 border-t border-surface-100 pt-4 dark:border-surface-800">
                        <textarea value={answerDraft} onChange={(e) => setAnswerDraft(e.target.value)}
                          placeholder="回复这位同学"
                          className="min-h-24 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
                        />
                        <div className="flex justify-end">
                          <Button onClick={replyThread} disabled={sendingAnswer}>
                            {sendingAnswer ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <CheckCircle2 className="mr-2 h-4 w-4" />}
                            回复
                          </Button>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="flex h-full items-center justify-center p-8 text-sm text-surface-500 dark:text-surface-400">暂无提问详情</div>
                  )}
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>

        {/* New conversation modal */}
        <Modal isOpen={newConvOpen} onClose={() => { setNewConvOpen(false); setContactSearch(''); }} title="新建私信对话" className="max-w-lg">
          <div className="space-y-4">
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">选择学生</label>
            <Input value={contactSearch} onChange={(e) => { setContactSearch(e.target.value); if (!e.target.value.trim()) setSelectedStudentId(''); }}
              placeholder="搜索学生姓名或 ID…" />
            {allStudentSearchResults.length > 0 ? (
              <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-md border border-surface-200 dark:border-surface-700">
                {allStudentSearchResults.map((c) => (
                  <button key={c.id} type="button"
                    onClick={() => { setSelectedStudentId(c.id); setContactSearch(''); setGlobalSearchResults([]); }}
                    className={cn('w-full px-4 py-2.5 text-left text-sm hover:bg-surface-50 dark:hover:bg-surface-800',
                      selectedStudentId === c.id && 'bg-primary-50 ring-1 ring-inset ring-primary-200 dark:bg-primary-950/30 dark:ring-primary-800')}>
                    <div className="font-medium text-surface-900 dark:text-surface-100">{c.teacher_name}</div>
                    <div className="flex items-center justify-between text-xs text-surface-500 dark:text-surface-400">
                      <span>{c.scope || '全校'}</span>
                      <span className="font-mono text-surface-400">{c.id}</span>
                    </div>
                  </button>
                ))}
              </div>
            ) : contactSearch.trim() ? (
              <p className="text-sm text-surface-500 dark:text-surface-400">未找到匹配的学生。</p>
            ) : studentContacts.length === 0 ? (
              <p className="text-sm text-surface-500 dark:text-surface-400">暂无班级学生。</p>
            ) : null}
            <textarea value={newConvDraft} onChange={(e) => setNewConvDraft(e.target.value)}
              placeholder="可以先写一句要发给学生的消息"
              className="min-h-28 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
            />
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => { setNewConvOpen(false); setContactSearch(''); }}>取消</Button>
              <Button onClick={createConversation} disabled={!selectedStudentId || creatingConv}>
                {creatingConv ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}创建对话
              </Button>
            </div>
          </div>
        </Modal>

        {/* Publish notice modal */}
        <Modal isOpen={noticeModalOpen} onClose={() => setNoticeModalOpen(false)} title="发布班级通知" className="max-w-xl">
          <div className="space-y-4">
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">目标班级</label>
            <select value={noticeClassID} onChange={(e) => setNoticeClassID(e.target.value)}
              className="h-10 w-full rounded-md border border-surface-200 bg-white px-3 text-sm dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100">
              {noticeClasses.length > 0
                ? noticeClasses.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)
                : <option value="">暂无可用班级</option>}
            </select>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">通知标题</label>
            <Input value={noticeTitle} onChange={(e) => setNoticeTitle(e.target.value)} placeholder="通知标题" />
            <textarea value={noticeBody} onChange={(e) => setNoticeBody(e.target.value)}
              placeholder="通知正文"
              className="min-h-32 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
            />
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setNoticeModalOpen(false)}>取消</Button>
              <Button onClick={publishNotice} disabled={publishing || !noticeClassID}>
                {publishing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}发布
              </Button>
            </div>
          </div>
        </Modal>
      </div>
    </MainLayout>
  );
};
