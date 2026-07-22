import { apiClient } from '@/libs/http/apiClient';
import type {
  AnnouncementActionResponse,
  AnnouncementListResponse,
  SaveAnnouncementRequest,
  SystemAnnouncement,
} from './types';

function announcementPath(id: string): string {
  return `/announcements/${encodeURIComponent(id)}`;
}

export const announcementService = {
  async listForAdmin(signal?: AbortSignal): Promise<AnnouncementListResponse> {
    const response = await apiClient.get<AnnouncementListResponse>('/admin/announcements', { signal });
    return response.data;
  },

  async listForUser(signal?: AbortSignal): Promise<AnnouncementListResponse> {
    const response = await apiClient.get<AnnouncementListResponse>('/announcements', { signal });
    return response.data;
  },

  async create(payload: SaveAnnouncementRequest): Promise<SystemAnnouncement> {
    const response = await apiClient.post<SystemAnnouncement>('/admin/announcements', payload);
    return response.data;
  },

  async update(id: string, payload: SaveAnnouncementRequest): Promise<SystemAnnouncement> {
    const response = await apiClient.put<SystemAnnouncement>(`/admin${announcementPath(id)}`, payload);
    return response.data;
  },

  async delete(id: string): Promise<AnnouncementActionResponse> {
    const response = await apiClient.delete<AnnouncementActionResponse>(`/admin${announcementPath(id)}`);
    return response.data;
  },

  async dismiss(id: string): Promise<AnnouncementActionResponse> {
    const response = await apiClient.post<AnnouncementActionResponse>(`${announcementPath(id)}/dismiss`);
    return response.data;
  },
};
