import type { LucideIcon } from 'lucide-react';
import { cn } from '../../utils/cn';
import { roleSelectorStyles } from '../../styles/formStyles';

/**
 * 角色选项配置
 */
export interface RoleOption<T extends string = string> {
  /** 角色值 */
  value: T;
  /** 显示标签 */
  label: string;
  /** 描述文字 */
  description: string;
  /** 图标组件 */
  icon: LucideIcon;
  /** 渐变色（用于选中状态的图标背景） */
  gradient: string;
  /** 背景渐变色（用于选中状态的按钮背景） */
  bgGradient: string;
  /** 边框颜色（用于选中状态） */
  borderColor: string;
  /** 文字颜色（用于选中状态） */
  textColor: string;
  /** 是否禁用此选项 */
  disabled?: boolean;
  /** 禁用原因（显示在描述位置） */
  disabledReason?: string;
}

interface RoleSelectorProps<T extends string = string> {
  /** 角色选项列表 */
  options: RoleOption<T>[];
  /** 当前选中的角色 */
  value: T;
  /** 选择变更回调 */
  onChange: (value: T) => void;
  /** 是否禁用 */
  disabled?: boolean;
  /** 标签文字 */
  label?: string;
  /** 错误信息 */
  error?: string;
  /** 展示密度 */
  variant?: 'cards' | 'compact';
}

/**
 * 角色选择器组件
 *
 * @description 用于登录/注册表单中的角色选择
 * 遵循 DRY 原则，从 LoginForm 和 RegisterForm 中提取的公共组件
 *
 * @example
 * ```tsx
 * <RoleSelector
 *   options={roleOptions}
 *   value={role}
 *   onChange={(value) => setValue('role', value)}
 *   label="选择身份"
 * />
 * ```
 */
export function RoleSelector<T extends string = string>({
  options,
  value,
  onChange,
  disabled = false,
  label,
  error,
  variant = 'cards',
}: RoleSelectorProps<T>) {
  const isCompact = variant === 'compact';

  return (
    <div className="space-y-2">
      {label && (
        <span className="text-sm font-medium text-surface-700 dark:text-surface-300">
          {label}
        </span>
      )}
      <div className="grid grid-cols-2 gap-3" role="group" aria-label={label}>
        {options.map((option) => {
          const Icon = option.icon;
          const isSelected = value === option.value;
          const isOptionDisabled = disabled || option.disabled;

          return (
            <button
              key={option.value}
              type="button"
              onClick={() => !isOptionDisabled && onChange(option.value)}
              disabled={isOptionDisabled}
              aria-pressed={isSelected}
              className={cn(
                roleSelectorStyles.button.base,
                isCompact && 'min-h-16 rounded-lg border p-2.5',
                isSelected && !isOptionDisabled
                  ? `${option.borderColor} bg-gradient-to-br ${option.bgGradient}`
                  : roleSelectorStyles.button.unselected,
                isOptionDisabled && 'opacity-50 cursor-not-allowed'
              )}
            >
              <div
                className={cn(
                  'flex gap-2',
                  isCompact ? 'flex-row items-center text-left' : 'flex-col items-center text-center'
                )}
              >
                <div
                  className={cn(
                    isCompact ? 'h-9 w-9 shrink-0' : 'h-10 w-10',
                    roleSelectorStyles.iconContainer.base,
                    isSelected && !isOptionDisabled
                      ? `bg-gradient-to-br ${option.gradient} ${roleSelectorStyles.iconContainer.selected}`
                      : roleSelectorStyles.iconContainer.unselected
                  )}
                >
                  <Icon className={isCompact ? 'h-4 w-4' : 'h-5 w-5'} />
                </div>
                <div>
                  <div
                    className={cn(
                      roleSelectorStyles.label.base,
                      isSelected && !isOptionDisabled ? option.textColor : roleSelectorStyles.label.unselected
                    )}
                  >
                    {option.label}
                  </div>
                  <div className={cn(
                    roleSelectorStyles.description,
                    isCompact && 'text-[11px]',
                    option.disabled && 'text-amber-500 dark:text-amber-400'
                  )}>
                    {option.disabled && option.disabledReason ? option.disabledReason : option.description}
                  </div>
                </div>
              </div>
              {/* Selected indicator */}
              {isSelected && !isOptionDisabled && (
                <div
                  className={cn(
                    roleSelectorStyles.indicator,
                    isCompact && 'right-1.5 top-1.5 h-4 w-4',
                    `bg-gradient-to-br ${option.gradient}`
                  )}
                >
                  <svg
                    className="w-3 h-3 text-white"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={3}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                </div>
              )}
            </button>
          );
        })}
      </div>
      {error && <p className="text-sm text-red-500 dark:text-red-400">{error}</p>}
    </div>
  );
}
