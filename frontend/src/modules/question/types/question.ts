/**
 * 题目管理相关类型定义
 */

export interface Question {
  id: string;
  /** 题目分组（如：极限与连续、微分方程） */
  title: string;
  body: string; // LaTeX/Markdown
  type: 'short_answer' | 'multiple_choice' | 'proof';
  difficulty: number; // 0-1
  conceptIds: string[];
  tags: string[];
  status: 'draft' | 'published' | 'archived';
  isDailyCandidate: boolean;
  meta: {
    answer: string;
    answerType: string;
    hints: string[];
    solutionSteps: string[];
    options?: string[];
    estimatedTimeSeconds: number;
  };
  createdAt: string;
  updatedAt: string;
  usageCount: number;
  correctRate: number;
}

export interface QuestionCreateData {
  /** 题目分组 */
  title: string;
  body: string;
  type: string;
  difficulty: number;
  conceptIds: string[];
  tags: string[];
  answer: string;
  answerType: string;
  hints: string[];
  solutionSteps: string[];
  options?: string[];
  estimatedTimeSeconds: number;
}

export interface QuestionUpdateData {
  title?: string;
  body?: string;
  type?: string;
  difficulty?: number;
  conceptIds?: string[];
  tags?: string[];
  answer?: string;
  answerType?: string;
  hints?: string[];
  solutionSteps?: string[];
  options?: string[];
  estimatedTimeSeconds?: number;
  status?: string;
}

export interface QuestionListParams {
  page: number;
  pageSize: number;
  search?: string;
  chapter?: string;
  difficulty?: string;
  type?: string;
  status?: string;
  tags?: string[];
  group?: string;
  sortBy?: string;
  sortOrder?: 'asc' | 'desc';
}

export interface QuestionListResponse {
  items: Question[];
  total: number;
  page: number;
  pageSize: number;
}

export interface BatchOperationResult {
  success: number;
  failed: number;
  failedIds: string[];
  errors: string[];
}

export interface QuestionStats {
  total: number;
  byDifficulty: Record<string, number>;
  byType: Record<string, number>;
  byStatus: Record<string, number>;
}
