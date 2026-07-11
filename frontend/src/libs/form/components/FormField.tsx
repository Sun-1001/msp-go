import React from 'react';
import type { LucideIcon } from 'lucide-react';
import { Input } from '../../../components/ui/Input';
import { FormError } from '../../../components/ui/FormError';
import { cn } from '../../utils/cn';
import { inputStyles, labelStyles } from '../../styles/formStyles';

interface FormFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** 字段标签 */
  label: string;
  /** 标签图标 */
  icon?: LucideIcon;
  /** 错误信息 */
  error?: string;
  /** 输入框 ref（用于 react-hook-form） */
  inputRef?: React.Ref<HTMLInputElement>;
  /** 输入框尾部操作，例如密码显隐按钮 */
  trailingAction?: React.ReactNode;
}

/**
 * 表单字段组件
 *
 * @description 封装 Label + Input + FormError 的通用表单字段
 * 遵循 DRY 原则，消除表单中重复的字段结构代码
 *
 * @example
 * ```tsx
 * <FormField
 *   label="用户名"
 *   icon={User}
 *   placeholder="请输入用户名"
 *   error={errors.username?.message}
 *   {...register('username')}
 * />
 * ```
 */
export const FormField = React.forwardRef<HTMLInputElement, FormFieldProps>(
  ({ label, icon: Icon, error, className, id, trailingAction, ...inputProps }, ref) => {
    const fieldId = id || `field-${label}`;

    return (
      <div className="space-y-2">
        <label
          htmlFor={fieldId}
          className={Icon ? labelStyles.withIcon : labelStyles.base}
        >
          {Icon && <Icon className={labelStyles.icon} />}
          {label}
        </label>
        <div className="relative">
          <Input
            id={fieldId}
            ref={ref}
            className={cn(
              inputStyles.base,
              error && inputStyles.error,
              className,
              trailingAction && 'pr-11'
            )}
            {...inputProps}
          />
          {trailingAction && (
            <div className="absolute inset-y-0 right-0 flex items-center pr-1">
              {trailingAction}
            </div>
          )}
        </div>
        <FormError message={error} />
      </div>
    );
  }
);

FormField.displayName = 'FormField';
