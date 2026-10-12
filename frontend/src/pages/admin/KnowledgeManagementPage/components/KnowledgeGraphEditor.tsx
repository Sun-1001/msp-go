import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import type { Graph } from '@antv/g6';
import { ZoomIn, ZoomOut, Maximize2, RotateCcw, Loader2, Search, Link } from 'lucide-react';
import { cn } from '@/libs/utils/cn';
import { animationCombos } from '@/libs/animations';
import { graphZoomIn, graphZoomOut, graphFitView } from '@/libs/graph';
import {
  createAdminGraphInstance,
  updateAdminGraphData,
  enableCreateEdgeMode,
  disableCreateEdgeMode,
} from '@/libs/graph/createAdminGraphInstance';
import { ADMIN_NODE_LEGEND, ADMIN_EDGE_LEGEND } from '@/libs/graph/adminGraphConfig';
import { RelationCreatePanel } from './RelationCreatePanel';
import { EdgeDetailPanel } from './EdgeDetailPanel';
import type {
  SimpleNode,
  KnowledgeRelationAdmin,
  KnowledgeRelationCreateData,
  AdminRelationType,
} from '@/modules/admin/api';
import { NODE_TYPE_OPTIONS } from '../constants';

interface KnowledgeGraphEditorProps {
  allNodes: SimpleNode[];
  relations: KnowledgeRelationAdmin[];
  relationsLoading: boolean;
  saving: boolean;
  nodeTypeMap: Map<string, string>;
  chapters: string[];
  onCreateRelation: (data: KnowledgeRelationCreateData) => Promise<void>;
  onEditRelation: (relation: KnowledgeRelationAdmin) => void;
  onDeleteRelation: (id: string, name: string) => void;
}

interface PendingEdge {
  sourceId: string;
  targetId: string;
  sourceName: string;
  targetName: string;
}

/**
 * 知识图谱可视化编辑器
 *
 * 左键拖拽移动节点，点击「连线」按钮进入连线模式后依次点击两个节点创建关系
 * 点击边查看/编辑/删除关系
 */
export const KnowledgeGraphEditor: React.FC<KnowledgeGraphEditorProps> = ({
  allNodes, relations, relationsLoading, saving,
  nodeTypeMap, chapters,
  onCreateRelation, onEditRelation, onDeleteRelation,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const graphRef = useRef<Graph | null>(null);
  const [isReady, setIsReady] = useState(false);

  // 连线模式
  const [connectMode, setConnectMode] = useState(false);
  const [connectSourceId, setConnectSourceId] = useState<string | null>(null);
  // 面板状态
  const [pendingEdge, setPendingEdge] = useState<PendingEdge | null>(null);
  const [selectedRelation, setSelectedRelation] = useState<KnowledgeRelationAdmin | null>(null);

  // 筛选状态
  const [searchTerm, setSearchTerm] = useState('');
  const [chapterFilter, setChapterFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');

  // Ref 缓存
  const allNodesRef = useRef(allNodes);
  const relationsRef = useRef(relations);
  const nodeTypeMapRef = useRef(nodeTypeMap);
  const connectModeRef = useRef(connectMode);
  const connectSourceIdRef = useRef<string | null>(connectSourceId);
  const pendingEdgeRef = useRef<PendingEdge | null>(pendingEdge);

  useEffect(() => { allNodesRef.current = allNodes; }, [allNodes]);
  useEffect(() => { relationsRef.current = relations; }, [relations]);
  useEffect(() => { nodeTypeMapRef.current = nodeTypeMap; }, [nodeTypeMap]);
  useEffect(() => { connectModeRef.current = connectMode; }, [connectMode]);
  useEffect(() => { connectSourceIdRef.current = connectSourceId; }, [connectSourceId]);
  useEffect(() => { pendingEdgeRef.current = pendingEdge; }, [pendingEdge]);

  // 筛选
  const filteredNodes = useMemo(() => {
    let result = allNodes;
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      result = result.filter((n) => n.name.toLowerCase().includes(term));
    }
    if (chapterFilter) result = result.filter((n) => n.chapter === chapterFilter);
    if (typeFilter) result = result.filter((n) => nodeTypeMap.get(n.id) === typeFilter);
    return result;
  }, [allNodes, searchTerm, chapterFilter, typeFilter, nodeTypeMap]);

  const filteredRelations = useMemo(() => {
    const ids = new Set(filteredNodes.map((n) => n.id));
    return relations.filter((r) => ids.has(r.source_id) && ids.has(r.target_id));
  }, [filteredNodes, relations]);

  const getNodeName = useCallback((nodeId: string): string =>
    allNodesRef.current.find((n) => n.id === nodeId)?.name || nodeId, []);

  // 连线创建回调（含重复校验）
  const handleCreateEdge = useCallback((sourceId: string, targetId: string) => {
    const exists = relationsRef.current.some(
      (r) => r.source_id === sourceId && r.target_id === targetId,
    );
    if (exists) return;
    setPendingEdge({
      sourceId, targetId,
      sourceName: getNodeName(sourceId),
      targetName: getNodeName(targetId),
    });
  }, [getNodeName]);

  const handleEdgeClick = useCallback((edgeId: string) => {
    const rel = relationsRef.current.find((r) => r.id === edgeId);
    if (rel) {
      setSelectedRelation(rel);
      setPendingEdge(null);
      pendingEdgeRef.current = null;
      setConnectSourceId(null);
      connectSourceIdRef.current = null;
    }
  }, []);

  const handleNodeClick = useCallback((nodeId: string) => {
    setSelectedRelation(null);

    if (!connectModeRef.current || pendingEdgeRef.current) return;

    const sourceId = connectSourceIdRef.current;
    if (!sourceId) {
      connectSourceIdRef.current = nodeId;
      setConnectSourceId(nodeId);
      return;
    }

    if (sourceId === nodeId) {
      connectSourceIdRef.current = null;
      setConnectSourceId(null);
      return;
    }

    handleCreateEdge(sourceId, nodeId);
    connectSourceIdRef.current = null;
    setConnectSourceId(null);
  }, [handleCreateEdge]);

  // 初始化图谱
  useEffect(() => {
    if (!containerRef.current) return;
    let isMounted = true;
    const initTimer = setTimeout(() => {
      if (!isMounted || !containerRef.current || graphRef.current) return;
      const graph = createAdminGraphInstance({
        container: containerRef.current,
        width: containerRef.current.offsetWidth,
        height: containerRef.current.offsetHeight,
        nodes: filteredNodes,
        relations: filteredRelations,
        nodeTypeMap: nodeTypeMapRef.current,
        onNodeClick: handleNodeClick,
        onEdgeClick: handleEdgeClick,
      });
      graphRef.current = graph;
      graph.render()
        .then(() => { if (isMounted && graphRef.current) setIsReady(true); })
        .catch(() => {});
    }, 0);
    const handleResize = () => {
      if (containerRef.current && graphRef.current) {
        graphRef.current.setSize(containerRef.current.offsetWidth, containerRef.current.offsetHeight);
      }
    };
    window.addEventListener('resize', handleResize);
    return () => {
      isMounted = false;
      clearTimeout(initTimer);
      window.removeEventListener('resize', handleResize);
      if (graphRef.current) { graphRef.current.destroy(); graphRef.current = null; }
      setIsReady(false);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 数据变化时更新图谱
  useEffect(() => {
    if (graphRef.current && isReady) {
      updateAdminGraphData(graphRef.current, filteredNodes, filteredRelations, nodeTypeMap);
    }
  }, [filteredNodes, filteredRelations, nodeTypeMap, isReady]);

  // 连线模式切换
  const toggleConnectMode = useCallback(() => {
    if (!graphRef.current || !isReady) return;
    const next = !connectModeRef.current;
    connectModeRef.current = next;
    setConnectMode(next);
    connectSourceIdRef.current = null;
    setConnectSourceId(null);
    if (next) {
      enableCreateEdgeMode(graphRef.current);
    } else {
      disableCreateEdgeMode(graphRef.current);
    }
  }, [isReady]);

  // 退出连线模式（创建面板关闭时）
  const exitConnectMode = useCallback(() => {
    connectModeRef.current = false;
    setConnectMode(false);
    connectSourceIdRef.current = null;
    setConnectSourceId(null);
    pendingEdgeRef.current = null;
    setPendingEdge(null);
    if (graphRef.current) disableCreateEdgeMode(graphRef.current);
  }, []);

  // 缩放控制
  const handleZoomIn = useCallback(() => graphZoomIn(graphRef.current), []);
  const handleZoomOut = useCallback(() => graphZoomOut(graphRef.current), []);
  const handleFitView = useCallback(() => graphFitView(graphRef.current), []);
  const handleReLayout = useCallback(() => {
    if (graphRef.current && isReady) { graphRef.current.layout(); graphRef.current.fitView(); }
  }, [isReady]);

  // 创建关系确认
  const handleConfirmCreate = useCallback(
    async (data: { relation_type: AdminRelationType; weight: number; description?: string }) => {
      if (!pendingEdge) return;
      try {
        await onCreateRelation({
          source_id: pendingEdge.sourceId, target_id: pendingEdge.targetId, ...data,
        });
        setPendingEdge(null);
        // 保持连线模式，方便连续创建
      } catch { /* 失败时保留面板 */ }
    },
    [pendingEdge, onCreateRelation],
  );


  return (
    <div className="space-y-3">
      {/* 筛选栏 */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="relative flex-1 min-w-[180px] max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-surface-400" />
          <input
            className="w-full pl-8 pr-3 py-1.5 rounded-lg border border-surface-300 dark:border-surface-600 bg-white dark:bg-surface-800 text-surface-900 dark:text-surface-100 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            placeholder="搜索知识点..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
        <select
          className="px-3 py-1.5 rounded-lg border border-surface-300 dark:border-surface-600 bg-white dark:bg-surface-800 text-surface-900 dark:text-surface-100 text-sm"
          value={chapterFilter}
          onChange={(e) => setChapterFilter(e.target.value)}
        >
          <option value="">全部章节</option>
          {chapters.map((ch) => (
            <option key={ch} value={ch}>{ch}</option>
          ))}
        </select>
        <select
          className="px-3 py-1.5 rounded-lg border border-surface-300 dark:border-surface-600 bg-white dark:bg-surface-800 text-surface-900 dark:text-surface-100 text-sm"
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
        >
          {NODE_TYPE_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
        <span className="text-xs text-surface-500">
          {filteredNodes.length} 节点 · {filteredRelations.length} 关系
        </span>
      </div>

      {/* 图谱容器 */}
      <div className={cn('relative', connectMode && 'ring-2 ring-primary-400 rounded-xl')} style={{ height: 520 }}>
        {relationsLoading && (
          <div className="absolute inset-0 z-30 flex items-center justify-center bg-white/60 dark:bg-surface-900/60 rounded-xl">
            <Loader2 className="h-6 w-6 animate-spin text-primary-500" />
            <span className="ml-2 text-sm text-surface-500">加载中...</span>
          </div>
        )}

        <div
          ref={containerRef}
          className={cn(
            'w-full h-full rounded-xl bg-white dark:bg-surface-900 border border-surface-200 dark:border-surface-700 overflow-hidden',
            connectMode && 'cursor-crosshair',
          )}
        />

        {/* 工具栏 */}
        {isReady && (
          <div className="absolute top-3 right-3 flex items-center gap-1 bg-white/90 dark:bg-surface-800/90 backdrop-blur-sm rounded-lg shadow-lg border border-surface-200 dark:border-surface-700 p-1">
            {/* 连线模式切换 */}
            <button
              type="button"
              onClick={toggleConnectMode}
              className={cn(
                'p-2 rounded-md',
                connectMode
                  ? 'bg-primary-100 dark:bg-primary-900/40 text-primary-600 dark:text-primary-400'
                  : 'text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-700 hover:text-primary-600 dark:hover:text-primary-400',
                animationCombos.buttonHover,
              )}
              title={connectMode ? '退出连线模式' : '进入连线模式'}
            >
              <Link className="w-4 h-4" />
            </button>
            <div className="w-px h-5 bg-surface-200 dark:bg-surface-700" />
            {[
              { icon: ZoomIn, title: '放大', onClick: handleZoomIn },
              { icon: ZoomOut, title: '缩小', onClick: handleZoomOut },
              { icon: Maximize2, title: '适应视图', onClick: handleFitView },
              { icon: RotateCcw, title: '重新布局', onClick: handleReLayout },
            ].map(({ icon: Icon, title, onClick }) => (
              <button
                key={title}
                type="button"
                onClick={onClick}
                className={cn(
                  'p-2 rounded-md text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-700 hover:text-primary-600 dark:hover:text-primary-400',
                  animationCombos.buttonHover,
                )}
                title={title}
              >
                <Icon className="w-4 h-4" />
              </button>
            ))}
          </div>
        )}

        {/* 操作提示 */}
        {isReady && !pendingEdge && !selectedRelation && (
          <div className="absolute bottom-3 left-1/2 -translate-x-1/2 px-3 py-1.5 bg-surface-900/70 dark:bg-surface-100/10 text-white text-xs rounded-full backdrop-blur-sm">
            {connectMode
              ? (connectSourceId
                  ? `已选择源节点「${getNodeName(connectSourceId)}」，请点击目标节点`
                  : '连线模式：依次点击源节点和目标节点创建关系')
              : '点击工具栏「连线」按钮进入连线模式 · 点击连线查看详情'}
          </div>
        )}

        {/* 连线创建面板 */}
        {pendingEdge && (
          <RelationCreatePanel
            sourceName={pendingEdge.sourceName}
            targetName={pendingEdge.targetName}
            saving={saving}
            onConfirm={handleConfirmCreate}
            onCancel={exitConnectMode}
          />
        )}

        {/* 边详情面板 */}
        {selectedRelation && (
          <EdgeDetailPanel
            relation={selectedRelation}
            saving={saving}
            onEdit={(rel) => { setSelectedRelation(null); onEditRelation(rel); }}
            onDelete={(id, name) => { setSelectedRelation(null); onDeleteRelation(id, name); }}
            onClose={() => setSelectedRelation(null)}
          />
        )}

        {/* 图例 */}
        {isReady && <GraphLegend />}
      </div>
    </div>
  );
};

/** 图例组件 */
const GraphLegend: React.FC = () => {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="absolute bottom-3 right-3 z-10">
      {expanded ? (
        <div className="bg-white/95 dark:bg-surface-800/95 backdrop-blur-sm rounded-lg shadow-lg border border-surface-200 dark:border-surface-700 p-3 w-48 animate-fade-in">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-semibold text-surface-700 dark:text-surface-300">图例</span>
            <button type="button" onClick={() => setExpanded(false)} className="text-xs text-surface-400 hover:text-surface-600">收起</button>
          </div>
          <div className="space-y-1.5 mb-2">
            <span className="text-[10px] text-surface-500 font-medium">节点类型</span>
            {ADMIN_NODE_LEGEND.map((item) => (
              <div key={item.type} className="flex items-center gap-1.5">
                <div className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: item.color }} />
                <span className="text-[11px] text-surface-600 dark:text-surface-400">{item.label}</span>
              </div>
            ))}
          </div>
          <div className="space-y-1.5">
            <span className="text-[10px] text-surface-500 font-medium">关系类型</span>
            {ADMIN_EDGE_LEGEND.map((item) => (
              <div key={item.type} className="flex items-center gap-1.5">
                <div className="w-5 h-0 shrink-0" style={{ borderTop: `2px ${item.lineDash ? 'dashed' : 'solid'} ${item.stroke}` }} />
                <span className="text-[11px] text-surface-600 dark:text-surface-400">{item.label}</span>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="px-2.5 py-1 bg-white/90 dark:bg-surface-800/90 backdrop-blur-sm rounded-lg shadow-lg border border-surface-200 dark:border-surface-700 text-xs text-surface-600 dark:text-surface-400 hover:text-primary-600"
        >
          图例
        </button>
      )}
    </div>
  );
};
