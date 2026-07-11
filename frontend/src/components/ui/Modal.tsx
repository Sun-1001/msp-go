import React, { useEffect, useId } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '../../libs/utils/cn';
import { animationCombos } from '../../libs/animations';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  showHeader?: boolean;
  ariaLabel?: string;
}

let bodyScrollLockCount = 0;
let bodyOverflowBeforeLock = '';

function acquireBodyScrollLock() {
  if (bodyScrollLockCount === 0) {
    bodyOverflowBeforeLock = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }

  bodyScrollLockCount += 1;
  let released = false;

  return () => {
    if (released) return;
    released = true;
    bodyScrollLockCount -= 1;

    if (bodyScrollLockCount === 0) {
      document.body.style.overflow = bodyOverflowBeforeLock;
      bodyOverflowBeforeLock = '';
    }
  };
}

export const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  title,
  children,
  className,
  showHeader = true,
  ariaLabel,
}) => {
  const titleId = useId();
  const hasVisibleTitle = showHeader && Boolean(title);

  // Prevent scrolling when modal is open
  useEffect(() => {
    if (!isOpen) return;

    return acquireBodyScrollLock();
  }, [isOpen]);

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return createPortal(
    <div className="fixed inset-0 z-100 flex items-center justify-center overflow-y-auto overflow-x-hidden p-4">
      {/* Backdrop with gradient */}
      <div
        className="absolute inset-0 bg-surface-900/60 backdrop-blur-md dark:bg-surface-950/80 animate-fade-in motion-reduce:animate-none"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Decorative background elements */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-primary-500/10 rounded-full blur-[100px]" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-secondary-500/10 rounded-full blur-[100px]" />
      </div>

      {/* Modal Content */}
      <div
        className={cn(
          "relative w-full max-w-md transform rounded-3xl bg-white p-8 text-left shadow-2xl border border-surface-100",
          "dark:bg-surface-900 dark:border-surface-700",
          "animate-fade-in animate-scale-in motion-reduce:animate-none",
          className
        )}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={hasVisibleTitle ? undefined : ariaLabel || '弹窗'}
        aria-labelledby={hasVisibleTitle ? titleId : undefined}
      >
        {/* Close button - always visible */}
        <button
          type="button"
          className={cn(
            "absolute top-4 right-4 rounded-full p-2 hover:bg-surface-100 text-surface-400 hover:text-surface-600 focus:outline-none focus:ring-2 focus:ring-primary-500/20 z-10 dark:hover:bg-surface-800 dark:text-surface-500 dark:hover:text-surface-300",
            animationCombos.buttonHover
          )}
          onClick={onClose}
          aria-label="关闭弹窗"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Header */}
        {showHeader && title && (
          <div className="mb-6 pr-8">
            <h3 id={titleId} className="text-xl font-semibold leading-6 text-surface-900 dark:text-surface-100">
              {title}
            </h3>
          </div>
        )}

        {/* Content */}
        <div>{children}</div>

        {/* Subtle bottom gradient decoration */}
        <div className="absolute bottom-0 left-0 right-0 h-32 bg-linear-to-t from-surface-50/50 to-transparent rounded-b-3xl pointer-events-none dark:from-surface-800/50" />
      </div>
    </div>,
    document.body
  );
};
