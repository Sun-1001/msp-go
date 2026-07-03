import { describe, expect, it } from 'vitest';
import {
  normalizeSafeExternalUrl,
  normalizeSafeHttpUrl,
  normalizeSafeImageAttachmentUrl,
  normalizeSafeMailtoUrl,
} from '@/libs/utils/safeUrl';

describe('safe URL utilities', () => {
  it('normalizes safe external URLs and bare hosts', () => {
    expect(normalizeSafeExternalUrl(' example.com/path ')).toBe('https://example.com/path');
    expect(normalizeSafeExternalUrl('https://example.com/path')).toBe('https://example.com/path');
    expect(normalizeSafeExternalUrl('mailto:teacher@example.com')).toBe('mailto:teacher@example.com');
  });

  it('rejects unsafe external URLs', () => {
    const cases = [
      'javascript:alert(1)',
      'data:text/html,<script>alert(1)</script>',
      'https://user:pass@example.com/path',
      '/uploads/images/file.png',
      '//example.com/path',
      'https://example.com/line\nbreak',
    ];

    for (const value of cases) {
      expect(normalizeSafeExternalUrl(value)).toBeNull();
    }
  });

  it('normalizes only http URLs for resource links', () => {
    expect(normalizeSafeHttpUrl(' example.com:8443/path ')).toBe(
      'https://example.com:8443/path'
    );
    expect(normalizeSafeHttpUrl('https://example.com/path')).toBe('https://example.com/path');

    const cases = [
      'mailto:teacher@example.com',
      'javascript:alert(1)',
      'https://user:pass@example.com/path',
      '/uploads/documents/file.pdf',
      '//example.com/path',
    ];

    for (const value of cases) {
      expect(normalizeSafeHttpUrl(value)).toBeNull();
    }
  });

  it('allows only uploaded image attachment paths', () => {
    expect(normalizeSafeImageAttachmentUrl('/uploads/images/file.png')).toBe(
      'http://localhost:3000/uploads/images/file.png'
    );

    const cases = [
      'https://example.com/image.png',
      '/uploads/documents/file.pdf',
      '/uploads/images/../secret.png',
      '/uploads/images/%2e%2e/secret.png',
      '/uploads/images/file.png?download=1',
    ];

    for (const value of cases) {
      expect(normalizeSafeImageAttachmentUrl(value)).toBeNull();
    }
  });

  it('normalizes plain email addresses into safe mailto URLs', () => {
    expect(normalizeSafeMailtoUrl(' teacher.name+math@example.edu ')).toBe(
      'mailto:teacher.name+math@example.edu'
    );

    const cases = [
      'mailto:teacher@example.edu',
      'teacher@example.edu?subject=hello',
      'teacher@example.edu&body=token',
      'teacher@example.edu\nbcc:other@example.edu',
      'teacher@localhost',
      'https://example.edu',
    ];

    for (const value of cases) {
      expect(normalizeSafeMailtoUrl(value)).toBeNull();
    }
  });
});
