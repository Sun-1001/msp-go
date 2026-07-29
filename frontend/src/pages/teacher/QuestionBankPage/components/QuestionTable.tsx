import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent } from '../../../../components/ui/Card';
import { Button } from '../../../../components/ui/Button';
import { MathText } from '../../../../libs/math/MathText';
import {
  Table, TableHeader, TableBody, TableHead, TableRow, TableCell,
} from '../../../../components/ui/Table';
import {
  Edit, MoreHorizontal, Copy, Trash2, Send, FileEdit, Archive, Star,
} from 'lucide-react';
import { getDifficultyBadge, getTypeBadge, getStatusBadge } from '../constants';
import type { Question } from '@/modules/question/types/question';

interface QuestionTableProps {
  questions: Question[];
  loading: boolean;
  selectedQuestions: string[];
  onToggleSelect: (id: string) => void;
  onToggleSelectAll: () => void;
  openMenuId: string | null;
  onSetOpenMenuId: (id: string | null) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
  onDuplicate: (id: string) => void;
  onStatusChange: (id: string, status: string) => void;
  onDailyCandidateChange: (id: string, enabled: boolean) => void;
  dailyCandidateUpdatingIds: string[];
  onDelete: (id: string) => void;
}

export const QuestionTable: React.FC<QuestionTableProps> = ({
  questions, loading, selectedQuestions,
  onToggleSelect, onToggleSelectAll,
  openMenuId, onSetOpenMenuId, menuRef,
  onDuplicate, onStatusChange, onDailyCandidateChange, dailyCandidateUpdatingIds, onDelete,
}) => {
  const navigate = useNavigate();

  if (loading && questions.length === 0) {
    return (
      <Card>
        <CardContent className="p-8 text-center text-surface-500 dark:text-surface-400">
          加载中...
        </CardContent>
      </Card>
    );
  }

  if (questions.length === 0) {
    return (
      <Card>
        <CardContent className="p-8 text-center text-surface-500 dark:text-surface-400">
          暂无题目，点击"新建题目"开始创建吧～
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">
                <input
                  type="checkbox"
                  checked={selectedQuestions.length === questions.length && questions.length > 0}
                  onChange={onToggleSelectAll}
                  className="rounded border-surface-300 dark:border-surface-600"
                />
              </TableHead>
              <TableHead className="w-[40%]">题目内容</TableHead>
              <TableHead>题型</TableHead>
              <TableHead>难度</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="text-center">每日题</TableHead>
              <TableHead>使用次数</TableHead>
              <TableHead>正确率</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {questions.map((question) => (
              <QuestionRow
                key={question.id}
                question={question}
                isSelected={selectedQuestions.includes(question.id)}
                onToggleSelect={onToggleSelect}
                isMenuOpen={openMenuId === question.id}
                onToggleMenu={() => onSetOpenMenuId(openMenuId === question.id ? null : question.id)}
                menuRef={openMenuId === question.id ? menuRef : undefined}
                onEdit={() => navigate(`/teacher/question/${question.id}/edit`)}
                onDuplicate={() => onDuplicate(question.id)}
                onStatusChange={(status) => onStatusChange(question.id, status)}
                onDailyCandidateChange={(enabled) => onDailyCandidateChange(question.id, enabled)}
                isDailyCandidateUpdating={dailyCandidateUpdatingIds.includes(question.id)}
                onDelete={() => onDelete(question.id)}
              />
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
};

interface QuestionRowProps {
  question: Question;
  isSelected: boolean;
  onToggleSelect: (id: string) => void;
  isMenuOpen: boolean;
  onToggleMenu: () => void;
  menuRef?: React.RefObject<HTMLDivElement | null>;
  onEdit: () => void;
  onDuplicate: () => void;
  onStatusChange: (status: string) => void;
  onDailyCandidateChange: (enabled: boolean) => void;
  isDailyCandidateUpdating: boolean;
  onDelete: () => void;
}

const QuestionRow: React.FC<QuestionRowProps> = ({
  question, isSelected, onToggleSelect,
  isMenuOpen, onToggleMenu, menuRef,
  onEdit, onDuplicate, onStatusChange, onDailyCandidateChange, isDailyCandidateUpdating, onDelete,
}) => (
  <TableRow>
    <TableCell>
      <input
        type="checkbox"
        checked={isSelected}
        onChange={() => onToggleSelect(question.id)}
        className="rounded border-surface-300 dark:border-surface-600"
      />
    </TableCell>
    <TableCell>
      <div className="max-w-md">
        <div className="flex items-center gap-2 mb-1">
          <span className="inline-flex items-center rounded-full border border-primary-200 dark:border-primary-800 bg-primary-50 dark:bg-primary-900/30 px-2.5 py-0.5 text-xs font-medium text-primary-700 dark:text-primary-300">
            {question.title || '未分组'}
          </span>
        </div>
        <div className="text-sm text-surface-500 dark:text-surface-400 line-clamp-2">
          <MathText>{question.body.substring(0, 100) + '...'}</MathText>
        </div>
      </div>
    </TableCell>
    <TableCell>{getTypeBadge(question.type)}</TableCell>
    <TableCell>{getDifficultyBadge(question.difficulty)}</TableCell>
    <TableCell>{getStatusBadge(question.status)}</TableCell>
    <TableCell className="text-center">
      <Button
        variant="ghost"
        size="icon"
        aria-label={question.isDailyCandidate ? '取消每日题候选' : '设为每日题候选'}
        aria-pressed={question.isDailyCandidate}
        title={
          question.isDailyCandidate
            ? '取消每日题候选'
            : question.status === 'published'
              ? '设为每日题候选'
              : '发布后可设为每日题候选'
        }
        disabled={isDailyCandidateUpdating || (!question.isDailyCandidate && question.status !== 'published')}
        onClick={() => onDailyCandidateChange(!question.isDailyCandidate)}
        className={question.isDailyCandidate ? 'text-amber-500 hover:text-amber-600' : 'text-surface-400'}
      >
        <Star className={`h-4 w-4 ${question.isDailyCandidate ? 'fill-current' : ''}`} />
      </Button>
    </TableCell>
    <TableCell>{question.usageCount}</TableCell>
    <TableCell>
      {question.usageCount > 0 ? `${(question.correctRate * 100).toFixed(1)}%` : '-'}
    </TableCell>
    <TableCell className="text-right">
      <div className="flex justify-end gap-1">
        <Button variant="ghost" size="icon" title="编辑题目" onClick={onEdit}>
          <Edit className="h-4 w-4" />
        </Button>
        <div className="relative" ref={menuRef}>
          <Button variant="ghost" size="icon" title="更多操作" onClick={(e) => { e.stopPropagation(); onToggleMenu(); }}>
            <MoreHorizontal className="h-4 w-4" />
          </Button>
          {isMenuOpen && (
            <div
              className="absolute right-0 top-full mt-1 w-36 rounded-md border border-surface-200 bg-white shadow-lg z-50 py-1 dark:border-surface-700 dark:bg-surface-800"
              onClick={(e) => e.stopPropagation()}
            >
              <button className="w-full px-3 py-2 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-700 flex items-center gap-2" onClick={onDuplicate}>
                <Copy className="h-4 w-4" /> 复制
              </button>
              {question.status !== 'published' && (
                <button className="w-full px-3 py-2 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-700 flex items-center gap-2" onClick={() => onStatusChange('published')}>
                  <Send className="h-4 w-4" /> 发布
                </button>
              )}
              {question.status === 'published' && (
                <button className="w-full px-3 py-2 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-700 flex items-center gap-2" onClick={() => onStatusChange('draft')}>
                  <FileEdit className="h-4 w-4" /> 转为草稿
                </button>
              )}
              {question.status !== 'archived' && (
                <button className="w-full px-3 py-2 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-700 flex items-center gap-2" onClick={() => onStatusChange('archived')}>
                  <Archive className="h-4 w-4" /> 归档
                </button>
              )}
              <div className="border-t border-surface-200 dark:border-surface-700 my-1" />
              <button className="w-full px-3 py-2 text-left text-sm hover:bg-red-50 dark:hover:bg-red-900/20 text-red-600 dark:text-red-400 flex items-center gap-2" onClick={onDelete}>
                <Trash2 className="h-4 w-4" /> 删除
              </button>
            </div>
          )}
        </div>
      </div>
    </TableCell>
  </TableRow>
);
