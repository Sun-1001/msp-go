# 系统架构

本文描述 MathStudyPlatform 当前有效的技术架构。历史迁移决策和阶段验证证据见 [后端迁移跟踪](../backend-python-to-go-refactor.md)。

## 系统边界

```text
Browser
  |
  v
React + Vite/Nginx
  |
  v
Go net/http API
  |-- PostgreSQL + pgvector
  |-- Redis
  |-- Local/Qiniu/S3 storage
  |-- OpenAI-compatible providers through Eino
  `-- Xidian IDS account verification
```

Go API 是唯一默认后端。旧 Python FastAPI、LangGraph、LiteLLM、SymPy 和 OCR 工作流不属于当前运行链路。

## 技术栈

| 层级 | 主要技术 |
|------|----------|
| 前端 | React 19、TypeScript 5.9、Vite 7、React Router、Redux Toolkit、Tailwind CSS |
| 交互与展示 | Framer Motion、KaTeX、ECharts、AntV G6、React Hook Form、Zod |
| 后端 | Go 1.25、`net/http`、pgx、go-redis |
| AI/Agent | CloudWeGo Eino、OpenAI-compatible ChatModel、持久化 provider/model/Agent 配置 |
| 数据 | PostgreSQL 18、pgvector、Redis 7 |
| 交付 | Docker、Docker Compose、Nginx、Prometheus text exposition |

具体版本以 [backend-go/go.mod](../../backend-go/go.mod) 和 [frontend/package.json](../../frontend/package.json) 为准。

## 前端分层

```text
frontend/src/
├── app/          # Provider、路由和应用装配
├── pages/        # 学生、教师、管理员、公共页面
├── modules/      # 业务模块及其组件、Hooks、Service、状态和类型
├── components/   # 通用 UI、布局、图表和聊天组件
├── store/        # Redux Toolkit 根 Store
├── libs/         # HTTP、SSE、数学渲染、验证和导出
├── hooks/        # 跨模块复用逻辑
└── types/        # 公共 API 与模型类型
```

页面保持为组合层，业务逻辑进入模块 Hook 或 Service。模块外部通过 `index.ts` 公共接口访问，避免深层路径耦合。

## Go 后端分层

```text
backend-go/
├── cmd/api/                    # API 入口和依赖装配
├── cmd/migrate/                # 数据库迁移入口
├── internal/application/       # 用例编排和事务边界
├── internal/adapter/http/      # REST/SSE handler、鉴权和错误映射
├── internal/adapter/postgres/  # pgx Repository 和读模型
├── internal/adapter/llm/       # Eino Agent 适配
├── internal/adapter/storage/   # 本地、七牛和 S3 存储
├── internal/integration/       # 西电账户验证等外部集成
├── internal/platform/          # 配置、HTTP 公共能力、缓存、指标和安全基础设施
├── migrations/                 # Go forward migrations
└── tests/contract/             # 路由、前端调用面和 AI 边界契约
```

依赖方向以应用层接口为中心：HTTP 适配器负责协议转换，PostgreSQL、Redis、存储、LLM 和外部服务通过适配器接入，应用服务负责业务规则与事务编排。

## 核心领域

| 领域 | 主要职责 |
|------|----------|
| Auth/Admin | 登录、JWT/Cookie 兼容、用户、密码重置和平台设置 |
| Session/Exercise | 学习会话、题目生成、判题、诊断、错题和 DKT 更新 |
| Progress/Portrait | 掌握度、学习路径、统计、知识图谱和学生画像 |
| Classroom/Teacher | 班级、成员、题库、教学资源和教师分析 |
| Resource/Upload | 资源元数据、收藏、上传和对象存储 |
| AI Config | provider、model、凭据和 Agent 运行配置 |
| Xidian/Security | 西电账户绑定、安全日志、告警、健康检查和指标 |

## AI 与降级边界

七类 Agent 配置分别为 `tutor`、`portrait`、`diagnostician`、`math_solver`、`question_parser`、`question_generator` 和 `ocr`。运行时优先读取数据库中的 Agent 配置；部分既有能力在无模型时使用本地确定性实现或模板降级。

关键契约：

- 图片作答只从当前 Local/Qiniu/S3 存储命名空间回读 PNG、JPEG 或 GIF，并在完整解码、OCR 置信度和数学判定均可靠后才开启事务；失败不产生 attempt、diagnosis、learning session、DKT 或 profile 更新。
- 判题结果只有 `correct`、`incorrect`、`indeterminate` 三态；本地确定性比较不能覆盖的代数、三角、极限、导数、积分、方程/解集、矩阵和证明题可交给 Eino Math Solver，服务不可用、超时、无效输出或低置信度统一返回带阶段、原因和重试语义的降级结果。
- 无缓存解析时，Math Solver 不接收标准答案并独立求解；候选最终答案以及推导步骤需经过单独的 `solution_verification` 调用，未验证步骤不会返回给前端。
- 自主出题模型不可用或结构化输出非法时返回 `503 AI_GENERATION_UNAVAILABLE`，不保存题目。
- 未知 `/api/v1/*` 路径返回 JSON `404 NOT_FOUND`，不回落到旧后端。
- 外部 provider、上传地址和西电账户验证地址经过出站地址校验，默认阻断本地和内网目标。

尚未完成的能力与验收项只在 [项目待办](../TODO.md) 中维护。

## 数据与迁移

PostgreSQL 是业务数据源，Redis 用于缓存和运行时辅助状态。数据库结构由 `backend-go/migrations/` 中的 Go forward migration 管理；历史 Alembic 链已退出当前工作区。迁移规则见 [Go 数据库迁移策略](../../backend-go/migrations/README.md)。
