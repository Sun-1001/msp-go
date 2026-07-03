/**
 * 题目管理服务
 */

import { apiClient } from '@/libs/http/apiClient';
import { logger } from '@/libs/utils/logger';
import type {
  Question,
  QuestionCreateData,
  QuestionUpdateData,
  QuestionListParams,
  QuestionListResponse,
  BatchOperationResult,
  QuestionStats,
} from '@/modules/question/types/question';
import type { AIParseResponse } from '@/modules/question/types/questionImport';

const log = logger.createContextLogger('questionService');

/**
 * 后端响应类型
 */
interface QuestionResponse {
  id: string;
  title: string;
  body: string;
  type: string;
  difficulty: number;
  concept_ids?: string[];
  tags?: string[];
  status: string;
  meta: {
    answer: string;
    answer_type: string;
    hints: string[];
    solution_steps: string[];
    options?: string[];
    estimated_time_seconds: number;
  };
  created_at: string;
  updated_at: string;
  usage_count?: number;
  correct_rate?: number;
}

/**
 * 将后端响应转换为前端 Question 类型
 */
function transformQuestion(data: QuestionResponse): Question {
  return {
    id: data.id,
    title: data.title,
    body: data.body,
    type: data.type as 'short_answer' | 'multiple_choice' | 'proof',
    difficulty: data.difficulty,
    conceptIds: data.concept_ids || [],
    tags: data.tags || [],
    status: data.status as 'draft' | 'published' | 'archived',
    meta: {
      answer: data.meta.answer || '',
      answerType: data.meta.answer_type || 'expression',
      hints: data.meta.hints || [],
      solutionSteps: data.meta.solution_steps || [],
      options: data.meta.options,
      estimatedTimeSeconds: data.meta.estimated_time_seconds || 300,
    },
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    usageCount: data.usage_count || 0,
    correctRate: data.correct_rate || 0,
  };
}

/**
 * 后端创建请求类型
 */
interface QuestionCreateRequest {
  title: string;
  body: string;
  type: string;
  difficulty: number;
  concept_ids: string[];
  tags: string[];
  answer: string;
  answer_type: string;
  hints: string[];
  solution_steps: string[];
  options?: string[];
  estimated_time_seconds: number;
}

/**
 * 将前端创建数据转换为后端格式
 */
function transformCreateData(data: QuestionCreateData): QuestionCreateRequest {
  return {
    title: data.title,
    body: data.body,
    type: data.type,
    difficulty: data.difficulty,
    concept_ids: data.conceptIds,
    tags: data.tags,
    answer: data.answer,
    answer_type: data.answerType,
    hints: data.hints,
    solution_steps: data.solutionSteps,
    options: data.options,
    estimated_time_seconds: data.estimatedTimeSeconds,
  };
}

/**
 * 后端更新请求类型
 */
interface QuestionUpdateRequest {
  title?: string;
  body?: string;
  type?: string;
  difficulty?: number;
  concept_ids?: string[];
  tags?: string[];
  answer?: string;
  answer_type?: string;
  hints?: string[];
  solution_steps?: string[];
  options?: string[];
  estimated_time_seconds?: number;
  status?: string;
}

/**
 * 将前端更新数据转换为后端格式
 */
function transformUpdateData(data: QuestionUpdateData): QuestionUpdateRequest {
  const result: QuestionUpdateRequest = {};
  if (data.title !== undefined) result.title = data.title;
  if (data.body !== undefined) result.body = data.body;
  if (data.type !== undefined) result.type = data.type;
  if (data.difficulty !== undefined) result.difficulty = data.difficulty;
  if (data.conceptIds !== undefined) result.concept_ids = data.conceptIds;
  if (data.tags !== undefined) result.tags = data.tags;
  if (data.answer !== undefined) result.answer = data.answer;
  if (data.answerType !== undefined) result.answer_type = data.answerType;
  if (data.hints !== undefined) result.hints = data.hints;
  if (data.solutionSteps !== undefined) result.solution_steps = data.solutionSteps;
  if (data.options !== undefined) result.options = data.options;
  if (data.estimatedTimeSeconds !== undefined)
    result.estimated_time_seconds = data.estimatedTimeSeconds;
  if (data.status !== undefined) result.status = data.status;
  return result;
}

/**
 * 统计响应类型
 */
interface QuestionStatsResponse {
  total: number;
  by_difficulty: Record<string, number>;
  by_type: Record<string, number>;
  by_status: Record<string, number>;
}

/**
 * 批量操作响应类型
 */
interface BatchOperationResponse {
  success: number;
  failed: number;
  failed_ids?: string[];
  errors?: string[];
}

/**
 * 后端列表响应类型
 */
interface QuestionListBackendResponse {
  items: QuestionResponse[];
  total: number;
  page: number;
  page_size: number;
}

/**
 * 后端查询参数类型
 */
interface QuestionQueryParams {
  page: number;
  page_size: number;
  search?: string;
  chapter?: string;
  difficulty?: string;
  type?: string;
  status?: string;
  tags?: string[];
  group?: string;
  sort_by?: string;
  sort_order?: string;
}

export const questionService = {
  /**
   * 创建题目
   */
  async createQuestion(data: QuestionCreateData): Promise<Question> {
    log.info('创建题目', { title: data.title });
    try {
      const response = await apiClient.post<QuestionResponse>('/questions', transformCreateData(data));
      log.info('题目创建成功', { id: response.data.id });
      return transformQuestion(response.data);
    } catch (error) {
      log.error('创建题目失败', error);
      throw error;
    }
  },

  /**
   * 获取题目详情
   */
  async getQuestion(id: string): Promise<Question> {
    log.info('获取题目详情', { id });
    try {
      const response = await apiClient.get<QuestionResponse>(`/questions/${id}`);
      return transformQuestion(response.data);
    } catch (error) {
      log.error('获取题目详情失败', error);
      throw error;
    }
  },

  /**
   * 更新题目
   */
  async updateQuestion(id: string, data: QuestionUpdateData): Promise<Question> {
    log.info('更新题目', { id });
    try {
      const response = await apiClient.put<QuestionResponse>(`/questions/${id}`, transformUpdateData(data));
      log.info('题目更新成功', { id });
      return transformQuestion(response.data);
    } catch (error) {
      log.error('更新题目失败', error);
      throw error;
    }
  },

  /**
   * 删除题目
   */
  async deleteQuestion(id: string): Promise<void> {
    log.info('删除题目', { id });
    try {
      await apiClient.delete(`/questions/${id}`);
      log.info('题目删除成功', { id });
    } catch (error) {
      log.error('删除题目失败', error);
      throw error;
    }
  },

  /**
   * 获取题目列表
   */
  async listQuestions(params: QuestionListParams): Promise<QuestionListResponse> {
    log.info('获取题目列表', params);
    try {
      const queryParams: QuestionQueryParams = {
        page: params.page,
        page_size: params.pageSize,
      };
      if (params.search) queryParams.search = params.search;
      if (params.chapter) queryParams.chapter = params.chapter;
      if (params.difficulty) queryParams.difficulty = params.difficulty;
      if (params.type) queryParams.type = params.type;
      if (params.status) queryParams.status = params.status;
      if (params.tags && params.tags.length > 0) queryParams.tags = params.tags;
      if (params.group) queryParams.group = params.group;
      if (params.sortBy) queryParams.sort_by = params.sortBy;
      if (params.sortOrder) queryParams.sort_order = params.sortOrder;

      const response = await apiClient.get<QuestionListBackendResponse>('/questions', { params: queryParams });
      return {
        items: response.data.items.map(transformQuestion),
        total: response.data.total,
        page: response.data.page,
        pageSize: response.data.page_size,
      };
    } catch (error) {
      log.error('获取题目列表失败', error);
      throw error;
    }
  },

  /**
   * 批量发布题目
   */
  async batchPublish(ids: string[]): Promise<BatchOperationResult> {
    log.info('批量发布题目', { count: ids.length });
    try {
      const response = await apiClient.post<BatchOperationResponse>('/questions/batch/publish', {
        question_ids: ids,
      });
      return {
        success: response.data.success,
        failed: response.data.failed,
        failedIds: response.data.failed_ids || [],
        errors: response.data.errors || [],
      };
    } catch (error) {
      log.error('批量发布题目失败', error);
      throw error;
    }
  },

  /**
   * 批量删除题目
   */
  async batchDelete(ids: string[]): Promise<BatchOperationResult> {
    log.info('批量删除题目', { count: ids.length });
    try {
      const response = await apiClient.post<BatchOperationResponse>('/questions/batch/delete', {
        question_ids: ids,
      });
      return {
        success: response.data.success,
        failed: response.data.failed,
        failedIds: response.data.failed_ids || [],
        errors: response.data.errors || [],
      };
    } catch (error) {
      log.error('批量删除题目失败', error);
      throw error;
    }
  },

  /**
   * 批量复制题目
   */
  async batchDuplicate(ids: string[]): Promise<BatchOperationResult> {
    log.info('批量复制题目', { count: ids.length });
    try {
      const response = await apiClient.post<BatchOperationResponse>('/questions/batch/duplicate', {
        question_ids: ids,
      });
      return {
        success: response.data.success,
        failed: response.data.failed,
        failedIds: response.data.failed_ids || [],
        errors: response.data.errors || [],
      };
    } catch (error) {
      log.error('批量复制题目失败', error);
      throw error;
    }
  },

  /**
   * 批量导入题目（前端解析后的 JSON 数组）
   */
  async batchImport(questions: QuestionCreateData[]): Promise<BatchOperationResult> {
    log.info('批量导入题目', { count: questions.length });
    try {
      const backendQuestions = questions.map(transformCreateData);
      const response = await apiClient.post<BatchOperationResponse>('/questions/batch/import', {
        questions: backendQuestions,
      });
      return {
        success: response.data.success,
        failed: response.data.failed,
        failedIds: response.data.failed_ids || [],
        errors: response.data.errors || [],
      };
    } catch (error) {
      log.error('批量导入题目失败', error);
      throw error;
    }
  },

  /**
   * AI 辅助识别题目
   */
  async aiParseQuestions(rawTexts: string[]): Promise<AIParseResponse> {
    log.info('AI 识别题目', { count: rawTexts.length });
    try {
      const response = await apiClient.post<AIParseResponse>('/questions/ai-parse', {
        raw_texts: rawTexts,
      });
      return response.data;
    } catch (error) {
      log.error('AI 识别题目失败', error);
      throw error;
    }
  },

  /**
   * 获取题目分组列表
   */
  async getGroups(): Promise<string[]> {
    log.info('获取题目分组列表');
    try {
      const response = await apiClient.get<{ groups: string[] }>('/questions/groups');
      return response.data.groups;
    } catch (error) {
      log.error('获取题目分组列表失败', error);
      throw error;
    }
  },

  /**
   * 获取题目统计
   */
  async getStats(): Promise<QuestionStats> {
    log.info('获取题目统计');
    try {
      const response = await apiClient.get<QuestionStatsResponse>('/questions/stats');
      return {
        total: response.data.total,
        byDifficulty: response.data.by_difficulty,
        byType: response.data.by_type,
        byStatus: response.data.by_status,
      };
    } catch (error) {
      log.error('获取题目统计失败', error);
      throw error;
    }
  },
};

export default questionService;
