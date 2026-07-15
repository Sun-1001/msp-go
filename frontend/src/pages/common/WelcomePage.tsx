import { useRef, useState } from 'react';
import { motion, useReducedMotion } from 'framer-motion';
import {
  ArrowRight,
  BookOpen,
  BrainCircuit,
  Check,
  Network,
  Route,
} from 'lucide-react';
import { MainLayout } from '@/components/layout/MainLayout';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { LoginForm, RegisterForm } from '@/modules/auth';
import { WelcomeAudienceShowcase } from './welcome/WelcomeAudienceShowcase';
import { WelcomeLearningJourney } from './welcome/WelcomeLearningJourney';
import { WelcomeStory } from './welcome/WelcomeStory';

const learningLoop = [
  {
    icon: BrainCircuit,
    index: '01',
    title: '先看懂思路',
    description: 'AI 沿着关键步骤提问、提示与验证，让你知道为什么这样做。',
  },
  {
    icon: Network,
    index: '02',
    title: '再连起知识',
    description: '每道题都会回到相关概念，暴露薄弱点，也标出下一步。',
  },
  {
    icon: Route,
    index: '03',
    title: '最后形成路径',
    description: '掌握度持续反馈到练习安排，让学习节奏跟着你的状态变化。',
  },
] as const;

const subjects = [
  { name: '微积分', topics: ['极限', '导数', '积分', '级数'] },
  { name: '线性代数', topics: ['矩阵', '向量空间', '特征值'] },
  { name: '概率统计', topics: ['随机变量', '分布', '统计推断'] },
  { name: '离散数学', topics: ['逻辑', '图论', '组合数学'] },
] as const;

interface LearningLoopItemProps {
  item: (typeof learningLoop)[number];
  index: number;
  reduceMotion: boolean;
}

const LearningLoopItem = ({ item, index, reduceMotion }: LearningLoopItemProps) => {
  const Icon = item.icon;

  return (
    <motion.article
      initial={reduceMotion ? false : { opacity: 0, y: 28 }}
      whileInView={reduceMotion ? undefined : { opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.35 }}
      transition={{ duration: 0.55, delay: index * 0.08, ease: [0.22, 1, 0.36, 1] }}
      className="border-t border-surface-300 py-7 dark:border-surface-700"
    >
      <div className="flex items-start justify-between gap-5">
        <span className="text-xs font-bold text-primary-700 dark:text-primary-300">{item.index}</span>
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-200">
          <Icon className="h-5 w-5" aria-hidden="true" />
        </span>
      </div>
      <h3 className="mt-8 text-2xl font-bold tracking-normal text-surface-950 dark:text-white">
        {item.title}
      </h3>
      <p className="mt-3 max-w-sm text-sm leading-7 text-surface-600 dark:text-surface-300">
        {item.description}
      </p>
    </motion.article>
  );
};

export const WelcomePage = () => {
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [isRegisterMode, setIsRegisterMode] = useState(false);
  const storyRef = useRef<HTMLElement>(null);
  const reduceMotion = useReducedMotion() ?? false;

  const handleLogin = () => {
    setIsRegisterMode(false);
    setIsLoginModalOpen(true);
  };

  const handleRegister = () => {
    setIsRegisterMode(true);
    setIsLoginModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsLoginModalOpen(false);
    setIsRegisterMode(false);
  };

  return (
    <MainLayout
      className="welcome-page-layout"
      headerVariant="default"
      footerVariant="default"
      onLoginClick={handleLogin}
      onRegisterClick={handleRegister}
    >
      <Modal
        isOpen={isLoginModalOpen}
        onClose={handleCloseModal}
        showHeader={false}
        ariaLabel={isRegisterMode ? '创建账号' : '登录'}
        className="max-h-[calc(100vh-2rem)] max-w-[600px] overflow-hidden rounded-2xl border-white/20 bg-white p-0 transition-[max-width,padding] duration-500 motion-reduce:transition-none dark:bg-surface-900 lg:max-w-[1000px]"
      >
        {isRegisterMode ? (
          <RegisterForm onSwitchToLogin={() => setIsRegisterMode(false)} />
        ) : (
          <LoginForm
            onSuccess={() => setIsLoginModalOpen(false)}
            onSwitchToRegister={() => setIsRegisterMode(true)}
          />
        )}
      </Modal>

      <div className="relative overflow-x-clip bg-surface-50 text-surface-900 transition-colors duration-300 dark:bg-surface-950 dark:text-surface-100">
        <WelcomeStory sectionRef={storyRef} onStart={handleLogin} />

        <section className="content-visibility-auto bg-white px-5 py-20 dark:bg-surface-950 sm:px-8 lg:py-28">
          <div className="mx-auto max-w-7xl">
            <div className="grid gap-8 lg:grid-cols-[0.8fr_1.2fr] lg:items-end">
              <h2 className="max-w-xl text-4xl font-bold leading-tight tracking-normal text-surface-950 dark:text-white sm:text-5xl">
                <span className="block">从问题，</span>
                <span className="block">到真正掌握。</span>
              </h2>
              <p className="max-w-2xl text-base leading-8 text-surface-600 dark:text-surface-300 lg:justify-self-end">
                学习不是在几个功能之间来回切换。高数智学把提问、理解、练习与反馈放在同一条连续的学习闭环里。
              </p>
            </div>

            <div className="mt-14 grid gap-x-10 md:grid-cols-3">
              {learningLoop.map((item, index) => (
                <LearningLoopItem
                  key={item.title}
                  item={item}
                  index={index}
                  reduceMotion={reduceMotion}
                />
              ))}
            </div>
          </div>
        </section>

        <WelcomeAudienceShowcase />

        <WelcomeLearningJourney />

        <section className="content-visibility-auto border-y border-surface-200 bg-surface-100 px-5 py-20 dark:border-surface-800 dark:bg-surface-900 sm:px-8 lg:py-24">
          <div className="mx-auto max-w-7xl">
            <div className="flex flex-col justify-between gap-5 md:flex-row md:items-end">
              <div>
                <BookOpen className="h-6 w-6 text-primary-600 dark:text-primary-400" aria-hidden="true" />
                <h2 className="mt-5 text-3xl font-bold tracking-normal text-surface-950 dark:text-white sm:text-4xl">
                  大学数学核心课程
                </h2>
              </div>
              <p className="max-w-lg text-sm leading-7 text-surface-600 dark:text-surface-300">
                从基础概念到综合应用，课程、练习与知识关系使用同一套学习脉络组织。
              </p>
            </div>

            <div className="mt-12 grid border-t border-surface-300 dark:border-surface-700 sm:grid-cols-2 lg:grid-cols-4">
              {subjects.map((subject, index) => (
                <div
                  key={subject.name}
                  className={`py-7 sm:px-6 ${
                    index > 0 ? 'border-t border-surface-300 dark:border-surface-700 sm:border-t-0' : ''
                  } ${index % 2 === 1 ? 'sm:border-l sm:border-surface-300 sm:dark:border-surface-700' : ''} ${
                    index > 1 ? 'sm:border-t sm:border-surface-300 sm:dark:border-surface-700 lg:border-t-0' : ''
                  } ${index > 0 ? 'lg:border-l lg:border-surface-300 lg:dark:border-surface-700' : ''}`}
                >
                  <p className="text-xl font-bold tracking-normal text-surface-900 dark:text-surface-100">
                    {subject.name}
                  </p>
                  <ul className="mt-4 space-y-2">
                    {subject.topics.map((topic) => (
                      <li key={topic} className="flex items-center gap-2 text-sm text-surface-600 dark:text-surface-300">
                        <Check className="h-3.5 w-3.5 text-emerald-500" aria-hidden="true" />
                        {topic}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="content-visibility-auto bg-white px-5 py-20 dark:bg-surface-950 sm:px-8 lg:py-28">
          <motion.div
            initial={reduceMotion ? false : { opacity: 0, y: 24 }}
            whileInView={reduceMotion ? undefined : { opacity: 1, y: 0 }}
            viewport={{ once: true, amount: 0.5 }}
            transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
            className="mx-auto flex max-w-5xl flex-col items-center text-center"
          >
            <h2 className="text-4xl font-bold leading-tight tracking-normal text-surface-950 dark:text-white sm:text-5xl">
              下一道难题，从这里开始。
            </h2>
            <p className="mt-5 max-w-2xl text-base leading-8 text-surface-600 dark:text-surface-300">
              登录后继续你的学习记录，让每一次提问都沉淀为可见的进步。
            </p>
            <Button
              type="button"
              size="lg"
              onClick={handleLogin}
              className="group mt-8 h-12 rounded-md bg-primary-600 px-8 text-white shadow-lg shadow-primary-600/20 hover:bg-primary-700"
            >
              立即开始学习
              <ArrowRight className="ml-2 h-4 w-4 transition-transform duration-300 group-hover:translate-x-1 motion-reduce:transition-none" aria-hidden="true" />
            </Button>
          </motion.div>
        </section>
      </div>
    </MainLayout>
  );
};

export default WelcomePage;
