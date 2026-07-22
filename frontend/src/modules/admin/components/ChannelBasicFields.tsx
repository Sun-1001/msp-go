import React from 'react';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { cn } from '@/libs/utils/cn';
import { PROVIDER_PRESETS } from '../constants/providerPresets';
import { ChannelProviderIcon } from './ChannelProviderIcon';

interface ChannelBasicFieldsProps {
  isActive: boolean;
  isEditMode: boolean;
  name: string;
  onActiveChange: (value: boolean) => void;
  onNameChange: (value: string) => void;
  onProviderTypeChange: (value: string) => void;
  providerType: string;
}

export const ChannelBasicFields: React.FC<ChannelBasicFieldsProps> = ({
  isActive,
  isEditMode,
  name,
  onActiveChange,
  onNameChange,
  onProviderTypeChange,
  providerType,
}) => (
  <div className="space-y-5">
    <div className="grid gap-5 sm:grid-cols-2">
      <div className="min-w-0">
        <label className="mb-2 block text-sm font-medium text-surface-900 dark:text-surface-100">
          类型 <span className="text-red-500">*</span>
        </label>
        <div className="relative">
          <span className="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-surface-700 dark:text-surface-200">
            <ChannelProviderIcon code={providerType} className="h-4 w-4" />
          </span>
          <Select
            value={providerType}
            onChange={onProviderTypeChange}
            disabled={isEditMode}
            className="h-11 w-full pl-10"
            aria-label="渠道类型"
            options={PROVIDER_PRESETS.map((preset) => ({
              value: preset.code,
              label: preset.name,
            }))}
          />
        </div>
      </div>

      <div className="min-w-0">
        <label className="mb-2 block text-sm font-medium text-surface-900 dark:text-surface-100">
          名称 <span className="text-red-500">*</span>
        </label>
        <Input
          value={name}
          onChange={(event) => onNameChange(event.target.value)}
          placeholder="例如，OpenAI GPT-4 生产环境"
          className="h-11"
          maxLength={100}
          autoComplete="off"
        />
      </div>
    </div>

    <div className="flex min-h-16 items-center justify-between gap-4 border-y border-surface-200 py-3 dark:border-surface-700">
      <div>
        <div className="text-sm font-medium text-surface-900 dark:text-surface-100">已启用</div>
        <div className="mt-0.5 text-xs text-surface-500 dark:text-surface-400">
          启用或禁用此渠道
        </div>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={isActive}
        aria-label="启用渠道"
        onClick={() => onActiveChange(!isActive)}
        className={cn(
          'relative h-7 w-12 shrink-0 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2',
          isActive ? 'bg-primary-500' : 'bg-surface-300 dark:bg-surface-600'
        )}
      >
        <span
          className={cn(
            'absolute left-0.5 top-0.5 h-6 w-6 rounded-full bg-white shadow-sm transition-transform',
            isActive ? 'translate-x-5' : 'translate-x-0'
          )}
        />
      </button>
    </div>
  </div>
);
