import React from 'react';
import { Card, CardContent } from '../../../../components/ui/Card';
import type { QuestionStats } from '@/modules/question';

interface QuestionStatsCardsProps {
  stats: QuestionStats;
}

export const QuestionStatsCards = React.memo<QuestionStatsCardsProps>(({ stats }) => (
  <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
    <Card>
      <CardContent className="p-4">
        <div className="text-sm text-surface-500 dark:text-surface-400 mb-1">总题目数</div>
        <div className="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {stats.total}
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent className="p-4">
        <div className="text-sm text-surface-500 dark:text-surface-400 mb-1">已发布</div>
        <div className="text-2xl font-bold text-green-600 dark:text-green-400">
          {stats.byStatus.published || 0}
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent className="p-4">
        <div className="text-sm text-surface-500 dark:text-surface-400 mb-1">草稿</div>
        <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
          {stats.byStatus.draft || 0}
        </div>
      </CardContent>
    </Card>
    <Card>
      <CardContent className="p-4">
        <div className="text-sm text-surface-500 dark:text-surface-400 mb-1">已归档</div>
        <div className="text-2xl font-bold text-surface-600 dark:text-surface-400">
          {stats.byStatus.archived || 0}
        </div>
      </CardContent>
    </Card>
  </div>
));

QuestionStatsCards.displayName = 'QuestionStatsCards';
