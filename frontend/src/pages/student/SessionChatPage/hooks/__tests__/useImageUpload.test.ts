import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useImageUpload } from '../useImageUpload';

vi.mock('@/modules/upload/services/uploadService', () => ({
  uploadService: {
    validateImageFile: vi.fn(() => ({ valid: true })),
  },
}));

describe('useImageUpload', () => {
  const createObjectURL = vi.fn((file: File) => `blob:${file.name}`);
  const revokeObjectURL = vi.fn();

  beforeEach(() => {
    createObjectURL.mockClear();
    revokeObjectURL.mockClear();
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    });
  });

  it('keeps existing preview URLs live while appending and revokes removed URLs', () => {
    const { result, unmount } = renderHook(() => useImageUpload());
    const first = new File(['first'], 'first.png', { type: 'image/png' });
    const second = new File(['second'], 'second.png', { type: 'image/png' });

    act(() => {
      result.current.handleImageSelect({ target: { files: [first] } } as never);
    });
    expect(result.current.previewUrls).toEqual(['blob:first.png']);

    act(() => {
      result.current.handleImageSelect({ target: { files: [second] } } as never);
    });
    expect(result.current.previewUrls).toEqual(['blob:first.png', 'blob:second.png']);
    expect(revokeObjectURL).not.toHaveBeenCalledWith('blob:first.png');

    act(() => {
      result.current.handleRemoveImage(0);
    });
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:first.png');
    expect(result.current.previewUrls).toEqual(['blob:second.png']);

    unmount();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:second.png');
  });
});
