export interface EmailSettings {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_from: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
  smtp_password_configured: boolean;
  configured: boolean;
}

export interface EmailSettingsUpdate {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password?: string;
  smtp_from: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
  clear_password: boolean;
}

export interface EmailSettingsOverride {
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_password?: string;
  smtp_from?: string;
  smtp_from_name?: string;
  smtp_use_tls?: boolean;
}

export interface EmailActionResponse {
  success: boolean;
  message: string;
}

export interface EmailNotificationResult {
  status: 'sent' | 'failed' | 'skipped';
  message: string;
}

export interface EmailTemplate {
  event: string;
  locale: string;
  name: string;
  subject: string;
  html_body: string;
  variables: string[];
  is_custom: boolean;
  updated_at: string | null;
}

export interface EmailTemplateListResponse {
  items: EmailTemplate[];
}

export interface EmailTemplateUpdate {
  subject: string;
  html_body: string;
}

export interface EmailTemplatePreviewRequest {
  event: string;
  locale: string;
  subject?: string;
  html_body?: string;
  variables?: Record<string, string>;
}

export interface EmailTemplatePreviewResponse {
  subject: string;
  html_body: string;
}
