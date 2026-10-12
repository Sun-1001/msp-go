import React from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { useToast } from '@/components/ui/Toast';
import { Loader2 } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { MessageAttachmentPicker } from '@/modules/message-center/MessageAttachmentPicker';
import type { Contact } from '@/modules/message-center/services/conversationService';
import type { MessageAttachment } from '@/modules/message-center/attachmentTypes';

interface NewConversationModalLabels {
  contact: string;
  searchPlaceholder: string;
  noMatch: string;
  noContacts: string;
  draftPlaceholder: string;
}

interface NewConversationModalProps {
  isOpen: boolean;
  onClose: () => void;
  contacts: Contact[];
  selectedContactId: string;
  onSelectContact: (id: string) => void;
  contactSearch: string;
  onContactSearchChange: (value: string) => void;
  draft: string;
  onDraftChange: (value: string) => void;
  attachments: MessageAttachment[];
  onAttachmentsChange: (attachments: MessageAttachment[]) => void;
  uploading: boolean;
  onUploadingChange: (uploading: boolean) => void;
  creating: boolean;
  onCreate: () => void;
  showNoContactsHint: boolean;
  labels: NewConversationModalLabels;
}

export const NewConversationModal: React.FC<NewConversationModalProps> = ({
  isOpen,
  onClose,
  contacts,
  selectedContactId,
  onSelectContact,
  contactSearch,
  onContactSearchChange,
  draft,
  onDraftChange,
  attachments,
  onAttachmentsChange,
  uploading,
  onUploadingChange,
  creating,
  onCreate,
  showNoContactsHint,
  labels,
}) => {
  const { toast } = useToast();

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="新建私信对话" className="max-w-lg">
      <div className="space-y-4">
        <label className="block text-sm font-medium text-surface-700 dark:text-surface-300">{labels.contact}</label>
        <Input value={contactSearch} onChange={(e) => onContactSearchChange(e.target.value)}
          placeholder={labels.searchPlaceholder} />
        {contacts.length > 0 ? (
          <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-md border border-surface-200 dark:border-surface-700">
            {contacts.map((c) => (
              <button key={c.id} type="button"
                onClick={() => onSelectContact(c.id)}
                className={cn('w-full px-4 py-2.5 text-left text-sm hover:bg-surface-50 dark:hover:bg-surface-800',
                  selectedContactId === c.id && 'bg-primary-50 ring-1 ring-inset ring-primary-200 dark:bg-primary-950/30 dark:ring-primary-800')}>
                <div className="font-medium text-surface-900 dark:text-surface-100">{c.display_name}</div>
                <div className="flex items-center justify-between text-xs text-surface-500 dark:text-surface-400">
                  <span>{c.scope || '全校'}</span>
                  <span className="font-mono text-surface-400">{c.id}</span>
                </div>
              </button>
            ))}
          </div>
        ) : contactSearch.trim() ? (
          <p className="text-sm text-surface-500 dark:text-surface-400">{labels.noMatch}</p>
        ) : showNoContactsHint ? (
          <p className="text-sm text-surface-500 dark:text-surface-400">{labels.noContacts}</p>
        ) : null}
        <textarea value={draft} onChange={(e) => onDraftChange(e.target.value)}
          placeholder={labels.draftPlaceholder}
          className="min-h-28 w-full rounded-md border border-surface-200 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
        />
        <MessageAttachmentPicker
          value={attachments}
          onChange={onAttachmentsChange}
          onUploadingChange={onUploadingChange}
          onError={(message) => toast({ type: 'error', title: message })}
          onFeedback={(feedback) => toast(feedback)}
          disabled={creating}
        />
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={onCreate} disabled={!selectedContactId || creating || uploading}>
            {creating ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}创建对话
          </Button>
        </div>
      </div>
    </Modal>
  );
};
