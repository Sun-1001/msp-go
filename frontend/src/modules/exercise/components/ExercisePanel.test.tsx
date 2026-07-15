import type { ComponentProps } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
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
    submitPhase: 'idle',
    submitResult: null,
    solution: null,
    isLoadingSolution: false,
    solutionError: null,
    error: null,
    errorType: null,
    onNextQuestion: vi.fn(),
    submitAnswer: vi.fn().mockResolvedValue(undefined),
    onLoadSolution: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };

  return { ...render(<ExercisePanel {...props} />), props };
};

describe('ExercisePanel', () => {
  beforeEach(() => {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn((file: File) => `blob:${file.name}`),
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    });
  });

  it('does not load on mount and submits trimmed text without uploading the selected image', async () => {
    const onNextQuestion = vi.fn().mockResolvedValue(undefined);
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    renderPanel({ onNextQuestion, submitAnswer });

    expect(onNextQuestion).not.toHaveBeenCalled();
    const image = new File(['image'], 'answer.png', { type: 'image/png' });
    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [image] },
    });

    fireEvent.change(screen.getByPlaceholderText(/输入答案/), {
      target: { value: '  x + 1  ' },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerText: 'x + 1' }));
  });

  it('submits a pure image answer while keeping its preview mounted', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    renderPanel({ submitAnswer });
    const image = new File(['image'], 'answer.png', { type: 'image/png' });

    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [image] },
    });
    expect(screen.getByRole('img', { name: '答案图片预览：answer.png' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerImage: image }));
    expect(screen.getByRole('img', { name: '答案图片预览：answer.png' })).toBeInTheDocument();
  });

  it('keeps the selected image after failure and disables attachment controls only while busy', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    const { rerender, props } = renderPanel({ submitAnswer });
    const image = new File(['image'], 'retry.png', { type: 'image/png' });
    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [image] },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalledTimes(1));

    rerender(<ExercisePanel {...props} isSubmitting />);
    expect(screen.getByLabelText('选择答案图片')).toBeDisabled();
    expect(screen.getByRole('button', { name: '移除答案图片' })).toBeDisabled();

    rerender(<ExercisePanel {...props} error="图片识别服务暂不可用" />);
    expect(screen.getByRole('img', { name: '答案图片预览：retry.png' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '重试提交' }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalledTimes(2));
    expect(submitAnswer).toHaveBeenLastCalledWith({ answerImage: image });
  });

  it('distinguishes image upload from OCR and grading progress', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    const { rerender, props } = renderPanel({ submitAnswer });
    const image = new File(['image'], 'progress.png', { type: 'image/png' });
    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [image] },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerImage: image }));

    rerender(<ExercisePanel {...props} isSubmitting submitPhase="uploading" />);
    expect(screen.getByRole('button', { name: '上传中' })).toBeDisabled();

    rerender(<ExercisePanel {...props} isSubmitting submitPhase="recognizing" />);
    expect(screen.getByRole('button', { name: '识别中' })).toBeDisabled();
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

    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerText: '4' }));
    expect(screen.queryByLabelText('选择答案图片')).not.toBeInTheDocument();
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
    expect(screen.queryByLabelText('选择答案图片')).not.toBeInTheDocument();
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

  it('presents an unavailable verified solution as a failure instead of empty steps', async () => {
    const onLoadSolution = vi.fn().mockResolvedValue(undefined);
    renderPanel({
      onLoadSolution,
      submitResult: {
        isCorrect: false,
        recorded: true,
        gradingStatus: 'incorrect',
        score: 0,
        studentAnswerLatex: '3',
        correctAnswerLatex: '4',
        diagnosis: null,
        feedback: '回答错误',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'review',
      },
      solution: {
        answer: '4',
        steps: [],
        source: 'unavailable',
        verification: {
          method: 'math_solver',
          reasonCode: 'verification_needed',
          reason: '生成步骤未通过标准答案验证',
          confidence: 0.4,
          degraded: true,
          retryable: true,
          evidence: [{ kind: 'verification', summary: '步骤二无法推出结论' }],
        },
        failure: {
          code: 'verification_failed',
          stage: 'solution_verification',
          message: '暂时无法生成经过验证的题目解析',
          retryable: true,
        },
      },
    });

    expect(screen.queryByText('解题解析')).not.toBeInTheDocument();
    expect(screen.getByText('暂时无法生成经过验证的题目解析')).toBeInTheDocument();
    expect(screen.getByText('步骤二无法推出结论')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '重试解析' }));
    await waitFor(() => expect(onLoadSolution).toHaveBeenCalledTimes(1));
  });

  it('shows the recognized answer after a pure image submission succeeds', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    const { rerender, props } = renderPanel({ submitAnswer });
    const image = new File(['image'], 'fraction.png', { type: 'image/png' });
    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [image] },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerImage: image }));

    rerender(
      <ExercisePanel
        {...props}
        submitResult={{
          isCorrect: true,
          recorded: true,
          gradingStatus: 'correct',
          score: 1,
          studentAnswerLatex: '\\frac{1}{2}',
          correctAnswerLatex: '\\frac{1}{2}',
          diagnosis: null,
          feedback: '回答正确',
          masteryUpdate: null,
          masteryModel: 'dkt-sakt-lite',
          nextRecommendation: 'advance',
        }}
      />
    );

    expect(screen.getByText('图片识别结果：')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '回答正确' })).toBeInTheDocument();
  });

  it('keeps an indeterminate unrecorded result neutral and retryable', () => {
    renderPanel({
      submitResult: {
        isCorrect: false,
        recorded: false,
        gradingStatus: 'indeterminate',
        evaluation: {
          method: 'math_solver',
          reasonCode: 'MATH_SOLVER_UNAVAILABLE',
          reason: '求解服务暂不可用，请稍后重试',
          confidence: 0,
          degraded: true,
          retryable: true,
          evidence: [
            {
              kind: 'comparison_boundary',
              summary: '字符串差异不能证明表达式不等价',
            },
          ],
        },
        score: 0,
        studentAnswerLatex: 'x + 1',
        correctAnswerLatex: '',
        diagnosis: null,
        feedback: '暂时无法可靠判定',
        masteryUpdate: null,
        masteryModel: 'dkt-sakt-lite',
        nextRecommendation: 'retry',
      },
    });

    expect(screen.getByText('暂无法自动判定')).toBeInTheDocument();
    expect(screen.getByText('求解服务暂不可用，请稍后重试')).toBeInTheDocument();
    expect(screen.getByText('判定依据：')).toBeInTheDocument();
    expect(screen.getByText('字符串差异不能证明表达式不等价')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('暂无法自动判定');
    expect(screen.queryByText('回答错误')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '下一题' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重试提交' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/输入答案/)).toBeEnabled();
  });

  it('requires a changed answer before resubmitting a non-retryable result', async () => {
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    const { rerender, props } = renderPanel({ submitAnswer });
    fireEvent.change(screen.getByPlaceholderText(/输入答案/), {
      target: { value: 'x + 1' },
    });
    fireEvent.click(screen.getByRole('button', { name: '提交答案' }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalledWith({ answerText: 'x + 1' }));

    rerender(
      <ExercisePanel
        {...props}
        submitResult={{
          isCorrect: false,
          recorded: false,
          gradingStatus: 'indeterminate',
          evaluation: {
            method: 'deterministic',
            reasonCode: 'AMBIGUOUS_DOMAIN',
            reason: '题目未给出变量定义域，请修改答案并补充必要条件',
            confidence: 0.4,
            degraded: true,
            retryable: false,
            evidence: [],
          },
          score: 0,
          studentAnswerLatex: 'x + 1',
          correctAnswerLatex: '',
          diagnosis: null,
          feedback: '暂时无法可靠判定',
          masteryUpdate: null,
          masteryModel: 'dkt-sakt-lite',
          nextRecommendation: 'retry',
        }}
      />
    );

    expect(screen.getByRole('button', { name: '请修改答案后提交' })).toBeDisabled();
    fireEvent.change(screen.getByPlaceholderText(/输入答案/), {
      target: { value: 'x + 1, x > 0' },
    });
    const changedAnswerButton = screen.getByRole('button', { name: '提交修改后的答案' });
    expect(changedAnswerButton).toBeEnabled();
    fireEvent.click(changedAnswerButton);
    await waitFor(() => expect(submitAnswer).toHaveBeenLastCalledWith({
      answerText: 'x + 1, x > 0',
    }));
  });

  it('reloads instead of resubmitting when the exercise changed', async () => {
    const onNextQuestion = vi.fn().mockResolvedValue(undefined);
    const submitAnswer = vi.fn().mockResolvedValue(undefined);
    renderPanel({
      error: '题目已更新，请重新加载后提交',
      errorType: 'exercise_changed',
      onNextQuestion,
      submitAnswer,
    });

    fireEvent.click(screen.getByRole('button', { name: '重新加载题目' }));

    await waitFor(() => expect(onNextQuestion).toHaveBeenCalledTimes(1));
    expect(submitAnswer).not.toHaveBeenCalled();
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
    expect(container.querySelectorAll('svg').length).toBeGreaterThanOrEqual(2);
  });
});
