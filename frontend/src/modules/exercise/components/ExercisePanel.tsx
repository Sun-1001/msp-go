import React, { useCallback, useState } from 'react';
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { MathRenderer } from '@/libs/math/MathRenderer';
import { MathText } from '@/libs/math/MathText';
import { CheckCircle2, Lightbulb, Loader2, XCircle } from 'lucide-react';
import { EmptyExerciseState } from './EmptyExerciseState';
import type { Question, SubmitResult } from '@/modules/exercise/services/exerciseService';
import type { ExerciseErrorType } from '../hooks/exerciseViewModel';

const inlineOrBlockMathRegex = /\$\$?[\s\S]+?\$\$?/;
const latexHintRegex = /\\[a-zA-Z]+|[_^]/;

const questionTypeLabels: Record<Question['type'], string> = {
  multiple_choice: '选择题',
  short_answer: '简答题',
  proof: '证明题',
};

const renderMathContent = (
  value: string,
  options: { className?: string; block?: boolean } = {}
) => {
  if (!value) return null;

  if (inlineOrBlockMathRegex.test(value)) {
    return <MathText className={options.className}>{value}</MathText>;
  }

  if (latexHintRegex.test(value)) {
    return <MathRenderer expression={value} block={options.block} className={options.className} />;
  }

  return <span className={options.className}>{value}</span>;
};

export interface ExercisePanelProps {
  currentQuestion: Question | null;
  isLoading: boolean;
  isSubmitting: boolean;
  submitResult: SubmitResult | null;
  error: string | null;
  errorType: ExerciseErrorType | null;
  onNextQuestion: () => void | Promise<void>;
  submitAnswer: (answerText: string) => Promise<void>;
}

const ExercisePanelContent: React.FC<ExercisePanelProps> = ({
  currentQuestion,
  isLoading,
  isSubmitting,
  submitResult,
  error,
  errorType,
  onNextQuestion,
  submitAnswer,
}) => {
  const [answer, setAnswer] = useState('');

  // 提交答案
  const handleSubmit = useCallback(async () => {
    const normalizedAnswer = answer.trim();
    if (!normalizedAnswer) return;
    await submitAnswer(normalizedAnswer);
  }, [answer, submitAnswer]);

  // 下一题
  const handleNext = useCallback(async () => {
    await onNextQuestion();
  }, [onNextQuestion]);

  // ========== 渲染 ==========

  if (isLoading && !currentQuestion) {
    return (
      <div className="flex justify-center p-10">
        <Loader2 className="h-8 w-8 animate-spin text-primary-500" />
      </div>
    );
  }

  // 使用新的空状态组件处理错误
  if (!currentQuestion && errorType) {
    return (
      <EmptyExerciseState
        errorType={errorType}
        errorMessage={error ?? undefined}
        onRetry={
          errorType === 'network_error' ||
          errorType === 'generation_unavailable' ||
          errorType === 'unknown'
            ? onNextQuestion
            : undefined
        }
      />
    );
  }

  if (!currentQuestion) {
    return <div className="p-4 text-center text-surface-500">暂无可用题目</div>;
  }

  const isBusy = isSubmitting || isLoading;

  return (
    <div className="space-y-6 animate-fade-in">
      {/* 题目卡片 */}
      <Card className="border-primary-100 dark:border-primary-900 shadow-md overflow-hidden">
        <CardHeader className="bg-primary-50/50 dark:bg-primary-950/50 border-b border-primary-100 dark:border-primary-900">
          <CardTitle className="text-lg text-primary-900 dark:text-primary-100 flex justify-between items-center">
            <span>{currentQuestion.title || '练习题'}</span>
            <div className="flex items-center gap-2">
              <span className="text-xs font-normal text-surface-500 dark:text-surface-400">
                难度 {Math.round(currentQuestion.difficulty * 100)}%
              </span>
              <span className="rounded-full bg-primary-100 px-2 py-1 text-xs font-normal text-primary-600 dark:bg-primary-900 dark:text-primary-400">
                {questionTypeLabels[currentQuestion.type]}
              </span>
            </div>
          </CardTitle>
        </CardHeader>
        <CardContent className="p-6">
          {/* 题目内容 */}
          <div className="text-lg text-surface-800 dark:text-surface-200 mb-6 leading-relaxed">
            {renderMathContent(currentQuestion.content, { block: true })}
          </div>

          {/* 答案输入区 */}
          <div className="space-y-4">
            {currentQuestion.type === 'multiple_choice' &&
            (currentQuestion.options?.length ?? 0) > 0 ? (
              <fieldset disabled={isBusy || !!submitResult}>
                <legend className="sr-only">选择答案</legend>
                <div role="radiogroup" aria-label="答案选项" className="grid gap-3">
                  {(currentQuestion.options ?? []).map((option, index) => {
                    const optionId = `exercise-${currentQuestion.id}-option-${index}`;
                    const isSelected = answer === option;
                    return (
                      <label
                        key={optionId}
                        htmlFor={optionId}
                        className={`flex min-h-12 cursor-pointer items-center gap-3 rounded-md border px-4 py-3 transition-colors ${
                          isSelected
                            ? 'border-primary-500 bg-primary-50 text-primary-900 dark:bg-primary-950/40 dark:text-primary-100'
                            : 'border-surface-200 bg-white text-surface-800 hover:border-primary-300 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-200'
                        }`}
                      >
                        <input
                          id={optionId}
                          type="radio"
                          name={`exercise-${currentQuestion.id}-answer`}
                          value={option}
                          checked={isSelected}
                          onChange={(event) => setAnswer(event.target.value)}
                          className="h-4 w-4 shrink-0 accent-primary-600"
                        />
                        <span className="w-5 shrink-0 text-sm font-semibold text-surface-500">
                          {String.fromCharCode(65 + index)}.
                        </span>
                        {renderMathContent(option, { className: 'min-w-0' })}
                      </label>
                    );
                  })}
                </div>
              </fieldset>
            ) : (
              <Input
                value={answer}
                onChange={(event) => setAnswer(event.target.value)}
                placeholder="输入答案（支持 LaTeX 格式，如 \frac{x^3}{3} + C）"
                className="text-lg"
                disabled={isBusy || !!submitResult}
              />
            )}
          </div>
        </CardContent>
        <CardFooter className="bg-surface-50 dark:bg-surface-800 p-4 flex justify-between items-center">
          {!submitResult ? (
            <Button
              onClick={handleSubmit}
              isLoading={isBusy}
              disabled={!answer.trim() || isBusy}
              className="w-full sm:w-auto"
            >
              提交答案
            </Button>
          ) : (
            <Button
              onClick={handleNext}
              variant="secondary"
              isLoading={isLoading}
              disabled={isLoading}
            >
              下一题
            </Button>
          )}
        </CardFooter>
      </Card>

      {/* 反馈卡片 */}
      {submitResult && (
        <Card
          className={`animate-slide-up border-l-4 ${
            submitResult.isCorrect
              ? 'border-l-green-500 bg-green-50 dark:bg-green-950/30'
              : 'border-l-red-500 bg-red-50 dark:bg-red-950/30'
          }`}
        >
          <CardContent className="p-4 space-y-3">
            <h4
              className={`flex items-center gap-2 font-bold text-lg ${
                submitResult.isCorrect
                  ? 'text-green-800 dark:text-green-400'
                  : 'text-red-800 dark:text-red-400'
              }`}
            >
              {submitResult.isCorrect ? (
                <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden="true" />
              ) : (
                <XCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
              )}
              <span>{submitResult.isCorrect ? '回答正确' : '回答错误'}</span>
            </h4>

            {/* 反馈文本 */}
            <div className="text-surface-700 dark:text-surface-300">
              {renderMathContent(submitResult.feedback)}
            </div>

            {/* 正确答案 */}
            {!submitResult.isCorrect && submitResult.correctAnswerLatex && (
              <div className="mt-2 p-3 bg-white/50 dark:bg-surface-900/50 rounded-lg">
                <span className="text-sm font-medium text-surface-500">正确答案：</span>
                <MathRenderer expression={submitResult.correctAnswerLatex} block />
              </div>
            )}

            {/* 诊断详情 */}
            {submitResult.diagnosis && (
              <div className="mt-2 p-3 bg-amber-50 dark:bg-amber-950/30 rounded-lg border border-amber-200 dark:border-amber-800">
                <p className="text-sm font-medium text-amber-800 dark:text-amber-400 mb-1">
                  错误类型：{submitResult.diagnosis.errorType || '未知'}
                  {submitResult.diagnosis.severity && (
                    <span className="ml-2 text-xs opacity-75">
                      ({submitResult.diagnosis.severity})
                    </span>
                  )}
                </p>
                {submitResult.diagnosis.suggestion && (
                  <p className="flex items-start gap-2 text-sm text-amber-700 dark:text-amber-300">
                    <Lightbulb className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                    <span>{submitResult.diagnosis.suggestion}</span>
                  </p>
                )}
              </div>
            )}

            {/* 掌握度变化 */}
            {submitResult.masteryUpdate && Object.keys(submitResult.masteryUpdate).length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
                {Object.entries(submitResult.masteryUpdate).map(([concept, mastery]) => (
                  <span
                    key={concept}
                    className="text-xs px-2 py-1 rounded-full bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300"
                  >
                    {concept}: {Math.round(mastery * 100)}%
                  </span>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* 错误提示 */}
      {error ? (
        <div role="alert" className="text-center text-sm text-red-500">
          {error}
        </div>
      ) : null}
    </div>
  );
};

export const ExercisePanel: React.FC<ExercisePanelProps> = (props) => (
  <ExercisePanelContent key={props.currentQuestion?.id ?? 'no-question'} {...props} />
);
