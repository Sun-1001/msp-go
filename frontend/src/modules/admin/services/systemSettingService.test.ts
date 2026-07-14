import { beforeEach, describe, expect, it, vi } from 'vitest';
import { systemSettingService, type DatabaseMonitorResponse } from './systemSettingService';

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock('@/libs/http/apiClient', () => ({
  apiClient: apiClientMock,
}));

describe('systemSettingService polling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('forwards the abort signal when loading database monitoring data', async () => {
    const controller = new AbortController();
    const monitor = {
      overview: {
        database_name: 'msp',
        database_size: '11 MB',
        postgres_version: '18.1',
        uptime: '1 day',
        active_connections: 2,
        max_connections: 100,
      },
      connection_pool: {
        pool_size: 20,
        max_overflow: 0,
        checked_out: 2,
        checked_in: 18,
        overflow: 0,
        pool_timeout: 30,
        pool_recycle: 1800,
        usage_percent: 10,
      },
      tables: [],
      health_status: 'healthy',
      checked_at: '2026-07-14T00:00:00Z',
    } satisfies DatabaseMonitorResponse;
    apiClientMock.get.mockResolvedValue({ data: monitor });

    await expect(systemSettingService.getDatabaseMonitor(controller.signal)).resolves.toBe(monitor);
    expect(apiClientMock.get).toHaveBeenCalledWith('/admin/settings/database/monitor', { signal: controller.signal });
  });
});
