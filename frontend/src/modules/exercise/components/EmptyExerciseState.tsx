import React from 'react';
import { useNavigate } from 'react-router-dom';
import { BookOpen, Home, Users, AlertCircle, Wifi } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card, CardContent } from '@/components/ui/Card';
import type { ExerciseErrorType } from '../hooks/exerciseViewModel';

interface EmptyExerciseStateProps {
  errorType: ExerciseErrorType;
  errorMessage?: string;
  onRetry?: () => void;
}

const actionGroupClass =
  'flex w-full max-w-xs flex-col justify-center gap-3 sm:w-auto sm:max-w-none sm:flex-row';
const actionButtonClass = 'w-full whitespace-nowrap sm:w-auto';

/**
 * 练习题空状态组件
 *
 * 根据不同的错误类型展示友好的提示和操作引导
 */
export const EmptyExerciseState: React.FC<EmptyExerciseStateProps> = ({
  errorType,
  errorMessage,
  onRetry,
}) => {
  const navigate = useNavigate();

  // 根据错误类型渲染不同的内容
  const renderContent = () => {
    switch (errorType) {
      case 'not_enrolled':
        return (
          <>
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 rounded-full bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
                <Users className="w-8 h-8 text-amber-600 dark:text-amber-400" />
              </div>
            </div>
            <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
              请先加入班级
            </h3>
            <p className="text-surface-600 dark:text-surface-400 mb-6 text-center max-w-md">
              您还没有加入任何班级。加入班级后，您就可以开始练习教师发布的题目啦～
            </p>
            <div className={actionGroupClass}>
              <Button
                variant="outline"
                onClick={() => navigate('/course/overview')}
                className={actionButtonClass}
              >
                <Home className="w-4 h-4 mr-2" />
                返回首页
              </Button>
              <Button
                onClick={() => navigate('/my-class')}
                className={actionButtonClass}
              >
                <Users className="w-4 h-4 mr-2" />
                加入班级
              </Button>
            </div>
          </>
        );

      case 'no_questions':
        return (
          <>
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
                <BookOpen className="w-8 h-8 text-blue-600 dark:text-blue-400" />
              </div>
            </div>
            <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
              暂时还没有可用的练习题
            </h3>
            <p className="text-surface-600 dark:text-surface-400 mb-2 text-center max-w-md">
              您的教师可能还在准备题目中...
            </p>
            <p className="text-surface-500 dark:text-surface-500 text-sm mb-6 text-center max-w-md">
              或者您已经完成了所有可用题目 ✨
            </p>
            <div className={actionGroupClass}>
              <Button
                variant="outline"
                onClick={() => navigate('/course/overview')}
                className={actionButtonClass}
              >
                <Home className="w-4 h-4 mr-2" />
                返回首页
              </Button>
              <Button
                variant="secondary"
                onClick={() => navigate('/resources')}
                className={actionButtonClass}
              >
                <BookOpen className="w-4 h-4 mr-2" />
                查看学习资源
              </Button>
            </div>
          </>
        );

      case 'network_error':
        return (
          <>
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
                <Wifi className="w-8 h-8 text-red-600 dark:text-red-400" />
              </div>
            </div>
            <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
              无法连接到服务器
            </h3>
            <p className="text-surface-600 dark:text-surface-400 mb-6 text-center max-w-md">
              请检查您的网络连接，然后重试
            </p>
            <div className={actionGroupClass}>
              <Button
                variant="outline"
                onClick={() => navigate('/course/overview')}
                className={actionButtonClass}
              >
                <Home className="w-4 h-4 mr-2" />
                返回首页
              </Button>
              {onRetry && (
                <Button onClick={onRetry} className={actionButtonClass}>
                  重试
                </Button>
              )}
            </div>
          </>
        );

      case 'unknown':
      default:
        return (
          <>
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 rounded-full bg-surface-100 dark:bg-surface-800 flex items-center justify-center">
                <AlertCircle className="w-8 h-8 text-surface-600 dark:text-surface-400" />
              </div>
            </div>
            <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
              加载题目失败
            </h3>
            <p className="text-surface-600 dark:text-surface-400 mb-2 text-center max-w-md">
              {errorMessage || '发生了一些问题，请稍后重试'}
            </p>
            <div className={`${actionGroupClass} mt-6`}>
              <Button
                variant="outline"
                onClick={() => navigate('/course/overview')}
                className={actionButtonClass}
              >
                <Home className="w-4 h-4 mr-2" />
                返回首页
              </Button>
              {onRetry && (
                <Button onClick={onRetry} className={actionButtonClass}>
                  重试
                </Button>
              )}
            </div>
          </>
        );
    }
  };

  return (
    <Card className="border-surface-200 dark:border-surface-700">
      <CardContent className="p-6 sm:p-12">
        <div className="flex flex-col items-center">
          {renderContent()}
        </div>
      </CardContent>
    </Card>
  );
};
