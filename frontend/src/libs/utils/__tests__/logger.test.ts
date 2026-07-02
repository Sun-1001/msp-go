import { describe, expect, it } from 'vitest';
import { normalizeRemoteLogEndpoint, sanitizeLogData } from '../logger';

describe('sanitizeLogData', () => {
  it('redacts sensitive key variants recursively', () => {
    const result = sanitizeLogData({
      api_key: 'sk-secret',
      refreshToken: 'refresh-token',
      nested: {
        xCsrfToken: 'csrf-token',
        credentials: 'cookie=value',
      },
      keep: 'visible',
    });

    expect(result).toEqual({
      api_key: '[REDACTED]',
      refreshToken: '[REDACTED]',
      nested: {
        xCsrfToken: '[REDACTED]',
        credentials: '[REDACTED]',
      },
      keep: 'visible',
    });
  });

  it('redacts bearer tokens, JWTs, and sensitive string values', () => {
    const jwt = 'aaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbb.cccccccccc';
    const result = sanitizeLogData(
      `Authorization: Bearer secret-token-value url=/api?token=abc123&safe=ok api_key=sk-test password: hunter2 jwt=${jwt}`
    );

    expect(result).toBe(
      'Authorization: Bearer [REDACTED] url=/api?token=[REDACTED]&safe=ok api_key=[REDACTED] password: [REDACTED] jwt=[REDACTED]'
    );
  });

  it('handles circular references without throwing', () => {
    const data: Record<string, unknown> = { name: 'request' };
    data.self = data;

    expect(sanitizeLogData(data)).toEqual({
      name: 'request',
      self: '[Circular]',
    });
  });

  it('keeps production error stack out of sanitized data', () => {
    const error = new Error('failed with token=abc123');
    error.stack = 'Error: failed\nAuthorization: Bearer secret-token-value';

    expect(sanitizeLogData(error)).toEqual({
      name: 'Error',
      message: 'failed with token=[REDACTED]',
      stack: undefined,
    });
    expect(sanitizeLogData(error, { includeStack: true })).toEqual({
      name: 'Error',
      message: 'failed with token=[REDACTED]',
      stack: 'Error: failed\nAuthorization: Bearer [REDACTED]',
    });
  });
});

describe('normalizeRemoteLogEndpoint', () => {
  it('allows only same-origin API paths for remote logging', () => {
    expect(normalizeRemoteLogEndpoint('/api/v1/browser-logs')).toBe('/api/v1/browser-logs');
    expect(normalizeRemoteLogEndpoint('/api/v1/browser-logs?level=warn')).toBe(
      '/api/v1/browser-logs?level=warn'
    );
  });

  it('rejects empty, external, protocol-relative, and malformed endpoints', () => {
    expect(normalizeRemoteLogEndpoint('')).toBeUndefined();
    expect(normalizeRemoteLogEndpoint('https://example.com/api/v1/logs')).toBeUndefined();
    expect(normalizeRemoteLogEndpoint('//example.com/api/v1/logs')).toBeUndefined();
    expect(normalizeRemoteLogEndpoint('/logs')).toBeUndefined();
    expect(normalizeRemoteLogEndpoint('/api/v1/logs\\evil')).toBeUndefined();
    expect(normalizeRemoteLogEndpoint('/api/v1/logs bad')).toBeUndefined();
  });
});
