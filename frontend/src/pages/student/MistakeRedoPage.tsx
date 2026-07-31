import React, { useCallback, useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import { useNavigate, useParams } from 'react-router-dom';
import {
  AlertCircle,
  ArrowLeft,
  ChevronDown,
  History,
  Loader2,
  RefreshCw,
} from 'lucide-react';
import { MainLayout } from '@/components/layout/MainLayout';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import { Progress } from '@/components/ui/Progress';
import { ExercisePanel, useExerciseViewModel } from '@/modules/exercise';
import type { Question } from '@/modules/exercise/services/exerciseService';
import {
  fetchReviewExerciseByAttempt,
  type ReviewExerciseResponse,
} from '@/modules/mistake/services/mistakeService';
import {
  getDifficultyBadge,
  getErrorTypeLabel,
} from '@/modules/mistake/hooks/useMistakeBook';
import { getApiErrorMessage } from '@/libs/http/apiClient';

const toQuestion = (review: ReviewExerciseResponse): Question => ({
  id: review.exercise.id,
  title: review.exercise.title,
  content: review.exercise.content,
  difficulty: review.exercise.difficulty,
  type: review.exercise.type,
  source: 'review',
  knowledgePoints: review.exercise.knowledgePoints,
  knowledgePointNames: review.exercise.knowledgePointNames,
  hintsAvailable: review.exercise.hintsAvailable,
  estimatedTimeSeconds: review.exercise.estimatedTimeSeconds,
  options: review.exercise.options,
});

const validateReview = (review: ReviewExerciseResponse): string | null => {
  if (!review.exercise.id.trim() || !review.exercise.content.trim()) {
    return '这道错题缺少必要的题目内容，暂时无法重做';
  }
  if (
    review.exercise.type === 'multiple_choice'
    && (review.exercise.options?.length ?? 0) === 0
  ) {
    return '这道选择题缺少选项，暂时无法重做';
  }
  return null;
};

export const MistakeRedoPage: React.FC = () => {
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();
  const [review, setReview] = useState<ReviewExerciseResponse | null>(null);
  const [isLoadingReview, setIsLoadingReview] = useState(Boolean(attemptId));
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reloadVersion, setReloadVersion] = useState(0);
  const [resetKey, setResetKey] = useState(0);
  const {
    currentQuestion,
    isSubmitting,
    submitPhase,
    submitResult,
    solution,
    isLoadingSolution,
    solutionError,
    error,
    errorType,
    loadQuestion,
    submitAnswer,
    loadSolution,
  } = useExerciseViewModel({ dailyAssignmentId: review?.context.dailyAssignmentId });

  useEffect(() => {
    if (!attemptId) {
      setIsLoadingReview(false);
      setLoadError('缺少错题记录 ID');
      return;
    }

    const controller = new AbortController();
    const loadReview = async () => {
      setIsLoadingReview(true);
      setLoadError(null);
      setReview(null);
      try {
        const nextReview = await fetchReviewExerciseByAttempt(attemptId, controller.signal);
        if (controller.signal.aborted) return;
        const validationError = validateReview(nextReview);
        if (validationError) {
          setLoadError(validationError);
          return;
        }
        setReview(nextReview);
        loadQuestion(toQuestion(nextReview));
        setResetKey((current) => current + 1);
      } catch (loadFailure) {
        if (controller.signal.aborted) return;
        if (axios.isAxiosError(loadFailure) && loadFailure.response?.status === 404) {
          setLoadError('错题记录不存在、已下架或当前不可重做');
        } else {
          setLoadError(getApiErrorMessage(loadFailure, '加载错题失败，请稍后重试'));
        }
      } finally {
        if (!controller.signal.aborted) {
          setIsLoadingReview(false);
        }
      }
    };

    void loadReview();
    return () => controller.abort();
  }, [attemptId, loadQuestion, reloadVersion]);

  const retryQuestion = useCallback(() => {
    if (!review) return;
    loadQuestion(toQuestion(review));
    setResetKey((current) => current + 1);
  }, [loadQuestion, review]);

  const masteryPercent = Math.round(Math.min(Math.max(review?.context.masteryBefore ?? 0, 0), 1) * 100);
  const difficulty = useMemo(
    () => getDifficultyBadge(review?.exercise.difficulty ?? 0),
    [review?.exercise.difficulty]
  );

  if (isLoadingReview) {
    return (
      <MainLayout>
        <div className="flex min-h-[60vh] items-center justify-center gap-3 text-surface-500">
          <Loader2 className="h-8 w-8 animate-spin text-primary-500" />
          <span>正在加载错题...</span>
        </div>
      </MainLayout>
    );
  }

  if (loadError || !review || !currentQuestion) {
    return (
      <MainLayout>
        <div className="container mx-auto flex min-h-[60vh] max-w-3xl flex-col items-center justify-center px-6 text-center">
          <AlertCircle className="mb-4 h-12 w-12 text-red-400" />
          <h1 className="text-xl font-semibold text-surface-900 dark:text-surface-100">无法打开这道错题</h1>
          <p className="mt-2 text-surface-500 dark:text-surface-400">
            {loadError || '错题内容暂不可用'}
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            {attemptId ? (
              <Button variant="outline" onClick={() => setReloadVersion((current) => current + 1)}>
                <RefreshCw className="mr-2 h-4 w-4" />
                重新加载
              </Button>
            ) : null}
            <Button onClick={() => navigate('/mistake-book')}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              返回错题本
            </Button>
          </div>
        </div>
      </MainLayout>
    );
  }

  const previousErrorLabel = getErrorTypeLabel(review.context.previousErrorType);
  const knowledgeLabel = review.exercise.knowledgePointNames[0]
    || review.exercise.knowledgePoints[0]
    || '未分类';

  return (
    <MainLayout>
      <div className="container mx-auto max-w-5xl px-4 py-6 sm:px-6 sm:py-8">
        <Button variant="ghost" className="mb-4" onClick={() => navigate('/mistake-book')}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          返回错题本
        </Button>

        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100">错题重做</h1>
            <div className="mt-3 flex flex-wrap gap-2">
              <Badge variant="outline">{knowledgeLabel}</Badge>
              <Badge variant={difficulty.variant}>{difficulty.label}</Badge>
              <Badge variant="secondary">错误 {review.context.errorCount} 次</Badge>
              {review.context.previousErrorType ? (
                <Badge variant="warning">{previousErrorLabel}</Badge>
              ) : null}
            </div>
          </div>
          <div className="w-full rounded-lg border border-surface-200 bg-white p-3 dark:border-surface-700 dark:bg-surface-900 sm:w-48">
            <div className="mb-2 flex items-center justify-between text-xs text-surface-500">
              <span>当前掌握度</span>
              <span>{masteryPercent}%</span>
            </div>
            <Progress
              value={masteryPercent}
              size="sm"
              variant={masteryPercent < 60 ? 'destructive' : masteryPercent < 80 ? 'warning' : 'success'}
            />
          </div>
        </div>

        <Card className="mb-6 border-surface-200 dark:border-surface-700">
          <CardContent className="p-0">
            <details className="group">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 text-sm font-medium text-surface-700 dark:text-surface-300">
                <span className="flex items-center gap-2">
                  <History className="h-4 w-4 text-primary-500" />
                  查看上次错误提示
                </span>
                <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" />
              </summary>
              <div className="space-y-3 border-t border-surface-200 px-4 py-4 text-sm dark:border-surface-700">
                <div>
                  <p className="mb-1 text-xs font-medium text-surface-500">上次答案</p>
                  <div className="rounded-md bg-surface-50 px-3 py-2 text-surface-800 [overflow-wrap:anywhere] dark:bg-surface-800 dark:text-surface-200">
                    {review.context.previousAnswer || '未记录'}
                  </div>
                </div>
                {review.context.previousExplanation ? (
                  <div>
                    <p className="mb-1 text-xs font-medium text-surface-500">错误诊断</p>
                    <p className="text-surface-700 dark:text-surface-300">{review.context.previousExplanation}</p>
                  </div>
                ) : null}
                {review.context.previousSuggestion ? (
                  <div>
                    <p className="mb-1 text-xs font-medium text-surface-500">改进建议</p>
                    <p className="text-surface-700 dark:text-surface-300">{review.context.previousSuggestion}</p>
                  </div>
                ) : null}
              </div>
            </details>
          </CardContent>
        </Card>

        <ExercisePanel
          currentQuestion={currentQuestion}
          isLoading={false}
          isSubmitting={isSubmitting}
          submitPhase={submitPhase}
          submitResult={submitResult}
          solution={solution}
          isLoadingSolution={isLoadingSolution}
          solutionError={solutionError}
          error={error}
          errorType={errorType}
          onNextQuestion={retryQuestion}
          submitAnswer={submitAnswer}
          onLoadSolution={loadSolution}
          nextButtonLabel="再做一次"
          resetKey={resetKey}
        />
      </div>
    </MainLayout>
  );
};

export default MistakeRedoPage;
