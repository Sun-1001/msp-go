import { hasAsciiControlCharacters } from '@/libs/utils/controlCharacters';

const LOCAL_IMAGE_PATH_PATTERN = /^\/uploads\/images\/[A-Za-z0-9._~!$&'()*+,;=:@/-]+$/;
const EMAIL_ADDRESS_PATTERN = /^[A-Za-z0-9.!#$&'*+/=?^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$/;
const EXTERNAL_URL_PROTOCOLS = new Set(['http:', 'https:', 'mailto:']);
const HTTP_URL_PROTOCOLS = new Set(['http:', 'https:']);

export function normalizeSafeExternalUrl(rawUrl: string | null | undefined): string | null {
  return normalizeSafeUrl(rawUrl, EXTERNAL_URL_PROTOCOLS);
}

export function normalizeSafeHttpUrl(rawUrl: string | null | undefined): string | null {
  return normalizeSafeUrl(rawUrl, HTTP_URL_PROTOCOLS);
}

export function hasUnsafeUrlCharacters(value: string): boolean {
  return hasAsciiControlCharacters(value) || /\s/.test(value) || value.includes('\\');
}

export function normalizeSafeMailtoUrl(rawEmail: string | null | undefined): string | null {
  const value = rawEmail?.trim();
  if (!value || value.length > 254 || hasUnsafeUrlCharacters(value) || /[?&#,;:]/.test(value)) {
    return null;
  }
  if (!EMAIL_ADDRESS_PATTERN.test(value)) {
    return null;
  }
  return `mailto:${value}`;
}

function normalizeSafeUrl(rawUrl: string | null | undefined, allowedProtocols: Set<string>): string | null {
  const value = rawUrl?.trim();
  if (!value || hasUnsafeUrlCharacters(value)) {
    return null;
  }
  const hasScheme = hasUrlScheme(value);
  if (value.startsWith('/') || value.startsWith('//') || (!hasScheme && hasNonPortColonBeforePath(value))) {
    return null;
  }

  try {
    const parsed = new URL(hasScheme ? value : 'https://' + value);
    if (!allowedProtocols.has(parsed.protocol)) {
      return null;
    }
    if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && (parsed.username || parsed.password)) {
      return null;
    }
    return parsed.href;
  } catch {
    return null;
  }
}

export function normalizeSafeImageAttachmentUrl(rawUrl: string | null | undefined): string | null {
  const value = rawUrl?.trim();
  if (!value || hasUnsafeUrlCharacters(value)) {
    return null;
  }
  if (!LOCAL_IMAGE_PATH_PATTERN.test(value) || value.includes('%')) {
    return null;
  }
  if (value.split('/').some((part) => part === '..' || part === '.')) {
    return null;
  }
  try {
    return new URL(value, window.location.origin).href;
  } catch {
    return null;
  }
}

function hasUrlScheme(value: string): boolean {
  return /^[A-Za-z][A-Za-z0-9+.-]*:\/\//.test(value) || /^mailto:/i.test(value);
}

function hasNonPortColonBeforePath(value: string): boolean {
  if (/^mailto:/i.test(value)) {
    return false;
  }
  const match = /[:/?#]/.exec(value);
  if (!match || match[0] !== ':') {
    return false;
  }
  const rest = value.slice(match.index + 1);
  const port = /^\d+/.exec(rest)?.[0] || '';
  if (!port) {
    return true;
  }
  const next = rest[port.length];
  return next !== undefined && next !== '/' && next !== '?' && next !== '#';
}
