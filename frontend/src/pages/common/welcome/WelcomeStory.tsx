import { useRef, useState, type ReactNode, type RefObject } from 'react';
import {
  motion,
  useMotionValueEvent,
  useReducedMotion,
  useScroll,
  useSpring,
  useTransform,
} from 'framer-motion';
import {
  ArrowRight,
  BrainCircuit,
  ChevronDown,
  Network,
  Route,
  Sparkles,
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import {
  KnowledgeWorkspacePreview,
  LearningPathWorkspacePreview,
  TutorWorkspacePreview,
} from './WelcomeProductVisuals';

interface WelcomeStoryProps {
  sectionRef: RefObject<HTMLElement | null>;
  onStart: () => void;
}

const storySteps = [
  {
    label: 'AI 分步解析',
    shortLabel: 'AI 解析',
    description: '沿着你的思路拆解推理，不只给出答案。',
    icon: BrainCircuit,
    progress: 0.34,
  },
  {
    label: '知识关系可视化',
    shortLabel: '知识图谱',
    description: '把前置概念、当前薄弱点和下一目标连起来。',
    icon: Network,
    progress: 0.6,
  },
  {
    label: '个性化学习路径',
    shortLabel: '学习路径',
    description: '让每次练习都反馈到接下来的学习安排。',
    icon: Route,
    progress: 0.84,
  },
] as const;

interface HeroCopyProps {
  onStart: () => void;
  onExplore: () => void;
  reduceMotion: boolean;
}

const heroTitleCharacters = [
  { character: '高', accent: false },
  { character: '数', accent: false },
  { character: '智', accent: true },
  { character: '学', accent: true },
] as const;

const HeroCopy = ({ onStart, onExplore, reduceMotion }: HeroCopyProps) => (
  <div className="mx-auto w-full max-w-4xl px-5 text-center sm:px-8">
    <h1
      aria-label="高数智学"
      data-testid="welcome-title"
      data-title-motion={reduceMotion ? 'static' : 'animated'}
      className="perspective-1000 text-6xl font-bold leading-none tracking-normal sm:text-7xl lg:text-8xl"
    >
      <span aria-hidden="true" className="inline-flex transform-style-3d">
        {heroTitleCharacters.map((item, index) => (
          <motion.span
            key={item.character}
            initial={reduceMotion ? false : { opacity: 0, y: 46, rotateX: 78, scale: 0.92 }}
            animate={{ opacity: 1, y: 0, rotateX: 0, scale: 1 }}
            transition={{
              duration: reduceMotion ? 0 : 0.72,
              delay: reduceMotion ? 0 : 0.08 + index * 0.09,
              ease: [0.22, 1, 0.36, 1],
            }}
            style={{
              transformOrigin: '50% 100%',
              willChange: reduceMotion ? 'auto' : 'transform, opacity',
            }}
            className={item.accent ? 'welcome-title-accent inline-block' : 'inline-block text-surface-950 dark:text-white'}
          >
            {item.character}
          </motion.span>
        ))}
      </span>
      <span aria-hidden="true" className="mx-auto mt-4 flex w-36 items-center justify-center gap-2 sm:w-44">
        <motion.span
          initial={reduceMotion ? false : { opacity: 0, scaleX: 0 }}
          animate={{ opacity: 1, scaleX: 1 }}
          transition={{ duration: reduceMotion ? 0 : 0.55, delay: reduceMotion ? 0 : 0.46 }}
          className="h-px flex-1 origin-right bg-primary-400/70 dark:bg-primary-400/60"
        />
        <motion.span
          initial={reduceMotion ? false : { opacity: 0, scaleX: 0 }}
          animate={{ opacity: 1, scaleX: 1 }}
          transition={{ duration: reduceMotion ? 0 : 0.42, delay: reduceMotion ? 0 : 0.58 }}
          className="h-1 w-8 origin-center rounded-full bg-secondary-500"
        />
        <motion.span
          initial={reduceMotion ? false : { opacity: 0, scaleX: 0 }}
          animate={{ opacity: 1, scaleX: 1 }}
          transition={{ duration: reduceMotion ? 0 : 0.55, delay: reduceMotion ? 0 : 0.46 }}
          className="h-px flex-1 origin-left bg-primary-400/70 dark:bg-primary-400/60"
        />
      </span>
    </h1>
    <p className="mx-auto mt-5 max-w-3xl text-2xl font-semibold leading-tight tracking-normal text-surface-800 dark:text-surface-100 sm:text-3xl lg:text-4xl">
      真正理解一道题，然后走向下一步。
    </p>
    <p className="mx-auto mt-5 max-w-2xl text-sm leading-7 text-surface-600 dark:text-surface-300 sm:text-base">
      把 AI 分步引导、知识图谱与学习反馈连成一条清晰路径，让抽象的高等数学变得可观察、可练习、可掌握。
    </p>
    <div className="mt-7 flex flex-col items-center justify-center gap-3 sm:flex-row">
      <Button
        type="button"
        size="lg"
        onClick={onStart}
        className="group h-12 rounded-md bg-surface-950 px-7 text-sm text-white shadow-lg shadow-surface-950/15 hover:bg-primary-700 dark:bg-white dark:text-surface-950 dark:hover:bg-primary-100"
      >
        开始学习
        <ArrowRight className="ml-2 h-4 w-4 transition-transform duration-300 group-hover:translate-x-1 motion-reduce:transition-none" aria-hidden="true" />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="lg"
        onClick={onExplore}
        className="h-12 rounded-md border-surface-300 bg-white/80 px-7 text-sm backdrop-blur-sm dark:border-surface-700 dark:bg-surface-900/80"
      >
        <Sparkles className="mr-2 h-4 w-4 text-secondary-500" aria-hidden="true" />
        查看产品演示
      </Button>
    </div>
  </div>
);

interface StaticStageProps {
  index: number;
  children: ReactNode;
}

const StaticStage = ({ index, children }: StaticStageProps) => {
  const step = storySteps[index];
  const Icon = step.icon;

  return (
    <section
      data-story-step={index}
      className="border-t border-surface-200 px-5 py-16 dark:border-surface-800 sm:px-8 lg:py-24"
    >
      <div className="mx-auto grid max-w-7xl items-center gap-10 lg:grid-cols-[20rem_1fr]">
        <div>
          <div className="mb-5 flex h-11 w-11 items-center justify-center rounded-md bg-primary-600 text-white">
            <Icon className="h-5 w-5" aria-hidden="true" />
          </div>
          <p className="text-xs font-semibold text-primary-700 dark:text-primary-300">0{index + 1}</p>
          <h2 className="mt-2 text-3xl font-bold tracking-normal text-surface-950 dark:text-white">
            {step.label}
          </h2>
          <p className="mt-4 text-base leading-7 text-surface-600 dark:text-surface-300">
            {step.description}
          </p>
        </div>
        <div>{children}</div>
      </div>
    </section>
  );
};

export const WelcomeStory = ({ sectionRef, onStart }: WelcomeStoryProps) => {
  const reduceMotion = useReducedMotion() ?? false;
  const [activeStep, setActiveStep] = useState(0);
  const activeStepRef = useRef(0);
  const { scrollYProgress } = useScroll({
    target: sectionRef,
    offset: ['start start', 'end end'],
  });
  const progress = useSpring(scrollYProgress, {
    stiffness: 120,
    damping: 28,
    mass: 0.25,
  });

  const heroOpacity = useTransform(progress, [0, 0.12, 0.24], [1, 1, 0]);
  const heroY = useTransform(progress, [0, 0.24], [0, -88]);
  const heroScale = useTransform(progress, [0, 0.24], [1, 0.96]);
  const frameScale = useTransform(progress, [0, 0.18, 0.32], [0.76, 0.84, 1]);
  const frameY = useTransform(progress, [0, 0.22, 0.32], [246, 132, 0]);
  const frameRadius = useTransform(progress, [0, 0.3], [24, 8]);
  const backgroundY = useTransform(progress, [0, 1], [-24, 32]);
  const navigationOpacity = useTransform(progress, [0.18, 0.28], [0, 1]);

  const tutorOpacity = useTransform(progress, [0, 0.44, 0.56], [1, 1, 0]);
  const tutorX = useTransform(progress, [0.44, 0.6], ['0%', '-62%']);
  const tutorScale = useTransform(progress, [0.44, 0.6], [1, 0.94]);
  const tutorVisibility = useTransform(progress, (value) => value > 0.57 ? 'hidden' : 'visible');

  const graphOpacity = useTransform(progress, [0.44, 0.53, 0.72, 0.82], [0, 1, 1, 0]);
  const graphX = useTransform(progress, [0.44, 0.56, 0.74, 0.86], ['58%', '0%', '0%', '-58%']);
  const graphScale = useTransform(progress, [0.44, 0.56, 0.78, 0.86], [0.94, 1, 1, 0.94]);
  const graphVisibility = useTransform(progress, (value) => value < 0.43 || value > 0.83 ? 'hidden' : 'visible');

  const pathOpacity = useTransform(progress, [0.72, 0.82], [0, 1]);
  const pathX = useTransform(progress, [0.72, 0.84], ['58%', '0%']);
  const pathScale = useTransform(progress, [0.72, 0.84], [0.94, 1]);
  const pathVisibility = useTransform(progress, (value) => value < 0.71 ? 'hidden' : 'visible');

  useMotionValueEvent(progress, 'change', (value) => {
    if (reduceMotion) return;
    const nextStep = value < 0.5 ? 0 : value < 0.76 ? 1 : 2;
    if (nextStep !== activeStepRef.current) {
      activeStepRef.current = nextStep;
      setActiveStep(nextStep);
    }
  });

  const scrollToStep = (index: number) => {
    const section = sectionRef.current;
    if (!section) return;

    if (reduceMotion) {
      section.querySelector<HTMLElement>(`[data-story-step="${index}"]`)?.scrollIntoView({
        behavior: 'auto',
        block: 'start',
      });
      return;
    }

    const scrollRange = Math.max(section.offsetHeight - window.innerHeight, 0);
    const sectionTop = section.getBoundingClientRect().top + window.scrollY;
    window.scrollTo({
      top: sectionTop + scrollRange * storySteps[index].progress,
      behavior: 'smooth',
    });
  };

  if (reduceMotion) {
    return (
      <section
        ref={sectionRef}
        data-testid="welcome-story"
        data-reduced-motion="true"
        className="relative overflow-hidden bg-surface-50 text-surface-900 dark:bg-surface-950 dark:text-surface-100"
      >
        <div className="flex min-h-[calc(100svh-4rem)] items-center py-16">
          <HeroCopy onStart={onStart} onExplore={() => scrollToStep(0)} reduceMotion={reduceMotion} />
        </div>
        <StaticStage index={0}><TutorWorkspacePreview /></StaticStage>
        <StaticStage index={1}><KnowledgeWorkspacePreview /></StaticStage>
        <StaticStage index={2}><LearningPathWorkspacePreview /></StaticStage>
      </section>
    );
  }

  return (
    <section
      ref={sectionRef}
      data-testid="welcome-story"
      data-reduced-motion="false"
      className="relative h-[430svh] overflow-clip bg-surface-50 text-surface-900 dark:bg-surface-950 dark:text-surface-100"
    >
      <div className="sticky top-16 h-[calc(100svh-4rem)] overflow-hidden">
        <motion.div
          aria-hidden="true"
          style={{ y: backgroundY }}
          className="pointer-events-none absolute -inset-12 opacity-50 dark:opacity-30"
        >
          <div
            className="h-full w-full"
            style={{
              backgroundImage:
                'linear-gradient(to right, currentColor 1px, transparent 1px), linear-gradient(to bottom, currentColor 1px, transparent 1px)',
              backgroundSize: '64px 64px',
              color: 'rgb(148 163 184 / 0.16)',
            }}
          />
        </motion.div>

        <motion.div
          style={{ opacity: heroOpacity, y: heroY, scale: heroScale }}
          className="absolute inset-x-0 top-[6%] z-20 will-change-transform"
        >
          <HeroCopy onStart={onStart} onExplore={() => scrollToStep(0)} reduceMotion={reduceMotion} />
        </motion.div>

        <div className="absolute inset-0 z-10 flex items-center justify-center">
          <motion.div
            style={{
              scale: frameScale,
              y: frameY,
              borderRadius: frameRadius,
              willChange: 'transform',
            }}
            className="relative h-[455px] w-[118vw] max-w-[70rem] overflow-hidden lg:h-[511px] lg:w-[90vw]"
          >
            <motion.div
              data-testid="tutor-story-scene"
              style={{ opacity: tutorOpacity, x: tutorX, scale: tutorScale, visibility: tutorVisibility, willChange: 'transform, opacity' }}
              className="absolute inset-0"
            >
              <TutorWorkspacePreview />
            </motion.div>
            <motion.div
              data-testid="graph-story-scene"
              style={{ opacity: graphOpacity, x: graphX, scale: graphScale, visibility: graphVisibility, willChange: 'transform, opacity' }}
              className="absolute inset-0"
            >
              <KnowledgeWorkspacePreview />
            </motion.div>
            <motion.div
              data-testid="path-story-scene"
              style={{ opacity: pathOpacity, x: pathX, scale: pathScale, visibility: pathVisibility, willChange: 'transform, opacity' }}
              className="absolute inset-0"
            >
              <LearningPathWorkspacePreview />
            </motion.div>
          </motion.div>
        </div>

        <motion.nav
          aria-label="产品演示阶段"
          style={{ opacity: navigationOpacity }}
          className="absolute inset-x-4 bottom-5 z-30 flex items-center justify-center gap-1 rounded-lg border border-surface-200 bg-white/90 p-1 shadow-lg backdrop-blur-md dark:border-surface-700 dark:bg-surface-900/90 sm:inset-x-auto sm:bottom-auto sm:right-5 sm:top-1/2 sm:-translate-y-1/2 sm:flex-col"
        >
          {storySteps.map((step, index) => {
            const Icon = step.icon;
            const isActive = activeStep === index;
            return (
              <button
                key={step.label}
                type="button"
                aria-current={isActive ? 'step' : undefined}
                aria-label={`跳转到${step.label}`}
                title={step.label}
                onClick={() => scrollToStep(index)}
                className={`flex h-10 min-w-10 items-center justify-center rounded-md px-3 text-xs font-semibold transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 motion-reduce:transition-none sm:w-10 sm:px-0 ${
                  isActive
                    ? 'bg-primary-600 text-white'
                    : 'text-surface-500 hover:bg-surface-100 hover:text-surface-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-white'
                }`}
              >
                <Icon className="h-4 w-4 sm:h-[18px] sm:w-[18px]" aria-hidden="true" />
                <span className="ml-2 whitespace-nowrap sm:hidden">{step.shortLabel}</span>
              </button>
            );
          })}
        </motion.nav>

        <motion.button
          type="button"
          aria-label="继续查看产品演示"
          title="继续查看产品演示"
          style={{ opacity: heroOpacity }}
          onClick={() => scrollToStep(0)}
          className="absolute bottom-4 left-1/2 z-30 flex h-10 w-10 -translate-x-1/2 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-white hover:text-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-surface-400 dark:hover:bg-surface-900 dark:hover:text-primary-300"
        >
          <ChevronDown className="h-5 w-5 animate-bounce motion-reduce:animate-none" aria-hidden="true" />
        </motion.button>
      </div>
    </section>
  );
};
