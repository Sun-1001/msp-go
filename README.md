# MathStudyPlatform

**基于多智能体协作与深度知识追踪的高等数学教育生态系统**

## 项目简介

MathStudyPlatform 是一个旨在革新高等数学教育的智能学习平台。区别于传统的问答机器人，本系统构建了一个复杂的多智能体系统（Multi-Agent System, MAS），将大语言模型（LLM）的自然语言理解能力与符号计算引擎的精确性相结合，利用贝叶斯知识追踪（BKT）实现精准的个性化路径规划。

## 核心特性

- **智能对话学习** — 基于 LangGraph 的多智能体协作（4 核心智能体 + 1 纯函数路由），支持概念讲解、问题求解、错误诊断
- **自适应练习** — 根据学生知识状态（BKT 模型）动态调整题目难度和类型
- **知识图谱** — 基于 AntV G6 可视化高等数学知识结构，追踪学习进度
- **错题本** — 智能归纳错误类型，提供针对性复习建议
- **数学公式渲染** — 基于 KaTeX 的高质量数学公式显示，支持 Markdown 流式渲染
- **多角色支持** — 学生、教师、管理员三种角色，满足不同使用场景
- **班级管理** — 教师创建班级、学生加入班级、班级成员管理
- **学习资源管理** — 资源收藏、分类浏览、智能推荐
- **西电教务集成** — 对接西安电子科技大学教务系统，自动同步课表/成绩/考试数据
- **AI 学生画像** — 基于学习数据生成个性化学生画像与学情分析
- **安全审计** — 完整的安全日志系统，支持事件监控、告警与自动归档
- **监控指标** — Prometheus 指标采集，支持系统运行状态监控

## 核心设计理念

1. **神经符号双脑协同 (Neuro-Symbolic Synergy)**
   - **右脑 (LLM)**: 负责语义理解、教学话术生成和情感支持
   - **左脑 (Symbolic Engine)**: 利用 SymPy 负责严谨的数学推导和真值验证，消除计算"幻觉"

2. **多智能体协作 (Multi-Agent Collaboration)**
   - 采用 LangGraph 框架编排专门的智能体群（求解者、导师、诊断者、追踪者），通过状态机管理复杂的教学工作流

3. **数据驱动的自适应性 (Data-Driven Adaptivity)**
   - 利用贝叶斯知识追踪（BKT）模型实时预测学生的知识状态，构建基于知识图谱的动态学习路径

## 技术栈

| 层级 | 技术选型 | 版本 |
|------|----------|------|
| **前端框架** | React + TypeScript | 19.2 + 5.9 |
| **状态管理** | Redux Toolkit | 2.11 |
| **路由** | React Router | 7.9 |
| **样式** | TailwindCSS | 4.1 |
| **动画** | Framer Motion | 12.x |
| **数学渲染** | KaTeX + remark-math + rehype-katex | 0.16 |
| **图表** | ECharts | 6.0 |
| **知识图谱可视化** | AntV G6 | 5.0 |
| **表单** | React Hook Form + Zod | 7.71 + 4.3 |
| **HTTP 客户端** | Axios | 1.13 |
| **构建工具** | Vite | 7.2 |
| **后端框架** | Go `net/http` | Go 1.25+ |
| **后端语言** | Go（默认启动） | 1.25+ |
| **数据访问** | pgx + go-redis | P1 骨架 |
| **数据库迁移** | Alembic legacy 基线，Go 迁移器待接入 | 迁移期 |
| **数据库** | PostgreSQL + pgvector | 18+ |
| **缓存** | Redis | 7+ |
| **认证** | JWT 兼容迁移中 | P3 |
| **AI 工作流** | LangGraph | 0.2+ |
| **LLM 集成** | LiteLLM (OpenAI 兼容) | 1.30+ |
| **数学计算** | SymPy + NumPy + SciPy | — |
| **数据处理** | Pandas | 2.1+ |
| **图像/OCR** | Pillow + pytesseract | — |
| **监控** | Prometheus text exposition | P1 占位 |
| **序列化** | Go JSON + legacy orjson 对照 | 迁移期 |
| **SSE** | Go 迁移中 | P4 |
| **容器化** | Docker + Docker Compose | — |
| **反向代理** | Nginx | alpine |

## 项目结构

```
MathStudyPlatform/
├── frontend/                          # React 前端应用
│   ├── src/
│   │   ├── pages/                     # 页面组件 (学生/教师/管理员/公共)
│   │   ├── components/                # 通用组件 (UI/布局/图表/聊天)
│   │   ├── modules/                   # 业务功能模块 (16 个)
│   │   │   ├── admin/                 # 管理员模块
│   │   │   ├── ai-config/             # AI 配置模块
│   │   │   ├── analytics/             # 学习分析模块
│   │   │   ├── auth/                  # 认证模块
│   │   │   ├── classroom/             # 班级管理
│   │   │   ├── exercise/              # 自适应练习
│   │   │   ├── knowledge/             # 知识图谱
│   │   │   ├── mistake/               # 错题本
│   │   │   ├── password-reset/        # 密码重置
│   │   │   ├── question/              # 题目管理
│   │   │   ├── resource/              # 资源管理
│   │   │   ├── session/               # AI 学习会话
│   │   │   ├── student/               # 学生模块
│   │   │   ├── teacher/               # 教师模块
│   │   │   ├── upload/                # 文件上传
│   │   │   └── xidian/                # 西电教务集成
│   │   ├── store/                     # Redux 状态管理
│   │   ├── libs/                      # 工具库 (HTTP/动画/数学/验证/导出)
│   │   ├── hooks/                     # 自定义 Hooks
│   │   ├── types/                     # TypeScript 类型定义
│   │   └── app/                       # 应用层 (Provider/路由)
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── Dockerfile
│
├── backend-go/                        # Go 后端服务（默认启动入口）
│   ├── cmd/api/                       # API 进程入口
│   ├── internal/platform/             # 配置、HTTP、中间件、健康检查、指标
│   ├── go.mod
│   └── Dockerfile
│
├── backend/                           # Legacy FastAPI 后端（迁移参考，不再默认启动）
│   ├── app/
│   │   ├── main.py                    # 应用入口
│   │   ├── config.py                  # 配置管理
│   │   ├── api/v1/                    # REST API 接口层
│   │   │   ├── auth.py                # 认证
│   │   │   ├── session.py             # 学习会话
│   │   │   ├── exercise.py            # 练习
│   │   │   ├── questions.py           # 题目
│   │   │   ├── mistakes.py            # 错题
│   │   │   ├── progress.py            # 进度
│   │   │   ├── classes.py             # 班级
│   │   │   ├── resources.py           # 资源
│   │   │   ├── upload.py              # 上传
│   │   │   ├── xidian.py              # 西电教务
│   │   │   ├── portrait.py            # 学生画像
│   │   │   ├── teacher_stats.py       # 教师统计
│   │   │   └── admin/                 # 管理员接口
│   │   ├── domain/                    # 领域层 (DDD)
│   │   │   ├── models/                # 领域模型 (10 个)
│   │   │   └── services/              # 领域服务
│   │   ├── services/                  # 应用服务层 (27 个服务)
│   │   ├── infrastructure/            # 基础设施层
│   │   │   ├── database/              # 数据库 ORM 模型与会话
│   │   │   ├── repositories/          # 仓储模式 (8 个仓储)
│   │   │   └── cache/                 # Redis 缓存与内存缓存
│   │   ├── agents/                    # 多智能体系统
│   │   │   ├── core/                  # 核心组件 (LiteLLM/路由/状态/缓存)
│   │   │   ├── roles/                 # 智能体角色 (4 个)
│   │   │   └── workflow/              # LangGraph 工作流
│   │   └── core/                      # 安全/异常/中间件/日志
│   ├── alembic/                       # 数据库迁移 (18 个迁移文件)
│   ├── tests/                         # 测试 (API 测试 + 单元测试)
│   ├── scripts/                       # 脚本 (初始化/种子数据)
│   └── pyproject.toml
│
├── docs/                              # 项目文档
│   ├── api/                           # API 接口规范
│   ├── architecture/                  # 架构设计文档
│   ├── design/                        # 详细设计文档
│   ├── deployment/                    # 部署指南
│   └── development/                   # 开发文档
│
├── .env.example                       # 唯一环境变量模板
├── docker-compose.yml                 # Docker Compose 编排
└── nginx-site.conf                    # Nginx 站点配置
```

## 智能体系统

平台采用多智能体协作架构，基于 LangGraph 编排。经过重构，从 9 个智能体精简为 **4 个核心智能体 + 1 个纯函数路由**，LLM 调用次数从 3-6 次/请求降至 1-2 次/请求。

### 工作流架构

```
用户输入 → Router(纯函数) → [MathSolver | Tutor | Diagnostician] → Tracker → 输出
```

### 智能体列表

| 智能体 | 职责 | LLM 调用 | 触发条件 |
|--------|------|----------|----------|
| **Router** (纯函数) | 意图识别、任务路由 | 0 次 | 所有用户输入（entry 节点内） |
| **MathSolver** | 数学问题求解 (SymPy)，内置安全检查和结果验证 | 2 次 | 计算/求解请求 |
| **Tutor** | 概念讲解、苏格拉底式教学、学习路径规划 | 1 次 | 概念询问/一般对话/规划请求 |
| **Diagnostician** | 错误诊断、步骤比对、错误分类 | 1 次 | 答案提交/图片上传 |
| **Tracker** (零LLM) | 学习追踪，更新 mastery_vector / error_tendency | 0 次 | 每次交互后自动运行 |

### 性能优化

| 任务类型 | 优化前 | 优化后 |
|----------|--------|--------|
| 求解任务 | 3-6 次 LLM 调用 | 2 次 |
| 讲解任务 | 2-4 次 LLM 调用 | 1 次 |
| 诊断任务 | 2-5 次 LLM 调用 | 1 次 |
| 学习追踪 | 需要 LLM | 0 次（纯计算） |

## 功能模块

### 学生端

| 功能 | 路径 | 说明 |
|------|------|------|
| 课程总览 | `/course/overview` | 查看学习进度和课程内容 |
| 智能刷题 | `/exercise` | AI 驱动的自适应练习（BKT 难度调节） |
| 学习会话 | `/session/new` | 与 AI 导师进行对话式学习 |
| 错题本 | `/mistake-book` | 错题归纳与智能复习建议 |
| 知识图谱 | `/knowledge-graph` | 可视化知识结构与掌握度 |
| 学习路径 | `/learning-path` | 个性化学习规划 |
| 学习分析 | `/analytics` | 学习数据统计与置信度分析 |
| 学习资源 | `/resources` | 学习资料与收藏管理 |
| 我的班级 | `/my-class` | 查看/加入/退出班级 |
| 西电教务 | `/xidian` | 同步课表/成绩/考试数据 |
| 学生画像 | — | AI 生成的个性化学习画像 |

### 教师端

| 功能 | 路径 | 说明 |
|------|------|------|
| 教师仪表盘 | `/teacher/dashboard` | 教学数据概览 |
| 学生管理 | `/teacher/students` | 查看学生学习情况 |
| 班级管理 | `/teacher/classes` | 创建班级、查看详情、移除学生 |
| 题库管理 | `/teacher/question-bank` | 创建和管理题目 |
| 作业管理 | `/teacher/assignments` | 布置和批改作业 |
| 教学分析 | `/teacher/analytics` | 班级学习数据分析 |
| 教学资源 | `/teacher/resources` | 教学资料管理 |

### 管理员端

| 功能 | 路径 | 说明 |
|------|------|------|
| 管理控制台 | `/admin/dashboard` | 平台运营数据概览 |
| 用户管理 | `/admin/users` | 用户账户的增删改查 |
| 统计分析 | `/admin/stats` | 平台使用统计与数据分析 |
| AI 模型配置 | `/admin/ai-models` | 配置 LLM 渠道和模型（LiteLLM 多 Provider） |
| 知识图谱管理 | `/admin/knowledge` | 知识节点的增删改查 |
| 系统设置 | `/admin/settings` | 平台参数配置 |
| 安全日志 | `/admin/security-logs` | 安全事件监控与审计 |

### 公共页面

| 功能 | 路径 | 说明 |
|------|------|------|
| 使用指南 | `/guide` | 平台功能介绍与快速入门 |
| 常见问题 | `/faq` | FAQ 问答 |
| 团队介绍 | `/about` | 项目愿景与核心理念 |
| 联系我们 | `/contact` | 联系方式与服务时间 |
| 隐私政策 | `/privacy-policy` | 用户隐私保护说明 |
| 服务条款 | `/terms-of-service` | 平台使用条款 |

## 后端服务层

平台采用分层架构（DDD），服务层包含 27 个应用服务：

| 服务 | 职责 |
|------|------|
| `AuthService` | 用户认证与授权（JWT） |
| `SessionService` | AI 学习会话管理 |
| `ExerciseService` | 自适应练习管理 |
| `MistakeService` | 错题本管理 |
| `ProgressService` | 学习进度追踪 |
| `BKTService` | 贝叶斯知识追踪模型 |
| `ClassService` | 班级管理 |
| `ResourceService` | 学习资源管理 |
| `AIConfigService` | AI 模型配置管理 |
| `QuestionAIService` | AI 题目生成 |
| `StudentPortraitService` | 学生画像生成 |
| `XidianService` | 西电教务系统集成 |
| `AdminStatsService` | 管理员统计分析 |
| `AdminUserService` | 管理员用户管理 |
| `KnowledgeAdminService` | 知识图谱管理 |
| `ContentService` | 内容管理 |
| `UploadService` | 文件上传 |
| `EncryptionService` | 数据加密（Fernet） |
| `PasswordResetService` | 密码重置 |
| `SecurityLogService` | 安全日志记录 |
| `LogCleanupService` | 日志自动清理与归档 |
| `AlertService` | 告警服务 |
| `HealthChecker` | 健康检查 |
| `SystemSettingService` | 系统设置管理 |
| `DatabaseManagementService` | 数据库管理 |
| `TaskManager` | 后台任务调度管理 |

## 快速开始

### 环境要求

- **Node.js** >= 18.x
- **Go** >= 1.25
- **Python** >= 3.11（仅 legacy 后端对照和历史 Alembic 手动操作需要）
- **PostgreSQL** >= 14（需安装 pgvector 扩展）
- **Redis** >= 6

### 1. 克隆项目

```bash
git clone https://github.com/your-org/MathStudyPlatform.git
cd MathStudyPlatform
```

### 2. 启动后端

```bash
cd backend-go

# Go 服务优先读取进程环境变量，并兼容读取仓库根目录 ../.env
# 本地和云端都使用同名 .env，内容按环境分别维护

# 下载依赖并启动服务
go mod download
go run ./cmd/api
```

### 3. 启动前端

```bash
cd frontend

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

### 4. 访问应用

- **前端应用**: http://localhost:5173
- **Go 后端健康检查**: http://localhost:8000/health
- **迁移期 API 行为**: 未迁移的 `/api/v1/*` 接口返回 `501 NOT_IMPLEMENTED`

## 环境变量配置

### 单一环境文件 (`.env`)

本地和云端都使用仓库根目录的 `.env`，文件名保持一致，内容按环境分别维护。`.env` 不提交，`.env.example` 是唯一模板。

```bash
cp .env.example .env
# 编辑 .env，设置当前环境的数据库、Redis、JWT、CORS 和对象存储配置
```

## 部署

### Docker 生产环境部署

项目提供完整的 Docker Compose 生产环境配置，包含 4 个服务：

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| PostgreSQL | `pgvector/pgvector:pg18-trixie` | 5432 | 带 pgvector 向量扩展 |
| Redis | `redis:7-alpine` | 6379 | 缓存与会话存储 |
| Backend | 自定义 (Go Alpine) | 8000 | Go 后端服务 |
| Frontend | 自定义 (Nginx Alpine) | 9000 | 静态资源服务 |

```bash
# 1. 配置当前环境变量
cp .env.example .env
# 编辑 .env，修改所有密码、密钥、域名和对象存储配置

# 2. 构建镜像
docker build -t your-username/backend-go:latest ./backend-go
docker build -t your-username/frontend:latest ./frontend

# 3. 启动服务
docker compose -f docker-compose.yml up -d
```

> Go 迁移器尚未接入，默认生产启动不再运行 Python Alembic。如需初始化 legacy schema，请按 `docs/backend-python-to-go-refactor.md` 手动执行基线迁移。

### 手动部署

**前端构建**
```bash
cd frontend
npm run build
# 构建产物在 dist/ 目录，使用 Nginx 托管
```

**后端启动**
```bash
cd backend-go
go run ./cmd/api
```

## 开发指南

### 代码规范

**前端**
```bash
cd frontend
npm run lint          # ESLint 检查
npm run build         # TypeScript 编译 + Vite 构建
```

**后端**
```bash
cd backend-go
go test ./...
go vet ./...
gofmt -w .
```

**Legacy Python 后端（迁移对照，不再默认启动）**
```bash
cd backend
ruff check .
mypy app
```

### 测试

**Go 后端**
```bash
cd backend-go
go test ./...
```

**Legacy Python 后端（迁移对照）**
```bash
cd backend
pytest                        # 运行所有测试
pytest --cov=app              # 带覆盖率报告
pytest tests/api/ -v          # API 测试
pytest tests/unit/ -v         # 单元测试
```

### 数据库迁移

```bash
cd backend

# Legacy Alembic 仅用于冻结和对照旧 schema，默认启动/部署不再自动执行
alembic revision --autogenerate -m "描述变更内容"

# 手动应用 legacy 基线
alembic upgrade head

# 回滚迁移
alembic downgrade -1

# 查看迁移历史
alembic history
```

### Git 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

类型: feat | fix | docs | style | refactor | perf | test | chore
范围: auth | exercise | session | ui | api | domain | agent | admin | security | resource
```

示例：
```bash
git commit -m "feat(agent): 添加数学求解智能体"
git commit -m "fix(auth): 修复 JWT 过期处理逻辑"
git commit -m "perf(exercise): 优化 BKT 模型计算性能"
```

## 架构说明

### 前端架构

```
src/
├── app/                  # 应用层：Provider 注入、路由配置
├── pages/                # 页面组件：按角色分组 (student/teacher/admin/public)
├── components/           # 通用组件：UI 基础组件、布局、图表
├── modules/              # 业务模块：16 个独立功能模块，每个模块包含组件/hooks/服务
├── store/                # 状态管理：Redux Toolkit (slices + selectors)
├── libs/                 # 工具库：HTTP 客户端、动画、数学渲染、表单验证、导出
├── hooks/                # 自定义 Hooks：复用逻辑抽象
└── types/                # 类型定义：API 类型、模型类型、通用类型
```

### 后端架构 (DDD 分层)

```
app/
├── api/v1/               # 接口层：REST API 端点、请求/响应 Schema
├── domain/               # 领域层：领域模型、领域服务
│   ├── models/           # 10 个领域模型 (Student, Exercise, BKT, KnowledgeNode...)
│   └── services/         # 领域服务
├── services/             # 应用层：27 个应用服务，编排领域逻辑
├── infrastructure/       # 基础设施层
│   ├── database/         # ORM 模型、异步数据库会话 (asyncpg)
│   ├── repositories/     # 8 个仓储 (User, Knowledge, BKT, AIConfig, Content...)
│   └── cache/            # Redis 缓存、内存缓存、统计缓存
├── agents/               # 智能体系统
│   ├── core/             # LiteLLM 客户端、纯函数路由、状态管理、数学等价性检查
│   ├── roles/            # 4 个智能体角色
│   └── workflow/         # LangGraph 工作流图、节点、边、检查点
└── core/                 # 横切关注点
    ├── security.py       # 安全工具
    ├── middleware/        # 限流中间件、Prometheus 指标中间件
    ├── log_sanitizer.py  # 日志脱敏
    └── exception_handlers.py  # 全局异常处理
```

## 数据库设计

### 核心领域模型

| 模型 | 说明 |
|------|------|
| `Student` | 学生信息、学习偏好 |
| `LearningSession` | AI 学习会话记录 |
| `Exercise` | 练习题目与答题记录 |
| `KnowledgeNode` | 知识图谱节点 |
| `BKT` | 贝叶斯知识追踪参数（掌握度、学习率、猜测率、失误率） |
| `AIConfig` | AI 模型配置（多 Provider 支持） |
| `SecurityLog` | 安全审计日志 |
| `PasswordReset` | 密码重置请求 |
| `Content` | 学习内容管理 |
| `Embedding` | 向量嵌入（pgvector） |

### 数据库迁移历史

项目包含 18 个 Alembic 迁移文件，涵盖：
- 初始化表结构（用户、学生、知识图谱等）
- 系统设置与安全日志
- 性能索引优化
- 西电教务集成
- 学生画像字段
- 班级管理
- 知识图谱种子数据
- 密码重置与错题本索引
- BKT 模型相关表

## 项目文档

| 文档 | 说明 |
|------|------|
| [后端 Python 到 Go 重构迁移文档](docs/backend-python-to-go-refactor.md) | 后端 Go 重写阶段计划、验收规则和进度记录 |

## 发展路线图

### 已完成

- ✅ 核心智能体系统：LangGraph 工作流、4 核心智能体 + 1 纯函数路由
- ✅ 神经符号协同：LLM + SymPy 双引擎数学求解
- ✅ 贝叶斯知识追踪（BKT）：实时掌握度评估与自适应练习
- ✅ 前端应用：React 19 + TypeScript 5.9，16 个业务模块
- 🚧 后端服务：Go API 骨架已作为默认启动入口；legacy FastAPI 保留为迁移参考
- ✅ 多角色支持：学生/教师/管理员三种角色
- ✅ 班级管理：创建班级、加入班级、成员管理
- ✅ 西电教务集成：课表/成绩/考试数据同步
- ✅ AI 学生画像：基于学习数据生成个性化画像
- ✅ LLM 多 Provider：LiteLLM 统一接口，支持 OpenAI 兼容 API
- ✅ 安全审计：安全日志、告警服务、日志自动清理与归档
- ✅ 知识图谱可视化：AntV G6 交互式知识图谱
- ✅ 数学公式渲染：KaTeX + Markdown 流式渲染
- ✅ Docker 容器化：多阶段构建、生产环境编排
- ✅ 监控指标：Prometheus 指标采集

### 进行中

- 🚧 测试覆盖：前端单元测试 (Vitest) + 后端 API/单元测试 (pytest)
- 🚧 性能优化：前端代码分割、后端查询优化
- 🚧 文档完善：智能体工作流详细文档、数据库 Schema 文档

### 计划中

- 📋 系统集成：LTI/OneRoster 对接，支持更多教务系统
- 📋 灰度测试：小规模试点，收集用户反馈
- 📋 扩展学科：支持线性代数、概率论等其他数学学科
- 📋 强化学习优化：基于用户反馈优化智能体策略
- 📋 移动端支持：开发移动端应用或响应式优化

## 许可证

[MIT License](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'feat: Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request
