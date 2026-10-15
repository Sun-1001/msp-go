import { apiClient } from '@/libs/http/apiClient';

export interface CourseConcept { id: string; label: string; chapter?: string | null }
export interface LearningPathItem {
  id: string; title: string; description: string; chapter?: string | null;
  status: 'completed' | 'locked' | 'current' | 'available'; locked_by?: string[];
  recommendation?: string; mastery: number; confidence: number; exercises: number; difficulty: number;
}
export interface LearningPathResponse {
  path: LearningPathItem[]; estimated_exercises: number;
  statistics: { total: number; completed: number; progress: number };
}
export const curriculumService = {
  async getConcepts(signal?: AbortSignal): Promise<CourseConcept[]> {
    return (await apiClient.get<CourseConcept[]>('/progress/concepts', { signal })).data;
  },
  async getLearningGoal(signal?: AbortSignal): Promise<{ target_id: string | null; updated_at: string | null }> {
    return (await apiClient.get('/progress/learning-goal', { signal })).data;
  },
  async getLearningPath(targetId?: string, signal?: AbortSignal): Promise<LearningPathResponse> {
    return (await apiClient.get<LearningPathResponse>('/progress/path', { params: targetId ? { target: targetId } : undefined, signal })).data;
  },
};
