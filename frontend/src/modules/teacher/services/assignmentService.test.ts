import { describe, expect, it } from 'vitest';
import { assignmentService } from './assignmentService';

describe('assignmentService', () => {
  it('returns an explicit empty list while the Go backend has no assignment route', async () => {
    await expect(assignmentService.list({ page: 2, pageSize: 7, status: 'active' })).resolves.toEqual({
      items: [],
      total: 0,
      page: 2,
      pageSize: 7,
    });
  });

  it('returns zero stats while the Go backend has no assignment route', async () => {
    await expect(assignmentService.getStats()).resolves.toEqual({
      total: 0,
      active: 0,
      pending: 0,
    });
  });
});
