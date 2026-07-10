import type { ReactNode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { KnowledgeGraphPage } from '@/pages/student/KnowledgeGraphPage';

const mocks = vi.hoisted(() => ({
  dispatch: vi.fn(),
  fetchKnowledgeGraph: vi.fn((filters: unknown) => ({
    type: 'knowledge/fetch',
    payload: filters,
  })),
  updateFilter: vi.fn((payload: unknown) => ({
    type: 'knowledge/update-filter',
    payload,
  })),
  selectNode: vi.fn((payload: unknown) => ({
    type: 'knowledge/select-node',
    payload,
  })),
  getChapters: vi.fn(),
  knowledgeState: {
    nodes: [],
    edges: [],
    statistics: null,
    filters: {},
    selectedNodeId: null,
    loadingState: 'loading',
    error: null as string | null,
  },
}));

vi.mock('@/store', () => ({
  useAppDispatch: () => mocks.dispatch,
  useAppSelector: (
    selector: (state: { knowledge: typeof mocks.knowledgeState }) => unknown,
  ) => selector({ knowledge: mocks.knowledgeState }),
}));

vi.mock('@/modules/knowledge/store/knowledgeSlice', () => ({
  fetchKnowledgeGraph: mocks.fetchKnowledgeGraph,
  updateFilter: mocks.updateFilter,
  selectNode: mocks.selectNode,
}));

vi.mock('@/modules/knowledge/services/knowledgeService', () => ({
  knowledgeService: {
    getChapters: mocks.getChapters,
  },
}));

vi.mock('@/modules/knowledge', () => ({
  KnowledgeGraph: () => null,
  KnowledgeGraphLegend: () => null,
}));

vi.mock('@/components/layout/MainLayout', () => ({
  MainLayout: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

describe('KnowledgeGraphPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getChapters.mockResolvedValue([]);
    mocks.knowledgeState.filters = {};
    mocks.knowledgeState.loadingState = 'loading';
    mocks.knowledgeState.error = null;
  });

  it('loads graph data only once on initial render', async () => {
    render(<KnowledgeGraphPage />);

    await waitFor(() => {
      expect(mocks.fetchKnowledgeGraph).toHaveBeenCalledTimes(1);
    });
    expect(mocks.fetchKnowledgeGraph).toHaveBeenCalledWith({});
    expect(mocks.getChapters).toHaveBeenCalledTimes(1);
  });

  it('retries with the active filters', async () => {
    const filters = { search: '极限' };
    mocks.knowledgeState.filters = filters;
    mocks.knowledgeState.loadingState = 'error';
    mocks.knowledgeState.error = '加载失败';

    render(<KnowledgeGraphPage />);
    await waitFor(() => {
      expect(mocks.fetchKnowledgeGraph).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole('button', { name: '重试' }));

    expect(mocks.fetchKnowledgeGraph).toHaveBeenLastCalledWith(filters);
    expect(mocks.fetchKnowledgeGraph).toHaveBeenCalledTimes(2);
  });
});
