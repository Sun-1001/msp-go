import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  Ban,
  CheckCircle2,
  Clock3,
  Gauge,
  LockKeyhole,
  MessageSquareText,
  RefreshCw,
  Save,
  Search,
  ShieldAlert,
  UnlockKeyhole,
  Users,
  X,
} from 'lucide-react';
import { AdminLayout } from '@/modules/admin/components/AdminLayout';
import { aiRiskService } from '@/modules/admin/services/aiRiskService';
import type {
  AIRiskEvent,
  AIRiskEventListResponse,
  AIRiskEventType,
  AIRiskOverview,
  AIRiskSettings,
  AIStudentItem,
  AIStudentListResponse,
  AIStudentStatusFilter,
  UpdateAIRiskSettingsRequest,
} from '@/modules/admin/types/aiRisk';
import { getApiErrorMessage } from '@/libs/http/apiClient';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Progress } from '@/components/ui/Progress';
import { Select } from '@/components/ui/Select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs';

type ViewTab = 'policy' | 'students' | 'events';

interface SettingsDraft {
  dailyReplyLimit: number;
  maxConcurrentRequests: number;
  blockedKeywords: string;
}

const emptySettingsDraft: SettingsDraft = {
  dailyReplyLimit: 50,
  maxConcurrentRequests: 2,
  blockedKeywords: '',
};

const emptyStudentPage: AIStudentListResponse = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  total_pages: 0,
};

const emptyEventPage: AIRiskEventListResponse = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  total_pages: 0,
};

export const AIRiskControlPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ViewTab>('policy');
  const [overview, setOverview] = useState<AIRiskOverview | null>(null);
  const [settings, setSettings] = useState<AIRiskSettings | null>(null);
  const [settingsDraft, setSettingsDraft] = useState<SettingsDraft>(emptySettingsDraft);
  const [students, setStudents] = useState<AIStudentListResponse>(emptyStudentPage);
  const [events, setEvents] = useState<AIRiskEventListResponse>(emptyEventPage);
  const [studentPage, setStudentPage] = useState(1);
  const [studentSearch, setStudentSearch] = useState('');
  const [studentStatus, setStudentStatus] = useState<AIStudentStatusFilter>('all');
  const [eventPage, setEventPage] = useState(1);
  const [eventSearch, setEventSearch] = useState('');
  const [eventType, setEventType] = useState<AIRiskEventType | 'all'>('all');
  const [loading, setLoading] = useState({ summary: true, students: true, events: true, save: false });
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [accessTarget, setAccessTarget] = useState<AIStudentItem | null>(null);
  const [accessReason, setAccessReason] = useState('');
  const [accessLoading, setAccessLoading] = useState(false);

  const applySettings = useCallback((value: AIRiskSettings) => {
    setSettings(value);
    setSettingsDraft({
      dailyReplyLimit: value.daily_reply_limit,
      maxConcurrentRequests: value.max_concurrent_requests,
      blockedKeywords: value.blocked_keywords.join('\n'),
    });
  }, []);

  const loadSummary = useCallback(async () => {
    setLoading((current) => ({ ...current, summary: true }));
    try {
      const [overviewResponse, settingsResponse] = await Promise.all([
        aiRiskService.getOverview(),
        aiRiskService.getSettings(),
      ]);
      setOverview(overviewResponse);
      applySettings(settingsResponse);
      setError(null);
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, '加载风控概览失败'));
    } finally {
      setLoading((current) => ({ ...current, summary: false }));
    }
  }, [applySettings]);

  const loadStudents = useCallback(async () => {
    setLoading((current) => ({ ...current, students: true }));
    try {
      const response = await aiRiskService.listStudents({
        page: studentPage,
        page_size: 20,
        search: studentSearch.trim() || undefined,
        status: studentStatus,
      });
      setStudents(response);
      setError(null);
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, '加载学生 AI 状态失败'));
    } finally {
      setLoading((current) => ({ ...current, students: false }));
    }
  }, [studentPage, studentSearch, studentStatus]);

  const loadEvents = useCallback(async () => {
    setLoading((current) => ({ ...current, events: true }));
    try {
      const response = await aiRiskService.listEvents({
        page: eventPage,
        page_size: 20,
        search: eventSearch.trim() || undefined,
        event_type: eventType,
      });
      setEvents(response);
      setError(null);
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, '加载风控事件失败'));
    } finally {
      setLoading((current) => ({ ...current, events: false }));
    }
  }, [eventPage, eventSearch, eventType]);

  useEffect(() => {
    void loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadStudents(), 250);
    return () => window.clearTimeout(timer);
  }, [loadStudents]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadEvents(), 250);
    return () => window.clearTimeout(timer);
  }, [loadEvents]);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadSummary(), loadStudents(), loadEvents()]);
  }, [loadEvents, loadStudents, loadSummary]);

  const saveSettings = async () => {
    const request: UpdateAIRiskSettingsRequest = {
      daily_reply_limit: settingsDraft.dailyReplyLimit,
      max_concurrent_requests: settingsDraft.maxConcurrentRequests,
      blocked_keywords: settingsDraft.blockedKeywords
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter(Boolean),
    };
    if (request.daily_reply_limit < 1 || request.daily_reply_limit > 10_000) {
      setError('每日回复额度必须在 1 到 10000 之间');
      return;
    }
    if (request.max_concurrent_requests < 1 || request.max_concurrent_requests > 20) {
      setError('每生并发上限必须在 1 到 20 之间');
      return;
    }
    setLoading((current) => ({ ...current, save: true }));
    try {
      const response = await aiRiskService.updateSettings(request);
      applySettings(response);
      setOverview((current) => current ? {
        ...current,
        daily_reply_limit: response.daily_reply_limit,
        max_concurrent_requests: response.max_concurrent_requests,
      } : current);
      setNotice('风控策略已保存');
      setError(null);
      await loadStudents();
    } catch (saveError) {
      setError(getApiErrorMessage(saveError, '保存风控策略失败'));
    } finally {
      setLoading((current) => ({ ...current, save: false }));
    }
  };

  const submitAccessChange = async () => {
    if (!accessTarget) return;
    const shouldBlock = !accessTarget.ai_blocked;
    const reason = accessReason.trim();
    if (shouldBlock && !reason) {
      setError('请输入封禁原因');
      return;
    }
    setAccessLoading(true);
    try {
      await aiRiskService.updateStudentAccess(accessTarget.id, { blocked: shouldBlock, reason });
      setNotice(shouldBlock ? '学生 AI 权限已封禁' : '学生 AI 权限已恢复');
      setError(null);
      setAccessTarget(null);
      setAccessReason('');
      await Promise.all([loadSummary(), loadStudents(), loadEvents()]);
    } catch (accessError) {
      setError(getApiErrorMessage(accessError, '更新学生 AI 权限失败'));
    } finally {
      setAccessLoading(false);
    }
  };

  const metrics = useMemo(() => [
    {
      label: '学生总数',
      value: overview?.total_students ?? 0,
      icon: Users,
      iconClass: 'bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
    },
    {
      label: '今日 AI 回复',
      value: overview?.replies_today ?? 0,
      icon: MessageSquareText,
      iconClass: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300',
    },
    {
      label: '额度耗尽',
      value: overview?.quota_exhausted_students ?? 0,
      icon: Gauge,
      iconClass: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
    },
    {
      label: 'AI 已封禁',
      value: overview?.blocked_students ?? 0,
      icon: Ban,
      iconClass: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300',
    },
    {
      label: '今日风险事件',
      value: overview?.risk_events_today ?? 0,
      icon: AlertTriangle,
      iconClass: 'bg-orange-100 text-orange-700 dark:bg-orange-950 dark:text-orange-300',
    },
  ], [overview]);

  return (
    <AdminLayout>
      <div className="space-y-6" data-testid="ai-risk-control-page">
        <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300">
              <ShieldAlert className="h-5 w-5" />
            </div>
            <div>
              <h1 className="text-2xl font-semibold text-surface-950 dark:text-surface-50">风控中心</h1>
              <p className="text-sm text-surface-500 dark:text-surface-400">
                {settings ? `额度重置 ${formatDateTime(settings.next_reset_at)}` : '加载策略中'}
              </p>
            </div>
          </div>
          <Button variant="outline" size="icon" onClick={() => void refreshAll()} title="刷新风控数据" aria-label="刷新风控数据">
            <RefreshCw className={`h-4 w-4 ${loading.summary ? 'animate-spin' : ''}`} />
          </Button>
        </header>

        {(error || notice) && (
          <div
            role="status"
            className={`flex items-center justify-between rounded-lg border px-4 py-3 text-sm ${
              error
                ? 'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200'
                : 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200'
            }`}
          >
            <span>{error ?? notice}</span>
            <button
              type="button"
              className="rounded-md p-1 hover:bg-black/5"
              onClick={() => { setError(null); setNotice(null); }}
              aria-label="关闭提示"
              title="关闭提示"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}

        <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-5" aria-label="风控概览">
          {metrics.map((metric) => (
            <Card key={metric.label} className="min-h-28 shadow-none">
              <CardContent className="flex h-full items-center gap-4 p-5">
                <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${metric.iconClass}`}>
                  <metric.icon className="h-5 w-5" />
                </div>
                <div className="min-w-0">
                  <div className="text-sm text-surface-500 dark:text-surface-400">{metric.label}</div>
                  <div className="mt-1 text-2xl font-semibold text-surface-950 dark:text-surface-50">
                    {loading.summary ? '-' : metric.value.toLocaleString('zh-CN')}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </section>

        <Tabs value={activeTab} defaultValue="policy" onValueChange={(value) => setActiveTab(value as ViewTab)} keepMounted={false}>
          <div className="overflow-x-auto pb-1">
            <TabsList className="min-w-max">
              <TabsTrigger value="policy"><Gauge className="mr-2 h-4 w-4" />统一策略</TabsTrigger>
              <TabsTrigger value="students"><Users className="mr-2 h-4 w-4" />学生控制</TabsTrigger>
              <TabsTrigger value="events"><AlertTriangle className="mr-2 h-4 w-4" />风险事件</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="policy" className="mt-4">
            <PolicyPanel
              draft={settingsDraft}
              loading={loading.save || loading.summary}
              onChange={setSettingsDraft}
              onSave={() => void saveSettings()}
            />
          </TabsContent>

          <TabsContent value="students" className="mt-4">
            <StudentsPanel
              data={students}
              loading={loading.students}
              dailyLimit={settings?.daily_reply_limit ?? overview?.daily_reply_limit ?? 1}
              search={studentSearch}
              status={studentStatus}
              onSearch={(value) => { setStudentSearch(value); setStudentPage(1); }}
              onStatus={(value) => { setStudentStatus(value); setStudentPage(1); }}
              onPage={setStudentPage}
              onAccess={(student) => { setAccessTarget(student); setAccessReason(''); }}
            />
          </TabsContent>

          <TabsContent value="events" className="mt-4">
            <EventsPanel
              data={events}
              loading={loading.events}
              search={eventSearch}
              eventType={eventType}
              onSearch={(value) => { setEventSearch(value); setEventPage(1); }}
              onEventType={(value) => { setEventType(value); setEventPage(1); }}
              onPage={setEventPage}
            />
          </TabsContent>
        </Tabs>
      </div>

      <AccessDialog
        student={accessTarget}
        reason={accessReason}
        loading={accessLoading}
        onReason={setAccessReason}
        onClose={() => { setAccessTarget(null); setAccessReason(''); }}
        onConfirm={() => void submitAccessChange()}
      />
    </AdminLayout>
  );
};

function PolicyPanel({
  draft,
  loading,
  onChange,
  onSave,
}: {
  draft: SettingsDraft;
  loading: boolean;
  onChange: (value: SettingsDraft) => void;
  onSave: () => void;
}) {
  return (
    <section className="rounded-lg border border-surface-200 bg-white p-5 dark:border-surface-700 dark:bg-surface-900">
      <div className="flex flex-col gap-4 border-b border-surface-200 pb-5 dark:border-surface-700 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-surface-950 dark:text-surface-50">统一学生策略</h2>
          <div className="mt-1 flex items-center gap-2 text-sm text-surface-500 dark:text-surface-400">
            <Clock3 className="h-4 w-4" />
            <span>每日 00:00（Asia/Shanghai）重置</span>
          </div>
        </div>
        <Button onClick={onSave} isLoading={loading}>
          <Save className="mr-2 h-4 w-4" />保存策略
        </Button>
      </div>

      <div className="grid gap-5 py-5 md:grid-cols-2">
        <label className="space-y-2" htmlFor="daily-reply-limit">
          <span className="text-sm font-medium text-surface-800 dark:text-surface-200">每生每日 AI 回复额度</span>
          <Input
            id="daily-reply-limit"
            type="number"
            min={1}
            max={10000}
            value={draft.dailyReplyLimit}
            onChange={(event) => onChange({ ...draft, dailyReplyLimit: Number(event.target.value) })}
            disabled={loading}
          />
        </label>
        <label className="space-y-2" htmlFor="max-concurrent-requests">
          <span className="text-sm font-medium text-surface-800 dark:text-surface-200">每生 AI 并发上限</span>
          <Input
            id="max-concurrent-requests"
            type="number"
            min={1}
            max={20}
            value={draft.maxConcurrentRequests}
            onChange={(event) => onChange({ ...draft, maxConcurrentRequests: Number(event.target.value) })}
            disabled={loading}
          />
        </label>
      </div>

      <label className="block space-y-2" htmlFor="blocked-keywords">
        <span className="flex items-center justify-between text-sm font-medium text-surface-800 dark:text-surface-200">
          <span>风险关键词</span>
          <span className="font-normal text-surface-500 dark:text-surface-400">每行一个</span>
        </span>
        <textarea
          id="blocked-keywords"
          value={draft.blockedKeywords}
          onChange={(event) => onChange({ ...draft, blockedKeywords: event.target.value })}
          disabled={loading}
          rows={8}
          className="w-full resize-y rounded-md border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-50 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
        />
      </label>
    </section>
  );
}

function StudentsPanel({
  data,
  loading,
  dailyLimit,
  search,
  status,
  onSearch,
  onStatus,
  onPage,
  onAccess,
}: {
  data: AIStudentListResponse;
  loading: boolean;
  dailyLimit: number;
  search: string;
  status: AIStudentStatusFilter;
  onSearch: (value: string) => void;
  onStatus: (value: AIStudentStatusFilter) => void;
  onPage: (value: number) => void;
  onAccess: (student: AIStudentItem) => void;
}) {
  return (
    <section className="rounded-lg border border-surface-200 bg-white dark:border-surface-700 dark:bg-surface-900">
      <div className="flex flex-col gap-3 border-b border-surface-200 p-4 dark:border-surface-700 md:flex-row md:items-center md:justify-between">
        <div className="relative w-full md:max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
          <Input
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            placeholder="搜索学生、邮箱"
            aria-label="搜索学生"
            className="pl-9"
          />
        </div>
        <Select
          value={status}
          onChange={(value) => onStatus(value as AIStudentStatusFilter)}
          aria-label="筛选学生 AI 状态"
          className="w-full md:w-44"
          options={[
            { value: 'all', label: '全部状态' },
            { value: 'active', label: 'AI 可用' },
            { value: 'blocked', label: 'AI 已封禁' },
            { value: 'quota_exhausted', label: '额度已用完' },
          ]}
        />
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-[920px] table-fixed text-left text-sm">
          <thead className="bg-surface-50 text-xs uppercase text-surface-500 dark:bg-surface-800/70 dark:text-surface-400">
            <tr>
              <th className="w-[24%] px-4 py-3 font-medium">学生</th>
              <th className="w-[15%] px-4 py-3 font-medium">AI 权限</th>
              <th className="w-[31%] px-4 py-3 font-medium">今日回复额度</th>
              <th className="w-[20%] px-4 py-3 font-medium">最近 AI 回复</th>
              <th className="w-[10%] px-4 py-3 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-200 dark:divide-surface-700">
            {loading ? (
              <TableMessage colSpan={5} text="加载学生状态中..." />
            ) : data.items.length === 0 ? (
              <TableMessage colSpan={5} text="暂无匹配学生" />
            ) : data.items.map((student) => (
              <tr key={student.id} className="hover:bg-surface-50 dark:hover:bg-surface-800/50">
                <td className="px-4 py-4">
                  <div className="font-medium text-surface-950 dark:text-surface-100">{student.display_name || student.username}</div>
                  <div className="mt-0.5 truncate text-xs text-surface-500 dark:text-surface-400">{student.username} · {student.email}</div>
                </td>
                <td className="px-4 py-4">
                  <Badge variant={student.ai_blocked ? 'destructive' : 'success'}>
                    {student.ai_blocked ? '已封禁' : '可用'}
                  </Badge>
                  {student.ai_blocked && student.blocked_reason && (
                    <div className="mt-1 truncate text-xs text-red-600 dark:text-red-300" title={student.blocked_reason}>{student.blocked_reason}</div>
                  )}
                </td>
                <td className="px-4 py-4">
                  <div className="mb-2 flex items-center justify-between text-xs">
                    <span className="font-medium text-surface-700 dark:text-surface-300">{student.replies_used} / {dailyLimit}</span>
                    <span className={student.quota_exhausted ? 'text-red-600 dark:text-red-300' : 'text-surface-500 dark:text-surface-400'}>
                      剩余 {student.replies_remaining}
                    </span>
                  </div>
                  <Progress
                    value={student.replies_used}
                    max={Math.max(dailyLimit, 1)}
                    size="sm"
                    variant={student.quota_exhausted ? 'destructive' : student.replies_used / Math.max(dailyLimit, 1) >= 0.8 ? 'warning' : 'default'}
                  />
                </td>
                <td className="px-4 py-4 text-surface-600 dark:text-surface-300">{formatDateTime(student.last_ai_reply_at)}</td>
                <td className="px-4 py-4 text-right">
                  <Button
                    variant={student.ai_blocked ? 'ghost' : 'destructive'}
                    size="icon"
                    className="h-9 w-9"
                    onClick={() => onAccess(student)}
                    aria-label={`${student.ai_blocked ? '解除' : '封禁'} ${student.username} 的 AI 权限`}
                    title={`${student.ai_blocked ? '解除' : '封禁'} AI 权限`}
                  >
                    {student.ai_blocked ? <UnlockKeyhole className="h-4 w-4" /> : <LockKeyhole className="h-4 w-4" />}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <PaginationFooter page={data.page} totalPages={data.total_pages} total={data.total} onPage={onPage} />
    </section>
  );
}

function EventsPanel({
  data,
  loading,
  search,
  eventType,
  onSearch,
  onEventType,
  onPage,
}: {
  data: AIRiskEventListResponse;
  loading: boolean;
  search: string;
  eventType: AIRiskEventType | 'all';
  onSearch: (value: string) => void;
  onEventType: (value: AIRiskEventType | 'all') => void;
  onPage: (value: number) => void;
}) {
  return (
    <section className="rounded-lg border border-surface-200 bg-white dark:border-surface-700 dark:bg-surface-900">
      <div className="flex flex-col gap-3 border-b border-surface-200 p-4 dark:border-surface-700 md:flex-row md:items-center md:justify-between">
        <div className="relative w-full md:max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
          <Input
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            placeholder="搜索学生、规则或内容"
            aria-label="搜索风险事件"
            className="pl-9"
          />
        </div>
        <Select
          value={eventType}
          onChange={(value) => onEventType(value as AIRiskEventType | 'all')}
          aria-label="筛选风险事件类型"
          className="w-full md:w-48"
          options={[
            { value: 'all', label: '全部事件' },
            { value: 'content_blocked', label: '内容拦截' },
            { value: 'admin_blocked', label: '管理员封禁' },
            { value: 'admin_unblocked', label: '管理员解封' },
          ]}
        />
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-[980px] table-fixed text-left text-sm">
          <thead className="bg-surface-50 text-xs uppercase text-surface-500 dark:bg-surface-800/70 dark:text-surface-400">
            <tr>
              <th className="w-[18%] px-4 py-3 font-medium">时间</th>
              <th className="w-[17%] px-4 py-3 font-medium">学生</th>
              <th className="w-[15%] px-4 py-3 font-medium">事件</th>
              <th className="w-[18%] px-4 py-3 font-medium">命中规则</th>
              <th className="w-[32%] px-4 py-3 font-medium">记录</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-200 dark:divide-surface-700">
            {loading ? (
              <TableMessage colSpan={5} text="加载风险事件中..." />
            ) : data.items.length === 0 ? (
              <TableMessage colSpan={5} text="暂无风险事件" />
            ) : data.items.map((event) => (
              <tr key={event.id} className="hover:bg-surface-50 dark:hover:bg-surface-800/50">
                <td className="px-4 py-4 text-surface-600 dark:text-surface-300">{formatDateTime(event.created_at)}</td>
                <td className="px-4 py-4 font-medium text-surface-900 dark:text-surface-100">{event.student_username || '已删除学生'}</td>
                <td className="px-4 py-4"><EventBadge event={event} /></td>
                <td className="px-4 py-4 text-surface-700 dark:text-surface-300">{event.matched_rule || '-'}</td>
                <td className="px-4 py-4">
                  <div className="line-clamp-2 text-surface-600 dark:text-surface-300" title={event.content_excerpt}>{event.content_excerpt || '-'}</div>
                  <div className="mt-1 text-xs text-surface-400">{sourceLabel(event.source)}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <PaginationFooter page={data.page} totalPages={data.total_pages} total={data.total} onPage={onPage} />
    </section>
  );
}

function EventBadge({ event }: { event: AIRiskEvent }) {
  const config: Record<AIRiskEventType, { label: string; variant: 'destructive' | 'warning' | 'success' }> = {
    content_blocked: { label: '内容拦截', variant: 'destructive' },
    admin_blocked: { label: '管理员封禁', variant: 'warning' },
    admin_unblocked: { label: '管理员解封', variant: 'success' },
  };
  const current = config[event.event_type];
  return <Badge variant={current.variant}>{current.label}</Badge>;
}

function PaginationFooter({
  page,
  totalPages,
  total,
  onPage,
}: {
  page: number;
  totalPages: number;
  total: number;
  onPage: (value: number) => void;
}) {
  if (total === 0) return null;
  return (
    <div className="flex flex-col gap-3 border-t border-surface-200 px-4 py-3 text-sm dark:border-surface-700 sm:flex-row sm:items-center sm:justify-between">
      <span className="text-surface-500 dark:text-surface-400">共 {total.toLocaleString('zh-CN')} 条</span>
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>上一页</Button>
        <span className="min-w-20 text-center text-surface-600 dark:text-surface-300">{page} / {Math.max(totalPages, 1)}</span>
        <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => onPage(page + 1)}>下一页</Button>
      </div>
    </div>
  );
}

function TableMessage({ colSpan, text }: { colSpan: number; text: string }) {
  return (
    <tr>
      <td colSpan={colSpan} className="h-32 px-4 text-center text-surface-500 dark:text-surface-400">{text}</td>
    </tr>
  );
}

function AccessDialog({
  student,
  reason,
  loading,
  onReason,
  onClose,
  onConfirm,
}: {
  student: AIStudentItem | null;
  reason: string;
  loading: boolean;
  onReason: (value: string) => void;
  onClose: () => void;
  onConfirm: () => void;
}) {
  if (!student) return null;
  const blocking = !student.ai_blocked;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-surface-950/60 p-4" role="presentation" onMouseDown={onClose}>
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="access-dialog-title"
        className="w-full max-w-md rounded-lg border border-surface-200 bg-white p-5 shadow-2xl dark:border-surface-700 dark:bg-surface-900"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id="access-dialog-title" className="text-lg font-semibold text-surface-950 dark:text-surface-50">
              {blocking ? '封禁学生 AI 权限' : '解除学生 AI 封禁'}
            </h2>
            <p className="mt-1 text-sm text-surface-500 dark:text-surface-400">{student.display_name || student.username} · {student.username}</p>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose} aria-label="关闭权限操作" title="关闭">
            <X className="h-4 w-4" />
          </Button>
        </div>

        {blocking ? (
          <label className="mt-5 block space-y-2" htmlFor="access-reason">
            <span className="text-sm font-medium text-surface-800 dark:text-surface-200">封禁原因</span>
            <textarea
              id="access-reason"
              value={reason}
              onChange={(event) => onReason(event.target.value)}
              rows={4}
              maxLength={500}
              autoFocus
              className="w-full resize-none rounded-md border border-surface-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-red-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
            />
          </label>
        ) : (
          <div className="mt-5 flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200">
            <CheckCircle2 className="h-5 w-5 shrink-0" />
            <span>恢复该学生的 AI 请求权限。</span>
          </div>
        )}

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} disabled={loading}>取消</Button>
          <Button
            variant={blocking ? 'destructive' : 'primary'}
            onClick={onConfirm}
            isLoading={loading}
            disabled={blocking && !reason.trim()}
          >
            {blocking ? <LockKeyhole className="mr-2 h-4 w-4" /> : <UnlockKeyhole className="mr-2 h-4 w-4" />}
            {blocking ? '确认封禁' : '解除封禁'}
          </Button>
        </div>
      </section>
    </div>
  );
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function sourceLabel(source: string): string {
  const labels: Record<string, string> = {
    session_chat: '学习会话',
    exercise_submit: '答案判定',
    admin_risk_center: '管理员操作',
  };
  return labels[source] ?? source;
}

export default AIRiskControlPage;
