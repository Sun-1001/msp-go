import type { Question } from '@/modules/exercise/services/exerciseService';

export type DailyQuestionStatus =
  | 'not_started'
  | 'preparing'
  | 'ready'
  | 'completed'
  | 'unavailable';

export type DailyQuestionFirstResult = 'correct' | 'incorrect';

export interface DailyQuestionAssignment {
  status: DailyQuestionStatus;
  assignmentId: string | null;
  assignmentDate: string;
  targetConceptName: string | null;
  source: string | null;
  selectionReason: string | null;
  firstAttemptId: string | null;
  correctedAttemptId: string | null;
  firstResult: DailyQuestionFirstResult | null;
  openedAt: string | null;
  countsTowardStreak: boolean;
  streakDays: number;
  failureCode: string | null;
  question: Question | null;
}

export type DailyQuestionHistoryItem = Omit<DailyQuestionAssignment, 'question'>;

export interface DailyQuestionHistory {
  items: DailyQuestionHistoryItem[];
  streakDays: number;
}

export type DailyQuestionClassStrategy = 'personalized' | 'uniform';

export interface DailyQuestionClassSettings {
  strategy: DailyQuestionClassStrategy;
  effectiveStrategy: DailyQuestionClassStrategy;
  effectiveDate: string;
  todayAssignmentCount: number;
  uniformReady: boolean;
  autoReminderEnabled: boolean;
  todayReminderSent: boolean;
  todayReminderRecipientCount: number;
}

export interface DailyQuestionUniformScheduleItem {
  assignmentDate: string;
  contentId: string;
  targetConceptId: string | null;
  title: string;
  body: string;
  difficulty: number;
  locked: boolean;
}

export interface DailyQuestionUniformSchedule {
  startDate: string;
  scheduleVersion: number;
  items: DailyQuestionUniformScheduleItem[];
}

interface DailyQuestionWeakConcept {
  conceptId: string;
  conceptName: string;
  wrongCount: number;
}

export interface DailyQuestionClassStatistics {
  studentCount: number;
  assignedCount: number;
  completedCount: number;
  firstCorrectCount: number;
  correctedCount: number;
  completionRate: number;
  firstCorrectRate: number;
  correctionRate: number;
  weakConcepts: DailyQuestionWeakConcept[];
}

export interface DailyQuestionReminderResult {
  recipientCount: number;
}

export type DailyQuestionTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger';

interface DailyQuestionPresentation {
  label: string;
  description: string;
  actionLabel: string;
  tone: DailyQuestionTone;
}

export function getDailyQuestionPresentation(
  assignment: Pick<
    DailyQuestionAssignment,
    'status' | 'openedAt' | 'firstResult' | 'correctedAttemptId' | 'streakDays' | 'failureCode'
  > | null,
): DailyQuestionPresentation {
  if (!assignment) {
    return {
      label: '暂不可用',
      description: '每日一题状态暂时无法加载',
      actionLabel: '查看详情',
      tone: 'danger',
    };
  }

  if (assignment.failureCode === 'teacher_not_assigned') {
    return {
      label: '老师未布置',
      description: '老师今天还没有布置班级统一题',
      actionLabel: '查看安排',
      tone: 'warning',
    };
  }

  switch (assignment.status) {
    case 'not_started':
      return {
        label: '未开始',
        description: '今天的题目等待你开启',
        actionLabel: '开始今日一题',
        tone: 'neutral',
      };
    case 'preparing':
      return {
        label: '准备中',
        description: 'AI 正在生成并验证今天的题目',
        actionLabel: '查看生成状态',
        tone: 'info',
      };
    case 'ready':
      return assignment.openedAt
        ? {
            label: '作答中',
            description: '今天的固定题目正在等待提交',
            actionLabel: '继续作答',
            tone: 'info',
          }
        : {
            label: '未开始',
            description: '今天的固定题目已经准备好',
            actionLabel: '开始今日一题',
            tone: 'neutral',
          };
    case 'completed':
      if (assignment.firstResult === 'incorrect') {
        return assignment.correctedAttemptId
          ? {
              label: '已完成，已订正',
              description: '首次正确率已记录，订正也已完成',
              actionLabel: '查看今日记录',
              tone: 'success',
            }
          : {
              label: '已完成，待订正',
              description: '答错也计入连续完成，建议及时订正',
              actionLabel: '查看诊断并订正',
              tone: 'warning',
            };
      }
      return {
        label: '已完成',
        description: `今日已答对，连续完成 ${Math.max(0, assignment.streakDays)} 天`,
        actionLabel: '查看今日记录',
        tone: 'success',
      };
    case 'unavailable':
      return {
        label: '暂不可用',
        description: '今天的题目暂时无法准备，可以进入页面重试',
        actionLabel: '查看并重试',
        tone: 'danger',
      };
  }
}
