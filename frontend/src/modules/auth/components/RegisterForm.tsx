import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { registerSchema, type RegisterFormData } from '@/libs/validation';
import { User, Lock, Mail, UserPlus, GraduationCap, BookOpen, AlertCircle, Loader2, CheckCircle } from 'lucide-react';
import {
  FormField,
  FormHeader,
  FormDivider,
  FormFooterLink,
  FormFooterText,
  FormRootError,
  FormSubmitButton,
  RoleSelector,
  type RoleOption,
} from '@/libs/form';
import { systemSettingService, type RegistrationSettings } from '@/modules/admin/services/systemSettingService';
import { authService } from '@/modules/auth/services/authService';
import { getApiErrorMessage } from '@/libs/http/apiClient';

type UserRole = 'student' | 'teacher';

interface RegisterFormProps {
  onSwitchToLogin?: () => void;
}

/**
 * 角色选项配置
 */
const roleOptions: RoleOption<UserRole>[] = [
  {
    value: 'student',
    label: '学生',
    description: '我想学习数学知识',
    icon: GraduationCap,
    gradient: 'from-primary-500 to-secondary-500',
    bgGradient: 'from-primary-50 to-secondary-50 dark:from-primary-900/50 dark:to-secondary-900/50',
    borderColor: 'border-primary-500 dark:border-primary-400',
    textColor: 'text-primary-600 dark:text-primary-400',
  },
  {
    value: 'teacher',
    label: '教师',
    description: '我想管理学生和课程',
    icon: BookOpen,
    gradient: 'from-emerald-500 to-teal-500',
    bgGradient: 'from-emerald-50 to-teal-50 dark:from-emerald-900/50 dark:to-teal-900/50',
    borderColor: 'border-emerald-500 dark:border-emerald-400',
    textColor: 'text-emerald-600 dark:text-emerald-400',
  },
];

export const RegisterForm: React.FC<RegisterFormProps> = ({ onSwitchToLogin }) => {
  // 注册状态
  const [registrationStatus, setRegistrationStatus] = useState<RegistrationSettings | null>(null);
  const [isLoadingStatus, setIsLoadingStatus] = useState(true);
  const [registerSuccess, setRegisterSuccess] = useState(false);
  const [registeredEmail, setRegisteredEmail] = useState('');

  // 加载注册状态
  useEffect(() => {
    const loadStatus = async () => {
      try {
        const status = await systemSettingService.getRegistrationStatus();
        setRegistrationStatus(status);
      } catch {
        // 加载失败时默认允许注册
        setRegistrationStatus({ allow_student: true, allow_teacher: true });
      } finally {
        setIsLoadingStatus(false);
      }
    };
    loadStatus();
  }, []);

  const {
    register,
    handleSubmit,
    control,
    setValue,
    formState: { errors, isSubmitting },
    setError,
  } = useForm<RegisterFormData>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
      role: 'student',
    },
  });

  // 使用 useWatch 替代 watch()，与 React Compiler 兼容
  const role = useWatch({ control, name: 'role' });

  // 当前选择的角色是否允许注册
  const isCurrentRoleAllowed = registrationStatus
    ? role === 'student'
      ? registrationStatus.allow_student
      : registrationStatus.allow_teacher
    : true;

  // 是否所有注册都关闭
  const isAllRegistrationClosed = registrationStatus
    ? !registrationStatus.allow_student && !registrationStatus.allow_teacher
    : false;

  // 动态生成角色选项（添加禁用状态）
  const dynamicRoleOptions: RoleOption<UserRole>[] = roleOptions.map((option) => ({
    ...option,
    disabled: registrationStatus
      ? option.value === 'student'
        ? !registrationStatus.allow_student
        : !registrationStatus.allow_teacher
      : false,
    disabledReason:
      registrationStatus &&
      ((option.value === 'student' && !registrationStatus.allow_student) ||
        (option.value === 'teacher' && !registrationStatus.allow_teacher))
        ? '暂停注册'
        : undefined,
  }));

  const onSubmit = async (data: RegisterFormData) => {
    // 检查当前角色是否允许注册
    if (!isCurrentRoleAllowed) {
      setError('root', {
        type: 'manual',
        message: `${role === 'student' ? '学生' : '教师'}注册功能已暂停`,
      });
      return;
    }

    try {
      await authService.register({
        username: data.username,
        email: data.email,
        password: data.password,
        role: data.role,
      });

      setRegisteredEmail(data.email);
      setRegisterSuccess(true);
    } catch (err) {
      const message = getApiErrorMessage(err, '注册失败，请稍后重试');
      setError('root', {
        type: 'manual',
        message,
      });
    }
  };

  // 加载中状态
  if (isLoadingStatus) {
    return (
      <div className="w-full space-y-6">
        <FormHeader
          icon={UserPlus}
          title="创建账号"
          subtitle="加入我们，开启智能数学学习之旅"
        />
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
        </div>
      </div>
    );
  }

  // 注册成功
  if (registerSuccess) {
    return (
      <div className="w-full space-y-6">
        <FormHeader
          icon={UserPlus}
          title="注册成功"
          subtitle="账号已创建，可以直接登录"
        />
        <div className="flex flex-col py-6 space-y-4">
          <div className="p-4 rounded-full bg-emerald-50 dark:bg-emerald-900/30 w-fit mx-auto">
            <CheckCircle className="w-12 h-12 text-emerald-500" />
          </div>
          <div className="text-center">
            <h3 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-1">
              账号创建成功
            </h3>
            <p className="text-sm text-surface-500 dark:text-surface-400">
              邮箱 {registeredEmail} 已保存，邮箱验证功能暂未接入。
            </p>
          </div>
        </div>
        <FormDivider />
        <FormFooterLink
          text="下一步"
          linkText="立即登录"
          onClick={onSwitchToLogin}
        />
      </div>
    );
  }

  // 所有注册都关闭
  if (isAllRegistrationClosed) {
    return (
      <div className="w-full space-y-6">
        <FormHeader
          icon={UserPlus}
          title="创建账号"
          subtitle="加入我们，开启智能数学学习之旅"
        />
        <div className="flex flex-col items-center justify-center py-8 space-y-4">
          <div className="p-4 rounded-full bg-amber-50 dark:bg-amber-900/30">
            <AlertCircle className="w-12 h-12 text-amber-500" />
          </div>
          <div className="text-center">
            <h3 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-2">
              注册功能已暂停
            </h3>
            <p className="text-sm text-surface-500 dark:text-surface-400 max-w-xs">
              系统当前暂停了新用户注册，请稍后再试或联系管理员。
            </p>
          </div>
        </div>
        <FormDivider />
        <FormFooterLink
          text="已有账号？"
          linkText="立即登录"
          onClick={onSwitchToLogin}
        />
      </div>
    );
  }

  return (
    <div className="w-full space-y-6">
      <FormHeader
        icon={UserPlus}
        title="创建账号"
        subtitle="加入我们，开启智能数学学习之旅"
      />

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
        <RoleSelector
          options={dynamicRoleOptions}
          value={role}
          onChange={(value) => setValue('role', value)}
          label="选择身份"
          error={errors.role?.message}
        />

        <FormField
          label="用户名"
          icon={User}
          placeholder="请输入用户名"
          autoComplete="username"
          disabled={isSubmitting || !isCurrentRoleAllowed}
          error={errors.username?.message}
          {...register('username')}
        />

        <div className="space-y-2">
          <FormField
            label="邮箱"
            icon={Mail}
            type="email"
            placeholder="请输入邮箱地址"
            autoComplete="email"
            disabled={isSubmitting || !isCurrentRoleAllowed}
            error={errors.email?.message}
            {...register('email')}
          />
          <p className="text-xs text-surface-500 dark:text-surface-400 -mt-1">
            用于账号联系与后续找回；邮箱验证功能暂未接入
          </p>
        </div>

        <FormField
          label="密码"
          icon={Lock}
          type="password"
          placeholder="请输入强密码"
          autoComplete="new-password"
          disabled={isSubmitting || !isCurrentRoleAllowed}
          error={errors.password?.message}
          {...register('password')}
        />

        <FormField
          label="确认密码"
          icon={Lock}
          type="password"
          placeholder="请再次输入密码"
          autoComplete="new-password"
          disabled={isSubmitting || !isCurrentRoleAllowed}
          error={errors.confirmPassword?.message}
          {...register('confirmPassword')}
        />

        {/* 当前角色不允许注册的提示 */}
        {!isCurrentRoleAllowed && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-600 dark:text-amber-400">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span className="text-sm">
              {role === 'student' ? '学生' : '教师'}注册功能已暂停，请选择其他身份或稍后再试
            </span>
          </div>
        )}

        <FormRootError message={errors.root?.message} />

        <FormSubmitButton isLoading={isSubmitting} disabled={!isCurrentRoleAllowed}>
          注册
        </FormSubmitButton>
      </form>

      <FormDivider />

      <FormFooterLink
        text="已有账号？"
        linkText="立即登录"
        onClick={onSwitchToLogin}
      />

      <FormFooterText>
        注册即表示您同意我们的
        <Link
          to="/terms-of-service"
          className="ml-1 text-primary-600 hover:text-primary-500 dark:text-primary-400 dark:hover:text-primary-300 underline underline-offset-2"
        >
          服务条款
        </Link>
        和
        <Link
          to="/privacy-policy"
          className="ml-1 text-primary-600 hover:text-primary-500 dark:text-primary-400 dark:hover:text-primary-300 underline underline-offset-2"
        >
          隐私政策
        </Link>
      </FormFooterText>
    </div>
  );
};
