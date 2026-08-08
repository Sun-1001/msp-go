import type { ChatMode } from '@/modules/session/types';
import { GraduationCap, MessageCircle, Target, Lightbulb, Calculator, TrendingUp, Brain, HelpCircle } from 'lucide-react';

export interface ModeConfig {
  id: ChatMode;
  name: string;
  description: string;
  icon: React.ReactNode;
  color: string;
  bgColor: string;
}

export interface QuickAction {
  icon: React.ReactNode;
  label: string;
  prompt: string;
}

export const CHAT_MODES: ModeConfig[] = [
  {
    id: 'study',
    name: '学习模式',
    description: '系统化学习，逐步引导',
    icon: <GraduationCap className="w-5 h-5" />,
    color: 'text-blue-600 dark:text-blue-400',
    bgColor: 'bg-blue-50 dark:bg-blue-900/30',
  },
  {
    id: 'chat',
    name: '聊天模式',
    description: '自由对话，快速答疑',
    icon: <MessageCircle className="w-5 h-5" />,
    color: 'text-emerald-600 dark:text-emerald-400',
    bgColor: 'bg-emerald-50 dark:bg-emerald-900/30',
  },
  {
    id: 'practice',
    name: '练习模式',
    description: '刷题训练，巩固知识',
    icon: <Target className="w-5 h-5" />,
    color: 'text-orange-600 dark:text-orange-400',
    bgColor: 'bg-orange-50 dark:bg-orange-900/30',
  },
  {
    id: 'explain',
    name: '讲解模式',
    description: '深入讲解，透彻理解',
    icon: <Lightbulb className="w-5 h-5" />,
    color: 'text-purple-600 dark:text-purple-400',
    bgColor: 'bg-purple-50 dark:bg-purple-900/30',
  },
];

export const QUICK_ACTIONS: QuickAction[] = [
  { icon: <Calculator className="w-4 h-4" />, label: '解方程', prompt: '帮我解这个方程' },
  { icon: <TrendingUp className="w-4 h-4" />, label: '求导数', prompt: '帮我求这个函数的导数' },
  { icon: <Brain className="w-4 h-4" />, label: '求积分', prompt: '帮我计算这个积分' },
  { icon: <HelpCircle className="w-4 h-4" />, label: '解释概念', prompt: '请解释一下' },
];
