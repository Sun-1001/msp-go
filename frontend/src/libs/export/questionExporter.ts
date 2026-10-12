/**
 * 题目导出格式生成器
 *
 * 支持 JSON / Markdown / TXT 三种格式
 * 全部在前端本地生成，不依赖后端
 */

import { saveAs } from 'file-saver';
import type { Question, ExportOptions } from '@/modules/question';

// ==================== 格式化工具 ====================

function formatDate(): string {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');
  const h = String(now.getHours()).padStart(2, '0');
  const min = String(now.getMinutes()).padStart(2, '0');
  return `${y}${m}${d}_${h}${min}`;
}

function getDifficultyLabel(difficulty: number): string {
  if (difficulty < 0.33) return '简单';
  if (difficulty < 0.67) return '中等';
  return '困难';
}

function getTypeLabel(type: string): string {
  const map: Record<string, string> = {
    short_answer: '简答题',
    multiple_choice: '选择题',
    proof: '证明题',
  };
  return map[type] || type;
}

// ==================== JSON 导出 ====================

interface JsonExportItem {
  title: string;
  body: string;
  type: string;
  difficulty: number;
  tags: string[];
  answer?: string;
  answerType?: string;
  options?: string[];
  hints?: string[];
  solutionSteps?: string[];
}

function formatQuestionForJson(q: Question, options: ExportOptions): JsonExportItem {
  const item: JsonExportItem = {
    title: q.title,
    body: q.body,
    type: q.type,
    difficulty: q.difficulty,
    tags: q.tags,
  };

  if (options.includeAnswers) {
    item.answer = q.meta.answer;
    item.answerType = q.meta.answerType;
    if (q.meta.options) {
      item.options = q.meta.options;
    }
  }

  if (options.includeHints && q.meta.hints.length > 0) {
    item.hints = q.meta.hints;
  }

  if (options.includeSolutionSteps && q.meta.solutionSteps.length > 0) {
    item.solutionSteps = q.meta.solutionSteps;
  }

  return item;
}

export function exportAsJson(questions: Question[], options: ExportOptions): void {
  const data = questions.map((q) => formatQuestionForJson(q, options));
  const json = JSON.stringify(data, null, 2);
  const blob = new Blob([json], { type: 'application/json;charset=utf-8' });
  saveAs(blob, `题库导出_${formatDate()}.json`);
}

// ==================== Markdown 导出 ====================

function formatQuestionAsMarkdown(
  q: Question,
  index: number,
  options: ExportOptions,
): string {
  const lines: string[] = [];

  lines.push(`## ${index}. ${q.title}`);
  lines.push('');
  lines.push(`**题型**: ${getTypeLabel(q.type)} | **难度**: ${getDifficultyLabel(q.difficulty)}`);
  if (q.tags.length > 0) {
    lines.push(`**标签**: ${q.tags.join(', ')}`);
  }
  lines.push('');
  lines.push(q.body);

  // 选择题选项
  if (q.meta.options && q.meta.options.length > 0) {
    lines.push('');
    q.meta.options.forEach((opt, i) => {
      lines.push(`${String.fromCharCode(65 + i)}. ${opt}`);
    });
  }

  // 答案
  if (options.includeAnswers && q.meta.answer) {
    lines.push('');
    lines.push(`**答案**: ${q.meta.answer}`);
  }

  // 提示
  if (options.includeHints && q.meta.hints.length > 0) {
    lines.push('');
    lines.push('**提示**:');
    q.meta.hints.forEach((hint, i) => {
      lines.push(`${i + 1}. ${hint}`);
    });
  }

  // 解题步骤
  if (options.includeSolutionSteps && q.meta.solutionSteps.length > 0) {
    lines.push('');
    lines.push('**解题步骤**:');
    q.meta.solutionSteps.forEach((step, i) => {
      lines.push(`${i + 1}. ${step}`);
    });
  }

  return lines.join('\n');
}

export function exportAsMarkdown(questions: Question[], options: ExportOptions): void {
  const header = `# 题库导出\n\n> 导出时间: ${new Date().toLocaleString('zh-CN')}\n> 题目数量: ${questions.length}\n`;
  const body = questions
    .map((q, i) => formatQuestionAsMarkdown(q, i + 1, options))
    .join('\n\n---\n\n');
  const content = `${header}\n${body}\n`;
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' });
  saveAs(blob, `题库导出_${formatDate()}.md`);
}

// ==================== TXT 导出 ====================

function formatQuestionAsTxt(
  q: Question,
  index: number,
  options: ExportOptions,
): string {
  const lines: string[] = [];

  lines.push(`${index}. ${q.title}`);
  lines.push(`[${getTypeLabel(q.type)}] [${getDifficultyLabel(q.difficulty)}]`);
  if (q.tags.length > 0) {
    lines.push(`标签: ${q.tags.join(', ')}`);
  }
  lines.push('');
  lines.push(q.body);

  // 选择题选项
  if (q.meta.options && q.meta.options.length > 0) {
    lines.push('');
    q.meta.options.forEach((opt, i) => {
      lines.push(`${String.fromCharCode(65 + i)}. ${opt}`);
    });
  }

  // 答案
  if (options.includeAnswers && q.meta.answer) {
    lines.push('');
    lines.push(`答案: ${q.meta.answer}`);
  }

  // 提示
  if (options.includeHints && q.meta.hints.length > 0) {
    lines.push('');
    lines.push('提示:');
    q.meta.hints.forEach((hint, i) => {
      lines.push(`  ${i + 1}. ${hint}`);
    });
  }

  // 解题步骤
  if (options.includeSolutionSteps && q.meta.solutionSteps.length > 0) {
    lines.push('');
    lines.push('解题步骤:');
    q.meta.solutionSteps.forEach((step, i) => {
      lines.push(`  ${i + 1}. ${step}`);
    });
  }

  return lines.join('\n');
}

export function exportAsTxt(questions: Question[], options: ExportOptions): void {
  const header = `题库导出\n导出时间: ${new Date().toLocaleString('zh-CN')}\n题目数量: ${questions.length}\n`;
  const body = questions
    .map((q, i) => formatQuestionAsTxt(q, i + 1, options))
    .join('\n\n========================================\n\n');
  const content = `${header}\n${'='.repeat(40)}\n\n${body}\n`;
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  saveAs(blob, `题库导出_${formatDate()}.txt`);
}

// ==================== 统一导出入口 ====================

/**
 * 根据选项导出题目
 */
export function exportQuestions(questions: Question[], options: ExportOptions): void {
  if (questions.length === 0) return;

  switch (options.format) {
    case 'json':
      exportAsJson(questions, options);
      break;
    case 'markdown':
      exportAsMarkdown(questions, options);
      break;
    case 'txt':
      exportAsTxt(questions, options);
      break;
  }
}
