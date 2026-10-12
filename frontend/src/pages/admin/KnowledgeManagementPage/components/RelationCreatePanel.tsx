import React, { useState } from 'react';
import { X, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { RELATION_TYPE_OPTIONS, INPUT_CLASS } from '../constants';
import type { AdminRelationType } from '@/modules/admin/api';

interface RelationCreatePanelProps {
  sourceName: string;
  targetName: string;
  saving: boolean;
  onConfirm: (data: {
    relation_type: AdminRelationType;
    weight: number;
    description?: string;
  }) => void;
  onCancel: () => void;
}

/**
 * 拖拽连线后弹出的关系创建面板
 *
 * 浮动在图谱上方，让管理员选择关系类型并确认创建
 */
export const RelationCreatePanel = React.memo<RelationCreatePanelProps>(
  ({ sourceName, targetName, saving, onConfirm, onCancel }) => {
    const [relationType, setRelationType] = useState<AdminRelationType>('has_prerequisite');
    const [weight, setWeight] = useState(1.0);
    const [description, setDescription] = useState('');

    const handleConfirm = () => {
      onConfirm({
        relation_type: relationType,
        weight,
        description: description || undefined,
      });
    };

    return (
      <div className="absolute inset-0 z-20 flex items-center justify-center bg-black/20 backdrop-blur-[2px] rounded-xl">
        <div className="bg-white dark:bg-surface-800 rounded-xl shadow-2xl border border-surface-200 dark:border-surface-700 p-5 w-80 animate-scale-in">
          {/* 标题 */}
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-sm font-semibold text-surface-900 dark:text-surface-100">
              创建知识关系
            </h4>
            <button
              type="button"
              onClick={onCancel}
              className="p-1 rounded-md text-surface-400 hover:text-surface-600 dark:hover:text-surface-300"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* 连接信息 */}
          <div className="flex items-center gap-2 mb-4 p-2.5 bg-surface-50 dark:bg-surface-900 rounded-lg">
            <span className="text-xs font-medium text-primary-600 dark:text-primary-400 truncate max-w-[100px]">
              {sourceName}
            </span>
            <span className="text-surface-400 text-xs">→</span>
            <span className="text-xs font-medium text-primary-600 dark:text-primary-400 truncate max-w-[100px]">
              {targetName}
            </span>
          </div>

          {/* 关系类型 */}
          <div className="mb-3">
            <label className="block text-xs font-medium text-surface-600 dark:text-surface-400 mb-1">
              关系类型
            </label>
            <select
              className={INPUT_CLASS}
              value={relationType}
              onChange={(e) => setRelationType(e.target.value as AdminRelationType)}
            >
              {RELATION_TYPE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>

          {/* 权重 */}
          <div className="mb-3">
            <label className="block text-xs font-medium text-surface-600 dark:text-surface-400 mb-1">
              权重: {weight.toFixed(2)}
            </label>
            <input
              type="range"
              min="0"
              max="1"
              step="0.05"
              className="w-full"
              value={weight}
              onChange={(e) => setWeight(parseFloat(e.target.value))}
            />
          </div>

          {/* 描述 */}
          <div className="mb-4">
            <label className="block text-xs font-medium text-surface-600 dark:text-surface-400 mb-1">
              描述 (可选)
            </label>
            <input
              className={INPUT_CLASS}
              placeholder="简要描述关系..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          {/* 按钮 */}
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={onCancel} disabled={saving}>
              取消
            </Button>
            <Button size="sm" onClick={handleConfirm} disabled={saving}>
              {saving && <Loader2 className="h-3 w-3 animate-spin mr-1" />}
              创建关系
            </Button>
          </div>
        </div>
      </div>
    );
  }
);

RelationCreatePanel.displayName = 'RelationCreatePanel';
