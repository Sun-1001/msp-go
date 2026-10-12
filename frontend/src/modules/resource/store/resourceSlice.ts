/**
 * 资源状态管理
 *
 * 管理资源中心的状态：资源列表、统计、收藏、筛选等
 */

import { api, scopedRequest, type ApiScope } from '@/store/api';
import { resourceService } from '@/modules/resource/services/resourceService';
import type { AppError } from '@/libs/http/appError';
import type {
  Resource,
  ResourceFilter,
  ResourceListResponse,
  ResourceStats,
  ResourceCreateRequest,
  ResourceUpdateRequest,
  FavoriteToggleResponse,
} from '@/modules/resource/types/resource';

export { resetQueryState as resetState, selectApiScope as selectResourceScope } from '@/store/api';

// =============================================================================
// 状态类型
// =============================================================================

/**
 * 设置筛选条件
 */
interface ResourceListQuery extends ApiScope {
  // 筛选条件
  filter?: ResourceFilter;
}

function resourceTags(scope: string) {
  return [
    // 更新资源列表中的收藏状态
    { type: 'ResourceList' as const, id: scope },
    // 更新统计中的收藏数
    { type: 'ResourceStats' as const, id: scope },
  ];
}

function invalidateResources(_result: unknown, error: AppError | undefined, { scope }: ApiScope) {
  return error ? [] : resourceTags(scope);
}

// =============================================================================
// Slice
// =============================================================================

export const resourceApi = api.enhanceEndpoints({
  addTagTypes: ['ResourceList', 'ResourceStats'],
}).injectEndpoints({
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
      queryFn: ({ scope, filter }, api) => scopedRequest(
        scope, api, (signal) => resourceService.getResources(filter, signal), '获取资源列表失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'ResourceList', id: scope }],
    }),
    /**
     * 获取资源统计
     */
    // 统计数据
    getResourceStats: builder.query<ResourceStats, ApiScope>({
      // 获取资源统计
      queryFn: ({ scope }, api) => scopedRequest(
        scope, api, (signal) => resourceService.getStats(signal), '获取资源统计失败',
      ),
      providesTags: (_result, _error, { scope }) => [{ type: 'ResourceStats', id: scope }],
    }),
    /**
     * 切换收藏状态
     */
    toggleFavorite: builder.mutation<FavoriteToggleResponse, ApiScope & { id: string }>({
      // 切换收藏
      queryFn: ({ scope, id }, api) => scopedRequest(
        scope, api, (signal) => resourceService.toggleFavorite(id, signal), '收藏状态更新失败',
      ),
      invalidatesTags: invalidateResources,
    }),
    /**
     * 创建资源
     */
    createResource: builder.mutation<Resource, ApiScope & { data: ResourceCreateRequest }>({
      // 创建资源
      queryFn: ({ scope, data }, api) => scopedRequest(
        scope, api, (signal) => resourceService.createResource(data, signal), '创建资源失败',
      ),
      // 更新统计
      invalidatesTags: invalidateResources,
    }),
    /**
     * 更新资源
     */
    updateResource: builder.mutation<Resource, ApiScope & { id: string; data: ResourceUpdateRequest }>({
      // 更新资源
      queryFn: ({ scope, id, data }, api) => scopedRequest(
        scope, api, (signal) => resourceService.updateResource(id, data, signal), '更新资源失败',
      ),
      invalidatesTags: invalidateResources,
    }),
    /**
     * 删除资源
     */
    deleteResource: builder.mutation<string, ApiScope & { id: string }>({
      // 删除资源
      queryFn: ({ scope, id }, api) => scopedRequest(scope, api, async (signal) => {
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

export default resourceApi.reducer;
