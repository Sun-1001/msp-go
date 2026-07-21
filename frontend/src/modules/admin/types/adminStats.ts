/**
 * 管理员统计相关类型定义
 */

// =============================================================================
// 趋势数据
// =============================================================================

export interface TrendData {
  /** 用户数变化百分比 */
  users_change: number;
  /** 学生数变化百分比 */
  students_change: number;
  /** 教师数变化百分比 */
  teachers_change: number;
  /** 活跃率变化百分比 */
  active_rate_change: number;
}

// =============================================================================
// 概览统计
// =============================================================================

export interface OverviewStats {
  /** 总用户数 */
  total_users: number;
  /** 学生数量 */
  student_count: number;
  /** 教师数量 */
  teacher_count: number;
  /** 管理员数量 */
  admin_count: number;
  /** 今日活跃用户数 */
  active_users_today: number;
  /** 活跃率（百分比） */
  active_rate: number;
  /** 趋势数据 */
  trends: TrendData;
}

// =============================================================================
// 用户增长
// =============================================================================

export interface UserGrowthDataPoint {
  /** 日期 (YYYY-MM-DD) */
  date: string;
  /** 累计总用户数 */
  total: number;
  /** 累计学生数 */
  students: number;
  /** 累计教师数 */
  teachers: number;
}

export interface UserGrowthSummary {
  /** 期间新增用户总数 */
  total_new_users: number;
  /** 日均增长数 */
  avg_daily_growth: number;
}

export interface UserGrowthResponse {
  /** 统计周期 (7d/30d/90d) */
  period: string;
  /** 增长数据点列表 */
  data: UserGrowthDataPoint[];
  /** 增长摘要 */
  summary: UserGrowthSummary;
}

// =============================================================================
// 最近活动
// =============================================================================

export interface ActivityItem {
  /** 活动 ID */
  id: string;
  /** 用户名 */
  user_name: string;
  /** 操作描述 */
  action_display: string;
  /** 时间戳 */
  timestamp: string;
  /** 活动类型 (success/info/warning) */
  type: 'success' | 'info' | 'warning';
}

export interface RecentActivitiesResponse {
  /** 活动列表 */
  items: ActivityItem[];
  /** 总数 */
  total: number;
}

// =============================================================================
// 系统状态
// =============================================================================

export interface ServiceStatus {
  /** 服务名称 */
  name: string;
  /** 状态 (running/stopped/warning) */
  status: 'running' | 'stopped' | 'warning';
  /** 延迟（毫秒） */
  latency_ms: number | null;
}

export interface SystemAlert {
  /** 警告 ID */
  id: string;
  /** 标题 */
  title: string;
  /** 描述 */
  description: string;
  /** 严重程度 (error/warning/info) */
  severity: 'error' | 'warning' | 'info';
}

export interface RuntimeStatus {
  version: string;
  environment: string;
  started_at: string;
  uptime_seconds: number;
  cpu_usage_percent: number;
  heap_used_bytes: number;
  heap_reserved_bytes: number;
  heap_usage_percent: number;
  goroutines: number;
  logical_cpus: number;
  gomaxprocs: number;
  go_version: string;
  os: string;
  arch: string;
}

export interface TrafficStatus {
  window_started_at: string;
  window_seconds: number;
  requests_total: number;
  average_qps: number;
  client_errors_total: number;
  client_error_rate_percent: number;
  server_errors_total: number;
  server_error_rate_percent: number;
  average_latency_ms: number;
  p95_latency_ms: number;
  p95_clamped: boolean;
}

export interface ResetTrafficMetricsResponse {
  success: boolean;
  message: string;
  reset_at: string;
}

export interface LearningStatus {
  /** 当前存在未结束学习会话的学生数；null 表示采集失败。 */
  online_users: number | null;
  /** 当前未结束的学习会话数；null 表示采集失败。 */
  active_sessions: number | null;
}

export interface DatabaseStatus {
  max_connections: number;
  total_connections: number;
  acquired_connections: number;
  idle_connections: number;
  usage_percent: number;
  empty_acquire_count: number;
  canceled_acquire_count: number;
}

export interface RedisStatus {
  max_connections: number;
  total_connections: number;
  idle_connections: number;
  stale_connections: number;
  usage_percent: number;
  reuse_percent: number;
  timeouts: number;
  wait_count: number;
  unusable: number;
}

export interface SystemStatusResponse {
  /** healthy/degraded/unhealthy */
  status: 'healthy' | 'degraded' | 'unhealthy';
  checked_at: string;
  /** 服务状态列表 */
  services: ServiceStatus[];
  /** 系统警告列表 */
  alerts: SystemAlert[];
  runtime: RuntimeStatus;
  traffic: TrafficStatus;
  learning: LearningStatus;
  postgresql: DatabaseStatus;
  redis: RedisStatus;
}

// =============================================================================
// 用户增长周期类型
// =============================================================================

export type UserGrowthPeriod = '7d' | '30d' | '90d';
