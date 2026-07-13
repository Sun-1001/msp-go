import { describe, expect, it, vi } from 'vitest';
import { apiClient } from './apiClient';

vi.mock('../utils/logger', () => ({
  logger: {
    createContextLogger: () => ({
      debug: vi.fn(),
      error: vi.fn(),
      security: vi.fn(),
      warn: vi.fn(),
    }),
  },
}));

vi.mock('./rateLimitEvents', () => ({
  emitRateLimited: vi.fn(),
}));

describe('apiClient captcha rate limits', () => {
  it('does not automatically retry captcha issuance after a 429', async () => {
    let attempts = 0;
    const adapter = vi.fn(async (config) => {
      attempts += 1;
      return Promise.reject({
        isAxiosError: true,
        message: 'rate limited',
        config,
        response: {
          status: 429,
          data: { code: 'CAPTCHA_RATE_LIMITED', detail: '请求过于频繁' },
          headers: { 'retry-after': '60' },
          config,
        },
      });
    });

    await expect(apiClient.get('/auth/captcha', { adapter })).rejects.toMatchObject({ message: 'rate limited' });

    expect(attempts).toBe(1);
    expect(adapter).toHaveBeenCalledOnce();
  });
});
