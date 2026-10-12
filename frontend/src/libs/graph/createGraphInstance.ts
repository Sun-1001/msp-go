import { Graph } from '@antv/g6';
import type { KnowledgeNode, KnowledgeEdge } from '@/modules/knowledge/api';
import {
  transformGraphData,
  getNodeConfig,
  getEdgeConfig,
  getLayoutConfig,
  getBehaviorsConfig,
} from './graphConfig';

/**
 * Graph 实例配置选项
 */
export interface GraphInstanceOptions {
  container: HTMLElement;
  width: number;
  height: number;
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
  padding?: [number, number, number, number];
  nodeSize?: number;
  fontSize?: number;
  labelOffsetY?: number;
  lineWidth?: number;
  arrowSize?: number;
  nodesep?: number;
  ranksep?: number;
  onNodeClick?: (nodeId: string, nodes: KnowledgeNode[]) => void;
  onNodeHover?: (nodeId: string | null, nodes: KnowledgeNode[]) => void;
}

/**
 * 创建 Graph 实例的工厂函数
 *
 * 统一管理 G6 图谱的创建逻辑，避免代码重复
 */
export const createGraphInstance = (options: GraphInstanceOptions): Graph => {
  const {
    container,
    width,
    height,
    nodes,
    edges,
    padding = [40, 40, 40, 40],
    nodeSize = 40,
    fontSize = 12,
    labelOffsetY = 8,
    lineWidth = 1.5,
    arrowSize = 6,
    nodesep = 60,
    ranksep = 80,
    onNodeClick,
    onNodeHover,
  } = options;

  // 转换数据
  const data = transformGraphData(nodes, edges);

  // 创建 Graph 实例
  const graph = new Graph({
    container,
    width,
    height,
    autoFit: 'view',
    padding,
    data,
    node: getNodeConfig(nodeSize, fontSize, labelOffsetY),
    edge: getEdgeConfig(lineWidth, arrowSize),
    layout: getLayoutConfig(nodesep, ranksep),
    behaviors: getBehaviorsConfig(),
    animation: false,
  });

  // 绑定节点点击事件
  if (onNodeClick) {
    graph.on('node:click', (event: unknown) => {
      const evt = event as { target?: { id?: string } };
      const nodeId = evt.target?.id;
      if (nodeId) {
        onNodeClick(nodeId, nodes);
      }
    });
  }

  // 绑定节点悬停事件
  if (onNodeHover) {
    graph.on('node:pointerenter', (event: unknown) => {
      const evt = event as { target?: { id?: string } };
      const nodeId = evt.target?.id;
      if (nodeId) {
        onNodeHover(nodeId, nodes);
      }
    });

    graph.on('node:pointerleave', () => {
      onNodeHover(null, nodes);
    });
  }

  return graph;
};

/**
 * 更新 Graph 数据
 */
export const updateGraphData = (
  graph: Graph,
  nodes: KnowledgeNode[],
  edges: KnowledgeEdge[]
) => {
  const data = transformGraphData(nodes, edges);
  graph.setData(data);
  graph.layout();
  graph.fitView();
};
