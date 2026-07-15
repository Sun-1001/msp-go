import type { ReactNode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Question } from '@/modules/exercise/services/exerciseService';

const mocks = vi.hoisted(() => ({
  hookCall: 0,
  classVM: {
    currentQuestion: null as Question | null,
    isLoading: false,
    isGenerating: false,
    isSubmitting: false,
    submitPhase: 'idle',
    submitResult: null,
    solution: null,
    isLoadingSolution: false,
    solutionError: null,
    error: null as string | null,
    errorType: null as string | null,
    errorSource: null as 'load' | 'generation' | 'submission' | null,
    loadNextQuestion: vi.fn().mockResolvedValue(undefined),
    generateQuestion: vi.fn().mockResolvedValue(undefined),
    submitAnswer: vi.fn().mockResolvedValue(undefined),
    loadSolution: vi.fn().mockResolvedValue(undefined),
  },
  aiVM: {
    currentQuestion: null as Question | null,
    isLoading: false,
    isGenerating: false,
    isSubmitting: false,
    submitPhase: 'idle',
    submitResult: null,
    solution: null,
    isLoadingSolution: false,
    solutionError: null,
    error: null as string | null,
    errorType: null as string | null,
    errorSource: null as 'load' | 'generation' | 'submission' | null,
    loadNextQuestion: vi.fn().mockResolvedValue(undefined),
    generateQuestion: vi.fn().mockResolvedValue(undefined),
    submitAnswer: vi.fn().mockResolvedValue(undefined),
    loadSolution: vi.fn().mockResolvedValue(undefined),
  },
  getKnowledgeGraph: vi.fn(),
}));

vi.mock('@/components/layout/MainLayout', () => ({
  MainLayout: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock('@/modules/knowledge/services/knowledgeService', () => ({
  knowledgeService: { getKnowledgeGraph: mocks.getKnowledgeGraph },
}));

vi.mock('@/modules/exercise', () => ({
  useExerciseViewModel: () => {
    const value = mocks.hookCall % 2 === 0 ? mocks.classVM : mocks.aiVM;
    mocks.hookCall += 1;
    return value;
  },
  ExercisePanel: ({
    currentQuestion,
    submitPhase,
    error,
  }: {
    currentQuestion: Question | null;
    submitPhase: string;
    error: string | null;
  }) => (
    <div
      data-testid={`exercise-panel-${currentQuestion?.id ?? 'empty'}`}
      data-submit-phase={submitPhase}
      data-error={error ?? ''}
    >
      {currentQuestion?.title || 'EMPTY_EXERCISE'}
    </div>
  ),
  AIPracticeConfigurator: ({
    error,
    isSubmitting,
  }: {
    error: string | null;
    isSubmitting: boolean;
  }) => (
    <div
      data-testid="ai-configurator"
      data-error={error ?? ''}
      data-is-submitting={String(isSubmitting)}
    >
      AI_CONFIGURATOR
      {error ? <span>{error}</span> : null}
    </div>
  ),
}));

import { ExercisePage } from './ExercisePage';

const classQuestion: Question = {
  id: 'class-1',
  title: '班级极限题',
  content: '班级题干',
  difficulty: 0.5,
  type: 'multiple_choice',
  source: 'class',
  knowledgePoints: ['limit'],
  knowledgePointNames: ['函数极限'],
  hintsAvailable: false,
  estimatedTimeSeconds: 120,
  options: ['A', 'B', 'C', 'D'],
};

const aiQuestion: Question = {
  ...classQuestion,
  id: 'ai-1',
  title: '自主极限题',
  content: 'AI 题干',
  source: 'ai_generated',
};

const LaunchDestination = () => {
  const location = useLocation();
  return <pre>{JSON.stringify(location.state)}</pre>;
};

const renderPage = () => render(
  <MemoryRouter initialEntries={['/exercise']}>
    <Routes>
      <Route path="/exercise" element={<ExercisePage />} />
      <Route path="/session/new" element={<LaunchDestination />} />
    </Routes>
  </MemoryRouter>,
);

describe('ExercisePage practice sources', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.hookCall = 0;
    mocks.classVM.currentQuestion = classQuestion;
    mocks.classVM.isSubmitting = false;
    mocks.classVM.submitPhase = 'idle';
    mocks.classVM.error = null;
    mocks.classVM.errorType = null;
    mocks.classVM.errorSource = null;
    mocks.aiVM.currentQuestion = aiQuestion;
    mocks.aiVM.isSubmitting = false;
    mocks.aiVM.submitPhase = 'idle';
    mocks.aiVM.error = null;
    mocks.aiVM.errorType = null;
    mocks.aiVM.errorSource = null;
    mocks.getKnowledgeGraph.mockResolvedValue({ nodes: [], edges: [], statistics: null });
  });

  it('shows class tutor role and launches a class-specific session', async () => {
    renderPage();

    expect(screen.getByText('班级题辅导')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '询问 AI 导师' }));

    await waitFor(() => expect(screen.getByText(/班级题目辅导/)).toBeInTheDocument());
    expect(screen.getByText(/老师发布的班级题目/)).toBeInTheDocument();
  });

  it('switches to the AI coach and keeps its launch context separate', async () => {
    renderPage();
    fireEvent.click(screen.getByRole('tab', { name: /AI 自主练习/ }));

    expect(screen.getByText('AI 练习教练')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '询问 AI 导师' }));

    await waitFor(() => expect(screen.getByText(/AI 自主练习辅导/)).toBeInTheDocument());
    expect(screen.getByText(/题干或选项有歧义/)).toBeInTheDocument();
  });

  it('keeps both practice panels mounted after switching modes', async () => {
    renderPage();
    fireEvent.click(screen.getByRole('tab', { name: /AI 自主练习/ }));

    await waitFor(() => expect(screen.getByText(
      '暂无可用知识点，请联系管理员配置',
    )).toBeInTheDocument());
    expect(screen.getByText('自主极限题')).toBeVisible();
    expect(screen.getByText('班级极限题')).not.toBeVisible();

    fireEvent.click(screen.getByRole('tab', { name: /班级题目/ }));

    expect(screen.getByText('班级极限题')).toBeVisible();
    expect(screen.getByText('自主极限题')).not.toBeVisible();
  });

  it('passes each submission phase to its matching exercise panel', async () => {
    mocks.classVM.submitPhase = 'uploading';
    mocks.aiVM.submitPhase = 'recognizing';

    renderPage();

    expect(screen.getByTestId('exercise-panel-class-1')).toHaveAttribute(
      'data-submit-phase',
      'uploading',
    );
    fireEvent.click(screen.getByRole('tab', { name: /AI 自主练习/ }));
    await waitFor(() => expect(screen.getByTestId('exercise-panel-ai-1')).toHaveAttribute(
      'data-submit-phase',
      'recognizing',
    ));
  });

  it('routes submission state and errors only to the answer workflow', async () => {
    mocks.aiVM.isSubmitting = true;
    mocks.aiVM.error = '未能从图片中识别出有效答案';
    mocks.aiVM.errorType = 'answer_unreadable';
    mocks.aiVM.errorSource = 'submission';

    renderPage();
    fireEvent.click(screen.getByRole('tab', { name: /AI 自主练习/ }));

    await waitFor(() => {
      const configurator = screen.getByTestId('ai-configurator');
      expect(configurator).toHaveAttribute('data-is-submitting', 'true');
      expect(configurator).toHaveAttribute('data-error', '');
      expect(screen.getByTestId('exercise-panel-ai-1')).toHaveAttribute(
        'data-error',
        '未能从图片中识别出有效答案',
      );
    });
  });

  it('explains when no knowledge points are configured', async () => {
    renderPage();
    fireEvent.click(screen.getByRole('tab', { name: /AI 自主练习/ }));

    await waitFor(() => expect(screen.getByText(
      '暂无可用知识点，请联系管理员配置',
    )).toBeInTheDocument());
  });
});
