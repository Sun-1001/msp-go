import { beforeEach, describe, expect, it, vi } from 'vitest';
import { authService } from './authService';

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
  delete: vi.fn(),
}));

vi.mock('@/libs/http/apiClient', () => ({
  apiClient: apiClientMock,
}));

vi.mock('@/libs/auth/tokenStorage', () => ({
  authTokenStorage: {
    clear: vi.fn(),
  },
}));

describe('authService email verification boundary', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('rejects bindEmail locally while the backend route is unavailable', async () => {
    await expect(authService.bindEmail('alice@example.com')).rejects.toThrow('邮箱绑定与验证功能暂未接入');

    expect(apiClientMock.post).not.toHaveBeenCalled();
    expect(apiClientMock.get).not.toHaveBeenCalled();
  });

});
