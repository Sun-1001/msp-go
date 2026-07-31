import type { LucideIcon } from 'lucide-react';
import { TabsTrigger } from '@/components/ui/Tabs';
import { TabCount } from '@/modules/message-center/TabCount';

interface MessageCenterSideTabProps {
  value: string;
  label: string;
  count: number;
  icon: LucideIcon;
}

export function MessageCenterSideTab({ value, label, count, icon: Icon }: MessageCenterSideTabProps) {
  return (
    <TabsTrigger
      value={value}
      aria-label={label}
      className="group relative h-10 w-10 rounded-lg p-0 data-[state=active]:text-primary-600"
    >
      <Icon className="h-5 w-5" />
      <TabCount count={count} />
      <span
        role="tooltip"
        className="pointer-events-none absolute left-full top-1/2 z-30 ml-2 -translate-y-1/2 whitespace-nowrap rounded-md bg-surface-900 px-2 py-1 text-xs font-medium text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100 dark:bg-surface-100 dark:text-surface-900"
      >
        {label}
      </span>
    </TabsTrigger>
  );
}
