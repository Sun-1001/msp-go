/**
 * 类型定义统一导出
 */

// 通用类型
export type {
  PaginationParams,
  PaginationResponse,
  ApiResponse,
  ApiError,
  Theme,
  UserRole,
  DifficultyLevel,
  LoadingState,
} from './common';

// 数据模型
export type {
  User,
  Student,
  Teacher,
  Exercise,
  LearningSession,
  SessionMessage,
  MistakeRecord,
  DiagnosisReport,
  Course,
  Class,
} from './models';

// API 类型
export type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
  ExerciseListRequest,
  ExerciseListResponse,
  ExerciseSubmitRequest,
  ExerciseSubmitResponse,
  SessionCreateRequest,
  SessionCreateResponse,
  SessionMessageRequest,
  SessionMessageResponse,
  SessionListResponse,
  MistakeBookListRequest,
  MistakeBookListResponse,
  DiagnosisReportResponse,
  CourseListResponse,
  CourseDetailResponse,
  ClassListResponse,
  ClassDetailResponse,
} from './api';

// 管理员统计类型
export type {
  TrendData,
  OverviewStats,
  UserGrowthDataPoint,
  UserGrowthSummary,
  UserGrowthResponse,
  ActivityItem,
  RecentActivitiesResponse,
  ServiceStatus,
  SystemAlert,
  SystemStatusResponse,
  UserGrowthPeriod,
} from '@/modules/admin/api';
