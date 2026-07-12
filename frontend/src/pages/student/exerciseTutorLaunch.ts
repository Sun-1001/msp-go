import type { Question } from '@/modules/exercise/services/exerciseService';
import type { SessionMode } from '@/modules/session/services/sessionService';

export interface ExerciseTutorLaunchState {
  initialMessage: string;
  mode: SessionMode;
  source: Question['source'];
  topic: string;
}

const sourceLabel = (source: Question['source']): string =>
  source === 'ai_generated' ? 'AI 自主练习' : '班级题目';

export const buildExerciseTutorLaunch = (question: Question): ExerciseTutorLaunchState => {
  const knowledgePoints = question.knowledgePointNames.length > 0
    ? question.knowledgePointNames
    : question.knowledgePoints;
  const knowledgeLabel = knowledgePoints.length > 0 ? knowledgePoints.join('、') : '未标注';
  const options = question.options?.length
    ? ['', '选项：', ...question.options.map((option, index) => `${index + 1}. ${option}`)]
    : [];
  const tutorInstruction = question.source === 'ai_generated'
    ? '这是按所选知识点和难度生成的自主练习题。请围绕该知识点分步辅导；如果题干或选项有歧义，请明确指出并建议重新生成。除非我说明已提交，否则不要直接给出正确选项。'
    : '这是老师发布的班级题目。请尊重原题意，通过追问、提示和分步思路帮助我作答；除非我说明已提交，否则不要直接给出最终答案。';
  const topic = `${sourceLabel(question.source)}辅导 · ${knowledgePoints[0] || question.title || '当前题目'}`;

  return {
    source: question.source,
    mode: 'explain',
    topic: topic.slice(0, 36),
    initialMessage: [
      `【辅导场景：${sourceLabel(question.source)}】`,
      tutorInstruction,
      '',
      `题目 ID：${question.id}`,
      `标题：${question.title || '练习题'}`,
      `难度：${Math.round(question.difficulty * 100)}%`,
      `知识点：${knowledgeLabel}`,
      '',
      question.content,
      ...options,
      '',
      '请先帮我梳理解题切入点。',
    ].join('\n'),
  };
};
