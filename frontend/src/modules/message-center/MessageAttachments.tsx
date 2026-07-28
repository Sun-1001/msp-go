import { useState } from 'react';
import { FileText } from 'lucide-react';

import type { MessageAttachment } from '@/modules/message-center/attachmentTypes';
import { MessageImagePreview } from '@/modules/message-center/MessageImagePreview';

interface MessageAttachmentsProps {
  attachments?: MessageAttachment[];
}

function formatFileSize(size: number): string {
  if (!Number.isFinite(size) || size <= 0) return '';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export function MessageAttachments({ attachments = [] }: MessageAttachmentsProps) {
  const [previewImage, setPreviewImage] = useState<MessageAttachment | null>(null);

  if (attachments.length === 0) return null;
  return (
    <>
      <div className="mt-2 flex flex-col gap-2">
        {attachments.map((attachment) => attachment.kind === 'image' ? (
          <button
            key={attachment.url}
            type="button"
            title={`查看图片 ${attachment.name}`}
            onClick={() => setPreviewImage(attachment)}
            className="group overflow-hidden rounded-md border border-surface-200 bg-white text-left dark:border-surface-700 dark:bg-surface-900"
          >
            <img src={attachment.url} alt={attachment.name} className="max-h-56 w-full object-contain" />
            <span className="block truncate border-t border-surface-100 px-2 py-1.5 text-xs text-surface-500 group-hover:text-surface-800 dark:border-surface-800 dark:text-surface-400 dark:group-hover:text-surface-100">
              {attachment.name}
            </span>
          </button>
        ) : (
          <a
            key={attachment.url}
            href={attachment.url}
            download={attachment.name}
            target="_blank"
            rel="noreferrer"
            className="flex min-w-0 items-center gap-2 rounded-md border border-surface-200 bg-white p-3 text-sm hover:bg-surface-50 dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
          >
            <FileText className="h-5 w-5 shrink-0 text-surface-400" />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium text-surface-700 dark:text-surface-200">{attachment.name}</span>
              <span className="block text-xs text-surface-400">{formatFileSize(attachment.size)}</span>
            </span>
          </a>
        ))}
      </div>
      <MessageImagePreview image={previewImage} onClose={() => setPreviewImage(null)} />
    </>
  );
}
