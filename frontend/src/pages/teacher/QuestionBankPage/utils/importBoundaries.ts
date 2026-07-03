import type { ParsedQuestion } from '@/modules/question/types/questionImport';
import {
  MAX_AI_PARSE_TEXT_LENGTH,
  MAX_AI_PARSE_TEXTS,
  MAX_PARSED_IMPORT_QUESTIONS,
} from '@/modules/question/types/questionImport';

interface LimitedQuestionsResult {
  questions: ParsedQuestion[];
  warnings: string[];
}

export function limitParsedQuestions(questions: ParsedQuestion[]): LimitedQuestionsResult {
  if (questions.length <= MAX_PARSED_IMPORT_QUESTIONS) {
    return { questions, warnings: [] };
  }

  return {
    questions: questions.slice(0, MAX_PARSED_IMPORT_QUESTIONS),
    warnings: [
      `已识别到 ${questions.length} 道题，预览仅保留前 ${MAX_PARSED_IMPORT_QUESTIONS} 道，请拆分文件后继续导入。`,
    ],
  };
}

export function selectAiParseQuestions(
  parsedQuestions: ParsedQuestion[],
  ids: string[]
): LimitedQuestionsResult {
  const selectedIdSet = new Set(ids);
  const selected = parsedQuestions.filter((q) => selectedIdSet.has(q.tempId));

  if (selected.length <= MAX_AI_PARSE_TEXTS) {
    return { questions: selected, warnings: [] };
  }

  return {
    questions: selected.slice(0, MAX_AI_PARSE_TEXTS),
    warnings: [
      `AI 辅助识别一次最多处理 ${MAX_AI_PARSE_TEXTS} 道题，已仅处理前 ${MAX_AI_PARSE_TEXTS} 道。`,
    ],
  };
}

export function buildAiParseTexts(questions: ParsedQuestion[]): string[] {
  return questions.map((q) => q.rawText.slice(0, MAX_AI_PARSE_TEXT_LENGTH));
}
