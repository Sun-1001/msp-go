import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useExerciseViewModel } from '@/modules/exercise/hooks/exerciseViewModel';

const mocks = vi.hoisted(() => ({
  fetchNextQuestion: vi.fn(),
  generateQuestion: vi.fn(),
  submitAnswer: vi.fn(),
}));

vi.mock('@/modules/exercise/services/exerciseService', () => ({
  exerciseService: {
    fetchNextQuestion: mocks.fetchNextQuestion,
    generateQuestion: mocks.generateQuestion,
    submitAnswer: mocks.submitAnswer,
  },
}));

const question = {
  id: 'exercise-1',
  title: '极限',
  content: 'lim x',
  difficulty: 0.4,
  type: 'short_answer' as const,
  source: 'class' as const,
  knowledgePoints: ['limit'],
  knowledgePointNames: ['函数极限'],
  hintsAvailable: false,
  estimatedTimeSeconds: 60,
};

const generatedQuestion = {
  ...question,
  id: 'generated-1',
  source: 'ai_generated' as const,
  difficulty: 0.5,
};

const axiosError = (status?: number) => ({
  isAxiosError: true,
  request: {},
  response: status === undefined ? undefined : { status, data: { detail: 'service error' } },
});

describe('useExerciseViewModel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchNextQuestion.mockResolvedValue(question);
    mocks.generateQuestion.mockResolvedValue(generatedQuestion);
    mocks.submitAnswer.mockResolvedValue({
      isCorrect: true,
      score: 1,
      studentAnswerLatex: '42',
      correctAnswerLatex: '42',
      diagnosis: null,
      feedback: '正确',
      masteryUpdate: {},
      masteryModel: 'dkt-sakt-lite',
      nextRecommendation: 'advance',
    });
  });

  it('submits text-only answer payloads', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    await act(async () => {
      await result.current.submitAnswer('42');
    });

    expect(mocks.submitAnswer).toHaveBeenCalledWith(
      expect.objectContaining({
        exerciseId: 'exercise-1',
        answerText: '42',
      })
    );
    expect(mocks.submitAnswer.mock.calls[0][0]).not.toHaveProperty('answerImageUrl');
  });

  it('rejects blank text without calling the service', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    await act(async () => {
      await result.current.submitAnswer('   ');
    });

    expect(mocks.submitAnswer).not.toHaveBeenCalled();
    expect(result.current.error).toBe('请输入答案');
  });

  it('keeps submitted feedback when loading the next class question fails', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer('42');
    });
    expect(result.current.submitResult).not.toBeNull();

    mocks.fetchNextQuestion.mockRejectedValueOnce(new Error('network unavailable'));
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    expect(result.current.currentQuestion).toEqual(question);
    expect(result.current.submitResult).not.toBeNull();
    expect(result.current.error).toBeTruthy();
  });

  it('generates a question once for concurrent calls and resets feedback', async () => {
    let resolveGeneration: ((value: typeof generatedQuestion) => void) | undefined;
    mocks.generateQuestion.mockImplementation(
      () =>
        new Promise<typeof generatedQuestion>((resolve) => {
          resolveGeneration = resolve;
        })
    );
    const { result } = renderHook(() => useExerciseViewModel());

    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer('42');
    });
    expect(result.current.submitResult).not.toBeNull();

    let firstRequest: Promise<void> | undefined;
    act(() => {
      firstRequest = result.current.generateQuestion('limit', 0.5);
      void result.current.generateQuestion('limit', 0.5);
    });

    expect(result.current.isGenerating).toBe(true);
    expect(mocks.generateQuestion).toHaveBeenCalledTimes(1);
    expect(mocks.generateQuestion).toHaveBeenCalledWith({
      conceptId: 'limit',
      difficulty: 0.5,
    });

    await act(async () => {
      resolveGeneration?.(generatedQuestion);
      await firstRequest;
    });

    expect(result.current.currentQuestion).toEqual(generatedQuestion);
    expect(result.current.submitResult).toBeNull();
    expect(result.current.isGenerating).toBe(false);
  });

  it('starts answer timing again after a generated question succeeds', async () => {
    let now = 1_000;
    const nowSpy = vi.spyOn(Date, 'now').mockImplementation(() => now);
    const { result } = renderHook(() => useExerciseViewModel());

    await act(async () => {
      await result.current.loadNextQuestion();
    });
    now = 4_000;
    await act(async () => {
      await result.current.generateQuestion('limit', 0.5);
    });
    now = 9_000;
    await act(async () => {
      await result.current.submitAnswer('42');
    });

    expect(mocks.submitAnswer).toHaveBeenLastCalledWith(
      expect.objectContaining({
        exerciseId: 'generated-1',
        timeSpentSeconds: 5,
      })
    );
    nowSpy.mockRestore();
  });

  it.each([
    [404, 'knowledge_point_not_found', '所选知识点不存在，请重新选择'],
    [429, 'generation_rate_limited', 'AI 出题请求过于频繁，请稍后再试'],
    [503, 'generation_unavailable', 'AI 出题服务暂不可用，请稍后重试'],
    [undefined, 'network_error', '无法连接到服务器，请检查网络后重试'],
  ] as const)('maps generation failure %s to %s', async (status, expectedType, expectedMessage) => {
    mocks.generateQuestion.mockRejectedValue(axiosError(status));
    const { result } = renderHook(() => useExerciseViewModel());

    await act(async () => {
      await result.current.generateQuestion('limit', 0.5);
    });

    expect(result.current.errorType).toBe(expectedType);
    expect(result.current.error).toBe(expectedMessage);
    expect(result.current.isGenerating).toBe(false);
  });

  it.each([
    ['', 0.5, '请选择知识点'],
    ['limit', Number.NaN, '请选择有效难度'],
    ['limit', 1.1, '请选择有效难度'],
  ])(
    'rejects invalid generation input without a request',
    async (conceptId, difficulty, expectedMessage) => {
      const { result } = renderHook(() => useExerciseViewModel());

      await act(async () => {
        await result.current.generateQuestion(conceptId, difficulty);
      });

      expect(mocks.generateQuestion).not.toHaveBeenCalled();
      expect(result.current.errorType).toBe('invalid_generation_request');
      expect(result.current.error).toBe(expectedMessage);
    }
  );
});
