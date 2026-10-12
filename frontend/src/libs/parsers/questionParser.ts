/**
 * 题目识别核心算法
 *
 * 三阶段流水线：LaTeX 保护 + 题目分割 → 结构提取 → 置信度评分
 * 纯前端实现，不依赖后端
 */

import type { ParsedQuestion } from '@/modules/question';

// ==================== 常量定义 ====================

let _tempIdCounter = 0;
function generateTempId(): string {
  return `parsed_${Date.now()}_${++_tempIdCounter}`;
}

// ==================== 阶段一：LaTeX 保护 ====================

interface LatexProtection {
  text: string;
  restore: (s: string) => string;
}

/**
 * 保护 LaTeX 公式区域，防止正则分割时误切公式
 */
function protectLatex(text: string): LatexProtection {
  const placeholders: Array<{ placeholder: string; original: string }> = [];
  let counter = 0;

  // 按优先级替换：先处理块级公式，再处理行内公式
  // $$...$$ 块级公式
    let protected_ = text.replace(/\$\$[\s\S]*?\$\$/g, (match) => {
    const placeholder = `__LATEX_BLOCK_${counter++}__`;
    placeholders.push({ placeholder, original: match });
    return placeholder;
  });

  // \[...\] 块级公式
  protected_ = protected_.replace(/\\\[[\s\S]*?\\\]/g, (match) => {
    const placeholder = `__LATEX_BLOCK_${counter++}__`;
    placeholders.push({ placeholder, original: match });
    return placeholder;
  });

  // $...$ 行内公式（不匹配 $$）
  protected_ = protected_.replace(/(?<!\$)\$(?!\$)([^$\n]+?)\$(?!\$)/g, (match) => {
    const placeholder = `__LATEX_INLINE_${counter++}__`;
    placeholders.push({ placeholder, original: match });
    return placeholder;
  });

  // \(...\) 行内公式
  protected_ = protected_.replace(/\\\([\s\S]*?\\\)/g, (match) => {
    const placeholder = `__LATEX_INLINE_${counter++}__`;
    placeholders.push({ placeholder, original: match });
    return placeholder;
  });

  const restore = (s: string): string => {
    let result = s;
    // 逆序恢复，避免嵌套替换问题
    for (let i = placeholders.length - 1; i >= 0; i--) {
      result = result.replace(placeholders[i].placeholder, placeholders[i].original);
    }
    return result;
  };

  return { text: protected_, restore };
}

// ==================== 阶段一：题目分割 ====================

/**
 * 大题分组标记正则（如"一、选择题"，提取为 tag）
 */
const SECTION_HEADER_PATTERN =
  /^[一二三四五六七八九十]+\s*[、.．]\s*(选择题|填空题|解答题|证明题|计算题|简答题|应用题|综合题)/m;

/**
 * 题目编号正则列表（按优先级排序）
 */
const QUESTION_NUMBER_PATTERNS = [
  // "第一题"、"第1题"、"第 1 题"
  /^第?\s*[一二三四五六七八九十百\d]+\s*题[、.．:：)）]?\s*/m,
  // "1."、"1．"、"1、"、"1）"、"1)"
  /^(\d{1,3})\s*[.．、)）]\s*/m,
  // "(1)"、"（1）"
  /^[（(]\s*(\d{1,3})\s*[)）]\s*/m,
];

/**
 * 将文本分割为题目块
 */
function splitIntoQuestionBlocks(text: string): Array<{ text: string; sectionTag?: string }> {
  const lines = text.split('\n');
  const blocks: Array<{ text: string; sectionTag?: string }> = [];
  let currentBlock: string[] = [];
  let currentSectionTag: string | undefined;

  for (const line of lines) {
    const trimmedLine = line.trim();
    if (!trimmedLine) {
      if (currentBlock.length > 0) {
        currentBlock.push('');
      }
      continue;
    }

    // 检查是否是大题分组标记
    const sectionMatch = trimmedLine.match(SECTION_HEADER_PATTERN);
    if (sectionMatch) {
      // 保存当前块
      if (currentBlock.length > 0) {
        const blockText = currentBlock.join('\n').trim();
        if (blockText) {
          blocks.push({ text: blockText, sectionTag: currentSectionTag });
        }
        currentBlock = [];
      }
      currentSectionTag = sectionMatch[1];
      continue;
    }

    // 检查是否是新题目的开始
    let isNewQuestion = false;
    for (const pattern of QUESTION_NUMBER_PATTERNS) {
      if (pattern.test(trimmedLine)) {
        isNewQuestion = true;
        break;
      }
    }

    if (isNewQuestion && currentBlock.length > 0) {
      // 保存当前块，开始新块
      const blockText = currentBlock.join('\n').trim();
      if (blockText) {
        blocks.push({ text: blockText, sectionTag: currentSectionTag });
      }
      currentBlock = [trimmedLine];
    } else {
      currentBlock.push(trimmedLine);
    }
  }

  // 保存最后一个块
  if (currentBlock.length > 0) {
    const blockText = currentBlock.join('\n').trim();
    if (blockText) {
      blocks.push({ text: blockText, sectionTag: currentSectionTag });
    }
  }

  return blocks;
}

// ==================== 阶段二：结构提取 ====================

/** 答案区域识别正则 */
const ANSWER_PATTERNS = [
  /^(?:答案|答|解答|标准答案)\s*[:：]\s*/m,
  /^(?:Answer|Solution|Ans)\s*[:：]\s*/im,
];

/** 解题步骤/解析区域识别正则 */
const SOLUTION_PATTERNS = [
  /^(?:解析|解题过程|解题步骤|详解|分析)\s*[:：]\s*/m,
  /^(?:解)\s*[:：]\s*/m,
  /^(?:Explanation|Analysis)\s*[:：]\s*/im,
];

/** 选项识别正则 */
const OPTION_PATTERN = /^([A-D])\s*[.、．:：)）]\s*/m;

/** 证明题关键词 */
const PROOF_KEYWORDS = /证明|证|prove|proof/i;

/**
 * 从题目编号行中移除编号前缀
 */
function removeQuestionNumber(text: string): string {
  let result = text;
  // 移除 "第X题" 格式
  result = result.replace(/^第?\s*[一二三四五六七八九十百\d]+\s*题[、.．:：)）]?\s*/, '');
  // 移除 "1." "1、" "1）" 格式
  result = result.replace(/^\d{1,3}\s*[.．、)）]\s*/, '');
  // 移除 "(1)" "（1）" 格式
  result = result.replace(/^[（(]\s*\d{1,3}\s*[)）]\s*/, '');
  return result;
}

/**
 * 从文本块中提取题目结构
 */
function extractQuestionStructure(
  blockText: string,
  sectionTag?: string,
): ParsedQuestion {
  const warnings: string[] = [];
  const lines = blockText.split('\n');

  // 移除题号前缀
  const cleanedFirstLine = removeQuestionNumber(lines[0]);
  lines[0] = cleanedFirstLine;

  // 分离各区域
  const bodyLines: string[] = [];
  const answerLines: string[] = [];
  const solutionLines: string[] = [];
  const options: string[] = [];
  let currentSection: 'body' | 'answer' | 'solution' = 'body';

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) {
      if (currentSection === 'body') bodyLines.push('');
      else if (currentSection === 'answer') answerLines.push('');
      else solutionLines.push('');
      continue;
    }

    // 检查是否进入答案区域
    let isAnswer = false;
    for (const pattern of ANSWER_PATTERNS) {
      if (pattern.test(trimmed)) {
        currentSection = 'answer';
        const content = trimmed.replace(pattern, '').trim();
        if (content) answerLines.push(content);
        isAnswer = true;
        break;
      }
    }
    if (isAnswer) continue;

    // 检查是否进入解析区域
    let isSolution = false;
    for (const pattern of SOLUTION_PATTERNS) {
      if (pattern.test(trimmed)) {
        currentSection = 'solution';
        const content = trimmed.replace(pattern, '').trim();
        if (content) solutionLines.push(content);
        isSolution = true;
        break;
      }
    }
    if (isSolution) continue;

    // 检查是否是选项
    const optionMatch = trimmed.match(OPTION_PATTERN);
    if (optionMatch && currentSection === 'body') {
      const optionContent = trimmed.replace(OPTION_PATTERN, '').trim();
      options.push(optionContent);
      continue;
    }

    // 添加到当前区域
    if (currentSection === 'body') bodyLines.push(trimmed);
    else if (currentSection === 'answer') answerLines.push(trimmed);
    else solutionLines.push(trimmed);
  }

  // 构建题目内容
  const body = bodyLines.join('\n').trim();
  const answer = answerLines.join('\n').trim();
  const solutionStepsRaw = solutionLines.join('\n').trim();

  // 使用大题分组标记作为分组名，否则留空让用户手动选择
  const title = sectionTag || '';

  // 推断题型
  let type: ParsedQuestion['type'] = 'unknown';
  if (options.length >= 2) {
    type = 'multiple_choice';
  } else if (PROOF_KEYWORDS.test(body) || PROOF_KEYWORDS.test(title)) {
    type = 'proof';
  } else if (body.length > 5) {
    type = 'short_answer';
  }

  // 根据大题分组标记辅助判断题型
  if (sectionTag) {
    if (sectionTag.includes('选择')) type = 'multiple_choice';
    else if (sectionTag.includes('证明')) type = 'proof';
    else if (sectionTag.includes('填空') || sectionTag.includes('简答') || sectionTag.includes('计算'))
      type = 'short_answer';
  }

  // 推断答案类型
  let answerType: ParsedQuestion['answerType'] = 'text';
  if (answer) {
    // 包含 LaTeX 数学表达式
    if (/\$.*\$/.test(answer) || /\\frac|\\sqrt|\\int|\\sum/.test(answer)) {
      answerType = 'expression';
    }
    // 纯数字
    else if (/^-?\d+(\.\d+)?$/.test(answer.trim())) {
      answerType = 'numeric';
    }
    // 选择题答案（A/B/C/D）
    else if (/^[A-D]$/.test(answer.trim())) {
      answerType = 'text';
    }
  }

  // 解题步骤拆分
  const solutionSteps = solutionStepsRaw
    ? solutionStepsRaw
        .split(/\n(?=(?:步骤|Step)\s*\d|(?:\d+)[.、．]\s)/)
        .map((s) => s.trim())
        .filter(Boolean)
    : [];

  // 构建标签
  const tags: string[] = [];
  if (sectionTag) tags.push(sectionTag);

  return {
    tempId: generateTempId(),
    title,
    body,
    type,
    difficulty: 0.5,
    answer,
    answerType,
    options: options.length > 0 ? options : undefined,
    hints: [],
    solutionSteps,
    tags,
    confidence: 0,
    rawText: blockText,
    parseWarnings: warnings,
  };
}

// ==================== 阶段三：置信度评分 + 后处理 ====================

/**
 * 计算题目解析置信度
 */
function calculateConfidence(q: ParsedQuestion): number {
  let score = 0;

  // 有实质内容 (+0.3)
  if (q.body.length > 10) score += 0.3;

  // 有答案 (+0.3)
  if (q.answer.length > 0) score += 0.3;

  // 题型已识别 (+0.2)
  if (q.type !== 'unknown') score += 0.2;

  // 包含数学公式 (+0.1)
  if (/\$.*\$/.test(q.body) || /\\[a-zA-Z]+/.test(q.body)) score += 0.1;

  // 有解题步骤 (+0.1)
  if (q.solutionSteps.length > 0) score += 0.1;

  return Math.min(score, 1);
}

/**
 * 后处理：过滤空题目、计算置信度
 */
function postProcess(questions: ParsedQuestion[]): ParsedQuestion[] {
  return questions
    .filter((q) => {
      // 过滤掉内容过短的题目
      if (q.body.length < 5) return false;
      return true;
    })
    .map((q) => ({
      ...q,
      confidence: calculateConfidence(q),
    }));
}

// ==================== 主入口 ====================

/**
 * 从纯文本中解析题目
 *
 * 三阶段流水线：
 * 1. LaTeX 保护 + 题目分割
 * 2. 结构提取
 * 3. 置信度评分 + 后处理
 *
 * @param text - 纯文本内容（来自 DOCX 或 TXT 解析）
 * @returns 解析出的题目数组
 */
export function parseQuestions(text: string): ParsedQuestion[] {
  if (!text || !text.trim()) {
    return [];
  }

  // 阶段一：LaTeX 保护
  const { text: protectedText, restore } = protectLatex(text);

  // 阶段一：题目分割
  const blocks = splitIntoQuestionBlocks(protectedText);

  if (blocks.length === 0) {
    return [];
  }

  // 阶段二：结构提取（恢复 LaTeX 后提取）
  const questions = blocks.map((block) => {
    const restoredText = restore(block.text);
    return extractQuestionStructure(restoredText, block.sectionTag);
  });

  // 阶段三：后处理
  return postProcess(questions);
}

/**
 * 将 AI 解析结果转换为 ParsedQuestion 格式
 */
export function aiResultToParsedQuestion(
  aiItem: {
    title: string;
    body: string;
    type: string;
    difficulty: number;
    answer: string;
    answer_type: string;
    options?: string[];
    hints: string[];
    solution_steps: string[];
    tags: string[];
  },
  originalRawText: string,
): ParsedQuestion {
  const typeMap: Record<string, ParsedQuestion['type']> = {
    short_answer: 'short_answer',
    multiple_choice: 'multiple_choice',
    proof: 'proof',
  };

  const answerTypeMap: Record<string, ParsedQuestion['answerType']> = {
    expression: 'expression',
    numeric: 'numeric',
    text: 'text',
  };

  return {
    tempId: generateTempId(),
    title: aiItem.title || '',
    body: aiItem.body || '',
    type: typeMap[aiItem.type] || 'short_answer',
    difficulty: Math.max(0, Math.min(1, aiItem.difficulty || 0.5)),
    answer: aiItem.answer || '',
    answerType: answerTypeMap[aiItem.answer_type] || 'text',
    options: aiItem.options,
    hints: aiItem.hints || [],
    solutionSteps: aiItem.solution_steps || [],
    tags: aiItem.tags || [],
    confidence: 0.9, // AI 解析结果给予较高置信度
    rawText: originalRawText,
    parseWarnings: [],
  };
}
