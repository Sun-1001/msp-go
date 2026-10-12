/** 西电账户绑定模块。 */
export { clearLegacyXidianStorage } from './services/credentialStorage';
export { default as xidianService } from './services/xidianService';
export type { XidianBindingStatus, XidianCaptchaChallenge } from './services/xidianService';
