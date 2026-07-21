import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AIRiskControlPage } from './AIRiskControlPage';

const mocks = vi.hoisted(() => ({
  getOverview: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listStudents: vi.fn(),
  updateStudentAccess: vi.fn(),
  listEvents: vi.fn(),
}));

vi.mock('@/modules/admin/components/AdminLayout', () => ({
  AdminLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock('@/modules/admin/services/aiRiskService', () => ({
  aiRiskService: mocks,
}));

describe('AIRiskControlPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getOverview.mockResolvedValue({
      total_students: 3,
      blocked_students: 1,
      quota_exhausted_students: 1,
      replies_today: 12,
      risk_events_today: 2,
      daily_reply_limit: 5,
      max_concurrent_requests: 2,
    });
    mocks.getSettings.mockResolvedValue({
      daily_reply_limit: 5,
      max_concurrent_requests: 2,
      blocked_keywords: ['代考'],
      reset_timezone: 'Asia/Shanghai',
      next_reset_at: '2026-07-22T00:00:00+08:00',
    });
    mocks.updateSettings.mockResolvedValue({
      daily_reply_limit: 8,
      max_concurrent_requests: 2,
      blocked_keywords: ['代考'],
      reset_timezone: 'Asia/Shanghai',
      next_reset_at: '2026-07-22T00:00:00+08:00',
    });
    mocks.listStudents.mockResolvedValue({
      items: [{
        id: 'student-1',
        username: 'alice',
        email: 'alice@example.com',
        display_name: 'Alice',
        ai_blocked: false,
        blocked_reason: '',
        blocked_at: null,
        replies_used: 4,
        replies_remaining: 1,
        quota_exhausted: false,
        last_ai_reply_at: '2026-07-21T04:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      total_pages: 1,
    });
    mocks.updateStudentAccess.mockResolvedValue({
      student_id: 'student-1',
      ai_blocked: true,
      blocked_reason: '违规',
      blocked_at: '2026-07-21T04:00:00Z',
    });
    mocks.listEvents.mockResolvedValue({
      items: [{
        id: 'event-1',
        student_id: 'student-1',
        student_username: 'alice',
        event_type: 'content_blocked',
        severity: 'critical',
        action: 'request_blocked',
        source: 'session_chat',
        matched_rule: '代考',
        content_excerpt: '请帮我代考',
        actor_id: null,
        created_at: '2026-07-21T04:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      total_pages: 1,
    });
  });

  it('edits uniform settings and controls a student', async () => {
    const user = userEvent.setup();
    render(<AIRiskControlPage />);

    expect(await screen.findByRole('heading', { name: '风控中心' })).toBeInTheDocument();
    expect(await screen.findByText('12')).toBeInTheDocument();

    const quotaInput = await screen.findByLabelText('每生每日 AI 回复额度');
    await user.clear(quotaInput);
    await user.type(quotaInput, '8');
    await user.click(screen.getByRole('button', { name: '保存策略' }));
    await waitFor(() => expect(mocks.updateSettings).toHaveBeenCalledWith(expect.objectContaining({ daily_reply_limit: 8 })));

    await user.click(screen.getByRole('tab', { name: '学生控制' }));
    expect(await screen.findByText('alice · alice@example.com')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '封禁 alice 的 AI 权限' }));
    await user.type(screen.getByLabelText('封禁原因'), '违规');
    await user.click(screen.getByRole('button', { name: '确认封禁' }));
    await waitFor(() => expect(mocks.updateStudentAccess).toHaveBeenCalledWith('student-1', { blocked: true, reason: '违规' }));
  });

  it('shows risk events', async () => {
    const user = userEvent.setup();
    render(<AIRiskControlPage />);
    await screen.findByRole('heading', { name: '风控中心' });

    await user.click(screen.getByRole('tab', { name: '风险事件' }));
    const excerpt = await screen.findByText('请帮我代考');
    expect(within(excerpt.closest('tr')!).getByText('内容拦截')).toBeInTheDocument();
  });
});
