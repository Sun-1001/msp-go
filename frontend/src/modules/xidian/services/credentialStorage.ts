const LEGACY_STORAGE_KEYS = ['xidian_cred', 'xidian_classtable_cache'] as const;

/** 清理旧版本遗留的西电密码和教务课表缓存。 */
export function clearLegacyXidianStorage(): void {
  if (typeof localStorage === 'undefined') {
    return;
  }
  try {
    LEGACY_STORAGE_KEYS.forEach((key) => localStorage.removeItem(key));
  } catch {
    // 浏览器隐私策略可能禁用 Storage，遗留数据清理保持尽力而为。
  }
}
