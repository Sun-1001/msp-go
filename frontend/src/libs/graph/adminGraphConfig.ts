import type { SimpleNode, KnowledgeRelationAdmin } from '@/modules/admin/api';

/**
 * 管理端知识图谱配置
 *
 * 扩展学生端配置，支持6种节点类型和5种关系类型的可视化编辑
 */

// ========== 节点类型颜色映射 ==========

const ADMIN_NODE_COLORS: Record<string, string> = {
  concept: '#0ea5e9',       // 蓝色 - 概念
  theorem: '#8b5cf6',       // 紫色 - 定理
  method: '#10b981',        // 绿色 - 方法
  problem: '#f59e0b',       // 橙色 - 习题
  misconception: '#ef4444', // 红色 - 迷思
  resource: '#6b7280',      // 灰色 - 资源
};

const ADMIN_NODE_TYPE_LABELS: Record<string, string> = {
  concept: '概念',
  theorem: '定理',
  method: '方法',
  problem: '习题',
  misconception: '迷思',
  resource: '资源',
};

// ========== 关系类型边样式映射 ==========

const ADMIN_EDGE_STYLES: Record<string, { stroke: string; lineDash?: number[]; label: string }> = {
  has_prerequisite: { stroke: '#0ea5e9', label: '先修' },
  is_a_special_case_of: { stroke: '#8b5cf6', lineDash: [6, 4], label: '特例' },
  used_in: { stroke: '#10b981', label: '应用' },
  prone_to_error: { stroke: '#ef4444', lineDash: [3, 3], label: '易错' },
  related_to: { stroke: '#94a3b8', lineDash: [2, 2], label: '关联' },
};

// ========== 工具函数 ==========

export const getAdminNodeColor = (nodeType: string): string =>
  ADMIN_NODE_COLORS[nodeType] || ADMIN_NODE_COLORS.concept;

export const getAdminNodeTypeLabel = (nodeType: string): string =>
  ADMIN_NODE_TYPE_LABELS[nodeType] || nodeType;

export const getAdminEdgeStyle = (relationType: string) =>
  ADMIN_EDGE_STYLES[relationType] || ADMIN_EDGE_STYLES.related_to;

// ========== 数据转换 ==========

/**
 * 将管理端数据转换为 G6 格式
 *
 * 优先使用 node.node_type，nodeTypeMap 作为兜底来源
 */
export const transformAdminGraphData = (
  nodes: SimpleNode[],
  relations: KnowledgeRelationAdmin[],
  nodeTypeMap: Map<string, string>,
) => {
  const g6Nodes = nodes.map((node) => {
    const nodeType = node.node_type || nodeTypeMap.get(node.id) || 'concept';
    const color = getAdminNodeColor(nodeType);
    return {
      id: node.id,
      data: {
        label: node.name,
        nodeType,
        chapter: node.chapter,
        color,
      } as { label: string; nodeType: string; chapter: string | null; color: string },
    };
  });

  const nodeIdSet = new Set(nodes.map((n) => n.id));
  const g6Edges = relations
    .filter((rel) => nodeIdSet.has(rel.source_id) && nodeIdSet.has(rel.target_id))
    .map((rel) => {
      const style = getAdminEdgeStyle(rel.relation_type);
      return {
        id: rel.id,
        source: rel.source_id,
        target: rel.target_id,
        data: {
          relationType: rel.relation_type,
          weight: rel.weight,
          description: rel.description,
          sourceName: rel.source_name,
          targetName: rel.target_name,
          edgeStyle: style,
        },
      };
    });

  return { nodes: g6Nodes, edges: g6Edges };
};

// ========== G6 配置 ==========

export const getAdminNodeConfig = () => ({
  type: 'circle' as const,
  style: {
    size: 36,
    fill: (d: { data?: { color?: string } }) => d.data?.color || '#0ea5e9',
    stroke: '#fff',
    lineWidth: 2,
    shadowColor: 'rgba(0, 0, 0, 0.08)',
    shadowBlur: 8,
    cursor: 'pointer' as const,
    labelText: (d: { data?: { label?: string } }) => d.data?.label || '',
    labelFill: () => {
      if (typeof document !== 'undefined') {
        return document.documentElement.classList.contains('dark') ? '#e5e7eb' : '#334155';
      }
      return '#334155';
    },
    labelFontSize: 11,
    labelFontWeight: 500,
    labelPlacement: 'bottom' as const,
    labelOffsetY: 6,
    // 端口配置 - 用于连线
    ports: [
      { key: 'top', placement: [0.5, 0] as [number, number] },
      { key: 'right', placement: [1, 0.5] as [number, number] },
      { key: 'bottom', placement: [0.5, 1] as [number, number] },
      { key: 'left', placement: [0, 0.5] as [number, number] },
    ],
  },
  state: {
    hover: {
      lineWidth: 3,
      shadowBlur: 16,
      shadowColor: 'rgba(14, 165, 233, 0.3)',
    },
    selected: {
      stroke: '#0ea5e9',
      lineWidth: 4,
      shadowBlur: 20,
      shadowColor: 'rgba(14, 165, 233, 0.4)',
    },
  },
});

export const getAdminEdgeConfig = () => ({
  type: 'line' as const,
  style: {
    stroke: (d: { data?: { edgeStyle?: { stroke?: string } } }) =>
      d.data?.edgeStyle?.stroke || '#94a3b8',
    lineWidth: 1.5,
    lineDash: (d: { data?: { edgeStyle?: { lineDash?: number[] } } }) =>
      d.data?.edgeStyle?.lineDash,
    endArrow: true,
    endArrowSize: 6,
    cursor: 'pointer' as const,
    labelText: (d: { data?: { edgeStyle?: { label?: string } } }) =>
      d.data?.edgeStyle?.label || '',
    labelFontSize: 10,
    labelFill: () => {
      if (typeof document !== 'undefined') {
        return document.documentElement.classList.contains('dark') ? '#9ca3af' : '#64748b';
      }
      return '#64748b';
    },
    labelBackground: true,
    labelBackgroundFill: () => {
      if (typeof document !== 'undefined') {
        return document.documentElement.classList.contains('dark') ? '#1e293b' : '#ffffff';
      }
      return '#ffffff';
    },
    labelBackgroundRadius: 4,
    labelPadding: [2, 6, 2, 6],
  },
  state: {
    hover: {
      lineWidth: 2.5,
      stroke: '#0ea5e9',
    },
    selected: {
      lineWidth: 3,
      stroke: '#0ea5e9',
    },
  },
});

export const getAdminLayoutConfig = () => ({
  type: 'dagre' as const,
  rankdir: 'TB' as const,
  nodesep: 50,
  ranksep: 70,
  align: 'UL' as const,
});

export const getAdminBehaviorsConfig = () => [
  'drag-canvas',
  'zoom-canvas',
  'drag-element',
  {
    type: 'hover-activate',
    degree: 1,
  },
];

// ========== 图例数据 ==========

export const ADMIN_NODE_LEGEND = Object.entries(ADMIN_NODE_COLORS).map(([type, color]) => ({
  type,
  label: ADMIN_NODE_TYPE_LABELS[type] || type,
  color,
}));

export const ADMIN_EDGE_LEGEND = Object.entries(ADMIN_EDGE_STYLES).map(([type, style]) => ({
  type,
  label: style.label,
  stroke: style.stroke,
  lineDash: style.lineDash,
}));
