import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { formatDateOrFallback } from '@/libs/utils/dateFormat';

describe('formatDateOrFallback', () => {
  beforeEach(() => {
    vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('formats valid date inputs', () => {
    expect(formatDateOrFallback('2026-07-03T09:30:00+08:00', 'yyyy-MM-dd')).toBe(
      '2026-07-03'
    );
    expect(formatDateOrFallback(new Date('2026-07-03T09:30:00+08:00'), 'yyyy')).toBe(
      '2026'
    );
  });

  it('returns the fallback for empty or invalid date values', () => {
    expect(formatDateOrFallback('', 'yyyy-MM-dd', { fallback: '未知' })).toBe('未知');
    expect(formatDateOrFallback('not-a-date', 'yyyy-MM-dd', { fallback: '未知' })).toBe(
      '未知'
    );
    expect(formatDateOrFallback(new Date('not-a-date'), 'yyyy-MM-dd')).toBe('-');
  });

  it('returns the fallback when date-fns rejects the format pattern', () => {
    expect(formatDateOrFallback('2026-07-03', 'YYYY-MM-dd', { fallback: '未知' })).toBe(
      '未知'
    );
  });
});
