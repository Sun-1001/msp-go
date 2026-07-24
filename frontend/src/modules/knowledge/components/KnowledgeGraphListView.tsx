import { forwardRef, useImperativeHandle } from 'react';
import { BookOpen, CircleDot, Lightbulb, Target } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import type { KnowledgeNode } from '@/modules/knowledge/types/knowledge';
import type { GraphRendererHandle, GraphRendererProps } from './graphRendererTypes';

const nodeIcons = {
  concept: CircleDot,
  theorem: BookOpen,
  method: Lightbulb,
};

function masteryTone(mastery: number): string {
  if (mastery <= 0) return 'bg-surface-400';
  if (mastery >= 0.8) return 'bg-emerald-500';
  if (mastery >= 0.4) return 'bg-amber-500';
  return 'bg-[#f97360]';
}

const KnowledgeGraphListView = forwardRef<GraphRendererHandle, GraphRendererProps>(({
  nodes,
  selectedNodeId,
  targetNodeId,
  highlightedNodeIds,
  onNodeClick,
  onNodeHover,
}, ref) => {
  useImperativeHandle(ref, () => ({
    zoomIn: () => undefined,
    zoomOut: () => undefined,
    fitView: () => undefined,
  }), []);

  return (
    <div
      className="absolute inset-0 overflow-y-auto bg-white p-3 dark:bg-surface-900 sm:p-4"
      data-graph-renderer="list"
      role="list"
      aria-label="知识点列表视图"
    >
      <div className="mx-auto grid max-w-4xl grid-cols-1 gap-2 md:grid-cols-2">
        {nodes.map((node) => (
          <KnowledgeNodeRow
            key={node.id}
            node={node}
            selected={node.id === selectedNodeId}
            target={node.id === targetNodeId}
            highlighted={highlightedNodeIds.has(node.id)}
            onClick={() => onNodeClick?.(node)}
            onHover={onNodeHover}
          />
        ))}
      </div>
    </div>
  );
});

KnowledgeGraphListView.displayName = 'KnowledgeGraphListView';

function KnowledgeNodeRow({
  node,
  selected,
  target,
  highlighted,
  onClick,
  onHover,
}: {
  node: KnowledgeNode;
  selected: boolean;
  target: boolean;
  highlighted: boolean;
  onClick: () => void;
  onHover?: (node: KnowledgeNode | null) => void;
}) {
  const Icon = nodeIcons[node.type];
  return (
    <button
      type="button"
      role="listitem"
      onClick={onClick}
      onFocus={() => onHover?.(node)}
      onBlur={() => onHover?.(null)}
      onMouseEnter={() => onHover?.(node)}
      onMouseLeave={() => onHover?.(null)}
      aria-current={selected ? 'true' : undefined}
      className={cn(
        'content-visibility-auto flex min-h-20 w-full items-center gap-3 rounded-md border px-3 py-3 text-left transition-colors',
        selected
          ? 'border-primary-500 bg-primary-50 dark:bg-primary-950/40'
          : 'border-surface-200 bg-white hover:border-surface-400 dark:border-surface-700 dark:bg-surface-900 dark:hover:border-surface-500',
        highlighted && !selected ? 'border-amber-400' : '',
      )}
    >
      <span className={cn('h-10 w-1 shrink-0 rounded-full', masteryTone(node.mastery))} />
      <Icon className="h-5 w-5 shrink-0 text-surface-500" aria-hidden="true" />
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2 font-medium text-surface-900 dark:text-surface-100">
          <span className="truncate">{node.label}</span>
          {target ? <Target className="h-4 w-4 shrink-0 text-amber-600" aria-label="当前目标" /> : null}
        </span>
        <span className="mt-1 block text-xs text-surface-500 dark:text-surface-400">
          {node.chapter || '未分章节'} · 掌握度 {Math.round(node.mastery * 100)}%
        </span>
      </span>
    </button>
  );
}

export default KnowledgeGraphListView;
