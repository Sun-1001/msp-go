import type { Graph } from '@antv/g6';
import type { KnowledgeNode, KnowledgeEdge } from '@/modules/knowledge/types/knowledge';

/**
 * 根据掌握度获取颜色
 */
export const getMasteryColor = (mastery: number): string => {
  if (mastery <= 0) return '#64748b'; // neutral / not started
  if (mastery >= 0.8) return '#16a34a'; // mastered
  if (mastery >= 0.4) return '#d97706'; // in progress
  return '#f97360'; // weak
};

/**
 * 根据节点类型获取形状和颜色
 */
export const getNodeStyle = (type: string, mastery: number) => {
  const baseColor = getMasteryColor(mastery);
  const typeConfig = {
    concept: { shape: 'circle', icon: '○' },
    theorem: { shape: 'diamond', icon: '◇' },
    method: { shape: 'rect', icon: '□' },
  };
  return {
    ...typeConfig[type as keyof typeof typeConfig] || typeConfig.concept,
    color: baseColor,
  };
};

/**
 * 根据关系类型获取边样式
 */
export const getEdgeStyle = (relation: string) => {
  const relationConfig = {
    prerequisite: { stroke: '#94a3b8', lineDash: [5, 5], label: '先修' },
    used_in: { stroke: '#0ea5e9', lineDash: undefined, label: '应用' },
    related: { stroke: '#a78bfa', lineDash: [2, 2], label: '相关' },
  };
  return relationConfig[relation as keyof typeof relationConfig] || relationConfig.related;
};

/**
 * 转换数据为 G6 格式
 */
export const transformGraphData = (nodes: KnowledgeNode[], edges: KnowledgeEdge[]) => {
  const g6Nodes = nodes.map((node) => {
    const style = getNodeStyle(node.type, node.mastery);
    return {
      id: node.id,
      data: {
        ...node,
        nodeStyle: style,
      },
    };
  });

  const g6Edges = edges.map((edge, index) => {
    const style = getEdgeStyle(edge.relation);
    return {
      id: `edge-${index}`,
      source: edge.source,
      target: edge.target,
      data: {
        ...edge,
        edgeStyle: style,
      },
    };
  });

  return { nodes: g6Nodes, edges: g6Edges };
};

/**
 * 获取节点配置
 */
export const getNodeConfig = (size: number, fontSize: number, labelOffsetY: number) => ({
  type: 'circle' as const,
  style: {
    size,
    fill: (d: { data?: { nodeStyle?: { color?: string } } }) => d.data?.nodeStyle?.color || '#0ea5e9',
    stroke: '#fff',
    lineWidth: 2,
    shadowColor: 'rgba(0, 0, 0, 0.1)',
    shadowBlur: 10,
    cursor: 'pointer' as const,
    labelText: (d: { data?: { label?: string } }) => d.data?.label || '',
    // 根据当前是否处于 dark 模式动态选择标签颜色
    labelFill: '#e5e7eb',
    labelFontSize: fontSize,
    labelFontWeight: 500,
    labelPlacement: 'bottom' as const,
    labelOffsetY,
  },
  state: {
    hover: {
      fill: (d: { data?: { nodeStyle?: { color?: string } } }) => {
        const baseColor = d.data?.nodeStyle?.color || '#0ea5e9';
        return baseColor;
      },
      lineWidth: 3,
      shadowBlur: 20,
      shadowColor: 'rgba(14, 165, 233, 0.4)',
    },
    selected: {
      size: size * 1.2,
      stroke: '#f8fafc',
      lineWidth: 4,
      shadowBlur: 25,
      shadowColor: 'rgba(255, 255, 255, 0.5)',
    },
    active: {
      lineWidth: 3,
      shadowBlur: 18,
      shadowColor: 'rgba(245, 158, 11, 0.35)',
    },
    inactive: {
      opacity: 0.2,
      labelOpacity: 0.25,
    },
  },
});

/**
 * 获取边配置
 */
export const getEdgeConfig = (lineWidth: number, arrowSize: number) => ({
  type: 'line' as const,
  style: {
    stroke: (d: { data?: { edgeStyle?: { stroke?: string } } }) => d.data?.edgeStyle?.stroke || '#94a3b8',
    lineWidth,
    lineDash: (d: { data?: { edgeStyle?: { lineDash?: number[] } } }) => d.data?.edgeStyle?.lineDash,
    endArrow: true,
    endArrowSize: arrowSize,
    cursor: 'pointer' as const,
  },
  state: {
    hover: {
      lineWidth: lineWidth + 1,
      stroke: '#0ea5e9',
    },
  },
});

/**
 * 获取布局配置
 */
export const getLayoutConfig = (nodesep: number, ranksep: number) => ({
  type: 'dagre' as const,
  rankdir: 'TB' as const,
  nodesep,
  ranksep,
  align: 'UL' as const,
});

/**
 * 获取行为配置
 */
export const getBehaviorsConfig = () => [
  'drag-canvas',
  'zoom-canvas',
  'drag-element',
  {
    type: 'hover-activate',
    degree: 1,
  },
];

/**
 * 图谱缩放工具函数
 */
export const graphZoomIn = (graph: Graph | null) => {
  if (graph) {
    const currentZoom = graph.getZoom();
    graph.zoomTo(currentZoom * 1.2);
  }
};

export const graphZoomOut = (graph: Graph | null) => {
  if (graph) {
    const currentZoom = graph.getZoom();
    graph.zoomTo(currentZoom / 1.2);
  }
};

export const graphFitView = (graph: Graph | null) => {
  if (graph) {
    graph.fitView();
  }
};
