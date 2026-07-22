/**
 * 提供商预设配置
 *
 * 定义常见 LLM 提供商的默认配置和模型列表
 */

import type { ProviderPreset } from '@/modules/ai-config/types/aiConfig';

/**
 * 提供商预设列表
 */
export const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    code: 'openai',
    name: 'OpenAI',
    defaultBaseUrl: 'https://api.openai.com',
    models: [
	  'gpt-4.1',
	  'gpt-4.1-mini',
	  'gpt-4.1-nano',
      'gpt-4o',
      'gpt-4o-mini',
      'gpt-4-turbo',
      'gpt-4',
      'gpt-3.5-turbo',
      'gpt-3.5-turbo-16k',
	  'o3',
	  'o4-mini',
	  'o1-pro',
    ],
  },
  {
    code: 'gemini',
    name: 'Google Gemini',
    defaultBaseUrl: 'https://generativelanguage.googleapis.com/v1beta',
    models: [
      'gemini-2.0-flash',
      'gemini-2.0-flash-lite',
      'gemini-1.5-pro',
      'gemini-1.5-flash',
      'gemini-1.5-flash-8b',
    ],
  },
  {
    code: 'deepseek',
    name: 'DeepSeek',
    defaultBaseUrl: 'https://api.deepseek.com',
    models: ['deepseek-chat', 'deepseek-coder', 'deepseek-reasoner'],
  },
  {
    code: 'qwen',
    name: '通义千问 (Qwen)',
    defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode',
    models: [
      'qwen-plus',
      'qwen-turbo',
      'qwen-max',
      'qwen-max-longcontext',
      'qwen-long',
      'qwen-vl-plus',
      'qwen-vl-max',
    ],
  },
  {
    code: 'anthropic',
    name: 'Anthropic (Claude)',
    defaultBaseUrl: 'https://api.anthropic.com',
    models: [
      'claude-3-5-sonnet-20241022',
      'claude-3-5-haiku-20241022',
      'claude-3-opus-20240229',
      'claude-3-sonnet-20240229',
      'claude-3-haiku-20240307',
    ],
  },
  {
    code: 'zhipu',
    name: '智谱 AI (GLM)',
    defaultBaseUrl: 'https://open.bigmodel.cn/api/paas',
    models: ['glm-4-plus', 'glm-4', 'glm-4-air', 'glm-4-airx', 'glm-4-flash', 'glm-4v-plus', 'glm-4v'],
  },
  {
    code: 'moonshot',
    name: 'Moonshot (Kimi)',
    defaultBaseUrl: 'https://api.moonshot.cn',
    models: ['moonshot-v1-8k', 'moonshot-v1-32k', 'moonshot-v1-128k'],
  },
  {
    code: 'baichuan',
    name: '百川智能',
    defaultBaseUrl: 'https://api.baichuan-ai.com',
    models: ['Baichuan4', 'Baichuan3-Turbo', 'Baichuan3-Turbo-128k', 'Baichuan2-Turbo'],
  },
  {
    code: 'minimax',
    name: 'MiniMax',
    defaultBaseUrl: 'https://api.minimax.chat',
    models: ['abab6.5s-chat', 'abab6.5g-chat', 'abab6.5t-chat', 'abab5.5s-chat', 'abab5.5-chat'],
  },
  {
    code: 'yi',
    name: '零一万物 (Yi)',
    defaultBaseUrl: 'https://api.lingyiwanwu.com',
    models: ['yi-large', 'yi-large-turbo', 'yi-medium', 'yi-medium-200k', 'yi-spark', 'yi-vision'],
  },
  {
    code: 'custom',
    name: '自定义',
    defaultBaseUrl: '',
    models: [],
  },
];

/**
 * 根据代码获取提供商预设
 */
export function getProviderPreset(code: string): ProviderPreset | undefined {
  return PROVIDER_PRESETS.find((p) => p.code === normalizeProviderPresetCode(code));
}

export function normalizeProviderPresetCode(code: string): string {
  return code === 'openai-responses' ? 'openai' : code;
}

/**
 * 获取所有预设模型（去重）
 */
export function getAllPresetModels(): string[] {
  const allModels = PROVIDER_PRESETS.flatMap((p) => p.models);
  return [...new Set(allModels)];
}

/**
 * 根据提供商代码获取相关模型
 */
export function getRelatedModels(code: string): string[] {
  const preset = getProviderPreset(code);
  return preset?.models || [];
}
