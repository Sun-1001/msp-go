import React from 'react';
import { MainLayout } from '../../components/layout/MainLayout';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { Progress } from '../../components/ui/Progress';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../components/ui/Tabs';
import {
  AlertCircle,
  CheckCircle,
  ArrowRight,
  Trash2,
  Sparkles,
  Clock,
  Loader2,
  User,
  RefreshCw,
} from 'lucide-react';
import { MarkdownContent } from '../../components/chat/MarkdownContent';
import {
  useMistakeBook,
  getDifficultyBadge,
  getErrorTypeLabel,
} from '@/modules/mistake/hooks/useMistakeBook';

export const MistakeBookPage: React.FC = () => {
  const {
    mistakes,
    pagination,
    mistakesLoading,
    mistakesError,
    portrait,
    portraitLoading,
    generating,
    clearing,
    handleTabChange,
    handleDeleteMistake,
    handleMarkAsMastered,
    handleFetchMistakes,
    handleGeneratePortrait,
    handleClearPortrait,
  } = useMistakeBook();

  return (
    <MainLayout>
      <div className="container mx-auto max-w-5xl px-6 py-8">
        <h1 className="mb-8 text-3xl font-bold text-surface-900 dark:text-surface-100">错题本</h1>

        <Tabs defaultValue="mistakes" onValueChange={handleTabChange}>
          <TabsList className="mb-6">
            <TabsTrigger value="mistakes">错题记录</TabsTrigger>
            <TabsTrigger value="portrait">学生画像</TabsTrigger>
          </TabsList>

          <TabsContent value="mistakes">
            <div className="space-y-4">
              {mistakesLoading === 'loading' && (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-primary-500" />
                  <span className="ml-3 text-surface-500">加载错题中...</span>
                </div>
              )}

              {mistakesLoading === 'error' && mistakesError && (
                <Card className="border-red-200 dark:border-red-800">
                  <CardContent className="p-6 text-center">
                    <AlertCircle className="mx-auto mb-3 h-12 w-12 text-red-500" />
                    <p className="text-red-600 dark:text-red-400">{mistakesError}</p>
                    <Button
                      onClick={() => handleFetchMistakes(1)}
                      variant="outline"
                      className="mt-4"
                    >
                      重试
                    </Button>
                  </CardContent>
                </Card>
              )}

              {mistakesLoading === 'success' && mistakes.length === 0 && (
                <Card>
                  <CardContent className="p-12 text-center">
                    <CheckCircle className="mx-auto mb-4 h-16 w-16 text-green-500" />
                    <h2 className="mb-2 text-lg font-semibold text-surface-900 dark:text-surface-100">
                      暂无错题
                    </h2>
                    <p className="text-surface-500 dark:text-surface-400">继续保持，多做练习巩固知识点</p>
                  </CardContent>
                </Card>
              )}

              {mistakesLoading === 'success' && mistakes.map((item) => {
                const difficultyBadge = getDifficultyBadge(item.exercise.difficulty);
                const masteryPercent = Math.round(item.mastery.current * 100);

                return (
                  <Card key={item.id} className="border-surface-200 transition-shadow hover:shadow-md dark:border-surface-700">
                    <CardContent className="p-5">
                      <div className="flex flex-col gap-4 sm:flex-row sm:justify-between">
                        <div className="min-w-0 flex-1 space-y-2">
                          <div className="flex flex-wrap items-center gap-3">
                            <Badge variant="outline" className="text-xs">
                              {item.exercise.knowledgePoints?.[0] || '未分类'}
                            </Badge>
                            <Badge variant={difficultyBadge.variant} className="text-xs">
                              {difficultyBadge.label}
                            </Badge>
                            {item.diagnosis.errorType && (
                              <Badge variant="secondary" className="text-xs">
                                {getErrorTypeLabel(item.diagnosis.errorType)}
                              </Badge>
                            )}
                          </div>
                          <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                            {item.exercise.title}
                          </h2>
                          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-surface-500 dark:text-surface-400">
                            <div className="flex min-w-0 items-center">
                              <AlertCircle className="mr-1 h-3.5 w-3.5 shrink-0 text-orange-500" />
                              <span className="truncate">{item.diagnosis.explanation || '暂无诊断'}</span>
                            </div>
                            {item.attempt.submittedAt && (
                              <div className="flex items-center">
                                <Clock className="mr-1 h-3.5 w-3.5 text-primary-500" />
                                <span>{new Date(item.attempt.submittedAt).toLocaleDateString()}</span>
                              </div>
                            )}
                            <div className="flex items-center">
                              <RefreshCw className="mr-1 h-3.5 w-3.5 text-blue-500" />
                              <span>错误 {item.errorCount} 次</span>
                            </div>
                          </div>
                        </div>

                        <div className="flex shrink-0 flex-col items-start gap-3 sm:items-end">
                          <div className="text-right">
                            <div className="mb-1 text-xs text-surface-500 dark:text-surface-400">掌握度</div>
                            <Progress
                              value={masteryPercent}
                              variant={masteryPercent < 60 ? 'destructive' : masteryPercent < 80 ? 'warning' : 'success'}
                              size="sm"
                              className="w-20"
                            />
                            <div className="mt-1 text-xs text-surface-600 dark:text-surface-300">
                              {masteryPercent}%
                            </div>
                          </div>
                          <div className="flex flex-wrap gap-2 sm:justify-end">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="text-surface-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/30 dark:hover:text-red-400"
                              onClick={() => handleDeleteMistake(item.id)}
                              title="删除错题"
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleMarkAsMastered(item.id)}
                              title="标记已掌握"
                            >
                              <CheckCircle className="mr-1 h-3 w-3" />
                              已掌握
                            </Button>
                            <Button size="sm">
                              重做 <ArrowRight className="ml-1 h-3 w-3" />
                            </Button>
                          </div>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                );
              })}

              {mistakesLoading === 'success' && pagination.totalPages > 1 && (
                <div className="mt-6 flex items-center justify-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pagination.page === 1}
                    onClick={() => handleFetchMistakes(pagination.page - 1)}
                  >
                    上一页
                  </Button>
                  <span className="text-sm text-surface-600 dark:text-surface-400">
                    第 {pagination.page} / {pagination.totalPages} 页
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pagination.page === pagination.totalPages}
                    onClick={() => handleFetchMistakes(pagination.page + 1)}
                  >
                    下一页
                  </Button>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value="portrait">
            <div className="space-y-6">
              {generating ? (
                <Card>
                  <CardContent className="p-12 text-center">
                    <Loader2 className="mx-auto mb-4 h-12 w-12 animate-spin text-primary-500" />
                    <p className="text-lg font-medium text-surface-700 dark:text-surface-300">AI 正在生成学生画像...</p>
                  </CardContent>
                </Card>
              ) : portraitLoading === 'loading' ? (
                <Card>
                  <CardContent className="p-12 text-center">
                    <Loader2 className="mx-auto mb-3 h-8 w-8 animate-spin text-surface-400" />
                    <p className="text-surface-500 dark:text-surface-400">加载中...</p>
                  </CardContent>
                </Card>
              ) : portrait?.has_content ? (
                <Card>
                  <CardHeader>
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <CardTitle className="flex items-center gap-2">
                        <User className="h-5 w-5 text-primary-500" />
                        学生画像
                      </CardTitle>
                      <div className="flex flex-wrap items-center gap-2">
                        {portrait.portrait_generated_at && (
                          <span className="flex items-center gap-1 text-xs text-surface-400 dark:text-surface-500">
                            <Clock className="h-3.5 w-3.5" />
                            {new Date(portrait.portrait_generated_at).toLocaleString('zh-CN')}
                          </span>
                        )}
                        <Button size="sm" variant="outline" onClick={handleGeneratePortrait} disabled={generating}>
                          <RefreshCw className="mr-1 h-4 w-4" />
                          重新生成
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/30"
                          onClick={handleClearPortrait}
                          disabled={clearing}
                        >
                          <Trash2 className="mr-1 h-4 w-4" />
                          清除
                        </Button>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <div className="prose prose-sm max-w-none dark:prose-invert">
                      <MarkdownContent content={portrait.portrait_content!} unwrapOuterFence />
                    </div>
                  </CardContent>
                </Card>
              ) : (
                <Card>
                  <CardContent className="p-12 text-center">
                    <User className="mx-auto mb-4 h-16 w-16 text-surface-300 dark:text-surface-600" />
                    <h2 className="mb-6 text-lg font-semibold text-surface-700 dark:text-surface-300">
                      尚未生成学生画像
                    </h2>
                    <Button onClick={handleGeneratePortrait} disabled={generating}>
                      <Sparkles className="mr-2 h-4 w-4" />
                      生成画像
                    </Button>
                  </CardContent>
                </Card>
              )}
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </MainLayout>
  );
};
