import { useEffect, useMemo, useState } from 'react';
import type {
  KnowledgeGraphViewMode,
  ResolvedKnowledgeGraphViewMode,
} from '@/modules/knowledge/types/knowledge';

export const KNOWLEDGE_GRAPH_VIEW_STORAGE_KEY = 'knowledge-graph:view-mode:v1';
export const MAX_3D_NODES = 300;
export const MAX_2D_NODES = 800;

interface ResolveKnowledgeGraphModeOptions {
  requestedMode: KnowledgeGraphViewMode;
  nodeCount: number;
  hasWebGL: boolean;
  coarsePointer: boolean;
  reducedMotion: boolean;
  threeDFailed?: boolean;
  twoDFailed?: boolean;
}

export function resolveKnowledgeGraphMode({
  requestedMode,
  nodeCount,
  hasWebGL,
  coarsePointer,
  reducedMotion,
  threeDFailed = false,
  twoDFailed = false,
}: ResolveKnowledgeGraphModeOptions): ResolvedKnowledgeGraphViewMode {
  if (requestedMode === 'list' || nodeCount > MAX_2D_NODES || twoDFailed) {
    return 'list';
  }
  if (requestedMode === '2d') {
    return '2d';
  }
  if (requestedMode === '3d') {
    return hasWebGL && nodeCount <= MAX_3D_NODES && !threeDFailed ? '3d' : '2d';
  }
  if (
    nodeCount <= MAX_3D_NODES
    && hasWebGL
    && !coarsePointer
    && !reducedMotion
    && !threeDFailed
  ) {
    return '3d';
  }
  return '2d';
}

export function detectWebGL(): boolean {
  if (typeof document === 'undefined') return false;
  try {
    const canvas = document.createElement('canvas');
    return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'));
  } catch {
    return false;
  }
}

function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => (
    typeof window !== 'undefined' ? window.matchMedia(query).matches : false
  ));

  useEffect(() => {
    const media = window.matchMedia(query);
    const update = () => setMatches(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, [query]);

  return matches;
}

export function useKnowledgeGraphMode(
  requestedMode: KnowledgeGraphViewMode,
  nodeCount: number,
  failures: { threeD: boolean; twoD: boolean },
): { mode: ResolvedKnowledgeGraphViewMode; reducedMotion: boolean } {
  const coarsePointer = useMediaQuery('(pointer: coarse)');
  const reducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)');
  const hasWebGL = useMemo(() => detectWebGL(), []);

  return {
    mode: resolveKnowledgeGraphMode({
      requestedMode,
      nodeCount,
      hasWebGL,
      coarsePointer,
      reducedMotion,
      threeDFailed: failures.threeD,
      twoDFailed: failures.twoD,
    }),
    reducedMotion,
  };
}

export function readStoredKnowledgeGraphViewMode(): KnowledgeGraphViewMode {
  if (typeof window === 'undefined') return 'auto';
  const stored = window.localStorage.getItem(KNOWLEDGE_GRAPH_VIEW_STORAGE_KEY);
  return stored === '3d' || stored === '2d' || stored === 'list' ? stored : 'auto';
}

export function storeKnowledgeGraphViewMode(mode: KnowledgeGraphViewMode): void {
  if (typeof window === 'undefined') return;
  if (mode === 'auto') {
    window.localStorage.removeItem(KNOWLEDGE_GRAPH_VIEW_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(KNOWLEDGE_GRAPH_VIEW_STORAGE_KEY, mode);
}
