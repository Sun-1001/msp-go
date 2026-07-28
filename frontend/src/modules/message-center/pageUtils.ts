export function matchesAllKeywords(haystack: string, search: string): boolean {
  const keywords = search.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (keywords.length === 0) return true;
  const normalized = haystack.toLowerCase();
  return keywords.every((keyword) => normalized.includes(keyword));
}

export function hasMinimumGlobalSearchCharacters(search: string): boolean {
  return (search.match(/[\p{L}\p{N}]/gu) ?? []).length >= 2;
}

export async function fetchLoadedPageRange<T extends { id: string }>(
  lastPage: number,
  pageSize: number,
  fetchPage: (page: number) => Promise<{ items: T[]; total: number }>,
): Promise<{ items: T[]; total: number }> {
  let previousFingerprint = '';
  for (let attempt = 0; attempt < 4; attempt++) {
    const pages: Array<{ items: T[]; total: number }> = [];
    for (let page = 1; page <= Math.max(1, lastPage); page++) {
      pages.push(await fetchPage(page));
    }
    const total = pages[0]?.total ?? 0;
    const items = mergeByID([], pages.flatMap((page) => page.items)).slice(0, total);
    const fingerprint = `${total}\u0000${items.map((item) => item.id).join('\u0000')}`;
    if (items.length === Math.min(total, Math.max(1, lastPage) * pageSize) && fingerprint === previousFingerprint) {
      return { items, total };
    }
    previousFingerprint = fingerprint;
  }

  throw new Error('list changed while refreshing loaded pages');
}

export async function fetchStableOffsetMessageWindow<T extends { id: string; time: string }>(
  lastPage: number,
  pageSize: number,
  fetchPage: (page: number) => Promise<{ messages: T[]; messages_total: number }>,
): Promise<{ messages: T[]; total: number }> {
  let previousFingerprint = '';
  for (let attempt = 0; attempt < 4; attempt++) {
    const pages: Array<{ messages: T[]; messages_total: number }> = [];
    for (let page = 1; page <= lastPage; page++) {
      pages.push(await fetchPage(page));
    }
    const total = Math.max(0, ...pages.map((page) => page.messages_total));
    const messages = pages.reduce<T[]>(
      (current, page) => mergeMessagesByID(current, page.messages),
      [],
    );
    const fingerprint = `${total}\u0000${messages.map((message) => message.id).join('\u0000')}`;
    if (messages.length === Math.min(total, lastPage * pageSize) && fingerprint === previousFingerprint) {
      return { messages, total };
    }
    previousFingerprint = fingerprint;
  }

  throw new Error('message history changed while loading');
}

export async function fetchCompleteOffsetMessageHistory<T extends { id: string; time: string }>(
  initialTotal: number,
  pageSize: number,
  fetchPage: (page: number) => Promise<{ messages: T[]; messages_total: number }>,
): Promise<{ messages: T[]; total: number; page: number }> {
  let lastPage = Math.max(1, Math.ceil(initialTotal / pageSize));
  for (let attempt = 0; attempt < 4; attempt++) {
    const history = await fetchStableOffsetMessageWindow(lastPage, pageSize, fetchPage);
    if (history.messages.length >= history.total) {
      return { ...history, page: lastPage };
    }
    lastPage = Math.max(1, Math.ceil(history.total / pageSize));
  }

  throw new Error('message history changed while loading all pages');
}

export function selectListItemID(
  currentID: string,
  itemIDs: string[],
  deepLinkID: string,
): string {
  if (deepLinkID) return deepLinkID;
  if (currentID && itemIDs.includes(currentID)) return currentID;
  return itemIDs[0] ?? '';
}

export function latestPageChanged(
  current: Array<{ id: string }>,
  latestPage: Array<{ id: string }>,
  currentTotal: number,
  latestTotal: number,
): boolean {
  if (currentTotal !== latestTotal) return true;
  if (current.length < latestPage.length) return true;
  return latestPage.some((item, index) => current[index]?.id !== item.id);
}

export function mergeByID<T extends { id: string }>(current: T[], incoming: T[]): T[] {
  const byID = new Map(current.map((item) => [item.id, item]));
  incoming.forEach((item) => byID.set(item.id, item));
  return [...byID.values()];
}

export function mergeLatestPageByID<T extends { id: string }>(
  current: T[],
  latestPage: T[],
  total: number,
): T[] {
  const latestIDs = new Set(latestPage.map((item) => item.id));
  return [
    ...latestPage,
    ...current.filter((item) => !latestIDs.has(item.id)),
  ].slice(0, total);
}

export function mergeMessagesByID<T extends { id: string; time: string }>(
  current: T[],
  incoming: T[],
): T[] {
  const byID = new Map(current.map((message) => [message.id, message]));
  incoming.forEach((message) => byID.set(message.id, message));

  return [...byID.values()].sort((left, right) => {
    const leftTime = Date.parse(left.time);
    const rightTime = Date.parse(right.time);
    if (Number.isFinite(leftTime) && Number.isFinite(rightTime) && leftTime !== rightTime) {
      return leftTime - rightTime;
    }

    const timeOrder = left.time.localeCompare(right.time);
    return timeOrder !== 0 ? timeOrder : left.id.localeCompare(right.id);
  });
}
