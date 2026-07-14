import type { ReactNode } from 'react';
import {
  ArrowRight,
  BarChart3,
  BookOpen,
  BrainCircuit,
  Check,
  CheckCircle2,
  CircleDot,
  Clock3,
  Lightbulb,
  Network,
  Route,
  Sparkles,
  Target,
  TrendingUp,
  type LucideIcon,
} from 'lucide-react';

interface PreviewShellProps {
  icon: LucideIcon;
  title: string;
  status: string;
  ariaLabel: string;
  children: ReactNode;
}

const PreviewShell = ({
  icon: Icon,
  title,
  status,
  ariaLabel,
  children,
}: PreviewShellProps) => (
  <div
    role="img"
    aria-label={ariaLabel}
    className="overflow-hidden rounded-lg border border-surface-200 bg-white shadow-2xl shadow-surface-950/10 dark:border-surface-700 dark:bg-surface-900 dark:shadow-black/30"
  >
    <div className="flex h-14 items-center justify-between border-b border-surface-200 bg-surface-50 px-4 dark:border-surface-700 dark:bg-surface-950 sm:px-5">
      <div className="flex min-w-0 items-center gap-3">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary-600 text-white shadow-sm">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold tracking-normal text-surface-900 dark:text-surface-100">
            {title}
          </p>
          <p className="text-[11px] text-surface-500 dark:text-surface-400">高数智学</p>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
        <span className="hidden sm:inline">{status}</span>
      </div>
    </div>
    {children}
  </div>
);

const ChapterNavigation = ({ active }: { active: 'tutor' | 'graph' | 'path' }) => {
  const items = [
    { id: 'tutor', label: 'AI 学习会话', icon: BrainCircuit },
    { id: 'graph', label: '知识图谱', icon: Network },
    { id: 'path', label: '学习路径', icon: Route },
  ] as const;

  return (
    <aside className="hidden border-r border-surface-200 bg-surface-50/80 p-4 dark:border-surface-700 dark:bg-surface-950/70 md:block">
      <p className="mb-3 text-[11px] font-semibold text-surface-400 dark:text-surface-500">学习空间</p>
      <nav className="space-y-1" aria-label="产品预览导航">
        {items.map((item) => {
          const Icon = item.icon;
          const isActive = active === item.id;
          return (
            <div
              key={item.id}
              className={`flex items-center gap-2.5 rounded-md px-3 py-2 text-xs font-medium ${
                isActive
                  ? 'bg-white text-primary-700 shadow-sm ring-1 ring-surface-200 dark:bg-surface-800 dark:text-primary-300 dark:ring-surface-700'
                  : 'text-surface-500 dark:text-surface-400'
              }`}
            >
              <Icon className="h-3.5 w-3.5" aria-hidden="true" />
              {item.label}
            </div>
          );
        })}
      </nav>
      <div className="mt-7 border-t border-surface-200 pt-4 dark:border-surface-700">
        <p className="text-[11px] text-surface-400 dark:text-surface-500">本周目标</p>
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-200 dark:bg-surface-700">
          <div className="h-full w-[68%] rounded-full bg-emerald-500" />
        </div>
        <p className="mt-2 text-xs font-medium text-surface-700 dark:text-surface-300">已完成 68%</p>
      </div>
    </aside>
  );
};

export const TutorWorkspacePreview = () => (
  <PreviewShell
    icon={Sparkles}
    title="AI 学习会话"
    status="推导已保存"
    ariaLabel="AI 助教逐步解析函数极值问题的产品界面预览"
  >
    <div className="grid min-h-[390px] md:grid-cols-[12.5rem_1fr] lg:min-h-[455px]">
      <ChapterNavigation active="tutor" />
      <div className="flex min-w-0 flex-col">
        <div className="flex items-center justify-between border-b border-surface-200 px-4 py-3 dark:border-surface-700 sm:px-6">
          <div>
            <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">函数极值与单调性</p>
            <p className="mt-0.5 text-[11px] text-surface-500 dark:text-surface-400">苏格拉底式引导</p>
          </div>
          <div className="flex items-center gap-1.5 text-[11px] text-surface-500 dark:text-surface-400">
            <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
            12 分钟
          </div>
        </div>

        <div className="flex-1 space-y-4 p-4 sm:p-6">
          <div className="ml-auto max-w-[88%] rounded-md bg-surface-100 px-4 py-3 text-sm leading-6 text-surface-700 dark:bg-surface-800 dark:text-surface-200">
            求函数 <span className="font-mono text-xs">f(x) = x^3 - 3x^2 + 2</span> 的极值点。
          </div>

          <div className="max-w-[94%] border-l-2 border-primary-500 pl-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-primary-700 dark:text-primary-300">
              <Lightbulb className="h-4 w-4" aria-hidden="true" />
              先定位导数为零的位置
            </div>
            <ol className="space-y-3">
              <li className="flex items-start gap-3 text-xs leading-5 text-surface-600 dark:text-surface-300 sm:text-sm">
                <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-primary-50 text-[11px] font-bold text-primary-700 dark:bg-primary-950 dark:text-primary-300">1</span>
                <span>求导得到 <code className="rounded bg-surface-100 px-1.5 py-1 font-mono text-xs text-surface-900 dark:bg-surface-800 dark:text-white">f'(x) = 3x^2 - 6x</code></span>
              </li>
              <li className="flex items-start gap-3 text-xs leading-5 text-surface-600 dark:text-surface-300 sm:text-sm">
                <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-secondary-50 text-[11px] font-bold text-secondary-700 dark:bg-secondary-950 dark:text-secondary-300">2</span>
                <span>令 <code className="rounded bg-surface-100 px-1.5 py-1 font-mono text-xs text-surface-900 dark:bg-surface-800 dark:text-white">f'(x) = 0</code>，得到候选点 <strong>x = 0</strong> 与 <strong>x = 2</strong></span>
              </li>
              <li className="flex items-start gap-3 text-xs leading-5 text-surface-600 dark:text-surface-300 sm:text-sm">
                <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Check className="h-3 w-3" aria-hidden="true" /></span>
                <span>结合导数符号变化，判断 <strong>x = 0</strong> 为极大值点，<strong>x = 2</strong> 为极小值点</span>
              </li>
            </ol>
          </div>
        </div>

        <div className="flex items-center justify-between border-t border-surface-200 bg-surface-50 px-4 py-3 dark:border-surface-700 dark:bg-surface-950/70 sm:px-6">
          <div className="flex items-center gap-2 text-xs text-surface-500 dark:text-surface-400">
            <CircleDot className="h-3.5 w-3.5 text-emerald-500" aria-hidden="true" />
            已关联 3 个知识点
          </div>
          <span className="flex items-center gap-1 text-xs font-semibold text-primary-700 dark:text-primary-300">
            生成巩固练习
            <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
          </span>
        </div>
      </div>
    </div>
  </PreviewShell>
);

const knowledgeRelations = [
  { title: '函数与映射', meta: '已掌握', tone: 'text-emerald-600 dark:text-emerald-400' },
  { title: '导数的定义', meta: '掌握度 86%', tone: 'text-primary-600 dark:text-primary-400' },
  { title: '函数单调性', meta: '正在学习', tone: 'text-secondary-600 dark:text-secondary-400' },
  { title: '极值与最值', meta: '下一目标', tone: 'text-amber-600 dark:text-amber-400' },
];

export const KnowledgeWorkspacePreview = () => (
  <PreviewShell
    icon={Network}
    title="知识图谱"
    status="关系已更新"
    ariaLabel="展示函数极值相关知识关系和掌握度的产品界面预览"
  >
    <div className="grid min-h-[390px] md:grid-cols-[12.5rem_1fr] lg:min-h-[455px]">
      <ChapterNavigation active="graph" />
      <div className="grid min-w-0 lg:grid-cols-[1fr_15rem]">
        <div className="p-4 sm:p-6">
          <div className="mb-5 flex items-start justify-between gap-4">
            <div>
              <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">当前知识链</p>
              <p className="mt-1 text-[11px] text-surface-500 dark:text-surface-400">从前置概念到下一学习目标</p>
            </div>
            <Network className="h-5 w-5 text-secondary-500" aria-hidden="true" />
          </div>

          <ol className="divide-y divide-surface-200 border-y border-surface-200 dark:divide-surface-700 dark:border-surface-700">
            {knowledgeRelations.map((item, index) => (
              <li key={item.title} className="flex items-center gap-3 py-3.5">
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-surface-100 text-xs font-bold text-surface-500 dark:bg-surface-800 dark:text-surface-300">
                  {String(index + 1).padStart(2, '0')}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-surface-800 dark:text-surface-100">{item.title}</p>
                  <p className={`mt-0.5 text-[11px] font-medium ${item.tone}`}>{item.meta}</p>
                </div>
                {index < knowledgeRelations.length - 1 ? (
                  <ArrowRight className="h-4 w-4 shrink-0 text-surface-300 dark:text-surface-600" aria-hidden="true" />
                ) : (
                  <Target className="h-4 w-4 shrink-0 text-amber-500" aria-hidden="true" />
                )}
              </li>
            ))}
          </ol>

          <div className="mt-5 flex items-start gap-3 border-l-2 border-secondary-500 pl-4">
            <BrainCircuit className="mt-0.5 h-4 w-4 shrink-0 text-secondary-600 dark:text-secondary-400" aria-hidden="true" />
            <p className="text-xs leading-5 text-surface-600 dark:text-surface-300">
              你已具备学习“极值与最值”的前置基础，建议先完成 2 道导数符号变式题。
            </p>
          </div>
        </div>

        <aside className="hidden border-t border-surface-200 bg-surface-50/80 p-4 dark:border-surface-700 dark:bg-surface-950/70 lg:block lg:border-l lg:border-t-0 sm:p-5">
          <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">章节掌握度</p>
          <div className="mt-5 flex items-end gap-2" aria-label="章节掌握度柱状图">
            {[42, 68, 86, 58, 34].map((value, index) => (
              <div key={value} className="flex flex-1 flex-col items-center gap-2">
                <div className="flex h-28 w-full items-end overflow-hidden rounded-sm bg-surface-200 dark:bg-surface-800">
                  <div
                    className={index === 2 ? 'w-full bg-primary-500' : 'w-full bg-surface-400 dark:bg-surface-600'}
                    style={{ height: `${value}%` }}
                  />
                </div>
                <span className="text-[10px] text-surface-400">{index + 1}</span>
              </div>
            ))}
          </div>
          <div className="mt-5 border-t border-surface-200 pt-4 dark:border-surface-700">
            <p className="text-2xl font-bold tracking-normal text-surface-900 dark:text-white">68%</p>
            <p className="mt-1 text-[11px] text-surface-500 dark:text-surface-400">本章综合掌握度</p>
          </div>
        </aside>
      </div>
    </div>
  </PreviewShell>
);

const pathTasks = [
  { title: '复习导数符号表', meta: '8 分钟', status: 'done' },
  { title: '完成极值基础练习', meta: '6 / 8 题', status: 'active' },
  { title: '进入最值应用', meta: '预计 18 分钟', status: 'next' },
] as const;

export const LearningPathWorkspacePreview = () => (
  <PreviewShell
    icon={Route}
    title="个性化学习路径"
    status="计划已同步"
    ariaLabel="展示本周学习路径、任务进度和学习趋势的产品界面预览"
  >
    <div className="grid min-h-[390px] md:grid-cols-[12.5rem_1fr] lg:min-h-[455px]">
      <ChapterNavigation active="path" />
      <div className="grid min-w-0 lg:grid-cols-[1fr_17rem]">
        <div className="p-4 sm:p-6">
          <div className="mb-5 flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">今天的学习路径</p>
              <p className="mt-1 text-[11px] text-surface-500 dark:text-surface-400">根据最近练习动态调整</p>
            </div>
            <Route className="h-5 w-5 text-emerald-500" aria-hidden="true" />
          </div>

          <ol className="space-y-2">
            {pathTasks.map((task, index) => (
              <li
                key={task.title}
                className={`flex items-center gap-3 rounded-md border px-3 py-3 ${
                  task.status === 'active'
                    ? 'border-primary-200 bg-primary-50/70 dark:border-primary-800 dark:bg-primary-950/50'
                    : 'border-surface-200 bg-white dark:border-surface-700 dark:bg-surface-900'
                }`}
              >
                <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${
                  task.status === 'done'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                    : task.status === 'active'
                      ? 'bg-primary-600 text-white'
                      : 'bg-surface-100 text-surface-400 dark:bg-surface-800'
                }`}>
                  {task.status === 'done' ? <Check className="h-4 w-4" aria-hidden="true" /> : index + 1}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-surface-800 dark:text-surface-100">{task.title}</p>
                  <p className="mt-0.5 text-[11px] text-surface-500 dark:text-surface-400">{task.meta}</p>
                </div>
                {task.status === 'active' ? (
                  <span className="text-[11px] font-semibold text-primary-700 dark:text-primary-300">进行中</span>
                ) : null}
              </li>
            ))}
          </ol>

          <div className="mt-5 flex items-center gap-3 border-t border-surface-200 pt-4 dark:border-surface-700">
            <BookOpen className="h-4 w-4 text-surface-400" aria-hidden="true" />
            <p className="text-xs text-surface-600 dark:text-surface-300">完成后将解锁“函数最值的实际应用”</p>
          </div>
        </div>

        <aside className="hidden border-t border-surface-200 bg-surface-50/80 p-4 dark:border-surface-700 dark:bg-surface-950/70 lg:block lg:border-l lg:border-t-0 sm:p-5">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold text-surface-900 dark:text-surface-100">近 7 天趋势</p>
            <TrendingUp className="h-4 w-4 text-emerald-500" aria-hidden="true" />
          </div>
          <div className="mt-6 flex h-28 items-end gap-2" aria-label="近七天学习完成度柱状图">
            {[34, 48, 44, 72, 58, 82, 68].map((value, index) => (
              <div key={`${value}-${index}`} className="flex flex-1 items-end">
                <div
                  className={index === 5 ? 'w-full rounded-sm bg-emerald-500' : 'w-full rounded-sm bg-surface-300 dark:bg-surface-700'}
                  style={{ height: `${value}%` }}
                />
              </div>
            ))}
          </div>
          <div className="mt-5 grid grid-cols-2 gap-3 border-t border-surface-200 pt-4 dark:border-surface-700">
            <div>
              <p className="text-lg font-bold tracking-normal text-surface-900 dark:text-white">42</p>
              <p className="text-[10px] text-surface-500 dark:text-surface-400">本周完成题数</p>
            </div>
            <div>
              <p className="text-lg font-bold tracking-normal text-surface-900 dark:text-white">+12%</p>
              <p className="text-[10px] text-surface-500 dark:text-surface-400">掌握度变化</p>
            </div>
          </div>
          <div className="mt-4 flex items-center gap-2 text-[11px] text-surface-500 dark:text-surface-400">
            <BarChart3 className="h-3.5 w-3.5" aria-hidden="true" />
            每次练习都会更新路径
          </div>
        </aside>
      </div>
    </div>
  </PreviewShell>
);
