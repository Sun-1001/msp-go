import React, { useState } from 'react';
import {
  Braces,
  Check,
  ClipboardCopy,
  Download,
  Eraser,
  FileText,
  Loader2,
  Plus,
  Table2,
  Trash2,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { cn } from '@/libs/utils/cn';

interface ChannelModelSelectorProps {
  availableModels: string[];
  canFetchModels: boolean;
  customModel: string;
  isFetchingModels: boolean;
  modelMapping: Record<string, string>;
  onAddCustomModel: () => void;
  onClearModels: () => void;
  onCustomModelChange: (value: string) => void;
  onFetchModels: () => void;
  onFillAllModels: () => void;
  onFillRelatedModels: () => void;
  onModelMappingChange: (mapping: Record<string, string>) => void;
  onRemoveModel: (model: string) => void;
  selectedModels: string[];
}

export const ChannelModelSelector: React.FC<ChannelModelSelectorProps> = ({
  availableModels,
  canFetchModels,
  customModel,
  isFetchingModels,
  modelMapping,
  onAddCustomModel,
  onClearModels,
  onCustomModelChange,
  onFetchModels,
  onFillAllModels,
  onFillRelatedModels,
  onModelMappingChange,
  onRemoveModel,
  selectedModels,
}) => {
  const [mappingMode, setMappingMode] = useState<'visual' | 'json'>('visual');
  const [mappingSource, setMappingSource] = useState('');
  const [mappingTarget, setMappingTarget] = useState('');
  const [mappingJson, setMappingJson] = useState(() => JSON.stringify(modelMapping, null, 2));
  const [mappingError, setMappingError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const switchMappingMode = (mode: 'visual' | 'json') => {
    if (mode === 'json') setMappingJson(JSON.stringify(modelMapping, null, 2));
    setMappingMode(mode);
    setMappingError(null);
  };

  const addMapping = () => {
    const source = mappingSource.trim();
    const target = mappingTarget.trim();
    if (!source || !target) {
      setMappingError('请同时填写逻辑模型和上游模型');
      return;
    }
    if (!selectedModels.includes(source)) {
      setMappingError('逻辑模型必须先加入模型列表');
      return;
    }
    onModelMappingChange({ ...modelMapping, [source]: target });
    setMappingSource('');
    setMappingTarget('');
    setMappingError(null);
  };

  const removeMapping = (source: string) => {
    const next = { ...modelMapping };
    delete next[source];
    onModelMappingChange(next);
  };

  const applyMappingJson = () => {
    try {
      const parsed: unknown = JSON.parse(mappingJson || '{}');
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error('模型映射必须是 JSON 对象');
      }
      const next: Record<string, string> = {};
      for (const [source, target] of Object.entries(parsed)) {
        if (typeof target !== 'string' || !source.trim() || !target.trim()) {
          throw new Error('模型映射的键和值都必须是非空字符串');
        }
        next[source.trim()] = target.trim();
      }
      onModelMappingChange(next);
      setMappingJson(JSON.stringify(next, null, 2));
      setMappingError(null);
    } catch (error) {
      setMappingError(error instanceof Error ? error.message : '模型映射 JSON 无效');
    }
  };

  const copyModels = async () => {
    if (!selectedModels.length) return;
    try {
      await navigator.clipboard.writeText(selectedModels.join(','));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-surface-200 bg-surface-50/50 p-5 dark:border-surface-700 dark:bg-surface-900/40">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <label className="text-sm font-medium text-surface-900 dark:text-surface-100">
              逻辑模型 <span className="text-red-500">*</span>
            </label>
            <p className="mt-1 text-xs leading-5 text-surface-500 dark:text-surface-400">
              此渠道提供的统一模型名称。
            </p>
          </div>
          <span className="w-fit rounded-full border border-surface-200 bg-white px-2.5 py-1 text-xs text-surface-600 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-300">
            已选 {selectedModels.length} 个
          </span>
        </div>

        <div className="mt-4 flex min-h-11 flex-wrap items-center gap-2 rounded-md border border-surface-200 bg-white px-3 py-2 focus-within:border-primary-400 focus-within:ring-2 focus-within:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-800">
          {selectedModels.map((model) => (
            <span
              key={model}
              className="inline-flex max-w-full items-center gap-1 rounded-md bg-surface-100 px-2 py-1 text-xs text-surface-700 dark:bg-surface-700 dark:text-surface-100"
            >
              <span className="truncate">{model}</span>
              <button
                type="button"
                onClick={() => onRemoveModel(model)}
                className="rounded p-0.5 text-surface-400 hover:bg-surface-200 hover:text-surface-700 dark:hover:bg-surface-600 dark:hover:text-white"
                aria-label={`移除模型 ${model}`}
              >
                <X className="h-3 w-3" aria-hidden="true" />
              </button>
            </span>
          ))}
          <input
            value={customModel}
            onChange={(event) => onCustomModelChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                onAddCustomModel();
              }
            }}
            placeholder={selectedModels.length ? '继续添加模型' : '选择模型或添加自定义模型'}
            className="h-7 min-w-52 flex-1 bg-transparent text-sm text-surface-900 outline-none placeholder:text-surface-400 dark:text-surface-100 dark:placeholder:text-surface-500"
            aria-label="添加模型"
          />
          <button
            type="button"
            onClick={onAddCustomModel}
            disabled={!customModel.trim()}
            className="rounded p-1 text-surface-400 hover:bg-surface-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-surface-700"
            aria-label="添加自定义模型"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>

        <div className="my-5 border-t border-surface-200 dark:border-surface-700" />

        <div>
          <div className="text-sm font-medium text-surface-900 dark:text-surface-100">快捷操作</div>
          <p className="mt-1 text-xs text-surface-500 dark:text-surface-400">
            使用预设快速填充，或从上游获取后按分类选择模型。
          </p>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button type="button" variant="outline" size="sm" onClick={onFillRelatedModels}>
              <FileText className="mr-1.5 h-4 w-4" aria-hidden="true" />
              填入相关模型
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={onFillAllModels}>
              <Plus className="mr-1.5 h-4 w-4" aria-hidden="true" />
              填入全部预设
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onFetchModels}
              disabled={isFetchingModels || !canFetchModels}
              title={!canFetchModels ? '请先填写有效凭证' : undefined}
            >
              {isFetchingModels ? (
                <Loader2 className="mr-1.5 h-4 w-4 animate-spin" aria-hidden="true" />
              ) : (
                <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
              )}
              获取模型
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={copyModels} disabled={!selectedModels.length}>
              {copied ? <Check className="mr-1.5 h-4 w-4" /> : <ClipboardCopy className="mr-1.5 h-4 w-4" />}
              {copied ? '已复制' : '全部复制'}
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={onClearModels} disabled={!selectedModels.length}>
              <Eraser className="mr-1.5 h-4 w-4" aria-hidden="true" />
              清除全部
            </Button>
          </div>
        </div>
      </div>

      <div className="rounded-lg border border-surface-200 p-5 dark:border-surface-700">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="text-sm font-medium text-surface-900 dark:text-surface-100">模型映射</div>
            <p className="mt-1 text-xs leading-5 text-surface-500 dark:text-surface-400">
              将逻辑模型名称映射到此渠道的上游模型 ID。
            </p>
          </div>
          <div className="inline-flex w-fit rounded-md bg-surface-100 p-1 dark:bg-surface-800" role="tablist" aria-label="模型映射编辑模式">
            <button
              type="button"
              role="tab"
              aria-selected={mappingMode === 'visual'}
              onClick={() => switchMappingMode('visual')}
              className={cn(
                'inline-flex h-8 items-center gap-1.5 rounded px-2.5 text-xs font-medium',
                mappingMode === 'visual'
                  ? 'bg-white text-surface-900 shadow-sm dark:bg-surface-700 dark:text-white'
                  : 'text-surface-500 dark:text-surface-400'
              )}
            >
              <Table2 className="h-4 w-4" aria-hidden="true" />
              可视化
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mappingMode === 'json'}
              onClick={() => switchMappingMode('json')}
              className={cn(
                'inline-flex h-8 items-center gap-1.5 rounded px-2.5 text-xs font-medium',
                mappingMode === 'json'
                  ? 'bg-white text-surface-900 shadow-sm dark:bg-surface-700 dark:text-white'
                  : 'text-surface-500 dark:text-surface-400'
              )}
            >
              <Braces className="h-4 w-4" aria-hidden="true" />
              JSON
            </button>
          </div>
        </div>

        {mappingMode === 'visual' ? (
          <div className="mt-4 space-y-3">
            {Object.entries(modelMapping).map(([source, target]) => (
              <div key={source} className="grid items-center gap-2 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto]">
                <div className="truncate rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-800">
                  {source}
                </div>
                <span className="text-xs text-surface-400">映射到</span>
                <input
                  value={target}
                  onChange={(event) => onModelMappingChange({ ...modelMapping, [source]: event.target.value })}
                  className="h-10 min-w-0 rounded-md border border-surface-200 bg-white px-3 text-sm outline-none focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-800"
                  aria-label={`${source} 的上游模型`}
                />
                <button
                  type="button"
                  onClick={() => removeMapping(source)}
                  className="rounded-md p-2 text-surface-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30"
                  aria-label={`删除模型映射 ${source}`}
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </button>
              </div>
            ))}

            <div className="grid items-center gap-2 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto]">
              <input
                value={mappingSource}
                onChange={(event) => setMappingSource(event.target.value)}
                placeholder="逻辑模型名称"
                list="channel-selected-models"
                className="h-10 min-w-0 rounded-md border border-surface-200 bg-white px-3 text-sm outline-none focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-800"
              />
              <span className="text-xs text-surface-400">映射到</span>
              <input
                value={mappingTarget}
                onChange={(event) => setMappingTarget(event.target.value)}
                placeholder="上游模型 ID"
                list="channel-available-models"
                className="h-10 min-w-0 rounded-md border border-surface-200 bg-white px-3 text-sm outline-none focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700 dark:bg-surface-800"
              />
              <Button type="button" variant="outline" size="icon" onClick={addMapping} aria-label="添加模型映射">
                <Plus className="h-4 w-4" aria-hidden="true" />
              </Button>
              <datalist id="channel-selected-models">
                {selectedModels.map((model) => <option key={model} value={model} />)}
              </datalist>
              <datalist id="channel-available-models">
                {availableModels.map((model) => <option key={model} value={model} />)}
              </datalist>
            </div>

            {!Object.keys(modelMapping).length && !mappingSource && (
              <div className="rounded-md border border-dashed border-surface-200 px-4 py-7 text-center text-sm text-surface-500 dark:border-surface-700 dark:text-surface-400">
                尚未配置模型映射。填写上方两项后点击添加即可。
              </div>
            )}
          </div>
        ) : (
          <div className="mt-4">
            <textarea
              value={mappingJson}
              onChange={(event) => setMappingJson(event.target.value)}
              onBlur={applyMappingJson}
              rows={8}
              spellCheck={false}
              className="w-full resize-y rounded-md border border-surface-200 bg-surface-950 p-3 font-mono text-xs leading-5 text-surface-100 outline-none focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-surface-700"
              aria-label="模型映射 JSON"
            />
            <div className="mt-2 flex justify-end">
              <Button type="button" variant="outline" size="sm" onClick={applyMappingJson}>应用 JSON</Button>
            </div>
          </div>
        )}

        {mappingError && <p className="mt-3 text-xs text-red-600 dark:text-red-400">{mappingError}</p>}
      </div>
    </div>
  );
};
