import type { ComponentProps } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ExercisePanel } from './ExercisePanel';

const question: NonNullable<ComponentProps<typeof ExercisePanel>['currentQuestion']> = {
  id: 'exercise-1',
  title: '基础练习',
  content: '2 + 2',
  difficulty: 0.2,
  type: 'short_answer',
  source: 'class',
  knowledgePoints: ['arithmetic'],
  knowledgePointNames: ['基础运算'],
  hintsAvailable: false,
  estimatedTimeSeconds: 30,
};

const renderPanel = (overrides: Partial<ComponentProps<typeof ExercisePanel>> = {}) => {
  const props: ComponentProps<typeof ExercisePanel> = {
    currentQuestion: question,
    isLoading: false,
    isSubmitting: false,
    submitResult: null,
    error: null,
    errorType: null,
    onNextQuestion: vi.fn(),
    submitAnswer: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };

  return { ...render(<ExercisePanel {...props} />), props };
};

describe('ExercisePanel', () => {
  it('does not load on mount and submits trimmed text without an image control', async () => {
    const onNextQuestion = vi.fn().mockResolvedValue(undefined);
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    const { container } = renderPanel({ onNextQuestion, submitAnswer });

    expect(onNextQuestion).not.toHaveBeenCalled();
    expect(container.querySelector('input[type="file"]')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /拍照|上传手写/ })).not.toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/输入答案/), {
      target: { value: '  x + 1  ' },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith('x + 1'));
  });

  it('renders four accessible radio options and submits the selected option', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    renderPanel({
      currentQuestion: {
        ...question,
        type: 'multiple_choice',
        options: ['1', '2', '4', '8'],
      },
      submitAnswer,
    });

    expect(screen.getByRole('radiogroup', { name: '答案选项' })).toBeInTheDocument();
    const options = screen.getAllByRole('radio');
    expect(options).toHaveLength(4);

    fireEvent.click(options[2]);
    expect(options[2]).toBeChecked();
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith('4'));
  });

  it('falls back to text input for legacy multiple-choice questions without options', () => {
    renderPanel({
      currentQuestion: {
        ...question,
        type: 'multiple_choice',
        options: [],
      },
    });

    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText(/输入答案/)).toBeInTheDocument();
  });

  it('clears the answer when the question id changes', () => {
    const { rerender, props } = renderPanel();
    const answerInput = screen.getByPlaceholderText(/输入答案/);
    fireEvent.change(answerInput, { target: { value: '旧答案' } });
    expect(answerInput).toHaveValue('旧答案');

    rerender(<ExercisePanel {...props} currentQuestion={{ ...question, id: 'exercise-2' }} />);

    expect(screen.getByPlaceholderText(/输入答案/)).toHaveValue('');
  });

  it('delegates the next-question action after feedback', async () => {
    const onNextQuestion = vi.fn().mockResolvedValue(undefined);
    renderPanel({
      onNextQuestion,
      submitResult: {
        isCorrect: true,
        score: 1,
        studentAnswerLatex: '4',
        correctAnswerLatex: '4',
        diagnosis: null,
        feedback: '回答正确',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'advance',
      },
    });

    fireEvent.click(screen.getByRole('button', { name: '下一题' }));
    await waitFor(() => expect(onNextQuestion).toHaveBeenCalledTimes(1));
  });

  it('prevents repeated next-question requests while loading', () => {
    const onNextQuestion = vi.fn();
    renderPanel({
      isLoading: true,
      onNextQuestion,
      submitResult: {
        isCorrect: true,
        score: 1,
        studentAnswerLatex: '4',
        correctAnswerLatex: '4',
        diagnosis: null,
        feedback: '回答正确',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'advance',
      },
    });

    const nextButton = screen.getByRole('button', { name: '下一题' });
    expect(nextButton).toBeDisabled();
    fireEvent.click(nextButton);
    expect(onNextQuestion).not.toHaveBeenCalled();
  });

  it('keeps the retry-next state after a next-question failure', () => {
    renderPanel({
      error: '加载下一题失败，请重试',
      submitResult: {
        isCorrect: true,
        score: 1,
        studentAnswerLatex: '4',
        correctAnswerLatex: '4',
        diagnosis: null,
        feedback: '回答正确',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'advance',
      },
    });

    expect(screen.getByRole('alert')).toHaveTextContent('加载下一题失败，请重试');
    expect(screen.getByRole('button', { name: '下一题' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: '提交答案' })).not.toBeInTheDocument();
  });

  it('uses icons instead of text symbols for feedback and suggestions', () => {
    const { container } = renderPanel({
      submitResult: {
        isCorrect: false,
        score: 0,
        studentAnswerLatex: '3',
        correctAnswerLatex: '',
        diagnosis: {
          errorType: 'calculation',
          errorDescription: '计算错误',
          errorStepIndex: null,
          severity: 'medium',
          suggestion: '重新检查运算顺序',
          relatedConcepts: [],
        },
        feedback: '再检查一下',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'review',
      },
    });

    expect(screen.getByText('回答错误')).toBeInTheDocument();
    expect(screen.getByText('重新检查运算顺序')).toBeInTheDocument();
    expect(container).not.toHaveTextContent('✓');
    expect(container).not.toHaveTextContent('✗');
    expect(container).not.toHaveTextContent('💡');
    expect(container.querySelectorAll('svg')).toHaveLength(2);
  });
});
