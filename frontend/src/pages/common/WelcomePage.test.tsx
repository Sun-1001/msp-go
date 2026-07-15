import type { ReactNode } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WelcomePage } from './WelcomePage';

const mocks = vi.hoisted(() => ({
  reduceMotion: false,
}));

vi.mock('framer-motion', async (importOriginal) => {
  const actual = await importOriginal<typeof import('framer-motion')>();
  return {
    ...actual,
    useReducedMotion: () => mocks.reduceMotion,
    useScroll: () => ({ scrollYProgress: actual.motionValue(0) }),
    useSpring: (source: import('framer-motion').MotionValue<number>) => source,
  };
});

vi.mock('@/components/layout/MainLayout', () => ({
  MainLayout: ({
    children,
    onLoginClick,
    onRegisterClick,
  }: {
    children: ReactNode;
    onLoginClick?: () => void;
    onRegisterClick?: () => void;
  }) => (
    <div>
      <header>
        <button type="button" onClick={onLoginClick}>页头登录</button>
        <button type="button" onClick={onRegisterClick}>页头注册</button>
      </header>
      <main>{children}</main>
    </div>
  ),
}));

vi.mock('@/components/ui/Modal', () => ({
  Modal: ({
    isOpen,
    onClose,
    ariaLabel,
    className,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    ariaLabel: string;
    className?: string;
    children: ReactNode;
  }) => isOpen ? (
    <div role="dialog" aria-label={ariaLabel} className={className}>
      <button type="button" onClick={onClose}>关闭弹窗</button>
      {children}
    </div>
  ) : null,
}));

vi.mock('@/modules/auth', () => ({
  LoginForm: ({ onSwitchToRegister }: { onSwitchToRegister: () => void }) => (
    <div>
      <span>登录表单</span>
      <button type="button" onClick={onSwitchToRegister}>切换到注册</button>
    </div>
  ),
  RegisterForm: ({ onSwitchToLogin }: { onSwitchToLogin: () => void }) => (
    <div>
      <span>注册表单</span>
      <button type="button" onClick={onSwitchToLogin}>切换到登录</button>
    </div>
  ),
}));

describe('WelcomePage', () => {
  const scrollTo = vi.fn();
  const scrollIntoView = vi.fn();

  class IntersectionObserverMock {
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.reduceMotion = false;
    Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo });
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });
    vi.stubGlobal('IntersectionObserver', IntersectionObserverMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders the product story and keeps the project subject coverage', () => {
    render(<WelcomePage />);

    expect(screen.getByRole('heading', { level: 1, name: '高数智学' })).toBeInTheDocument();
    expect(screen.getByTestId('welcome-title')).toHaveAttribute('data-title-motion', 'animated');
    expect(screen.getByRole('heading', { name: /从问题，\s*到真正掌握。/ })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '一套学习闭环，连接学生与教师。' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '从课前到复习，学习始终有下一步。' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '大学数学核心课程' })).toBeInTheDocument();
    expect(screen.getByText('微积分')).toBeInTheDocument();
    expect(screen.getByText('线性代数')).toBeInTheDocument();
    expect(screen.getByText('课前')).toBeInTheDocument();
    expect(screen.getByText('学习中')).toBeInTheDocument();
    expect(screen.getByText('复习')).toBeInTheDocument();
    expect(screen.queryByText('98%')).not.toBeInTheDocument();
  });

  it('switches between the student and teacher product views', () => {
    render(<WelcomePage />);

    const studentTab = screen.getByRole('tab', { name: '学生端' });
    const teacherTab = screen.getByRole('tab', { name: '教师端' });

    expect(studentTab).toHaveAttribute('aria-selected', 'true');
    expect(teacherTab).toHaveAttribute('aria-selected', 'false');
    expect(screen.getByRole('heading', { name: '把“不会做”，变成知道下一步怎么做。' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /学生端从知识回顾/ })).toBeInTheDocument();

    fireEvent.click(teacherTab);

    expect(studentTab).toHaveAttribute('aria-selected', 'false');
    expect(teacherTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('heading', { name: '先看清共性问题，再安排下一次教学。' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /教师端汇总班级知识点掌握/ })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: '把“不会做”，变成知道下一步怎么做。' })).not.toBeInTheDocument();
  });

  it('opens login and register flows from the welcome surface', () => {
    render(<WelcomePage />);

    fireEvent.click(screen.getByRole('button', { name: '开始学习' }));
    const loginDialog = screen.getByRole('dialog', { name: '登录' });
    const authDialogClassName = loginDialog.className;
    expect(loginDialog).toHaveClass('max-w-[600px]', 'overflow-hidden', 'p-0', 'lg:max-w-[1000px]');
    expect(screen.getByText('登录表单')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '切换到注册' }));
    expect(screen.getByRole('dialog', { name: '创建账号' })).toHaveAttribute(
      'class',
      authDialogClassName
    );
    expect(screen.getByText('注册表单')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '关闭弹窗' }));
    fireEvent.click(screen.getByRole('button', { name: '页头注册' }));
    expect(screen.getByRole('dialog', { name: '创建账号' })).toBeInTheDocument();
  });

  it('moves to the scroll-driven product demo when requested', () => {
    render(<WelcomePage />);

    fireEvent.click(screen.getByRole('button', { name: '查看产品演示' }));

    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ behavior: 'smooth' }));
  });

  it('renders all story stages without sticky motion when reduced motion is preferred', () => {
    mocks.reduceMotion = true;
    render(<WelcomePage />);

    expect(screen.getByTestId('welcome-story')).toHaveAttribute('data-reduced-motion', 'true');
    expect(screen.getByTestId('welcome-title')).toHaveAttribute('data-title-motion', 'static');
    expect(screen.queryByRole('navigation', { name: '产品演示阶段' })).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: /AI 助教逐步解析/ })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /展示函数极值相关知识关系/ })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /展示本周学习路径/ })).toBeInTheDocument();
  });
});
