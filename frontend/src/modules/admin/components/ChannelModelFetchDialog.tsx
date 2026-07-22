import React, {
  useDeferredValue,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';
import { ChevronDown, Search, X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs';
import { cn } from '@/libs/utils/cn';
import {
  buildChannelModelCatalog,
  filterChannelModelCatalog,
  getInitialChannelModelSelection,
  groupChannelModels,
  resolveChannelModelSelection,
  updateChannelModelSelection,
  type ChannelModelCandidate,
  type ChannelModelCatalog,
  type ChannelModelGroup,
  type ChannelModelStatus,
  type ResolvedChannelModelSelection,
} from './channelModelCatalog';

interface ChannelModelFetchDialogProps {
  channelName: string;
  fetchedModels: string[];
  initialMapping: Record<string, string>;
  initialModels: string[];
  onClose: () => void;
  onSave: (selection: ResolvedChannelModelSelection) => void;
}

interface SelectionCheckboxProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'checked' | 'type'> {
  checked: boolean;
  indeterminate?: boolean;
}

interface ChannelModelGroupSectionProps {
  collapsed: boolean;
  group: ChannelModelGroup;
  idPrefix: string;
  onCollapsedChange: () => void;
  onSelectionChange: (models: ChannelModelCandidate[], selected: boolean) => void;
  selectedKeys: ReadonlySet<string>;
}

const tabLabels: Record<ChannelModelStatus, string> = {
  existing: '现有模型',
  new: '新模型',
  removed: '已移除模型',
};

const modelTabOrder: ChannelModelStatus[] = ['new', 'existing', 'removed'];

function getPreferredTab(catalog: ChannelModelCatalog): ChannelModelStatus {
  if (catalog.new.length) return 'new';
  if (catalog.removed.length) return 'removed';
  if (catalog.existing.length) return 'existing';
  return 'new';
}

const SelectionCheckbox: React.FC<SelectionCheckboxProps> = ({
  checked,
  className,
  indeterminate = false,
  ...props
}) => {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (inputRef.current) inputRef.current.indeterminate = indeterminate;
  }, [indeterminate]);

  return (
    <input
      ref={inputRef}
      type="checkbox"
      checked={checked}
      aria-checked={indeterminate ? 'mixed' : checked}
      className={cn(
        'h-4 w-4 shrink-0 cursor-pointer accent-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2',
        className
      )}
      {...props}
    />
  );
};

const ChannelModelGroupSection: React.FC<ChannelModelGroupSectionProps> = ({
  collapsed,
  group,
  idPrefix,
  onCollapsedChange,
  onSelectionChange,
  selectedKeys,
}) => {
  const selectedCount = group.models.filter((model) => selectedKeys.has(model.key)).length;
  const allSelected = group.models.length > 0 && selectedCount === group.models.length;
  const partiallySelected = selectedCount > 0 && !allSelected;

  return (
    <section className="overflow-hidden rounded-lg border border-surface-200 bg-white dark:border-surface-700 dark:bg-surface-900">
      <div className="flex min-h-12 items-center justify-between gap-3 px-3 py-2 sm:px-4">
        <button
          type="button"
          onClick={onCollapsedChange}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md py-1 text-left font-medium text-surface-900 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-surface-100 dark:hover:text-primary-300"
          aria-expanded={!collapsed}
        >
          <ChevronDown
            className={cn('h-4 w-4 shrink-0 transition-transform', collapsed && '-rotate-90')}
            aria-hidden="true"
          />
          <span className="truncate">{group.name} ({group.models.length})</span>
        </button>
        <div className="flex shrink-0 items-center gap-2">
          <span className="hidden text-xs text-surface-500 sm:inline dark:text-surface-400">
            {selectedCount} / {group.models.length} 已选
          </span>
          <SelectionCheckbox
            checked={allSelected}
            indeterminate={partiallySelected}
            onChange={() => onSelectionChange(group.models, !allSelected)}
            aria-label={`${allSelected ? '取消选择' : '选择'} ${group.name} 分组全部模型`}
          />
        </div>
      </div>

      {!collapsed ? (
        <div className="grid grid-cols-1 gap-x-5 gap-y-1 border-t border-surface-100 px-3 py-2 sm:grid-cols-2 sm:px-4 dark:border-surface-800">
          {group.models.map((model, index) => {
            const inputId = `${idPrefix}-${index}`;
            return (
              <label
                key={model.key}
                htmlFor={inputId}
                className="flex min-w-0 cursor-pointer items-start gap-2 rounded-md px-2 py-2 text-sm hover:bg-surface-50 dark:hover:bg-surface-800"
              >
                <SelectionCheckbox
                  id={inputId}
                  checked={selectedKeys.has(model.key)}
                  onChange={(event) => onSelectionChange([model], event.target.checked)}
                  className="mt-0.5"
                />
                <span className="min-w-0">
                  <span className="block break-all text-surface-800 dark:text-surface-100">
                    {model.logicalName}
                  </span>
                  {model.logicalName !== model.upstreamId ? (
                    <span className="mt-0.5 block break-all text-xs text-surface-500 dark:text-surface-400">
                      上游：{model.upstreamId}
                    </span>
                  ) : null}
                </span>
              </label>
            );
          })}
        </div>
      ) : null}
    </section>
  );
};

export const ChannelModelFetchDialog: React.FC<ChannelModelFetchDialogProps> = ({
  channelName,
  fetchedModels,
  initialMapping,
  initialModels,
  onClose,
  onSave,
}) => {
  const dialogRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const restoreFocusRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const descriptionId = useId();
  const modelIdPrefix = useId().replace(/:/g, '');
  const catalog = useMemo(
    () => buildChannelModelCatalog(fetchedModels, initialModels, initialMapping),
    [fetchedModels, initialMapping, initialModels]
  );
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const filteredCatalog = useMemo(
    () => filterChannelModelCatalog(catalog, deferredSearch),
    [catalog, deferredSearch]
  );
  const [activeTab, setActiveTab] = useState<ChannelModelStatus>(() => getPreferredTab(catalog));
  const [selectedKeys, setSelectedKeys] = useState<string[]>(() =>
    getInitialChannelModelSelection(catalog)
  );
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set());
  const selectedKeySet = useMemo(() => new Set(selectedKeys), [selectedKeys]);
  const displayedTab = filteredCatalog[activeTab].length > 0
    ? activeTab
    : getPreferredTab(filteredCatalog);
  const activeGroups = useMemo(
    () => groupChannelModels(filteredCatalog[displayedTab]),
    [displayedTab, filteredCatalog]
  );

  useEffect(() => {
    restoreFocusRef.current = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const frame = window.requestAnimationFrame(() => searchInputRef.current?.focus());
    return () => {
      window.cancelAnimationFrame(frame);
      document.body.style.overflow = previousOverflow;
      restoreFocusRef.current?.focus();
    };
  }, []);

  const handleDialogKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== 'Tab' || !dialogRef.current) return;
    const focusable = Array.from(
      dialogRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'
      )
    ).filter((element) => !element.hasAttribute('hidden'));
    if (!focusable.length) {
      event.preventDefault();
      dialogRef.current.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && (document.activeElement === first || document.activeElement === dialogRef.current)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  const handleSelectionChange = (models: ChannelModelCandidate[], selected: boolean) => {
    setSelectedKeys((current) =>
      updateChannelModelSelection(current, catalog.all, models, selected)
    );
  };

  const handleSave = () => {
    onSave(resolveChannelModelSelection(catalog, selectedKeys, initialMapping));
  };

  const toggleGroup = (group: ChannelModelGroup) => {
    const groupKey = `${displayedTab}:${group.name}`;
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (next.has(groupKey)) next.delete(groupKey);
      else next.add(groupKey);
      return next;
    });
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center p-2 sm:p-4"
      role="presentation"
    >
      <div
        className="absolute inset-0 bg-surface-950/65"
        onMouseDown={onClose}
        aria-hidden="true"
      />
      <div
        ref={dialogRef}
        tabIndex={-1}
        onKeyDown={handleDialogKeyDown}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        className="relative flex max-h-[calc(100dvh-1rem)] w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-surface-200 bg-white text-surface-900 shadow-2xl sm:max-h-[calc(100dvh-2rem)] dark:border-surface-700 dark:bg-surface-900 dark:text-surface-100"
      >
        <header className="shrink-0 border-b border-surface-200 px-4 py-4 pr-14 sm:px-6 sm:py-5 dark:border-surface-700">
          <h2 id={titleId} className="text-lg font-semibold sm:text-xl">获取模型</h2>
          <p id={descriptionId} className="mt-1 text-sm text-surface-500 dark:text-surface-400">
            渠道：<strong className="font-semibold text-surface-700 dark:text-surface-200">{channelName}</strong>
          </p>
          <button
            type="button"
            onClick={onClose}
            className="absolute right-3 top-3 rounded-md p-2 text-surface-500 hover:bg-surface-100 hover:text-surface-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 sm:right-4 sm:top-4 dark:hover:bg-surface-800 dark:hover:text-white"
            aria-label="关闭模型选择"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6 sm:py-5">
          <div className="relative">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400"
              aria-hidden="true"
            />
            <Input
              ref={searchInputRef}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="搜索模型..."
              className="pl-9"
              aria-label="搜索模型"
            />
          </div>

          <Tabs
            defaultValue={getPreferredTab(catalog)}
            value={displayedTab}
            onValueChange={(value) => setActiveTab(value as ChannelModelStatus)}
            keepMounted={false}
            className="mt-4"
          >
            <TabsList className="grid h-auto w-full grid-cols-3 gap-1">
              {modelTabOrder.map((tab) => (
                <TabsTrigger
                  key={tab}
                  value={tab}
                  disabled={filteredCatalog[tab].length === 0}
                  className="min-w-0 px-2 text-xs sm:text-sm"
                >
                  <span className="truncate">{tabLabels[tab]} ({filteredCatalog[tab].length})</span>
                </TabsTrigger>
              ))}
            </TabsList>

            {modelTabOrder.map((tab) => (
              <TabsContent key={tab} value={tab} className="mt-3 space-y-2">
                {tab === 'removed' && filteredCatalog.removed.length > 0 ? (
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                    这些模型仍在当前渠道中，但本次未由上游返回。取消勾选后，保存模型会将其移出渠道。
                  </p>
                ) : null}
                {tab === displayedTab && activeGroups.length > 0 ? (
                  activeGroups.map((group, index) => {
                    const groupKey = `${tab}:${group.name}`;
                    return (
                      <ChannelModelGroupSection
                        key={groupKey}
                        group={group}
                        collapsed={collapsedGroups.has(groupKey)}
                        idPrefix={`${modelIdPrefix}-${tab}-${index}`}
                        onCollapsedChange={() => toggleGroup(group)}
                        onSelectionChange={handleSelectionChange}
                        selectedKeys={selectedKeySet}
                      />
                    );
                  })
                ) : (
                  <div className="rounded-lg border border-dashed border-surface-200 px-4 py-10 text-center text-sm text-surface-500 dark:border-surface-700 dark:text-surface-400">
                    {deferredSearch ? '没有匹配的模型' : '此分类暂无模型'}
                  </div>
                )}
              </TabsContent>
            ))}
          </Tabs>

          <div
            className="mt-4 rounded-lg border border-surface-200 bg-surface-50 px-4 py-3 text-sm text-surface-700 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-200"
            aria-live="polite"
          >
            已选 {selectedKeys.length} 个模型
          </div>
        </div>

        <footer className="flex shrink-0 justify-end gap-2 border-t border-surface-200 bg-surface-50 px-4 py-4 sm:px-6 dark:border-surface-700 dark:bg-surface-900">
          <Button type="button" variant="outline" onClick={onClose}>取消</Button>
          <Button type="button" onClick={handleSave}>保存模型</Button>
        </footer>
      </div>
    </div>,
    document.body
  );
};
