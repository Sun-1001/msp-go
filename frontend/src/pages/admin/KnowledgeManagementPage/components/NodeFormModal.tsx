import React, { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Loader2 } from 'lucide-react';
import { NODE_TYPE_OPTIONS, INPUT_CLASS } from '../constants';
import type {
  KnowledgeNodeAdmin,
  KnowledgeNodeCreateData,
  KnowledgeNodeUpdateData,
} from '@/modules/admin/api';

interface NodeFormModalProps {
  node: KnowledgeNodeAdmin | null;
  chapters: string[];
  saving: boolean;
  onSave: (data: KnowledgeNodeCreateData | KnowledgeNodeUpdateData) => void;
  onClose: () => void;
}

export const NodeFormModal = React.memo<NodeFormModalProps>(({ node, chapters, saving, onSave, onClose }) => {
  const [form, setForm] = useState({
    name: node?.name || '',
    name_en: node?.name_en || '',
    node_type: node?.node_type || 'concept',
    description: node?.description || '',
    chapter: node?.chapter || '',
    section: node?.section || '',
    difficulty: node?.difficulty ?? 0.5,
    latex_formula: node?.latex_formula || '',
    tags: (node?.tags || []).join(', '),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const data = {
      name: form.name,
      name_en: form.name_en || undefined,
      node_type: form.node_type as KnowledgeNodeCreateData['node_type'],
      description: form.description || undefined,
      chapter: form.chapter || undefined,
      section: form.section || undefined,
      difficulty: form.difficulty,
      latex_formula: form.latex_formula || undefined,
      tags: form.tags
        ? form.tags
            .split(',')
            .map((t) => t.trim())
            .filter(Boolean)
        : [],
    };
    onSave(data);
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-surface-900 rounded-xl p-6 max-w-lg w-full mx-4 shadow-xl max-h-[90vh] overflow-y-auto">
        <h3 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">
          {node ? '编辑知识节点' : '新增知识节点'}
        </h3>
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* 名称 */}
          <div>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">名称 *</label>
            <input
              className={INPUT_CLASS}
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              required
            />
          </div>

          {/* 英文名 */}
          <div>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">英文名称</label>
            <input
              className={INPUT_CLASS}
              value={form.name_en}
              onChange={(e) => setForm((f) => ({ ...f, name_en: e.target.value }))}
            />
          </div>

          {/* 类型 + 难度 */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">类型 *</label>
              <select
                className={INPUT_CLASS}
                value={form.node_type}
                onChange={(e) => setForm((f) => ({ ...f, node_type: e.target.value }))}
              >
                {NODE_TYPE_OPTIONS.filter((o) => o.value).map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
                难度: {form.difficulty.toFixed(1)}
              </label>
              <input
                type="range"
                min="0"
                max="1"
                step="0.1"
                className="w-full"
                value={form.difficulty}
                onChange={(e) => setForm((f) => ({ ...f, difficulty: parseFloat(e.target.value) }))}
              />
            </div>
          </div>

          {/* 章节 + 小节 */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">章节</label>
              <input
                className={INPUT_CLASS}
                value={form.chapter}
                onChange={(e) => setForm((f) => ({ ...f, chapter: e.target.value }))}
                list="chapter-list"
              />
              <datalist id="chapter-list">
                {chapters.map((ch) => (
                  <option key={ch} value={ch} />
                ))}
              </datalist>
            </div>
            <div>
              <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">小节</label>
              <input
                className={INPUT_CLASS}
                value={form.section}
                onChange={(e) => setForm((f) => ({ ...f, section: e.target.value }))}
              />
            </div>
          </div>

          {/* 描述 */}
          <div>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">描述</label>
            <textarea
              className={`${INPUT_CLASS} h-20 resize-none`}
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
            />
          </div>

          {/* LaTeX 公式 */}
          <div>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">LaTeX 公式</label>
            <input
              className={INPUT_CLASS}
              value={form.latex_formula}
              onChange={(e) => setForm((f) => ({ ...f, latex_formula: e.target.value }))}
              placeholder="例: \int_a^b f(x)dx"
            />
          </div>

          {/* 标签 */}
          <div>
            <label className="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
              标签（逗号分隔）
            </label>
            <input
              className={INPUT_CLASS}
              value={form.tags}
              onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))}
              placeholder="例: 基础, 重要, 考试常考"
            />
          </div>

          {/* 按钮 */}
          <div className="flex justify-end gap-3 pt-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
              取消
            </Button>
            <Button type="submit" disabled={saving || !form.name}>
              {saving ? <Loader2 className="h-4 w-4 animate-spin mr-1" /> : null}
              {node ? '保存修改' : '创建节点'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
});

NodeFormModal.displayName = 'NodeFormModal';
