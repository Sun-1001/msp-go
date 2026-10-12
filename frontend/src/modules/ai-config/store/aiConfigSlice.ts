/**
 * AI 配置状态管理
 *
 * 管理 AI 模型配置的查询缓存与界面选择状态
 */

import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { api, scopedRequest, type ApiScope } from '@/store/api';
import { aiConfigService } from '@/modules/ai-config/services/aiConfigService';
import type {
  LLMProvider,
  LLMModel,
  AgentModelConfig,
  AgentTypeInfo,
  CreateProviderRequest,
  CreateProviderWithModelsRequest,
  ProviderWithModelsResponse,
  UpdateProviderRequest,
  CreateModelRequest,
  UpdateModelRequest,
  UpdateAgentConfigRequest,
  ProviderTestResult,
  FetchModelsResponse,
  ModelsUpdateRequest,
  ModelsUpdateResponse,
  SuccessResponse,
} from '@/modules/ai-config/types/aiConfig';
import type { AppError } from '@/libs/http/appError';

/**
 * AI 配置状态
 */
interface AIConfigState {
  // 当前选中
  // 提供商
  selectedProviderId: string | null;
  // 模型
  selectedModelId: string | null;
  // 智能体配置
  selectedAgentType: string | null;
}

const initialState: AIConfigState = {
  selectedProviderId: null,
  selectedModelId: null,
  selectedAgentType: null,
};

type ProviderQuery = ApiScope & { includeInactive?: boolean };
type ModelQuery = ProviderQuery & { providerId?: string };
const aiTagTypes = ['AIProviders', 'AIModels', 'AIAgentConfigs', 'AIAgentTypes'] as const;

function invalidateAI(...tags: (typeof aiTagTypes)[number][]) {
  return (_result: unknown, error: AppError | undefined, { scope }: ApiScope) =>
    error ? [] : tags.map((type) => ({ type, id: scope }));
}

const invalidateModels = invalidateAI('AIModels', 'AIAgentConfigs', 'AIAgentTypes');
const invalidateAgents = invalidateAI('AIAgentConfigs', 'AIAgentTypes');

// ========== 查询与变更 ==========

export const aiConfigApi = api.enhanceEndpoints({ addTagTypes: [...aiTagTypes] }).injectEndpoints({
  endpoints: (builder) => ({
    // 提供商相关
    // ========== 提供商 ==========
    getAIProviders: builder.query<LLMProvider[], ProviderQuery>({
      serializeQueryArgs: ({ queryArgs }) => ({
        scope: queryArgs.scope,
        includeInactive: queryArgs.includeInactive ?? false,
      }),
      queryFn: ({ scope, includeInactive = false }, api) => scopedRequest(
        scope, api, async (signal) => (await aiConfigService.listProviders(includeInactive, signal)).items,
        '获取提供商列表失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'AIProviders', id: scope }],
    }),
    createAIProvider: builder.mutation<LLMProvider, ApiScope & { data: CreateProviderRequest }>({
      queryFn: ({ scope, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.createProvider(data, signal), '创建提供商失败',
      ),
      invalidatesTags: invalidateAI('AIProviders'),
    }),
    updateAIProvider: builder.mutation<LLMProvider, ApiScope & { id: string; data: UpdateProviderRequest }>({
      queryFn: ({ scope, id, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.updateProvider(id, data, signal), '更新提供商失败',
      ),
      invalidatesTags: invalidateAI(...aiTagTypes),
    }),
    deleteAIProvider: builder.mutation<SuccessResponse, ApiScope & { id: string }>({
      queryFn: ({ scope, id }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.deleteProvider(id, signal), '删除提供商失败',
      ),
      // 同时删除关联的模型
      invalidatesTags: invalidateAI(...aiTagTypes),
    }),
    // 连接测试
    testAIProvider: builder.mutation<ProviderTestResult, ApiScope & { id: string; modelId?: string }>({
      queryFn: ({ scope, id, modelId }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.testProvider(id, modelId, signal), '测试连接失败',
      ),
    }),
    createAIProviderWithModels: builder.mutation<ProviderWithModelsResponse, ApiScope & { data: CreateProviderWithModelsRequest; deferRefresh?: boolean }>({
      // 新增 provider
      queryFn: ({ scope, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.createProviderWithModels(data, signal), '创建渠道失败',
      ),
      // 重新读取模型列表，保留服务端的去重结果
      invalidatesTags: (result, error, args) => args.deferRefresh
        ? [] : invalidateAI('AIProviders', 'AIModels')(result, error, args),
    }),
    fetchAIAvailableModels: builder.mutation<FetchModelsResponse, ApiScope & { providerId: string }>({
      queryFn: ({ scope, providerId }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.fetchAvailableModels(providerId, signal), '获取可用模型列表失败',
      ),
    }),
    updateAIProviderModels: builder.mutation<ModelsUpdateResponse, ApiScope & { providerId: string; data: ModelsUpdateRequest }>({
      // 从请求参数获取提供商 ID，空模型响应也能正确刷新
      queryFn: ({ scope, providerId, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.updateProviderModels(providerId, data, signal), '更新模型列表失败',
      ),
      // 移除该提供商的旧模型
      // 添加新模型
      invalidatesTags: invalidateModels,
    }),
    // 模型相关
    // ========== 模型 ==========
    getAIModels: builder.query<LLMModel[], ModelQuery>({
      serializeQueryArgs: ({ queryArgs }) => ({
        scope: queryArgs.scope,
        providerId: queryArgs.providerId || undefined,
        includeInactive: queryArgs.includeInactive ?? false,
      }),
      queryFn: ({ scope, providerId, includeInactive = false }, api) => scopedRequest(
        scope, api, async (signal) => (await aiConfigService.listModels(providerId, includeInactive, signal)).items,
        '获取模型列表失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'AIModels', id: scope }],
    }),
    createAIModel: builder.mutation<LLMModel, ApiScope & { data: CreateModelRequest }>({
      queryFn: ({ scope, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.createModel(data, signal), '创建模型失败',
      ),
      invalidatesTags: invalidateModels,
    }),
    updateAIModel: builder.mutation<LLMModel, ApiScope & { id: string; data: UpdateModelRequest }>({
      queryFn: ({ scope, id, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.updateModel(id, data, signal), '更新模型失败',
      ),
      invalidatesTags: invalidateModels,
    }),
    deleteAIModel: builder.mutation<SuccessResponse, ApiScope & { id: string }>({
      queryFn: ({ scope, id }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.deleteModel(id, signal), '删除模型失败',
      ),
      invalidatesTags: invalidateModels,
    }),
    setDefaultAIModel: builder.mutation<SuccessResponse, ApiScope & { id: string }>({
      queryFn: ({ scope, id }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.setDefaultModel(id, signal), '设置默认模型失败',
      ),
      // 更新所有模型的 is_default 状态
      invalidatesTags: invalidateModels,
    }),
    // 智能体配置相关
    // ========== 智能体配置 ==========
    getAIAgentConfigs: builder.query<AgentModelConfig[], ApiScope>({
      queryFn: ({ scope }, api) => scopedRequest(
        scope, api, async (signal) => (await aiConfigService.listAgentConfigs(signal)).items, '获取智能体配置失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'AIAgentConfigs', id: scope }],
    }),
    getAIAgentTypes: builder.query<AgentTypeInfo[], ApiScope>({
      queryFn: ({ scope }, api) => scopedRequest(
        scope, api, async (signal) => (await aiConfigService.listAgentTypes(signal)).items, '获取智能体类型失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'AIAgentTypes', id: scope }],
    }),
    updateAIAgentConfig: builder.mutation<AgentModelConfig, ApiScope & { agentType: string; data: UpdateAgentConfigRequest }>({
      queryFn: ({ scope, agentType, data }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.updateAgentConfig(agentType, data, signal), '更新智能体配置失败',
      ),
      // 更新 agentTypes 中的 configured 状态
      invalidatesTags: invalidateAgents,
    }),
    deleteAIAgentConfig: builder.mutation<SuccessResponse, ApiScope & { agentType: string }>({
      queryFn: ({ scope, agentType }, api) => scopedRequest(
        scope, api, (signal) => aiConfigService.deleteAgentConfig(agentType, signal), '删除智能体配置失败',
      ),
      // 更新 agentTypes 中的 configured 状态
      invalidatesTags: invalidateAgents,
    }),
  }),
});

// ========== Slice ==========

const aiConfigSlice = createSlice({
  name: 'aiConfig',
  initialState,
  reducers: {
    // 选择提供商
    setSelectedProvider(state, action: PayloadAction<string | null>) {
      state.selectedProviderId = action.payload;
    },
    // 选择模型
    setSelectedModel(state, action: PayloadAction<string | null>) {
      state.selectedModelId = action.payload;
    },
    // 选择智能体类型
    setSelectedAgentType(state, action: PayloadAction<string | null>) {
      state.selectedAgentType = action.payload;
    },
    // 重置状态
    resetAIConfigState() {
      return initialState;
    },
  },
  extraReducers: (builder) => {
    builder.addCase(api.util.resetApiState, () => initialState);
  },
});

// 导出 actions
export const {
  setSelectedProvider,
  setSelectedModel,
  setSelectedAgentType,
  resetAIConfigState,
} = aiConfigSlice.actions;

// ========== Selectors ==========

// 类型定义
type StateWithAIConfig = { aiConfig?: AIConfigState };

// 提供商
export const selectSelectedProviderId = (state: StateWithAIConfig) =>
  state.aiConfig?.selectedProviderId ?? null;

// 模型
export const selectSelectedModelId = (state: StateWithAIConfig) =>
  state.aiConfig?.selectedModelId ?? null;

// 智能体配置
export const selectSelectedAgentType = (state: StateWithAIConfig) =>
  state.aiConfig?.selectedAgentType ?? null;

// 测试结果
export const {
  useGetAIProvidersQuery,
  useGetAIModelsQuery,
  useGetAIAgentConfigsQuery,
  useGetAIAgentTypesQuery,
  useCreateAIProviderMutation,
  useUpdateAIProviderMutation,
  useDeleteAIProviderMutation,
  useTestAIProviderMutation,
  useCreateAIProviderWithModelsMutation,
  useFetchAIAvailableModelsMutation,
  useUpdateAIProviderModelsMutation,
  useCreateAIModelMutation,
  useUpdateAIModelMutation,
  useDeleteAIModelMutation,
  useSetDefaultAIModelMutation,
  useUpdateAIAgentConfigMutation,
  useDeleteAIAgentConfigMutation,
} = aiConfigApi;

export default aiConfigSlice.reducer;
