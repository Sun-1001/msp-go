/**
 * 请求去重工具
 *
 * 防止相同的 API 请求在短时间内重复发送
 * 遵循 KISS 原则，使用简单的 Map 缓存实现
 */

interface PendingRequest<T> {
  promise: Promise<T>;
  timestamp: number;
}

/**
 * 请求去重管理器
 */
class RequestDeduplicationManager {
  // 存储正在进行的请求
  private pendingRequests = new Map<string, PendingRequest<unknown>>();

  // 缓存过期时间（毫秒）
  private readonly CACHE_DURATION = 5000; // 5秒

  /**
   * 生成请求的唯一键
   */
  private generateKey(url: string, params?: Record<string, unknown>): string {
    const paramsStr = params ? JSON.stringify(params) : '';
    return `${url}:${paramsStr}`;
  }

  /**
   * 清理过期的请求缓存
   */
  private cleanupExpired(): void {
    const now = Date.now();
    for (const [key, request] of this.pendingRequests.entries()) {
      if (now - request.timestamp > this.CACHE_DURATION) {
        this.pendingRequests.delete(key);
      }
    }
  }

  /**
   * 执行去重请求
   *
   * @param url - 请求 URL
   * @param requestFn - 实际的请求函数
   * @param params - 请求参数（用于生成唯一键）
   * @returns Promise<T>
   *
   * @example
   * ```ts
   * const data = await requestDeduplication.dedupe(
   *   '/api/v1/knowledge/stats',
   *   () => apiClient.get('/api/v1/knowledge/stats'),
   *   {}
   * );
   * ```
   */
  async dedupe<T>(
    url: string,
    requestFn: () => Promise<T>,
    params?: Record<string, unknown>
  ): Promise<T> {
    // 定期清理过期缓存
    this.cleanupExpired();

    const key = this.generateKey(url, params);

    // 如果已有相同的请求正在进行，直接返回该 Promise
    const existing = this.pendingRequests.get(key);
    if (existing) {
      return existing.promise as Promise<T>;
    }

    const entry: PendingRequest<T> = {
      promise: requestFn(),
      timestamp: Date.now(),
    };
    this.pendingRequests.set(key, entry);

    try {
      return await entry.promise;
    } finally {
      // A slow expired request must not remove a newer request using the same key.
      if (this.pendingRequests.get(key) === entry) {
        this.pendingRequests.delete(key);
      }
    }
  }

  /**
   * 清除所有缓存
   */
  clear(): void {
    this.pendingRequests.clear();
  }

  /**
   * 清除指定 URL 的缓存
   */
  clearByUrl(url: string): void {
    for (const key of this.pendingRequests.keys()) {
      if (key.startsWith(`${url}:`)) {
        this.pendingRequests.delete(key);
      }
    }
  }

  /**
   * 获取当前缓存的请求数量
   */
  get size(): number {
    return this.pendingRequests.size;
  }
}

/**
 * 全局请求去重管理器实例
 */
export const requestDeduplication = new RequestDeduplicationManager();

/**
 * 创建带去重功能的请求包装器
 *
 * @param url - 请求 URL
 * @param requestFn - 实际的请求函数
 * @returns 包装后的请求函数
 *
 * @example
 * ```ts
 * const fetchStatsWithDedupe = createDedupedRequest(
 *   '/api/v1/knowledge/stats',
 *   () => apiClient.get('/api/v1/knowledge/stats')
 * );
 *
 * // 多次调用只会发送一次请求
 * const [result1, result2] = await Promise.all([
 *   fetchStatsWithDedupe(),
 *   fetchStatsWithDedupe(),
 * ]);
 * ```
 */
export function createDedupedRequest<T, P extends Record<string, unknown> = Record<string, unknown>>(
  url: string,
  requestFn: (params?: P) => Promise<T>
) {
  return (params?: P): Promise<T> => {
    return requestDeduplication.dedupe(url, () => requestFn(params), params);
  };
}
