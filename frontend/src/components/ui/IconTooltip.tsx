import type { ReactElement } from 'react';

import { cn } from '@/libs/utils/cn';

type IconTooltipSide = 'top' | 'bottom' | 'left' | 'right';

interface IconTooltipProps {
  label: string;
  children: ReactElement;
  side?: IconTooltipSide;
  className?: string;
}

const sideClasses: Record<IconTooltipSide, string> = {
  top: 'bottom-full left-1/2 mb-2 -translate-x-1/2',
  bottom: 'left-1/2 top-full mt-2 -translate-x-1/2',
  left: 'right-full top-1/2 mr-2 -translate-y-1/2',
  right: 'left-full top-1/2 ml-2 -translate-y-1/2',
};

export function IconTooltip({ label, children, side = 'top', className }: IconTooltipProps) {
  return (
    <span className={cn('group/tooltip relative inline-flex', className)}>
      {children}
      <span
        role="tooltip"
        className={cn(
          'pointer-events-none invisible absolute z-50 whitespace-nowrap rounded-md bg-surface-900 px-2 py-1 text-xs font-medium text-white opacity-0 shadow-lg transition-opacity duration-150 group-hover/tooltip:visible group-hover/tooltip:opacity-100 group-focus-within/tooltip:visible group-focus-within/tooltip:opacity-100 motion-reduce:transition-none dark:bg-surface-100 dark:text-surface-900',
          sideClasses[side],
        )}
      >
        {label}
      </span>
    </span>
  );
}
