import {
  forwardRef,
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from 'react';
import {
  Box,
  Expand,
  List,
  Maximize2,
  Minimize2,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { useKnowledgeGraphMode } from '@/modules/knowledge/hooks/useKnowledgeGraphMode';
import type {
  KnowledgeEdge,
  KnowledgeGraphViewMode,
  KnowledgeNode,
  ResolvedKnowledgeGraphViewMode,
} from '@/modules/knowledge/types/knowledge';
import type { GraphRendererHandle, GraphRendererProps } from './graphRendererTypes';

const KnowledgeGraphRenderer3D = lazy(() => import('./KnowledgeGraphRenderer3D'));
const KnowledgeGraphRenderer2D = lazy(() => import('./KnowledgeGraphRenderer2D'));
const KnowledgeGraphListView = lazy(() => import('./KnowledgeGraphListView'));

export type { KnowledgeNode, KnowledgeEdge } from '@/modules/knowledge/types/knowledge';

export interface KnowledgeGraphProps {
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
  selectedNodeId?: string | null;
  targetNodeId?: string | null;
  highlightedNodeIds?: ReadonlySet<string>;
  highlightedEdgeKeys?: ReadonlySet<string>;
  viewMode?: KnowledgeGraphViewMode;
  onResolvedViewModeChange?: (mode: ResolvedKnowledgeGraphViewMode) => void;
  onNodeClick?: (node: KnowledgeNode) => void;
  onNodeHover?: (node: KnowledgeNode | null) => void;
  className?: string;
  height?: CSSProperties['height'];
  showToolbar?: boolean;
}

export const KnowledgeGraph = ({
  nodes,
  edges,
  selectedNodeId = null,
  targetNodeId = null,
  highlightedNodeIds = EMPTY_SET,
  highlightedEdgeKeys = EMPTY_SET,
  viewMode = 'auto',
  onResolvedViewModeChange,
  onNodeClick,
  onNodeHover,
  className,
  height = 640,
  showToolbar = true,
}: KnowledgeGraphProps) => {
  const rendererRef = useRef<GraphRendererHandle | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [failures, setFailures] = useState({ threeD: false, twoD: false });
  const { mode, reducedMotion } = useKnowledgeGraphMode(viewMode, nodes.length, failures);
  const fallbackMessage = getFallbackMessage(viewMode, mode, failures);

  useEffect(() => {
    onResolvedViewModeChange?.(mode);
  }, [mode, onResolvedViewModeChange]);

  useEffect(() => {
    if (!isFullscreen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setIsFullscreen(false);
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isFullscreen]);

  const handleRendererError = useCallback((error: Error) => {
    if (mode === '3d') {
      setFailures((current) => ({ ...current, threeD: true }));
      return;
    }
    if (mode === '2d') {
      setFailures((current) => ({ ...current, twoD: true }));
      return;
    }
    console.error('Knowledge graph renderer failed', error);
  }, [mode]);

  const rendererProps: GraphRendererProps = {
    nodes,
    edges,
    selectedNodeId,
    targetNodeId,
    highlightedNodeIds,
    highlightedEdgeKeys,
    reducedMotion,
    onNodeClick,
    onNodeHover,
    onRendererError: handleRendererError,
  };

  const stage = (
    <GraphStage
      ref={rendererRef}
      mode={mode}
      rendererProps={rendererProps}
      fallbackMessage={fallbackMessage}
      showToolbar={showToolbar}
      fullscreen={isFullscreen}
      onZoomIn={() => rendererRef.current?.zoomIn()}
      onZoomOut={() => rendererRef.current?.zoomOut()}
      onFit={() => rendererRef.current?.fitView()}
      onToggleFullscreen={() => setIsFullscreen((current) => !current)}
    />
  );

  return (
    <div
      className={cn(
        isFullscreen
          ? 'fixed inset-0 z-60 overflow-hidden bg-[#111315]'
          : 'relative overflow-hidden rounded-md border border-[#2d3238] bg-[#111315]',
        !isFullscreen && className,
      )}
      style={isFullscreen ? undefined : { height }}
      data-resolved-view-mode={mode}
      role={isFullscreen ? 'dialog' : undefined}
      aria-modal={isFullscreen ? true : undefined}
      aria-label={isFullscreen ? '知识星图全屏视图' : undefined}
    >
      <div className={cn('absolute inset-x-0 bottom-0', isFullscreen ? 'top-14' : 'top-0')}>
        {stage}
      </div>
      {isFullscreen ? (
        <div className="absolute inset-x-0 top-0 flex h-14 items-center justify-between border-b border-white/10 px-4 text-white sm:px-6">
          <div className="flex items-center gap-3">
            <Box className="h-5 w-5 text-amber-400" aria-hidden="true" />
            <h2 className="text-base font-semibold tracking-normal">知识星图全景</h2>
          </div>
          <button
            type="button"
            onClick={() => setIsFullscreen(false)}
            className="inline-flex h-10 w-10 items-center justify-center rounded-md text-surface-300 hover:bg-white/10 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
            title="退出全屏"
            aria-label="退出全屏"
          >
            <Minimize2 className="h-5 w-5" />
          </button>
        </div>
      ) : null}
      <p className="sr-only">
        当前以 {mode === '3d' ? '三维图谱' : mode === '2d' ? '二维图谱' : '列表'} 展示
        {nodes.length} 个知识点。可通过页面右侧的节点索引使用键盘选择知识点。
      </p>
    </div>
  );
};

interface GraphStageProps {
  mode: ResolvedKnowledgeGraphViewMode;
  rendererProps: GraphRendererProps;
  fallbackMessage: string | null;
  showToolbar: boolean;
  fullscreen: boolean;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
  onToggleFullscreen: () => void;
}

const GraphStage = forwardRef<GraphRendererHandle, GraphStageProps>(function GraphStage({
    mode,
    rendererProps,
    fallbackMessage,
    showToolbar,
    fullscreen,
    onZoomIn,
    onZoomOut,
    onFit,
    onToggleFullscreen,
  }, ref) {
  return (
    <div className="absolute inset-0">
      <Suspense
        fallback={(
          <div className="absolute inset-0 flex items-center justify-center bg-[#111315] text-sm text-surface-300">
            正在准备图谱视图...
          </div>
        )}
      >
        {mode === '3d' ? <KnowledgeGraphRenderer3D ref={ref} {...rendererProps} /> : null}
        {mode === '2d' ? <KnowledgeGraphRenderer2D ref={ref} {...rendererProps} /> : null}
        {mode === 'list' ? <KnowledgeGraphListView ref={ref} {...rendererProps} /> : null}
      </Suspense>

      {fallbackMessage ? (
        <div
          className="pointer-events-none absolute left-1/2 top-3 z-10 max-w-[calc(100%-7rem)] -translate-x-1/2 rounded-md border border-amber-400/40 bg-[#1a1d20]/94 px-3 py-2 text-center text-xs text-amber-100 shadow-lg"
          role="status"
        >
          {fallbackMessage}
        </div>
      ) : null}

      <div className="pointer-events-none absolute bottom-3 left-3 z-10 inline-flex h-7 items-center gap-1.5 rounded-md border border-white/10 bg-[#1a1d20]/88 px-2.5 text-xs font-medium text-surface-300">
        {mode === '3d' ? <Box className="h-3.5 w-3.5" /> : null}
        {mode === '2d' ? <Maximize2 className="h-3.5 w-3.5" /> : null}
        {mode === 'list' ? <List className="h-3.5 w-3.5" /> : null}
        {mode === '3d' ? '3D 星图' : mode === '2d' ? '2D 图谱' : '节点列表'}
      </div>

      {showToolbar ? (
        <GraphToolbar
          listMode={mode === 'list'}
          fullscreen={fullscreen}
          onZoomIn={onZoomIn}
          onZoomOut={onZoomOut}
          onFit={onFit}
          onToggleFullscreen={onToggleFullscreen}
        />
      ) : null}
    </div>
  );
});

GraphStage.displayName = 'GraphStage';

function GraphToolbar({
  listMode,
  fullscreen,
  onZoomIn,
  onZoomOut,
  onFit,
  onToggleFullscreen,
}: {
  listMode: boolean;
  fullscreen: boolean;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
  onToggleFullscreen: () => void;
}) {
  return (
    <div className="absolute right-3 top-3 z-20 flex items-center gap-1 rounded-md border border-white/10 bg-[#1a1d20]/92 p-1 shadow-lg">
      <ToolButton label="放大" disabled={listMode} onClick={onZoomIn}>
        <ZoomIn className="h-4 w-4" />
      </ToolButton>
      <ToolButton label="缩小" disabled={listMode} onClick={onZoomOut}>
        <ZoomOut className="h-4 w-4" />
      </ToolButton>
      <ToolButton label="适应视图" disabled={listMode} onClick={onFit}>
        <Maximize2 className="h-4 w-4" />
      </ToolButton>
      <ToolButton label={fullscreen ? '退出全屏' : '全屏'} onClick={onToggleFullscreen}>
        {fullscreen ? <Minimize2 className="h-4 w-4" /> : <Expand className="h-4 w-4" />}
      </ToolButton>
    </div>
  );
}

function ToolButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="inline-flex h-9 w-9 items-center justify-center rounded-md text-surface-300 hover:bg-white/10 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-400 disabled:cursor-not-allowed disabled:opacity-35"
      title={label}
      aria-label={label}
    >
      {children}
    </button>
  );
}

export const KnowledgeGraphLegend = () => (
  <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-surface-500 dark:text-surface-400">
    <LegendItem color="bg-emerald-500" label="已掌握" />
    <LegendItem color="bg-amber-500" label="学习中" />
    <LegendItem color="bg-[#f97360]" label="薄弱" />
    <LegendItem color="bg-surface-500" label="未开始" />
    <span className="inline-flex items-center gap-1.5">
      <span className="h-px w-5 bg-surface-400" />先修
    </span>
    <span className="inline-flex items-center gap-1.5">
      <span className="h-px w-5 bg-teal-400" />应用
    </span>
    <span className="inline-flex items-center gap-1.5">
      <span className="h-px w-5 bg-[#f97360]" />相关
    </span>
  </div>
);

function LegendItem({ color, label }: { color: string; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={cn('h-2.5 w-2.5 rounded-full', color)} />{label}
    </span>
  );
}

const EMPTY_SET: ReadonlySet<string> = new Set();

function getFallbackMessage(
  requestedMode: KnowledgeGraphViewMode,
  resolvedMode: ResolvedKnowledgeGraphViewMode,
  failures: { threeD: boolean; twoD: boolean },
): string | null {
  if (failures.twoD && resolvedMode === 'list') {
    return '图形渲染不可用，已切换到可访问列表';
  }
  if (failures.threeD && resolvedMode === '2d') {
    return '3D 渲染不可用，已保留学习操作并切换到 2D 视图';
  }
  if (requestedMode === '3d' && resolvedMode !== '3d') {
    return '当前设备或数据规模不适合 3D，已切换到 2D 视图';
  }
  if (requestedMode === '2d' && resolvedMode === 'list') {
    return '节点数量较多，已切换到列表视图';
  }
  return null;
}
