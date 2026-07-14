/* eslint-disable react-refresh/only-export-components */
import React, { createContext, useContext, useState, useCallback, useEffect, useMemo, useRef } from 'react';
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-react';
import { cn } from '../../libs/utils/cn';
import { animationDuration } from '../../libs/animations';

/**
 * Toast 类型定义
 */
export type ToastType = 'success' | 'error' | 'info' | 'warning';

/**
 * Toast 样式配置（模块级常量，避免重复创建）
 */
const TOAST_ICONS: Record<ToastType, React.ReactNode> = {
  success: <CheckCircle className="w-5 h-5" />,
  error: <AlertCircle className="w-5 h-5" />,
  info: <Info className="w-5 h-5" />,
  warning: <AlertTriangle className="w-5 h-5" />,
};

const TOAST_COLORS: Record<ToastType, string> = {
  success: 'bg-emerald-50 dark:bg-emerald-900/30 border-emerald-200 dark:border-emerald-800 text-emerald-900 dark:text-emerald-100',
  error: 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-800 text-red-900 dark:text-red-100',
  info: 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-900 dark:text-blue-100',
  warning: 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-200 dark:border-yellow-800 text-yellow-900 dark:text-yellow-100',
};

const TOAST_ICON_COLORS: Record<ToastType, string> = {
  success: 'text-emerald-600 dark:text-emerald-400',
  error: 'text-red-600 dark:text-red-400',
  info: 'text-blue-600 dark:text-blue-400',
  warning: 'text-yellow-600 dark:text-yellow-400',
};

/**
 * Toast 数据接口
 */
export interface Toast {
  id: string;
  type: ToastType;
  title: string;
  description?: string;
  duration?: number;
}

/**
 * Toast Context 接口
 */
interface ToastActionsContextType {
  addToast: (toast: Omit<Toast, 'id'>) => void;
  removeToast: (id: string) => void;
}

const ToastActionsContext = createContext<ToastActionsContextType | undefined>(undefined);
const ToastStateContext = createContext<Toast[] | undefined>(undefined);

/**
 * useToast Hook
 *
 * 用于在组件中触发 Toast 通知
 *
 * @example
 * ```tsx
 * const { toast } = useToast();
 *
 * toast({
 *   type: 'success',
 *   title: '保存成功',
 *   description: '您的更改已保存',
 *   duration: 3000
 * });
 * ```
 */
export const useToast = () => {
  const actions = useContext(ToastActionsContext);
  if (!actions) {
    throw new Error('useToast must be used within ToastProvider');
  }

  const toast = useCallback((options: Omit<Toast, 'id'>) => {
    actions.addToast(options);
  }, [actions]);

  return { toast, removeToast: actions.removeToast };
};

const useToastState = (): Toast[] => {
  const toasts = useContext(ToastStateContext);
  if (!toasts) {
    throw new Error('useToastState must be used within ToastProvider');
  }
  return toasts;
};

/**
 * ToastProvider - Toast 上下文提供者
 *
 * 需要包裹在应用的根组件中
 */
export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = Math.random().toString(36).substring(2, 9);
    const newToast: Toast = {
      ...toast,
      id,
      duration: toast.duration ?? animationDuration.slow * 6 // 默认 3000ms
    };
    setToasts((prev) => [...prev, newToast]);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  const actions = useMemo(() => ({ addToast, removeToast }), [addToast, removeToast]);

  return (
    <ToastActionsContext.Provider value={actions}>
      <ToastStateContext.Provider value={toasts}>
        {children}
        <ToastContainer />
      </ToastStateContext.Provider>
    </ToastActionsContext.Provider>
  );
};

/**
 * ToastContainer - Toast 容器组件
 *
 * 负责渲染所有 Toast 通知
 */
const ToastContainer: React.FC = () => {
  const toasts = useToastState();

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} />
      ))}
    </div>
  );
};

/**
 * ToastItem - 单个 Toast 通知组件
 */
const ToastItem: React.FC<{ toast: Toast }> = ({ toast }) => {
  const { removeToast } = useToast();
  const [isExiting, setIsExiting] = useState(false);
  const removalTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const startExit = useCallback(() => {
    if (removalTimerRef.current !== null) return;
    setIsExiting(true);
    removalTimerRef.current = setTimeout(() => {
      removalTimerRef.current = null;
      removeToast(toast.id);
    }, animationDuration.normal);
  }, [removeToast, toast.id]);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (toast.duration && toast.duration > 0) {
      timer = setTimeout(startExit, toast.duration);
    }
    return () => {
      if (timer !== undefined) clearTimeout(timer);
      if (removalTimerRef.current !== null) {
        clearTimeout(removalTimerRef.current);
        removalTimerRef.current = null;
      }
    };
  }, [toast.duration, startExit]);

  const handleClose = startExit;

  return (
    <div
      className={cn(
        "pointer-events-auto w-96 max-w-[calc(100vw-2rem)] rounded-lg border shadow-lg p-4",
        "transition-all duration-300",
        TOAST_COLORS[toast.type],
        isExiting ? "opacity-0 translate-x-full" : "opacity-100 translate-x-0 animate-slide-in-right"
      )}
      role="alert"
      aria-live="polite"
    >
      <div className="flex items-start gap-3">
        <div className={cn("shrink-0 mt-0.5", TOAST_ICON_COLORS[toast.type])}>
          {TOAST_ICONS[toast.type]}
        </div>

        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-sm mb-1">{toast.title}</h3>
          {toast.description && (
            <p className="text-sm opacity-90">{toast.description}</p>
          )}
        </div>

        <button
          onClick={handleClose}
          className="shrink-0 opacity-70 hover:opacity-100 transition-opacity"
          aria-label="关闭通知"
        >
          <X className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
};
