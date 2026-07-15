import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { RegisterForm } from './RegisterForm';

const mocks = vi.hoisted(() => ({
  getRegistrationStatus: vi.fn(),
  register: vi.fn(),
}));

vi.mock('@/modules/admin/services/systemSettingService', () => ({
  systemSettingService: {
    getRegistrationStatus: mocks.getRegistrationStatus,
  },
}));

vi.mock('@/modules/auth/services/authService', () => ({
  authService: {
    register: mocks.register,
  },
}));

function renderRegisterForm(props: { onSwitchToLogin?: () => void } = {}) {
  return render(
    <MemoryRouter>
      <RegisterForm {...props} />
    </MemoryRouter>
  );
}

async function fillRegistrationForm() {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText('用户名'), 'alice');
  await user.type(screen.getByLabelText('邮箱'), 'alice@example.com');
  await user.type(screen.getByLabelText('密码'), 'Strong1!');
  await user.type(screen.getByLabelText('确认密码'), 'Strong1!');
  return user;
}

describe('RegisterForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getRegistrationStatus.mockResolvedValue({
      allow_student: true,
      allow_teacher: true,
    });
    mocks.register.mockResolvedValue(undefined);
  });

  it('uses the login-aligned layout and toggles both password fields', async () => {
    const user = userEvent.setup();
    renderRegisterForm();

    await screen.findByLabelText('用户名');

    expect(screen.getByTestId('auth-form-layout')).toHaveClass(
      'lg:grid-cols-[minmax(390px,0.92fr)_minmax(440px,1.08fr)]'
    );
    expect(screen.getByRole('button', { name: '学生 我想学习数学知识' })).toHaveClass('min-h-16');

    const scene = screen.getByTestId('animated-login-characters');
    const passwordInput = screen.getByLabelText('密码');
    const confirmPasswordInput = screen.getByLabelText('确认密码');

    expect(scene).toHaveAttribute('data-gaze', 'tracking');
    expect(passwordInput).toHaveAttribute('type', 'password');
    expect(confirmPasswordInput).toHaveAttribute('type', 'password');

    await user.click(screen.getByRole('button', { name: '显示密码' }));
    expect(passwordInput).toHaveAttribute('type', 'text');
    expect(scene).toHaveAttribute('data-gaze', 'averted');

    await user.click(screen.getByRole('button', { name: '隐藏密码' }));
    await user.click(screen.getByRole('button', { name: '显示确认密码' }));
    expect(confirmPasswordInput).toHaveAttribute('type', 'text');
    expect(scene).toHaveAttribute('data-gaze', 'averted');
  });

  it('keeps validation local for an empty submission', async () => {
    const user = userEvent.setup();
    renderRegisterForm();

    await screen.findByLabelText('用户名');
    await user.click(screen.getByRole('button', { name: '注册' }));

    expect(await screen.findByText('用户名至少需要 3 个字符')).toBeInTheDocument();
    expect(screen.getByText('请输入邮箱地址')).toBeInTheDocument();
    expect(screen.getByText('密码至少需要 8 个字符')).toBeInTheDocument();
    expect(screen.getByText('请确认密码')).toBeInTheDocument();
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it('submits the supported fields and preserves the success handoff', async () => {
    const onSwitchToLogin = vi.fn();
    renderRegisterForm({ onSwitchToLogin });
    const user = await fillRegistrationForm();

    await user.click(screen.getByRole('button', { name: '显示密码' }));
    expect(screen.getByTestId('animated-login-characters')).toHaveAttribute('data-gaze', 'averted');

    await user.click(screen.getByRole('button', { name: '注册' }));

    await waitFor(() => {
      expect(mocks.register).toHaveBeenCalledWith({
        username: 'alice',
        email: 'alice@example.com',
        password: 'Strong1!',
        role: 'student',
      });
    });
    expect(await screen.findByRole('heading', { level: 1, name: '注册成功' })).toBeInTheDocument();
    expect(screen.getByText('邮箱 alice@example.com 已保存，可以直接登录。')).toBeInTheDocument();
    expect(screen.getByTestId('animated-login-characters')).toHaveAttribute('data-gaze', 'tracking');

    await user.click(screen.getByRole('button', { name: '立即登录' }));
    expect(onSwitchToLogin).toHaveBeenCalledOnce();
  });

  it('shows service errors inside the form', async () => {
    mocks.register.mockRejectedValue(new Error('该邮箱已注册'));
    renderRegisterForm();
    const user = await fillRegistrationForm();

    await user.click(screen.getByRole('button', { name: '注册' }));

    expect(await screen.findByText('该邮箱已注册')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: '创建账号' })).toBeInTheDocument();
  });

  it('disables a paused default role and allows switching to an open role', async () => {
    mocks.getRegistrationStatus.mockResolvedValue({
      allow_student: false,
      allow_teacher: true,
    });
    const user = userEvent.setup();
    renderRegisterForm();

    const usernameInput = await screen.findByLabelText('用户名');
    const studentRole = screen.getByRole('button', { name: '学生 暂停注册' });
    const teacherRole = screen.getByRole('button', { name: '教师 我想管理学生和课程' });

    expect(studentRole).toBeDisabled();
    expect(usernameInput).toBeDisabled();
    expect(screen.getByText('学生注册功能已暂停，请选择其他身份或稍后再试')).toBeInTheDocument();

    await user.click(teacherRole);

    expect(usernameInput).toBeEnabled();
    expect(screen.queryByText('学生注册功能已暂停，请选择其他身份或稍后再试')).not.toBeInTheDocument();
  });

  it('renders the closed-registration state inside the shared layout', async () => {
    const onSwitchToLogin = vi.fn();
    mocks.getRegistrationStatus.mockResolvedValue({
      allow_student: false,
      allow_teacher: false,
    });
    const user = userEvent.setup();
    renderRegisterForm({ onSwitchToLogin });

    expect(await screen.findByRole('heading', { name: '注册功能已暂停' })).toBeInTheDocument();
    expect(screen.getByTestId('auth-form-layout')).toBeInTheDocument();
    expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '立即登录' }));
    expect(onSwitchToLogin).toHaveBeenCalledOnce();
  });

  it('falls back to open registration when loading the setting fails', async () => {
    mocks.getRegistrationStatus.mockRejectedValue(new Error('network unavailable'));
    renderRegisterForm();

    expect(await screen.findByLabelText('用户名')).toBeEnabled();
    expect(screen.getByRole('button', { name: '学生 我想学习数学知识' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '教师 我想管理学生和课程' })).toBeEnabled();
  });
});
