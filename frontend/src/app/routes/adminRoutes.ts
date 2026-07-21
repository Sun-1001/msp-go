import { lazy } from 'react';
import type { RouteConfig } from './index';

// 管理员页面懒加载
const AdminLoginPage = lazy(() => import('@/pages/admin/AdminLoginPage').then(m => ({ default: m.AdminLoginPage })));
const AdminDashboardPage = lazy(() => import('@/pages/admin/AdminDashboardPage').then(m => ({ default: m.AdminDashboardPage })));
const AccountManagementPage = lazy(() => import('@/pages/admin/AccountManagementPage').then(m => ({ default: m.AccountManagementPage })));
const AIModelSettingsPage = lazy(() => import('@/pages/admin/AIModelSettingsPage').then(m => ({ default: m.AIModelSettingsPage })));
const SystemSettingsPage = lazy(() => import('@/pages/admin/SystemSettingsPage').then(m => ({ default: m.SystemSettingsPage })));
const KnowledgeManagementPage = lazy(() => import('@/pages/admin/KnowledgeManagementPage').then(m => ({ default: m.KnowledgeManagementPage })));
const InboxPage = lazy(() => import('@/pages/admin/InboxPage').then(m => ({ default: m.InboxPage })));
const AIRiskControlPage = lazy(() => import('@/pages/admin/AIRiskControlPage').then(m => ({ default: m.AIRiskControlPage })));

/**
 * 管理员路由 - 需要登录 + admin 角色
 */
export const adminRoutes: RouteConfig[] = [
  { path: '/admin', component: AdminLoginPage, title: '管理员登录', protected: false },
  { path: '/admin/dashboard', component: AdminDashboardPage, title: '运维控制台', protected: true, requiredRole: 'admin' },
  { path: '/admin/inbox', component: InboxPage, title: '信箱', protected: true, requiredRole: 'admin' },
  { path: '/admin/accounts', component: AccountManagementPage, title: '账户管理', protected: true, requiredRole: 'admin' },
  { path: '/admin/ai-models', component: AIModelSettingsPage, title: 'AI 模型设置', protected: true, requiredRole: 'admin' },
  { path: '/admin/risk-control', component: AIRiskControlPage, title: 'AI 风控中心', protected: true, requiredRole: 'admin' },
  { path: '/admin/settings', component: SystemSettingsPage, title: '系统设置', protected: true, requiredRole: 'admin' },
  { path: '/admin/knowledge', component: KnowledgeManagementPage, title: '知识点管理', protected: true, requiredRole: 'admin' },
];
