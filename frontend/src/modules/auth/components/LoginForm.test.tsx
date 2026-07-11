import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LoginForm } from './LoginForm';

const mocks = vi.hoisted(() => ({
  dispatch: vi.fn(),
  login: vi.fn(),
  navigate: vi.fn(),
  loggerInfo: vi.fn(),
  loggerSecurity: vi.fn(),
}));

vi.mock('@/store', () => ({
  useAppDispatch: () => mocks.dispatch,
}));

vi.mock('@/modules/auth/services/authService', () => ({
  authService: {
    login: mocks.login,
  },
}));

vi.mock('@/libs/utils/logger', () => ({
  logger: {
    createContextLogger: () => ({
      info: mocks.loggerInfo,
      security: mocks.loggerSecurity,
    }),
  },
}));

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useNavigate: () => mocks.navigate,
  };
});

vi.mock('./ForgotPasswordModal', () => ({
  ForgotPasswordModal: ({ isOpen }: { isOpen: boolean }) => (
    isOpen ? <div role="dialog" aria-label="找回密码">找回密码</div> : null
  ),
}));

function renderLoginForm(props: { onSuccess?: () => void; onSwitchToRegister?: () => void } = {}) {
  return render(
    <MemoryRouter>
      <LoginForm {...props} />
    </MemoryRouter>
  );
}

async function fillCredentials(username = 'alice', password = 'secret') {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText('用户名'), username);
  await user.type(screen.getByLabelText('密码'), password);
  return user;
}

describe('LoginForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps full-screen gaze tracking when fields receive focus', async () => {
    const user = userEvent.setup();
    renderLoginForm();

    const scene = screen.getByTestId('animated-login-characters');
    expect(scene).toHaveAttribute('data-gaze', 'tracking');
    expect(screen.getByRole('button', { name: '学生登录' })).toBeInTheDocument();

    await user.click(screen.getByLabelText('用户名'));
    expect(scene).toHaveAttribute('data-gaze', 'tracking');

    await user.click(screen.getByLabelText('密码'));
    expect(scene).toHaveAttribute('data-gaze', 'tracking');
  });

  it('toggles password visibility and only then averts the mascots gaze', async () => {
    const user = userEvent.setup();
    renderLoginForm();

    const passwordInput = screen.getByLabelText('密码');
    const scene = screen.getByTestId('animated-login-characters');
    const showPasswordButton = screen.getByRole('button', { name: '显示密码' });

    expect(passwordInput).toHaveAttribute('type', 'password');
    expect(showPasswordButton).toHaveAttribute('aria-pressed', 'false');
    expect(scene).toHaveAttribute('data-gaze', 'tracking');

    await user.click(showPasswordButton);

    const hidePasswordButton = screen.getByRole('button', { name: '隐藏密码' });
    expect(passwordInput).toHaveAttribute('type', 'text');
    expect(hidePasswordButton).toHaveAttribute('aria-pressed', 'true');
    expect(scene).toHaveAttribute('data-gaze', 'averted');

    await user.click(hidePasswordButton);
    expect(passwordInput).toHaveAttribute('type', 'password');
    expect(screen.getByRole('button', { name: '显示密码' })).toHaveAttribute('aria-pressed', 'false');
    expect(scene).toHaveAttribute('data-gaze', 'tracking');
  });

  it('changes the submit label when the teacher role is selected', async () => {
    const user = userEvent.setup();
    renderLoginForm();

    await user.click(screen.getByRole('button', { name: '教师 进入教学管理' }));

    expect(screen.getByRole('button', { name: '教师登录' })).toBeInTheDocument();
  });

  it('keeps validation local and does not call the login service for empty fields', async () => {
    const user = userEvent.setup();
    renderLoginForm();

    await user.click(screen.getByRole('button', { name: '学生登录' }));

    expect(await screen.findByText('用户名至少需要 3 个字符')).toBeInTheDocument();
    expect(screen.getByText('请输入密码')).toBeInTheDocument();
    expect(mocks.login).not.toHaveBeenCalled();
    expect(screen.getByTestId('animated-login-characters')).toHaveAttribute('data-gaze', 'tracking');
  });

  it('submits valid student credentials and preserves the success route', async () => {
    const onSuccess = vi.fn();
    mocks.login.mockResolvedValue({
      token: 'student-token',
      user: { id: 'student-1', name: 'Alice', role: 'student' },
    });
    renderLoginForm({ onSuccess });
    const user = await fillCredentials();

    await user.click(screen.getByRole('button', { name: '显示密码' }));
    expect(screen.getByLabelText('密码')).toHaveAttribute('type', 'text');

    await user.click(screen.getByRole('button', { name: '学生登录' }));

    await waitFor(() => {
      expect(mocks.login).toHaveBeenCalledWith({
        username: 'alice',
        password: 'secret',
        role: 'student',
      });
    });
    expect(mocks.dispatch).toHaveBeenCalledWith(expect.objectContaining({
      payload: {
        token: 'student-token',
        user: { id: 'student-1', name: 'Alice', role: 'student' },
      },
    }));
    expect(onSuccess).toHaveBeenCalledOnce();
    expect(mocks.navigate).toHaveBeenCalledWith('/course/overview');
  });

  it('preserves the teacher route and selected role in the request', async () => {
    mocks.login.mockResolvedValue({
      token: 'teacher-token',
      user: { id: 'teacher-1', name: 'Taylor', role: 'teacher' },
    });
    renderLoginForm();
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: '教师 进入教学管理' }));
    await user.type(screen.getByLabelText('用户名'), 'taylor');
    await user.type(screen.getByLabelText('密码'), 'secret');
    await user.click(screen.getByRole('button', { name: '教师登录' }));

    await waitFor(() => expect(mocks.navigate).toHaveBeenCalledWith('/teacher/dashboard'));
    expect(mocks.login).toHaveBeenCalledWith(expect.objectContaining({ role: 'teacher' }));
  });

  it('keeps role mismatch errors inside the form', async () => {
    mocks.login.mockResolvedValue({
      token: 'teacher-token',
      user: { id: 'teacher-1', name: 'Taylor', role: 'teacher' },
    });
    renderLoginForm();
    const user = await fillCredentials();

    await user.click(screen.getByRole('button', { name: '学生登录' }));

    expect(await screen.findByText('您已注册为教师，请选择「教师」身份或使用教师入口登录')).toBeInTheDocument();
    expect(mocks.dispatch).not.toHaveBeenCalled();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it('shows the existing generic error when the request fails', async () => {
    mocks.login.mockRejectedValue(new Error('network unavailable'));
    renderLoginForm();
    const user = await fillCredentials();

    await user.click(screen.getByRole('button', { name: '学生登录' }));

    expect(await screen.findByText('登录失败，请检查用户名和密码')).toBeInTheDocument();
    expect(mocks.loggerSecurity).toHaveBeenCalled();
  });

  it('preserves forgot-password and register callbacks', async () => {
    const user = userEvent.setup();
    const onSwitchToRegister = vi.fn();
    renderLoginForm({ onSwitchToRegister });

    await user.click(screen.getByRole('button', { name: '忘记密码？' }));
    expect(screen.getByRole('dialog', { name: '找回密码' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '立即注册' }));
    expect(onSwitchToRegister).toHaveBeenCalledOnce();
  });
});
