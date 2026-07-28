export interface MessageAttachment {
  url: string;
  name: string;
  kind: 'image' | 'file';
  content_type: string;
  size: number;
}

export const MAX_MESSAGE_ATTACHMENTS = 5;
