import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { Card, CardContent } from '../../components/ui/Card';
import { cn } from '../../libs/utils/cn';
import {
  ArrowLeft,
  BookOpen,
  Brain,
  Target,
  MessageSquare,
  BarChart3,
  FileQuestion,
  GraduationCap,
  Users,
  Library,
  TrendingUp,
  AlertTriangle,
  FolderOpen,
  PenTool,
} from 'lucide-react';

type TabType = 'student' | 'teacher';

interface FeatureItem {
  icon: React.ElementType;
  title: string;
  description: string;
}

interface QuickStartStep {
  step: number;
  content: string;
}

// 学生端功能
const studentFeatures: FeatureItem[] = [
  {
    icon: BookOpen,
    title: '智能刷题',
    description: '根据你的学习进度和薄弱点，智能推荐练习题目。支持选择题、填空题和解答题等多种题型，AI 会根据你的答题情况动态调整难度。',
  },
  {
    icon: Brain,
    title: 'AI 学习助手',
    description: '与 AI 助手进行对话式学习，获得个性化的解题指导和概念讲解。支持 LaTeX 数学公式输入和图片上传，AI 能够识别手写题目并给出详细解答。',
  },
  {
    icon: Target,
    title: '知识关联',
    description: '上传学习材料，编写个人笔记并插入笔记链接。在二维网络中查看关联与反向链接，点击笔记即可打开。',
  },
  {
    icon: FileQuestion,
    title: '错题本',
    description: '自动收集做错的题目，支持按知识点、难度、时间等维度分类查看。可以重新练习错题，系统会追踪你的改进情况。',
  },
  {
    icon: MessageSquare,
    title: '学习会话',
    description: '创建学习会话，与 AI 进行深度交流。支持多轮对话，AI 会记住上下文，采用苏格拉底式教学法引导你思考，而非直接给出答案。',
  },
  {
    icon: BarChart3,
    title: '学习分析',
    description: '查看详细的学习数据统计，包括学习时长、答题正确率、知识点掌握度、学习趋势等，帮助你了解学习进展并制定改进计划。',
  },
  {
    icon: TrendingUp,
    title: '学习路径',
    description: '基于深度知识追踪 (DKT) 模型，系统会为你规划个性化的学习路径，推荐下一步应该学习的知识点，实现高效学习。',
  },
  {
    icon: FolderOpen,
    title: '学习资源',
    description: '浏览和下载教师上传的学习资料，包括课件、习题集、参考答案等。支持收藏常用资源，方便随时查阅。',
  },
];

// 教师端功能
const teacherFeatures: FeatureItem[] = [
  {
    icon: Users,
    title: '班级管理',
    description: '创建和管理多个班级，查看班级整体学情。支持批量导入学生名单，一键查看班级成员的学习进度和成绩分布。',
  },
  {
    icon: GraduationCap,
    title: '学生分析',
    description: '深入了解每位学生的学习情况，包括知识点掌握度、答题正确率、学习时长等。系统会自动识别学习困难的学生并发出预警。',
  },
  {
    icon: Library,
    title: '题库管理',
    description: '创建、编辑和管理题目，支持 LaTeX 公式编辑。可按知识点、难度、题型分类组织题目，支持批量导入导出。',
  },
  {
    icon: AlertTriangle,
    title: '学情预警',
    description: '系统自动监测学生学习状态，对连续低活跃、成绩下滑等情况发出预警，帮助教师及时干预和辅导。',
  },
  {
    icon: BarChart3,
    title: '教学分析',
    description: '查看班级整体的学习数据分析，包括知识点掌握分布、练习参与率和成绩趋势等，为教学决策提供数据支持。',
  },
  {
    icon: FolderOpen,
    title: '教学资源',
    description: '上传和管理教学资源，包括课件、讲义、习题集等。可设置资源的可见范围，支持按班级或全平台共享。',
  },
  {
    icon: PenTool,
    title: '题目编辑',
    description: '强大的题目编辑器，支持富文本和 LaTeX 公式混排。可添加题目解析、知识点标签、难度评级，方便后续检索和组卷。',
  },
];

// 学生端快速开始
const studentQuickStart: QuickStartStep[] = [
  { step: 1, content: '使用学号和密码登录平台，首次登录需要完成注册并绑定班级。' },
  { step: 2, content: '进入「课程总览」查看当前学习进度，了解推荐的学习内容。' },
  { step: 3, content: '点击「智能刷题」开始练习，系统会根据你的水平推荐合适的题目。' },
  { step: 4, content: '遇到不懂的问题，点击「学习会话」与 AI 助手交流，获得个性化指导。' },
  { step: 5, content: '定期查看「错题本」复习做错的题目，巩固薄弱知识点。' },
  { step: 6, content: '通过「学习分析」了解自己的学习进展，调整学习策略。' },
];

// 教师端快速开始
const teacherQuickStart: QuickStartStep[] = [
  { step: 1, content: '使用教师账号登录平台，首次登录请联系管理员开通权限。' },
  { step: 2, content: '进入「教师工作台」查看整体教学数据和待处理事项。' },
  { step: 3, content: '在「班级管理」中创建班级或导入学生名单，建立教学班级。' },
  { step: 4, content: '使用「题库管理」添加题目，可手动创建或批量导入。' },
  { step: 5, content: '查看「学情预警」关注需要帮助的学生，及时进行辅导干预。' },
];

// 学生端使用技巧
const studentTips = [
  '在学习会话中，可以上传题目图片让 AI 识别并解答',
  '使用 $...$ 包裹 LaTeX 公式，如 $x^2$ 显示为 x 的平方',
  '错题本支持按「最近错误」排序，优先复习近期的薄弱点',
  '在笔记中插入内部链接并保存，知识关联网络会自动更新',
  '学习会话支持多轮对话，AI 会记住上下文帮你深入理解',
];

// 教师端使用技巧
const teacherTips = [
  '批量导入学生时，使用 Excel 模板可以快速完成',
  '题目支持设置多个知识点标签，方便后续按知识点组卷',
  '学情预警支持自定义规则，可根据班级情况调整阈值',
  '教学资源可以设置「仅本班可见」或「全平台共享」',
];

export const GuidePage: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabType>('student');

  const features = activeTab === 'student' ? studentFeatures : teacherFeatures;
  const quickStart = activeTab === 'student' ? studentQuickStart : teacherQuickStart;
  const tips = activeTab === 'student' ? studentTips : teacherTips;

  return (
    <div className="min-h-screen bg-surface-50 dark:bg-surface-950 text-surface-900 dark:text-surface-100">
      <div className="container mx-auto px-6 py-8 max-w-5xl">
        <Button
          variant="ghost"
          className="mb-6 pl-0 hover:bg-transparent hover:text-primary-600 dark:hover:text-primary-400"
          onClick={() => navigate('/')}
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          返回主界面
        </Button>

        <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100">
            使用指南
          </h1>
          <p className="mt-3 text-surface-500 dark:text-surface-400">
            了解高数智学平台的核心功能，开启高效学习之旅
          </p>
        </div>

        {/* Tab 切换 */}
        <div className="flex justify-center mb-8">
          <div className="inline-flex rounded-lg bg-surface-100 dark:bg-surface-800 p-1">
            <button
              onClick={() => setActiveTab('student')}
              className={cn(
                'px-6 py-2.5 rounded-md text-sm font-medium transition-all duration-200',
                activeTab === 'student'
                  ? 'bg-white dark:bg-surface-700 text-primary-600 dark:text-primary-400 shadow-sm'
                  : 'text-surface-600 dark:text-surface-400 hover:text-surface-900 dark:hover:text-surface-200'
              )}
            >
              <GraduationCap className="w-4 h-4 inline-block mr-2 -mt-0.5" />
              学生端
            </button>
            <button
              onClick={() => setActiveTab('teacher')}
              className={cn(
                'px-6 py-2.5 rounded-md text-sm font-medium transition-all duration-200',
                activeTab === 'teacher'
                  ? 'bg-white dark:bg-surface-700 text-primary-600 dark:text-primary-400 shadow-sm'
                  : 'text-surface-600 dark:text-surface-400 hover:text-surface-900 dark:hover:text-surface-200'
              )}
            >
              <Users className="w-4 h-4 inline-block mr-2 -mt-0.5" />
              教师端
            </button>
          </div>
        </div>

        {/* 功能介绍 */}
        <div className="mb-8">
          <h2 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-4">
            {activeTab === 'student' ? '学生功能' : '教师功能'}
          </h2>
          <div className="grid gap-4 md:grid-cols-2">
            {features.map((feature) => (
              <Card key={feature.title} className="hover:shadow-md transition-shadow">
                <CardContent className="p-5">
                  <div className="flex items-start gap-4">
                    <div className={cn(
                      'shrink-0 w-10 h-10 rounded-lg flex items-center justify-center',
                      activeTab === 'student'
                        ? 'bg-primary-100 dark:bg-primary-900/30'
                        : 'bg-secondary-100 dark:bg-secondary-900/30'
                    )}>
                      <feature.icon className={cn(
                        'w-5 h-5',
                        activeTab === 'student'
                          ? 'text-primary-600 dark:text-primary-400'
                          : 'text-secondary-600 dark:text-secondary-400'
                      )} />
                    </div>
                    <div>
                      <h3 className="font-semibold text-surface-900 dark:text-surface-100 mb-1.5">
                        {feature.title}
                      </h3>
                      <p className="text-sm text-surface-600 dark:text-surface-400 leading-relaxed">
                        {feature.description}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>

        {/* 快速开始 */}
        <Card className="mb-8">
          <CardContent className="p-6">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">
              快速开始
            </h2>
            <ol className="space-y-3 text-sm text-surface-600 dark:text-surface-400">
              {quickStart.map((item) => (
                <li key={item.step} className="flex gap-3">
                  <span className={cn(
                    'shrink-0 w-6 h-6 text-white rounded-full flex items-center justify-center text-xs font-medium',
                    activeTab === 'student' ? 'bg-primary-500' : 'bg-secondary-500'
                  )}>
                    {item.step}
                  </span>
                  <span className="pt-0.5">{item.content}</span>
                </li>
              ))}
            </ol>
          </CardContent>
        </Card>

        {/* 使用技巧 */}
        <Card>
          <CardContent className="p-6">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">
              使用技巧
            </h2>
            <ul className="space-y-2.5 text-sm text-surface-600 dark:text-surface-400">
              {tips.map((tip, index) => (
                <li key={index} className="flex gap-3">
                  <span className={cn(
                    'shrink-0 w-1.5 h-1.5 rounded-full mt-2',
                    activeTab === 'student' ? 'bg-primary-500' : 'bg-secondary-500'
                  )} />
                  <span>{tip}</span>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>

        {/* 底部帮助链接 */}
        <div className="mt-8 text-center text-sm text-surface-500 dark:text-surface-400">
          <p>还有其他问题？</p>
          <div className="flex justify-center gap-4 mt-2">
            <Button
              variant="link"
              className="text-primary-600 dark:text-primary-400"
              onClick={() => navigate('/faq')}
            >
              查看常见问题
            </Button>
            <Button
              variant="link"
              className="text-primary-600 dark:text-primary-400"
              onClick={() => navigate('/contact')}
            >
              联系我们
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};
