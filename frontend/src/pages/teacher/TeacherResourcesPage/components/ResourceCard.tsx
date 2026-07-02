import React, { useCallback } from 'react';
import type { Resource } from '@/modules/resource/types/resource';
import { Card, CardContent } from '../../../../components/ui/Card';
import { Button } from '../../../../components/ui/Button';
import { Video, FileText, Star, Eye, Edit, Trash2, ExternalLink, Check } from 'lucide-react';
import { cn } from '../../../../libs/utils/cn';
import { openResourceUrl } from '@/libs/utils/resourceUtils';

const typeConfig = {
  video: {
    icon: Video,
    label: '视频',
    color: 'bg-primary-100 text-primary-600 dark:bg-primary-900/50 dark:text-primary-400',
  },
  document: {
    icon: FileText,
    label: '文档',
    color: 'bg-secondary-100 text-secondary-600 dark:bg-secondary-900/50 dark:text-secondary-400',
  },
};

interface ResourceCardProps {
  resource: Resource;
  isSelected: boolean;
  selectionMode: boolean;
  onToggleSelection: (id: string) => void;
  onView: (resource: Resource) => void;
  onEdit: (resource: Resource) => void;
  onDelete: (id: string) => void;
}

export const ResourceCard = React.memo<ResourceCardProps>(
  ({ resource, isSelected, selectionMode, onToggleSelection, onView, onEdit, onDelete }) => {
    const config = typeConfig[resource.type as keyof typeof typeConfig];
    const Icon = config?.icon || FileText;

    const handleOpenResource = useCallback(() => {
      openResourceUrl(resource.url);
    }, [resource.url]);

    const handleCardClick = useCallback(() => {
      if (selectionMode) {
        onToggleSelection(resource.id);
      } else {
        onView(resource);
      }
    }, [selectionMode, onToggleSelection, onView, resource]);

    const handleEditClick = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onEdit(resource);
      },
      [onEdit, resource]
    );

    const handleDeleteClick = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onDelete(resource.id);
      },
      [onDelete, resource.id]
    );

    const handleOpenClick = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        handleOpenResource();
      },
      [handleOpenResource]
    );

    const handleSelectionClick = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onToggleSelection(resource.id);
      },
      [onToggleSelection, resource.id]
    );

    return (
      <Card
        className={cn(
          "group hover:border-emerald-300 dark:hover:border-emerald-700 transition-all cursor-pointer",
          isSelected && "border-primary-400 dark:border-primary-600 bg-primary-50/50 dark:bg-primary-900/10"
        )}
        onClick={handleCardClick}
      >
        <CardContent className="p-0">
          {/* 预览区域 */}
          <div
            className={cn(
              "h-36 flex items-center justify-center relative",
              resource.type === 'video'
                ? "bg-linear-to-br from-primary-50 to-primary-100 dark:from-primary-900/30 dark:to-primary-800/30"
                : "bg-linear-to-br from-secondary-50 to-secondary-100 dark:from-secondary-900/30 dark:to-secondary-800/30"
            )}
          >
            {/* 选择框 */}
            {selectionMode && (
              <button
                onClick={handleSelectionClick}
                className={cn(
                  "absolute top-3 left-3 w-6 h-6 rounded border-2 flex items-center justify-center transition-colors z-10",
                  isSelected
                    ? "bg-primary-500 border-primary-500 text-white"
                    : "border-white bg-white/80 dark:border-surface-300 dark:bg-surface-800/80"
                )}
              >
                {isSelected && <Check className="w-4 h-4" />}
              </button>
            )}
            <Icon className={cn("w-12 h-12", config?.color.split(' ')[1] || 'text-surface-400')} />
            {resource.is_favorite && (
              <Star className="absolute top-3 right-3 w-5 h-5 text-amber-500 fill-amber-500" />
            )}
            {resource.type === 'video' && resource.duration && (
              <span className="absolute bottom-3 right-3 px-2 py-1 bg-black/70 text-white text-xs rounded">
                {resource.duration}
              </span>
            )}
          </div>

          {/* 信息区域 */}
          <div className="p-4">
            <div className="flex items-start justify-between mb-2">
              <span
                className={cn(
                  "px-2 py-0.5 text-xs font-medium rounded-full",
                  config?.color || 'bg-surface-100 text-surface-600'
                )}
              >
                {config?.label || resource.type}
              </span>
              <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleEditClick}>
                  <Edit className="w-4 h-4 text-primary-500" />
                </Button>
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleDeleteClick}>
                  <Trash2 className="w-4 h-4 text-red-500" />
                </Button>
              </div>
            </div>
            <h3 className="font-medium text-surface-900 dark:text-surface-100 mb-2 line-clamp-2">
              {resource.title}
            </h3>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4 text-xs text-surface-500">
                <span className="flex items-center gap-1">
                  <Eye className="w-3 h-3" /> {resource.views.toLocaleString()}
                </span>
                <span className="flex items-center gap-1">
                  <Star className="w-3 h-3" /> {resource.likes.toLocaleString()}
                </span>
              </div>
              {resource.url && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity"
                  onClick={handleOpenClick}
                >
                  <ExternalLink className="w-4 h-4" />
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    );
  },
  (prevProps, nextProps) => {
    // 自定义比较函数：只有这些属性变化时才重新渲染
    return (
      prevProps.resource.id === nextProps.resource.id &&
      prevProps.isSelected === nextProps.isSelected &&
      prevProps.selectionMode === nextProps.selectionMode &&
      prevProps.resource.title === nextProps.resource.title &&
      prevProps.resource.views === nextProps.resource.views &&
      prevProps.resource.likes === nextProps.resource.likes
    );
  }
);

ResourceCard.displayName = 'ResourceCard';
