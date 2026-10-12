import React from 'react';
import { X, Edit, Trash2, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { RELATION_TYPE_LABELS } from '@/modules/admin/api';
import type { KnowledgeRelationAdmin } from '@/modules/admin/api';

interface EdgeDetailPanelProps {
  relation: KnowledgeRelationAdmin;
  saving: boolean;
  onEdit: (relation: KnowledgeRelationAdmin) => void;
  onDelete: (id: string, name: string) => void;
  onClose: () => void;
}

/**
 * 边详情浮动面板
 *
 * 点击图谱中的边时弹出，显示关系详情并提供编辑/删除操作
 */
export const EdgeDetailPanel = React.memo<EdgeDetailPanelProps>(
  ({ relation, saving, onEdit, onDelete, onClose }) => {
    const relationLabel = RELATION_TYPE_LABELS[relation.relation_type] || relation.relation_type;

    return (
      <div className="absolute top-3 left-3 z-20 bg-white dark:bg-surface-800 rounded-xl shadow-2xl border border-surface-200 dark:border-surface-700 p-4 w-72 animate-fade-in">
        {/* 标题 */}
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-sm font-semibold text-surface-900 dark:text-surface-100">
            关系详情
          </h4>
          <button
            type="button"
            onClick={onClose}
            className="p-1 rounded-md text-surface-400 hover:text-surface-600 dark:hover:text-surface-300"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* 关系信息 */}
        <div className="space-y-2.5 mb-4">
          <div className="flex items-center gap-2">
            <span className="text-xs text-surface-500 w-14 shrink-0">源节点</span>
            <span className="text-sm font-medium text-surface-900 dark:text-surface-100 truncate">
              {relation.source_name || relation.source_id}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-surface-500 w-14 shrink-0">关系</span>
            <Badge variant="secondary">{relationLabel}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-surface-500 w-14 shrink-0">目标</span>
            <span className="text-sm font-medium text-surface-900 dark:text-surface-100 truncate">
              {relation.target_name || relation.target_id}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-surface-500 w-14 shrink-0">权重</span>
            <span className="text-sm text-surface-700 dark:text-surface-300">
              {relation.weight?.toFixed(2)}
            </span>
          </div>
          {relation.description && (
            <div className="flex items-start gap-2">
              <span className="text-xs text-surface-500 w-14 shrink-0 mt-0.5">描述</span>
              <span className="text-sm text-surface-600 dark:text-surface-400">
                {relation.description}
              </span>
            </div>
          )}
        </div>

        {/* 操作按钮 */}
        <div className="flex justify-end gap-2 pt-2 border-t border-surface-100 dark:border-surface-700">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onEdit(relation)}
            disabled={saving}
          >
            <Edit className="h-3 w-3 mr-1" />
            编辑
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="text-red-500 hover:text-red-700 border-red-200 hover:border-red-300"
            onClick={() => onDelete(relation.id, `${relation.source_name} → ${relation.target_name}`)}
            disabled={saving}
          >
            {saving ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : <Trash2 className="h-3 w-3 mr-1" />}
            删除
          </Button>
        </div>
      </div>
    );
  }
);

EdgeDetailPanel.displayName = 'EdgeDetailPanel';
