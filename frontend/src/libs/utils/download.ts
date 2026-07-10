import { replaceAsciiControlCharacters } from '@/libs/utils/controlCharacters';

const DEFAULT_DOWNLOAD_FILENAME = 'download';
const RESERVED_WINDOWS_NAMES = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i;
const MAX_BASE64_DOWNLOAD_LENGTH = 64 * 1024 * 1024;
const BASE64_PATTERN = /^[A-Za-z0-9+/]*={0,2}$/;

export function sanitizeDownloadFilename(
  rawFilename: string | null | undefined,
  fallback = DEFAULT_DOWNLOAD_FILENAME,
): string {
  const fallbackName = fallback.trim() || DEFAULT_DOWNLOAD_FILENAME;
  const raw = rawFilename?.trim();
  if (!raw) {
    return fallbackName;
  }

  let filename = replaceAsciiControlCharacters(raw, '_')
    .replace(/[/\\:*?"<>|\u202a-\u202e]/g, '_')
    .replace(/_+/g, '_')
    .replace(/\s+/g, ' ')
    .trim();

  filename = filename.replace(/^\.+_?/, '_').replace(/\.+$/, '');
  if (!filename || filename === '_' || filename === '..') {
    return fallbackName;
  }
  if (RESERVED_WINDOWS_NAMES.test(filename)) {
    filename = `_${filename}`;
  }
  if (filename.length > 180) {
    filename = filename.slice(0, 180).trim();
  }
  return filename || fallbackName;
}

export function downloadBlob(
  blob: Blob,
  filename: string | null | undefined,
  fallback = DEFAULT_DOWNLOAD_FILENAME,
): string {
  const safeFilename = sanitizeDownloadFilename(filename, fallback);
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = safeFilename;
  link.rel = 'noopener noreferrer';
  link.style.display = 'none';

  try {
    document.body.appendChild(link);
    link.click();
  } finally {
    link.remove();
    URL.revokeObjectURL(url);
  }

  return safeFilename;
}

export function base64ToBlob(
  content: string | null | undefined,
  contentType = 'application/octet-stream',
): Blob {
  const normalized = content?.replace(/\s/g, '') ?? '';
  if (
    !normalized ||
    normalized.length > MAX_BASE64_DOWNLOAD_LENGTH ||
    normalized.length % 4 === 1 ||
    !BASE64_PATTERN.test(normalized)
  ) {
    throw new Error('导出文件内容异常');
  }

  let binaryString: string;
  try {
    binaryString = atob(normalized);
  } catch {
    throw new Error('导出文件内容异常');
  }

  const bytes = new Uint8Array(binaryString.length);
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }

  return new Blob([bytes], { type: contentType });
}
