import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminLoginPage } from './AdminLoginPage';

const mocks = vi.hoisted(() => ({
  adminLogin: vi.fn(),
  dispatch: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock('react-redux', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-redux')>();
  return { ...actual, useDispatch: () => mocks.dispatch };
});

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return { ...actual, useNavigate: () => mocks.navigate };
});

vi.mock('@/modules/auth/services/authService', () => ({
  authService: { adminLogin: mocks.adminLogin },
}));

vi.mock('@/modules/auth/components/SliderCaptcha', () => ({
  SliderCaptcha: ({ onTokenChange }: { onTokenChange: (token: string) => void }) => (
    <button type="button" onClick={() => onTokenChange('admin-proof')}>完成安全验证</button>
  ),
}));

vi.mock('../../components/ui/ThemeToggle', () => ({ ThemeToggle: () => null }));

describe('AdminLoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('requires captcha before sending administrator credentials', async () => {
    const user = userEvent.setup();
    render(<AdminLoginPage />);
    await user.type(screen.getByPlaceholderText('请输入管理员账号'), 'admin');
    await user.type(screen.getByPlaceholderText('请输入密码'), 'secret');

    await user.click(screen.getByRole('button', { name: '登录管理后台' }));

    expect(await screen.findByText('请先完成安全验证')).toBeInTheDocument();
    expect(mocks.adminLogin).not.toHaveBeenCalled();
  });

  it('submits a captcha proof and enters the admin dashboard', async () => {
    const user = userEvent.setup();
    mocks.adminLogin.mockResolvedValue({
      access_token: 'admin-token',
      token_type: 'bearer',
      user: { id: 'admin-1', username: 'admin', email: 'admin@example.com', role: 'admin' },
    });
    render(<AdminLoginPage />);
    await user.type(screen.getByPlaceholderText('请输入管理员账号'), 'admin');
    await user.type(screen.getByPlaceholderText('请输入密码'), 'secret');
    await user.click(screen.getByRole('button', { name: '完成安全验证' }));
    await user.click(screen.getByRole('button', { name: '登录管理后台' }));

    await waitFor(() => expect(mocks.adminLogin).toHaveBeenCalledWith({
      username: 'admin',
      password: 'secret',
      captchaToken: 'admin-proof',
    }));
    expect(mocks.dispatch).toHaveBeenCalled();
    expect(mocks.navigate).toHaveBeenCalledWith('/admin/dashboard');
  });

  it('does not enter the dashboard when the server returns a non-admin account', async () => {
    const user = userEvent.setup();
    mocks.adminLogin.mockResolvedValue({
      access_token: 'teacher-token',
      token_type: 'bearer',
      user: { id: 'teacher-1', username: 'teacher', email: 'teacher@example.com', role: 'teacher' },
    });
    render(<AdminLoginPage />);
    await user.type(screen.getByPlaceholderText('请输入管理员账号'), 'teacher');
    await user.type(screen.getByPlaceholderText('请输入密码'), 'secret');
    await user.click(screen.getByRole('button', { name: '完成安全验证' }));
    await user.click(screen.getByRole('button', { name: '登录管理后台' }));

    expect(await screen.findByText('该账户不是管理员，请使用对应身份的登录入口')).toBeInTheDocument();
    expect(mocks.dispatch).not.toHaveBeenCalled();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });
});
