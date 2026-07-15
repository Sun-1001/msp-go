import { useState } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AnswerImageInput } from './AnswerImageInput';

const mocks = vi.hoisted(() => ({
  validateImageFile: vi.fn(),
}));

vi.mock('@/modules/upload/services/uploadService', () => ({
  uploadService: {
    validateImageFile: mocks.validateImageFile,
  },
}));

const ControlledInput = ({ disabled = false }: { disabled?: boolean }) => {
  const [file, setFile] = useState<File | null>(null);
  return <AnswerImageInput file={file} disabled={disabled} onChange={setFile} />;
};

describe('AnswerImageInput', () => {
  const createObjectURL = vi.fn((file: File) => `blob:${file.name}`);
  const revokeObjectURL = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.validateImageFile.mockReturnValue({ valid: true });
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    });
  });

  it('selects one image, replaces its preview, and releases object URLs', () => {
    const first = new File(['first'], 'first.png', { type: 'image/png' });
    const second = new File(['second'], 'second.jpg', { type: 'image/jpeg' });
    const { unmount } = render(<ControlledInput />);
    const input = screen.getByLabelText('选择答案图片');

    fireEvent.change(input, { target: { files: [first] } });
    expect(screen.getByRole('img', { name: `答案图片预览：${first.name}` })).toHaveAttribute(
      'src',
      'blob:first.png'
    );

    fireEvent.change(input, { target: { files: [second] } });
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:first.png');
    expect(screen.getByRole('img', { name: `答案图片预览：${second.name}` })).toHaveAttribute(
      'src',
      'blob:second.jpg'
    );

    fireEvent.click(screen.getByRole('button', { name: '移除答案图片' }));
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:second.jpg');
    expect(screen.getByRole('button', { name: '上传答案图片' })).toBeEnabled();

    fireEvent.change(input, { target: { files: [first] } });
    unmount();
    expect(revokeObjectURL).toHaveBeenLastCalledWith('blob:first.png');
  });

  it('keeps the current state unchanged when validation rejects a file', () => {
    mocks.validateImageFile.mockReturnValue({ valid: false, error: '不支持的图片' });
    render(<ControlledInput />);

    fireEvent.change(screen.getByLabelText('选择答案图片'), {
      target: { files: [new File(['image'], 'answer.png', { type: 'image/png' })] },
    });

    expect(screen.getByRole('alert')).toHaveTextContent('不支持的图片');
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('accepts only answer image formats that the OCR runtime can decode', () => {
    render(<ControlledInput />);

    const input = screen.getByLabelText('选择答案图片');
    expect(input).toHaveAttribute('accept', 'image/jpeg,image/png,image/gif');
    fireEvent.change(input, {
      target: { files: [new File(['webp'], 'answer.webp', { type: 'image/webp' })] },
    });

    expect(screen.getByRole('alert')).toHaveTextContent('答案图片仅支持 JPEG、PNG 或 GIF 格式');
    expect(mocks.validateImageFile).not.toHaveBeenCalled();
  });

  it('disables both the picker and visible command while busy', () => {
    render(<ControlledInput disabled />);

    expect(screen.getByLabelText('选择答案图片')).toBeDisabled();
    expect(screen.getByRole('button', { name: '上传答案图片' })).toBeDisabled();
  });
});
