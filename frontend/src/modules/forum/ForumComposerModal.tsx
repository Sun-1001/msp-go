import { useMemo, useState } from 'react';
import { Send } from 'lucide-react';

import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { MessageAttachmentPicker } from '@/modules/message-center/MessageAttachmentPicker';
import type { MessageAttachment } from '@/modules/message-center/attachmentTypes';

import type { ForumPost, SaveForumPostPayload } from './types';

interface ForumComposerModalProps {
  isOpen: boolean;
  post: ForumPost | null;
  saving: boolean;
  onClose: () => void;
  onSave: (payload: SaveForumPostPayload) => Promise<void>;
}

function normalizeTags(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(/[,，]/)
    .map((tag) => tag.trim())
    .filter((tag) => {
      const normalized = tag.toLocaleLowerCase();
      if (!tag || seen.has(normalized)) return false;
      seen.add(normalized);
      return true;
    })
    .slice(0, 8);
}

export function ForumComposerModal({
  isOpen,
  post,
  saving,
  onClose,
  onSave,
}: ForumComposerModalProps) {
  const [title, setTitle] = useState(post?.title ?? '');
  const [content, setContent] = useState(post?.content ?? '');
  const [tagInput, setTagInput] = useState(post?.tags.join('，') ?? '');
  const [attachments, setAttachments] = useState<MessageAttachment[]>(post?.attachments ?? []);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');

  const tags = useMemo(() => normalizeTags(tagInput), [tagInput]);
  const canSubmit = Boolean(title.trim() && content.trim()) && !saving && !uploading;

  const submit = async () => {
    if (!title.trim()) {
      setError('请输入帖子标题');
      return;
    }
    if (!content.trim()) {
      setError('请输入帖子内容');
      return;
    }
    if (title.trim().length > 200) {
      setError('帖子标题不能超过 200 个字符');
      return;
    }
    if (content.trim().length > 50_000) {
      setError('帖子内容不能超过 50000 个字符');
      return;
    }
    if (tags.some((tag) => tag.length > 30)) {
      setError('单个标签不能超过 30 个字符');
      return;
    }

    setError('');
    await onSave({
      title,
      content,
      attachments,
      tags,
      knowledgeNodeId: post?.knowledgeNodeId || undefined,
    });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={saving ? () => undefined : onClose}
      title={post ? '编辑帖子' : '发布帖子'}
      className="max-h-[calc(100vh-2rem)] max-w-2xl overflow-y-auto p-6 sm:p-8"
    >
      <div className="relative z-10 space-y-5">
        <label className="block space-y-2 text-sm font-medium text-surface-700 dark:text-surface-300">
          <span>标题 <span className="font-normal text-surface-400">（必填）</span></span>
          <Input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            maxLength={200}
            disabled={saving}
            placeholder="请输入帖子标题"
          />
        </label>

        <label className="block space-y-2 text-sm font-medium text-surface-700 dark:text-surface-300">
          <span>正文 <span className="font-normal text-surface-400">（必填）</span></span>
          <textarea
            value={content}
            onChange={(event) => setContent(event.target.value)}
            maxLength={50_000}
            disabled={saving}
            rows={9}
            placeholder="像贴吧一样写下你的问题、经验或想法"
            className="w-full resize-y rounded-md border border-surface-200 bg-white px-3 py-3 text-sm leading-6 text-surface-900 outline-none ring-offset-2 placeholder:text-surface-400 focus:ring-2 focus:ring-primary-500 disabled:opacity-60 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
          />
        </label>

        <label className="block space-y-2 text-sm font-medium text-surface-700 dark:text-surface-300">
          <span>标签</span>
          <Input
            value={tagInput}
            onChange={(event) => setTagInput(event.target.value)}
            disabled={saving}
            placeholder="极限，洛必达法则"
          />
          {tags.length > 0 ? (
            <span className="flex flex-wrap gap-1.5">
              {tags.map((tag) => (
                <span key={tag} className="rounded-full bg-surface-100 px-2 py-0.5 text-xs font-normal text-surface-600 dark:bg-surface-800 dark:text-surface-300">
                  #{tag}
                </span>
              ))}
            </span>
          ) : null}
        </label>

        <div className="space-y-2">
          <span className="text-sm font-medium text-surface-700 dark:text-surface-300">附件</span>
          <MessageAttachmentPicker
            value={attachments}
            onChange={setAttachments}
            onUploadingChange={setUploading}
            onError={setError}
            disabled={saving}
          />
        </div>

        {error ? (
          <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200">
            {error}
          </div>
        ) : null}

        <div className="flex justify-end gap-2 border-t border-surface-100 pt-4 dark:border-surface-800">
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>取消</Button>
          <Button type="button" onClick={() => void submit()} disabled={!canSubmit} isLoading={saving}>
            {!saving ? <Send className="mr-2 h-4 w-4" /> : null}
            {post ? '保存修改' : '发布'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
