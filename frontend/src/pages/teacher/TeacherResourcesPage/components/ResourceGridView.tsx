import React from 'react';
import type { Resource } from '@/modules/resource/api';
import { ResourceCard } from './ResourceCard';
import { Card, CardContent } from '../../../../components/ui/Card';
import { Plus } from 'lucide-react';

interface ResourceGridViewProps {
  resources: Resource[];
  selectionMode: boolean;
  selectedResourceIds: Set<string>;
  onToggleSelection: (id: string) => void;
  onViewResource: (resource: Resource) => void;
  onEditResource: (resource: Resource) => void;
  onDeleteResource: (id: string) => void;
  onOpenBatchImport: () => void;
}

export const ResourceGridView = React.memo<ResourceGridViewProps>(
  ({
    resources,
    selectionMode,
    selectedResourceIds,
    onToggleSelection,
    onViewResource,
    onEditResource,
    onDeleteResource,
    onOpenBatchImport,
  }) => {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {resources.map((resource) => (
          <ResourceCard
            key={resource.id}
            resource={resource}
            isSelected={selectedResourceIds.has(resource.id)}
            selectionMode={selectionMode}
            onToggleSelection={onToggleSelection}
            onView={onViewResource}
            onEdit={onEditResource}
            onDelete={onDeleteResource}
          />
        ))}

        {/* 添加新资源卡片 */}
        <Card
          className="border-dashed border-2 hover:border-emerald-400 dark:hover:border-emerald-600 transition-colors cursor-pointer group"
          onClick={onOpenBatchImport}
        >
          <CardContent className="h-full flex flex-col items-center justify-center py-12 text-surface-400 group-hover:text-emerald-500 transition-colors">
            <Plus className="w-10 h-10 mb-2" />
            <span className="text-sm font-medium">上传新资源</span>
          </CardContent>
        </Card>
      </div>
    );
  }
);

ResourceGridView.displayName = 'ResourceGridView';
