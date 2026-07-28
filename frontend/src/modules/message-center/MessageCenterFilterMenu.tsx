import { useEffect, useId, useRef, useState } from 'react';
import { Check, ChevronDown, Funnel } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { cn } from '@/libs/utils/cn';

interface MessageCenterFilterMenuProps {
  options: readonly string[];
  value: string;
  onValueChange: (value: string) => void;
  subject?: string;
}

export function MessageCenterFilterMenu({ options, value, onValueChange, subject = '通知' }: MessageCenterFilterMenuProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  useEffect(() => {
    if (!open) return;

    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };

    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  return (
    <div ref={rootRef} className="relative shrink-0">
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setOpen((current) => !current)}
        aria-label={`筛选${subject}，当前为${value}`}
        aria-haspopup="menu"
        aria-controls={open ? menuId : undefined}
        aria-expanded={open}
        title={`筛选${subject}（当前：${value}）`}
      >
        <Funnel className="mr-1 h-4 w-4" />
        筛选
        <ChevronDown className={cn('ml-1 h-3.5 w-3.5 transition-transform', open && 'rotate-180')} />
      </Button>

      {open && (
        <div
          id={menuId}
          role="menu"
          aria-label={`${subject}筛选条件`}
          className="absolute right-0 top-full z-30 mt-2 min-w-32 overflow-hidden rounded-md border border-surface-200 bg-white p-1 shadow-xl dark:border-surface-700 dark:bg-surface-900"
        >
          {options.map((option) => (
            <button
              key={option}
              type="button"
              role="menuitemradio"
              aria-checked={value === option}
              onClick={() => {
                onValueChange(option);
                setOpen(false);
              }}
              className={cn(
                'flex w-full items-center justify-between gap-3 rounded px-3 py-2 text-left text-sm text-surface-700 hover:bg-surface-100 dark:text-surface-200 dark:hover:bg-surface-800',
                value === option && 'bg-primary-50 text-primary-700 dark:bg-primary-950/30 dark:text-primary-300',
              )}
            >
              <span>{option}</span>
              {value === option && <Check className="h-4 w-4" />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
