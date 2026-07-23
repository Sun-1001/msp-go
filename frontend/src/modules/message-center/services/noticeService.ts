import { apiClient } from '@/libs/http/apiClient';

export interface StudentNoticeItem {
  id: string;
  class_name: string;
  title: string;
  body: string;
  published_at: string;
  confirmed: boolean;
  attachments: string[];
}

export interface TeacherNoticeItem {
  id: string;
  class_name: string;
  title: string;
  body: string;
  published_at: string;
  confirmed_count: number;
  total_count: number;
  unconfirmed_students: string[];
}

export interface ListResponse {
  items: (StudentNoticeItem | TeacherNoticeItem)[];
  total: number;
  page: number;
  page_size: number;
}

const BASE = '/notices';

const toParams = (params: Record<string, string | number | undefined>) => {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') searchParams.set(k, String(v));
  });
  return searchParams.toString();
};

export const noticeService = {
  async list(params: {
    search?: string;
    status?: string;
    class_name?: string;
    page?: number;
    page_size?: number;
  }): Promise<ListResponse> {
    const qs = toParams(params);
    const { data } = await apiClient.get<ListResponse>(`${BASE}?${qs}`);
    return data;
  },

  async get(id: string): Promise<StudentNoticeItem | TeacherNoticeItem> {
    const { data } = await apiClient.get<StudentNoticeItem | TeacherNoticeItem>(`${BASE}/${id}`);
    return data;
  },

  async create(body: {
    class_id: string;
    title: string;
    body: string;
  }): Promise<TeacherNoticeItem> {
    const { data } = await apiClient.post<TeacherNoticeItem>(BASE, body);
    return data;
  },

  async confirm(id: string): Promise<void> {
    await apiClient.post(`${BASE}/${id}/confirm`);
  },

  async remind(id: string): Promise<{ unconfirmed_students: string[]; count: number }> {
    const { data } = await apiClient.post<{ unconfirmed_students: string[]; count: number }>(`${BASE}/${id}/remind`);
    return data;
  },
};
