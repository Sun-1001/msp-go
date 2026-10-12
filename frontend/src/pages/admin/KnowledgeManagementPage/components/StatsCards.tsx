import React from 'react';
import { Card, CardContent } from '@/components/ui/Card';
import { Network, GitBranch, BookOpen, AlertCircle } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { NODE_TYPE_LABELS } from '@/modules/admin/api';
import type { KnowledgeStats } from '@/modules/admin/api';

interface StatsCardsProps {
  stats: KnowledgeStats | null;
  loading: boolean;
}

export const StatsCards = React.memo<StatsCardsProps>(({ stats, loading }) => {
  return (
    <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
      {/* 知识节点 */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-primary-100 dark:bg-primary-900/30 rounded-lg flex items-center justify-center">
              <Network className="h-5 w-5 text-primary-600 dark:text-primary-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">知识节点</p>
              <p className="text-xl font-bold text-surface-900 dark:text-surface-100">
                {loading ? '...' : stats?.total_nodes ?? 0}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 知识关系 */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-accent-100 dark:bg-accent-900/30 rounded-lg flex items-center justify-center">
              <GitBranch className="h-5 w-5 text-accent-600 dark:text-accent-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">知识关系</p>
              <p className="text-xl font-bold text-surface-900 dark:text-surface-100">
                {loading ? '...' : stats?.total_relations ?? 0}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 章节数 */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-success-100 dark:bg-success-900/30 rounded-lg flex items-center justify-center">
              <BookOpen className="h-5 w-5 text-success-600 dark:text-success-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">章节数</p>
              <p className="text-xl font-bold text-surface-900 dark:text-surface-100">
                {loading ? '...' : stats?.chapters_count ?? 0}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 类型分布 */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-warning-100 dark:bg-warning-900/30 rounded-lg flex items-center justify-center">
              <AlertCircle className="h-5 w-5 text-warning-600 dark:text-warning-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">类型分布</p>
              <div className="flex gap-1 flex-wrap mt-1">
                {stats?.type_distribution &&
                  Object.entries(stats.type_distribution).map(([type, count]) => (
                    <Badge key={type} variant="secondary" className="text-xs">
                      {NODE_TYPE_LABELS[type] || type}: {count}
                    </Badge>
                  ))}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
});

StatsCards.displayName = 'StatsCards';
