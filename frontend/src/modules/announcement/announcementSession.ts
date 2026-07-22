import type { SystemAnnouncement } from './types';

const storagePrefix = 'announcement-session-closed:v1:';
const maxStoredKeys = 500;

export function announcementRevisionKey(announcement: Pick<SystemAnnouncement, 'id' | 'revision'>): string {
  return `${announcement.id}:${announcement.revision}`;
}

export function loadSessionClosedAnnouncementKeys(userID: string): Set<string> {
  if (!userID) return new Set<string>();
  try {
    const raw = sessionStorage.getItem(`${storagePrefix}${userID}`);
    if (!raw) return new Set<string>();
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return new Set<string>();
    const keys = value.filter(
      (item): item is string => typeof item === 'string' && item.length > 0 && item.length <= 100
    );
    return new Set(keys.slice(-maxStoredKeys));
  } catch {
    return new Set<string>();
  }
}

export function closeAnnouncementForSession(userID: string, announcement: SystemAnnouncement): void {
  if (!userID) return;
  try {
    const keys = loadSessionClosedAnnouncementKeys(userID);
    keys.add(announcementRevisionKey(announcement));
    sessionStorage.setItem(`${storagePrefix}${userID}`, JSON.stringify([...keys].slice(-maxStoredKeys)));
  } catch {
    // Session storage can be unavailable under browser privacy policies.
  }
}
