import { describe, expect, it } from 'vitest';
import type { ParsedQuestion } from '@/modules/question/types/questionImport';
import {
  MAX_AI_PARSE_TEXT_LENGTH,
  MAX_AI_PARSE_TEXTS,
  MAX_PARSED_IMPORT_QUESTIONS,
} from '@/modules/question/types/questionImport';
import {
  buildAiParseTexts,
  limitParsedQuestions,
  selectAiParseQuestions,
} from '../importBoundaries';

function makeQuestion(index: number): ParsedQuestion {
  return {
    tempId: `q-${index}`,
    title: '',
    body: `第 ${index} 题`,
    type: 'short_answer',
    difficulty: 0.5,
    answer: '',
    answerType: 'text',
    hints: [],
    solutionSteps: [],
    tags: [],
    confidence: 0.5,
    rawText: `raw-${index}`,
    parseWarnings: [],
  };
}

describe('question import boundaries', () => {
  it('keeps parsed question lists within the preview limit', () => {
    const questions = Array.from(
      { length: MAX_PARSED_IMPORT_QUESTIONS + 2 },
      (_, index) => makeQuestion(index)
    );

    const result = limitParsedQuestions(questions);

    expect(result.questions).toHaveLength(MAX_PARSED_IMPORT_QUESTIONS);
    expect(result.questions[0].tempId).toBe('q-0');
    expect(result.questions.at(-1)?.tempId).toBe(`q-${MAX_PARSED_IMPORT_QUESTIONS - 1}`);
    expect(result.warnings[0]).toContain(`${MAX_PARSED_IMPORT_QUESTIONS + 2}`);
  });

  it('leaves small parsed question lists unchanged', () => {
    const questions = [makeQuestion(1), makeQuestion(2)];

    expect(limitParsedQuestions(questions)).toEqual({ questions, warnings: [] });
  });

  it('limits AI parse selections and preserves parsed question order', () => {
    const questions = Array.from({ length: MAX_AI_PARSE_TEXTS + 3 }, (_, index) =>
      makeQuestion(index)
    );
    const ids = questions.map((q) => q.tempId).reverse();

    const result = selectAiParseQuestions(questions, ids);

    expect(result.questions).toHaveLength(MAX_AI_PARSE_TEXTS);
    expect(result.questions.map((q) => q.tempId)).toEqual(
      questions.slice(0, MAX_AI_PARSE_TEXTS).map((q) => q.tempId)
    );
    expect(result.warnings[0]).toContain(`${MAX_AI_PARSE_TEXTS}`);
  });

  it('truncates raw texts sent to AI parsing', () => {
    const question = {
      ...makeQuestion(1),
      rawText: 'x'.repeat(MAX_AI_PARSE_TEXT_LENGTH + 1),
    };

    expect(buildAiParseTexts([question])).toEqual(['x'.repeat(MAX_AI_PARSE_TEXT_LENGTH)]);
  });
});
