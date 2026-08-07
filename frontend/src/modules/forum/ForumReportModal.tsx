import { useState } from 'react';
import { TriangleAlert } from 'lucide-react';

import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { Select } from '@/components/ui/Select';

import type { ForumReportReason, ForumReportTargetType } from './types';

export interface ForumReportTarget {
  type: ForumReportTargetType;
  id: string;
  label: string;
}

interface ForumReportModalProps {
  target: ForumReportTarget;
  onClose: () => void;
  onSubmit: (
    targetType: ForumReportTargetType,
    targetId: string,
    reason: ForumReportReason,
    detail: string,
  ) => Promise<boolean>;
}

const reasonOptions: Array<{ value: ForumReportReason; label: string }> = [
  { value: 'spam', label: '广告或垃圾信息' },
  { value: 'abuse', label: '辱骂或不友善内容' },
  { value: 'answer_leak', label: '泄露答案' },
  { value: 'misinformation', label: '错误或误导信息' },
  { value: 'copyright', label: '侵权内容' },
  { value: 'other', label: '其他问题' },
];

export function ForumReportModal({ target, onClose, onSubmit }: ForumReportModalProps) {
  const [reason, setReason] = useState<ForumReportReason>('spam');
  const [detail, setDetail] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const submit = async () => {
    if (submitting) return;
    setSubmitting(true);
    const saved = await onSubmit(target.type, target.id, reason, detail.trim());
    setSubmitting(false);
    if (saved) onClose();
  };

  return (
    <Modal
      isOpen
      onClose={submitting ? () => undefined : onClose}
      title={target.type === 'post' ? '举报帖子' : '举报回复'}
      className="max-w-lg p-6 sm:p-8"
    >
      <div className="relative z-10 space-y-4">
        <div className="flex min-w-0 items-center gap-2 rounded-md bg-surface-50 px-3 py-2 text-sm text-surface-600 dark:bg-surface-800 dark:text-surface-300">
          <TriangleAlert className="h-4 w-4 shrink-0 text-red-500" />
          <span className="truncate">{target.label}</span>
        </div>

        <label className="block space-y-2 text-sm font-medium text-surface-700 dark:text-surface-300">
          <span>举报原因</span>
          <Select
            value={reason}
            onChange={(value) => setReason(value as ForumReportReason)}
            options={reasonOptions}
            disabled={submitting}
            aria-label="举报原因"
          />
        </label>

        <label className="block space-y-2 text-sm font-medium text-surface-700 dark:text-surface-300">
          <span>补充说明</span>
          <textarea
            value={detail}
            onChange={(event) => setDetail(event.target.value)}
            rows={4}
            maxLength={2_000}
            disabled={submitting}
            placeholder="可选"
            className="w-full resize-y rounded-md border border-surface-200 bg-white px-3 py-2 text-sm leading-6 text-surface-900 outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-60 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100"
          />
        </label>

        <div className="flex justify-end gap-2 border-t border-surface-100 pt-4 dark:border-surface-800">
          <Button type="button" variant="outline" onClick={onClose} disabled={submitting}>取消</Button>
          <Button type="button" onClick={() => void submit()} isLoading={submitting}>提交举报</Button>
        </div>
      </div>
    </Modal>
  );
}
