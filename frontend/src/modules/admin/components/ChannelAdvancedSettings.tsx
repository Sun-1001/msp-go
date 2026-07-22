import React from 'react';
import { Input } from '@/components/ui/Input';

interface ChannelAdvancedSettingsProps {
  description: string;
  onDescriptionChange: (value: string) => void;
  onPriorityChange: (value: number) => void;
  onWeightChange: (value: number) => void;
  priority: number;
  weight: number;
}

export const ChannelAdvancedSettings: React.FC<ChannelAdvancedSettingsProps> = ({
  description,
  onDescriptionChange,
  onPriorityChange,
  onWeightChange,
  priority,
  weight,
}) => (
  <div className="space-y-5 rounded-lg border border-surface-200 bg-surface-50/50 p-5 dark:border-surface-700 dark:bg-surface-900/40">
    <div className="grid gap-5 sm:grid-cols-2">
      <div>
        <label className="mb-2 block text-sm font-medium text-surface-900 dark:text-surface-100">
          优先级
        </label>
        <Input
          type="number"
          min={0}
          max={1000}
          step={1}
          value={priority}
          onChange={(event) => onPriorityChange(Number(event.target.value))}
		  aria-label="渠道优先级"
        />
      </div>
      <div>
        <label className="mb-2 block text-sm font-medium text-surface-900 dark:text-surface-100">
          权重
        </label>
        <Input
          type="number"
          min={1}
          max={1000}
          step={1}
          value={weight}
          onChange={(event) => onWeightChange(Number(event.target.value))}
		  aria-label="渠道权重"
        />
      </div>
    </div>
    <div>
      <label className="mb-2 block text-sm font-medium text-surface-900 dark:text-surface-100">
        渠道备注
      </label>
      <textarea
        value={description}
        onChange={(event) => onDescriptionChange(event.target.value)}
        placeholder="可选，记录用途、环境或维护说明"
        rows={4}
        maxLength={500}
        className="w-full resize-y rounded-md border border-surface-200 bg-white px-3 py-3 text-sm leading-6 text-surface-900 outline-none transition-shadow placeholder:text-surface-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-100 dark:placeholder:text-surface-500"
      />
      <div className="mt-2 text-right text-xs text-surface-400">{description.length}/500</div>
    </div>
  </div>
);
