/**
 * 文档解析工具
 *
 * 在客户端本地解析文档文件，提取文本内容
 * 支持格式：txt, md, csv, docx
 */

import { parseDocxFile } from '@/libs/parsers/docxParser';
import { parseTxtFile } from '@/libs/parsers/txtParser';

/** 解析后的文档 */
export interface ParsedDocument {
  /** 提取的文本内容 */
  content: string;
  /** 原始文件名 */
  filename: string;
  /** 文件类型 */
  type: string;
  /** 文件大小（字节） */
  size: number;
}

/** 支持的文档 MIME 类型 */
const SUPPORTED_TYPES: Record<string, string> = {
  'text/plain': 'txt',
  'text/markdown': 'md',
  'text/csv': 'csv',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'docx',
};

/** 支持的文件扩展名（用于 MIME 类型不准确时的回退判断） */
const SUPPORTED_EXTENSIONS = ['txt', 'md', 'csv', 'docx'];

/** 最大文件大小：20MB */
const MAX_FILE_SIZE = 20 * 1024 * 1024;

/** 最大提取文本长度：50000 字符（避免消息过长） */
const MAX_CONTENT_LENGTH = 50000;

/** 最大展示文件名长度 */
const MAX_FILENAME_LENGTH = 120;

/**
 * 获取文件扩展名
 */
function getFileExtension(filename: string): string {
  return filename.split('.').pop()?.toLowerCase() || '';
}

function normalizeDocumentFilename(filename: string): string {
  const cleaned = filename
    .replace(/[\u0000-\u001F\u007F]+/g, ' ')
    .replace(/[\\/]+/g, '_')
    .replace(/\s+/g, ' ')
    .trim();

  const safeName = cleaned || 'document';
  if (safeName.length <= MAX_FILENAME_LENGTH) {
    return safeName;
  }

  return `${safeName.slice(0, MAX_FILENAME_LENGTH - 3)}...`;
}

function sanitizeDocumentContent(content: string): string {
  return content
    .replace(/\r\n?/g, '\n')
    .replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F]/g, '');
}

/**
 * 判断文件是否为支持的文档类型
 */
export function isSupportedDocument(file: File): boolean {
  if (SUPPORTED_TYPES[file.type]) return true;
  return SUPPORTED_EXTENSIONS.includes(getFileExtension(file.name));
}

/**
 * 验证文档文件
 */
export function validateDocumentFile(file: File): { valid: boolean; error?: string } {
  if (!isSupportedDocument(file)) {
    const ext = getFileExtension(file.name);
    return {
      valid: false,
      error: `不支持的文件类型: .${ext}。支持的类型: TXT, MD, CSV, DOCX`,
    };
  }

  if (file.size > MAX_FILE_SIZE) {
    return {
      valid: false,
      error: `文件大小超过限制: ${(file.size / 1024 / 1024).toFixed(2)}MB > 20MB`,
    };
  }

  if (file.size === 0) {
    return { valid: false, error: '文件内容为空' };
  }

  return { valid: true };
}

/**
 * 读取文本文件（txt / md / csv）
 */
async function parseTextFile(file: File): Promise<string> {
  const result = await parseTxtFile(file);
  return result.text;
}

/**
 * 解析 DOCX 文件
 *
 * 使用 mammoth.js 提取纯文本内容
 */
async function parseDocx(file: File): Promise<string> {
  const result = await parseDocxFile(file);
  return result.text;
}

/**
 * 解析文档文件（主入口）
 *
 * @param file 文档文件
 * @returns 解析后的文档对象
 * @throws Error 当文件类型不支持或解析失败时
 */
export async function parseDocument(file: File): Promise<ParsedDocument> {
  const validation = validateDocumentFile(file);
  if (!validation.valid) {
    throw new Error(validation.error);
  }

  const ext = getFileExtension(file.name);
  let content: string;

  switch (ext) {
    case 'docx':
      content = await parseDocx(file);
      break;
    case 'txt':
    case 'md':
    case 'csv':
      content = await parseTextFile(file);
      break;
    default:
      // 回退：尝试按文本读取
      content = await parseTextFile(file);
      break;
  }

  content = sanitizeDocumentContent(content);

  // 截断过长的内容
  if (content.length > MAX_CONTENT_LENGTH) {
    content = content.slice(0, MAX_CONTENT_LENGTH) + '\n\n[内容已截断，原文共 ' + content.length + ' 字符]';
  }

  return {
    content: content.trim(),
    filename: normalizeDocumentFilename(file.name),
    type: ext,
    size: file.size,
  };
}

/**
 * 获取文件上传 accept 属性值
 */
export function getDocumentAcceptTypes(): string {
  return '.txt,.md,.csv,.docx';
}

/**
 * 格式化文件大小
 */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * 将解析后的文档内容格式化为消息前缀
 */
export function formatDocumentAsContext(docs: ParsedDocument[]): string {
  if (docs.length === 0) return '';

  return docs
    .map((doc) => {
      const filename = normalizeDocumentFilename(doc.filename);
      const content = sanitizeDocumentContent(doc.content);
      return `【附件：${filename}】\n${content}`;
    })
    .join('\n\n---\n\n');
}
