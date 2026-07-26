import { format, type Locale } from 'date-fns';

const beijingMonthDayTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  month: 'numeric',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
});

function formatBeijingMonthDayTime(date: Date): string {
  const parts = new Map(
    beijingMonthDayTimeFormatter.formatToParts(date).map(({ type, value }) => [type, value])
  );
  return `${parts.get('month') ?? ''}月${parts.get('day') ?? ''}日 ${parts.get('hour') ?? ''}:${parts.get('minute') ?? ''}`;
}

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

export function formatRelativeTime(value: string | number | Date | null | undefined): string {
  const fallback = '-';
  if (value === null || value === undefined) return fallback;
  if (typeof value === 'string' && value.trim() === '') return fallback;

  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;

  const now = Date.now();
  const diffMs = now - date.getTime();
  if (diffMs < 0) return formatBeijingMonthDayTime(date);

  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return '刚刚';
  if (diffMin < 60) return `${diffMin}分钟前`;

  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}小时前`;

  const diffDays = Math.floor(diffHours / 24);
  if (diffDays === 1) return '昨天';
  if (diffDays < 7) return `${diffDays}天前`;

  return formatBeijingMonthDayTime(date);
}
