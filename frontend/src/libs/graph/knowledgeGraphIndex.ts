import type {
  KnowledgeEdge,
  KnowledgeNode,
} from '@/modules/knowledge/types/knowledge';

export interface KnowledgeGraphIndex {
  nodesById: ReadonlyMap<string, KnowledgeNode>;
  prerequisiteEdgesByTarget: ReadonlyMap<string, readonly KnowledgeEdge[]>;
  successorEdgesBySource: ReadonlyMap<string, readonly KnowledgeEdge[]>;
}

function appendEdge(
  groups: Map<string, KnowledgeEdge[]>,
  key: string,
  edge: KnowledgeEdge,
): void {
  const group = groups.get(key);
  if (group) {
    group.push(edge);
    return;
  }
  groups.set(key, [edge]);
}

export function buildKnowledgeGraphIndex(
  nodes: readonly KnowledgeNode[],
  edges: readonly KnowledgeEdge[],
): KnowledgeGraphIndex {
  const nodesById = new Map<string, KnowledgeNode>();
  const prerequisiteEdgesByTarget = new Map<string, KnowledgeEdge[]>();
  const successorEdgesBySource = new Map<string, KnowledgeEdge[]>();

  for (const node of nodes) {
    nodesById.set(node.id, node);
  }

  for (const edge of edges) {
    if (edge.relation !== 'prerequisite') {
      continue;
    }
    appendEdge(prerequisiteEdgesByTarget, edge.target, edge);
    appendEdge(successorEdgesBySource, edge.source, edge);
  }

  return {
    nodesById,
    prerequisiteEdgesByTarget,
    successorEdgesBySource,
  };
}
