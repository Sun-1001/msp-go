/**
 * 导入预览编辑表格
 *
 * 显示解析出的题目列表，支持编辑、AI 识别、删除等操作
 */

import React, { useState } from 'react';
import { Button } from '../../../../components/ui/Button';
import { Badge } from '../../../../components/ui/Badge';
import { Input } from '../../../../components/ui/Input';
import { Select } from '../../../../components/ui/Select';
import { MathText } from '../../../../libs/math/MathText';
import { Edit, Trash2, Sparkles, ChevronUp, Check, X } from 'lucide-react';
import type { ParsedQuestion } from '@/modules/question/types/questionImport';

interface ImportPreviewTableProps {
  questions: ParsedQuestion[];
  selectedIds: Set<string>;
  aiParsingIds: Set<string>;
  allowSelectAll?: boolean;
  onToggleSelect: (id: string) => void;
  onToggleSelectAll: () => void;
  onUpdateQuestion: (id: string, updates: Partial<ParsedQuestion>) => void;
  onDeleteQuestion: (id: string) => void;
  onAiParse: (ids: string[]) => void;
}

const difficultyOptions = [
  { value: '0.15', label: '简单' },
  { value: '0.5', label: '中等' },
  { value: '0.85', label: '困难' },
];

const typeOptions = [
  { value: 'short_answer', label: '简答题' },
  { value: 'multiple_choice', label: '选择题' },
  { value: 'proof', label: '证明题' },
];

function getConfidenceBadge(confidence: number) {
  if (confidence >= 0.7) {
    return <Badge variant="success">高置信</Badge>;
  } else if (confidence >= 0.5) {
    return <Badge variant="warning">中置信</Badge>;
  } else {
    return <Badge variant="destructive">低置信</Badge>;
  }
}

export const ImportPreviewTable: React.FC<ImportPreviewTableProps> = ({
  questions,
  selectedIds,
  aiParsingIds,
  allowSelectAll = true,
  onToggleSelect,
  onToggleSelectAll,
  onUpdateQuestion,
  onDeleteQuestion,
  onAiParse,
}) => {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<Partial<ParsedQuestion>>({});

  const allSelected = questions.length > 0 && selectedIds.size === questions.length;
  const lowConfidenceIds = questions
    .filter((q) => q.confidence < 0.5)
    .map((q) => q.tempId);

  const startEdit = (q: ParsedQuestion) => {
    setEditingId(q.tempId);
    setEditForm({
      title: q.title,
      body: q.body,
      type: q.type,
      difficulty: q.difficulty,
      answer: q.answer,
    });
  };

  const saveEdit = () => {
    if (editingId && editForm) {
      onUpdateQuestion(editingId, editForm);
      setEditingId(null);
      setEditForm({});
    }
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditForm({});
  };

  return (
    <div className="space-y-3">
      {/* 统计栏 */}
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center gap-4 text-sm text-surface-600 dark:text-surface-400">
          <span>共 <strong>{questions.length}</strong> 道题目</span>
          <span className="text-green-600">高置信 {questions.filter((q) => q.confidence >= 0.7).length} 道</span>
          <span className="text-yellow-600">中置信 {questions.filter((q) => q.confidence >= 0.5 && q.confidence < 0.7).length} 道</span>
          <span className="text-red-600">低置信 {lowConfidenceIds.length} 道</span>
        </div>
        <div className="flex items-center gap-2">
          {lowConfidenceIds.length > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onAiParse(lowConfidenceIds)}
              disabled={aiParsingIds.size > 0}
            >
              <Sparkles className="h-4 w-4 mr-1" />
              AI 识别低置信题目 ({lowConfidenceIds.length})
            </Button>
          )}
        </div>
      </div>

      {/* 题目列表 */}
      <div className="border rounded-lg dark:border-surface-700 max-h-[500px] overflow-y-auto">
        {/* 表头 */}
        <div className="flex items-center gap-3 px-4 py-2 bg-surface-50 dark:bg-surface-800 border-b dark:border-surface-700 sticky top-0 z-10">
          {allowSelectAll ? (
            <input
              type="checkbox"
              checked={allSelected}
              onChange={onToggleSelectAll}
              className="rounded border-surface-300 dark:border-surface-600"
            />
          ) : (
            <span className="h-4 w-4" aria-hidden="true" />
          )}
          <span className="w-8 text-xs text-surface-500">#</span>
          <span className="flex-1 text-xs text-surface-500">题目内容</span>
          <span className="w-16 text-xs text-surface-500">题型</span>
          <span className="w-16 text-xs text-surface-500">置信度</span>
          <span className="w-24 text-xs text-surface-500 text-right">操作</span>
        </div>

        {/* 题目行 */}
        {questions.map((q, index) => (
          <div key={q.tempId}>
            <div
              className={`flex items-start gap-3 px-4 py-3 border-b dark:border-surface-700 hover:bg-surface-50 dark:hover:bg-surface-800/50 ${
                aiParsingIds.has(q.tempId) ? 'opacity-60' : ''
              }`}
            >
              <input
                type="checkbox"
                checked={selectedIds.has(q.tempId)}
                onChange={() => onToggleSelect(q.tempId)}
                className="mt-1 rounded border-surface-300 dark:border-surface-600"
              />
              <span className="w-8 text-sm text-surface-400 mt-0.5">{index + 1}</span>
              <div className="flex-1 min-w-0">
                <div className="font-medium text-surface-900 dark:text-surface-100 text-sm truncate">
                  {q.title || '(无标题)'}
                </div>
                <div className="text-xs text-surface-500 dark:text-surface-400 mt-1 line-clamp-2">
                  <MathText>{q.body.substring(0, 120) + (q.body.length > 120 ? '...' : '')}</MathText>
                </div>
                {q.answer && (
                  <div className="text-xs text-green-600 dark:text-green-400 mt-1">
                    答案: {q.answer.substring(0, 50)}{q.answer.length > 50 ? '...' : ''}
                  </div>
                )}
                {q.parseWarnings.length > 0 && (
                  <div className="text-xs text-yellow-600 dark:text-yellow-400 mt-1">
                    ⚠ {q.parseWarnings[0]}
                  </div>
                )}
              </div>
              <div className="w-16">
                <Badge variant={q.type === 'multiple_choice' ? 'secondary' : q.type === 'proof' ? 'outline' : 'default'}>
                  {q.type === 'short_answer' ? '简答' : q.type === 'multiple_choice' ? '选择' : q.type === 'proof' ? '证明' : '未知'}
                </Badge>
              </div>
              <div className="w-16">{getConfidenceBadge(q.confidence)}</div>
              <div className="w-24 flex justify-end gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => editingId === q.tempId ? cancelEdit() : startEdit(q)}
                >
                  {editingId === q.tempId ? <ChevronUp className="h-4 w-4" /> : <Edit className="h-4 w-4" />}
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onAiParse([q.tempId])}
                  disabled={aiParsingIds.has(q.tempId)}
                  title="AI 重新识别"
                >
                  <Sparkles className={`h-4 w-4 ${aiParsingIds.has(q.tempId) ? 'animate-spin' : ''}`} />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onDeleteQuestion(q.tempId)}
                >
                  <Trash2 className="h-4 w-4 text-red-500" />
                </Button>
              </div>
            </div>

            {/* 内联编辑表单 */}
            {editingId === q.tempId && (
              <div className="px-4 py-3 bg-surface-50 dark:bg-surface-800/50 border-b dark:border-surface-700 space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-surface-500 mb-1 block">标题</label>
                    <Input
                      value={editForm.title || ''}
                      onChange={(e) => setEditForm({ ...editForm, title: e.target.value })}
                      placeholder="题目标题"
                    />
                  </div>
                  <div className="flex gap-3">
                    <div className="flex-1">
                      <label className="text-xs text-surface-500 mb-1 block">题型</label>
                      <Select
                        options={typeOptions}
                        value={editForm.type || 'short_answer'}
                        onChange={(v) => setEditForm({ ...editForm, type: v as ParsedQuestion['type'] })}
                      />
                    </div>
                    <div className="flex-1">
                      <label className="text-xs text-surface-500 mb-1 block">难度</label>
                      <Select
                        options={difficultyOptions}
                        value={String(editForm.difficulty || 0.5)}
                        onChange={(v) => setEditForm({ ...editForm, difficulty: parseFloat(v) })}
                      />
                    </div>
                  </div>
                </div>
                <div>
                  <label className="text-xs text-surface-500 mb-1 block">题目内容</label>
                  <textarea
                    className="w-full rounded-md border border-surface-300 dark:border-surface-600 bg-white dark:bg-surface-900 px-3 py-2 text-sm min-h-20 resize-y"
                    value={editForm.body || ''}
                    onChange={(e) => setEditForm({ ...editForm, body: e.target.value })}
                    placeholder="题目内容（支持 LaTeX）"
                  />
                </div>
                <div>
                  <label className="text-xs text-surface-500 mb-1 block">答案</label>
                  <Input
                    value={editForm.answer || ''}
                    onChange={(e) => setEditForm({ ...editForm, answer: e.target.value })}
                    placeholder="标准答案"
                  />
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="outline" size="sm" onClick={cancelEdit}>
                    <X className="h-4 w-4 mr-1" />
                    取消
                  </Button>
                  <Button size="sm" onClick={saveEdit}>
                    <Check className="h-4 w-4 mr-1" />
                    保存
                  </Button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
