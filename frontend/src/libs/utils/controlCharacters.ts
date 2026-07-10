const ASCII_DELETE = 0x7f;
const HORIZONTAL_TAB = 0x09;
const LINE_FEED = 0x0a;
const CARRIAGE_RETURN = 0x0d;

function isAsciiControlCode(code: number): boolean {
  return code <= 0x1f || code === ASCII_DELETE;
}

function transformCharacters(
  value: string,
  replacement: string,
  shouldReplace: (code: number) => boolean,
): string {
  const parts: string[] = [];
  let segmentStart = 0;

  for (let index = 0; index < value.length; index++) {
    if (!shouldReplace(value.charCodeAt(index))) {
      continue;
    }
    parts.push(value.slice(segmentStart, index), replacement);
    segmentStart = index + 1;
  }

  if (segmentStart === 0) {
    return value;
  }
  parts.push(value.slice(segmentStart));
  return parts.join('');
}

export function hasAsciiControlCharacters(value: string): boolean {
  for (let index = 0; index < value.length; index++) {
    if (isAsciiControlCode(value.charCodeAt(index))) {
      return true;
    }
  }
  return false;
}

export function replaceAsciiControlCharacters(value: string, replacement: string): string {
  return transformCharacters(value, replacement, isAsciiControlCode);
}

export function stripUnsafeTextControlCharacters(value: string): string {
  return transformCharacters(
    value,
    '',
    (code) =>
      isAsciiControlCode(code) &&
      code !== HORIZONTAL_TAB &&
      code !== LINE_FEED &&
      code !== CARRIAGE_RETURN,
  );
}
