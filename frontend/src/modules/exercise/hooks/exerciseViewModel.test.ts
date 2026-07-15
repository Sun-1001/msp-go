import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useExerciseViewModel } from '@/modules/exercise/hooks/exerciseViewModel';

const mocks = vi.hoisted(() => ({
  fetchNextQuestion: vi.fn(),
  generateQuestion: vi.fn(),
  submitAnswer: vi.fn(),
  getSolution: vi.fn(),
  validateImageFile: vi.fn(),
  uploadImage: vi.fn(),
}));

vi.mock('@/modules/exercise/services/exerciseService', () => ({
  exerciseService: {
    fetchNextQuestion: mocks.fetchNextQuestion,
    generateQuestion: mocks.generateQuestion,
    submitAnswer: mocks.submitAnswer,
    getSolution: mocks.getSolution,
  },
}));

vi.mock('@/modules/upload/services/uploadService', () => ({
  uploadService: {
    validateImageFile: mocks.validateImageFile,
    uploadImage: mocks.uploadImage,
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

const successfulSubmitResult = {
  isCorrect: true,
  recorded: true,
  gradingStatus: 'correct' as const,
  score: 1,
  studentAnswerLatex: '42',
  correctAnswerLatex: '42',
  diagnosis: null,
  feedback: '正确',
  masteryUpdate: {},
  masteryModel: 'dkt-sakt-lite',
  nextRecommendation: 'advance' as const,
};

const axiosError = (status?: number, code?: string) => ({
  isAxiosError: true,
  request: {},
  response: status === undefined ? undefined : { status, data: { code, detail: 'service error' } },
});

describe('useExerciseViewModel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchNextQuestion.mockResolvedValue(question);
    mocks.generateQuestion.mockResolvedValue(generatedQuestion);
    mocks.validateImageFile.mockReturnValue({ valid: true });
    mocks.uploadImage.mockResolvedValue({ url: '/uploads/images/answer.png' });
    mocks.submitAnswer.mockResolvedValue(successfulSubmitResult);
    mocks.getSolution.mockResolvedValue({
      answer: '42',
      steps: ['计算得到 42'],
      source: 'cached',
      verification: null,
      failure: null,
    });
  });

  it('submits text-only answer payloads', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    await act(async () => {
      await result.current.submitAnswer({ answerText: '42' });
    });

    expect(mocks.submitAnswer).toHaveBeenCalledWith(
      expect.objectContaining({
        exerciseId: 'exercise-1',
        answerText: '42',
      })
    );
    expect(mocks.submitAnswer.mock.calls[0][0]).not.toHaveProperty('answerImageUrl');
  });

  it('gives non-empty text priority without validating or uploading an attached image', async () => {
    const image = new File(['image'], 'answer.png', { type: 'image/png' });
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    await act(async () => {
      await result.current.submitAnswer({ answerText: '  42  ', answerImage: image });
    });

    expect(mocks.validateImageFile).not.toHaveBeenCalled();
    expect(mocks.uploadImage).not.toHaveBeenCalled();
    expect(mocks.submitAnswer).toHaveBeenCalledWith(
      expect.objectContaining({ answerText: '42' })
    );
    expect(mocks.submitAnswer.mock.calls[0][0]).not.toHaveProperty('answerImageUrl');
  });

  it('uploads an image before grading and ignores concurrent submissions', async () => {
    const image = new File(['image'], 'answer.png', { type: 'image/png' });
    let resolveUpload: ((value: { url: string }) => void) | undefined;
    let resolveSubmit: ((value: typeof successfulSubmitResult) => void) | undefined;
    mocks.uploadImage.mockImplementation(
      () => new Promise<{ url: string }>((resolve) => { resolveUpload = resolve; })
    );
    mocks.submitAnswer.mockImplementation(
      () => new Promise<typeof successfulSubmitResult>((resolve) => { resolveSubmit = resolve; })
    );
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    let pending: Promise<void> | undefined;
    act(() => {
      pending = result.current.submitAnswer({ answerImage: image });
      void result.current.submitAnswer({ answerImage: image });
    });

    expect(result.current.submitPhase).toBe('uploading');
    expect(result.current.isSubmitting).toBe(true);
    expect(mocks.uploadImage).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveUpload?.({ url: '/uploads/images/answer.png' });
      await Promise.resolve();
    });
    expect(result.current.submitPhase).toBe('recognizing');
    expect(mocks.submitAnswer).toHaveBeenCalledWith(
      expect.objectContaining({
        exerciseId: 'exercise-1',
        answerImageUrl: '/uploads/images/answer.png',
      })
    );
    expect(mocks.submitAnswer.mock.calls[0][0]).not.toHaveProperty('answerText');

    await act(async () => {
      resolveSubmit?.(successfulSubmitResult);
      await pending;
    });
    expect(result.current.submitPhase).toBe('idle');
    expect(result.current.isSubmitting).toBe(false);
  });

  it('keeps the same image retryable after client validation fails', async () => {
    const image = new File(['image'], 'answer.png', { type: 'image/png' });
    mocks.validateImageFile
      .mockReturnValueOnce({ valid: false, error: '图片过大' })
      .mockReturnValueOnce({ valid: true });
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer({ answerImage: image });
    });

    expect(result.current.error).toBe('图片过大');
    expect(mocks.uploadImage).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.submitAnswer({ answerImage: image });
    });
    expect(mocks.uploadImage).toHaveBeenCalledWith(image);
    expect(mocks.submitAnswer).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['OCR_UNREADABLE', 'answer_unreadable', '未能从图片中识别出有效答案，请重新拍摄或改用文字答案'],
    ['OCR_UNAVAILABLE', 'answer_service_unavailable', '图片识别服务暂不可用，请稍后重试或改用文字答案'],
    ['OCR_TIMEOUT', 'answer_service_unavailable', '图片识别超时，请稍后重试'],
    ['OCR_RATE_LIMITED', 'answer_rate_limited', '图片识别请求过于频繁，请稍后重试'],
    ['RATE_LIMITED', 'answer_rate_limited', '答案提交请求过于频繁，请稍后重试'],
    ['ANSWER_PARSE_FAILED', 'invalid_answer', '答案格式无法安全解析，请检查输入或改用更清晰的图片后重试'],
    ['MATH_UNSUPPORTED', 'answer_unsupported', '当前题型暂不支持自动判定，请补充解题步骤或联系教师'],
    ['MATH_SOLVER_INVALID_RESPONSE', 'answer_service_unavailable', '数学判题服务返回异常，请稍后重试'],
    ['MATH_SOLVER_UNAVAILABLE', 'answer_service_unavailable', '数学判题服务暂不可用，请稍后重试'],
    ['MATH_SOLVER_TIMEOUT', 'answer_service_unavailable', '数学判题服务处理超时，请稍后重试'],
    ['EXERCISE_CHANGED', 'exercise_changed', '题目已更新，请重新加载后提交'],
  ] as const)('maps submission code %s to an actionable retry state', async (code, type, message) => {
    mocks.submitAnswer.mockRejectedValue(axiosError(503, code));
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer({ answerText: '42' });
    });

    expect(result.current.errorType).toBe(type);
    expect(result.current.error).toBe(message);
    expect(result.current.errorSource).toBe('submission');
    expect(result.current.submitPhase).toBe('idle');
  });

  it('identifies upload rate limits before OCR starts', async () => {
    mocks.uploadImage.mockRejectedValueOnce(axiosError(429, 'RATE_LIMITED'));
    const image = new File(['image'], 'answer.png', { type: 'image/png' });
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer({ answerImage: image });
    });

    expect(mocks.submitAnswer).not.toHaveBeenCalled();
    expect(result.current.error).toBe('图片上传请求过于频繁，请稍后重试');
    expect(result.current.errorType).toBe('answer_rate_limited');
    expect(result.current.errorSource).toBe('submission');
  });

  it('rejects WebP answer images without changing generic upload support', async () => {
    const image = new File(['image'], 'answer.webp', { type: 'image/webp' });
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer({ answerImage: image });
    });

    expect(mocks.validateImageFile).not.toHaveBeenCalled();
    expect(mocks.uploadImage).not.toHaveBeenCalled();
    expect(result.current.error).toBe('答案图片仅支持 JPEG、PNG 或 GIF 格式');
  });

  it('rejects a submission without text or an image', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    await act(async () => {
      await result.current.submitAnswer({ answerText: '   ' });
    });

    expect(mocks.submitAnswer).not.toHaveBeenCalled();
    expect(result.current.error).toBe('请输入答案或上传答案图片');
  });

  it('keeps submitted feedback when loading the next class question fails', async () => {
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.submitAnswer({ answerText: '42' });
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

  it('keeps an unavailable solution failure explainable', async () => {
    mocks.getSolution.mockResolvedValueOnce({
      answer: '42',
      steps: [],
      source: 'unavailable',
      verification: null,
      failure: {
        code: 'solver_unavailable',
        stage: 'solution_generation',
        message: '通用数学求解服务未配置',
        retryable: true,
      },
    });
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });
    await act(async () => {
      await result.current.loadSolution();
    });

    expect(mocks.getSolution).toHaveBeenCalledWith('exercise-1');
    expect(result.current.solution).toMatchObject({
      source: 'unavailable',
      steps: [],
      failure: { code: 'solver_unavailable', retryable: true },
    });
    expect(result.current.solutionError).toBeNull();
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
      await result.current.submitAnswer({ answerText: '42' });
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

  it('blocks regeneration during submission and discards a stale result after the question changes', async () => {
    let resolveSubmit: ((value: typeof successfulSubmitResult) => void) | undefined;
    mocks.submitAnswer.mockImplementationOnce(
      () => new Promise<typeof successfulSubmitResult>((resolve) => { resolveSubmit = resolve; })
    );
    const replacementQuestion = { ...question, id: 'exercise-2' };
    mocks.fetchNextQuestion
      .mockResolvedValueOnce(question)
      .mockResolvedValueOnce(replacementQuestion);
    const { result } = renderHook(() => useExerciseViewModel());
    await act(async () => {
      await result.current.loadNextQuestion();
    });

    let pendingSubmission: Promise<void> | undefined;
    act(() => {
      pendingSubmission = result.current.submitAnswer({ answerText: '42' });
    });
    await act(async () => {
      await result.current.generateQuestion('limit', 0.5);
    });
    expect(mocks.generateQuestion).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.loadNextQuestion();
    });
    expect(result.current.currentQuestion).toEqual(replacementQuestion);

    await act(async () => {
      resolveSubmit?.(successfulSubmitResult);
      await pendingSubmission;
    });
    expect(result.current.currentQuestion).toEqual(replacementQuestion);
    expect(result.current.submitResult).toBeNull();
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
      await result.current.submitAnswer({ answerText: '42' });
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
    expect(result.current.errorSource).toBe('generation');
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
