import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { MathRenderer } from '@/libs/math/MathRenderer';
import { MathText } from '@/libs/math/MathText';
import { Camera, Loader2, X } from 'lucide-react';
import { EmptyExerciseState } from './EmptyExerciseState';
import type { Question, SubmitResult } from '@/modules/exercise/services/exerciseService';
import type { ExerciseErrorType } from '../hooks/exerciseViewModel';

const inlineOrBlockMathRegex = /\$\$?[\s\S]+?\$\$?/;
const latexHintRegex = /\\[a-zA-Z]+|[_^]/;

const renderMathContent = (
  value: string,
  options: { className?: string; block?: boolean } = {},
) => {
  if (!value) return null;

  if (inlineOrBlockMathRegex.test(value)) {
    return (
      <MathText className={options.className}>
        {value}
      </MathText>
    );
  }

  if (latexHintRegex.test(value)) {
    return (
      <MathRenderer
        expression={value}
        block={options.block}
        className={options.className}
      />
    );
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
  loadNextQuestion: (conceptId?: string, difficulty?: number) => Promise<void>;
  submitAnswer: (answerText?: string, answerImageUrl?: string) => Promise<void>;
  uploadAnswerImage: (file: File) => Promise<string | null>;
}

export const ExercisePanel: React.FC<ExercisePanelProps> = ({
  currentQuestion,
  isLoading,
  isSubmitting,
  submitResult,
  error,
  errorType,
  loadNextQuestion,
  submitAnswer,
  uploadAnswerImage,
}) => {

  const [answer, setAnswer] = useState('');
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imagePreviewRef = useRef<string | null>(null);

  useEffect(() => {
    loadNextQuestion();
  }, [loadNextQuestion]);

  useEffect(() => {
    return () => {
      if (imagePreviewRef.current) {
        URL.revokeObjectURL(imagePreviewRef.current);
      }
    };
  }, []);

  const replaceImagePreview = useCallback((url: string | null) => {
    if (imagePreviewRef.current) {
      URL.revokeObjectURL(imagePreviewRef.current);
    }
    imagePreviewRef.current = url;
    setImagePreview(url);
  }, []);

  // 图片选择
  const handleImageSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setImageFile(file);
    replaceImagePreview(URL.createObjectURL(file));
  }, [replaceImagePreview]);

  // 移除图片
  const handleRemoveImage = useCallback(() => {
    replaceImagePreview(null);
    setImageFile(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, [replaceImagePreview]);
  // 提交答案
  const handleSubmit = useCallback(async () => {
    let imageUrl: string | undefined;

    // 如果有图片，先上传
    if (imageFile) {
      setIsUploading(true);
      const url = await uploadAnswerImage(imageFile);
      setIsUploading(false);
      if (!url) return; // 上传失败
      imageUrl = url;
    }

    await submitAnswer(answer || undefined, imageUrl);
  }, [answer, imageFile, submitAnswer, uploadAnswerImage]);

  // 下一题
  const handleNext = useCallback(() => {
    setAnswer('');
    handleRemoveImage();
    loadNextQuestion();
  }, [handleRemoveImage, loadNextQuestion]);

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
        onRetry={errorType === 'network_error' || errorType === 'unknown' ? loadNextQuestion : undefined}
      />
    );
  }

  if (!currentQuestion) {
    return <div className="p-4 text-center text-surface-500">暂无可用题目</div>;
  }

  const isBusy = isSubmitting || isUploading;

  return (
    <div className="space-y-6 animate-fade-in">
      {/* 提示信息 */}
      <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
        <p className="text-sm text-blue-700 dark:text-blue-300">
          💡 提示：刷新页面不会更换题目，请认真完成当前练习
        </p>
      </div>

      {/* 题目卡片 */}
      <Card className="border-primary-100 dark:border-primary-900 shadow-md overflow-hidden">
        <CardHeader className="bg-primary-50/50 dark:bg-primary-950/50 border-b border-primary-100 dark:border-primary-900">
          <CardTitle className="text-lg text-primary-900 dark:text-primary-100 flex justify-between items-center">
            <span>{currentQuestion.title || '练习题'}</span>
            <div className="flex items-center gap-2">
              <span className="text-xs font-normal text-surface-500 dark:text-surface-400">
                难度 {Math.round(currentQuestion.difficulty * 100)}%
              </span>
              <span className="text-xs font-normal text-primary-600 dark:text-primary-400 bg-primary-100 dark:bg-primary-900 px-2 py-1 rounded-full uppercase tracking-wider">
                {currentQuestion.type}
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
            <Input
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder="输入答案（支持 LaTeX 格式，如 \frac{x^3}{3} + C）"
              className="text-lg"
              disabled={isBusy || !!submitResult}
            />

            {/* 图片上传区 */}
            <div className="flex items-center gap-3">
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                onChange={handleImageSelect}
                className="hidden"
                disabled={isBusy || !!submitResult}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={isBusy || !!submitResult}
              >
                <Camera className="h-4 w-4 mr-1" />
                拍照/上传手写答案
              </Button>
              {isUploading && (
                <span className="text-sm text-surface-500 flex items-center gap-1">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  上传中...
                </span>
              )}
            </div>

            {/* 图片预览 */}
            {imagePreview && (
              <div className="relative inline-block">
                <img
                  src={imagePreview}
                  alt="手写答案预览"
                  className="max-h-48 rounded-lg border border-surface-200 dark:border-surface-700"
                />
                <button
                  onClick={handleRemoveImage}
                  className="absolute -top-2 -right-2 bg-red-500 text-white rounded-full p-1 hover:bg-red-600 transition-colors"
                  disabled={isBusy}
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            )}
          </div>
        </CardContent>
        <CardFooter className="bg-surface-50 dark:bg-surface-800 p-4 flex justify-between items-center">
          {!submitResult ? (
            <Button
              onClick={handleSubmit}
              isLoading={isBusy}
              disabled={(!answer && !imageFile) || isBusy}
              className="w-full sm:w-auto"
            >
              提交答案
            </Button>
          ) : (
            <Button onClick={handleNext} variant="secondary">
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
              className={`font-bold text-lg ${
                submitResult.isCorrect
                  ? 'text-green-800 dark:text-green-400'
                  : 'text-red-800 dark:text-red-400'
              }`}
            >
              {submitResult.isCorrect ? '✓ 回答正确！' : '✗ 回答错误'}
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
                  <p className="text-sm text-amber-700 dark:text-amber-300">
                    💡 {submitResult.diagnosis.suggestion}
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
      {error && !submitResult && (
        <div className="text-red-500 text-sm text-center">{error}</div>
      )}
    </div>
  );
};
