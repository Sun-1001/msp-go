import { format, type Locale } from 'date-fns';

interface FormatDateOptions {
  locale?: Locale;
  fallback?: string;
}

export function formatDateOrFallback(
  value: string | number | Date | null | undefined,
  pattern: string,
  options: FormatDateOptions = {}
): string {
  const fallback = options.fallback ?? '-';
  if (value === null || value === undefined) {
    return fallback;
  }
  if (typeof value === 'string' && value.trim() === '') {
    return fallback;
  }

  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return fallback;
  }

  try {
    return format(date, pattern, options.locale ? { locale: options.locale } : undefined);
  } catch {
    return fallback;
  }
}
