import { beforeEach, describe, expect, it, vi } from 'vitest';
import { exerciseService } from './exerciseService';

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}));

vi.mock('@/libs/http/apiClient', () => ({
  apiClient: apiClientMock,
}));

const questionResponse = {
  id: 'generated-1',
  title: '函数极限',
  content: '计算极限',
  difficulty: 0.5,
  type: 'multiple_choice',
  source: 'ai_generated',
  knowledge_points: ['limit'],
  knowledge_point_names: ['函数极限'],
  hints_available: true,
  estimated_time_seconds: 120,
  options: ['0', '1', '2', '不存在'],
};

const submitResponse = {
  is_correct: true,
  score: 1,
  student_answer_latex: 'x + 1',
  correct_answer_latex: 'x + 1',
  diagnosis: null,
  feedback: '回答正确',
  mastery_update: null,
  mastery_model: 'dkt',
  next_recommendation: 'continue',
};

describe('exerciseService', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('maps class questions while remaining compatible with responses without source', async () => {
    apiClientMock.get.mockResolvedValue({
      data: {
        id: questionResponse.id,
        title: questionResponse.title,
        content: questionResponse.content,
        difficulty: questionResponse.difficulty,
        type: questionResponse.type,
        knowledge_points: questionResponse.knowledge_points,
        hints_available: questionResponse.hints_available,
        estimated_time_seconds: questionResponse.estimated_time_seconds,
        options: questionResponse.options,
      },
    });

    const question = await exerciseService.fetchNextQuestion('limit', 0.5);

    expect(apiClientMock.get).toHaveBeenCalledWith('/exercise/next', {
      params: { concept_id: 'limit', difficulty: '0.5' },
    });
    expect(question).toMatchObject({
      id: 'generated-1',
      source: 'class',
      knowledgePoints: ['limit'],
      knowledgePointNames: [],
    });
  });

  it('posts generation parameters and maps generated question metadata', async () => {
    apiClientMock.post.mockResolvedValue({ data: questionResponse });

    const question = await exerciseService.generateQuestion({
      conceptId: 'limit',
      difficulty: 0.5,
    });

    expect(apiClientMock.post).toHaveBeenCalledWith('/exercise/generate', {
      concept_id: 'limit',
      difficulty: 0.5,
    });
    expect(question).toMatchObject({
      id: 'generated-1',
      source: 'ai_generated',
      knowledgePoints: ['limit'],
      knowledgePointNames: ['函数极限'],
      options: ['0', '1', '2', '不存在'],
    });
  });

  it('sends no image answer field', async () => {
    apiClientMock.post.mockResolvedValue({ data: submitResponse });

    await exerciseService.submitAnswer({
      exerciseId: 'exercise-1',
      answerText: 'x + 1',
      timeSpentSeconds: 12,
    });

    expect(apiClientMock.post).toHaveBeenCalledWith(
      '/exercise/submit',
      {
        exercise_id: 'exercise-1',
        answer_text: 'x + 1',
        answer_steps: undefined,
        time_spent_seconds: 12,
      },
      { timeout: 120_000 }
    );
    expect(apiClientMock.post.mock.calls[0][1]).not.toHaveProperty('answer_image_url');
  });

  it('sends an image-only answer and omits empty text', async () => {
    apiClientMock.post.mockResolvedValue({ data: submitResponse });

    await exerciseService.submitAnswer({
      exerciseId: 'exercise-1',
      answerImageUrl: '/uploads/images/answer.png',
      timeSpentSeconds: 8,
    });

    expect(apiClientMock.post).toHaveBeenCalledWith(
      '/exercise/submit',
      {
        exercise_id: 'exercise-1',
        answer_image_url: '/uploads/images/answer.png',
        answer_steps: undefined,
        time_spent_seconds: 8,
      },
      { timeout: 120_000 }
    );
    expect(apiClientMock.post.mock.calls[0][1]).not.toHaveProperty('answer_text');
  });

  it('maps explainable indeterminate grading fields', async () => {
    apiClientMock.post.mockResolvedValue({
      data: {
        ...submitResponse,
        is_correct: false,
        recorded: false,
        grading_status: 'indeterminate',
        evaluation: {
          method: 'math_solver',
          reason_code: 'MATH_SOLVER_UNAVAILABLE',
          reason: '暂时无法可靠判定',
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
        next_recommendation: 'retry',
      },
    });

    const result = await exerciseService.submitAnswer({
      exerciseId: 'exercise-1',
      answerText: 'x + 1',
      timeSpentSeconds: 5,
    });

    expect(result).toMatchObject({
      recorded: false,
      gradingStatus: 'indeterminate',
      nextRecommendation: 'retry',
      evaluation: {
        reasonCode: 'MATH_SOLVER_UNAVAILABLE',
        retryable: true,
        evidence: [
          {
            kind: 'comparison_boundary',
            summary: '字符串差异不能证明表达式不等价',
          },
        ],
      },
    });
  });

  it('preserves unavailable solution verification and failure details', async () => {
    apiClientMock.get.mockResolvedValue({
      data: {
        exercise_id: 'exercise-1',
        answer: '42',
        steps: [],
        source: 'unavailable',
        verification: {
          method: 'math_solver',
          reason_code: 'verification_needed',
          reason: '生成步骤未通过标准答案验证',
          confidence: 0.4,
          degraded: true,
          retryable: true,
          evidence: [{ kind: 'verification', summary: '步骤二无法推出结论' }],
        },
        failure: {
          code: 'verification_failed',
          stage: 'solution_verification',
          message: '生成解析未通过验证',
          retryable: true,
        },
      },
    });

    const solution = await exerciseService.getSolution('exercise-1');

    expect(apiClientMock.get).toHaveBeenCalledWith('/exercise/exercise-1/solution');
    expect(solution).toEqual({
      answer: '42',
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
        message: '生成解析未通过验证',
        retryable: true,
      },
    });
  });
});
