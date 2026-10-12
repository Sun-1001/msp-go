/**
 * AI 配置 Selectors（记忆化）
 *
 * 对会产生新数组的 .filter() 和需要派生查找的 .find() 使用 createSelector
 * 简单字段访问保持原有 selector 不变
 */

import { createSelector } from '@reduxjs/toolkit';
import type { LLMModel, LLMProvider } from '@/modules/ai-config/types/aiConfig';

// ========== 基础 Selector ==========

const providerData = (providers: LLMProvider[]) => providers;
const modelData = (models: LLMModel[]) => models;

// ========== 派生 Selectors（记忆化） ==========

/** 活跃的提供商列表（.filter 产生新引用） */
export const selectActiveProviders = createSelector(
  [providerData],
  (providers) => providers.filter((provider) => provider.is_active)
);

/** 当前选中的提供商（避免重复线性查找） */
export const selectSelectedProvider = createSelector(
  [providerData, (_: LLMProvider[], selectedId: string | null) => selectedId],
  (providers, selectedId) => providers.find((provider) => provider.id === selectedId) || null
);

/** 活跃的模型列表（.filter 产生新引用） */
export const selectActiveModels = createSelector(
  [modelData],
  (models) => models.filter((model) => model.is_active)
);

/** 按提供商筛选模型（参数化 selector） */
export const selectModelsByProvider = createSelector(
  [modelData, (_: LLMModel[], providerId: string) => providerId],
  (models, providerId) => models.filter((model) => model.provider_id === providerId)
);

/** 当前选中的模型（避免重复线性查找） */
export const selectSelectedModel = createSelector(
  [modelData, (_: LLMModel[], selectedId: string | null) => selectedId],
  (models, selectedId) => models.find((model) => model.id === selectedId) || null
);

/** 默认模型（避免重复线性查找） */
export const selectDefaultModel = createSelector(
  [modelData],
  (models) => models.find((model) => model.is_default) || null
);
