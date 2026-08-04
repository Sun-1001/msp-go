import React from 'react';
import { MathRenderer } from '@/libs/math/MathRenderer';
import { MathText } from '@/libs/math/MathText';

const inlineOrBlockMathRegex = /\$\$?[\s\S]+?\$\$?/;
const latexHintRegex = /\\[a-zA-Z]+|[_^]/;

const normalizeExerciseMathDelimiters = (value: string) =>
  value
    .replace(/\\\[\s*([\s\S]*?)\s*\\\]/g, (_match, expression: string) => `$$${expression}$$`)
    .replace(/\\\(\s*([\s\S]*?)\s*\\\)/g, (_match, expression: string) => `$${expression}$`);

interface ExerciseMathContentProps {
  value: string;
  className?: string;
  block?: boolean;
}

export const ExerciseMathContent: React.FC<ExerciseMathContentProps> = ({
  value,
  className,
  block = false,
}) => {
  if (!value) return null;

  const normalizedValue = normalizeExerciseMathDelimiters(value);
  const safeContentClassName = [
    'min-w-0 max-w-full overflow-x-auto overflow-y-hidden [overflow-wrap:anywhere]',
    className,
  ].filter(Boolean).join(' ');

  if (inlineOrBlockMathRegex.test(normalizedValue)) {
    return <MathText className={safeContentClassName}>{normalizedValue}</MathText>;
  }

  if (latexHintRegex.test(normalizedValue)) {
    return (
      <MathRenderer
        expression={normalizedValue}
        block={block}
        className={`inline-block ${safeContentClassName}`}
      />
    );
  }

  return <span className={safeContentClassName}>{normalizedValue}</span>;
};
