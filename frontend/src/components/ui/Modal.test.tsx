import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Modal } from './Modal';

describe('Modal', () => {
  afterEach(() => {
    document.body.style.overflow = '';
  });

  it('locks body scrolling while open and restores the previous value', () => {
    document.body.style.overflow = 'auto';
    const { rerender } = render(
      <Modal isOpen onClose={vi.fn()}>
        内容
      </Modal>
    );

    expect(document.body.style.overflow).toBe('hidden');

    rerender(
      <Modal isOpen={false} onClose={vi.fn()}>
        内容
      </Modal>
    );

    expect(document.body.style.overflow).toBe('auto');
  });

  it('keeps the parent scroll lock when a nested modal closes', () => {
    const { rerender } = render(
      <>
        <Modal isOpen onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen={false} onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    expect(document.body.style.overflow).toBe('hidden');

    rerender(
      <>
        <Modal isOpen onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    rerender(
      <>
        <Modal isOpen onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen={false} onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    expect(document.body.style.overflow).toBe('hidden');
  });

  it('keeps scrolling locked when the parent closes before its nested modal', () => {
    document.body.style.overflow = 'auto';
    const { rerender } = render(
      <>
        <Modal isOpen onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    rerender(
      <>
        <Modal isOpen={false} onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    expect(document.body.style.overflow).toBe('hidden');

    rerender(
      <>
        <Modal isOpen={false} onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen={false} onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    expect(document.body.style.overflow).toBe('auto');
  });

  it('restores scrolling when multiple open modals unmount together', () => {
    document.body.style.overflow = 'auto';
    const { unmount } = render(
      <>
        <Modal isOpen onClose={vi.fn()}>父弹窗</Modal>
        <Modal isOpen onClose={vi.fn()}>子弹窗</Modal>
      </>
    );

    expect(document.body.style.overflow).toBe('hidden');

    unmount();

    expect(document.body.style.overflow).toBe('auto');
  });

  it('keeps close-button and Escape behavior intact', () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen onClose={onClose} title="测试弹窗">
        内容
      </Modal>
    );

    fireEvent.click(screen.getByRole('button', { name: '关闭弹窗' }));
    fireEvent.keyDown(window, { key: 'Escape' });

    expect(onClose).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('dialog', { name: '测试弹窗' })).toHaveAttribute('aria-modal', 'true');
  });

  it('uses an explicit accessible name when the visual header is hidden', () => {
    render(
      <Modal isOpen onClose={vi.fn()} showHeader={false} ariaLabel="登录">
        内容
      </Modal>
    );

    expect(screen.getByRole('dialog', { name: '登录' })).toBeInTheDocument();
  });
});
