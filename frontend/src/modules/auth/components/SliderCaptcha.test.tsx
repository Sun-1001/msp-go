import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SliderCaptcha } from './SliderCaptcha';

const mocks = vi.hoisted(() => ({
  getLoginCaptcha: vi.fn(),
  verifyLoginCaptcha: vi.fn(),
}));

vi.mock('@/modules/auth/services/authService', () => ({
  authService: mocks,
}));

const challenge = {
  captcha_id: 'challenge-1',
  background_image: 'data:image/png;base64,background',
  piece_image: 'data:image/png;base64,piece',
  width: 320,
  height: 160,
  piece_width: 48,
  piece_height: 48,
  piece_y: 54,
  expires_in: 120,
};

describe('SliderCaptcha', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getLoginCaptcha.mockResolvedValue(challenge);
  });

  it('loads a stable puzzle and verifies keyboard-selected position', async () => {
    const onTokenChange = vi.fn();
    mocks.verifyLoginCaptcha.mockResolvedValue({ captcha_token: 'proof-1', expires_in: 120 });
    render(<SliderCaptcha onTokenChange={onTokenChange} />);

    const slider = await screen.findByRole('slider', { name: '滑块验证码' });
    await screen.findByText('请完成安全验证');
    expect(slider).toHaveAttribute('aria-valuemax', '272');

    fireEvent.keyDown(slider, { key: 'End' });
    expect(slider).toHaveAttribute('aria-valuenow', '272');
    fireEvent.keyDown(slider, { key: 'Enter' });

    await waitFor(() => expect(mocks.verifyLoginCaptcha).toHaveBeenCalledWith('challenge-1', 272));
    expect(await screen.findByText('验证通过')).toBeInTheDocument();
    expect(onTokenChange).toHaveBeenLastCalledWith('proof-1');
  });

  it('maps pointer travel to source-image pixels', async () => {
    mocks.verifyLoginCaptcha.mockResolvedValue({ captcha_token: 'proof-2', expires_in: 120 });
    render(<SliderCaptcha onTokenChange={vi.fn()} />);
    const slider = await screen.findByRole('slider', { name: '滑块验证码' });
    await screen.findByText('请完成安全验证');
    vi.spyOn(slider, 'getBoundingClientRect').mockReturnValue({
      width: 320,
      height: 44,
      top: 0,
      right: 320,
      bottom: 44,
      left: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerDown(slider, { pointerId: 1, clientX: 0 });
    fireEvent.pointerMove(slider, { pointerId: 1, clientX: 138 });
    fireEvent.pointerUp(slider, { pointerId: 1, clientX: 138 });

    await waitFor(() => expect(mocks.verifyLoginCaptcha).toHaveBeenCalledWith('challenge-1', 136));
  });

  it('shows verification errors and allows an immediate manual refresh', async () => {
    mocks.verifyLoginCaptcha.mockRejectedValue(new Error('位置不正确'));
    render(<SliderCaptcha onTokenChange={vi.fn()} />);
    const slider = await screen.findByRole('slider', { name: '滑块验证码' });
    await screen.findByText('请完成安全验证');

    fireEvent.keyDown(slider, { key: 'Enter' });
    await waitFor(() => expect(mocks.verifyLoginCaptcha).toHaveBeenCalledOnce());
    expect(screen.getByText('位置不正确')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '刷新验证码' }));

    await waitFor(() => expect(mocks.getLoginCaptcha).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('请完成安全验证')).toBeInTheDocument();
  });

  it('keeps login blocked when challenge loading fails', async () => {
    const onTokenChange = vi.fn();
    mocks.getLoginCaptcha.mockRejectedValue(new Error('网络不可用'));
    render(<SliderCaptcha onTokenChange={onTokenChange} />);

    expect(await screen.findByText('网络不可用')).toBeInTheDocument();
    expect(onTokenChange).toHaveBeenCalledWith(null);
    expect(screen.getByTestId('slider-captcha')).toBeInTheDocument();
  });
});
