import { useState, useCallback, useRef } from 'react';
import axios from 'axios';
import {
  exerciseService,
  type Question,
  type SubmitResult,
} from '@/modules/exercise/services/exerciseService';
import { logger } from '@/libs/utils/logger';
import { getApiErrorMessage } from '@/libs/http/apiClient';

const exerciseLogger = logger.createContextLogger('ExerciseViewModel');

/**
 * 练习题错误类型
 */
export type ExerciseErrorType =
  | 'not_enrolled' // 403: 未加入班级
  | 'no_questions' // 无可用题目
  | 'invalid_generation_request' // AI 出题参数无效
  | 'knowledge_point_not_found' // 404: 所选知识点不存在
  | 'generation_rate_limited' // 429: AI 出题请求过于频繁
  | 'generation_unavailable' // 503: AI 出题服务不可用
  | 'network_error' // 网络错误
  | 'unknown'; // 其他错误

const getGenerationError = (err: unknown): { message: string; type: ExerciseErrorType } => {
  if (!axios.isAxiosError(err)) {
    return {
      message: getApiErrorMessage(err, '生成题目失败，请稍后重试'),
      type: 'unknown',
    };
  }

  const status = err.response?.status;
  if (status === 404) {
    return {
      message: '所选知识点不存在，请重新选择',
      type: 'knowledge_point_not_found',
    };
  }
  if (status === 429) {
    return {
      message: 'AI 出题请求过于频繁，请稍后再试',
      type: 'generation_rate_limited',
    };
  }
  if (status === 503) {
    return {
      message: 'AI 出题服务暂不可用，请稍后重试',
      type: 'generation_unavailable',
    };
  }
  if (!err.response) {
    return {
      message: '无法连接到服务器，请检查网络后重试',
      type: 'network_error',
    };
  }

  return {
    message: getApiErrorMessage(err, '生成题目失败，请稍后重试'),
    type: 'unknown',
  };
};

/**
 * 练习题 ViewModel Hook
 *
 * 管理题目加载、文本答案提交和反馈展示
 */
export function useExerciseViewModel() {
  const [currentQuestion, setCurrentQuestion] = useState<Question | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitResult, setSubmitResult] = useState<SubmitResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [errorType, setErrorType] = useState<ExerciseErrorType | null>(null);

  // 答题计时
  const startTimeRef = useRef<number>(Date.now());
  const generationInFlightRef = useRef(false);

  const loadNextQuestion = useCallback(async (conceptId?: string, difficulty?: number) => {
    setIsLoading(true);
    setError(null);
    setErrorType(null);
    try {
      const question = await exerciseService.fetchNextQuestion(conceptId, difficulty);
      if (!question) {
        setCurrentQuestion(null);
        setSubmitResult(null);
        setError(null);
        setErrorType('no_questions');
        exerciseLogger.info('No questions available');
        return;
      }
      setCurrentQuestion(question);
      setSubmitResult(null);
      startTimeRef.current = Date.now();
      exerciseLogger.debug('Question loaded', { questionId: question.id });
    } catch (err) {
      const msg = getApiErrorMessage(err, '加载题目失败，请稍后重试');
      setError(msg);

      // 识别错误类型
      if (axios.isAxiosError(err)) {
        const status = err.response?.status;
        if (status === 403) {
          setErrorType('not_enrolled');
        } else if (status === 404) {
          setErrorType('no_questions');
        } else if (!err.response) {
          setErrorType('network_error');
        } else {
          setErrorType('unknown');
        }
      } else {
        setErrorType('unknown');
      }

      exerciseLogger.error('Failed to load question', { error: err });
    } finally {
      setIsLoading(false);
    }
  }, []);

  const generateQuestion = useCallback(async (conceptId: string, difficulty: number) => {
    if (generationInFlightRef.current) return;

    const normalizedConceptId = conceptId.trim();
    if (!normalizedConceptId) {
      setError('请选择知识点');
      setErrorType('invalid_generation_request');
      return;
    }
    if (!Number.isFinite(difficulty) || difficulty < 0 || difficulty > 1) {
      setError('请选择有效难度');
      setErrorType('invalid_generation_request');
      return;
    }

    generationInFlightRef.current = true;
    setIsGenerating(true);
    setError(null);
    setErrorType(null);

    try {
      const question = await exerciseService.generateQuestion({
        conceptId: normalizedConceptId,
        difficulty,
      });
      setCurrentQuestion(question);
      setSubmitResult(null);
      startTimeRef.current = Date.now();
      exerciseLogger.info('AI question generated', {
        questionId: question.id,
        conceptId: normalizedConceptId,
        difficulty,
      });
    } catch (err) {
      const generationError = getGenerationError(err);
      setError(generationError.message);
      setErrorType(generationError.type);
      exerciseLogger.error('Failed to generate question', { error: err });
    } finally {
      generationInFlightRef.current = false;
      setIsGenerating(false);
    }
  }, []);

  const submitAnswer = useCallback(
    async (answerText: string) => {
      if (!currentQuestion) return;
      if (!answerText.trim()) {
        setError('请输入答案');
        return;
      }

      setIsSubmitting(true);
      setError(null);
      setErrorType(null);
      try {
        const timeSpent = Math.round((Date.now() - startTimeRef.current) / 1000);
        const result = await exerciseService.submitAnswer({
          exerciseId: currentQuestion.id,
          answerText,
          timeSpentSeconds: timeSpent,
        });
        setSubmitResult(result);
        exerciseLogger.info('Answer submitted', {
          questionId: currentQuestion.id,
          isCorrect: result.isCorrect,
        });
      } catch (err) {
        const msg = getApiErrorMessage(err, '提交答案失败，请稍后重试');
        setError(msg);
        exerciseLogger.error('Failed to submit answer', {
          questionId: currentQuestion.id,
          error: err,
        });
      } finally {
        setIsSubmitting(false);
      }
    },
    [currentQuestion]
  );

  return {
    currentQuestion,
    isLoading,
    isGenerating,
    isSubmitting,
    submitResult,
    error,
    errorType,
    loadNextQuestion,
    generateQuestion,
    submitAnswer,
  };
}
