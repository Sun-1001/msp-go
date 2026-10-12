/**
 * AI 模型设置页面
 *
 * 管理 AI 渠道（提供商 + 模型）和智能体配置
 */

import React, { useState, useCallback } from 'react';
import { AdminLayout } from '@/modules/admin/components/AdminLayout';
import { Card, CardContent } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../../components/ui/Tabs';
import { useToast } from '../../components/ui/Toast';
import { RequestErrorNotice } from '@/components/feedback';
import {
  Brain,
  Bot,
  Plus,
  RefreshCw,
  Loader2,
  CheckCircle,
  Server,
  DatabaseZap,
} from 'lucide-react';

// 导入 Admin 模块组件
import {
  ChannelFormModal,
  ChannelCard,
  AgentConfigPanel,
  EmbeddingConfigPanel,
  useAIConfig,
} from '@/modules/admin';

// 导入 Hooks 和类型
import { useAppSelector } from '../../store';
import { selectApiScope } from '@/store/api';
import type {
  LLMProvider,
  LLMModel,
  CreateProviderWithModelsRequest,
  UpdateProviderRequest,
  UpdateAgentConfigRequest,
  ProviderTestResult,
  FetchModelsResponse,
  ModelCreateSimple,
  ModelsUpdateResponse,
} from '@/modules/ai-config/types/aiConfig';
import { getAdminErrorToast } from '@/modules/admin/utils/errorFeedback';

export const AIModelSettingsPage: React.FC = () => {
  const scope = useAppSelector(selectApiScope);
  const { toast } = useToast();

  // 初始化加载数据
  // 相同查询由 RTK Query 共享，StrictMode 不再手工延迟派发。
  const { providers: providersState, models: modelsState, agentConfigs: agentsState, testConnection, loadAll } = useAIConfig();
  const { test: testProviderConnection } = testConnection;
  const { fetchAvailableModels, updateModels } = providersState;

  // 查询状态（保留稳定的空数据默认值）
  const providers = providersState.providers;
  const providersLoading = providersState.loading;
  const providersError = providersState.error;
  const models = modelsState.models;
  const modelsLoading = modelsState.loading;
  const modelsError = modelsState.error;
  const agentConfigs = agentsState.configs;
  const agentTypes = agentsState.agentTypes;
  const agentConfigsLoading = agentsState.loading;
  const agentConfigsError = agentsState.error;

  // 本地状态
  const [editor, setEditor] = useState<{ scope: string; provider: LLMProvider | null; models: LLMModel[] } | null>(null);
  const currentEditor = editor?.scope === scope ? editor : null;
  const isModalOpen = currentEditor !== null;
  const editingProvider = currentEditor?.provider ?? null;
  const isRefreshing = providersState.fetching || modelsState.fetching || agentsState.fetching;

  // 刷新数据
  const handleRefresh = async () => {
    await loadAll();
  };

  // 打开新建 Modal
  const handleOpenCreateModal = () => {
    if (scope) setEditor({ scope, provider: null, models: [] });
  };

  // 打开编辑 Modal
  const handleOpenEditModal = (provider: LLMProvider) => {
    if (scope) setEditor({ scope, provider, models: modelsState.getModelsByProvider(provider.id) });
  };

  // 关闭 Modal
  const handleCloseModal = () => {
    setEditor(null);
  };

  // 创建渠道（提供商 + 模型）
  const handleCreateChannel = (data: CreateProviderWithModelsRequest, deferRefresh = false) => {
    return providersState.createWithModels(data, deferRefresh);
  };

  // 更新提供商
  const handleUpdateProvider = async (id: string, data: UpdateProviderRequest) => {
    await providersState.update(id, data);
  };

  // 删除提供商
  const handleDeleteProvider = async (provider: LLMProvider) => {
    if (window.confirm(`确定要删除渠道 "${provider.name}" 吗？这将同时删除所有关联的模型。`)) {
      try {
        await providersState.remove(provider.id);
      } catch (error) {
        const feedback = getAdminErrorToast(error, '删除渠道失败');
        if (feedback) toast(feedback);
      }
    }
  };

  // 切换提供商状态
  const handleToggleProviderActive = async (provider: LLMProvider) => {
    try {
      await providersState.update(provider.id, { is_active: !provider.is_active });
    } catch (error) {
      const feedback = getAdminErrorToast(error, '更新渠道状态失败');
      if (feedback) toast(feedback);
    }
  };

  // 测试连接
  const handleTestConnection = useCallback(
    (provider: LLMProvider, modelId?: string): Promise<ProviderTestResult> => testProviderConnection(provider.id, modelId),
    [testProviderConnection]
  );

  // 获取可用模型列表
  const handleFetchModels = useCallback(
    (providerId: string): Promise<FetchModelsResponse> => fetchAvailableModels(providerId),
    [fetchAvailableModels]
  );

  // 更新提供商的模型列表
  const handleUpdateProviderModels = useCallback(
    (providerId: string, models: ModelCreateSimple[]): Promise<ModelsUpdateResponse> => updateModels(providerId, { models }),
    [updateModels]
  );

  // 设置默认模型
  const handleSetDefaultModel = async (modelId: string) => {
    try {
      await modelsState.setDefault(modelId);
    } catch (error) {
      const feedback = getAdminErrorToast(error, '设置默认模型失败');
      if (feedback) toast(feedback);
    }
  };

  // 更新智能体配置
  const handleUpdateAgentConfig = async (agentType: string, data: UpdateAgentConfigRequest) => {
    await agentsState.update(agentType, data);
  };

  // 删除智能体配置
  const handleDeleteAgentConfig = async (agentType: string) => {
    await agentsState.remove(agentType);
  };

  // 获取提供商的模型
  const getProviderModels = modelsState.getModelsByProvider;
  const editingProviderModels = currentEditor?.models ?? [];

  // 统计数据
  const stats = {
    totalProviders: providers.length,
    activeProviders: providers.filter((p) => p.is_active).length,
    totalModels: models.length,
    configuredAgents: agentConfigs.length,
  };

  // 加载状态
  const isLoading = providersLoading === 'loading' || modelsLoading === 'loading';

  return (
    <AdminLayout>
      <div className="container mx-auto max-w-7xl">
        {/* 页面标题 */}
        <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100 mb-2">
              AI 模型设置
            </h1>
            <p className="text-surface-500 dark:text-surface-400">
              配置和管理系统使用的 AI 渠道和智能体
            </p>
          </div>
          <div className="flex w-full gap-3 sm:w-auto">
            <Button className="flex-1 whitespace-nowrap sm:flex-none" variant="outline" onClick={handleRefresh} disabled={isRefreshing}>
              {isRefreshing ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4 mr-2" />
              )}
              刷新
            </Button>
            <Button className="flex-1 whitespace-nowrap sm:flex-none" onClick={handleOpenCreateModal}>
              <Plus className="w-4 h-4 mr-2" />
              添加渠道
            </Button>
          </div>
        </div>

        {/* 统计卡片 */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-2xl font-bold text-surface-900 dark:text-surface-100">
                    {stats.totalProviders}
                  </div>
                  <div className="text-sm text-surface-500 dark:text-surface-400 mt-1">
                    渠道总数
                  </div>
                </div>
                <Server className="w-8 h-8 text-primary-600 dark:text-primary-400" />
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                    {stats.activeProviders}
                  </div>
                  <div className="text-sm text-surface-500 dark:text-surface-400 mt-1">
                    运行中
                  </div>
                </div>
                <CheckCircle className="w-8 h-8 text-emerald-600 dark:text-emerald-400" />
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-2xl font-bold text-surface-900 dark:text-surface-100">
                    {stats.totalModels}
                  </div>
                  <div className="text-sm text-surface-500 dark:text-surface-400 mt-1">
                    模型总数
                  </div>
                </div>
                <Brain className="w-8 h-8 text-blue-600 dark:text-blue-400" />
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-2xl font-bold text-surface-900 dark:text-surface-100">
                    {stats.configuredAgents}
                  </div>
                  <div className="text-sm text-surface-500 dark:text-surface-400 mt-1">
                    已配置智能体
                  </div>
                </div>
                <Bot className="w-8 h-8 text-purple-600 dark:text-purple-400" />
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 错误提示 */}
        {providersError && (
          <RequestErrorNotice
            error={providersError}
            onRetry={() => { void providersState.loadProviders(true); }}
            onRefresh={() => { void providersState.loadProviders(true); }}
            className="mb-6"
          />
        )}

        {/* 主要内容 */}
        <Tabs defaultValue="channels" className="space-y-6">
          <div className="overflow-x-auto pb-1">
            <TabsList className="min-w-max">
              <TabsTrigger value="channels">
                <Server className="w-4 h-4 mr-2" />
                渠道管理
              </TabsTrigger>
              <TabsTrigger value="agents">
                <Bot className="w-4 h-4 mr-2" />
                智能体配置
              </TabsTrigger>
              <TabsTrigger value="embeddings">
                <DatabaseZap className="w-4 h-4 mr-2" />
                向量模型
              </TabsTrigger>
            </TabsList>
          </div>

          {/* 渠道管理 Tab */}
          <TabsContent value="channels" className="space-y-4">
            {modelsError ? (
              <RequestErrorNotice
                error={modelsError}
                onRetry={() => { void modelsState.loadModels(undefined, true); }}
                onRefresh={() => { void modelsState.loadModels(undefined, true); }}
              />
            ) : null}
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="w-8 h-8 animate-spin text-primary-600" />
              </div>
            ) : providers.length === 0 ? (
              <Card>
                <CardContent className="py-12">
                  <div className="text-center">
                    <Server className="w-12 h-12 mx-auto mb-4 text-surface-400 dark:text-surface-500" />
                    <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-2">
                      暂无渠道
                    </h3>
                    <p className="text-surface-500 dark:text-surface-400 mb-4">
                      点击"添加渠道"按钮创建您的第一个 AI 渠道
                    </p>
                    <Button onClick={handleOpenCreateModal}>
                      <Plus className="w-4 h-4 mr-2" />
                      添加渠道
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ) : (
              <div className="space-y-4">
                {providers.map((provider) => (
                  <ChannelCard
                    key={provider.id}
                    provider={provider}
                    models={getProviderModels(provider.id)}
                    onEdit={handleOpenEditModal}
                    onDelete={handleDeleteProvider}
                    onToggleActive={handleToggleProviderActive}
                    onTestConnection={handleTestConnection}
                    onSetDefaultModel={handleSetDefaultModel}
                  />
                ))}
              </div>
            )}
          </TabsContent>

          {/* 智能体配置 Tab */}
          <TabsContent value="agents">
            {agentConfigsError ? (
              <RequestErrorNotice
                error={agentConfigsError}
                onRetry={() => {
                  void agentsState.loadConfigs();
                  void agentsState.loadTypes();
                }}
                onRefresh={() => {
                  void agentsState.loadConfigs();
                  void agentsState.loadTypes();
                }}
                className="mb-4"
              />
            ) : null}
            <AgentConfigPanel
              key={scope ?? 'anonymous'}
              agentTypes={agentTypes}
              agentConfigs={agentConfigs}
              providers={providers}
              models={models}
              onUpdateConfig={handleUpdateAgentConfig}
              onDeleteConfig={handleDeleteAgentConfig}
              loading={agentConfigsLoading === 'loading'}
            />
          </TabsContent>

          <TabsContent value="embeddings">
            <EmbeddingConfigPanel key={scope ?? 'anonymous'} providers={providers} models={models} />
          </TabsContent>
        </Tabs>

        {/* 渠道表单 Modal */}
        <ChannelFormModal
          key={scope ?? 'anonymous'}
          isOpen={isModalOpen}
          onClose={handleCloseModal}
          onSubmit={handleCreateChannel}
          onBatchSettled={providersState.finishBatch}
          onUpdate={handleUpdateProvider}
          onUpdateModels={handleUpdateProviderModels}
          onFetchModels={handleFetchModels}
          editingProvider={editingProvider}
          existingModels={editingProviderModels}
        />
      </div>
    </AdminLayout>
  );
};

export default AIModelSettingsPage;
