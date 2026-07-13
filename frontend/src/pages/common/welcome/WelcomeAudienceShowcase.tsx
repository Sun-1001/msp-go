import type { ReactNode } from 'react';
import { motion, useReducedMotion } from 'framer-motion';
import {
  AlertTriangle,
  BarChart3,
  BookOpen,
  BrainCircuit,
  Check,
  CircleDot,
  GraduationCap,
  Library,
  Network,
  Route,
  Sparkles,
  Target,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs';

interface AudienceFeature {
  icon: LucideIcon;
  title: string;
  description: string;
}

interface AudienceContent {
  id: 'student' | 'teacher';
  title: string;
  description: string;
  features: readonly AudienceFeature[];
}

const audienceContent: readonly AudienceContent[] = [
  {
    id: 'student',
    title: '把“不会做”，变成知道下一步怎么做。',
    description: '从提问到练习再到复习，学习记录不会在功能切换中断开，每一步都会留下可继续的上下文。',
    features: [
      { icon: BrainCircuit, title: 'AI 学习会话', description: '围绕你的推理继续追问，按关键步骤提示与验证。' },
      { icon: Target, title: '智能练习', description: '根据当前知识点和错误类型安排难度合适的下一题。' },
      { icon: Network, title: '知识图谱与错题', description: '把前置概念、薄弱点和错误记录放回同一条知识链。' },
      { icon: BarChart3, title: '学习分析与路径', description: '用掌握变化调整今日任务，不让复习只凭感觉。' },
    ],
  },
  {
    id: 'teacher',
    title: '先看清共性问题，再安排下一次教学。',
    description: '把班级学情、知识点掌握与练习反馈汇总到教学视图，帮助教师快速定位值得讲评的地方。',
    features: [
      { icon: Users, title: '班级与学生分析', description: '从班级整体下钻到个人学习轨迹，查看进度与困难点。' },
      { icon: AlertTriangle, title: '学情预警', description: '集中呈现持续低活跃、掌握下滑和高频错误等信号。' },
      { icon: Library, title: '题库与教学资源', description: '按章节、知识点与难度组织题目、讲义和练习材料。' },
      { icon: Route, title: '教学反馈闭环', description: '让讲评、练习和后续任务围绕同一份学情持续更新。' },
    ],
  },
] as const;

const studentTasks = [
  { title: '回顾导数符号与单调性', meta: '知识回顾', status: '已复习', tone: 'emerald' },
  { title: '完成函数极值基础练习', meta: '当前任务', status: '进行中', tone: 'primary' },
  { title: '进入最值的实际应用', meta: '路径推荐', status: '下一步', tone: 'amber' },
] as const;

const studentNavigation = [
  { label: '学习计划', icon: Route, active: true },
  { label: 'AI 学习会话', icon: Sparkles, active: false },
  { label: '错题复习', icon: BookOpen, active: false },
] as const;

const studentMastery = [
  { label: '导数定义', width: '88%', tone: 'bg-emerald-500' },
  { label: '函数单调性', width: '70%', tone: 'bg-primary-500' },
  { label: '极值与最值', width: '46%', tone: 'bg-secondary-500' },
] as const;

const teacherFocusItems = [
  { title: '极值判别步骤', detail: '共性误区', tone: 'text-red-600 dark:text-red-400' },
  { title: '导数符号表', detail: '建议讲评', tone: 'text-amber-600 dark:text-amber-400' },
  { title: '最值建模', detail: '下一课', tone: 'text-secondary-600 dark:text-secondary-400' },
] as const;

const teacherNavigation = [
  { label: '教学概览', icon: BarChart3, active: true },
  { label: '班级管理', icon: Users, active: false },
  { label: '题库资源', icon: Library, active: false },
] as const;

const teacherMastery = [
  { label: '导数定义', width: '84%', tone: 'bg-emerald-500' },
  { label: '函数单调性', width: '64%', tone: 'bg-amber-500' },
  { label: '极值判别', width: '42%', tone: 'bg-red-500' },
] as const;

interface AudiencePreviewShellProps {
  audience: 'student' | 'teacher';
  icon: LucideIcon;
  title: string;
  subtitle: string;
  ariaLabel: string;
  children: ReactNode;
}

const AudiencePreviewShell = ({
  audience,
  icon: Icon,
  title,
  subtitle,
  ariaLabel,
  children,
}: AudiencePreviewShellProps) => {
  const isStudent = audience === 'student';

  return (
    <div
      role="img"
      aria-label={ariaLabel}
      className="overflow-hidden rounded-lg border border-surface-200 bg-white shadow-2xl shadow-surface-950/10 dark:border-surface-700 dark:bg-surface-900 dark:shadow-black/30"
    >
      <div className="flex h-14 items-center justify-between border-b border-surface-200 bg-surface-50 px-4 dark:border-surface-700 dark:bg-surface-950 sm:px-5">
        <div className="flex min-w-0 items-center gap-3">
          <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-white ${isStudent ? 'bg-primary-600' : 'bg-secondary-600'}`}>
            <Icon className="h-4 w-4" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-surface-900 dark:text-surface-100">{title}</p>
            <p className="truncate text-[11px] text-surface-500 dark:text-surface-400">{subtitle}</p>
          </div>
        </div>
        <span className="flex shrink-0 items-center gap-1.5 text-[11px] font-medium text-surface-500 dark:text-surface-400">
          <CircleDot className={`h-3.5 w-3.5 ${isStudent ? 'text-emerald-500' : 'text-secondary-500'}`} aria-hidden="true" />
          示例视图
        </span>
      </div>
      {children}
    </div>
  );
};

const StudentWorkspacePreview = () => {
  return (
    <AudiencePreviewShell
      audience="student"
      icon={GraduationCap}
      title="学生学习中心"
      subtitle="今日任务随学习反馈更新"
      ariaLabel="学生端从知识回顾、智能练习到下一学习目标的产品界面示例"
    >
      <div className="grid min-h-[410px] sm:grid-cols-[10.5rem_1fr] lg:min-h-[440px]">
        <aside className="hidden border-r border-surface-200 bg-surface-50/80 p-4 dark:border-surface-700 dark:bg-surface-950/70 sm:block">
          <p className="text-[11px] font-semibold text-surface-400 dark:text-surface-500">今日学习</p>
          <div className="mt-3 space-y-1">
            {studentNavigation.map((item) => {
              const Icon = item.icon;
              return (
                <div
                  key={item.label}
                  className={`flex items-center gap-2 rounded-md px-3 py-2 text-xs font-medium ${item.active ? 'bg-white text-primary-700 shadow-sm ring-1 ring-surface-200 dark:bg-surface-800 dark:text-primary-300 dark:ring-surface-700' : 'text-surface-500 dark:text-surface-400'}`}
                >
                  <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                  {item.label}
                </div>
              );
            })}
          </div>
          <div className="mt-7 border-t border-surface-200 pt-4 dark:border-surface-700">
            <p className="text-[11px] text-surface-400 dark:text-surface-500">本次学习目标</p>
            <p className="mt-2 text-xs font-semibold leading-5 text-surface-700 dark:text-surface-300">理解极值判别，而不只是记住结论</p>
          </div>
        </aside>

        <div className="grid min-w-0 lg:grid-cols-[1fr_13rem]">
          <div className="p-4 sm:p-5">
            <div className="flex items-start justify-between gap-4 border-b border-surface-200 pb-4 dark:border-surface-700">
              <div>
                <p className="text-base font-bold text-surface-900 dark:text-white">从函数极值继续</p>
                <p className="mt-1 text-[11px] text-surface-500 dark:text-surface-400">系统已衔接上一次练习记录</p>
              </div>
              <BrainCircuit className="h-5 w-5 shrink-0 text-primary-500" aria-hidden="true" />
            </div>

            <ol className="divide-y divide-surface-200 dark:divide-surface-700">
              {studentTasks.map((task, index) => (
                <li key={task.title} className="flex items-center gap-3 py-4">
                  <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-xs font-bold ${task.tone === 'emerald' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : task.tone === 'primary' ? 'bg-primary-600 text-white' : 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'}`}>
                    {task.tone === 'emerald' ? <Check className="h-4 w-4" aria-hidden="true" /> : index + 1}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-surface-800 dark:text-surface-100">{task.title}</p>
                    <p className="mt-0.5 text-[11px] text-surface-500 dark:text-surface-400">{task.meta}</p>
                  </div>
                  <span className={`text-[11px] font-semibold ${task.tone === 'emerald' ? 'text-emerald-600 dark:text-emerald-400' : task.tone === 'primary' ? 'text-primary-700 dark:text-primary-300' : 'text-amber-600 dark:text-amber-400'}`}>{task.status}</span>
                </li>
              ))}
            </ol>

            <div className="mt-4 border-l-2 border-primary-500 pl-4">
              <p className="text-xs font-semibold text-primary-700 dark:text-primary-300">AI 学习建议</p>
              <p className="mt-1 text-xs leading-5 text-surface-600 dark:text-surface-300">先用导数符号表验证两个候选点，再进入最值应用。</p>
            </div>
          </div>

          <aside className="hidden border-l border-surface-200 bg-surface-50/80 p-5 dark:border-surface-700 dark:bg-surface-950/70 lg:block">
            <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">掌握脉络</p>
            <div className="mt-5 space-y-5">
              {studentMastery.map((item) => (
                <div key={item.label}>
                  <p className="text-[11px] text-surface-600 dark:text-surface-300">{item.label}</p>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-200 dark:bg-surface-800">
                    <div className={`h-full rounded-full ${item.tone}`} style={{ width: item.width }} />
                  </div>
                </div>
              ))}
            </div>
          </aside>
        </div>
      </div>
    </AudiencePreviewShell>
  );
};

const TeacherWorkspacePreview = () => {
  return (
    <AudiencePreviewShell
      audience="teacher"
      icon={Users}
      title="教师工作台"
      subtitle="示例班级 · 高等数学 A"
      ariaLabel="教师端汇总班级知识点掌握、共性误区和教学建议的产品界面示例"
    >
      <div className="grid min-h-[410px] sm:grid-cols-[10.5rem_1fr] lg:min-h-[440px]">
        <aside className="hidden border-r border-surface-200 bg-surface-50/80 p-4 dark:border-surface-700 dark:bg-surface-950/70 sm:block">
          <p className="text-[11px] font-semibold text-surface-400 dark:text-surface-500">教学空间</p>
          <div className="mt-3 space-y-1">
            {teacherNavigation.map((item) => {
              const Icon = item.icon;
              return (
                <div
                  key={item.label}
                  className={`flex items-center gap-2 rounded-md px-3 py-2 text-xs font-medium ${item.active ? 'bg-white text-secondary-700 shadow-sm ring-1 ring-surface-200 dark:bg-surface-800 dark:text-secondary-300 dark:ring-surface-700' : 'text-surface-500 dark:text-surface-400'}`}
                >
                  <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                  {item.label}
                </div>
              );
            })}
          </div>
          <div className="mt-7 border-t border-surface-200 pt-4 dark:border-surface-700">
            <p className="text-[11px] text-surface-400 dark:text-surface-500">当前目标</p>
            <p className="mt-2 text-xs font-semibold leading-5 text-surface-700 dark:text-surface-300">确定下一次课堂讲评重点</p>
          </div>
        </aside>

        <div className="grid min-w-0 lg:grid-cols-[1fr_13rem]">
          <div className="p-4 sm:p-5">
            <div className="flex items-start justify-between gap-4 border-b border-surface-200 pb-4 dark:border-surface-700">
              <div>
                <p className="text-base font-bold text-surface-900 dark:text-white">本周需要关注</p>
                <p className="mt-1 text-[11px] text-surface-500 dark:text-surface-400">从练习记录中汇总可行动信息</p>
              </div>
              <AlertTriangle className="h-5 w-5 shrink-0 text-amber-500" aria-hidden="true" />
            </div>

            <ol className="divide-y divide-surface-200 dark:divide-surface-700">
              {teacherFocusItems.map((item, index) => (
                <li key={item.title} className="flex items-center gap-3 py-4">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-surface-100 text-xs font-bold text-surface-500 dark:bg-surface-800 dark:text-surface-300">
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-surface-800 dark:text-surface-100">{item.title}</p>
                    <p className={`mt-0.5 text-[11px] font-medium ${item.tone}`}>{item.detail}</p>
                  </div>
                  <Target className="h-4 w-4 shrink-0 text-surface-300 dark:text-surface-600" aria-hidden="true" />
                </li>
              ))}
            </ol>

            <div className="mt-4 border-l-2 border-secondary-500 pl-4">
              <p className="text-xs font-semibold text-secondary-700 dark:text-secondary-300">教学建议</p>
              <p className="mt-1 text-xs leading-5 text-surface-600 dark:text-surface-300">先集中讲评极值判别，再布置一组导数符号变式题。</p>
            </div>
          </div>

          <aside className="hidden border-l border-surface-200 bg-surface-50/80 p-5 dark:border-surface-700 dark:bg-surface-950/70 lg:block">
            <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">知识点掌握</p>
            <div className="mt-5 space-y-5">
              {teacherMastery.map((item) => (
                <div key={item.label}>
                  <p className="text-[11px] text-surface-600 dark:text-surface-300">{item.label}</p>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-200 dark:bg-surface-800">
                    <div className={`h-full rounded-full ${item.tone}`} style={{ width: item.width }} />
                  </div>
                </div>
              ))}
            </div>
          </aside>
        </div>
      </div>
    </AudiencePreviewShell>
  );
};

interface AudiencePanelProps {
  content: AudienceContent;
  reduceMotion: boolean;
}

const AudiencePanel = ({ content, reduceMotion }: AudiencePanelProps) => {
  const isStudent = content.id === 'student';

  return (
    <motion.div
      initial={reduceMotion ? false : { opacity: 0, y: 18 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: reduceMotion ? 0 : 0.45, ease: [0.22, 1, 0.36, 1] }}
      className="grid items-center gap-10 lg:grid-cols-[0.72fr_1.28fr] xl:gap-14"
    >
      <div>
        <h3 className="max-w-xl text-3xl font-bold leading-tight tracking-normal text-surface-950 dark:text-white sm:text-4xl">
          {content.title}
        </h3>
        <p className="mt-5 max-w-xl text-sm leading-7 text-surface-600 dark:text-surface-300 sm:text-base">
          {content.description}
        </p>
        <div className="mt-8 border-b border-surface-300 dark:border-surface-700">
          {content.features.map((feature, index) => {
            const Icon = feature.icon;
            return (
              <motion.article
                key={feature.title}
                initial={reduceMotion ? false : { opacity: 0, x: -18 }}
                whileInView={reduceMotion ? undefined : { opacity: 1, x: 0 }}
                viewport={{ once: true, amount: 0.5 }}
                transition={{ duration: 0.4, delay: index * 0.06, ease: [0.22, 1, 0.36, 1] }}
                className="flex gap-4 border-t border-surface-300 py-4 dark:border-surface-700"
              >
                <span className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md ${isStudent ? 'bg-primary-50 text-primary-700 dark:bg-primary-950 dark:text-primary-300' : 'bg-secondary-50 text-secondary-700 dark:bg-secondary-950 dark:text-secondary-300'}`}>
                  <Icon className="h-4 w-4" aria-hidden="true" />
                </span>
                <div>
                  <h4 className="text-sm font-semibold text-surface-900 dark:text-surface-100">{feature.title}</h4>
                  <p className="mt-1 text-xs leading-5 text-surface-600 dark:text-surface-400">{feature.description}</p>
                </div>
              </motion.article>
            );
          })}
        </div>
      </div>

      {isStudent ? <StudentWorkspacePreview /> : <TeacherWorkspacePreview />}
    </motion.div>
  );
};

export const WelcomeAudienceShowcase = () => {
  const reduceMotion = useReducedMotion() ?? false;

  return (
    <section className="content-visibility-auto border-y border-surface-200 bg-surface-100 px-5 py-20 dark:border-surface-800 dark:bg-surface-900 sm:px-8 lg:py-28">
      <div className="mx-auto max-w-7xl">
        <div className="flex flex-col justify-between gap-8 lg:flex-row lg:items-end">
          <div>
            <h2 className="max-w-3xl text-4xl font-bold leading-tight tracking-normal text-surface-950 dark:text-white sm:text-5xl">
              一套学习闭环，连接学生与教师。
            </h2>
            <p className="mt-5 max-w-2xl text-base leading-8 text-surface-600 dark:text-surface-300">
              学生看到清晰的下一步，教师看到值得行动的学情；两端共享同一套知识关系与学习反馈。
            </p>
          </div>
        </div>

        <Tabs defaultValue="student" keepMounted={false} className="mt-9">
          <TabsList aria-label="选择产品角色视图" className="h-12 bg-white p-1 shadow-sm ring-1 ring-surface-200 dark:bg-surface-950 dark:ring-surface-700">
            <TabsTrigger value="student" className="h-10 px-5 sm:px-7">
              <GraduationCap className="mr-2 h-4 w-4" aria-hidden="true" />
              学生端
            </TabsTrigger>
            <TabsTrigger value="teacher" className="h-10 px-5 sm:px-7">
              <Users className="mr-2 h-4 w-4" aria-hidden="true" />
              教师端
            </TabsTrigger>
          </TabsList>

          {audienceContent.map((content) => (
            <TabsContent key={content.id} value={content.id} className="mt-10">
              <AudiencePanel content={content} reduceMotion={reduceMotion} />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </section>
  );
};
