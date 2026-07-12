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
    apiClientMock.post.mockResolvedValue({
      data: {
        is_correct: true,
        score: 1,
        student_answer_latex: 'x + 1',
        correct_answer_latex: 'x + 1',
        diagnosis: null,
        feedback: '回答正确',
        mastery_update: null,
        mastery_model: 'dkt',
        next_recommendation: 'continue',
      },
    });

    await exerciseService.submitAnswer({
      exerciseId: 'exercise-1',
      answerText: 'x + 1',
      timeSpentSeconds: 12,
    });

    expect(apiClientMock.post).toHaveBeenCalledWith('/exercise/submit', {
      exercise_id: 'exercise-1',
      answer_text: 'x + 1',
      answer_steps: undefined,
      time_spent_seconds: 12,
    });
    expect(apiClientMock.post.mock.calls[0][1]).not.toHaveProperty('answer_image_url');
  });
});
