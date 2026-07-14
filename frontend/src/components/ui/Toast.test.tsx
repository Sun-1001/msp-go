import { useEffect } from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ToastProvider, useToast } from './Toast';
import { animationDuration } from '../../libs/animations';

const ToastHarness = ({ onRender }: { onRender?: () => void }) => {
  const { toast } = useToast();
  useEffect(() => {
    onRender?.();
  });
  return (
    <>
      <button type="button" onClick={() => toast({ type: 'info', title: 'Test', duration: 1_000 })}>
        Add
      </button>
      <button type="button" onClick={() => toast({ type: 'info', title: 'Persistent', duration: 0 })}>
        Add persistent
      </button>
    </>
  );
};

describe('Toast timers', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('clears the pending removal timer when unmounted during the exit animation', () => {
    vi.useFakeTimers();
    const view = render(<ToastProvider><ToastHarness /></ToastProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Add' }));
    expect(screen.getByRole('alert')).toBeInTheDocument();

    act(() => vi.advanceTimersByTime(1_000));
    expect(vi.getTimerCount()).toBe(1);
    view.unmount();

    expect(vi.getTimerCount()).toBe(0);
  });

  it('removes a toast after the existing display and exit durations', () => {
    vi.useFakeTimers();
    render(<ToastProvider><ToastHarness /></ToastProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Add' }));

    act(() => vi.advanceTimersByTime(1_000 + animationDuration.normal));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not rerender action-only consumers when toast state changes', () => {
    const onRender = vi.fn();
    render(<ToastProvider><ToastHarness onRender={onRender} /></ToastProvider>);
    expect(onRender).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole('button', { name: 'Add' }));

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(onRender).toHaveBeenCalledOnce();
  });

  it('clears manual exit timers for persistent toasts on unmount', () => {
    vi.useFakeTimers();
    const view = render(<ToastProvider><ToastHarness /></ToastProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Add persistent' }));
    fireEvent.click(screen.getByRole('button', { name: '关闭通知' }));
    expect(vi.getTimerCount()).toBe(1);

    view.unmount();

    expect(vi.getTimerCount()).toBe(0);
  });
});
