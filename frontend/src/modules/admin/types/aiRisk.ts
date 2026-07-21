export interface AIRiskSettings {
  daily_reply_limit: number;
  max_concurrent_requests: number;
  blocked_keywords: string[];
  reset_timezone: string;
  next_reset_at: string;
}

export interface UpdateAIRiskSettingsRequest {
  daily_reply_limit: number;
  max_concurrent_requests: number;
  blocked_keywords: string[];
}

export interface AIRiskOverview {
  total_students: number;
  blocked_students: number;
  quota_exhausted_students: number;
  replies_today: number;
  risk_events_today: number;
  daily_reply_limit: number;
  max_concurrent_requests: number;
}

export type AIStudentStatusFilter = 'all' | 'active' | 'blocked' | 'quota_exhausted';

export interface AIStudentItem {
  id: string;
  username: string;
  email: string;
  display_name: string | null;
  ai_blocked: boolean;
  blocked_reason: string;
  blocked_at: string | null;
  replies_used: number;
  replies_remaining: number;
  quota_exhausted: boolean;
  last_ai_reply_at: string | null;
}

export interface AIStudentListQuery {
  page?: number;
  page_size?: number;
  search?: string;
  status?: AIStudentStatusFilter;
}

export interface AIStudentListResponse {
  items: AIStudentItem[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface UpdateAIStudentAccessRequest {
  blocked: boolean;
  reason: string;
}

export interface AIStudentAccessResponse {
  student_id: string;
  ai_blocked: boolean;
  blocked_reason: string;
  blocked_at: string | null;
}

export type AIRiskEventType = 'content_blocked' | 'admin_blocked' | 'admin_unblocked';

export interface AIRiskEvent {
  id: string;
  student_id: string | null;
  student_username: string;
  event_type: AIRiskEventType;
  severity: 'info' | 'warning' | 'critical';
  action: string;
  source: string;
  matched_rule: string;
  content_excerpt: string;
  actor_id: string | null;
  created_at: string;
}

export interface AIRiskEventListQuery {
  page?: number;
  page_size?: number;
  search?: string;
  event_type?: AIRiskEventType | 'all';
}

export interface AIRiskEventListResponse {
  items: AIRiskEvent[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}
