import type { ComponentProps } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AIPracticeConfigurator } from './AIPracticeConfigurator';

const knowledgeOptions = [
  { value: 'limit', label: '函数极限' },
  { value: 'integral', label: '定积分' },
];

const renderConfigurator = (
  overrides: Partial<ComponentProps<typeof AIPracticeConfigurator>> = {}
) => {
  const props: ComponentProps<typeof AIPracticeConfigurator> = {
    knowledgeOptions,
    selectedConceptId: 'limit',
    difficulty: 0.5,
    isLoadingKnowledge: false,
    isGenerating: false,
    isSubmitting: false,
    error: null,
    hasQuestion: false,
    onConceptChange: vi.fn(),
    onDifficultyChange: vi.fn(),
    onGenerate: vi.fn(),
    ...overrides,
  };

  return { ...render(<AIPracticeConfigurator {...props} />), props };
};

describe('AIPracticeConfigurator', () => {
  it('changes the knowledge point and difficulty before generating', () => {
    const { props } = renderConfigurator();

    fireEvent.change(screen.getByLabelText('知识点'), {
      target: { value: 'integral' },
    });
    fireEvent.change(screen.getByLabelText('难度'), {
      target: { value: '0.85' },
    });
    fireEvent.click(screen.getByRole('button', { name: '生成题目' }));

    expect(props.onConceptChange).toHaveBeenCalledWith('integral');
    expect(props.onDifficultyChange).toHaveBeenCalledWith(0.85);
    expect(props.onGenerate).toHaveBeenCalledTimes(1);
  });

  it('shows a regenerate action when a question is present', () => {
    renderConfigurator({ hasQuestion: true });

    expect(screen.getByRole('button', { name: '重新生成' })).toBeInTheDocument();
  });

  it('disables regeneration while an answer submission is pending', () => {
    const onGenerate = vi.fn();
    renderConfigurator({ hasQuestion: true, isSubmitting: true, onGenerate });

    const button = screen.getByRole('button', { name: '重新生成' });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onGenerate).not.toHaveBeenCalled();
  });

  it('disables controls while knowledge points load without duplicate button icons', () => {
    renderConfigurator({
      knowledgeOptions: [],
      selectedConceptId: '',
      isLoadingKnowledge: true,
    });

    expect(screen.getByLabelText('知识点')).toBeDisabled();
    const button = screen.getByRole('button', { name: '加载知识点' });
    expect(button).toBeDisabled();
    expect(button.querySelectorAll('svg')).toHaveLength(1);
  });

  it('offers a retry when the initial knowledge-point load fails', () => {
    const onRetryKnowledge = vi.fn();
    renderConfigurator({
      knowledgeOptions: [],
      selectedConceptId: '',
      error: '知识点加载失败',
      onRetryKnowledge,
    });

    expect(screen.getByRole('alert')).toHaveTextContent('知识点加载失败');
    const retryButton = screen.getByRole('button', { name: '重试加载' });
    expect(retryButton).toBeEnabled();
    fireEvent.click(retryButton);
    expect(onRetryKnowledge).toHaveBeenCalledTimes(1);
  });
});
