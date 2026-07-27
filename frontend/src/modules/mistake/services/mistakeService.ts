/**
 * 错题本服务
 *
 * 对接后端 /mistakes API
 */

import { apiClient } from '@/libs/http/apiClient';
import { logger } from '@/libs/utils/logger';

const mistakeLogger = logger.createContextLogger('MistakeService');

// ========== 类型定义 ==========

export interface MistakeExercise {
  id: string;
  title: string;
  content: string;
  difficulty: number;
  knowledgePoints: string[];
}

export interface MistakeAttempt {
  studentAnswer: string;
  correctAnswer: string;
  isCorrect: boolean;
  score: number;
  submittedAt: string | null;
  timeSpentSeconds: number;
}

export interface MistakeDiagnosis {
  errorType: string | null;
  errorSubtype: string;
  severity: string;
  explanation: string;
  suggestion: string;
  relatedConcepts: string[];
}

export interface MistakeMastery {
  current: number;
  previous: number;
  trend: 'improving' | 'declining' | 'stable';
}

export interface MistakeRecord {
  id: string;
  exercise: MistakeExercise;
  attempt: MistakeAttempt;
  diagnosis: MistakeDiagnosis;
  mastery: MistakeMastery;
  errorCount: number;
  lastReviewedAt: string | null;
}

export interface PaginationInfo {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
}

export interface MistakeStatistics {
  totalMistakes: number;
  weakConcepts: number;
  avgMastery: number;
}

export interface MistakeListResponse {
  items: MistakeRecord[];
  pagination: PaginationInfo;
  statistics: MistakeStatistics;
}

export interface ErrorTypeDistribution {
  count: number;
  percentage: number;
  label: string;
}

export interface ConceptWeakness {
  conceptId: string;
  conceptName: string;
  mistakeCount: number;
  mastery: number;
  recentMistakes: number;
}

export interface StatisticsOverview {
  totalMistakes: number;
  totalExercises: number;
  mistakeRate: number;
  avgMastery: number;
}

export interface MistakeStatisticsResponse {
  overview: StatisticsOverview;
  errorTypeDistribution: Record<string, ErrorTypeDistribution>;
  conceptWeakness: ConceptWeakness[];
}

export interface MistakeDetailExercise {
  id: string;
  title: string;
  content: string;
  difficulty: number;
  knowledgePoints: string[];
  hints: string[];
}

export interface MistakeDetailAttempt {
  studentAnswer: string;
  studentSteps: string[];
  correctAnswer: string;
  submittedAt: string | null;
  timeSpentSeconds: number;
}

export interface MistakeDetailDiagnosis {
  errorType: string | null;
  errorStepIndex: number | null;
  explanation: string;
  suggestion: string;
  relatedConcepts: string[];
}

export interface MistakeSolution {
  answer: string;
  steps: string[];
  source: string;
}

export interface MistakeHistory {
  attemptId: string;
  submittedAt: string | null;
  isCorrect: boolean;
  score: number;
}

export interface MistakeDetail {
  attemptId: string;
  exercise: MistakeDetailExercise;
  attempt: MistakeDetailAttempt;
  diagnosis: MistakeDetailDiagnosis;
  solution: MistakeSolution;
  history: MistakeHistory[];
}

export interface MarkAsMasteredResponse {
  success: boolean;
  masteredAt: string;
  masteryUpdate: Record<string, number>;
}

export interface ReviewExercise {
  id: string;
  title: string;
  content: string;
  difficulty: number;
  type: 'multiple_choice' | 'short_answer' | 'proof';
  knowledgePoints: string[];
  knowledgePointNames: string[];
  hintsAvailable: boolean;
  estimatedTimeSeconds: number;
  options: string[] | null;
}

export interface ReviewContext {
  isReview: boolean;
  originalAttemptId: string;
  previousAnswer: string;
  previousErrorType: string | null;
  previousExplanation: string;
  previousSuggestion: string;
  masteryBefore: number;
  errorCount: number;
}

export interface ReviewExerciseResponse {
  exercise: ReviewExercise;
  context: ReviewContext;
}

export interface MistakeQueryParams {
  page?: number;
  pageSize?: number;
  errorType?: string;
  conceptId?: string;
  difficultyMin?: number;
  difficultyMax?: number;
  dateFrom?: string;
  dateTo?: string;
  masteryStatus?: 'all' | 'weak' | 'improving' | 'mastered';
  sortBy?: 'time' | 'error_count' | 'mastery';
  sortOrder?: 'asc' | 'desc';
}

export interface ReviewParams {
  focusConcept?: string;
  focusErrorType?: string;
}

// ========== 后端原始响应（snake_case）==========

interface MistakeListResponseRaw {
  items: Array<{
    id: string;
    exercise: {
      id: string;
      title: string;
      content: string;
      difficulty: number;
      knowledge_points: string[] | null;
    };
    attempt: {
      student_answer: string;
      correct_answer: string;
      is_correct: boolean;
      score: number;
      submitted_at: string | null;
      time_spent_seconds: number;
    };
    diagnosis: {
      error_type: string | null;
      error_subtype: string;
      severity: string;
      explanation: string;
      suggestion: string;
      related_concepts: string[] | null;
    };
    mastery: {
      current: number;
      previous: number;
      trend: 'improving' | 'declining' | 'stable';
    };
    error_count: number;
    last_reviewed_at: string | null;
  }>;
  pagination: {
    page: number;
    page_size: number;
    total: number;
    total_pages: number;
  };
  statistics: {
    total_mistakes: number;
    weak_concepts: number;
    avg_mastery: number;
  };
}

interface MistakeStatisticsResponseRaw {
  overview: {
    total_mistakes: number;
    total_exercises: number;
    mistake_rate: number;
    avg_mastery: number;
  };
  error_type_distribution: Record<
    string,
    {
      count: number;
      percentage: number;
      label: string;
    }
  >;
  concept_weakness: Array<{
    concept_id: string;
    concept_name: string;
    mistake_count: number;
    mastery: number;
    recent_mistakes: number;
  }>;
}

interface MistakeDetailRaw {
  attempt_id: string;
  exercise: {
    id: string;
    title: string;
    content: string;
    difficulty: number;
    knowledge_points: string[] | null;
    hints: string[] | null;
  };
  attempt: {
    student_answer: string;
    student_steps: string[] | null;
    correct_answer: string;
    submitted_at: string | null;
    time_spent_seconds: number;
  };
  diagnosis: {
    error_type: string | null;
    error_step_index: number | null;
    explanation: string;
    suggestion: string;
    related_concepts: string[] | null;
  };
  solution: {
    answer: string;
    steps: string[] | null;
    source: string;
  };
  history: Array<{
    attempt_id: string;
    submitted_at: string | null;
    is_correct: boolean;
    score: number;
  }>;
}

interface MarkAsMasteredResponseRaw {
  success: boolean;
  mastered_at: string;
  mastery_update: Record<string, number>;
}

interface ReviewExerciseResponseRaw {
  exercise: {
    id: string;
    title: string;
    content: string;
    difficulty: number;
    type: string;
    knowledge_points: string[] | null;
    knowledge_point_names: string[] | null;
    hints_available: boolean;
    estimated_time_seconds: number;
    options: string[] | null;
  };
  context: {
    is_review: boolean;
    original_attempt_id: string;
    previous_answer: string;
    previous_error_type: string | null;
    previous_explanation: string;
    previous_suggestion: string;
    mastery_before: number;
    error_count: number;
  };
}

// ========== 数据映射（snake_case -> camelCase）==========

function mapMistakeRecord(raw: MistakeListResponseRaw['items'][number]): MistakeRecord {
  return {
    id: raw.id,
    exercise: {
      id: raw.exercise.id,
      title: raw.exercise.title,
      content: raw.exercise.content,
      difficulty: raw.exercise.difficulty,
      knowledgePoints: raw.exercise.knowledge_points ?? [],
    },
    attempt: {
      studentAnswer: raw.attempt.student_answer,
      correctAnswer: raw.attempt.correct_answer,
      isCorrect: raw.attempt.is_correct,
      score: raw.attempt.score,
      submittedAt: raw.attempt.submitted_at,
      timeSpentSeconds: raw.attempt.time_spent_seconds,
    },
    diagnosis: {
      errorType: raw.diagnosis.error_type,
      errorSubtype: raw.diagnosis.error_subtype,
      severity: raw.diagnosis.severity,
      explanation: raw.diagnosis.explanation,
      suggestion: raw.diagnosis.suggestion,
      relatedConcepts: raw.diagnosis.related_concepts ?? [],
    },
    mastery: raw.mastery,
    errorCount: raw.error_count,
    lastReviewedAt: raw.last_reviewed_at,
  };
}

function mapMistakeListResponse(raw: MistakeListResponseRaw): MistakeListResponse {
  return {
    items: raw.items.map(mapMistakeRecord),
    pagination: {
      page: raw.pagination.page,
      pageSize: raw.pagination.page_size,
      total: raw.pagination.total,
      totalPages: raw.pagination.total_pages,
    },
    statistics: {
      totalMistakes: raw.statistics.total_mistakes,
      weakConcepts: raw.statistics.weak_concepts,
      avgMastery: raw.statistics.avg_mastery,
    },
  };
}

function mapMistakeStatisticsResponse(raw: MistakeStatisticsResponseRaw): MistakeStatisticsResponse {
  return {
    overview: {
      totalMistakes: raw.overview.total_mistakes,
      totalExercises: raw.overview.total_exercises,
      mistakeRate: raw.overview.mistake_rate,
      avgMastery: raw.overview.avg_mastery,
    },
    errorTypeDistribution: raw.error_type_distribution,
    conceptWeakness: raw.concept_weakness.map((c) => ({
      conceptId: c.concept_id,
      conceptName: c.concept_name,
      mistakeCount: c.mistake_count,
      mastery: c.mastery,
      recentMistakes: c.recent_mistakes,
    })),
  };
}

function mapMistakeDetail(raw: MistakeDetailRaw): MistakeDetail {
  return {
    attemptId: raw.attempt_id,
    exercise: {
      id: raw.exercise.id,
      title: raw.exercise.title,
      content: raw.exercise.content,
      difficulty: raw.exercise.difficulty,
      knowledgePoints: raw.exercise.knowledge_points ?? [],
      hints: raw.exercise.hints ?? [],
    },
    attempt: {
      studentAnswer: raw.attempt.student_answer,
      studentSteps: raw.attempt.student_steps ?? [],
      correctAnswer: raw.attempt.correct_answer,
      submittedAt: raw.attempt.submitted_at,
      timeSpentSeconds: raw.attempt.time_spent_seconds,
    },
    diagnosis: {
      errorType: raw.diagnosis.error_type,
      errorStepIndex: raw.diagnosis.error_step_index,
      explanation: raw.diagnosis.explanation,
      suggestion: raw.diagnosis.suggestion,
      relatedConcepts: raw.diagnosis.related_concepts ?? [],
    },
    solution: {
      answer: raw.solution.answer,
      steps: raw.solution.steps ?? [],
      source: raw.solution.source,
    },
    history: raw.history.map((h) => ({
      attemptId: h.attempt_id,
      submittedAt: h.submitted_at,
      isCorrect: h.is_correct,
      score: h.score,
    })),
  };
}

function mapReviewExerciseResponse(raw: ReviewExerciseResponseRaw): ReviewExerciseResponse {
  return {
    exercise: {
      id: raw.exercise.id,
      title: raw.exercise.title,
      content: raw.exercise.content,
      difficulty: raw.exercise.difficulty,
      type: raw.exercise.type as ReviewExercise['type'],
      knowledgePoints: raw.exercise.knowledge_points ?? [],
      knowledgePointNames: raw.exercise.knowledge_point_names ?? [],
      hintsAvailable: raw.exercise.hints_available,
      estimatedTimeSeconds: raw.exercise.estimated_time_seconds,
      options: raw.exercise.options,
    },
    context: {
      isReview: raw.context.is_review,
      originalAttemptId: raw.context.original_attempt_id,
      previousAnswer: raw.context.previous_answer,
      previousErrorType: raw.context.previous_error_type,
      previousExplanation: raw.context.previous_explanation,
      previousSuggestion: raw.context.previous_suggestion,
      masteryBefore: raw.context.mastery_before,
      errorCount: raw.context.error_count,
    },
  };
}

// ========== API 方法 ==========

/**
 * 获取错题列表
 */
export async function fetchMistakes(
  params: MistakeQueryParams = {}
): Promise<MistakeListResponse> {
  mistakeLogger.info('Fetching mistakes', { params });

  try {
    const response = await apiClient.get<MistakeListResponseRaw>('/mistakes', {
      params: {
        page: params.page || 1,
        page_size: params.pageSize || 20,
        error_type: params.errorType,
        concept_id: params.conceptId,
        difficulty_min: params.difficultyMin,
        difficulty_max: params.difficultyMax,
        date_from: params.dateFrom,
        date_to: params.dateTo,
        mastery_status: params.masteryStatus || 'all',
        sort_by: params.sortBy || 'time',
        sort_order: params.sortOrder || 'desc',
      },
    });

    const mapped = mapMistakeListResponse(response.data);

    mistakeLogger.info('Mistakes fetched successfully', {
      total: mapped.pagination.total,
    });

    return mapped;
  } catch (error) {
    mistakeLogger.error('Failed to fetch mistakes', { error });
    throw error;
  }
}

/**
 * 获取错题统计
 */
export async function fetchStatistics(
  timeRange: string = 'month'
): Promise<MistakeStatisticsResponse> {
  mistakeLogger.info('Fetching mistake statistics', { timeRange });

  try {
    const response = await apiClient.get<MistakeStatisticsResponseRaw>(
      '/mistakes/statistics',
      {
        params: { time_range: timeRange },
      }
    );

    mistakeLogger.info('Statistics fetched successfully');

    return mapMistakeStatisticsResponse(response.data);
  } catch (error) {
    mistakeLogger.error('Failed to fetch statistics', { error });
    throw error;
  }
}

/**
 * 获取错题详情
 */
export async function fetchMistakeDetail(
  attemptId: string
): Promise<MistakeDetail> {
  mistakeLogger.info('Fetching mistake detail', { attemptId });

  try {
    const response = await apiClient.get<MistakeDetailRaw>(
      `/mistakes/${attemptId}`
    );

    mistakeLogger.info('Mistake detail fetched successfully');

    return mapMistakeDetail(response.data);
  } catch (error) {
    mistakeLogger.error('Failed to fetch mistake detail', { error });
    throw error;
  }
}

/**
 * 标记错题已掌握
 */
export async function markAsMastered(
  attemptId: string
): Promise<MarkAsMasteredResponse> {
  mistakeLogger.info('Marking mistake as mastered', { attemptId });

  try {
    const response = await apiClient.post<MarkAsMasteredResponseRaw>(
      `/mistakes/${attemptId}/master`
    );

    mistakeLogger.info('Mistake marked as mastered successfully');

    return {
      success: response.data.success,
      masteredAt: response.data.mastered_at,
      masteryUpdate: response.data.mastery_update,
    };
  } catch (error) {
    mistakeLogger.error('Failed to mark mistake as mastered', { error });
    throw error;
  }
}

/**
 * 删除错题
 */
export async function deleteMistake(attemptId: string): Promise<void> {
  mistakeLogger.info('Deleting mistake', { attemptId });

  try {
    await apiClient.delete(`/mistakes/${attemptId}`);

    mistakeLogger.info('Mistake deleted successfully');
  } catch (error) {
    mistakeLogger.error('Failed to delete mistake', { error });
    throw error;
  }
}

/**
 * 获取复习题目
 */
export async function fetchReviewExercise(
  params: ReviewParams = {}
): Promise<ReviewExerciseResponse> {
  mistakeLogger.info('Fetching review exercise', { params });

  try {
    const response = await apiClient.get<ReviewExerciseResponseRaw>(
      '/mistakes/review/next',
      {
        params: {
          focus_concept: params.focusConcept,
          focus_error_type: params.focusErrorType,
        },
      }
    );

    mistakeLogger.info('Review exercise fetched successfully');

    return mapReviewExerciseResponse(response.data);
  } catch (error) {
    mistakeLogger.error('Failed to fetch review exercise', { error });
    throw error;
  }
}

/**
 * 获取指定错题的重做内容
 */
export async function fetchReviewExerciseByAttempt(
  attemptId: string,
  signal?: AbortSignal
): Promise<ReviewExerciseResponse> {
  const normalizedAttemptId = attemptId.trim();
  if (!normalizedAttemptId) {
    throw new Error('缺少错题记录 ID');
  }

  mistakeLogger.info('Fetching exact mistake review exercise', {
    attemptId: normalizedAttemptId,
  });

  try {
    const response = await apiClient.get<ReviewExerciseResponseRaw>(
      `/mistakes/${encodeURIComponent(normalizedAttemptId)}/review`,
      { signal }
    );
    return mapReviewExerciseResponse(response.data);
  } catch (error) {
    mistakeLogger.error('Failed to fetch exact mistake review exercise', { error });
    throw error;
  }
}

// 默认导出
export const mistakeService = {
  fetchMistakes,
  fetchStatistics,
  fetchMistakeDetail,
  markAsMastered,
  deleteMistake,
  fetchReviewExercise,
  fetchReviewExerciseByAttempt,
};

export default mistakeService;
