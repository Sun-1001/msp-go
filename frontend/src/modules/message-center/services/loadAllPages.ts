type PageResponse<T> = {
  items: T[];
  total: number;
};

export async function loadAllPages<T>(fetchPage: (page: number) => Promise<PageResponse<T>>, pageSize = 50): Promise<T[]> {
  const first = await fetchPage(1);
  const items = [...first.items];
  const pages = Math.ceil(first.total / pageSize);
  const concurrency = 4;

  for (let start = 2; start <= pages; start += concurrency) {
    const end = Math.min(start + concurrency, pages + 1);
    const batch = await Promise.all(
      Array.from({ length: end - start }, (_, index) => fetchPage(start + index)),
    );
    for (const next of batch) {
      items.push(...next.items);
    }
  }
  return items;
}
