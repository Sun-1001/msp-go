import type {
  AgentModelConfig,
  LLMModel,
  LLMProvider,
} from '@/modules/ai-config/types/aiConfig';

export interface LogicalModelOption {
  value: string;
  label: string;
  representativeModelId: string;
  channelCount: number;
}

export function buildLogicalModelOptions(
  providers: LLMProvider[],
  models: LLMModel[]
): LogicalModelOption[] {
  const activeProviderIds = new Set(
    providers.filter((provider) => provider.is_active).map((provider) => provider.id)
  );
  const groups = new Map<
    string,
    { channelIds: Set<string>; representativeModelId: string }
  >();

  for (const model of models) {
    const modelKey = model.name.trim();
    if (!modelKey || !model.is_active || !activeProviderIds.has(model.provider_id)) continue;
    const current = groups.get(modelKey);
    if (current) {
      current.channelIds.add(model.provider_id);
      continue;
    }
    groups.set(modelKey, {
      channelIds: new Set([model.provider_id]),
      representativeModelId: model.id,
    });
  }

  return Array.from(groups.entries())
    .map(([modelKey, group]) => ({
      value: modelKey,
      label: `${modelKey} · ${group.channelIds.size} 个可用渠道`,
      representativeModelId: group.representativeModelId,
      channelCount: group.channelIds.size,
    }))
    .sort((left, right) => left.value.localeCompare(right.value));
}

export function resolveAgentModelKey(
  config: AgentModelConfig | undefined,
  models: LLMModel[]
): string {
  if (!config) return '';
  const storedKey = config.model_key?.trim();
  if (storedKey) return storedKey;
  const joinedName = config.model_name?.trim();
  if (joinedName) return joinedName;
  return models.find((model) => model.id === config.model_id)?.name.trim() ?? '';
}
