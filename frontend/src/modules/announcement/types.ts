export type AnnouncementAudience = 'student' | 'teacher' | 'all';
export type AnnouncementContentFormat = 'markdown' | 'html';

export interface SystemAnnouncement {
  id: string;
  title: string;
  content: string;
  content_format: AnnouncementContentFormat;
  audience: AnnouncementAudience;
  append: boolean;
  persistent: boolean;
  is_active: boolean;
  revision: number;
  published_at: string;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface AnnouncementListResponse {
  items: SystemAnnouncement[];
}

export interface SaveAnnouncementRequest {
  title: string;
  content: string;
  content_format: AnnouncementContentFormat;
  audience: AnnouncementAudience;
  append: boolean;
  persistent: boolean;
  is_active: boolean;
}

export interface AnnouncementActionResponse {
  success: boolean;
  message: string;
}
