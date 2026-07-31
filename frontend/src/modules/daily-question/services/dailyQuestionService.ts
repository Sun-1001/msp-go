import { apiClient } from '@/libs/http/apiClient';
import {
  mapExerciseQuestion,
  type ExerciseQuestionResponse,
} from '@/modules/exercise/services/exerciseService';
import type {
  DailyQuestionAssignment,
  DailyQuestionClassSettings,
  DailyQuestionClassStatistics,
  DailyQuestionClassStrategy,
  DailyQuestionFirstResult,
  DailyQuestionHistory,
  DailyQuestionHistoryItem,
  DailyQuestionReminderResult,
  DailyQuestionStatus,
  DailyQuestionUniformSchedule,
} from '../types/dailyQuestion';

interface DailyQuestionAssignmentResponse {
  status: DailyQuestionStatus;
  assignment_id?: string | null;
  assignment_date?: string;
  date?: string;
  target_concept_name?: string | null;
  source?: string | null;
  selection_reason?: string | null;
  first_attempt_id?: string | null;
  corrected_attempt_id?: string | null;
  first_result?: DailyQuestionFirstResult | null;
  opened_at?: string | null;
  counts_toward_streak?: boolean;
  streak_days?: number;
  failure_code?: string | null;
  question?: ExerciseQuestionResponse | null;
}

interface DailyQuestionHistoryResponse {
  items?: DailyQuestionAssignmentResponse[];
  streak_days?: number;
}

interface DailyQuestionClassSettingsResponse {
  strategy: DailyQuestionClassStrategy;
  effective_strategy?: DailyQuestionClassStrategy;
  effective_date?: string;
  today_assignment_count?: number;
  uniform_ready?: boolean;
  auto_reminder_enabled?: boolean;
  today_reminder_sent?: boolean;
  today_reminder_recipient_count?: number;
}

interface DailyQuestionClassStatisticsResponse {
  student_count: number;
  assigned_count: number;
  completed_count: number;
  first_correct_count: number;
  corrected_count: number;
  completion_rate: number;
  first_correct_rate: number;
  correction_rate: number;
  weak_concepts?: Array<{
    concept_id: string;
    concept_name: string;
    wrong_count: number;
  }>;
}

interface DailyQuestionReminderResponse {
  recipient_count: number;
}

interface DailyQuestionUniformScheduleResponse {
  start_date?: string;
  schedule_version?: number;
  items?: Array<{
    assignment_date: string;
    content_id: string;
    target_concept_id?: string | null;
    title?: string;
    body?: string;
    difficulty?: number;
    locked?: boolean;
  }>;
}

const datePattern = /^\d{4}-\d{2}-\d{2}$/;

function requireISODate(value: string): string {
  const normalized = value.trim();
  if (!datePattern.test(normalized)) {
    throw new Error('日期格式无效');
  }
  return normalized;
}

function mapAssignment(raw: DailyQuestionAssignmentResponse): DailyQuestionAssignment {
  return {
    status: raw.status,
    assignmentId: raw.assignment_id ?? null,
    assignmentDate: raw.assignment_date ?? raw.date ?? '',
    targetConceptName: raw.target_concept_name ?? raw.question?.knowledge_point_names?.[0] ?? null,
    source: raw.source ?? null,
    selectionReason: raw.selection_reason ?? null,
    firstAttemptId: raw.first_attempt_id ?? null,
    correctedAttemptId: raw.corrected_attempt_id ?? null,
    firstResult: raw.first_result ?? null,
    openedAt: raw.opened_at ?? null,
    countsTowardStreak: raw.counts_toward_streak ?? false,
    streakDays: Math.max(0, raw.streak_days ?? 0),
    failureCode: raw.failure_code ?? null,
    question: raw.question ? mapExerciseQuestion(raw.question, 'daily') : null,
  };
}

function toHistoryItem(assignment: DailyQuestionAssignment): DailyQuestionHistoryItem {
  const { question, ...item } = assignment;
  void question;
  return item;
}

function requireClassId(value: string): string {
  const normalized = value.trim();
  if (!normalized) {
    throw new Error('班级无效');
  }
  return normalized;
}

function classDailyQuestionPath(classId: string, suffix: string): string {
  return `/daily-question/teacher/classes/${encodeURIComponent(requireClassId(classId))}/${suffix}`;
}

function mapClassSettings(raw: DailyQuestionClassSettingsResponse): DailyQuestionClassSettings {
  return {
    strategy: raw.strategy,
    effectiveStrategy: raw.effective_strategy ?? raw.strategy,
    effectiveDate: raw.effective_date ?? '',
    todayAssignmentCount: Math.max(0, raw.today_assignment_count ?? 0),
    uniformReady: raw.uniform_ready ?? false,
    autoReminderEnabled: raw.auto_reminder_enabled ?? false,
    todayReminderSent: raw.today_reminder_sent ?? false,
    todayReminderRecipientCount: Math.max(0, raw.today_reminder_recipient_count ?? 0),
  };
}

function mapUniformSchedule(
  raw: DailyQuestionUniformScheduleResponse,
): DailyQuestionUniformSchedule {
  return {
    startDate: raw.start_date ?? raw.items?.[0]?.assignment_date ?? '',
    scheduleVersion: Math.max(0, Math.trunc(raw.schedule_version ?? 0)),
    items: (raw.items ?? []).map((item) => ({
      assignmentDate: item.assignment_date,
      contentId: item.content_id,
      targetConceptId: item.target_concept_id ?? null,
      title: item.title ?? '',
      body: item.body ?? '',
      difficulty: Math.max(0, Math.min(1, item.difficulty ?? 0.5)),
      locked: item.locked ?? false,
    })),
  };
}

export const dailyQuestionService = {
  async getToday(signal?: AbortSignal): Promise<DailyQuestionAssignment> {
    const response = await apiClient.get<DailyQuestionAssignmentResponse>(
      '/daily-question/today',
      { signal },
    );
    return mapAssignment(response.data);
  },

  async prepareToday(signal?: AbortSignal): Promise<DailyQuestionAssignment> {
    const response = await apiClient.post<DailyQuestionAssignmentResponse>(
      '/daily-question/today/prepare',
      undefined,
      { timeout: 120_000, signal },
    );
    return mapAssignment(response.data);
  },

  async getByDate(date: string, signal?: AbortSignal): Promise<DailyQuestionAssignment> {
    const normalizedDate = requireISODate(date);
    const response = await apiClient.get<DailyQuestionAssignmentResponse>(
      `/daily-question/${encodeURIComponent(normalizedDate)}`,
      { timeout: 120_000, signal },
    );
    return mapAssignment(response.data);
  },

  async getHistory(
    days = 7,
    signal?: AbortSignal,
  ): Promise<DailyQuestionHistory> {
    const normalizedDays = Math.trunc(days);
    if (!Number.isFinite(normalizedDays) || normalizedDays < 1 || normalizedDays > 366) {
      throw new Error('历史天数无效');
    }
    const response = await apiClient.get<DailyQuestionHistoryResponse>(
      '/daily-question/history',
      {
        params: { days: normalizedDays },
        signal,
      },
    );
    return {
      items: (response.data.items ?? []).map((item) => toHistoryItem(mapAssignment(item))),
      streakDays: Math.max(0, response.data.streak_days ?? 0),
    };
  },

  async getClassSettings(
    classId: string,
    signal?: AbortSignal,
  ): Promise<DailyQuestionClassSettings> {
    const response = await apiClient.get<DailyQuestionClassSettingsResponse>(
      classDailyQuestionPath(classId, 'settings'),
      { signal },
    );
    return mapClassSettings(response.data);
  },

  async setClassSettings(
    classId: string,
    strategy: DailyQuestionClassStrategy,
  ): Promise<DailyQuestionClassSettings> {
    const response = await apiClient.put<DailyQuestionClassSettingsResponse>(
      classDailyQuestionPath(classId, 'settings'),
      { strategy },
    );
    return mapClassSettings(response.data);
  },

  async setClassAutoReminder(
    classId: string,
    enabled: boolean,
  ): Promise<DailyQuestionClassSettings> {
    const response = await apiClient.put<DailyQuestionClassSettingsResponse>(
      classDailyQuestionPath(classId, 'settings'),
      { auto_reminder_enabled: enabled },
    );
    return mapClassSettings(response.data);
  },

  async getUniformSchedule(
    classId: string,
    signal?: AbortSignal,
  ): Promise<DailyQuestionUniformSchedule> {
    const response = await apiClient.get<DailyQuestionUniformScheduleResponse>(
      classDailyQuestionPath(classId, 'uniform-schedule'),
      { signal },
    );
    return mapUniformSchedule(response.data);
  },

  async setUniformSchedule(
    classId: string,
    scheduleVersion: number,
    contentIds: string[],
  ): Promise<DailyQuestionUniformSchedule> {
    const normalizedScheduleVersion = Math.trunc(scheduleVersion);
    const normalizedContentIds = contentIds.map((contentId) => contentId.trim());
    if (
      !Number.isSafeInteger(normalizedScheduleVersion)
      || normalizedScheduleVersion < 0
      || normalizedContentIds.length > 60
      || normalizedContentIds.some((contentId) => !contentId)
      || new Set(normalizedContentIds).size !== normalizedContentIds.length
    ) {
      throw new Error('统一题日程无效');
    }
    const response = await apiClient.put<DailyQuestionUniformScheduleResponse>(
      classDailyQuestionPath(classId, 'uniform-schedule'),
      {
        schedule_version: normalizedScheduleVersion,
        content_ids: normalizedContentIds,
      },
    );
    return mapUniformSchedule(response.data);
  },

  async getClassStatistics(
    classId: string,
    date: string,
    signal?: AbortSignal,
  ): Promise<DailyQuestionClassStatistics> {
    const response = await apiClient.get<DailyQuestionClassStatisticsResponse>(
      classDailyQuestionPath(classId, 'statistics'),
      { params: { date: requireISODate(date) }, signal },
    );
    const data = response.data;
    return {
      studentCount: Math.max(0, data.student_count ?? 0),
      assignedCount: Math.max(0, data.assigned_count ?? 0),
      completedCount: Math.max(0, data.completed_count ?? 0),
      firstCorrectCount: Math.max(0, data.first_correct_count ?? 0),
      correctedCount: Math.max(0, data.corrected_count ?? 0),
      completionRate: Math.max(0, Math.min(100, data.completion_rate ?? 0)),
      firstCorrectRate: Math.max(0, Math.min(100, data.first_correct_rate ?? 0)),
      correctionRate: Math.max(0, Math.min(100, data.correction_rate ?? 0)),
      weakConcepts: (data.weak_concepts ?? []).map((concept) => ({
        conceptId: concept.concept_id,
        conceptName: concept.concept_name,
        wrongCount: Math.max(0, concept.wrong_count ?? 0),
      })),
    };
  },

  async sendClassReminder(
    classId: string,
    date: string,
  ): Promise<DailyQuestionReminderResult> {
    const response = await apiClient.post<DailyQuestionReminderResponse>(
      classDailyQuestionPath(classId, 'reminders'),
      { date: requireISODate(date) },
    );
    return {
      recipientCount: Math.max(0, response.data.recipient_count ?? 0),
    };
  },
};
