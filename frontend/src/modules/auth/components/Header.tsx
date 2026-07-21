import React from 'react';
import { Link } from 'react-router-dom';
import { useAppSelector } from '@/store';
import { Button } from '@/components/ui/Button';
import { ThemeToggle } from '@/components/ui/ThemeToggle';
import { cn } from '@/libs/utils/cn';
import { Loader2, LogOut, User, Users } from 'lucide-react';
import { selectIsAuthenticated, selectCurrentUser } from '@/modules/auth/store/authSlice';
import { getNavItemsByRole } from '@/modules/auth/constants/navigationConfig';
import { useAuth } from '@/modules/auth/hooks/useAuth';
import { animationCombos } from '@/libs/animations';
import { ResponsiveNavigation } from '@/modules/auth/components/ResponsiveNavigation';

interface HeaderProps {
  variant?: 'default' | 'transparent' | 'dark';
  onLoginClick?: () => void;
  onRegisterClick?: () => void;
}

/**
 * Header - 应用头部组件
 *
 * 设计原则：
 * - 单一职责: 只负责展示头部 UI，业务逻辑通过 hooks 获取
 * - DRY: 导航配置和认证逻辑已提取到独立模块
 * - SOLID: 依赖抽象（hooks），而非具体实现
 */
export const Header: React.FC<HeaderProps> = ({ variant = 'default', onLoginClick, onRegisterClick }) => {
  const isAuthenticated = useAppSelector(selectIsAuthenticated);
  const user = useAppSelector(selectCurrentUser);
  const { handleLogin, handleRegister, handleLogout, isLoggingOut } = useAuth();

  // 真正的登录状态：token 存在且 user 信息已加载
  const isLoggedIn = isAuthenticated && user !== null;

  // 获取导航菜单配置
  const navItems = getNavItemsByRole(user?.role);
  const isTeacher = user?.role === 'teacher';

  const isDark = variant === 'dark' || variant === 'transparent';

  return (
    <header className={cn(
      "top-0 z-50 w-full border-b",
      animationCombos.buttonHover,
      variant === 'default' && "sticky border-surface-200/60 bg-white/80 backdrop-blur-md supports-backdrop-filter:bg-white/60 dark:border-surface-700/60 dark:bg-surface-900/80 dark:supports-backdrop-filter:bg-surface-900/60",
      variant === 'transparent' && "fixed border-transparent bg-transparent backdrop-blur-sm",
      variant === 'dark' && "sticky border-surface-800 bg-surface-950/80 backdrop-blur-md"
    )}>
      <div className="container mx-auto flex h-16 items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
        {/* Logo */}
        <Link to={isLoggedIn ? (isTeacher ? '/teacher/dashboard' : '/my-class') : '/'} className="group flex shrink-0 items-center space-x-2">
          <div className={cn(
            "h-8 w-8 rounded-lg flex items-center justify-center text-white shadow-lg",
            animationCombos.buttonHover,
            isTeacher
              ? "bg-linear-to-br from-emerald-500 to-teal-600 group-hover:shadow-emerald-500/30"
              : "bg-linear-to-br from-primary-500 to-secondary-600 group-hover:shadow-primary-500/30"
          )}>
            <span className="font-bold text-lg">M</span>
          </div>
          <span className={cn(
            "whitespace-nowrap font-bold text-xl bg-clip-text text-transparent bg-linear-to-r",
            isDark ? "from-white to-surface-400" : "from-surface-900 to-surface-600 dark:from-white dark:to-surface-400"
          )}>
            高数智学
          </span>
          {/* 角色标识 */}
          {isLoggedIn && (
            <span className={cn(
              "ml-1 shrink-0 whitespace-nowrap px-2 py-0.5 text-xs font-medium rounded-full",
              isTeacher
                ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-400"
                : "bg-primary-100 text-primary-700 dark:bg-primary-900/50 dark:text-primary-400"
            )}>
              {isTeacher ? '教师端' : '学生端'}
            </span>
          )}
        </Link>

        {/* Desktop Navigation - Only show when authenticated */}
        {isLoggedIn && (
          <ResponsiveNavigation items={navItems} isTeacher={isTeacher} />
        )}

        {/* User Actions */}
        <div className="flex shrink-0 items-center space-x-3">
          {/* 主题切换按钮 */}
          <ThemeToggle variant="ghost" />

          {isLoggedIn ? (
            <div className="flex items-center space-x-3">
              <span className={cn("hidden sm:inline-block text-sm font-medium", isDark ? "text-surface-300" : "text-surface-700 dark:text-surface-300")}>
                你好，{user?.name || (isTeacher ? '老师' : '同学')}
              </span>
              <div className={cn(
                "h-9 w-9 rounded-full border shadow-sm flex items-center justify-center font-bold text-sm cursor-pointer transition-all group relative",
                isTeacher
                  ? "bg-linear-to-tr from-emerald-100 to-teal-100 dark:from-emerald-900 dark:to-teal-900 border-white dark:border-surface-700 text-emerald-700 dark:text-emerald-300 hover:ring-2 hover:ring-emerald-200 dark:hover:ring-emerald-700"
                  : "bg-linear-to-tr from-primary-100 to-secondary-100 dark:from-primary-900 dark:to-secondary-900 border-white dark:border-surface-700 text-primary-700 dark:text-primary-300 hover:ring-2 hover:ring-primary-200 dark:hover:ring-primary-700"
              )}>
                <User className="w-5 h-5" />
                {/* Quick Logout Dropdown (Simulated) */}
                <div className="absolute top-full right-0 mt-2 w-36 py-2 bg-white dark:bg-surface-800 rounded-lg shadow-lg border border-surface-100 dark:border-surface-700 opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 transform origin-top-right">
                  {!isTeacher && (
                    <Link
                      to="/my-class"
                      className="w-full px-4 py-2 text-left text-sm text-surface-700 dark:text-surface-300 hover:bg-surface-50 dark:hover:bg-surface-700 flex items-center"
                    >
                      <Users className="w-4 h-4 mr-2" />
                      我的班级
                    </Link>
                  )}
                  <Link
                    to={isTeacher ? "/teacher/profile" : "/profile"}
                    className="w-full px-4 py-2 text-left text-sm text-surface-700 dark:text-surface-300 hover:bg-surface-50 dark:hover:bg-surface-700 flex items-center"
                  >
                    <User className="w-4 h-4 mr-2" />
                    个人中心
                  </Link>
                  <button
                    onClick={handleLogout}
                    type="button"
                    disabled={isLoggingOut}
                    className="w-full px-4 py-2 text-left text-sm text-surface-700 dark:text-surface-300 hover:bg-surface-50 dark:hover:bg-surface-700 flex items-center"
                  >
                    {isLoggingOut ? (
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    ) : (
                      <LogOut className="w-4 h-4 mr-2" />
                    )}
                    {isLoggingOut ? '退出中...' : '退出登录'}
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex items-center space-x-2">
              <Button variant="ghost" size="sm" onClick={() => handleLogin(onLoginClick)} className={cn(isDark && "text-surface-300 hover:bg-white/10 hover:text-white")}>登录</Button>
              <Button variant="primary" size="sm" onClick={() => handleRegister(onRegisterClick, onLoginClick)}>注册</Button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
