/**
 * 练习服务
 *
 * 对接后端 /exercise API，替代原有 Mock 实现
 */

import { apiClient } from '@/libs/http/apiClient';
import { logger } from '@/libs/utils/logger';

const exerciseLogger = logger.createContextLogger('ExerciseService');

// ========== 类型定义 ==========

export interface Question {
  id: string;
  title: string;
  content: string; // LaTeX
  difficulty: number;
  type: 'multiple_choice' | 'short_answer' | 'proof';
  source: 'class' | 'ai_generated';
  knowledgePoints: string[];
  knowledgePointNames: string[];
  hintsAvailable: boolean;
  estimatedTimeSeconds: number;
  options?: string[] | null;
}

interface QuestionResponse {
  id: string;
  title: string;
  content: string;
  difficulty: number;
  type: string;
  source?: Question['source'];
  knowledge_points?: string[];
  knowledge_point_names?: string[];
  hints_available: boolean;
  estimated_time_seconds: number;
  options?: string[] | null;
}

export interface GenerateQuestionPayload {
  conceptId: string;
  difficulty: number;
}

export interface DiagnosisDetail {
  errorType: string | null;
  errorSubtype?: string;
  taxonomyCode?: string;
  errorDescription: string;
  errorStepIndex: number | null;
  severity: string;
  suggestion: string;
  relatedConcepts: string[];
}

export interface SubmitResult {
  isCorrect: boolean;
  score: number;
  studentAnswerLatex: string;
  correctAnswerLatex: string;
  diagnosis: DiagnosisDetail | null;
  feedback: string;
  masteryUpdate: Record<string, number> | null;
  masteryModel: string;
  nextRecommendation: 'continue' | 'review' | 'advance';
}

export interface SubmitPayload {
  exerciseId: string;
  answerText: string;
  answerSteps?: string[];
  timeSpentSeconds: number;
}

const mapQuestion = (data: QuestionResponse): Question => ({
  id: data.id,
  title: data.title,
  content: data.content,
  difficulty: data.difficulty,
  type: data.type as Question['type'],
  source: data.source ?? 'class',
  knowledgePoints: data.knowledge_points ?? [],
  knowledgePointNames: data.knowledge_point_names ?? [],
  hintsAvailable: data.hints_available,
  estimatedTimeSeconds: data.estimated_time_seconds,
  options: data.options,
});

// ========== API 调用 ==========

export const exerciseService = {
  /**
   * 获取下一道自适应练习题
   */
  async fetchNextQuestion(conceptId?: string, difficulty?: number): Promise<Question | null> {
    exerciseLogger.debug('Fetching next question', { conceptId, difficulty });

    const params: Record<string, string> = {};
    if (conceptId) params.concept_id = conceptId;
    if (difficulty !== undefined) params.difficulty = String(difficulty);

    const res = await apiClient.get<QuestionResponse | null>('/exercise/next', {
      params,
    });

    const data = res.data;
    if (!data) {
      return null;
    }
    return mapQuestion(data);
  },

  /**
   * 按指定知识点和难度生成一道学生自主练习题
   */
  async generateQuestion(payload: GenerateQuestionPayload): Promise<Question> {
    exerciseLogger.debug('Generating question', {
      conceptId: payload.conceptId,
      difficulty: payload.difficulty,
    });

    const res = await apiClient.post<QuestionResponse>('/exercise/generate', {
      concept_id: payload.conceptId,
      difficulty: payload.difficulty,
    });

    return mapQuestion(res.data);
  },

  /**
   * 提交答案
   */
  async submitAnswer(payload: SubmitPayload): Promise<SubmitResult> {
    exerciseLogger.debug('Submitting answer', {
      exerciseId: payload.exerciseId,
    });

    const res = await apiClient.post<{
      is_correct: boolean;
      score: number;
      student_answer_latex: string;
      correct_answer_latex: string;
      diagnosis: {
        error_type: string | null;
        error_subtype?: string;
        taxonomy_code?: string;
        error_description: string;
        error_step_index: number | null;
        severity: string;
        suggestion: string;
        related_concepts: string[];
      } | null;
      feedback: string;
      mastery_update: Record<string, number> | null;
      mastery_model: string;
      next_recommendation: string;
    }>('/exercise/submit', {
      exercise_id: payload.exerciseId,
      answer_text: payload.answerText,
      answer_steps: payload.answerSteps,
      time_spent_seconds: payload.timeSpentSeconds,
    });

    const data = res.data;
    return {
      isCorrect: data.is_correct,
      score: data.score,
      studentAnswerLatex: data.student_answer_latex,
      correctAnswerLatex: data.correct_answer_latex,
      diagnosis: data.diagnosis
        ? {
            errorType: data.diagnosis.error_type,
            errorSubtype: data.diagnosis.error_subtype,
            taxonomyCode: data.diagnosis.taxonomy_code,
            errorDescription: data.diagnosis.error_description,
            errorStepIndex: data.diagnosis.error_step_index,
            severity: data.diagnosis.severity,
            suggestion: data.diagnosis.suggestion,
            relatedConcepts: data.diagnosis.related_concepts,
          }
        : null,
      feedback: data.feedback,
      masteryUpdate: data.mastery_update,
      masteryModel: data.mastery_model,
      nextRecommendation: data.next_recommendation as SubmitResult['nextRecommendation'],
    };
  },

  /**
   * 获取题目解析
   */
  async getSolution(exerciseId: string): Promise<{
    answer: string;
    steps: string[];
  }> {
    const res = await apiClient.get<{
      exercise_id: string;
      answer: string;
      steps: string[];
      source: string;
    }>(`/exercise/${exerciseId}/solution`);

    return {
      answer: res.data.answer,
      steps: res.data.steps,
    };
  },
};

export default exerciseService;
