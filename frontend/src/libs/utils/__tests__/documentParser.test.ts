import { describe, expect, it } from 'vitest';
import {
  formatDocumentAsContext,
  parseDocument,
  validateDocumentFile,
  type ParsedDocument,
} from '@/libs/utils/documentParser';

function makeFile(content: string, name: string, type = 'text/plain'): File {
  return new File([content], name, { type });
}

describe('documentParser', () => {
  it('parses supported text files and removes unsafe control characters', async () => {
    const parsed = await parseDocument(
      makeFile('第一行\r\n第二行\u0000\u0008结束', 'notes.txt')
    );

    expect(parsed).toMatchObject({
      content: '第一行\n第二行结束',
      filename: 'notes.txt',
      type: 'txt',
    });
  });

  it('normalizes unsafe and oversized filenames before storing parsed documents', async () => {
    const longName = `${'a'.repeat(140)}\n..\\secret.txt`;
    const parsed = await parseDocument(makeFile('hello', longName));

    expect(parsed.filename).not.toContain('\n');
    expect(parsed.filename).not.toContain('\\');
    expect(parsed.filename.length).toBeLessThanOrEqual(120);
    expect(parsed.filename.endsWith('...')).toBe(true);
  });

  it('truncates oversized extracted content after sanitization', async () => {
    const parsed = await parseDocument(makeFile('a'.repeat(50001), 'large.md'));

    expect(parsed.content).toContain('[内容已截断，原文共 50001 字符]');
    expect(parsed.content.length).toBeGreaterThan(50000);
  });

  it('rejects unsupported empty or oversized files', () => {
    expect(validateDocumentFile(makeFile('hello', 'image.png', 'image/png'))).toEqual({
      valid: false,
      error: '不支持的文件类型: .png。支持的类型: TXT, MD, CSV, DOCX',
    });

    expect(validateDocumentFile(makeFile('', 'empty.txt'))).toEqual({
      valid: false,
      error: '文件内容为空',
    });

    const oversizedFile = new File(['x'], 'large.txt', { type: 'text/plain' });
    Object.defineProperty(oversizedFile, 'size', { value: 20 * 1024 * 1024 + 1 });

    expect(validateDocumentFile(oversizedFile)).toEqual({
      valid: false,
      error: '文件大小超过限制: 20.00MB > 20MB',
    });
  });

  it('formats document context with safe filenames and sanitized content', () => {
    const docs: ParsedDocument[] = [
      {
        filename: 'bad\n..\\name.txt',
        content: 'hello\u0000\nworld',
        type: 'txt',
        size: 12,
      },
    ];

    expect(formatDocumentAsContext(docs)).toBe('【附件：bad .._name.txt】\nhello\nworld');
  });
});
