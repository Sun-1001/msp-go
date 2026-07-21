import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clearLegacyXidianStorage } from './credentialStorage';

const CREDENTIAL_KEY = 'xidian_cred';
const CLASSTABLE_CACHE_KEY = 'xidian_classtable_cache';

describe('credentialStorage legacy cleanup', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('clears legacy credentials and academic caches', () => {
    localStorage.setItem(CREDENTIAL_KEY, 'legacy');
    localStorage.setItem(CLASSTABLE_CACHE_KEY, '{"courses":[]}');

    clearLegacyXidianStorage();

    expect(localStorage.getItem(CREDENTIAL_KEY)).toBeNull();
    expect(localStorage.getItem(CLASSTABLE_CACHE_KEY)).toBeNull();
  });

  it('does not throw when browser storage blocks cleanup', () => {
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('storage blocked');
    });

    expect(() => clearLegacyXidianStorage()).not.toThrow();
  });
});
