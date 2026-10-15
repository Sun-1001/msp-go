import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { Card, CardContent } from '../../components/ui/Card';
import {
  ArrowLeft,
  Target,
  Users,
  Lightbulb,
  GraduationCap,
  BookOpen,
  Brain,
  Cpu,
  Shield,
  Zap,
  Award,
  Calendar,
  Building2,
  Heart,
} from 'lucide-react';

interface StatItemProps {
  value: string;
  label: string;
  icon: React.ElementType;
}

const StatItem: React.FC<StatItemProps> = ({ value, label, icon: Icon }) => (
  <div className="text-center p-4">
    <div className="inline-flex items-center justify-center w-12 h-12 bg-primary-100 dark:bg-primary-900/30 rounded-full mb-3">
      <Icon className="w-6 h-6 text-primary-600 dark:text-primary-400" />
    </div>
    <div className="text-2xl font-bold text-surface-900 dark:text-surface-100">{value}</div>
    <div className="text-sm text-surface-500 dark:text-surface-400">{label}</div>
  </div>
);

interface TimelineItemProps {
  date: string;
  title: string;
  description: string;
  isLast?: boolean;
}

const TimelineItem: React.FC<TimelineItemProps> = ({ date, title, description, isLast }) => (
  <div className="relative pl-8 pb-6">
    {!isLast && (
      <div className="absolute left-[11px] top-6 bottom-0 w-0.5 bg-surface-200 dark:bg-surface-700" />
    )}
    <div className="absolute left-0 top-1 w-6 h-6 bg-primary-500 rounded-full flex items-center justify-center">
      <Calendar className="w-3 h-3 text-white" />
    </div>
    <div className="text-xs text-primary-600 dark:text-primary-400 font-medium mb-1">{date}</div>
    <div className="font-semibold text-surface-900 dark:text-surface-100 mb-1">{title}</div>
    <div className="text-sm text-surface-600 dark:text-surface-400">{description}</div>
  </div>
);

interface TeamMemberProps {
  name: string;
  role: string;
  description: string;
}

const TeamMember: React.FC<TeamMemberProps> = ({ name, role, description }) => (
  <div className="p-4 bg-surface-50 dark:bg-surface-800/50 rounded-lg border border-surface-200 dark:border-surface-700">
    <div className="flex items-center gap-3 mb-2">
      <div className="w-10 h-10 bg-linear-to-br from-primary-400 to-primary-600 rounded-full flex items-center justify-center text-white font-semibold">
        {name[0]}
      </div>
      <div>
        <div className="font-medium text-surface-900 dark:text-surface-100">{name}</div>
        <div className="text-xs text-primary-600 dark:text-primary-400">{role}</div>
      </div>
    </div>
    <p className="text-sm text-surface-600 dark:text-surface-400">{description}</p>
  </div>
);

export const AboutPage: React.FC = () => {
  const navigate = useNavigate();

  const techFeatures = [
    {
      icon: Brain,
      title: '多智能体协作',
      description: '诊断者、规划者、导师、解题者等专门智能体协同工作，各司其职',
    },
    {
      icon: Cpu,
      title: '深度知识追踪',
      description: '基于 DKT 模型实时预测知识掌握状态，智能推荐学习内容',
    },
    {
      icon: Zap,
      title: '符号计算引擎',
      description: '基于 SymPy 的精确数学推导，确保每一步解答的准确性',
    },
    {
      icon: Target,
      title: '自适应学习路径',
      description: '根据学习数据动态调整难度和内容，实现真正的个性化教学',
    },
    {
      icon: Shield,
      title: '安全可靠',
      description: '完善的数据加密和隐私保护机制，确保学习数据安全',
    },
    {
      icon: Award,
      title: '苏格拉底式教学',
      description: 'AI 采用启发式引导，培养独立思考能力而非直接给出答案',
    },
  ];

  const teamMembers: TeamMemberProps[] = [
    {
      name: '叶峰',
      role: '指导教师',
      description: '西安电子科技大学数学与统计学院教师，人工智能与教育技术专家，指导平台整体架构设计',
    },
    {
      name: '韩邦合',
      role: '指导教师',
      description: '西安电子科技大学数学与统计学院教师，负责项目技术指导与学术支持',
    },
    {
      name: '周禹睿',
      role: '组长 / 项目负责人',
      description: '西安电子科技大学通信工程学院学生，负责项目整体规划、架构设计与团队协调',
    },
    {
      name: '孙屹',
      role: '核心开发成员',
      description: '西安电子科技大学网络与信息安全学院学生，负责智能体系统与后端服务开发',
    },
    {
      name: '柯正',
      role: '核心开发成员',
      description: '西安电子科技大学网络与信息安全学院学生，负责前端界面与用户体验设计开发',
    },
  ];

  const milestones: TimelineItemProps[] = [
    {
      date: '2025 年 9 月',
      title: '项目启动',
      description: '完成需求调研和技术选型，确定多智能体协作架构方案',
    },
    {
      date: '2025 年 11 月',
      title: '核心功能开发',
      description: '完成智能刷题、AI 学习助手、知识关联等核心模块开发',
    },
    {
      date: '2026 年 1 月',
      title: '内测上线',
      description: '面向校内师生开放内测，收集反馈持续优化',
    },
    {
      date: '2026 年 3 月',
      title: '正式发布',
      description: '完成全部功能开发，面向更多高校推广使用',
      isLast: true,
    },
  ];

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

        {/* 页面标题 */}
        <div className="mb-10 text-center">
          <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100">
            关于我们
          </h1>
          <p className="mt-3 text-surface-500 dark:text-surface-400 max-w-2xl mx-auto">
            高数智学 - 基于多智能体协作与深度知识追踪的高等数学智能学习平台
          </p>
        </div>

        {/* 平台数据统计 */}
        <Card className="mb-8">
          <CardContent className="p-6">
            <div className="grid grid-cols-2 md:grid-cols-4 divide-x divide-surface-200 dark:divide-surface-700">
              <StatItem value="1000+" label="注册用户" icon={Users} />
              <StatItem value="5000+" label="题库数量" icon={BookOpen} />
              <StatItem value="98%" label="用户满意度" icon={Heart} />
              <StatItem value="50+" label="覆盖知识点" icon={GraduationCap} />
            </div>
          </CardContent>
        </Card>

        {/* 项目愿景与核心理念 */}
        <div className="grid gap-6 md:grid-cols-2 mb-8">
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-10 h-10 bg-primary-100 dark:bg-primary-900/30 rounded-lg flex items-center justify-center">
                  <Target className="w-5 h-5 text-primary-600 dark:text-primary-400" />
                </div>
                <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  项目愿景
                </h2>
              </div>
              <p className="text-surface-600 dark:text-surface-400 leading-relaxed text-sm">
                高数智学致力于打造新一代智能教育平台，将大语言模型的自然语言理解能力与符号计算引擎的精确性相结合，为每位学生提供"千人千面"的自适应学习体验。我们相信，通过 AI 技术的赋能，每个学生都能找到适合自己的学习节奏，攻克高等数学这座大山。
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-10 h-10 bg-secondary-100 dark:bg-secondary-900/30 rounded-lg flex items-center justify-center">
                  <Lightbulb className="w-5 h-5 text-secondary-600 dark:text-secondary-400" />
                </div>
                <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  核心理念
                </h2>
              </div>
              <p className="text-surface-600 dark:text-surface-400 leading-relaxed text-sm">
                我们采用"神经符号双脑协同"架构，让 AI 负责语义理解和教学引导，让符号计算引擎负责严谨的数学推导。通过深度知识追踪模型实时预测学生的知识状态，动态调整学习路径，真正实现因材施教，让学习更高效、更有针对性。
              </p>
            </CardContent>
          </Card>
        </div>

        {/* 技术特色 */}
        <Card className="mb-8">
          <CardContent className="p-6">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-5">
              技术特色
            </h2>
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {techFeatures.map((feature) => (
                <div
                  key={feature.title}
                  className="p-4 bg-surface-100 dark:bg-surface-800 rounded-lg hover:shadow-md transition-shadow"
                >
                  <div className="flex items-center gap-3 mb-2">
                    <feature.icon className="w-5 h-5 text-primary-600 dark:text-primary-400" />
                    <h3 className="font-medium text-surface-900 dark:text-surface-100">
                      {feature.title}
                    </h3>
                  </div>
                  <p className="text-sm text-surface-600 dark:text-surface-400">
                    {feature.description}
                  </p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* 团队介绍与发展历程 */}
        <div className="grid gap-6 md:grid-cols-2 mb-8">
          {/* 核心团队 */}
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-3 mb-5">
                <div className="w-10 h-10 bg-accent-100 dark:bg-accent-900/30 rounded-lg flex items-center justify-center">
                  <Users className="w-5 h-5 text-accent-600 dark:text-accent-400" />
                </div>
                <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  核心团队
                </h2>
              </div>
              <p className="text-sm text-surface-600 dark:text-surface-400 mb-4">
                我们是来自西安电子科技大学的研发团队，由计算机科学、人工智能和数学教育领域的师生组成。
              </p>
              <div className="grid gap-3">
                {teamMembers.map((member) => (
                  <TeamMember key={member.name} {...member} />
                ))}
              </div>
            </CardContent>
          </Card>

          {/* 发展历程 */}
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-3 mb-5">
                <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/30 rounded-lg flex items-center justify-center">
                  <Calendar className="w-5 h-5 text-emerald-600 dark:text-emerald-400" />
                </div>
                <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  发展历程
                </h2>
              </div>
              <div className="mt-2">
                {milestones.map((milestone, index) => (
                  <TimelineItem key={index} {...milestone} />
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 合作与支持 */}
        <Card className="mb-8">
          <CardContent className="p-6">
            <div className="flex items-center gap-3 mb-5">
              <div className="w-10 h-10 bg-orange-100 dark:bg-orange-900/30 rounded-lg flex items-center justify-center">
                <Building2 className="w-5 h-5 text-orange-600 dark:text-orange-400" />
              </div>
              <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
                合作与支持
              </h2>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="p-4 bg-surface-100 dark:bg-surface-800 rounded-lg text-center">
                <div className="font-medium text-surface-900 dark:text-surface-100 mb-1">
                  西安电子科技大学
                </div>
                <p className="text-xs text-surface-500 dark:text-surface-400">
                  技术研发与学术支持
                </p>
              </div>
              <div className="p-4 bg-surface-100 dark:bg-surface-800 rounded-lg text-center">
                <div className="font-medium text-surface-900 dark:text-surface-100 mb-1">
                  数学与统计学院
                </div>
                <p className="text-xs text-surface-500 dark:text-surface-400">
                  教学内容与题库建设
                </p>
              </div>
              <div className="p-4 bg-surface-100 dark:bg-surface-800 rounded-lg text-center">
                <div className="font-medium text-surface-900 dark:text-surface-100 mb-1">
                  人工智能学院
                </div>
                <p className="text-xs text-surface-500 dark:text-surface-400">
                  AI 算法与模型支持
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* 底部链接 */}
        <div className="text-center text-sm text-surface-500 dark:text-surface-400">
          <p>想要了解更多或加入我们？</p>
          <div className="flex justify-center gap-4 mt-2">
            <Button
              variant="link"
              className="text-primary-600 dark:text-primary-400"
              onClick={() => navigate('/contact')}
            >
              联系我们
            </Button>
            <Button
              variant="link"
              className="text-primary-600 dark:text-primary-400"
              onClick={() => navigate('/guide')}
            >
              使用指南
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};
