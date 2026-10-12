import { createApi, fakeBaseQuery } from '@reduxjs/toolkit/query/react';
import type { BaseQueryApi, QueryReturnValue } from '@reduxjs/toolkit/query';
import type { AuthState } from '@/modules/auth/state';
import { toAppError, type AppError } from '@/libs/http/appError';

export interface ApiScope {
  scope: string;
}

export function selectApiScope({ auth }: { auth: AuthState }): string | undefined {
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
export async function scopedRequest<T>(
  scope: string,
  api: BaseQueryApi,
  request: (signal: AbortSignal) => Promise<T>,
  fallback: string,
): Promise<QueryReturnValue<T, AppError, undefined>> {
  if (api.signal.aborted || scope !== selectApiScope(api.getState() as { auth: AuthState })) {
    return { error: { kind: 'cancelled', message: '登录身份已变更，请求已取消', retryable: false, source: 'ui' } };
  }
  try {
    return { data: await request(api.signal) };
  } catch (error) {
    return { error: toAppError(error, fallback) };
  }
}

export const api = createApi({
  reducerPath: 'api',
  baseQuery: fakeBaseQuery<AppError>(),
  ...cachePolicy,
  endpoints: () => ({}),
});

/**
 * 重置状态
 */
export const resetQueryState = api.util.resetApiState;
