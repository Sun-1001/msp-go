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

interface ResourceListViewProps {
  resources: Resource[];
  selectionMode: boolean;
  selectedResourceIds: Set<string>;
  onToggleSelection: (id: string) => void;
  onToggleSelectAll: () => void;
  onViewResource: (resource: Resource) => void;
  onEditResource: (resource: Resource) => void;
  onDeleteResource: (id: string) => void;
}

export const ResourceListView = React.memo<ResourceListViewProps>(
  ({
    resources,
    selectionMode,
    selectedResourceIds,
    onToggleSelection,
    onToggleSelectAll,
    onViewResource,
    onEditResource,
    onDeleteResource,
  }) => {
    const handleOpenResource = useCallback((resource: Resource) => {
      openResourceUrl(resource.url);
    }, []);

    return (
      <Card>
        <CardContent className="p-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-200 dark:border-surface-700">
                {selectionMode && (
                  <th className="w-12 py-3 px-4">
                    <button
                      onClick={onToggleSelectAll}
                      className={cn(
                        "w-5 h-5 rounded border-2 flex items-center justify-center transition-colors",
                        selectedResourceIds.size === resources.length && resources.length > 0
                          ? "bg-primary-500 border-primary-500 text-white"
                          : "border-surface-400 dark:border-surface-500"
                      )}
                    >
                      {selectedResourceIds.size === resources.length && resources.length > 0 && (
                        <Check className="w-3 h-3" />
                      )}
                    </button>
                  </th>
                )}
                <th className="text-left py-3 px-4 font-medium text-surface-500">资源名称</th>
                <th className="text-left py-3 px-4 font-medium text-surface-500">类型</th>
                <th className="text-left py-3 px-4 font-medium text-surface-500">主题</th>
                <th className="text-center py-3 px-4 font-medium text-surface-500">浏览</th>
                <th className="text-center py-3 px-4 font-medium text-surface-500">点赞</th>
                <th className="text-left py-3 px-4 font-medium text-surface-500">创建时间</th>
                <th className="text-right py-3 px-4 font-medium text-surface-500">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-100 dark:divide-surface-800">
              {resources.map((resource) => {
                const config = typeConfig[resource.type as keyof typeof typeConfig];
                const Icon = config?.icon || FileText;
                const isSelected = selectedResourceIds.has(resource.id);

                return (
                  <tr
                    key={resource.id}
                    className={cn(
                      "hover:bg-surface-50 dark:hover:bg-surface-800/50 cursor-pointer",
                      isSelected && "bg-primary-50 dark:bg-primary-900/20"
                    )}
                    onClick={() => {
                      if (selectionMode) {
                        onToggleSelection(resource.id);
                      } else {
                        onViewResource(resource);
                      }
                    }}
                  >
                    {selectionMode && (
                      <td className="py-3 px-4" onClick={(e) => e.stopPropagation()}>
                        <button
                          onClick={() => onToggleSelection(resource.id)}
                          className={cn(
                            "w-5 h-5 rounded border-2 flex items-center justify-center transition-colors",
                            isSelected
                              ? "bg-primary-500 border-primary-500 text-white"
                              : "border-surface-300 dark:border-surface-600"
                          )}
                        >
                          {isSelected && <Check className="w-3 h-3" />}
                        </button>
                      </td>
                    )}
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-3">
                        <div
                          className={cn(
                            "p-2 rounded-lg",
                            config?.color || 'bg-surface-100 text-surface-600'
                          )}
                        >
                          <Icon className="w-4 h-4" />
                        </div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-surface-900 dark:text-surface-100">
                            {resource.title}
                          </span>
                          {resource.is_favorite && (
                            <Star className="w-4 h-4 text-amber-500 fill-amber-500" />
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="py-3 px-4 text-surface-600 dark:text-surface-400">
                      {config?.label || resource.type}
                    </td>
                    <td className="py-3 px-4 text-surface-600 dark:text-surface-400">
                      {resource.topic || '-'}
                    </td>
                    <td className="py-3 px-4 text-center text-surface-600 dark:text-surface-400">
                      {resource.views.toLocaleString()}
                    </td>
                    <td className="py-3 px-4 text-center text-surface-600 dark:text-surface-400">
                      {resource.likes.toLocaleString()}
                    </td>
                    <td className="py-3 px-4 text-surface-500">
                      {new Date(resource.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-3 px-4">
                      <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => onViewResource(resource)}
                        >
                          <Eye className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => onEditResource(resource)}
                        >
                          <Edit className="w-4 h-4 text-primary-500" />
                        </Button>
                        {resource.url && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => handleOpenResource(resource)}
                          >
                            <ExternalLink className="w-4 h-4" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-red-500 hover:text-red-600"
                          onClick={() => onDeleteResource(resource.id)}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </CardContent>
      </Card>
    );
  }
);

ResourceListView.displayName = 'ResourceListView';
