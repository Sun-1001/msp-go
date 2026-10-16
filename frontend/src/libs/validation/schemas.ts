import { z } from 'zod';

// ============================================
// Common Validation Rules
// ============================================

/** Username validation: 3-20 characters, alphanumeric and underscore */
export const usernameSchema = z
  .string()
  .min(3, '用户名至少需要 3 个字符')
  .max(20, '用户名最多 20 个字符')
  .regex(/^[a-zA-Z0-9_\u4e00-\u9fa5]+$/, '用户名只能包含字母、数字、下划线或中文');

/** Email validation */
export const emailSchema = z
  .string()
  .min(1, '请输入邮箱地址')
  .email('请输入有效的邮箱地址');

/** Strong password validation: matches backend bcrypt-safe password policy */
export const strongPasswordSchema = z
  .string()
  .min(8, '密码至少需要 8 个字符')
  .max(72, '密码最多 72 个字符')
  .regex(/[a-z]/, '密码需要包含至少一个小写字母')
  .regex(/[A-Z]/, '密码需要包含至少一个大写字母')
  .regex(/[0-9]/, '密码需要包含至少一个数字')
  .regex(/[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?`~]/, '密码需要包含至少一个特殊字符');

/** Password validation used by registration, change-password, and admin reset flows */
export const passwordSchema = strongPasswordSchema;

/** User role validation */
export const userRoleSchema = z.enum(['student', 'teacher', 'admin'], {
  message: '请选择有效的用户角色',
});

/** Class code validation */
export const classCodeSchema = z.object({
  code: z
    .string()
    .min(4, '班级号至少需要 4 位')
    .max(12, '班级号最多 12 位'),
});

export type ClassCodeFormData = z.infer<typeof classCodeSchema>;

// ============================================
// Auth Schemas
// ============================================

/** Login form schema */
export const loginSchema = z.object({
  username: usernameSchema,
  password: z.string().min(1, '请输入密码'),
  role: z.enum(['student', 'teacher'], {
    message: '请选择登录身份',
  }),
});

export type LoginFormData = z.infer<typeof loginSchema>;

/** Register form schema */
export const registerSchema = z
  .object({
    username: usernameSchema,
    email: emailSchema,
    password: passwordSchema,
    confirmPassword: z.string().min(1, '请确认密码'),
    role: z.enum(['student', 'teacher'], {
      message: '请选择注册身份',
    }),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: '两次输入的密码不一致',
    path: ['confirmPassword'],
  });

export type RegisterFormData = z.infer<typeof registerSchema>;

/** Forgot password schema */
export const forgotPasswordSchema = z.object({
  username: usernameSchema,
  email: emailSchema,
  reason: z.string().max(500, '申请理由最多 500 个字符').optional(),
});

export type ForgotPasswordFormData = z.infer<typeof forgotPasswordSchema>;

// ============================================
// Profile Schemas
// ============================================

/** Profile update schema */
export const profileUpdateSchema = z.object({
  name: z.string().min(1, '请输入姓名').max(50, '姓名最多 50 个字符'),
  email: emailSchema.optional(),
  phone: z
    .string()
    .regex(/^1[3-9]\d{9}$/, '请输入有效的手机号码')
    .optional()
    .or(z.literal('')),
  bio: z.string().max(200, '个人简介最多 200 个字符').optional(),
});

export type ProfileUpdateFormData = z.infer<typeof profileUpdateSchema>;

/** Password change schema */
export const passwordChangeSchema = z
  .object({
    currentPassword: z.string().min(1, '请输入当前密码'),
    newPassword: passwordSchema,
    confirmNewPassword: z.string().min(1, '请确认新密码'),
  })
  .refine((data) => data.newPassword === data.confirmNewPassword, {
    message: '两次输入的新密码不一致',
    path: ['confirmNewPassword'],
  })
  .refine((data) => data.currentPassword !== data.newPassword, {
    message: '新密码不能与当前密码相同',
    path: ['newPassword'],
  });

export type PasswordChangeFormData = z.infer<typeof passwordChangeSchema>;

// ============================================
// Admin Schemas
// ============================================

/** AI Provider schema */
export const aiProviderSchema = z.object({
  name: z.string().min(1, '请输入提供商名称').max(50, '名称最多 50 个字符'),
  type: z.string().min(1, '请选择提供商类型'),
  apiKey: z.string().min(1, '请输入 API Key'),
  baseUrl: z.string().url('请输入有效的 URL').optional().or(z.literal('')),
  isActive: z.boolean().default(true),
});

export type AIProviderFormData = z.infer<typeof aiProviderSchema>;

/** AI Model schema */
export const aiModelSchema = z.object({
  name: z.string().min(1, '请输入模型名称').max(50, '名称最多 50 个字符'),
  modelId: z.string().min(1, '请输入模型 ID'),
  providerId: z.string().min(1, '请选择提供商'),
  isActive: z.boolean().default(true),
});

export type AIModelFormData = z.infer<typeof aiModelSchema>;

// ============================================
// Exercise Schemas
// ============================================

/** Exercise answer schema */
export const exerciseAnswerSchema = z.object({
  answer: z.string().min(1, '请输入答案'),
});

export type ExerciseAnswerFormData = z.infer<typeof exerciseAnswerSchema>;

/** Question create/edit schema */
export const questionSchema = z.object({
  title: z.string().min(1, '请输入题目标题').max(200, '标题最多 200 个字符'),
  content: z.string().min(1, '请输入题目内容'),
  difficulty: z.enum(['easy', 'medium', 'hard'], {
    message: '请选择难度等级',
  }),
  type: z.enum(['choice', 'fill', 'short_answer', 'proof'], {
    message: '请选择题目类型',
  }),
  conceptIds: z.array(z.string()).min(1, '请至少选择一个知识点'),
  solution: z.string().optional(),
  hints: z.array(z.string()).optional(),
});

export type QuestionFormData = z.infer<typeof questionSchema>;

// ============================================
// System Settings Schemas
// ============================================

/** System general settings schema */
export const systemGeneralSettingsSchema = z.object({
  systemName: z.string().min(1, '请输入系统名称').max(100, '名称最多 100 个字符'),
  systemDescription: z.string().max(500, '描述最多 500 个字符').optional(),
  timezone: z.string().min(1, '请选择时区'),
  language: z.string().min(1, '请选择语言'),
});

export type SystemGeneralSettingsFormData = z.infer<typeof systemGeneralSettingsSchema>;

/** Database settings schema */
export const databaseSettingsSchema = z.object({
  host: z.string().min(1, '请输入主机地址'),
  port: z.coerce.number().min(1, '端口号无效').max(65535, '端口号无效'),
  database: z.string().min(1, '请输入数据库名称'),
  username: z.string().min(1, '请输入用户名'),
  password: z.string().optional(),
});

export type DatabaseSettingsFormData = z.infer<typeof databaseSettingsSchema>;

/** Email settings schema */
export const emailSettingsSchema = z.object({
  smtpServer: z.string().min(1, '请输入 SMTP 服务器地址'),
  smtpPort: z.coerce.number().min(1, '端口号无效').max(65535, '端口号无效'),
  senderEmail: emailSchema,
  senderName: z.string().max(50, '发件人名称最多 50 个字符').optional(),
  username: z.string().optional(),
  password: z.string().optional(),
  useTls: z.boolean().default(true),
});

export type EmailSettingsFormData = z.infer<typeof emailSettingsSchema>;

/** Security settings schema */
export const securitySettingsSchema = z.object({
  minPasswordLength: z.coerce.number().min(6, '最小密码长度不能小于 6').max(32, '最小密码长度不能大于 32'),
  passwordExpiryDays: z.coerce.number().min(0, '密码过期天数不能为负数').max(365, '密码过期天数不能超过 365'),
  requireSpecialChar: z.boolean().default(false),
  sessionTimeoutMinutes: z.coerce.number().min(5, '会话超时时间不能小于 5 分钟').max(1440, '会话超时时间不能超过 24 小时'),
  maxConcurrentSessions: z.coerce.number().min(1, '最大并发会话数不能小于 1').max(10, '最大并发会话数不能超过 10'),
});

export type SecuritySettingsFormData = z.infer<typeof securitySettingsSchema>;
