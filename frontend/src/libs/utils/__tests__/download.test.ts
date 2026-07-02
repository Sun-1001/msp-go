import { beforeEach, describe, expect, it, vi } from 'vitest';
import { downloadBlob, sanitizeDownloadFilename } from '@/libs/utils/download';

describe('download utilities', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sanitizes unsafe download filenames', () => {
    expect(sanitizeDownloadFilename(' ../secret\\report?.csv ')).toBe('_secret_report_.csv');
    expect(sanitizeDownloadFilename('CON.txt')).toBe('_CON.txt');
    expect(sanitizeDownloadFilename('safe 文件.csv')).toBe('safe 文件.csv');
    expect(sanitizeDownloadFilename('\u202etxt.exe')).toBe('_txt.exe');
    expect(sanitizeDownloadFilename('', 'fallback.csv')).toBe('fallback.csv');
  });

  it('downloads a blob and always revokes the object URL', () => {
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:test');
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const filename = downloadBlob(new Blob(['content']), '../unsafe:name.csv', 'fallback.csv');

    expect(filename).toBe('_unsafe_name.csv');
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test');
  });
});
