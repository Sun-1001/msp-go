import { useState, useCallback, useRef } from 'react';
import axios from 'axios';
import {
  exerciseService,
  type ExerciseSolution,
  type Question,
  type SubmitResult,
} from '@/modules/exercise/services/exerciseService';
import { logger } from '@/libs/utils/logger';
import { getApiErrorMessage } from '@/libs/http/apiClient';
import { uploadService } from '@/modules/upload/services/uploadService';
import { validateAnswerImageFile } from '../utils/answerImageValidation';

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
  | 'invalid_answer' // 答案或图片不符合要求
  | 'answer_unreadable' // OCR 未识别出有效答案
  | 'answer_unsupported' // 当前答案或题型无法自动判定
  | 'answer_rate_limited' // OCR 请求被限流
  | 'answer_service_unavailable' // OCR 或数学判定服务不可用
  | 'exercise_changed' // 作答期间题目内容发生变化
  | 'network_error' // 网络错误
  | 'unknown'; // 其他错误

export type SubmitPhase = 'idle' | 'uploading' | 'recognizing';

export type ExerciseErrorSource = 'load' | 'generation' | 'submission';

export interface ExerciseAnswerSubmission {
  answerText?: string;
  answerImage?: File | null;
}

type SubmissionStage = 'upload' | 'grading';

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

const getErrorCode = (err: unknown): string => {
  if (!axios.isAxiosError(err)) return '';
  const data = err.response?.data as { code?: unknown } | undefined;
  return typeof data?.code === 'string' ? data.code.trim().toUpperCase() : '';
};

const getSubmissionError = (
  err: unknown,
  stage: SubmissionStage
): { message: string; type: ExerciseErrorType } => {
  const code = getErrorCode(err);
  switch (code) {
    case 'OCR_UNREADABLE':
      return {
        message: '未能从图片中识别出有效答案，请重新拍摄或改用文字答案',
        type: 'answer_unreadable',
      };
    case 'OCR_UNAVAILABLE':
      return {
        message: '图片识别服务暂不可用，请稍后重试或改用文字答案',
        type: 'answer_service_unavailable',
      };
    case 'OCR_TIMEOUT':
      return {
        message: '图片识别超时，请稍后重试',
        type: 'answer_service_unavailable',
      };
    case 'OCR_RATE_LIMITED':
      return {
        message: '图片识别请求过于频繁，请稍后重试',
        type: 'answer_rate_limited',
      };
    case 'RATE_LIMITED':
      return {
        message:
          stage === 'upload'
            ? '图片上传请求过于频繁，请稍后重试'
            : '答案提交请求过于频繁，请稍后重试',
        type: 'answer_rate_limited',
      };
    case 'ANSWER_PARSE_FAILED':
      return {
        message: '答案格式无法安全解析，请检查输入或改用更清晰的图片后重试',
        type: 'invalid_answer',
      };
    case 'MATH_UNSUPPORTED':
      return {
        message: '当前题型暂不支持自动判定，请补充解题步骤或联系教师',
        type: 'answer_unsupported',
      };
    case 'MATH_SOLVER_INVALID_RESPONSE':
      return {
        message: '数学判题服务返回异常，请稍后重试',
        type: 'answer_service_unavailable',
      };
    case 'MATH_SOLVER_UNAVAILABLE':
      return {
        message: '数学判题服务暂不可用，请稍后重试',
        type: 'answer_service_unavailable',
      };
    case 'MATH_SOLVER_TIMEOUT':
      return {
        message: '数学判题服务处理超时，请稍后重试',
        type: 'answer_service_unavailable',
      };
    case 'EXERCISE_CHANGED':
      return {
        message: '题目已更新，请重新加载后提交',
        type: 'exercise_changed',
      };
    default:
      if (code.startsWith('MATH_')) {
        return {
          message: '暂时无法可靠判定这份答案，请稍后重试',
          type: 'answer_service_unavailable',
        };
      }
  }

  if (axios.isAxiosError(err) && !err.response) {
    return {
      message: '无法连接到服务器，请检查网络后重试',
      type: 'network_error',
    };
  }
  return {
    message: getApiErrorMessage(err, '提交答案失败，请稍后重试'),
    type: 'unknown',
  };
};

/**
 * 练习题 ViewModel Hook
 *
 * 管理题目加载、文本或图片答案提交和反馈展示
 */
export function useExerciseViewModel() {
  const [currentQuestion, setCurrentQuestion] = useState<Question | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isGenerating, setIsGenerating] = useState(false);
  const [submitPhase, setSubmitPhase] = useState<SubmitPhase>('idle');
  const [submitResult, setSubmitResult] = useState<SubmitResult | null>(null);
  const [solution, setSolution] = useState<ExerciseSolution | null>(null);
  const [isLoadingSolution, setIsLoadingSolution] = useState(false);
  const [solutionError, setSolutionError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [errorType, setErrorType] = useState<ExerciseErrorType | null>(null);
  const [errorSource, setErrorSource] = useState<ExerciseErrorSource | null>(null);

  // 答题计时
  const startTimeRef = useRef<number>(Date.now());
  const generationInFlightRef = useRef(false);
  const submissionInFlightRef = useRef(false);
  const solutionInFlightRef = useRef(false);
  const questionVersionRef = useRef(0);

  const loadNextQuestion = useCallback(async (conceptId?: string, difficulty?: number) => {
    setIsLoading(true);
    setError(null);
    setErrorType(null);
    setErrorSource(null);
    try {
      const question = await exerciseService.fetchNextQuestion(conceptId, difficulty);
      if (!question) {
        questionVersionRef.current += 1;
        setCurrentQuestion(null);
        setSubmitResult(null);
        setSolution(null);
        setSolutionError(null);
        setError(null);
        setErrorType('no_questions');
        setErrorSource('load');
        exerciseLogger.info('No questions available');
        return;
      }
      questionVersionRef.current += 1;
      setCurrentQuestion(question);
      setSubmitResult(null);
      setSolution(null);
      setSolutionError(null);
      startTimeRef.current = Date.now();
      exerciseLogger.debug('Question loaded', { questionId: question.id });
    } catch (err) {
      const msg = getApiErrorMessage(err, '加载题目失败，请稍后重试');
      setError(msg);
      setErrorSource('load');

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
    if (generationInFlightRef.current || submissionInFlightRef.current) return;

    const normalizedConceptId = conceptId.trim();
    if (!normalizedConceptId) {
      setError('请选择知识点');
      setErrorType('invalid_generation_request');
      setErrorSource('generation');
      return;
    }
    if (!Number.isFinite(difficulty) || difficulty < 0 || difficulty > 1) {
      setError('请选择有效难度');
      setErrorType('invalid_generation_request');
      setErrorSource('generation');
      return;
    }

    generationInFlightRef.current = true;
    setIsGenerating(true);
    setError(null);
    setErrorType(null);
    setErrorSource(null);

    try {
      const question = await exerciseService.generateQuestion({
        conceptId: normalizedConceptId,
        difficulty,
      });
      questionVersionRef.current += 1;
      setCurrentQuestion(question);
      setSubmitResult(null);
      setSolution(null);
      setSolutionError(null);
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
      setErrorSource('generation');
      exerciseLogger.error('Failed to generate question', { error: err });
    } finally {
      generationInFlightRef.current = false;
      setIsGenerating(false);
    }
  }, []);

  const submitAnswer = useCallback(
    async ({ answerText, answerImage }: ExerciseAnswerSubmission) => {
      if (!currentQuestion || submissionInFlightRef.current || generationInFlightRef.current) return;

      const normalizedAnswer = answerText?.trim() ?? '';
      if (!normalizedAnswer && !answerImage) {
        setError('请输入答案或上传答案图片');
        setErrorType('invalid_answer');
        setErrorSource('submission');
        return;
      }

      const submittedQuestion = currentQuestion;
      const submittedQuestionVersion = questionVersionRef.current;
      let submissionStage: SubmissionStage = normalizedAnswer ? 'grading' : 'upload';
      submissionInFlightRef.current = true;
      setError(null);
      setErrorType(null);
      setErrorSource(null);
      setSubmitResult(null);
      setSolution(null);
      setSolutionError(null);
      try {
        let answerImageUrl: string | undefined;
        if (!normalizedAnswer && answerImage) {
          const validation = validateAnswerImageFile(answerImage);
          if (!validation.valid) {
            setError(validation.error ?? '答案图片不符合上传要求');
            setErrorType('invalid_answer');
            setErrorSource('submission');
            return;
          }

          setSubmitPhase('uploading');
          const uploaded = await uploadService.uploadImage(answerImage);
          answerImageUrl = uploaded.url.trim();
          if (!answerImageUrl) {
            throw new Error('图片上传失败，请稍后重试');
          }
        }

        submissionStage = 'grading';
        setSubmitPhase('recognizing');
        const timeSpent = Math.round((Date.now() - startTimeRef.current) / 1000);
        const result = await exerciseService.submitAnswer({
          exerciseId: submittedQuestion.id,
          ...(normalizedAnswer ? { answerText: normalizedAnswer } : {}),
          ...(answerImageUrl ? { answerImageUrl } : {}),
          timeSpentSeconds: timeSpent,
        });
        if (questionVersionRef.current !== submittedQuestionVersion) {
          exerciseLogger.info('Discarded stale answer result', {
            questionId: submittedQuestion.id,
          });
          return;
        }
        setSubmitResult(result);
        exerciseLogger.info('Answer submitted', {
          questionId: submittedQuestion.id,
          isCorrect: result.isCorrect,
          recorded: result.recorded,
          gradingStatus: result.gradingStatus,
        });
      } catch (err) {
        if (questionVersionRef.current !== submittedQuestionVersion) {
          exerciseLogger.info('Discarded stale answer error', {
            questionId: submittedQuestion.id,
          });
          return;
        }
        const submissionError = getSubmissionError(err, submissionStage);
        setError(submissionError.message);
        setErrorType(submissionError.type);
        setErrorSource('submission');
        exerciseLogger.error('Failed to submit answer', {
          questionId: submittedQuestion.id,
          error: err,
        });
      } finally {
        submissionInFlightRef.current = false;
        setSubmitPhase('idle');
      }
    },
    [currentQuestion]
  );

  const loadSolution = useCallback(async () => {
    if (!currentQuestion || solutionInFlightRef.current) return;

    const requestedQuestion = currentQuestion;
    const requestedQuestionVersion = questionVersionRef.current;
    solutionInFlightRef.current = true;
    setIsLoadingSolution(true);
    setSolutionError(null);
    try {
      const nextSolution = await exerciseService.getSolution(requestedQuestion.id);
      if (questionVersionRef.current !== requestedQuestionVersion) return;
      setSolution(nextSolution);
    } catch (err) {
      if (questionVersionRef.current !== requestedQuestionVersion) return;
      setSolutionError(getApiErrorMessage(err, '获取题目解析失败，请稍后重试'));
      exerciseLogger.error('Failed to load solution', {
        questionId: requestedQuestion.id,
        error: err,
      });
    } finally {
      solutionInFlightRef.current = false;
      setIsLoadingSolution(false);
    }
  }, [currentQuestion]);

  return {
    currentQuestion,
    isLoading,
    isGenerating,
    isSubmitting: submitPhase !== 'idle',
    submitPhase,
    submitResult,
    solution,
    isLoadingSolution,
    solutionError,
    error,
    errorType,
    errorSource,
    loadNextQuestion,
    generateQuestion,
    submitAnswer,
    loadSolution,
  };
}
