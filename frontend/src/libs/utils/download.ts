const DEFAULT_DOWNLOAD_FILENAME = 'download';
const RESERVED_WINDOWS_NAMES = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i;

export function sanitizeDownloadFilename(
  rawFilename: string | null | undefined,
  fallback = DEFAULT_DOWNLOAD_FILENAME,
): string {
  const fallbackName = fallback.trim() || DEFAULT_DOWNLOAD_FILENAME;
  const raw = rawFilename?.trim();
  if (!raw) {
    return fallbackName;
  }

  let filename = raw
    .replace(/[\u0000-\u001f\u007f/\\:*?"<>|\u202a-\u202e]/g, '_')
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
