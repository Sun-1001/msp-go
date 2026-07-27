/**
 * 管理员用户管理相关类型定义
 */

import type { EmailNotificationResult } from '@/modules/email/types/email';

// ========== 用户状态和角色 ==========

export type UserStatus = 'active' | 'suspended';
export type UserRole = 'student' | 'teacher' | 'admin';

// ========== 用户列表项 ==========

export interface UserItem {
  id: string;
  username: string;
  email: string;
  display_name: string | null;
  role: UserRole;
  status: UserStatus;
  created_at: string;
}

// ========== 查询参数 ==========

export interface UserListQuery {
  page?: number;
  page_size?: number;
  search?: string;
  role?: UserRole | 'all';
  status?: UserStatus | 'all';
}

// ========== 响应类型 ==========

export interface UserListResponse {
  items: UserItem[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface UserAccountStats {
  total: number;
  active: number;
  suspended: number;
}

export interface UserStatusUpdateResponse {
  success: boolean;
  message: string;
  user: UserItem;
  email_notification?: EmailNotificationResult;
}

export interface UserDeleteResponse {
  success: boolean;
  message: string;
}

// ========== 请求类型 ==========

export interface UserStatusUpdateRequest {
  status: UserStatus;
}

// ========== 显示配置 ==========

export const UserRoleLabels: Record<UserRole, string> = {
  student: '学生',
  teacher: '教师',
  admin: '管理员',
};

export const UserStatusLabels: Record<UserStatus, string> = {
  active: '活跃',
  suspended: '已停用',
};

// ========== 创建用户 ==========

export interface UserCreateRequest {
  username: string;
  email: string;
  password: string;
  role: UserRole;
  display_name?: string;
}

export interface UserCreateResponse {
  success: boolean;
  message: string;
  user: UserItem | null;
}

// ========== 更新用户 ==========

export interface UserUpdateRequest {
  display_name?: string;
  password?: string;
}

export interface UserUpdateResponse {
  success: boolean;
  message: string;
  user: UserItem;
}

// ========== 导入用户 ==========

export interface UserImportResult {
  row: number;
  username: string;
  success: boolean;
  message: string;
}

export interface UserImportResponse {
  success: boolean;
  total: number;
  created: number;
  failed: number;
  skipped: number;
  details: UserImportResult[];
}
