import { useState, type FormEvent } from 'react';
import { Eye, FileCode2, FileText } from 'lucide-react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { AnnouncementContent } from './AnnouncementContent';
import type {
  AnnouncementAudience,
  AnnouncementContentFormat,
  SaveAnnouncementRequest,
  SystemAnnouncement,
} from './types';

interface AnnouncementEditorModalProps {
  isOpen: boolean;
  announcement: SystemAnnouncement | null;
  isSaving: boolean;
  onClose: () => void;
  onSubmit: (payload: SaveAnnouncementRequest) => void;
}

const audienceOptions: Array<{ value: AnnouncementAudience; label: string }> = [
  { value: 'student', label: '学生' },
  { value: 'teacher', label: '教师' },
  { value: 'all', label: '学生与教师' },
];

function createFormState(announcement: SystemAnnouncement | null): SaveAnnouncementRequest {
  if (announcement) {
    return {
      title: announcement.title,
      content: announcement.content,
      content_format: announcement.content_format,
      audience: announcement.audience,
      append: announcement.append,
      persistent: announcement.persistent,
      is_active: announcement.is_active,
    };
  }
  return {
    title: '',
    content: '',
    content_format: 'markdown',
    audience: 'all',
    append: false,
    persistent: false,
    is_active: true,
  };
}

export function AnnouncementEditorModal({
  isOpen,
  announcement,
  isSaving,
  onClose,
  onSubmit,
}: AnnouncementEditorModalProps) {
  const [form, setForm] = useState<SaveAnnouncementRequest>(() => createFormState(announcement));

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit({ ...form, title: form.title.trim() });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={announcement ? '编辑公告' : '发布公告'}
      className="max-h-[calc(100vh-2rem)] max-w-6xl overflow-y-auto rounded-lg p-6"
    >
      <form className="relative z-1 space-y-5" onSubmit={handleSubmit}>
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div className="min-w-0 space-y-5">
            <label className="block space-y-2">
              <span className="text-sm font-medium text-surface-800 dark:text-surface-200">标题</span>
              <input
                required
                maxLength={120}
                value={form.title}
                onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
                className="h-10 w-full rounded-md border border-surface-300 bg-white px-3 text-sm text-surface-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-100"
              />
            </label>

            <fieldset className="space-y-2">
              <legend className="text-sm font-medium text-surface-800 dark:text-surface-200">受众</legend>
              <div className="grid grid-cols-3 overflow-hidden rounded-md border border-surface-300 dark:border-surface-700">
                {audienceOptions.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    aria-pressed={form.audience === option.value}
                    onClick={() => setForm((current) => ({ ...current, audience: option.value }))}
                    className={`min-h-10 px-2 text-sm font-medium transition-colors ${
                      form.audience === option.value
                        ? 'bg-primary-600 text-white'
                        : 'bg-white text-surface-700 hover:bg-surface-100 dark:bg-surface-900 dark:text-surface-300 dark:hover:bg-surface-800'
                    }`}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </fieldset>

            <fieldset className="space-y-2">
              <legend className="text-sm font-medium text-surface-800 dark:text-surface-200">正文格式</legend>
              <div className="grid grid-cols-2 overflow-hidden rounded-md border border-surface-300 dark:border-surface-700">
                {(['markdown', 'html'] as AnnouncementContentFormat[]).map((format) => {
                  const Icon = format === 'markdown' ? FileText : FileCode2;
                  return (
                    <button
                      key={format}
                      type="button"
                      aria-pressed={form.content_format === format}
                      onClick={() => setForm((current) => ({ ...current, content_format: format }))}
                      className={`flex min-h-10 items-center justify-center gap-2 px-3 text-sm font-medium transition-colors ${
                        form.content_format === format
                          ? 'bg-primary-600 text-white'
                          : 'bg-white text-surface-700 hover:bg-surface-100 dark:bg-surface-900 dark:text-surface-300 dark:hover:bg-surface-800'
                      }`}
                    >
                      <Icon className="h-4 w-4" />
                      {format === 'markdown' ? 'Markdown' : 'HTML'}
                    </button>
                  );
                })}
              </div>
            </fieldset>

            <label className="block space-y-2">
              <span className="text-sm font-medium text-surface-800 dark:text-surface-200">正文</span>
              <textarea
                required
                maxLength={50000}
                value={form.content}
                onChange={(event) => setForm((current) => ({ ...current, content: event.target.value }))}
                className="min-h-80 w-full resize-y rounded-md border border-surface-300 bg-white p-3 font-mono text-sm leading-6 text-surface-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-100"
              />
            </label>

            <div className="grid gap-3 sm:grid-cols-3">
              <label className="flex min-h-11 items-center gap-3 rounded-md border border-surface-200 px-3 dark:border-surface-700">
                <input
                  type="checkbox"
                  checked={form.append}
                  onChange={(event) => setForm((current) => ({ ...current, append: event.target.checked }))}
                  className="h-4 w-4 accent-primary-600"
                />
                <span className="text-sm text-surface-700 dark:text-surface-300">追加</span>
              </label>
              <label className="flex min-h-11 items-center gap-3 rounded-md border border-surface-200 px-3 dark:border-surface-700">
                <input
                  type="checkbox"
                  checked={form.persistent}
                  onChange={(event) => setForm((current) => ({ ...current, persistent: event.target.checked }))}
                  className="h-4 w-4 accent-primary-600"
                />
                <span className="text-sm text-surface-700 dark:text-surface-300">常驻</span>
              </label>
              <label className="flex min-h-11 items-center gap-3 rounded-md border border-surface-200 px-3 dark:border-surface-700">
                <input
                  type="checkbox"
                  checked={form.is_active}
                  onChange={(event) => setForm((current) => ({ ...current, is_active: event.target.checked }))}
                  className="h-4 w-4 accent-primary-600"
                />
                <span className="text-sm text-surface-700 dark:text-surface-300">立即生效</span>
              </label>
            </div>
          </div>

          <section className="min-w-0" aria-label="公告预览">
            <div className="mb-2 flex items-center gap-2 text-sm font-medium text-surface-800 dark:text-surface-200">
              <Eye className="h-4 w-4" />
              预览
            </div>
            <div className="h-[min(68vh,680px)] min-h-96 overflow-auto rounded-md border border-surface-200 bg-white p-4 dark:border-surface-700 dark:bg-surface-900">
              {form.title.trim() ? (
                <h2 className="mb-4 text-center text-lg font-semibold text-surface-900 dark:text-surface-100">
                  {form.title.trim()}
                </h2>
              ) : null}
              {form.content.trim() ? (
                <AnnouncementContent
                  content={form.content}
                  format={form.content_format}
                  title="公告 HTML 预览"
                />
              ) : (
                <div className="flex h-56 items-center justify-center text-sm text-surface-400">暂无内容</div>
              )}
            </div>
          </section>
        </div>

        <div className="flex flex-col-reverse gap-2 border-t border-surface-200 pt-4 dark:border-surface-700 sm:flex-row sm:justify-end">
          <Button type="button" variant="outline" onClick={onClose} disabled={isSaving}>
            取消
          </Button>
          <Button type="submit" isLoading={isSaving}>
            {announcement ? '保存' : '发布'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
