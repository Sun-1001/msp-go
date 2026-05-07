# MathStudyPlatform

**Go 后端驱动的高等数学学习平台；AI/Agent 能力保留 TODO 接口，待新工作流设计**

## 项目简介

MathStudyPlatform 是一个高等数学学习平台。当前后端迁移版以 Go 服务作为唯一默认运行入口，承接认证、学习、题库、班级、资源、上传、西电教务、管理员设置、统计和安全审计等非 AI 能力；旧 Python FastAPI 后端已从当前代码树清理。

AI/Agent、LLM、OCR 和数学求解工作流不在本轮迁移范围内。相关前端入口与 API 边界保留，但 Go 后端返回明确的 TODO/占位响应，后续需要基于新的工作流重新设计和实现。

## 核心特性

- **学习会话接口** — Go 后端保留会话、历史、任务取消和 SSE 形状兼容响应；完整 AI/Agent 回复工作流为 TODO
- **自适应练习** — 根据学生知识状态（BKT 模型）动态调整题目难度和类型
- **知识图谱** — 基于 AntV G6 可视化高等数学知识结构，追踪学习进度
- **错题本** — 智能归纳错误类型，提供针对性复习建议
- **数学公式渲染** — 基于 KaTeX 的高质量数学公式显示，支持 Markdown 流式渲染
- **多角色支持** — 学生、教师、管理员三种角色，满足不同使用场景
- **班级管理** — 教师创建班级、学生加入班级、班级成员管理
- **学习资源管理** — 资源收藏、分类浏览、文件上传和对象存储
- **西电教务集成** — 对接西安电子科技大学教务系统，自动同步课表/成绩/考试数据
- **学生画像** — Go 后端提供基于学习数据的模板画像；LLM 画像质量为 TODO
- **安全审计** — 完整的安全日志系统，支持事件监控、告警与自动归档
- **监控指标** — Prometheus 指标采集，支持系统运行状态监控

## 核心设计理念

1. **非 AI 能力先稳定**
   - 当前 Go 后端优先承接可确定的业务流程、数据读写、鉴权、审计和集成能力。
   - AI/Agent/OCR/LLM 旧实现不直接迁移，避免把 Python 工作流作为新的运行时依赖。

2. **接口边界保留**
   - AI 配置、题目 AI 解析、会话聊天、图片诊断和画像生成保留 API 形状或明确 TODO 响应。
   - 后续恢复 AI 能力时应通过模块化接口接入新的工作流，而不是回退到旧 Python 服务。

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
| **数据访问** | pgx + go-redis | Go 后端 |
| **数据库迁移** | Go migration runner + retired Alembic baseline | Go 后端 |
| **数据库** | PostgreSQL + pgvector | 18+ |
| **缓存** | Redis | 7+ |
| **认证** | Go JWT + Cookie 兼容层 | Go 后端 |
| **AI 工作流** | TODO，占位接口；旧 LangGraph 工作流已移除 | P6 待设计 |
| **LLM 集成** | TODO，占位接口；旧 LiteLLM 集成已移除 | P6 待设计 |
| **数学计算** | TODO；旧 SymPy 求解器已移除 | P6 待设计 |
| **图像/OCR** | TODO；旧 OCR 工作流已移除 | P6 待设计 |
| **监控** | Prometheus text exposition | Go 后端 |
| **序列化** | Go JSON | Go 后端 |
| **SSE** | Go 形状兼容降级，完整 Agent 流式回复 TODO | Go 后端 |
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
│   │   │   ├── session/               # 学习会话（AI 回复 TODO）
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

## AI/Agent 状态

当前 Go 后端不运行旧 Python LangGraph/LiteLLM/SymPy/OCR 工作流；相关代码已从当前代码树移除。生产启动、Docker Compose 和 Nginx 默认入口均指向 Go。

已保留的 TODO 边界：

- `/api/v1/admin/ai-config/*`：管理员鉴权后返回 `501 AI_CONFIG_TODO`。
- `/api/v1/questions/ai-parse`：返回非 LLM 形状兼容解析结果。
- `/api/v1/session/{id}/chat`：保存用户消息并返回 SSE 形状兼容占位回复。
- `/api/v1/portrait/generate`：生成模板画像，LLM 画像质量 TODO。
- `/api/v1/exercise/submit`：文本基础判题已迁移，OCR、数学等价和 LLM 诊断质量 TODO。

后续恢复 AI 能力时，需要先补充新的工作流 ADR、接口边界和验收方案，禁止静默回落 Python。

## 功能模块

### 学生端

| 功能 | 路径 | 说明 |
|------|------|------|
| 课程总览 | `/course/overview` | 查看学习进度和课程内容 |
| 智能刷题 | `/exercise` | BKT 驱动的自适应练习；OCR/LLM 诊断 TODO |
| 学习会话 | `/session/new` | 会话管理已迁移；完整 AI 导师回复 TODO |
| 错题本 | `/mistake-book` | 错题归纳与智能复习建议 |
| 知识图谱 | `/knowledge-graph` | 可视化知识结构与掌握度 |
| 学习路径 | `/learning-path` | 个性化学习规划 |
| 学习分析 | `/analytics` | 学习数据统计与置信度分析 |
| 学习资源 | `/resources` | 学习资料与收藏管理 |
| 我的班级 | `/my-class` | 查看/加入/退出班级 |
| 西电教务 | `/xidian` | 同步课表/成绩/考试数据 |
| 学生画像 | — | 模板画像已迁移；LLM 画像 TODO |

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
| AI 模型配置 | `/admin/ai-models` | Go 保留 501 TODO 占位；新 AI 工作流待设计 |
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

## Go 后端服务层

平台后端默认运行 `backend-go/`，采用 `cmd/api` 装配、`application` 用例编排、`adapter/http` 路由、`adapter/postgres` 数据访问和 `platform` 基础设施分层。主要应用服务如下：

| 服务 | 职责 |
|------|------|
| `AuthService` | 用户认证与授权（JWT） |
| `SessionService` | 学习会话管理和 AI 回复占位 |
| `ExerciseService` | 自适应练习管理 |
| `MistakeService` | 错题本管理 |
| `ProgressService` | 学习进度追踪 |
| `BKTService` | 贝叶斯知识追踪模型 |
| `ClassroomService` | 班级管理 |
| `ResourceService` | 学习资源管理 |
| `QuestionService` | 题库 CRUD、批量操作和 AI 解析占位 |
| `PortraitService` | 学生画像读取、清理和模板生成 |
| `XidianService` | 西电教务系统集成 |
| `AdminStatsService` | 管理员统计分析 |
| `AdminUserService` | 管理员用户管理 |
| `KnowledgeAdminService` | 知识图谱管理 |
| `UploadService` | 文件上传 |
| `Secret/Fernet` | 西电密码加密 |
| `PasswordResetService` | 密码重置 |
| `SecurityLogService` | 安全日志记录 |
| `HealthChecker` | 健康检查 |
| `SystemSettingService` | 系统设置管理 |
| `DatabaseManagementService` | 数据库管理 |
| `AdminAIConfigPlaceholder` | `/admin/ai-config/*` 认证与 `AI_CONFIG_TODO` 占位 |

## 快速开始

### 环境要求

- **Node.js** >= 18.x
- **Go** >= 1.25
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
- **AI TODO 行为**: `/api/v1/admin/ai-config/*` 返回 `501 AI_CONFIG_TODO`；其他 AI 质量能力使用 Go 占位/降级实现

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

> 默认生产启动不运行 Python 或 Alembic。Go 后端镜像包含 `msp-migrate`，`scripts/deploy.sh` 和 `scripts/update.sh` 会在启动应用容器前运行 Go migration runner。

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

### 测试

**Go 后端**
```bash
cd backend-go
go test ./...
go test ./tests/contract -count=1
```

### 数据库迁移

```bash
cd backend-go

# 应用 Go 迁移
go run ./cmd/migrate

# 历史 Alembic 链已下线；新增变更使用 Go forward migration
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

### 后端架构 (Go DDD 分层)

```
backend-go/
├── cmd/api/              # API 进程入口和依赖装配
├── cmd/migrate/          # Go 数据库迁移入口
├── internal/application/ # 用例编排：auth/session/exercise/upload/xidian/admin...
├── internal/adapter/http # REST handler 和错误映射
├── internal/adapter/postgres # pgx repository
├── internal/adapter/storage  # local/S3/Qiniu 上传存储适配
├── internal/integration/ # 第三方集成，例如 xidian
├── internal/platform/    # config/httpserver/middleware/metrics/health/redis/postgres
└── migrations/           # Go migration runner 使用的 SQL 基线
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
| `AIConfig` | AI 模型配置 legacy 表；Go 当前仅保留 TODO 占位接口 |
| `SecurityLog` | 安全审计日志 |
| `PasswordReset` | 密码重置请求 |
| `Content` | 学习内容管理 |
| `Embedding` | 向量嵌入（pgvector） |

### 数据库迁移历史

`backend-go/migrations/0001_initial_schema.up.sql` 是从已下线 Alembic head 冻结出的 Go 初始 schema，后续生产迁移由 Go migration runner 承接。旧 Alembic 链不再保留在当前代码树。

## 项目文档

| 文档 | 说明 |
|------|------|
| [后端 Python 到 Go 重构迁移文档](docs/backend-python-to-go-refactor.md) | 后端 Go 重写阶段计划、验收规则和进度记录 |

## 发展路线图

### 已完成

- ✅ Go 后端默认入口：Docker Compose、Nginx、Vite 代理和 `start.bat` 均指向 Go API
- ✅ 非 AI 后端迁移：认证、用户、学习、题库、资源、班级、教师统计、管理员、上传、西电教务和安全审计已迁移到 Go
- ✅ 贝叶斯知识追踪（BKT）：实时掌握度评估与自适应练习
- ✅ 前端应用：React 19 + TypeScript 5.9，16 个业务模块
- ✅ 多角色支持：学生/教师/管理员三种角色
- ✅ 班级管理：创建班级、加入班级、成员管理
- ✅ 西电教务集成：课表/成绩/考试数据同步
- ✅ 学生画像：Go 模板画像已迁移；LLM 画像质量 TODO
- ✅ AI 接口边界：`/admin/ai-config/*` 已保留管理员鉴权 501 TODO 占位
- ✅ 安全审计：安全日志、告警服务、日志自动清理与归档
- ✅ 知识图谱可视化：AntV G6 交互式知识图谱
- ✅ 数学公式渲染：KaTeX + Markdown 流式渲染
- ✅ Docker 容器化：多阶段构建、生产环境编排
- ✅ 监控指标：Prometheus 指标采集

### 进行中

- 🚧 P8 契约验证：继续收敛非 AI API 契约、运行时 smoke 和性能基线
- 🚧 P9 Python 下线：后端 Python 代码已清理，仍需 Docker/Compose 实机烟测和最终下线记录
- 🚧 文档完善：Go 后端模块、部署和数据库 Schema 文档继续收敛

### 计划中

- 📋 AI/Agent 新工作流：重新设计 LLM provider、Agent 编排、数学求解、OCR 和诊断能力
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
