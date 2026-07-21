import { MemoryRouter } from 'react-router-dom';
import { render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ProfilePage } from './ProfilePage';

const mocks = vi.hoisted(() => ({
  getBindingStatus: vi.fn(),
  user: {
    id: 'user-1',
    name: 'Alice',
    email: 'alice@example.com',
    role: 'student' as const,
  },
}));

vi.mock('@/store', () => ({
  useAppSelector: () => mocks.user,
}));

vi.mock('@/modules/auth/store/authSlice', () => ({
  selectCurrentUser: vi.fn(),
}));

vi.mock('@/modules/xidian/services/xidianService', () => ({
  xidianService: {
    getBindingStatus: mocks.getBindingStatus,
  },
}));

describe('ProfilePage account boundaries', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBindingStatus.mockResolvedValue({ is_bound: false });
  });

  it('shows the registered email without unsupported binding controls', async () => {
    render(
      <MemoryRouter>
        <ProfilePage />
      </MemoryRouter>,
    );

    await waitFor(() => expect(mocks.getBindingStatus).toHaveBeenCalledTimes(1));
    const emailSection = screen.getByLabelText('注册邮箱');
    expect(within(emailSection).getByText('alice@example.com')).toBeInTheDocument();
    expect(within(emailSection).queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByText('未验证')).not.toBeInTheDocument();
    expect(screen.queryByText('手机号码')).not.toBeInTheDocument();
  });

  it('keeps only account binding controls for a verified Xidian account', async () => {
    mocks.getBindingStatus.mockResolvedValue({
      is_bound: true,
      username: '20260001',
      last_verified_at: '2026-07-21T08:00:00Z',
    });

    render(
      <MemoryRouter>
        <ProfilePage />
      </MemoryRouter>,
    );

    expect(await screen.findByText('已绑定（20260001）')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '解绑' })).toBeInTheDocument();
    expect(screen.getByText(/不会保存西电密码或教务会话/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /同步/ })).not.toBeInTheDocument();
  });
});
