export type ChannelModelStatus = 'new' | 'existing' | 'removed';

export interface ChannelModelCandidate {
  group: string;
  key: string;
  logicalName: string;
  order: number;
  origin: 'current' | 'fetched';
  status: ChannelModelStatus;
  upstreamId: string;
}

export interface ChannelModelCatalog {
  all: ChannelModelCandidate[];
  existing: ChannelModelCandidate[];
  new: ChannelModelCandidate[];
  removed: ChannelModelCandidate[];
}

export interface ChannelModelGroup {
  models: ChannelModelCandidate[];
  name: string;
}

export interface ResolvedChannelModelSelection {
  mapping: Record<string, string>;
  models: string[];
}

const modelGroupRules: ReadonlyArray<readonly [string, readonly string[]]> = [
  ['OpenAI', ['gpt', 'o1', 'o3']],
  ['Anthropic', ['claude']],
  ['Gemini', ['gemini']],
  ['Qwen', ['qwen']],
  ['DeepSeek', ['deepseek']],
  ['Zhipu', ['glm']],
  ['Meta', ['llama']],
  ['Mistral', ['mistral']],
];

function uniqueTrimmed(values: readonly string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const rawValue of values) {
    const value = rawValue.trim();
    if (!value || seen.has(value)) continue;
    seen.add(value);
    result.push(value);
  }
  return result;
}

export function getChannelModelGroup(model: string): string {
  const normalized = model.toLowerCase();
  for (const [group, markers] of modelGroupRules) {
    if (markers.some((marker) => normalized.includes(marker))) return group;
  }
  return 'Other';
}

export function buildChannelModelCatalog(
  fetchedModels: readonly string[],
  selectedModels: readonly string[],
  modelMapping: Readonly<Record<string, string>>
): ChannelModelCatalog {
  const fetched = uniqueTrimmed(fetchedModels);
  const selected = uniqueTrimmed(selectedModels);
  const fetchedSet = new Set(fetched);
  const currentUpstreamIds = new Set<string>();

  const currentCandidates = selected.map((logicalName, order): ChannelModelCandidate => {
    const upstreamId = modelMapping[logicalName]?.trim() || logicalName;
    currentUpstreamIds.add(upstreamId);
    const status: ChannelModelStatus = fetchedSet.has(upstreamId) ? 'existing' : 'removed';
    return {
      group: getChannelModelGroup(upstreamId),
      key: `current:${logicalName}`,
      logicalName,
      order,
      origin: 'current',
      status,
      upstreamId,
    };
  });

  const newCandidates = fetched
    .filter((upstreamId) => !currentUpstreamIds.has(upstreamId))
    .map((upstreamId, index): ChannelModelCandidate => ({
      group: getChannelModelGroup(upstreamId),
      key: `fetched:${upstreamId}`,
      logicalName: upstreamId,
      order: selected.length + index,
      origin: 'fetched',
      status: 'new',
      upstreamId,
    }));

  const existing = currentCandidates.filter((model) => model.status === 'existing');
  const removed = currentCandidates.filter((model) => model.status === 'removed');
  return {
    all: [...currentCandidates, ...newCandidates],
    existing,
    new: newCandidates,
    removed,
  };
}

export function filterChannelModelCatalog(
  catalog: ChannelModelCatalog,
  search: string
): ChannelModelCatalog {
  const keyword = search.trim().toLowerCase();
  if (!keyword) return catalog;
  const matches = (model: ChannelModelCandidate) =>
    model.logicalName.toLowerCase().includes(keyword) ||
    model.upstreamId.toLowerCase().includes(keyword);
  return {
    all: catalog.all.filter(matches),
    existing: catalog.existing.filter(matches),
    new: catalog.new.filter(matches),
    removed: catalog.removed.filter(matches),
  };
}

export function groupChannelModels(models: readonly ChannelModelCandidate[]): ChannelModelGroup[] {
  const groups = new Map<string, ChannelModelCandidate[]>();
  for (const model of models) {
    const group = groups.get(model.group);
    if (group) group.push(model);
    else groups.set(model.group, [model]);
  }

  return Array.from(groups, ([name, groupedModels]) => ({ name, models: groupedModels }))
    .sort((left, right) => {
      if (left.name === 'Other') return 1;
      if (right.name === 'Other') return -1;
      return left.name.localeCompare(right.name, 'en', { sensitivity: 'base' });
    });
}

export function getInitialChannelModelSelection(catalog: ChannelModelCatalog): string[] {
  return catalog.all
    .filter((model) => model.origin === 'current')
    .map((model) => model.key);
}

export function updateChannelModelSelection(
  selectedKeys: readonly string[],
  allModels: readonly ChannelModelCandidate[],
  targetModels: readonly ChannelModelCandidate[],
  shouldSelect: boolean
): string[] {
  const selected = new Set(selectedKeys);
  if (!shouldSelect) {
    for (const model of targetModels) selected.delete(model.key);
    return Array.from(selected);
  }

  for (const target of targetModels) {
    for (const model of allModels) {
      if (model.key !== target.key && model.logicalName === target.logicalName) {
        selected.delete(model.key);
      }
    }
    selected.add(target.key);
  }
  return Array.from(selected);
}

export function resolveChannelModelSelection(
  catalog: ChannelModelCatalog,
  selectedKeys: readonly string[],
  modelMapping: Readonly<Record<string, string>>
): ResolvedChannelModelSelection {
  const selected = new Set(selectedKeys);
  const mapping: Record<string, string> = {};
  const models: string[] = [];
  const seen = new Set<string>();

  const orderedSelection = catalog.all
    .filter((model) => selected.has(model.key))
    .sort((left, right) => left.order - right.order);

  for (const model of orderedSelection) {
    if (seen.has(model.logicalName)) continue;
    seen.add(model.logicalName);
    models.push(model.logicalName);
    const mappedUpstreamId = modelMapping[model.logicalName]?.trim();
    if (model.origin === 'current' && mappedUpstreamId) {
      mapping[model.logicalName] = mappedUpstreamId;
    }
  }

  return { mapping, models };
}
