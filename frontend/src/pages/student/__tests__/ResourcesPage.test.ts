import { describe, expect, it, vi } from 'vitest';
import {
  getInitialResourceSearch,
  normalizeOpenableResourceUrl,
  openResourceUrl,
  parseLinksFromText,
} from '@/libs/utils/resourceUtils';

describe('ResourcesPage URL search params', () => {
  it('initializes resource search from the search query param', () => {
    expect(getInitialResourceSearch('?search=%E6%B4%9B%E5%BF%85%E8%BE%BE%E6%B3%95%E5%88%99')).toBe(
      '洛必达法则'
    );
  });

  it('trims empty search params', () => {
    expect(getInitialResourceSearch('?search=%20%20')).toBe('');
  });
});

describe('resource URL opening', () => {
  it('normalizes http urls, bare domains, and local uploaded resources', () => {
    expect(normalizeOpenableResourceUrl(' example.com:8443/video ')).toBe(
      'https://example.com:8443/video'
    );
    expect(normalizeOpenableResourceUrl('https://example.com/docs.pdf')).toBe(
      'https://example.com/docs.pdf'
    );
    expect(normalizeOpenableResourceUrl('/uploads/documents/file.pdf')).toBe(
      'http://localhost:3000/uploads/documents/file.pdf'
    );
  });

  it('rejects dangerous resource urls', () => {
    const cases = [
      'javascript:alert(1)',
      'data:text/html,<script>alert(1)</script>',
      'mailto:teacher@example.com',
      'https://user:pass@example.com/file.pdf',
      '/uploads/images/file.png',
      '/uploads/documents/../secret.pdf',
      '//example.com/file.pdf',
      'https://example.com/line\nbreak',
    ];

    for (const value of cases) {
      expect(normalizeOpenableResourceUrl(value)).toBeNull();
    }
  });

  it('opens normalized urls with noopener and noreferrer', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null);

    expect(openResourceUrl('example.com/file.pdf')).toBe(true);
    expect(open).toHaveBeenCalledWith(
      'https://example.com/file.pdf',
      '_blank',
      'noopener,noreferrer'
    );

    expect(openResourceUrl('javascript:alert(1)')).toBe(false);
    expect(open).toHaveBeenCalledTimes(1);
    open.mockRestore();
  });
});

describe('resource batch link parsing', () => {
  it('deduplicates normalized http links and rejects unsafe schemes', () => {
    expect(
      parseLinksFromText(`
        example.com/video
        https://example.com/video
        javascript:alert(1)
        data:text/html,<script>alert(1)</script>
        mailto:teacher@example.com
        https://user:pass@example.com/secret
      `)
    ).toEqual(['https://example.com/video']);
  });
});
