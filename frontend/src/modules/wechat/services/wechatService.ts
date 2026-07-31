import { apiClient } from '@/libs/http/apiClient';

export interface WechatBindingStatus {
  available: boolean;
  account_name?: string | null;
  is_bound: boolean;
  subscribed: boolean;
  bound_at?: string | null;
}

export interface WechatBindingTicket {
  ticket: string;
  command: string;
  expires_at: string;
  account_name?: string | null;
}

export const wechatService = {
  async getBindingStatus(signal?: AbortSignal): Promise<WechatBindingStatus> {
    const response = await apiClient.get<WechatBindingStatus>('/integrations/wechat/binding', { signal });
    return response.data;
  },

  async createBindingTicket(): Promise<WechatBindingTicket> {
    const response = await apiClient.post<WechatBindingTicket>('/integrations/wechat/binding-ticket');
    return response.data;
  },

  async unbind(): Promise<void> {
    await apiClient.delete('/integrations/wechat/binding');
  },
};
