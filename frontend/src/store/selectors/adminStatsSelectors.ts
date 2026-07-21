/**
 * 管理员统计 Selectors
 *
 * 使用 createSelector 实现记忆化，减少不必要的重渲染
 */

import { createSelector } from '@reduxjs/toolkit';
import type { RootState } from '../index';

// =============================================================================
// 基础 Selector
// =============================================================================

const selectAdminStatsState = (state: RootState) => state.adminStats;

// =============================================================================
// 记忆化 Selectors - 按功能区域拆分
// =============================================================================

/**
 * 概览数据 Selector
 * 仅当 overview 或 overviewLoading 变化时才重新计算
 */
export const selectOverviewData = createSelector(
  [selectAdminStatsState],
  (adminStats) => ({
    overview: adminStats.overview,
    overviewLoading: adminStats.overviewLoading,
    overviewError: adminStats.overviewError,
  })
);

/**
 * 用户增长数据 Selector
 */
export const selectUserGrowthData = createSelector(
  [selectAdminStatsState],
  (adminStats) => ({
    userGrowth: adminStats.userGrowth,
    userGrowthLoading: adminStats.userGrowthLoading,
    userGrowthError: adminStats.userGrowthError,
    userGrowthPeriod: adminStats.userGrowthPeriod,
  })
);

/**
 * 最近活动数据 Selector
 */
export const selectActivitiesData = createSelector(
  [selectAdminStatsState],
  (adminStats) => ({
    recentActivities: adminStats.recentActivities,
    activitiesLoading: adminStats.activitiesLoading,
    activitiesError: adminStats.activitiesError,
  })
);

/**
 * 系统状态数据 Selector
 */
export const selectSystemStatusData = createSelector(
  [selectAdminStatsState],
  (adminStats) => ({
    systemStatus: adminStats.systemStatus,
    systemStatusLoading: adminStats.systemStatusLoading,
    systemStatusError: adminStats.systemStatusError,
    trafficResetLoading: adminStats.trafficResetLoading,
    trafficResetError: adminStats.trafficResetError,
  })
);

/**
 * 是否有任何数据正在加载
 */
export const selectIsAnyLoading = createSelector(
  [selectAdminStatsState],
  (adminStats) =>
    adminStats.overviewLoading === 'loading' ||
    adminStats.userGrowthLoading === 'loading' ||
    adminStats.activitiesLoading === 'loading' ||
    adminStats.systemStatusLoading === 'loading' ||
    adminStats.trafficResetLoading === 'loading'
);

/**
 * 概览统计卡片数据 - 用于 StatCard 组件
 */
export const selectStatCardsData = createSelector(
  [selectOverviewData],
  ({ overview, overviewLoading }) => ({
    totalUsers: overview?.total_users,
    studentCount: overview?.student_count,
    teacherCount: overview?.teacher_count,
    activeRate: overview?.active_rate,
    trends: overview?.trends,
    isLoading: overviewLoading === 'loading',
  })
);
