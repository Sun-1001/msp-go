import { describe, expect, it } from 'vitest';
import {
  hasAsciiControlCharacters,
  replaceAsciiControlCharacters,
  stripUnsafeTextControlCharacters,
} from '@/libs/utils/controlCharacters';

describe('control character utilities', () => {
  it('detects C0 controls and delete without rejecting normal Unicode', () => {
    expect(hasAsciiControlCharacters('plain text')).toBe(false);
    expect(hasAsciiControlCharacters('中文内容')).toBe(false);
    expect(hasAsciiControlCharacters('line\nfeed')).toBe(true);
    expect(hasAsciiControlCharacters('delete\u007f')).toBe(true);
  });

  it('replaces every ASCII control character', () => {
    expect(replaceAsciiControlCharacters('a\u0000\u0008\u007fb', '_')).toBe('a___b');
  });

  it('strips unsafe text controls while preserving text whitespace', () => {
    expect(stripUnsafeTextControlCharacters('a\t\n\r\u0000\u000b\u007fb')).toBe(
      'a\t\n\rb',
    );
  });
});
