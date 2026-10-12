/**
 * 资源状态管理
 *
 * 管理资源中心的状态：资源列表、统计、收藏、筛选等
 */

import { createApi, fakeBaseQuery } from '@reduxjs/toolkit/query/react';
import type { BaseQueryApi, QueryReturnValue } from '@reduxjs/toolkit/query';
import type { AuthState } from '@/modules/auth/store/authSlice';
import { resourceService } from '@/modules/resource/services/resourceService';
import { toAppError, type AppError } from '@/libs/http/appError';
import type {
  Resource,
  ResourceFilter,
  ResourceListResponse,
  ResourceStats,
  ResourceCreateRequest,
  ResourceUpdateRequest,
  FavoriteToggleResponse,
} from '@/modules/resource/types/resource';

// =============================================================================
// 状态类型
// =============================================================================

interface ResourceScope {
  scope: string;
}

/**
 * 设置筛选条件
 */
interface ResourceListQuery extends ResourceScope {
  // 筛选条件
  filter?: ResourceFilter;
}

export function selectResourceScope({ auth }: { auth: AuthState }): string | undefined {
  return auth.isAuthenticated && auth.user ? `${auth.user.id}:${auth.user.role}` : undefined;
}

// =============================================================================
// 初始状态
// =============================================================================

// 加载状态
const cachePolicy = {
  keepUnusedDataFor: 60,
  refetchOnMountOrArgChange: 30,
  refetchOnFocus: false,
  refetchOnReconnect: false,
};

// =============================================================================
// 异步请求
// =============================================================================

/**
 * 归一化请求错误
 */
async function resourceRequest<T>(
  scope: string,
  api: BaseQueryApi,
  request: (signal: AbortSignal) => Promise<T>,
  fallback: string,
): Promise<QueryReturnValue<T, AppError, undefined>> {
  if (api.signal.aborted || scope !== selectResourceScope(api.getState() as { auth: AuthState })) {
    return { error: { kind: 'cancelled', message: '登录身份已变更，请求已取消', retryable: false, source: 'ui' } };
  }
  try {
    return { data: await request(api.signal) };
  } catch (error) {
    return { error: toAppError(error, fallback) };
  }
}

function resourceTags(scope: string) {
  return [
    // 更新资源列表中的收藏状态
    { type: 'ResourceList' as const, id: scope },
    // 更新统计中的收藏数
    { type: 'ResourceStats' as const, id: scope },
  ];
}

function invalidateResources(_result: unknown, error: AppError | undefined, { scope }: ResourceScope) {
  return error ? [] : resourceTags(scope);
}

// =============================================================================
// Slice
// =============================================================================

export const resourceApi = createApi({
  reducerPath: 'resourceApi',
  baseQuery: fakeBaseQuery<AppError>(),
  tagTypes: ['ResourceList', 'ResourceStats'],
  ...cachePolicy,
  endpoints: (builder) => ({
    /**
     * 获取资源列表
     */
    // 资源列表
    getResources: builder.query<ResourceListResponse, ResourceListQuery>({
      /**
       * 重置筛选条件
       */
      serializeQueryArgs: ({ queryArgs }) => ({ scope: queryArgs.scope, filter: queryArgs.filter ?? {} }),
      // 获取资源列表
      queryFn: ({ scope, filter }, api) => resourceRequest(
        scope, api, (signal) => resourceService.getResources(filter, signal), '获取资源列表失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'ResourceList', id: scope }],
    }),
    /**
     * 获取资源统计
     */
    // 统计数据
    getResourceStats: builder.query<ResourceStats, ResourceScope>({
      // 获取资源统计
      queryFn: ({ scope }, api) => resourceRequest(
        scope, api, (signal) => resourceService.getStats(signal), '获取资源统计失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'ResourceStats', id: scope }],
    }),
    /**
     * 切换收藏状态
     */
    toggleFavorite: builder.mutation<FavoriteToggleResponse, ResourceScope & { id: string }>({
      // 切换收藏
      queryFn: ({ scope, id }, api) => resourceRequest(
        scope, api, (signal) => resourceService.toggleFavorite(id, signal), '收藏状态更新失败',
      ),
      invalidatesTags: invalidateResources,
    }),
    /**
     * 创建资源
     */
    createResource: builder.mutation<Resource, ResourceScope & { data: ResourceCreateRequest }>({
      // 创建资源
      queryFn: ({ scope, data }, api) => resourceRequest(
        scope, api, (signal) => resourceService.createResource(data, signal), '创建资源失败',
      ),
      // 更新统计
      invalidatesTags: invalidateResources,
    }),
    /**
     * 更新资源
     */
    updateResource: builder.mutation<Resource, ResourceScope & { id: string; data: ResourceUpdateRequest }>({
      // 更新资源
      queryFn: ({ scope, id, data }, api) => resourceRequest(
        scope, api, (signal) => resourceService.updateResource(id, data, signal), '更新资源失败',
      ),
      invalidatesTags: invalidateResources,
    }),
    /**
     * 删除资源
     */
    deleteResource: builder.mutation<string, ResourceScope & { id: string }>({
      // 删除资源
      queryFn: ({ scope, id }, api) => resourceRequest(scope, api, async (signal) => {
        await resourceService.deleteResource(id, signal);
        return id;
      }, '删除资源失败'),
      // 更新统计
      invalidatesTags: invalidateResources,
    }),
  }),
});

// =============================================================================
// 导出
// =============================================================================

export const {
  useGetResourcesQuery,
  useGetResourceStatsQuery,
  useToggleFavoriteMutation,
  useCreateResourceMutation,
  useUpdateResourceMutation,
  useDeleteResourceMutation,
} = resourceApi;

/**
 * 重置状态
 */
export const resetState = resourceApi.util.resetApiState;

export default resourceApi.reducer;
