import { apiClient } from '@/libs/http/apiClient';
import type { Note, NoteInput, NoteMaterial, NoteNetwork, NoteSummary } from './types';

const base = '/notes';
export const noteService = {
  async list(search = '', signal?: AbortSignal): Promise<NoteSummary[]> {
    return (await apiClient.get<NoteSummary[]>(base, { params: { search }, signal })).data;
  },
  async get(id: string, signal?: AbortSignal): Promise<Note> {
    return (await apiClient.get<Note>(`${base}/${encodeURIComponent(id)}`, { signal })).data;
  },
  async save(id: string | undefined, input: NoteInput, signal?: AbortSignal): Promise<Note> {
    return (await (id ? apiClient.put<Note>(`${base}/${id}`, input, { signal }) : apiClient.post<Note>(base, input, { signal }))).data;
  },
  async remove(id: string, revision: number, signal?: AbortSignal): Promise<void> {
    await apiClient.delete(`${base}/${id}`, { params: { revision }, signal });
  },
  async network(id?: string, signal?: AbortSignal): Promise<NoteNetwork> {
    return (await apiClient.get<NoteNetwork>(id ? `${base}/${id}/network` : `${base}/network`, { signal })).data;
  },
  async materials(signal?: AbortSignal): Promise<NoteMaterial[]> {
    return (await apiClient.get<NoteMaterial[]>(`${base}/materials`, { signal })).data;
  },
  async upload(file: File, signal?: AbortSignal): Promise<NoteMaterial> {
    if (!/\.(pdf|docx|txt|md)$/i.test(file.name) || file.size === 0 || file.size > 10 * 1024 * 1024) {
      throw new Error('请选择不超过 10 MB 的 PDF、DOCX、TXT 或 Markdown 文件');
    }
    const form = new FormData();
    form.append('file', file);
    return (await apiClient.post<NoteMaterial>(`${base}/materials`, form, { headers: { 'Content-Type': 'multipart/form-data' }, signal })).data;
  },
  async download(material: NoteMaterial, signal?: AbortSignal): Promise<void> {
    const response = await apiClient.get<Blob>(`${base}/materials/${material.id}/file`, { responseType: 'blob', signal });
    const url = URL.createObjectURL(response.data);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = material.filename;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
};
