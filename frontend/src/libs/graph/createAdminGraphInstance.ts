import { Graph } from '@antv/g6';
import type { SimpleNode, KnowledgeRelationAdmin } from '@/modules/admin/api';
import {
  transformAdminGraphData,
  getAdminNodeConfig,
  getAdminEdgeConfig,
  getAdminLayoutConfig,
  getAdminBehaviorsConfig,
} from './adminGraphConfig';

/**
 * 管理端 Graph 实例配置
 */
export interface AdminGraphInstanceOptions {
  container: HTMLElement;
  width: number;
  height: number;
  nodes: SimpleNode[];
  relations: KnowledgeRelationAdmin[];
  nodeTypeMap: Map<string, string>;
  onNodeClick?: (nodeId: string) => void;
  onEdgeClick?: (edgeId: string) => void;
}

/** 拖拽节点行为 key */
const DRAG_ELEMENT_KEY = 'drag-element-behavior';

const setNodeCursor = (graph: Graph, cursor: 'pointer' | 'crosshair') => {
  const nodeOptions = graph.getOptions().node;
  if (!nodeOptions || typeof nodeOptions !== 'object') return;

  const rawNode = nodeOptions as Record<string, unknown>;
  const rawStyle = (rawNode.style ?? {}) as Record<string, unknown>;

  graph.setNode({
    ...rawNode,
    style: {
      ...rawStyle,
      cursor,
    },
  } as Parameters<Graph['setNode']>[0]);
};
/**
 * 创建管理端 Graph 实例
 *
 * 左键拖拽移动节点，通过切换连线模式来创建关系
 * 连线模式下：点击源节点 → 点击目标节点 = 创建关系
 */
export const createAdminGraphInstance = (options: AdminGraphInstanceOptions): Graph => {
  const {
    container, width, height,
    nodes, relations, nodeTypeMap,
    onNodeClick, onEdgeClick,
  } = options;

  const data = transformAdminGraphData(nodes, relations, nodeTypeMap);
  const behaviors = getAdminBehaviorsConfig().map((behavior) =>
    behavior === 'drag-element'
      ? {
          key: DRAG_ELEMENT_KEY,
          type: 'drag-element' as const,
          enable: true,
        }
      : behavior,
  );
  const graph = new Graph({
    container, width, height,
    autoFit: 'view',
    padding: [40, 40, 40, 40],
    data,
    node: getAdminNodeConfig(),
    edge: getAdminEdgeConfig(),
    layout: getAdminLayoutConfig(),
    behaviors,
    animation: false,
  });

  // 节点点击事件
  if (onNodeClick) {
    graph.on('node:click', (event: unknown) => {
      const evt = event as { target?: { id?: string } };
      if (evt.target?.id) onNodeClick(evt.target.id);
    });
  }

  // 边点击事件
  if (onEdgeClick) {
    graph.on('edge:click', (event: unknown) => {
      const evt = event as { target?: { id?: string } };
      if (evt.target?.id) onEdgeClick(evt.target.id);
    });
  }

  return graph;
};

/**
 * 启用连线模式
 *
 * 关闭拖拽并统一光标为十字，避免悬停节点时被 drag-element 改写
 */
export const enableCreateEdgeMode = (graph: Graph) => {
  graph.updateBehavior({
    key: DRAG_ELEMENT_KEY,
    enable: false,
    cursor: {
      default: 'crosshair',
      grab: 'crosshair',
      grabbing: 'crosshair',
    },
  });
  setNodeCursor(graph, 'crosshair');
  graph.getCanvas().setCursor('crosshair');
};

/**
 * 禁用连线模式
 */
export const disableCreateEdgeMode = (graph: Graph) => {
  try {
    graph.updateBehavior({
      key: DRAG_ELEMENT_KEY,
      enable: true,
      cursor: {
        default: 'default',
        grab: 'grab',
        grabbing: 'grabbing',
      },
    });
    setNodeCursor(graph, 'pointer');
    graph.getCanvas().setCursor('default');
  } catch {
    // 行为可能不存在，忽略
  }
};

/**
 * 更新管理端 Graph 数据
 */
export const updateAdminGraphData = (
  graph: Graph,
  nodes: SimpleNode[],
  relations: KnowledgeRelationAdmin[],
  nodeTypeMap: Map<string, string>,
) => {
  const data = transformAdminGraphData(nodes, relations, nodeTypeMap);
  graph.setData(data);
  graph.layout();
  graph.fitView();
};
