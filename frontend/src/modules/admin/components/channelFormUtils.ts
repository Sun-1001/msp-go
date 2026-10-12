import type { ModelCreateSimple } from '@/modules/ai-config';

export type CredentialMode = 'single' | 'batch' | 'multi';
export type KeyStrategy = 'round_robin' | 'random';

export interface ParsedChannelConnectionInfo {
  apiKeys: string[];
  baseUrl?: string;
  code?: string;
  models?: string[];
  name?: string;
}

const CONNECTION_INFO_TYPE = 'newapi_channel_conn';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function readString(record: Record<string, unknown>, keys: string[]): string | undefined {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return undefined;
}

function readStringList(record: Record<string, unknown>, keys: string[]): string[] {
  for (const key of keys) {
    const value = record[key];
    if (Array.isArray(value)) {
      return uniqueTrimmed(value.filter((item): item is string => typeof item === 'string'));
    }
    if (typeof value === 'string') return parseCredentialKeys(value);
  }
  return [];
}

export function uniqueTrimmed(values: string[]): string[] {
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

export function parseCredentialKeys(value: string): string[] {
  return uniqueTrimmed(value.split(/\r?\n/));
}

export function parseChannelConnectionInfo(text: string): ParsedChannelConnectionInfo | null {
  const value = text.trim();
  if (!value) return null;

  try {
    const parsed: unknown = JSON.parse(value);
    if (isRecord(parsed)) {
      const apiKeys = readStringList(parsed, ['api_keys', 'keys']);
      const singleKey = readString(parsed, ['api_key', 'key']);
      const normalizedKeys = uniqueTrimmed(apiKeys.length ? apiKeys : singleKey ? [singleKey] : []);
      const baseUrl = readString(parsed, ['base_url', 'url']);
      const isNewAPIConnection = parsed._type === CONNECTION_INFO_TYPE;
      if (normalizedKeys.length && (baseUrl || isNewAPIConnection)) {
        const result: ParsedChannelConnectionInfo = { apiKeys: normalizedKeys };
        const code = readString(parsed, ['code', 'provider', 'type']);
        const models = readStringList(parsed, ['models']);
        const name = readString(parsed, ['name']);
        if (baseUrl) result.baseUrl = baseUrl;
        if (code) result.code = code;
        if (models.length) result.models = models;
        if (name) result.name = name;
        return result;
      }
    }
  } catch {
    // Fall through to the line-oriented format.
  }

  const fields: Record<string, string[]> = {};
  for (const line of value.split(/\r?\n/)) {
    const match = line.match(/^\s*([A-Za-z_]+)\s*[:=]\s*(.+?)\s*$/);
    if (match) {
      const key = match[1].toLowerCase();
      (fields[key] ??= []).push(match[2]);
    }
  }
  const apiKeys = uniqueTrimmed([
    ...(fields.api_key ?? []),
    ...(fields.key ?? []),
  ]);
  const baseUrl = (fields.base_url ?? fields.url)?.[0];
  if (apiKeys.length && baseUrl) {
    const result: ParsedChannelConnectionInfo = {
      apiKeys,
      baseUrl: baseUrl.trim(),
    };
    const code = (fields.code ?? fields.provider)?.[0];
    const models = fields.models
      ? uniqueTrimmed(fields.models.flatMap((modelList) => modelList.split(',')))
      : [];
    if (code) result.code = code;
    if (models.length) result.models = models;
    if (fields.name?.[0]) result.name = fields.name[0];
    return result;
  }

  return null;
}

export function buildBatchChannelName(baseName: string, index: number, total: number): string {
  if (total <= 1) return baseName.trim();
  return `${baseName.trim()} ${index + 1}`;
}

export function buildModelRequests(
  selectedModels: string[],
  modelMapping: Record<string, string>
): ModelCreateSimple[] {
  const selected = uniqueTrimmed(selectedModels);
  const upstreamOwners = new Map<string, string>();
  return selected.map((name) => {
    const modelId = modelMapping[name]?.trim() || name;
    const owner = upstreamOwners.get(modelId);
    if (owner && owner !== name) {
      throw new Error(`模型“${owner}”和“${name}”不能同时映射到“${modelId}”`);
    }
    upstreamOwners.set(modelId, name);
    return { model_id: modelId, name };
  });
}
