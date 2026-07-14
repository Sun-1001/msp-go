import { beforeEach, describe, expect, it, vi } from 'vitest';
import { authService } from './authService';

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}));

const authTokenStorageMock = vi.hoisted(() => ({
  clear: vi.fn(),
}));

vi.mock('@/libs/http/apiClient', () => ({
  apiClient: apiClientMock,
}));

vi.mock('@/libs/auth/tokenStorage', () => ({
  authTokenStorage: authTokenStorageMock,
}));

describe('authService account profile', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns the registered email from the current-user response', async () => {
    const account = {
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'student' as const,
    };
    apiClientMock.get.mockResolvedValue({ data: account });

    await expect(authService.getCurrentUser()).resolves.toEqual(account);
    expect(apiClientMock.get).toHaveBeenCalledWith('/auth/me');
  });

  it('passes the one-time captcha proof to login', async () => {
    apiClientMock.post.mockResolvedValue({
      data: {
        access_token: 'access-token',
        token_type: 'bearer',
        user: { id: 'user-1', username: 'alice', email: 'alice@example.com', role: 'student' },
      },
    });

    await authService.login({ username: 'alice', password: 'secret', role: 'student', captchaToken: 'proof-1' });

    expect(apiClientMock.post).toHaveBeenCalledWith('/auth/login', {
      username: 'alice',
      password: 'secret',
      captcha_token: 'proof-1',
    });
  });

  it('loads and verifies slider captcha challenges', async () => {
    const challenge = { captcha_id: 'challenge-1', width: 320, height: 160 };
    apiClientMock.get.mockResolvedValueOnce({ data: challenge });
    apiClientMock.post.mockResolvedValueOnce({ data: { captcha_token: 'proof-1', expires_in: 120 } });

    await expect(authService.getLoginCaptcha()).resolves.toEqual(challenge);
    await expect(authService.verifyLoginCaptcha('challenge-1', 92)).resolves.toEqual({ captcha_token: 'proof-1', expires_in: 120 });

    expect(apiClientMock.get).toHaveBeenCalledWith('/auth/captcha');
    expect(apiClientMock.post).toHaveBeenCalledWith('/auth/captcha/verify', { captcha_id: 'challenge-1', position: 92 });
  });

  it('revokes the server session before clearing the local token', async () => {
    apiClientMock.post.mockResolvedValue({ data: { message: '登出成功' } });

    await authService.logout();

    expect(apiClientMock.post).toHaveBeenCalledWith('/auth/logout');
    expect(authTokenStorageMock.clear).toHaveBeenCalledOnce();
  });

  it('still clears the local token when the logout request fails', async () => {
    apiClientMock.post.mockRejectedValue(new Error('network unavailable'));

    await expect(authService.logout()).resolves.toBeUndefined();
    expect(authTokenStorageMock.clear).toHaveBeenCalledOnce();
  });
});
