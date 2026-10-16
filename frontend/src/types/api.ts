/**
 * API 请求和响应类型定义
 */

import type {
  User,
  Exercise,
  LearningSession,
  SessionMessage,
  MistakeRecord,
  DiagnosisReport,
  Course,
  Class,
} from './models';
import type { PaginationParams, PaginationResponse } from './common';

// ==================== 认证相关 ====================

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  user: User;
  token: string;
  refreshToken: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  role: 'student' | 'teacher';
}

export interface RegisterResponse {
  user: User;
  token: string;
  refreshToken: string;
}

// ==================== 练习相关 ====================

export interface ExerciseListRequest extends PaginationParams {
  difficulty?: string;
  conceptId?: string;
  search?: string;
}

export type ExerciseListResponse = PaginationResponse<Exercise>;

export interface ExerciseSubmitRequest {
  exerciseId: string;
  answer: string;
}

export interface ExerciseSubmitResponse {
  correct: boolean;
  feedback: string;
  solution?: string;
  diagnosisReportId?: string;
}

// ==================== 学习会话相关 ====================

export interface SessionCreateRequest {
  title?: string;
  initialMessage?: string;
}

export type SessionCreateResponse = LearningSession;

export interface SessionMessageRequest {
  sessionId: string;
  content: string;
}

export type SessionMessageResponse = SessionMessage;

export type SessionListResponse = PaginationResponse<LearningSession>;

// ==================== 错题本相关 ====================

export interface MistakeBookListRequest extends PaginationParams {
  masteredOnly?: boolean;
  conceptId?: string;
}

export type MistakeBookListResponse = PaginationResponse<MistakeRecord>;

// ==================== 诊断报告相关 ====================

export type DiagnosisReportResponse = DiagnosisReport;

// ==================== 课程相关 ====================

export type CourseListResponse = PaginationResponse<Course>;

export type CourseDetailResponse = Course;

// ==================== 班级相关 ====================

export type ClassListResponse = PaginationResponse<Class>;

export type ClassDetailResponse = Class;
