import React, { useRef, useState, useMemo } from 'react';
import { motion, useScroll, useTransform, useInView } from 'framer-motion';
import { Button } from '../../components/ui/Button';
import { MainLayout } from '../../components/layout/MainLayout';
import { Modal } from '../../components/ui/Modal';
import { LoginForm, RegisterForm } from '@/modules/auth';
import {
  ArrowRight,
  ChevronDown,
  Sparkles,
  BrainCircuit,
  Globe,
  GraduationCap,
  Zap,
  Activity,
  Check,
  Play,
  CheckCircle2,
  BookOpen,
  Target,
  TrendingUp,
  Layers,
  LineChart,
  Award,
  Users,
  Clock,
  Lightbulb
} from 'lucide-react';

// --- 简化的动画配置 ---
const fadeInUp = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0 }
};

const staggerContainer = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: { staggerChildren: 0.1 }
  }
};

// --- 简化的特性卡片 ---
const FeatureCard = ({
  icon,
  title,
  desc,
  gradient
}: {
  icon: React.ReactNode;
  title: string;
  desc: string;
  gradient: string;
}) => {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: "-50px" });

  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, y: 30 }}
      animate={isInView ? { opacity: 1, y: 0 } : {}}
      transition={{ duration: 0.5 }}
      className="group relative"
    >
      <div className={`absolute inset-0 ${gradient} rounded-2xl blur-xl opacity-0 group-hover:opacity-40 transition-opacity duration-500`} />
      <div className="relative h-full bg-white/90 dark:bg-surface-800/90 backdrop-blur-sm border border-surface-200/80 dark:border-surface-700/80 rounded-2xl p-8 shadow-sm hover:shadow-lg transition-all duration-300 hover:-translate-y-1">
        <div className={`mb-6 p-4 ${gradient} rounded-xl inline-block text-white shadow-lg`}>
          {icon}
        </div>
        <h3 className="text-xl font-bold mb-3 text-surface-900 dark:text-surface-100">{title}</h3>
        <p className="text-surface-600 dark:text-surface-400 leading-relaxed text-sm">{desc}</p>
      </div>
    </motion.div>
  );
};

// --- 统计数字组件 ---
const StatItem = ({ value, label, icon }: { value: string; label: string; icon: React.ReactNode }) => {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true });

  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, scale: 0.9 }}
      animate={isInView ? { opacity: 1, scale: 1 } : {}}
      transition={{ duration: 0.4 }}
      className="text-center p-6 rounded-2xl bg-white/60 dark:bg-surface-800/60 backdrop-blur-sm border border-surface-100 dark:border-surface-700 hover:border-primary-200 dark:hover:border-primary-700 transition-colors"
    >
      <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-primary-50 dark:bg-primary-900/50 text-primary-500 dark:text-primary-400 mb-4">
        {icon}
      </div>
      <div className="text-3xl md:text-4xl font-bold text-surface-900 dark:text-surface-100 mb-1">{value}</div>
      <div className="text-surface-500 dark:text-surface-400 text-sm font-medium">{label}</div>
    </motion.div>
  );
};

// --- 学习步骤组件 ---
const StepCard = ({
  step,
  icon,
  title,
  desc,
  isLast
}: {
  step: number;
  icon: React.ReactNode;
  title: string;
  desc: string;
  isLast?: boolean;
}) => {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: "-30px" });

  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, x: -20 }}
      animate={isInView ? { opacity: 1, x: 0 } : {}}
      transition={{ duration: 0.5, delay: step * 0.1 }}
      className="relative flex gap-6"
    >
      {/* 连接线 */}
      {!isLast && (
        <div className="absolute left-6 top-16 w-0.5 h-[calc(100%-2rem)] bg-linear-to-b from-primary-200 dark:from-primary-700 to-transparent" />
      )}

      {/* 步骤图标 */}
      <div className="relative z-10 shrink-0">
        <div className="w-12 h-12 rounded-full bg-linear-to-br from-primary-500 to-secondary-500 flex items-center justify-center text-white shadow-lg shadow-primary-500/30">
          {icon}
        </div>
      </div>

      {/* 内容 */}
      <div className="flex-1 pb-10">
        <div className="flex items-center gap-3 mb-2">
          <span className="text-xs font-bold text-primary-500 dark:text-primary-400 bg-primary-50 dark:bg-primary-900/50 px-2 py-1 rounded-full">
            步骤 {step}
          </span>
        </div>
        <h4 className="text-lg font-bold text-surface-900 dark:text-surface-100 mb-2">{title}</h4>
        <p className="text-surface-600 dark:text-surface-400 text-sm leading-relaxed">{desc}</p>
      </div>
    </motion.div>
  );
};

// --- 学科覆盖卡片 ---
const SubjectCard = ({ title, topics, color }: { title: string; topics: string[]; color: string }) => (
  <motion.div
    initial={{ opacity: 0, y: 20 }}
    whileInView={{ opacity: 1, y: 0 }}
    viewport={{ once: true }}
    transition={{ duration: 0.4 }}
    className="bg-white dark:bg-surface-800 rounded-2xl p-6 border border-surface-100 dark:border-surface-700 hover:border-primary-200 dark:hover:border-primary-700 transition-all hover:shadow-md"
  >
    <div className={`w-10 h-10 rounded-lg ${color} flex items-center justify-center mb-4`}>
      <BookOpen className="w-5 h-5 text-white" />
    </div>
    <h4 className="font-bold text-surface-900 dark:text-surface-100 mb-3">{title}</h4>
    <div className="flex flex-wrap gap-2">
      {topics.map((topic, i) => (
        <span key={i} className="text-xs px-2 py-1 bg-surface-50 dark:bg-surface-700 text-surface-600 dark:text-surface-300 rounded-full">
          {topic}
        </span>
      ))}
    </div>
  </motion.div>
);

// --- 主页面组件 ---
export const WelcomePage: React.FC = () => {
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [isRegisterMode, setIsRegisterMode] = useState(false);
  const { scrollY } = useScroll();

  // Hero 区域视差效果
  const heroOpacity = useTransform(scrollY, [0, 400], [1, 0]);
  const heroY = useTransform(scrollY, [0, 400], [0, 100]);

  const handleLogin = () => {
    setIsRegisterMode(false);
    setIsLoginModalOpen(true);
  };

  const handleRegister = () => {
    setIsRegisterMode(true);
    setIsLoginModalOpen(true);
  };

  // 学科数据
  const subjects = useMemo(() => [
    { title: '微积分', topics: ['极限', '导数', '积分', '级数'], color: 'bg-primary-500' },
    { title: '线性代数', topics: ['矩阵', '行列式', '向量空间', '特征值'], color: 'bg-secondary-500' },
    { title: '概率统计', topics: ['概率论', '统计推断', '分布', '假设检验'], color: 'bg-emerald-500' },
    { title: '离散数学', topics: ['集合论', '图论', '逻辑', '组合数学'], color: 'bg-amber-500' }
  ], []);

  return (
    <MainLayout headerVariant="default" footerVariant="default" onLoginClick={handleLogin} onRegisterClick={handleRegister}>
      <Modal
        isOpen={isLoginModalOpen}
        onClose={() => {
          setIsLoginModalOpen(false);
          setIsRegisterMode(false);
        }}
        showHeader={false}
        ariaLabel={isRegisterMode ? '创建账号' : '登录'}
        className={isRegisterMode
          ? 'max-h-[calc(100vh-2rem)] overflow-y-auto'
          : 'max-h-[calc(100vh-2rem)] max-w-[600px] overflow-hidden rounded-2xl border-white/20 bg-white p-0 transition-[max-width,padding] duration-500 motion-reduce:transition-none dark:bg-surface-900 lg:max-w-[1000px]'}
      >
        {isRegisterMode ? (
          <RegisterForm
            onSwitchToLogin={() => setIsRegisterMode(false)}
          />
        ) : (
          <LoginForm
            onSuccess={() => setIsLoginModalOpen(false)}
            onSwitchToRegister={() => setIsRegisterMode(true)}
          />
        )}
      </Modal>

      <div className="relative w-full overflow-x-hidden bg-surface-50 dark:bg-surface-950 text-surface-900 dark:text-surface-100 transition-colors duration-300">

        {/* === Hero Section === */}
        <section className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden">
          {/* 背景装饰 - 使用纯 CSS */}
          <div className="absolute inset-0 z-0">
            <div className="absolute top-0 left-1/4 w-96 h-96 bg-primary-200/30 dark:bg-primary-500/20 rounded-full blur-[120px]" />
            <div className="absolute bottom-0 right-1/4 w-96 h-96 bg-secondary-200/30 dark:bg-secondary-500/20 rounded-full blur-[120px]" />
            <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-linear-to-r from-primary-100/20 to-secondary-100/20 dark:from-primary-500/10 dark:to-secondary-500/10 rounded-full blur-[100px]" />
          </div>

          {/* 网格背景 */}
          <div
            className="absolute inset-0 z-0 opacity-[0.02] dark:opacity-[0.05]"
            style={{
              backgroundImage: `linear-gradient(to right, currentColor 1px, transparent 1px), linear-gradient(to bottom, currentColor 1px, transparent 1px)`,
              backgroundSize: '60px 60px'
            }}
          />

          <motion.div
            style={{ opacity: heroOpacity, y: heroY }}
            className="relative z-10 text-center max-w-5xl mx-auto px-6 space-y-10"
          >
            {/* 标签 */}
            <motion.div
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6 }}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-full bg-white/80 dark:bg-surface-800/80 backdrop-blur-sm border border-surface-200 dark:border-surface-700 shadow-sm"
            >
              <Sparkles className="w-4 h-4 text-amber-500" />
              <span className="text-sm font-medium text-surface-700 dark:text-surface-300">新一代高等数学智能学习平台</span>
            </motion.div>

            {/* 主标题 */}
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ duration: 0.8, delay: 0.1 }}
              className="space-y-4"
            >
              <h1 className="text-6xl md:text-8xl lg:text-9xl font-bold tracking-tight">
                <span className="text-surface-900 dark:text-white">Math</span>{" "}
                <span className="bg-clip-text text-transparent bg-linear-to-r from-primary-500 via-secondary-500 to-primary-500 animate-gradient-x bg-size-[200%_auto]">
                  Study
                </span>
              </h1>
              <p className="text-3xl md:text-4xl lg:text-5xl text-surface-400 font-light">
                重塑你的数学思维
              </p>
            </motion.div>

            {/* 描述 */}
            <motion.p
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: 0.2 }}
              className="text-lg md:text-xl text-surface-600 dark:text-surface-400 max-w-2xl mx-auto leading-relaxed"
            >
              融合大语言模型与深度知识图谱，为你拆解每一个复杂的公式，
              <br className="hidden md:block" />
              让抽象的数学概念变得触手可及。
            </motion.p>

            {/* CTA 按钮 */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: 0.3 }}
              className="flex flex-col sm:flex-row items-center justify-center gap-4"
            >
              <Button
                onClick={handleLogin}
                size="lg"
                className="group relative px-8 py-4 text-base rounded-full bg-surface-900 dark:bg-white text-white dark:text-surface-900 overflow-hidden shadow-xl hover:shadow-2xl transition-all hover:-translate-y-0.5"
              >
                <div className="absolute inset-0 bg-linear-to-r from-primary-600 to-secondary-600 opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                <span className="relative z-10 flex items-center font-medium group-hover:text-white">
                  立即开始学习
                  <ArrowRight className="ml-2 w-5 h-5 transition-transform group-hover:translate-x-1" />
                </span>
              </Button>

              <Button
                variant="ghost"
                size="lg"
                className="px-8 py-4 text-base rounded-full text-surface-600 dark:text-surface-300 hover:text-surface-900 dark:hover:text-white hover:bg-white dark:hover:bg-surface-800 border border-surface-200 dark:border-surface-700 hover:border-surface-300 dark:hover:border-surface-600 transition-all"
              >
                <Play className="w-4 h-4 mr-2 fill-current" />
                观看演示
              </Button>
            </motion.div>
          </motion.div>

          {/* 滚动提示 */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 1, duration: 0.5 }}
            className="absolute bottom-8 left-1/2 -translate-x-1/2 cursor-pointer text-surface-400 hover:text-primary-500 dark:hover:text-primary-400 transition-colors"
            onClick={() => window.scrollTo({ top: window.innerHeight, behavior: 'smooth' })}
          >
            <ChevronDown className="w-8 h-8 animate-bounce" />
          </motion.div>
        </section>

        {/* === 统计数据区域 === */}
        <section className="relative py-20 bg-linear-to-b from-surface-50 to-white dark:from-surface-950 dark:to-surface-900">
          <div className="container mx-auto px-6">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
              <StatItem value="5000+" label="知识点覆盖" icon={<Layers className="w-5 h-5" />} />
              <StatItem value="10万+" label="AI 解析生成" icon={<BrainCircuit className="w-5 h-5" />} />
              <StatItem value="24/7" label="智能助教在线" icon={<Clock className="w-5 h-5" />} />
              <StatItem value="98%" label="用户满意度" icon={<Award className="w-5 h-5" />} />
            </div>
          </div>
        </section>

        {/* === 核心功能区域 === */}
        <section className="relative py-24 bg-white dark:bg-surface-900">
          <div className="container mx-auto px-6">
            <motion.div
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true }}
              variants={staggerContainer}
              className="text-center max-w-3xl mx-auto mb-16"
            >
              <motion.span
                variants={fadeInUp}
                className="inline-block px-4 py-1.5 rounded-full bg-primary-50 dark:bg-primary-900/50 text-primary-600 dark:text-primary-400 text-sm font-medium mb-4"
              >
                为什么选择我们
              </motion.span>
              <motion.h2
                variants={fadeInUp}
                className="text-3xl md:text-4xl font-bold mb-4 text-surface-900 dark:text-surface-100"
              >
                智能化的学习体验
              </motion.h2>
              <motion.p
                variants={fadeInUp}
                className="text-lg text-surface-500 dark:text-surface-400"
              >
                不仅仅是做题，更是构建完整的数学认知体系
              </motion.p>
            </motion.div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
              <FeatureCard
                icon={<BrainCircuit className="w-8 h-8" />}
                title="AI 深度解析"
                desc="基于大模型的推理能力，AI 助教实时拆解步骤，精准定位知识盲区。"
                gradient="bg-gradient-to-br from-primary-500 to-primary-600"
              />
              <FeatureCard
                icon={<Globe className="w-8 h-8" />}
                title="动态知识图谱"
                desc="可视化你的知识网络，发现概念之间的隐秘联系。"
                gradient="bg-gradient-to-br from-secondary-500 to-secondary-600"
              />
              <FeatureCard
                icon={<Target className="w-8 h-8" />}
                title="个性化学习路径"
                desc="基于你的认知水平动态调整，让每次练习都在最佳学习区间。"
                gradient="bg-gradient-to-br from-emerald-500 to-emerald-600"
              />
              <FeatureCard
                icon={<TrendingUp className="w-8 h-8" />}
                title="学习数据追踪"
                desc="全方位记录你的学习轨迹，用数据驱动进步。"
                gradient="bg-gradient-to-br from-amber-500 to-amber-600"
              />
            </div>
          </div>
        </section>

        {/* === 学习流程区域 === */}
        <section className="relative py-24 bg-surface-50 dark:bg-surface-950 overflow-hidden">
          <div className="container mx-auto px-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-16 items-center">
              {/* 左侧：步骤 */}
              <div>
                <motion.div
                  initial={{ opacity: 0, x: -30 }}
                  whileInView={{ opacity: 1, x: 0 }}
                  viewport={{ once: true }}
                  transition={{ duration: 0.6 }}
                  className="mb-12"
                >
                  <span className="inline-block px-4 py-1.5 rounded-full bg-secondary-50 dark:bg-secondary-900/50 text-secondary-600 dark:text-secondary-400 text-sm font-medium mb-4">
                    简单三步
                  </span>
                  <h2 className="text-3xl md:text-4xl font-bold text-surface-900 dark:text-surface-100 mb-4">
                    学习，<span className="text-transparent bg-clip-text bg-linear-to-r from-primary-600 to-secondary-600">从未如此直观</span>
                  </h2>
                  <p className="text-surface-500 dark:text-surface-400 text-lg">
                    从问题到掌握，AI 全程陪伴
                  </p>
                </motion.div>

                <div className="space-y-2">
                  <StepCard
                    step={1}
                    icon={<Zap className="w-5 h-5" />}
                    title="提出问题"
                    desc="输入数学问题、拍照上传题目，或者直接描述你不理解的概念。"
                  />
                  <StepCard
                    step={2}
                    icon={<Activity className="w-5 h-5" />}
                    title="AI 智能分析"
                    desc="AI 逐步推导解题过程，详细解释每一步的数学原理和思路。"
                  />
                  <StepCard
                    step={3}
                    icon={<CheckCircle2 className="w-5 h-5" />}
                    title="巩固练习"
                    desc="系统自动生成同类变式题，通过举一反三确保完全掌握。"
                    isLast
                  />
                </div>
              </div>

              {/* 右侧：演示界面 */}
              <motion.div
                initial={{ opacity: 0, x: 30 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.6 }}
                className="relative"
              >
                <div className="absolute inset-0 bg-linear-to-r from-primary-500/10 to-secondary-500/10 rounded-3xl blur-2xl" />
                <div className="relative rounded-2xl overflow-hidden border border-surface-200 dark:border-surface-700 shadow-2xl bg-white dark:bg-surface-800">
                  {/* 模拟窗口顶栏 */}
                  <div className="p-4 border-b border-surface-100 dark:border-surface-700 flex items-center gap-2 bg-surface-50 dark:bg-surface-900">
                    <div className="w-3 h-3 rounded-full bg-red-400" />
                    <div className="w-3 h-3 rounded-full bg-yellow-400" />
                    <div className="w-3 h-3 rounded-full bg-green-400" />
                    <span className="ml-4 text-xs text-surface-400 font-medium">MathStudy AI 助教</span>
                  </div>

                  {/* 对话内容 */}
                  <div className="p-6 space-y-4 min-h-[350px] bg-linear-to-b from-white to-surface-50 dark:from-surface-800 dark:to-surface-900">
                    {/* 用户消息 */}
                    <div className="flex gap-3">
                      <div className="w-8 h-8 rounded-full bg-surface-200 dark:bg-surface-700 flex items-center justify-center shrink-0">
                        <Users className="w-4 h-4 text-surface-500 dark:text-surface-400" />
                      </div>
                      <div className="bg-surface-100 dark:bg-surface-700 rounded-2xl rounded-tl-sm px-4 py-3 max-w-[80%]">
                        <p className="text-sm text-surface-700 dark:text-surface-300">求函数 f(x) = x³ - 3x² + 2 的极值点</p>
                      </div>
                    </div>

                    {/* AI 回复 */}
                    <div className="flex gap-3 flex-row-reverse">
                      <div className="w-8 h-8 rounded-full bg-linear-to-br from-primary-500 to-secondary-500 flex items-center justify-center shrink-0">
                        <Sparkles className="w-4 h-4 text-white" />
                      </div>
                      <div className="bg-linear-to-br from-primary-50 to-secondary-50 dark:from-primary-900/50 dark:to-secondary-900/50 border border-primary-100 dark:border-primary-800 rounded-2xl rounded-tr-sm px-4 py-4 max-w-[85%] space-y-3">
                        <div className="flex items-center gap-2 text-primary-600 dark:text-primary-400 font-medium text-sm">
                          <Lightbulb className="w-4 h-4" />
                          解题思路
                        </div>
                        <div className="space-y-2 text-sm text-surface-700 dark:text-surface-300">
                          <div className="flex items-start gap-2">
                            <Check className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                            <span>对 f(x) 求导：<code className="bg-white dark:bg-surface-800 px-1.5 py-0.5 rounded text-surface-900 dark:text-surface-100 font-mono text-xs">f'(x) = 3x² - 6x</code></span>
                          </div>
                          <div className="flex items-start gap-2">
                            <Check className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                            <span>令 f'(x) = 0，得 <code className="bg-white dark:bg-surface-800 px-1.5 py-0.5 rounded text-surface-900 dark:text-surface-100 font-mono text-xs">x = 0</code> 或 <code className="bg-white dark:bg-surface-800 px-1.5 py-0.5 rounded text-surface-900 dark:text-surface-100 font-mono text-xs">x = 2</code></span>
                          </div>
                          <div className="flex items-start gap-2">
                            <Check className="w-4 h-4 text-emerald-500 mt-0.5 shrink-0" />
                            <span>验证：x=0 为极大值点，x=2 为极小值点</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </motion.div>
            </div>
          </div>
        </section>

        {/* === 学科覆盖区域 === */}
        <section className="relative py-24 bg-white dark:bg-surface-900">
          <div className="container mx-auto px-6">
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              className="text-center max-w-3xl mx-auto mb-16"
            >
              <span className="inline-block px-4 py-1.5 rounded-full bg-emerald-50 dark:bg-emerald-900/50 text-emerald-600 dark:text-emerald-400 text-sm font-medium mb-4">
                全面覆盖
              </span>
              <h2 className="text-3xl md:text-4xl font-bold mb-4 text-surface-900 dark:text-surface-100">
                涵盖大学数学核心课程
              </h2>
              <p className="text-lg text-surface-500 dark:text-surface-400">
                从基础到进阶，系统化知识体系助你全面提升
              </p>
            </motion.div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
              {subjects.map((subject, index) => (
                <SubjectCard key={index} {...subject} />
              ))}
            </div>
          </div>
        </section>

        {/* === 特色优势区域 === */}
        <section className="relative py-24 bg-linear-to-b from-surface-50 to-white dark:from-surface-950 dark:to-surface-900 overflow-hidden">
          <div className="container mx-auto px-6">
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
              {/* 左侧大卡片 */}
              <motion.div
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                className="lg:col-span-2 relative rounded-3xl overflow-hidden bg-linear-to-br from-primary-500 to-secondary-600 p-10 text-white"
              >
                <div className="absolute top-0 right-0 w-64 h-64 bg-white/10 rounded-full blur-3xl -translate-y-1/2 translate-x-1/2" />
                <div className="relative z-10">
                  <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-white/20 text-sm font-medium mb-6">
                    <GraduationCap className="w-4 h-4" />
                    核心优势
                  </div>
                  <h3 className="text-3xl md:text-4xl font-bold mb-4">
                    让数学学习不再孤单
                  </h3>
                  <p className="text-white/80 text-lg mb-8 max-w-xl">
                    AI 助教 24 小时在线，随时解答你的疑惑。不论是深夜复习还是考前冲刺，都有智能伙伴陪伴左右。
                  </p>
                  <div className="flex flex-wrap gap-4">
                    <div className="flex items-center gap-2 bg-white/10 rounded-full px-4 py-2">
                      <Check className="w-4 h-4" />
                      <span className="text-sm">即时响应</span>
                    </div>
                    <div className="flex items-center gap-2 bg-white/10 rounded-full px-4 py-2">
                      <Check className="w-4 h-4" />
                      <span className="text-sm">个性化辅导</span>
                    </div>
                    <div className="flex items-center gap-2 bg-white/10 rounded-full px-4 py-2">
                      <Check className="w-4 h-4" />
                      <span className="text-sm">深度解析</span>
                    </div>
                  </div>
                </div>
              </motion.div>

              {/* 右侧小卡片 */}
              <div className="space-y-6">
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  viewport={{ once: true }}
                  transition={{ delay: 0.1 }}
                  className="rounded-2xl bg-white dark:bg-surface-800 border border-surface-100 dark:border-surface-700 p-6 shadow-sm hover:shadow-md transition-shadow"
                >
                  <div className="w-12 h-12 rounded-xl bg-amber-50 dark:bg-amber-900/30 flex items-center justify-center mb-4">
                    <LineChart className="w-6 h-6 text-amber-500" />
                  </div>
                  <h4 className="font-bold text-surface-900 dark:text-surface-100 mb-2">学习进度可视化</h4>
                  <p className="text-sm text-surface-500 dark:text-surface-400">
                    直观展示你的学习轨迹，清晰了解薄弱环节
                  </p>
                </motion.div>

                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  viewport={{ once: true }}
                  transition={{ delay: 0.2 }}
                  className="rounded-2xl bg-white dark:bg-surface-800 border border-surface-100 dark:border-surface-700 p-6 shadow-sm hover:shadow-md transition-shadow"
                >
                  <div className="w-12 h-12 rounded-xl bg-emerald-50 dark:bg-emerald-900/30 flex items-center justify-center mb-4">
                    <Users className="w-6 h-6 text-emerald-500" />
                  </div>
                  <h4 className="font-bold text-surface-900 dark:text-surface-100 mb-2">学习社区</h4>
                  <p className="text-sm text-surface-500 dark:text-surface-400">
                    与志同道合的学习者交流讨论，共同进步
                  </p>
                </motion.div>
              </div>
            </div>
          </div>
        </section>

        {/* === 底部 CTA 区域 === */}
        <section className="relative py-24 bg-linear-to-br from-primary-50 via-white to-secondary-50 dark:from-surface-900 dark:via-surface-900 dark:to-surface-900 overflow-hidden">
          {/* 背景装饰 */}
          <div className="absolute inset-0">
            <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-linear-to-r from-primary-500/10 to-secondary-500/10 dark:from-primary-500/20 dark:to-secondary-500/20 rounded-full blur-[120px]" />
            <div className="absolute top-0 right-0 w-96 h-96 bg-primary-200/30 dark:bg-primary-500/10 rounded-full blur-[100px]" />
            <div className="absolute bottom-0 left-0 w-96 h-96 bg-secondary-200/30 dark:bg-secondary-500/10 rounded-full blur-[100px]" />
          </div>

          <div className="container mx-auto px-6 text-center relative z-10">
            <motion.div
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.6 }}
              className="max-w-3xl mx-auto"
            >
              <h2 className="text-4xl md:text-5xl font-bold mb-6 text-surface-900 dark:text-white">
                准备好开始你的
                <span className="text-transparent bg-clip-text bg-linear-to-r from-primary-600 to-secondary-600 dark:from-primary-400 dark:to-secondary-400"> 数学之旅 </span>
                了吗？
              </h2>
              <p className="text-lg text-surface-600 dark:text-surface-400 mb-10 max-w-xl mx-auto">
                加入我们，体验智能化的数学学习方式，让每一次学习都充满收获。
              </p>
              <Button
                onClick={handleLogin}
                size="lg"
                className="px-10 py-5 text-lg rounded-full bg-linear-to-r from-primary-600 to-secondary-600 dark:from-white dark:to-white text-white dark:text-surface-900 hover:from-primary-700 hover:to-secondary-700 dark:hover:from-surface-100 dark:hover:to-surface-100 shadow-xl shadow-primary-500/20 dark:shadow-white/10 transition-all hover:scale-105"
              >
                立即开始学习
                <ArrowRight className="ml-2 w-5 h-5" />
              </Button>
            </motion.div>
          </div>
        </section>
      </div>
    </MainLayout>
  );
};

export default WelcomePage;
