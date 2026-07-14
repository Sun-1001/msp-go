import { beforeEach, describe, expect, it, vi } from 'vitest';
import { passwordResetService } from './passwordResetService';

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock('@/libs/http/apiClient', () => ({
  apiClient: apiClientMock,
}));

describe('passwordResetService polling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('forwards the abort signal when loading the pending count', async () => {
    const controller = new AbortController();
    apiClientMock.get.mockResolvedValue({ data: { pending_count: 3 } });

    await expect(passwordResetService.getPendingCount(controller.signal)).resolves.toEqual({ pending_count: 3 });
    expect(apiClientMock.get).toHaveBeenCalledWith('/admin/inbox/pending-count', { signal: controller.signal });
  });
});
