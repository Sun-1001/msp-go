# 系统架构

本文描述 MathStudyPlatform 当前有效的技术架构。未完成工作见 [项目待办](../TODO.md)，历史时间点资料见 [归档索引](../archive/README.md)。

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

具体版本以 [backend/go.mod](../../backend/go.mod) 和 [frontend/package.json](../../frontend/package.json) 为准。

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
backend/
├── cmd/api/                    # API 入口和依赖装配
├── cmd/migrate/                # 数据库迁移入口
├── internal/application/       # 用例编排和事务边界
├── internal/adapter/http/      # REST/SSE handler、鉴权和错误映射
├── internal/adapter/postgres/  # pgx Repository 和读模型
├── internal/adapter/llm/       # Eino Agent 适配
├── internal/adapter/storage/   # 本地、七牛和 S3 存储
├── internal/integration/       # 西电账户验证等外部集成
├── internal/platform/          # 配置、HTTP 公共能力、缓存、指标和安全基础设施
└── migrations/                 # Go forward migrations
```

依赖方向以应用层接口为中心：HTTP 适配器负责协议转换，PostgreSQL、Redis、存储、LLM 和外部服务通过适配器接入，应用服务负责业务规则与事务编排。

| 层 | 负责 | 不负责 |
|----|------|--------|
| `cmd` | 进程入口、依赖装配、生命周期 | 业务规则 |
| `platform` | 配置、日志、HTTP 基础设施、缓存、指标和安全公共能力 | 具体领域用例 |
| `application` | 用例、权限、事务和领域流程编排 | SQL、HTTP DTO 和 provider 细节 |
| `adapter/http` | 路由、请求解析、响应与错误映射 | 复杂业务判断 |
| `adapter/postgres` | SQL、Repository、事务实现和读模型 | HTTP 协议语义 |
| `adapter/redis` | 缓存、限流、租约和可恢复的短期状态 | 唯一业务事实 |
| `adapter/llm`、`adapter/storage` | AI 与对象存储实现 | 向应用层泄露供应商协议 |
| `integration` | 微信、西电等第三方系统边界 | 绕过应用层直接修改业务数据 |

测试源码不属于永久目录结构。每次变更在生产代码完成后创建临时单元、集成或契约测试，验证并记录结果后删除；测试运行器配置可保留供后续重复使用。

## 核心领域

| 领域 | 主要职责 |
|------|----------|
| Auth/Admin | 登录、JWT/Cookie 兼容、用户、密码重置和平台设置 |
| Session/Exercise | 学习会话、题目生成、判题、诊断、错题和 DKT 更新 |
| Progress/Portrait | 掌握度、学习路径、统计、知识图谱和学生画像 |
| Classroom/Teacher | 班级、成员、题库、教学资源和教师分析 |
| Daily Question | 上海自然日固定题、班级统一/个性化分配、教师计划与公众号提醒 |
| Resource/Upload | 资源元数据、收藏、上传、对象存储和管理员运行时配置 |
| AI Config | provider、model、凭据和 Agent 运行配置 |
| Xidian/Security | 西电账户绑定、安全日志、告警、健康检查和指标 |

## API 与兼容契约

- 业务 API 默认使用 `/api/v1`，健康检查和指标使用明确的独立入口。
- Auth/Admin、Session/Exercise、Progress/Portrait、Classroom/Teacher、Resource/Upload、AI Config 和 Xidian/Security 的路由由对应 HTTP adapter 承接；实际路由注册是接口清单的代码事实来源。
- 前端依赖的 JSON 字段名保持稳定；错误响应保留稳定的 `code`、`message` 和 HTTP 状态码。
- JWT、Cookie、刷新令牌和角色权限共同构成认证兼容边界。
- 上传访问入口保留 `/uploads`；调整路径时必须同步修改前端、存储配置和部署代理。
- 未知 `/api/v1/*` 路径返回 JSON `404 NOT_FOUND`，不会回落到其他运行时。
- 学生学习统计和画像共用 `application/learningrange` 的北京时间窗口；`content_attempts` 及画像时间字段按 UTC 无时区约定解释，查询边界先转换为 UTC，日/周聚合再转换为北京时间，周序列保留范围起点所在的不完整周并以实际起点展示。每日一题的业务日期和调度独立使用上海自然日，两类模块通过练习事实协作，不互相读取业务表。
- `/progress/portrait-insights` 提供确定性的画像洞察：卡片主值统计学生全部练习，匿名班级对比只使用相同时间范围内的教师课程题。练习量、时长和活跃天数以全班其他同学（含零记录）为分母，正确率与知识点掌握度只比较达到各自练习次数和置信度门槛的有效样本；接口返回比较口径但不返回同伴身份。
- 画像行动由学生通过 `/progress/portrait-actions/{concept_id}/start` 显式开始，并独立保存在 `student_portrait_actions`；开始时冻结对应知识点的 DKT 尝试次数，完成度取当前次数与基线的差值，包括每日一题按冻结题目快照产生的有效练习。进行中的行动优先于最新推荐返回，不会因知识点离开当前薄弱项而丢失；已完成行动再次成为薄弱项时可显式开始新一轮，未完成行动的重复开始保持幂等。练习提交和每日一题模块均无需读写画像状态，画像也不读取每日一题业务表。
- `/portrait` 继续负责可选的 AI 文字解读。生成输入把“范围内行为数据”和“当前累计 DKT 掌握状态”拆开，结构化画像与 AI 解读统一通过 `application/masteryprojection` 计算不回写数据库的当前遗忘投影；范围内最近知识点从 DKT 状态及其最后练习时间读取，不反查可变题目或每日一题业务表。报告保存 `portrait_range` 和 `portrait_snapshot_at`，并通过 `portrait_revision` 乐观并发控制防止生成/删除互相覆盖；切换范围后旧报告会标记为不匹配。结构化画像、班级比较和行动入口不依赖 AI 是否配置或报告是否已经生成，删除报告也不会删除学习记录。

## 关键技术决策

- HTTP 使用标准库 `net/http` ServeMux；数据访问使用 pgx，Redis 使用 go-redis。
- AI/Agent 通过 Eino 和 OpenAI-compatible provider 接入，运行配置持久化到数据库。
- 对象存储仅使用管理员保存的加密数据库配置；未配置时运行时保持停用，保存前完成真实写入探测，成功后通过原子运行时快照即时切换，进行中的请求继续使用原快照，读取不会跨后端回退。
- 数据库只追加经过评审的 Go forward migration，不自动执行 down migration。
- 每日一题的教师提醒通过独立的、无正文的公众号任务事件持久化；手动提醒、每天 08:00 的自动提醒和统一题低库存预警均不创建站内通知。自动开关当天开启会即时尝试入队，08:00 失败在当天重试；发送前会重新确认学生仍有未完成可作答题目，或教师统一题日程仍只剩对应的一道题。
- Go API 是唯一后端进程入口，不保留 Python 运行时兼容层。
- JWT HMAC 契约保持前后端兼容；邮件发送使用受配置和安全边界约束的 SMTP adapter。

## AI 与降级边界

七类 Agent 配置分别为 `tutor`、`portrait`、`diagnostician`、`math_solver`、`question_parser`、`question_generator` 和 `ocr`。运行时优先读取数据库中的 Agent 配置；部分既有能力在无模型时使用本地确定性实现或模板降级。

关键契约：

- 图片作答从当前写入后端及仍已配置的 Local/Qiniu/S3 命名空间回读 PNG、JPEG 或 GIF，并在完整解码、OCR 置信度和数学判定均可靠后才开启事务；失败不产生 attempt、diagnosis、learning session、DKT 或 profile 更新。
- 判题结果只有 `correct`、`incorrect`、`indeterminate` 三态；本地确定性比较不能覆盖的代数、三角、极限、导数、积分、方程/解集、矩阵和证明题可交给 Eino Math Solver，服务不可用、超时、无效输出或低置信度统一返回带阶段、原因和重试语义的降级结果。
- 无缓存解析时，Math Solver 不接收标准答案并独立求解；候选最终答案以及推导步骤需经过单独的 `solution_verification` 调用，未验证步骤不会返回给前端。
- 自主出题模型不可用或结构化输出非法时返回 `503 AI_GENERATION_UNAVAILABLE`，不保存题目。
- 外部 provider、上传地址和西电账户验证地址经过出站地址校验，默认阻断本地和内网目标。

尚未完成的能力与验收项只在 [项目待办](../TODO.md) 中维护。

## 数据与迁移

PostgreSQL 是业务数据源，Redis 用于缓存和运行时辅助状态。数据库结构由 `backend/migrations/` 中的 Go forward migration 管理；首次生产基线按基础结构、AI 风控、站内通信和外部通知四个领域分组，发布后的变化只追加新版本。历史 Alembic 链和开发期增量链已退出当前工作区。迁移规则见 [Go 数据库迁移策略](../../backend/migrations/README.md)。
