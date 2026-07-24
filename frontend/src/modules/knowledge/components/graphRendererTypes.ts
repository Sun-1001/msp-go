import type { KnowledgeEdge, KnowledgeNode } from '@/modules/knowledge/types/knowledge';

export interface GraphRendererHandle {
  zoomIn: () => void;
  zoomOut: () => void;
  fitView: () => void;
}

export interface GraphRendererProps {
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
  selectedNodeId: string | null;
  targetNodeId: string | null;
  highlightedNodeIds: ReadonlySet<string>;
  highlightedEdgeKeys: ReadonlySet<string>;
  reducedMotion: boolean;
  onNodeClick?: (node: KnowledgeNode) => void;
  onNodeHover?: (node: KnowledgeNode | null) => void;
  onRendererError?: (error: Error) => void;
}

export function knowledgeEdgeKey(edge: Pick<KnowledgeEdge, 'source' | 'target' | 'relation'>): string {
  return `${edge.source}::${edge.target}::${edge.relation}`;
}
