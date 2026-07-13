import { motion, useReducedMotion } from 'framer-motion';
import {
  BookOpen,
  BrainCircuit,
  Check,
  RotateCcw,
  type LucideIcon,
} from 'lucide-react';

interface JourneyStage {
  phase: string;
  title: string;
  description: string;
  icon: LucideIcon;
  tone: string;
  iconTone: string;
  outcomes: readonly string[];
}

const journeyStages: readonly JourneyStage[] = [
  {
    phase: '课前',
    title: '带着问题进入课堂',
    description: '先回顾前置概念，再用一组短练习定位真正不确定的地方。',
    icon: BookOpen,
    tone: 'text-primary-700 dark:text-primary-300',
    iconTone: 'bg-primary-600 text-white',
    outcomes: ['前置知识快速回顾', '生成个人预习问题', '同步课程学习目标'],
  },
  {
    phase: '学习中',
    title: '在关键推导处停下来',
    description: '遇到卡点时，AI 沿当前思路给出提示、追问与步骤验证。',
    icon: BrainCircuit,
    tone: 'text-secondary-700 dark:text-secondary-300',
    iconTone: 'bg-secondary-600 text-white',
    outcomes: ['识别题目与数学公式', '保留多轮推理上下文', '关联对应知识节点'],
  },
  {
    phase: '复习',
    title: '让错误决定下一次练习',
    description: '错题、掌握变化和知识关系共同更新复习顺序与学习路径。',
    icon: RotateCcw,
    tone: 'text-emerald-700 dark:text-emerald-300',
    iconTone: 'bg-emerald-600 text-white',
    outcomes: ['按薄弱点整理错题', '安排针对性巩固练习', '持续更新掌握趋势'],
  },
] as const;

export const WelcomeLearningJourney = () => {
  const reduceMotion = useReducedMotion() ?? false;

  return (
    <section className="content-visibility-auto bg-white px-5 py-20 dark:bg-surface-950 sm:px-8 lg:py-28">
      <div className="mx-auto max-w-7xl">
        <div className="grid gap-7 lg:grid-cols-[0.9fr_1.1fr] lg:items-end">
          <h2 className="max-w-2xl text-4xl font-bold leading-tight tracking-normal text-surface-950 dark:text-white sm:text-5xl">
            从课前到复习，学习始终有下一步。
          </h2>
          <p className="max-w-2xl text-base leading-8 text-surface-600 dark:text-surface-300 lg:justify-self-end">
            高数智学不是一次性的答题工具，而是一条持续衔接课程、思考、练习和反馈的学习过程。
          </p>
        </div>

        <div className="relative mt-14 lg:mt-16">
          <div className="absolute bottom-0 left-[1.35rem] top-0 w-px bg-surface-200 dark:bg-surface-800 lg:bottom-auto lg:left-0 lg:right-0 lg:top-[1.35rem] lg:h-px lg:w-auto">
            <motion.div
              initial={reduceMotion ? false : { scaleY: 0 }}
              whileInView={reduceMotion ? undefined : { scaleY: 1 }}
              viewport={{ once: true, amount: 0.25 }}
              transition={{ duration: 0.9, ease: [0.22, 1, 0.36, 1] }}
              className="h-full w-full origin-top bg-primary-400 dark:bg-primary-600 lg:hidden"
            />
            <motion.div
              initial={reduceMotion ? false : { scaleX: 0 }}
              whileInView={reduceMotion ? undefined : { scaleX: 1 }}
              viewport={{ once: true, amount: 0.25 }}
              transition={{ duration: 0.9, ease: [0.22, 1, 0.36, 1] }}
              className="hidden h-full w-full origin-left bg-primary-400 dark:bg-primary-600 lg:block"
            />
          </div>

          <div className="relative grid gap-12 lg:grid-cols-3 lg:gap-10">
            {journeyStages.map((stage, index) => {
              const Icon = stage.icon;
              return (
                <motion.article
                  key={stage.phase}
                  initial={reduceMotion ? false : { opacity: 0, y: 24 }}
                  whileInView={reduceMotion ? undefined : { opacity: 1, y: 0 }}
                  viewport={{ once: true, amount: 0.4 }}
                  transition={{ duration: 0.5, delay: index * 0.09, ease: [0.22, 1, 0.36, 1] }}
                  className="relative pl-16 lg:pl-0"
                >
                  <span className={`absolute left-0 flex h-11 w-11 items-center justify-center rounded-md shadow-sm lg:relative ${stage.iconTone}`}>
                    <Icon className="h-5 w-5" aria-hidden="true" />
                  </span>
                  <p className={`text-xs font-bold ${stage.tone} lg:mt-8`}>{stage.phase}</p>
                  <h3 className="mt-2 text-2xl font-bold tracking-normal text-surface-950 dark:text-white">
                    {stage.title}
                  </h3>
                  <p className="mt-4 max-w-sm text-sm leading-7 text-surface-600 dark:text-surface-300">
                    {stage.description}
                  </p>
                  <ul className="mt-6 space-y-3 border-t border-surface-200 pt-5 dark:border-surface-800">
                    {stage.outcomes.map((outcome) => (
                      <li key={outcome} className="flex items-center gap-2.5 text-sm text-surface-700 dark:text-surface-300">
                        <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" aria-hidden="true" />
                        {outcome}
                      </li>
                    ))}
                  </ul>
                </motion.article>
              );
            })}
          </div>
        </div>
      </div>
    </section>
  );
};
