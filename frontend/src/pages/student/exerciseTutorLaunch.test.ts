import { describe, expect, it } from 'vitest';
import type { Question } from '@/modules/exercise/services/exerciseService';
import { buildExerciseTutorLaunch } from './exerciseTutorLaunch';

const baseQuestion: Question = {
  id: 'exercise-1',
  title: '函数极限',
  content: '判断下列极限结论。',
  difficulty: 0.5,
  type: 'multiple_choice',
  source: 'class',
  knowledgePoints: ['limit'],
  knowledgePointNames: ['函数极限'],
  hintsAvailable: true,
  estimatedTimeSeconds: 120,
  options: ['A', 'B', 'C', 'D'],
};

describe('buildExerciseTutorLaunch', () => {
  it('builds a class-question tutor session that withholds the final answer', () => {
    const launch = buildExerciseTutorLaunch(baseQuestion);

    expect(launch.source).toBe('class');
    expect(launch.mode).toBe('explain');
    expect(launch.topic).toContain('班级题目辅导');
    expect(launch.initialMessage).toContain('老师发布的班级题目');
    expect(launch.initialMessage).toContain('不要直接给出最终答案');
  });

  it('builds a separate AI-practice tutor context with ambiguity handling', () => {
    const launch = buildExerciseTutorLaunch({
      ...baseQuestion,
      id: 'generated-1',
      source: 'ai_generated',
    });

    expect(launch.source).toBe('ai_generated');
    expect(launch.topic).toContain('AI 自主练习辅导');
    expect(launch.initialMessage).toContain('生成的自主练习题');
    expect(launch.initialMessage).toContain('题干或选项有歧义');
    expect(launch.initialMessage).not.toContain('老师发布');
  });
});
