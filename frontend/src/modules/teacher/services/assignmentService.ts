/**
 * 作业管理服务
 *
 * Go 后端尚未提供作业管理路由；这里保持页面可渲染的空状态，
 * 避免前端持续请求不存在的 /teacher/assignments API。
 */

// ========== 类型定义 ==========

export interface Assignment {
  id: string;
  title: string;
  description: string;
  status: 'active' | 'ended' | 'draft';
  dueDate: string | null;
  createdAt: string;
  totalStudents: number;
  submitted: number;
  graded: number;
  questions: number;
  averageScore: number | null;
}

export interface AssignmentListResponse {
  items: Assignment[];
  total: number;
  page: number;
  pageSize: number;
}

export interface AssignmentStats {
  total: number;
  active: number;
  pending: number;
}

export const assignmentService = {
  /**
   * 获取作业列表
   */
  async list(params: {
    page?: number;
    pageSize?: number;
    status?: string;
  } = {}): Promise<AssignmentListResponse> {
    return {
      items: [],
      total: 0,
      page: params.page || 1,
      pageSize: params.pageSize || 20,
    };
  },

  /**
   * 获取作业统计
   */
  async getStats(): Promise<AssignmentStats> {
    return { total: 0, active: 0, pending: 0 };
  },
};
