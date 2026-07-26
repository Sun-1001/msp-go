/**
 * 教师统计数据与分析 API 服务
 *
 * 提供教师工作台、学生管理、数据分析、班级分析、学生详情的统计数据
 */

import { apiClient } from '@/libs/http/apiClient';
import { logger } from '@/libs/utils/logger';
import type {
  DashboardStats,
  StudentsStats,
  TeacherStudentListParams,
  TeacherStudentListResponse,
  TeacherAnalyticsData,
  ClassAnalyticsData,
  StudentDetailData,
} from '@/modules/teacher/types/teacher';

const teacherLogger = logger.createContextLogger('Teacher');
const BASE_PATH = '/teacher';

export const teacherService = {
  /**
   * 获取教师工作台统计数据
   */
  async getDashboardStats(): Promise<DashboardStats> {
    try {
      const response = await apiClient.get<DashboardStats>(`${BASE_PATH}/dashboard/stats`);
      return response.data;
    } catch (error) {
      teacherLogger.error('获取工作台统计数据失败', error);
      throw error;
    }
  },

  /**
   * 获取学生管理统计数据
   */
  async getStudentsStats(): Promise<StudentsStats> {
    try {
      const response = await apiClient.get<StudentsStats>(`${BASE_PATH}/students/stats`);
      return response.data;
    } catch (error) {
      teacherLogger.error('获取学生管理统计数据失败', error);
      throw error;
    }
  },

  /**
   * 获取教师学生分页列表
   */
  async getStudents(
    params: TeacherStudentListParams = {},
    signal?: AbortSignal,
  ): Promise<TeacherStudentListResponse> {
    try {
      const response = await apiClient.get<TeacherStudentListResponse>(`${BASE_PATH}/students`, {
        params,
        signal,
      });
      return response.data;
    } catch (error) {
      if (!signal?.aborted) teacherLogger.error('获取学生列表失败', error);
      throw error;
    }
  },

  /**
   * 获取教师数据分析（TeacherDashboardPage）
   */
  async getAnalytics(timeRange: string = 'week'): Promise<TeacherAnalyticsData> {
    try {
      const response = await apiClient.get<TeacherAnalyticsData>(`${BASE_PATH}/analytics`, {
        params: { time_range: timeRange },
      });
      return response.data;
    } catch (error) {
      teacherLogger.error('获取数据分析失败', error);
      throw error;
    }
  },

  /**
   * 获取班级分析数据（ClassDetailPage）
   */
  async getClassAnalytics(classId: string): Promise<ClassAnalyticsData> {
    try {
      const response = await apiClient.get<ClassAnalyticsData>(
        `${BASE_PATH}/classes/${classId}/analytics`
      );
      return response.data;
    } catch (error) {
      teacherLogger.error('获取班级分析数据失败', error);
      throw error;
    }
  },

  /**
   * 获取学生详情（StudentDetailPage）
   */
  async getStudentDetail(studentId: string, signal?: AbortSignal): Promise<StudentDetailData> {
    try {
      const response = await apiClient.get<StudentDetailData>(
        `${BASE_PATH}/students/${studentId}/detail`,
        { signal },
      );
      return response.data;
    } catch (error) {
      if (!signal?.aborted) teacherLogger.error('获取学生详情失败', error);
      throw error;
    }
  },
};

export default teacherService;
