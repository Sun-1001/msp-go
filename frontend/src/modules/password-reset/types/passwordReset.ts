/** 密码重置相关类型定义 */

import type { EmailNotificationResult } from '@/modules/email/types/email';

// 用户端
export interface PasswordResetSubmitRequest {
  username: string;
  email: string;
  reason?: string;
}

export interface PasswordResetSubmitResponse {
  success: boolean;
  message: string;
  request_id?: string | null;
}

export interface PasswordResetStatusResponse {
  has_pending: boolean;
  status: string | null;
  created_at: string | null;
}

// 管理员端
export interface PasswordResetRequestItem {
  id: string;
  user_id: string;
  username: string;
  email: string;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
  created_at: string;
  reviewed_at: string | null;
}

export interface PasswordResetListResponse {
  items: PasswordResetRequestItem[];
  total: number;
  pending_count: number;
}

export interface PasswordResetReviewRequest {
  action: 'approve' | 'reject';
  reject_reason?: string;
}

export interface PasswordResetReviewResponse {
  success: boolean;
  message: string;
  temp_password: string | null;
  email_notification?: EmailNotificationResult;
}
