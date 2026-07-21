import { lazy } from 'react';
import type { RouteConfig } from './index';

// 学生页面懒加载
const ExercisePage = lazy(() => import('@/pages/student/ExercisePage').then(m => ({ default: m.ExercisePage })));
const SessionChatPage = lazy(() => import('@/pages/student/SessionChatPage').then(m => ({ default: m.default })));
const MistakeBookPage = lazy(() => import('@/pages/student/MistakeBookPage').then(m => ({ default: m.MistakeBookPage })));
const KnowledgeGraphPage = lazy(() => import('@/pages/student/KnowledgeGraphPage').then(m => ({ default: m.KnowledgeGraphPage })));
const LearningPathPage = lazy(() => import('@/pages/student/LearningPathPage').then(m => ({ default: m.LearningPathPage })));
const DiagnosisReportPage = lazy(() => import('@/pages/student/DiagnosisReportPage').then(m => ({ default: m.DiagnosisReportPage })));
const AnalyticsPage = lazy(() => import('@/pages/student/AnalyticsPage').then(m => ({ default: m.AnalyticsPage })));
const ResourcesPage = lazy(() => import('@/pages/student/ResourcesPage').then(m => ({ default: m.ResourcesPage })));
const MyClassPage = lazy(() => import('@/pages/student/MyClassPage').then(m => ({ default: m.MyClassPage })));

/**
 * 学生路由 - 需要登录 + student 角色
 */
export const studentRoutes: RouteConfig[] = [
  { path: '/my-class', component: MyClassPage, title: '我的班级', protected: true, requiredRole: 'student' },
  { path: '/exercise', component: ExercisePage, title: '智能刷题', protected: true, requiredRole: 'student' },
  { path: '/session/new', component: SessionChatPage, title: '新建学习会话', protected: true, requiredRole: 'student' },
  { path: '/session/:sessionId', component: SessionChatPage, title: '学习会话', protected: true, requiredRole: 'student' },
  { path: '/mistake-book', component: MistakeBookPage, title: '错题本', protected: true, requiredRole: 'student' },
  { path: '/knowledge-graph', component: KnowledgeGraphPage, title: '知识图谱', protected: true, requiredRole: 'student' },
  { path: '/learning-path', component: LearningPathPage, title: '学习路径', protected: true, requiredRole: 'student' },
  { path: '/diagnosis/:id', component: DiagnosisReportPage, title: '诊断报告', protected: true, requiredRole: 'student' },
  { path: '/analytics', component: AnalyticsPage, title: '学习分析', protected: true, requiredRole: 'student' },
  { path: '/resources', component: ResourcesPage, title: '学习资源', protected: true, requiredRole: 'student' },
];
