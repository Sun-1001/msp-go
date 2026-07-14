import { afterEach, describe, it, expect, vi, beforeEach } from 'vitest';
import { requestDeduplication, createDedupedRequest } from '@/libs/utils/requestDeduplication';

// 每个测试前清空缓存，避免测试间相互影响
beforeEach(() => {
  requestDeduplication.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('requestDeduplication.dedupe', () => {
  it('返回请求函数的结果', async () => {
    const result = await requestDeduplication.dedupe(
      '/api/test',
      () => Promise.resolve({ data: 42 })
    );
    expect(result).toEqual({ data: 42 });
  });

  it('并发相同请求只调用一次请求函数', async () => {
    const requestFn = vi.fn(() => Promise.resolve('response'));

    const [r1, r2, r3] = await Promise.all([
      requestDeduplication.dedupe('/api/same', requestFn),
      requestDeduplication.dedupe('/api/same', requestFn),
      requestDeduplication.dedupe('/api/same', requestFn),
    ]);

    expect(requestFn).toHaveBeenCalledTimes(1);
    expect(r1).toBe('response');
    expect(r2).toBe('response');
    expect(r3).toBe('response');
  });

  it('不同 URL 不会去重', async () => {
    const fn1 = vi.fn(() => Promise.resolve('a'));
    const fn2 = vi.fn(() => Promise.resolve('b'));

    const [r1, r2] = await Promise.all([
      requestDeduplication.dedupe('/api/url1', fn1),
      requestDeduplication.dedupe('/api/url2', fn2),
    ]);

    expect(fn1).toHaveBeenCalledTimes(1);
    expect(fn2).toHaveBeenCalledTimes(1);
    expect(r1).toBe('a');
    expect(r2).toBe('b');
  });

  it('相同 URL 但不同参数不会去重', async () => {
    const requestFn = vi.fn((key: string) => Promise.resolve(key));

    const [r1, r2] = await Promise.all([
      requestDeduplication.dedupe('/api/data', () => requestFn('a'), { id: 1 }),
      requestDeduplication.dedupe('/api/data', () => requestFn('b'), { id: 2 }),
    ]);

    expect(requestFn).toHaveBeenCalledTimes(2);
    expect(r1).toBe('a');
    expect(r2).toBe('b');
  });

  it('相同 URL 和相同参数会去重', async () => {
    const requestFn = vi.fn(() => Promise.resolve('deduped'));

    const [r1, r2] = await Promise.all([
      requestDeduplication.dedupe('/api/data', requestFn, { id: 1 }),
      requestDeduplication.dedupe('/api/data', requestFn, { id: 1 }),
    ]);

    expect(requestFn).toHaveBeenCalledTimes(1);
    expect(r1).toBe('deduped');
    expect(r2).toBe('deduped');
  });

  it('请求失败后从缓存中移除', async () => {
    const error = new Error('网络错误');
    const requestFn = vi.fn(() => Promise.reject(error));

    await expect(
      requestDeduplication.dedupe('/api/fail', requestFn)
    ).rejects.toThrow('网络错误');

    // 失败后缓存应已清除，size 应为 0
    expect(requestDeduplication.size).toBe(0);
  });

  it('过期旧请求完成时不会删除同键的新请求', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-14T00:00:00Z'));
    let resolveOld!: (value: string) => void;
    let resolveCurrent!: (value: string) => void;
    const oldRequest = new Promise<string>((resolve) => { resolveOld = resolve; });
    const currentRequest = new Promise<string>((resolve) => { resolveCurrent = resolve; });
    const oldResult = requestDeduplication.dedupe('/api/slow', () => oldRequest);

    vi.setSystemTime(new Date('2026-07-14T00:00:06Z'));
    const currentFn = vi.fn(() => currentRequest);
    const currentResult = requestDeduplication.dedupe('/api/slow', currentFn);
    resolveOld('old');
    await expect(oldResult).resolves.toBe('old');

    const duplicateFn = vi.fn(() => Promise.resolve('duplicate'));
    const duplicateResult = requestDeduplication.dedupe('/api/slow', duplicateFn);
    expect(currentFn).toHaveBeenCalledOnce();
    expect(duplicateFn).not.toHaveBeenCalled();

    resolveCurrent('current');
    await expect(Promise.all([currentResult, duplicateResult])).resolves.toEqual(['current', 'current']);
  });
});

describe('requestDeduplication.clear', () => {
  it('清除所有待处理请求', async () => {
    // 创建一个永不 resolve 的请求来保持 pending 状态
    let resolve!: (v: string) => void;
    const pending = new Promise<string>(r => { resolve = r; });

    // 不 await，让它保持 pending
    requestDeduplication.dedupe('/api/pending1', () => pending);
    requestDeduplication.dedupe('/api/pending2', () => pending);

    expect(requestDeduplication.size).toBe(2);

    requestDeduplication.clear();
    expect(requestDeduplication.size).toBe(0);

    // 清理：resolve pending promise 避免内存泄漏
    resolve('done');
  });
});

describe('requestDeduplication.clearByUrl', () => {
  it('只清除指定 URL 的请求', async () => {
    let resolve1!: (v: string) => void;
    let resolve2!: (v: string) => void;
    const p1 = new Promise<string>(r => { resolve1 = r; });
    const p2 = new Promise<string>(r => { resolve2 = r; });

    requestDeduplication.dedupe('/api/target', () => p1);
    requestDeduplication.dedupe('/api/other', () => p2);

    expect(requestDeduplication.size).toBe(2);

    requestDeduplication.clearByUrl('/api/target');
    expect(requestDeduplication.size).toBe(1);

    // 清理
    resolve1('done');
    resolve2('done');
  });

  it('不会清除仅 URL 前缀相同的请求', () => {
    const pending = new Promise<string>(() => undefined);
    void requestDeduplication.dedupe('/api/user', () => pending);
    void requestDeduplication.dedupe('/api/users', () => pending);

    requestDeduplication.clearByUrl('/api/user');

    expect(requestDeduplication.size).toBe(1);
  });
});

describe('requestDeduplication.size', () => {
  it('初始 size 为 0', () => {
    expect(requestDeduplication.size).toBe(0);
  });

  it('添加请求后 size 增加', () => {
    let resolve!: (v: string) => void;
    const pending = new Promise<string>(r => { resolve = r; });

    requestDeduplication.dedupe('/api/count', () => pending);
    expect(requestDeduplication.size).toBe(1);

    // 清理
    resolve('done');
    requestDeduplication.clear();
  });

  it('请求完成后 size 减少', async () => {
    await requestDeduplication.dedupe('/api/done', () => Promise.resolve('ok'));
    expect(requestDeduplication.size).toBe(0);
  });
});

describe('createDedupedRequest', () => {
  it('创建可正常调用的包装函数', async () => {
    const fetchFn = vi.fn(() => Promise.resolve({ items: [1, 2, 3] }));
    const dedupedFetch = createDedupedRequest('/api/items', fetchFn);

    const result = await dedupedFetch();
    expect(result).toEqual({ items: [1, 2, 3] });
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });

  it('并发调用只执行一次请求', async () => {
    const fetchFn = vi.fn(() => Promise.resolve('data'));
    const dedupedFetch = createDedupedRequest('/api/deduped-wrapper', fetchFn);

    const [r1, r2] = await Promise.all([dedupedFetch(), dedupedFetch()]);

    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(r1).toBe('data');
    expect(r2).toBe('data');
  });

  it('传递参数给请求函数', async () => {
    const fetchFn = vi.fn((params?: { page: number }) =>
      Promise.resolve(`page-${params?.page}`)
    );
    const dedupedFetch = createDedupedRequest('/api/paged', fetchFn);

    const result = await dedupedFetch({ page: 2 });
    expect(result).toBe('page-2');
    expect(fetchFn).toHaveBeenCalledWith({ page: 2 });
  });
});
