import axios from 'axios';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiClient, waitForRetryDelay } from './apiClient';

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

describe('waitForRetryDelay', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('resolves only after the configured delay', async () => {
    vi.useFakeTimers();
    let resolved = false;
    const pending = waitForRetryDelay(1_000).then(() => {
      resolved = true;
    });

    await vi.advanceTimersByTimeAsync(999);
    expect(resolved).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    await pending;
    expect(resolved).toBe(true);
  });

  it('cancels the delay and releases its timer when the request is aborted', async () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    const pending = waitForRetryDelay(60_000, controller.signal).catch((error: unknown) => error);

    controller.abort();
    const error = await pending;

    expect(axios.isCancel(error)).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });
});
