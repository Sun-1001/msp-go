import React from 'react';
import type { Resource } from '@/modules/resource/types/resource';
import { Modal } from '../../../../components/ui/Modal';
import { Button } from '../../../../components/ui/Button';
import {
  Tag,
  FileText,
  Video,
  Calendar,
  Eye,
  Star,
  Edit,
  ExternalLink,
  Link as LinkIcon,
} from 'lucide-react';
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

interface ResourceDetailModalProps {
  resource: Resource | null;
  onClose: () => void;
  onEdit: (resource: Resource) => void;
}

export const ResourceDetailModal = React.memo<ResourceDetailModalProps>(
  ({ resource, onClose, onEdit }) => {
    if (!resource) return null;

    const config = typeConfig[resource.type as keyof typeof typeConfig];

    const handleOpenResource = () => {
      openResourceUrl(resource.url);
    };

    return (
      <Modal isOpen={!!resource} onClose={onClose} title="资源详情">
        <div className="space-y-6">
          {/* 资源类型和标题 */}
          <div>
            <div className="flex items-center gap-2 mb-2">
              <span
                className={cn(
                  "px-2 py-0.5 text-xs font-medium rounded-full",
                  config?.color || 'bg-surface-100 text-surface-600'
                )}
              >
                {config?.label || resource.type}
              </span>
              {resource.is_favorite && <Star className="w-4 h-4 text-amber-500 fill-amber-500" />}
            </div>
            <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100">
              {resource.title}
            </h3>
          </div>

          {/* 基本信息 */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            {resource.source && (
              <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
                <Tag className="w-4 h-4" />
                <span>来源: {resource.source}</span>
              </div>
            )}
            {resource.topic && (
              <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
                <FileText className="w-4 h-4" />
                <span>主题: {resource.topic}</span>
              </div>
            )}
            {resource.chapter && (
              <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
                <FileText className="w-4 h-4" />
                <span>章节: {resource.chapter}</span>
              </div>
            )}
            {resource.type === 'video' && resource.duration && (
              <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
                <Video className="w-4 h-4" />
                <span>时长: {resource.duration}</span>
              </div>
            )}
            {resource.type !== 'video' && resource.pages && (
              <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
                <FileText className="w-4 h-4" />
                <span>页数: {resource.pages}</span>
              </div>
            )}
            <div className="flex items-center gap-2 text-surface-600 dark:text-surface-400">
              <Calendar className="w-4 h-4" />
              <span>创建: {new Date(resource.created_at).toLocaleDateString()}</span>
            </div>
          </div>

          {/* 统计信息 */}
          <div className="flex items-center gap-6 py-3 px-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
            <div className="flex items-center gap-2">
              <Eye className="w-4 h-4 text-surface-500" />
              <span className="text-surface-900 dark:text-surface-100 font-medium">
                {resource.views.toLocaleString()}
              </span>
              <span className="text-surface-500 text-sm">浏览</span>
            </div>
            <div className="flex items-center gap-2">
              <Star className="w-4 h-4 text-surface-500" />
              <span className="text-surface-900 dark:text-surface-100 font-medium">
                {resource.likes.toLocaleString()}
              </span>
              <span className="text-surface-500 text-sm">点赞</span>
            </div>
          </div>

          {/* 描述 */}
          {resource.body && (
            <div>
              <h4 className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">描述</h4>
              <p className="text-surface-600 dark:text-surface-400 text-sm whitespace-pre-wrap">
                {resource.body}
              </p>
            </div>
          )}

          {/* 资源链接 */}
          {resource.url && (
            <div>
              <h4 className="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                资源链接
              </h4>
              <div className="flex items-center gap-2 p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
                <LinkIcon className="w-4 h-4 text-surface-500 shrink-0" />
                <span className="text-sm text-surface-600 dark:text-surface-400 truncate flex-1">
                  {resource.url}
                </span>
              </div>
            </div>
          )}

          {/* 操作按钮 */}
          <div className="flex justify-end gap-2 pt-4 border-t border-surface-200 dark:border-surface-700">
            <Button variant="outline" onClick={onClose}>
              关闭
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                onEdit(resource);
                onClose();
              }}
            >
              <Edit className="w-4 h-4 mr-2" />
              编辑
            </Button>
            {resource.url && (
              <Button onClick={handleOpenResource}>
                <ExternalLink className="w-4 h-4 mr-2" />
                打开资源
              </Button>
            )}
          </div>
        </div>
      </Modal>
    );
  }
);

ResourceDetailModal.displayName = 'ResourceDetailModal';
