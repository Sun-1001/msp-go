import { describe, expect, it } from 'vitest';
import { buildKnowledgeGraphIndex } from '@/libs/graph/knowledgeGraphIndex';
import type {
  KnowledgeEdge,
  KnowledgeNode,
} from '@/modules/knowledge/types/knowledge';

const nodes: KnowledgeNode[] = [
  { id: 'limits', label: '极限', type: 'concept', mastery: 0.8 },
  { id: 'derivative', label: '导数', type: 'concept', mastery: 0.6 },
  { id: 'integral', label: '积分', type: 'concept', mastery: 0.4 },
];

const edges: KnowledgeEdge[] = [
  { source: 'limits', target: 'derivative', relation: 'prerequisite' },
  { source: 'derivative', target: 'integral', relation: 'prerequisite' },
  { source: 'limits', target: 'integral', relation: 'related' },
];

describe('buildKnowledgeGraphIndex', () => {
  it('indexes nodes and prerequisite edges in one pass', () => {
    const index = buildKnowledgeGraphIndex(nodes, edges);

    expect(index.nodesById.get('derivative')).toEqual(nodes[1]);
    expect(index.prerequisiteEdgesByTarget.get('integral')).toEqual([edges[1]]);
    expect(index.successorEdgesBySource.get('limits')).toEqual([edges[0]]);
  });

  it('does not mix non-prerequisite relations into detail groups', () => {
    const index = buildKnowledgeGraphIndex(nodes, edges);

    expect(index.prerequisiteEdgesByTarget.get('integral')).not.toContain(edges[2]);
    expect(index.successorEdgesBySource.get('limits')).not.toContain(edges[2]);
  });
});
