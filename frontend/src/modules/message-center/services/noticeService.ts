import { apiClient } from '@/libs/http/apiClient';
import { buildQueryString } from '@/modules/message-center/services/queryParams';
import type { MessageAttachment } from '@/modules/message-center/attachmentTypes';

export interface StudentNoticeListItem {
  id: string;
  class_name: string;
  title: string;
  published_at: string;
  confirmed: boolean;
}

export interface StudentNoticeItem extends StudentNoticeListItem {
  body: string;
  attachments: MessageAttachment[];
}

export interface TeacherNoticeListItem {
  id: string;
  class_name: string;
  title: string;
  published_at: string;
  confirmed_count: number;
  total_count: number;
}

export interface TeacherNoticeItem extends TeacherNoticeListItem {
  body: string;
  unconfirmed_students: string[];
  attachments: MessageAttachment[];
}

export interface ListResponse<T extends StudentNoticeListItem | TeacherNoticeListItem = StudentNoticeListItem | TeacherNoticeListItem> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

const BASE = '/notices';

export const noticeService = {
  async list<T extends StudentNoticeListItem | TeacherNoticeListItem = StudentNoticeListItem | TeacherNoticeListItem>(params: {
    search?: string;
    status?: string;
    class_name?: string;
    page?: number;
    page_size?: number;
  }, signal?: AbortSignal): Promise<ListResponse<T>> {
    const qs = buildQueryString(params);
    const { data } = await apiClient.get<ListResponse<T>>(`${BASE}?${qs}`, { signal });
    return data;
  },

  async get(id: string, signal?: AbortSignal): Promise<StudentNoticeItem | TeacherNoticeItem> {
    const { data } = await apiClient.get<StudentNoticeItem | TeacherNoticeItem>(`${BASE}/${id}`, { signal });
    return data;
  },

  async create(body: {
    class_id: string;
    title: string;
    body: string;
    attachments?: MessageAttachment[];
  }): Promise<TeacherNoticeItem> {
    const { data } = await apiClient.post<TeacherNoticeItem>(BASE, body);
    return data;
  },

  async confirm(id: string): Promise<void> {
    await apiClient.post(`${BASE}/${id}/confirm`);
  },

  async remind(id: string): Promise<{ unconfirmed_students: string[]; count: number; queued_count: number }> {
    const { data } = await apiClient.post<{ unconfirmed_students: string[]; count: number; queued_count: number }>(`${BASE}/${id}/remind`);
    return data;
  },
};
