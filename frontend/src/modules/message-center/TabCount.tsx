import React from 'react';

interface TabCountProps {
  count: number;
}

export const TabCount: React.FC<TabCountProps> = ({ count }) => {
  if (count <= 0) return null;
  return (
    <span className="absolute -right-1 -top-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[9px] font-semibold leading-none text-white">
      {count > 99 ? '99+' : count}
    </span>
  );
};
