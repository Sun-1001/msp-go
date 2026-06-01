/**
 * 西电账号凭证兼容清理工具
 */

const CREDENTIAL_KEY = 'xidian_cred';

export interface XidianCredential {
  username: string;
  password: string;
}

/**
 * 不再在浏览器持久化西电密码；调用时同步清理旧版缓存。
 */
export function saveCredential(username: string, password: string): void {
  void username;
  void password;
  clearCredential();
}

/**
 * 旧版本曾从 localStorage 读取可逆混淆凭证；现在始终清理并返回空值。
 */
export function loadCredential(): XidianCredential | null {
  clearCredential();
  return null;
}

/**
 * 清除保存的凭证
 */
export function clearCredential(): void {
  localStorage.removeItem(CREDENTIAL_KEY);
}

/**
 * 检查是否有保存的凭证
 */
export function hasCredential(): boolean {
  clearCredential();
  return false;
}
