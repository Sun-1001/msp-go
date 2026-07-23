import { apiClient } from '@/libs/http/apiClient';

export interface ThreadMessage {
  id: string;
  from: string;
  text: string;
  time: string;
}

export interface StudentThreadItem {
  id: string;
  title: string;
  teacher_id: string;
  teacher_name: string;
  source: string;
  context: string;
  status: string;
  last_update: string;
}

export interface TeacherThreadItem {
  id: string;
  student_name: string;
  class_name: string;
  title: string;
  source: string;
  knowledge_point: string;
  resource_name?: string;
  status: string;
  context: string;
  last_update: string;
}

export interface ThreadDetail {
  id: string;
  student_name?: string;
  teacher_name?: string;
  class_name?: string;
  title: string;
  teacher_id?: string;
  source: string;
  knowledge_point?: string;
  resource_name?: string;
  status: string;
  context: string;
  messages: ThreadMessage[];
  messages_total: number;
  messages_page: number;
  messages_page_size: number;
}

export interface ListResponse {
  items: (StudentThreadItem | TeacherThreadItem)[];
  total: number;
  page: number;
  page_size: number;
}

const BASE = '/qa-threads';

const toParams = (params: Record<string, string | number | undefined>) => {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') searchParams.set(k, String(v));
  });
  return searchParams.toString();
};

export const qaThreadService = {
  async list(params: {
    search?: string;
    status?: string;
    class_name?: string;
    teacher_id?: string;
    page?: number;
    page_size?: number;
  }): Promise<ListResponse> {
    const qs = toParams(params);
    const { data } = await apiClient.get<ListResponse>(`${BASE}?${qs}`);
    return data;
  },

  async get(id: string, params?: { messages_page?: number; messages_page_size?: number }): Promise<ThreadDetail> {
    const { data } = await apiClient.get<ThreadDetail>(`${BASE}/${id}`, { params });
    return data;
  },

  async create(body: {
    teacher_id?: string;
    content: string;
    source?: string;
  }): Promise<ThreadDetail> {
    const { data } = await apiClient.post<ThreadDetail>(BASE, body);
    return data;
  },

  async importQuestion(body: {
    teacher_id: string;
    source: string;
    content: string;
  }): Promise<ThreadDetail> {
    const { data } = await apiClient.post<ThreadDetail>(`${BASE}/import`, body);
    return data;
  },

  async sendMessage(id: string, text: string): Promise<ThreadMessage> {
    const { data } = await apiClient.post<ThreadMessage>(`${BASE}/${id}/messages`, { text });
    return data;
  },

  async updateStatus(id: string, status: string): Promise<void> {
    await apiClient.put(`${BASE}/${id}/status`, { status });
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`${BASE}/${id}`);
  },
};
