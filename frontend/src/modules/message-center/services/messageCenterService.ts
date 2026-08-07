import { apiClient } from '@/libs/http/apiClient';

export interface MessagePreviewItem {
  id: string;
  type: 'conversation' | 'notice' | 'thread' | 'forum';
  /** Optional target fields used by forum previews when id is a notification id. */
  target_id?: string;
  post_id?: string;
  reply_id?: string;
  navigation_id?: string;
  title: string;
  summary: string;
  occurred_at: string;
  pending: boolean;
}

export interface MessageCenterSummary {
  conversation_count: number;
  notice_count: number;
  thread_count: number;
  forum_count?: number;
  items: MessagePreviewItem[];
}

let summaryRequest: Promise<MessageCenterSummary> | null = null;

export const messageCenterService = {
  summary(): Promise<MessageCenterSummary> {
    if (summaryRequest) return summaryRequest;

    summaryRequest = apiClient
      .get<MessageCenterSummary>('/message-center/summary')
      .then(({ data }) => data)
      .finally(() => {
        summaryRequest = null;
      });

    return summaryRequest;
  },
};
