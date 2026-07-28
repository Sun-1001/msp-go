import React from 'react';
import { cn } from '@/libs/utils/cn';

interface TabCountProps {
  count: number;
  compact?: boolean;
}

export const TabCount: React.FC<TabCountProps> = ({ count, compact = false }) => {
  if (count <= 0) return null;
  return (
    <span className={cn(
      'inline-flex items-center justify-center rounded-full bg-red-500 font-semibold leading-none text-white',
      compact
        ? 'absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[9px]'
        : 'ml-2 h-5 min-w-5 px-1.5 text-xs',
    )}>
      {count > 99 ? '99+' : count}
    </span>
  );
};
