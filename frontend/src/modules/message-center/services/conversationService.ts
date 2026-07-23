import { apiClient } from '@/libs/http/apiClient';

export interface Message {
  id: string;
  from: string;
  text: string;
  time: string;
  read_by_recipient?: boolean;
}

export interface ConversationItem {
  id: string;
  student_id?: string;
  teacher_id?: string;
  student_name?: string;
  teacher_name?: string;
  class_name?: string;
  scope?: string;
  last_message: string;
  last_time: string;
  unread: number;
  pending_reply?: boolean;
  archived: boolean;
}

export interface ConversationDetail extends ConversationItem {
  messages: Message[];
  messages_total: number;
  messages_page: number;
  messages_page_size: number;
}

export interface ListResponse {
  items: ConversationItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface Contact {
  id: string;
  teacher_name: string;
  scope: string;
}

const BASE = '/conversations';

const toParams = (params: Record<string, string | number | undefined>) => {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') searchParams.set(k, String(v));
  });
  return searchParams.toString();
};

export const conversationService = {
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

  async get(id: string, params?: { messages_page?: number; messages_page_size?: number }): Promise<ConversationDetail> {
    const { data } = await apiClient.get<ConversationDetail>(`${BASE}/${id}`, { params });
    return data;
  },

  async create(body: {
    target_id: string;
    subject?: string;
    initial_message?: string;
  }): Promise<ConversationDetail> {
    const { data } = await apiClient.post<ConversationDetail>(BASE, body);
    return data;
  },

  async sendMessage(id: string, text: string): Promise<Message> {
    const { data } = await apiClient.post<Message>(`${BASE}/${id}/messages`, { text });
    return data;
  },

  async studentContacts(): Promise<{ contacts: Contact[] }> {
    const { data } = await apiClient.get<{ contacts: Contact[] }>(`${BASE}/contacts/students`);
    return data;
  },

  async searchUsers(q: string): Promise<{ contacts: Contact[] }> {
    const { data } = await apiClient.get<{ contacts: Contact[] }>(`${BASE}/search-users`, { params: { q } });
    return data;
  },

  async archive(id: string): Promise<void> {
    await apiClient.put(`${BASE}/${id}/archive`);
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`${BASE}/${id}`);
  },

  async contacts(): Promise<{ contacts: Contact[] }> {
    const { data } = await apiClient.get<{ contacts: Contact[] }>(`${BASE}/contacts/teachers`);
    return data;
  },
};
