import {
  GraduationCap,
  LayoutDashboard,
  MessageSquare,
  GitBranch,
  BarChart3,
  FolderOpen,
  Users,
  FileText,
} from 'lucide-react';

/**
 * 导航菜单项接口
 */
export interface NavItem {
  label: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
}

/**
 * 学生导航菜单配置
 *
 * 设计原则：
 * - KISS: 简单的配置对象，易于维护
 * - 单一职责: 只负责导航配置，不包含业务逻辑
 */
export const studentNavItems: NavItem[] = [
  { label: '我的班级', href: '/my-class', icon: Users },
  { label: '智能刷题', href: '/exercise', icon: GraduationCap },
  { label: 'AI 辅导', href: '/session/new', icon: MessageSquare },
  { label: '错题本', href: '/mistake-book', icon: FileText },
  { label: '知识图谱', href: '/knowledge-graph', icon: GitBranch },
  { label: '学习统计', href: '/analytics', icon: BarChart3 },
  { label: '资源中心', href: '/resources', icon: FolderOpen },
];

/**
 * 教师导航菜单配置
 */
export const teacherNavItems: NavItem[] = [
  { label: '教学概览', href: '/teacher/dashboard', icon: LayoutDashboard },
  { label: '班级管理', href: '/teacher/classes', icon: Users },
  { label: '学生管理', href: '/teacher/students', icon: Users },
  { label: '题库管理', href: '/teacher/question-bank', icon: FileText },
  { label: '资源管理', href: '/teacher/resources', icon: FolderOpen },
];

/**
 * 根据用户角色获取导航菜单
 *
 * @param role - 用户角色
 * @returns 导航菜单项数组
 */
export const getNavItemsByRole = (role?: string): NavItem[] => {
  return role === 'teacher' ? teacherNavItems : studentNavItems;
};
