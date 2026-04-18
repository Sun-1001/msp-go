# 高等数学智能学习平台 - 前端

React + TypeScript 前端应用，提供学生、教师、管理员三种角色的 Web 界面。

## 技术栈

- **框架**: React 19 + TypeScript
- **状态管理**: Redux Toolkit
- **路由**: React Router v7
- **样式**: TailwindCSS v4
- **动画**: Framer Motion
- **数学渲染**: KaTeX
- **图表**: ECharts + AntV G6
- **表单**: React Hook Form + Zod
- **HTTP**: Axios

## 快速开始

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 生产构建
npm run build

# 预览构建产物
npm run preview

# 代码检查
npm run lint
```

开发服务器默认运行在 http://localhost:5173

## 目录结构

```
src/
├── main.tsx              # 应用入口
├── App.tsx               # 根组件 (路由 + Provider)
├── pages/                # 页面组件
│   ├── common/           # 公共页面
│   ├── student/          # 学生页面
│   ├── teacher/          # 教师页面
│   └── admin/            # 管理员页面
├── components/           # 通用组件
│   ├── layout/           # 布局组件
│   ├── ui/               # 基础 UI 组件
│   └── charts/           # 图表组件
├── modules/              # 业务功能模块
├── store/                # Redux 状态管理
├── app/                  # 应用层（Provider、路由）
├── libs/                 # 工具库
├── hooks/                # 自定义 Hooks
├── types/                # TypeScript 类型
└── app/routes/           # 路由配置
```

## 主要功能

### 学生端
- 课程总览、智能刷题、AI 学习会话
- 错题本、知识图谱、学习路径
- 学习分析、测验、学习资源

### 教师端
- 教师仪表盘、学生管理
- 题库管理、作业管理、教学分析

### 管理员端
- 管理控制台、账户管理
- AI 模型配置、系统设置

## 开发规范

- TypeScript 严格模式，禁止隐式 any
- 使用函数式组件 + Hooks
- 样式使用 TailwindCSS + `cn()` 工具函数
- 状态管理使用 `useAppDispatch` / `useAppSelector`

## 相关文档

- [模块详细文档](./CLAUDE.md)
- [后端 Python 到 Go 重构迁移文档](../docs/backend-python-to-go-refactor.md)
