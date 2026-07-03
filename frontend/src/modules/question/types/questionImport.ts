/**
 * 题库导入/导出相关类型定义
 */

/** 解析出的原始题目（前端本地解析结果） */
export interface ParsedQuestion {
  /** 前端临时 ID */
  tempId: string;
  /** 题目标题 */
  title: string;
  /** 题目内容（保留 LaTeX） */
  body: string;
  /** 题型 */
  type: 'short_answer' | 'multiple_choice' | 'proof' | 'unknown';
  /** 难度 0-1，默认 0.5 */
  difficulty: number;
  /** 标准答案 */
  answer: string;
  /** 答案类型 */
  answerType: 'expression' | 'numeric' | 'text';
  /** 选择题选项 */
  options?: string[];
  /** 提示列表 */
  hints: string[];
  /** 解题步骤 */
  solutionSteps: string[];
  /** 标签 */
  tags: string[];
  /** 解析置信度 0-1 */
  confidence: number;
  /** 原始文本（用于 AI 重新解析） */
  rawText: string;
  /** 解析警告 */
  parseWarnings: string[];
}

/** 导入步骤 */
export type ImportStep = 'upload' | 'parsing' | 'preview' | 'importing' | 'done';

/** 导入结果统计 */
export interface ImportResult {
  success: number;
  failed: number;
  errors: string[];
}

/** AI 解析请求 */
export interface AIParseRequest {
  raw_texts: string[];
}

/** AI 解析出的单个题目 */
export interface AIParseQuestionItem {
  title: string;
  body: string;
  type: string;
  difficulty: number;
  answer: string;
  answer_type: string;
  options?: string[];
  hints: string[];
  solution_steps: string[];
  tags: string[];
}

/** AI 解析响应 */
export interface AIParseResponse {
  questions: AIParseQuestionItem[];
}

/** 导出格式 */
export type ExportFormat = 'json' | 'markdown' | 'txt';

/** 导出选项 */
export interface ExportOptions {
  format: ExportFormat;
  includeAnswers: boolean;
  includeHints: boolean;
  includeSolutionSteps: boolean;
  /** 为空则导出当前筛选结果 */
  selectedIds?: string[];
}

/** 支持的导入文件类型 */
export const SUPPORTED_IMPORT_EXTENSIONS = ['.docx', '.txt'] as const;

/** 最大文件大小 10MB */
export const MAX_IMPORT_FILE_SIZE = 10 * 1024 * 1024;

/** 单次批量导入最大题目数 */
export const MAX_BATCH_IMPORT_SIZE = 200;

/** 单个文件本地解析后最多进入预览的题目数 */
export const MAX_PARSED_IMPORT_QUESTIONS = 1000;

/** AI 解析单次最大文本段数 */
export const MAX_AI_PARSE_TEXTS = 10;

/** AI 解析单段最大字符数 */
export const MAX_AI_PARSE_TEXT_LENGTH = 3000;
