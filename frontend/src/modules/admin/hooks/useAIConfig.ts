/**
 * AI 配置管理 Hook
 *
 * 封装查询、界面选择和业务逻辑
 */

import { useCallback, useState } from 'react';
import { skipToken } from '@reduxjs/toolkit/query/react';
import { useAppDispatch, useAppSelector } from '@/store';
import { selectApiScope } from '@/store/api';
import { toAppError } from '@/libs/http/appError';
import {
  // 查询与变更
  aiConfigApi,
  useGetAIProvidersQuery,
  useGetAIModelsQuery,
  useGetAIAgentConfigsQuery,
  useGetAIAgentTypesQuery,
  useCreateAIProviderMutation,
  useCreateAIProviderWithModelsMutation,
  useUpdateAIProviderMutation,
  useDeleteAIProviderMutation,
  useTestAIProviderMutation,
  useFetchAIAvailableModelsMutation,
  useUpdateAIProviderModelsMutation,
  useCreateAIModelMutation,
  useUpdateAIModelMutation,
  useDeleteAIModelMutation,
  useSetDefaultAIModelMutation,
  useUpdateAIAgentConfigMutation,
  useDeleteAIAgentConfigMutation,
  // Actions
  setSelectedProvider,
  setSelectedModel,
  setSelectedAgentType,
  // Selectors
  selectSelectedProviderId,
  selectSelectedModelId,
  selectSelectedAgentType,
} from '@/modules/ai-config/state';
import {
  selectActiveModels,
  selectActiveProviders,
  selectDefaultModel,
  selectModelsByProvider,
  selectSelectedModel,
  selectSelectedProvider,
} from '@/store/selectors/aiConfigSelectors';
import type {
  LLMProvider,
  LLMModel,
  AgentModelConfig,
  AgentTypeInfo,
  CreateProviderRequest,
  UpdateProviderRequest,
  CreateProviderWithModelsRequest,
  CreateModelRequest,
  UpdateModelRequest,
  UpdateAgentConfigRequest,
  ModelsUpdateRequest,
} from '@/modules/ai-config';
import type { LoadingState } from '@/types';

const emptyProviders: LLMProvider[] = [];
const emptyModels: LLMModel[] = [];
const emptyAgentConfigs: AgentModelConfig[] = [];
const emptyAgentTypes: AgentTypeInfo[] = [];
const refreshOptions = { subscribe: false, forceRefetch: true };

function useQueryFeedback(query: {
  currentData?: unknown;
  error?: unknown;
  isFetching: boolean;
  isError: boolean;
}, fallback: string) {
  const [dismissedError, setDismissedError] = useState<unknown>();
  const loading: LoadingState = query.isFetching && query.currentData === undefined
    ? 'loading'
    : query.isError ? 'error' : query.currentData !== undefined ? 'success' : 'idle';
  // 清除错误
  const clearError = useCallback(() => setDismissedError(query.error), [query.error]);
  return {
    loading,
    error: query.error && query.error !== dismissedError ? toAppError(query.error, fallback) : null,
    fetching: query.isFetching,
    clearError,
  };
}

/**
 * 提供商管理 Hook
 */
export function useProviders(initialIncludeInactive = false) {
  const dispatch = useAppDispatch();
  const scope = useAppSelector(selectApiScope);
  const [includeInactive, setIncludeInactive] = useState(initialIncludeInactive);
  const query = useGetAIProvidersQuery(scope ? { scope, includeInactive } : skipToken);
  const providers = query.currentData ?? emptyProviders;
  const feedback = useQueryFeedback(query, '获取提供商列表失败');
  const selectedId = useAppSelector(selectSelectedProviderId);
  const [createProvider] = useCreateAIProviderMutation();
  const [createProviderWithModels] = useCreateAIProviderWithModelsMutation();
  const [updateProvider] = useUpdateAIProviderMutation();
  const [deleteProvider] = useDeleteAIProviderMutation();
  const [testProvider] = useTestAIProviderMutation();
  const [fetchModels] = useFetchAIAvailableModelsMutation();
  const [updateProviderModels] = useUpdateAIProviderModelsMutation();

  // 加载提供商列表
  const loadProviders = useCallback((includeInactive = false) => {
    setIncludeInactive(includeInactive);
    return dispatch(aiConfigApi.endpoints.getAIProviders.initiate({ scope: scope ?? '', includeInactive }, refreshOptions));
  }, [dispatch, scope]);

  // 创建提供商
  const create = useCallback((data: CreateProviderRequest) =>
    createProvider({ scope: scope ?? '', data }).unwrap(), [createProvider, scope]);

  // 创建提供商并同时创建模型
  const createWithModels = useCallback((data: CreateProviderWithModelsRequest, deferRefresh = false) =>
    createProviderWithModels({ scope: scope ?? '', data, deferRefresh }).unwrap(), [createProviderWithModels, scope]);

  // 刷新列表
  const finishBatch = useCallback(() => {
    if (scope) dispatch(aiConfigApi.util.invalidateTags([
      { type: 'AIProviders', id: scope },
      { type: 'AIModels', id: scope },
    ]));
  }, [dispatch, scope]);

  // 更新提供商
  const update = useCallback((id: string, data: UpdateProviderRequest) =>
    updateProvider({ scope: scope ?? '', id, data }).unwrap(), [scope, updateProvider]);

  // 删除提供商
  const remove = useCallback((id: string) =>
    deleteProvider({ scope: scope ?? '', id }).unwrap(), [deleteProvider, scope]);

  // 测试连接
  // 显式触发连接测试，支持指定模型
  const testConnection = useCallback((id: string, modelId?: string) =>
    testProvider({ scope: scope ?? '', id, modelId }).unwrap(), [scope, testProvider]);

  // 获取可用模型列表
  const fetchAvailableModels = useCallback((providerId: string) =>
    fetchModels({ scope: scope ?? '', providerId }).unwrap(), [fetchModels, scope]);

  const updateModels = useCallback((providerId: string, data: ModelsUpdateRequest) =>
    updateProviderModels({ scope: scope ?? '', providerId, data }).unwrap(), [scope, updateProviderModels]);

  // 选择提供商
  const select = useCallback((id: string | null) => {
    dispatch(setSelectedProvider(id));
  }, [dispatch]);

  return {
    providers,
    activeProviders: selectActiveProviders(providers),
    loading: feedback.loading,
    error: feedback.error,
    fetching: feedback.fetching,
    selectedId,
    selectedProvider: selectSelectedProvider(providers, selectedId),
    loadProviders,
    create,
    createWithModels,
    finishBatch,
    update,
    remove,
    testConnection,
    fetchAvailableModels,
    updateModels,
    select,
    // 清除错误
    clearError: feedback.clearError,
  };
}

/**
 * 模型管理 Hook
 */
export function useModels(initialIncludeInactive = false) {
  const dispatch = useAppDispatch();
  const scope = useAppSelector(selectApiScope);
  const [filter, setFilter] = useState<{ providerId?: string; includeInactive: boolean }>({ includeInactive: initialIncludeInactive });
  const query = useGetAIModelsQuery(scope ? { scope, ...filter } : skipToken);
  const models = query.currentData ?? emptyModels;
  const feedback = useQueryFeedback(query, '获取模型列表失败');
  const selectedId = useAppSelector(selectSelectedModelId);
  const [createModel] = useCreateAIModelMutation();
  const [updateModel] = useUpdateAIModelMutation();
  const [deleteModel] = useDeleteAIModelMutation();
  const [setDefaultModel] = useSetDefaultAIModelMutation();

  // 加载模型列表
  const loadModels = useCallback((providerId?: string, includeInactive = false) => {
    setFilter({ providerId, includeInactive });
    return dispatch(aiConfigApi.endpoints.getAIModels.initiate({ scope: scope ?? '', providerId, includeInactive }, refreshOptions));
  }, [dispatch, scope]);

  // 根据提供商获取模型（基于已加载的 models 数据过滤）
  const getModelsByProvider = useCallback((providerId: string) =>
    selectModelsByProvider(models, providerId), [models]);

  // 创建模型
  const create = useCallback((data: CreateModelRequest) =>
    createModel({ scope: scope ?? '', data }).unwrap(), [createModel, scope]);

  // 更新模型
  const update = useCallback((id: string, data: UpdateModelRequest) =>
    updateModel({ scope: scope ?? '', id, data }).unwrap(), [scope, updateModel]);

  // 删除模型
  const remove = useCallback((id: string) =>
    deleteModel({ scope: scope ?? '', id }).unwrap(), [deleteModel, scope]);

  // 设置默认模型
  const setDefault = useCallback((id: string) =>
    setDefaultModel({ scope: scope ?? '', id }).unwrap(), [scope, setDefaultModel]);

  // 选择模型
  const select = useCallback((id: string | null) => {
    dispatch(setSelectedModel(id));
  }, [dispatch]);

  return {
    models,
    activeModels: selectActiveModels(models),
    loading: feedback.loading,
    error: feedback.error,
    fetching: feedback.fetching,
    selectedId,
    selectedModel: selectSelectedModel(models, selectedId),
    defaultModel: selectDefaultModel(models),
    loadModels,
    getModelsByProvider,
    create,
    update,
    remove,
    setDefault,
    select,
    // 清除错误
    clearError: feedback.clearError,
  };
}

/**
 * 智能体配置管理 Hook
 */
export function useAgentConfigs() {
  const dispatch = useAppDispatch();
  const scope = useAppSelector(selectApiScope);
  const query = useGetAIAgentConfigsQuery(scope ? { scope } : skipToken);
  const typesQuery = useGetAIAgentTypesQuery(scope ? { scope } : skipToken);
  const configs = query.currentData ?? emptyAgentConfigs;
  const agentTypes = typesQuery.currentData ?? emptyAgentTypes;
  const feedback = useQueryFeedback({
    currentData: query.currentData && typesQuery.currentData,
    error: query.error ?? typesQuery.error,
    isFetching: query.isFetching || typesQuery.isFetching,
    isError: query.isError || typesQuery.isError,
  }, '获取智能体配置失败');
  const selectedType = useAppSelector(selectSelectedAgentType);
  const [updateAgentConfig] = useUpdateAIAgentConfigMutation();
  const [deleteAgentConfig] = useDeleteAIAgentConfigMutation();

  // 加载智能体配置
  const loadConfigs = useCallback(() =>
    dispatch(aiConfigApi.endpoints.getAIAgentConfigs.initiate({ scope: scope ?? '' }, refreshOptions)), [dispatch, scope]);

  // 加载智能体类型
  const loadTypes = useCallback(() =>
    dispatch(aiConfigApi.endpoints.getAIAgentTypes.initiate({ scope: scope ?? '' }, refreshOptions)), [dispatch, scope]);

  // 根据类型获取配置（基于已加载的 configs 数据查找）
  const getConfigByType = useCallback((agentType: string) =>
    configs.find((config) => config.agent_type === agentType) || null, [configs]);

  // 更新配置
  const update = useCallback((agentType: string, data: UpdateAgentConfigRequest) =>
    updateAgentConfig({ scope: scope ?? '', agentType, data }).unwrap(), [scope, updateAgentConfig]);

  // 删除配置
  const remove = useCallback((agentType: string) =>
    deleteAgentConfig({ scope: scope ?? '', agentType }).unwrap(), [deleteAgentConfig, scope]);

  // 选择智能体类型
  const select = useCallback((type: string | null) => {
    dispatch(setSelectedAgentType(type));
  }, [dispatch]);

  return {
    configs,
    agentTypes,
    loading: feedback.loading,
    error: feedback.error,
    fetching: feedback.fetching,
    selectedType,
    loadConfigs,
    loadTypes,
    getConfigByType,
    update,
    remove,
    select,
    // 清除错误
    clearError: feedback.clearError,
  };
}

/**
 * 测试连接 Hook
 */
export function useTestConnection() {
  const scope = useAppSelector(selectApiScope);
  const [testProvider, mutation] = useTestAIProviderMutation();
  const current = mutation.originalArgs?.scope === scope;

  // 测试连接（使用 RTK Query mutation）
  const test = useCallback((providerId: string, modelId?: string) =>
    testProvider({ scope: scope ?? '', id: providerId, modelId }).unwrap(), [scope, testProvider]);

  // 清除结果
  // 清除测试结果
  const clear = mutation.reset;

  return {
    result: current ? mutation.data ?? null : null,
    loading: current && mutation.isLoading,
    test,
    clear,
  };
}

/**
 * AI 配置综合 Hook
 *
 * 组合所有配置管理功能
 */
export function useAIConfig() {
  // 初始化时按当前身份订阅数据
  const providers = useProviders(true);
  const models = useModels(true);
  const agentConfigs = useAgentConfigs();
  const testConnection = useTestConnection();

  // 初始化加载所有数据
  const { loadProviders } = providers;
  const { loadModels } = models;
  const { loadConfigs, loadTypes } = agentConfigs;
  const loadAll = useCallback((includeInactive = true) => Promise.all([
    loadProviders(includeInactive),
    loadModels(undefined, includeInactive),
    loadConfigs(),
    loadTypes(),
  ]), [loadProviders, loadModels, loadConfigs, loadTypes]);

  // 查询 Hook 自行管理订阅和卸载，无需手工重放首屏请求。
  return { providers, models, agentConfigs, testConnection, loadAll };
}

export default useAIConfig;
