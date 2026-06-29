# 后端 Python 到 Go 重构迁移文档

**Document status**: P4 in progress; P5 done; P6 AI/Agent in progress (Eino Tutor/Portrait/Diagnostician/Math Solver/Question Parser Agent and admin LLM/Agent config loop wired; OCR and broader math tools pending); P7 in progress; P8 static contract handoff done; P9 Python backend removed by user confirmation
**Last updated**: 2026-06-29
**适用范围**：原 `backend/` Python FastAPI 后端整体迁移到 Go 后端；`backend/` 已从当前工作区删除
**重构原则**：接口兼容、数据连续、分阶段验收、每阶段完成必须更新本文档

---

## 1. 文档使用规则

本文档是后端 Python -> Go 重构的唯一主跟踪文档。任何阶段开始、暂停、恢复、完成或范围变化，都必须同步更新本文档。

### 1.1 阶段完成标记要求

每个阶段完成前必须完成以下记录：

1. 在 [阶段总览](#5-阶段总览) 中将对应阶段状态改为 `DONE`。
2. 在 [阶段完成记录](#12-阶段完成记录) 中补充完成日期、负责人、验证命令、验证结果和遗留风险。
3. 如果产生架构选择，在 [架构决策记录](#11-架构决策记录) 中补充 ADR 条目。
4. 如果发现新风险，在 [风险清单](#10-风险清单) 中追加或更新状态。
5. 如果 API、数据表、部署方式发生变化，同步更新对应章节和相关文档链接。

未完成上述记录时，该阶段不能视为完成。

### 1.2 状态定义

| 状态 | 含义 |
|------|------|
| `TODO` | 尚未开始 |
| `IN_PROGRESS` | 正在执行 |
| `BLOCKED` | 被依赖、风险或决策阻塞 |
| `DONE` | 已完成并记录验收证据 |

---

## 2. 迁移目标

### 2.1 总目标

将现有 Python FastAPI 后端完整迁移到 Go 后端，在保持前端 API 契约、数据库数据和部署入口稳定的前提下，逐步替换 Python 服务。

最终状态：

- Go 后端承接所有非 AI `/api/v1` 业务接口、健康检查、监控指标和文件访问入口。
- Python 后端不再承载线上业务流量。
- 数据库迁移、缓存、对象存储、鉴权和审计能力在 Go 后端中具备等价实现。
- AI/Agent、LLM、OCR 和数学求解质量能力不沿旧 Python 实现迁移；P6 已先接入 Go/Eino 会话导师 Agent、画像 Agent、错因诊断 Agent、数学等价判定 Agent、题目解析 Agent 和后台 LLM/Agent 配置闭环，OCR 和更完整的通用数学求解仍需继续实现。
- 旧 Python 服务已按用户确认从当前工作区删除。

### 2.2 非目标

- 不在第一阶段重写前端业务。
- 不在未冻结 API 契约前改变前端请求路径和响应结构。
- 不在缺少数据备份和回滚方案时修改生产数据库结构。
- 不把临时兼容代码视为最终架构。
- 不在本轮迁移旧 Python AI/Agent/OCR/LLM 工作流；已落地的 Eino Tutor/Portrait/Diagnostician/Math Solver/Question Parser Agent 仅代表新架构首批能力，不能视为完整 AI 工作流完成。

---

## 3. 原 Python 后端基线（已删除）

### 3.1 技术栈

原后端曾位于 `backend/`。2026-05-07 用户明确确认“不用双跑，不用比对，用户自行测试”，随后 `backend/` 已从当前工作区删除。以下内容仅作为迁移历史记录：

| 类型 | 当前实现 |
|------|----------|
| Web 框架 | FastAPI |
| 运行时 | Python 3.11+ |
| ORM | SQLAlchemy async |
| 迁移 | Alembic |
| 数据库 | PostgreSQL + pgvector |
| 缓存 | Redis |
| AI 工作流 | LangGraph + LangChain + LiteLLM + SymPy |
| 认证 | JWT + Cookie/Token 刷新 |
| 监控 | Prometheus metrics |
| 文件 | 本地 uploads + 对象存储适配 |

### 3.2 应用入口与横切能力

当前 `backend/app/main.py` 负责：

- FastAPI 应用创建。
- 数据库和 Redis 生命周期管理。
- 管理员账号初始化。
- LLM 客户端池预热。
- 日志清理后台任务。
- 数据库连接池监控。
- 请求 ID、超时、安全头、GZip、CORS、限流、指标中间件。
- 全局异常处理器注册。
- `/health`、`/health/detailed`、`/metrics`。
- `/uploads` 静态目录挂载。

Go 重构时这些能力必须逐项映射，不能只迁移业务路由。

### 3.3 API 模块清单

当前 `/api/v1` 下已聚合的业务模块：

| 模块 | 当前前缀 | 说明 |
|------|----------|------|
| 认证 | `/auth` | 登录、注册、刷新、登出、用户信息、密码找回 |
| 学习会话 | `/session` | 会话创建、聊天、历史、结束、批量删除、任务取消 |
| 练习 | `/exercise` | 下一题、提交答案、详情、解析 |
| 错题本 | `/mistakes` | 错题列表、统计、掌握标记、删除、复习 |
| 题目管理 | `/questions` | CRUD、分组、统计、批量操作、AI 解析、导入 |
| 学习进度 | `/progress` | 总览、掌握度、路径、知识图谱、排行、章节 |
| 资源中心 | `/resources` | 资源 CRUD、统计、收藏 |
| 文件上传 | `/upload` | 图片和资源上传 |
| 西电教务 | `/xidian` | 绑定、同步课表/考试/成绩、快照 |
| 班级管理 | `/classes` | 班级 CRUD、成员、邀请和统计 |
| 教师统计 | `/teacher` | 教师视角统计 |
| 学生画像 | `/portrait` | 画像查询、生成、清理 |
| 管理员 AI 配置 | `/admin/ai-config` | LLM provider/model/agent 配置 |
| 管理员系统设置 | `/admin/settings` | 注册、通用设置、数据库导入导出和监控 |
| 管理员统计 | `/admin/stats` | 总览、用户增长、活动、系统状态 |
| 管理员用户 | `/admin/users` | 用户查询、更新、删除、导入导出 |
| 管理员安全日志 | `/admin/security-logs` | 查询、统计、清理、归档、导出 |
| 管理员知识点 | `/admin/knowledge` | 知识节点和关系维护 |
| 管理员信箱 | `/admin/inbox` | 密码重置审核 |

### 3.4 数据模型范围

当前 SQLAlchemy 模型至少覆盖：

- 用户、学生画像、认证与密码重置。
- DKT 学生知识点掌握状态。
- 班级、班级成员、西电账号和同步快照。
- 知识节点、知识关系。
- 内容、内容资产、访问控制、嵌入、审计、导入任务。
- Outbox 事件。
- 学习会话、会话消息。
- 练习提交、诊断报告、错题相关记录。
- 系统设置、安全日志、用户收藏。
- LLM provider、模型、Agent 模型配置。

Go 迁移必须先明确数据模型映射，再迁移业务逻辑，避免接口先行导致数据语义漂移。

---

## 4. 目标 Go 架构草案

### 4.1 目录结构建议

```text
backend-go/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── platform/
│   │   ├── config/
│   │   ├── logger/
│   │   ├── httpserver/
│   │   ├── middleware/
│   │   ├── metrics/
│   │   └── validation/
│   ├── domain/
│   │   ├── auth/
│   │   ├── learning/
│   │   ├── exercise/
│   │   ├── knowledge/
│   │   ├── resource/
│   │   ├── class/
│   │   ├── admin/
│   │   └── ai/
│   ├── application/
│   ├── adapter/
│   │   ├── http/
│   │   ├── postgres/
│   │   ├── redis/
│   │   ├── storage/
│   │   └── llm/
│   └── integration/
│       └── xidian/
├── migrations/
├── tests/
│   ├── contract/
│   ├── integration/
│   └── e2e/
├── go.mod
└── README.md
```

### 4.2 分层边界

| 层 | 职责 | 禁止事项 |
|----|------|----------|
| `cmd/api` | 启动进程、装配依赖、优雅关闭 | 放业务逻辑 |
| `platform` | 配置、日志、中间件、指标、HTTP server 基础设施 | 依赖具体业务模块 |
| `domain` | 领域实体、值对象、领域规则 | 直接访问数据库、Redis、HTTP |
| `application` | 用例编排、事务边界、权限编排 | 直接拼 SQL、泄露传输层 DTO |
| `adapter/http` | 路由、请求解析、响应格式、错误码映射 | 承载复杂业务规则 |
| `adapter/postgres` | Repository、SQL、事务实现 | 反向依赖 HTTP 层 |
| `adapter/redis` | 缓存、限流、短期状态 | 保存不可恢复的核心业务事实 |
| `adapter/llm` | LLM provider、Agent 调用抽象 | 在业务层散落 provider 细节 |
| `integration` | 第三方系统适配，如西电教务 | 绕过应用层直接写业务表 |

### 4.3 兼容策略

- API 路径默认保持 `/api/v1` 不变。
- 响应字段默认保持当前前端依赖的 JSON 名称不变。
- 错误响应必须保留稳定 `code`、`message` 和 HTTP 状态码。
- JWT、Cookie、刷新令牌和权限判断要做兼容测试。
- 数据库表名、关键列名和迁移历史在切换前保持可追溯。
- 上传文件访问路径 `/uploads` 必须保留，除非前端同步改造并记录。

---

## 5. 阶段总览

| 阶段 | 状态 | 目标 | 主要交付物 | 完成标记位置 |
|------|------|------|------------|--------------|
| P0 基线冻结 | TODO | 盘点 Python 后端、冻结 API 和数据基线 | API 清单、数据模型清单、现有测试基线 | 12.1 |
| P1 Go 技术选型与骨架 | DONE | 确认 Go 框架、ORM/SQL、配置、日志、测试栈 | `backend-go/` 骨架、ADR、健康检查 | 12.2 |
| P2 数据访问与迁移体系 | DONE | 建立 PostgreSQL、Redis、迁移和事务模式 | Repository 基础、迁移策略、集成测试 | 12.3 |
| P3 鉴权与用户域 | DONE | 迁移认证、用户、密码、权限基础能力 | `/auth`、用户上下文、管理员初始化 | 12.4 |
| P4 核心学习域 | IN_PROGRESS | 迁移会话、练习、错题、进度、画像 | `/session`、`/exercise`、`/mistakes`、`/progress`、`/portrait` | 12.5 |
| P5 内容与教学管理域 | DONE | 迁移题库、资源、班级、教师统计、知识点 | `/questions`、`/resources`、`/classes`、`/teacher`、`/admin/knowledge` | 12.6 |
| P6 AI and Agent capabilities | IN_PROGRESS | Integrate the new Eino-based AI/Agent architecture and remove explicit TODO/placeholders incrementally | Eino Tutor/Portrait/Diagnostician/Math Solver/Question Parser Agent and admin LLM/Agent config loop wired; OCR and broader math tools pending | 12.7 |
| P7 集成与运维域 | IN_PROGRESS | 迁移西电集成、上传、系统设置、安全日志、监控和管理员辅助能力 | `/xidian`、`/upload`、`/admin/settings`、`/admin/security-logs`、`/admin/inbox`、`/admin/stats`、`/metrics` | 12.8 |
| P8 静态契约验证与用户验收交接 | DONE | 保留 Go 静态契约守卫，运行时双跑和业务验收由用户自行执行 | Contract tests、用户验收交接记录 | 12.9 |
| P9 流量切换与 Python 下线 | DONE | 切换默认生产入口并删除旧 Python 后端目录 | 部署配置、下线记录、删除清单 | 12.10 |

---

## 6. 分阶段执行说明

### P0 基线冻结

目标：

- 生成当前 Python API 实际路由清单。
- 导出现有 OpenAPI JSON，并作为 P0 基线交付物记录。
- 盘点数据库表、索引、枚举、迁移头。
- 跑通当前 Python 后端测试并记录结果。

验收标准：

- API 基线文档完成。
- 数据模型基线完成。
- Python 现有测试结果已记录。
- 明确哪些接口以实际代码为准，哪些接口以文档为准。

### P1 Go 技术选型与骨架

目标：

- 建立 `backend-go/` 最小可运行服务。
- 实现 `/health`、`/health/detailed` 占位、`/metrics` 占位。
- 建立配置加载、结构化日志、请求 ID、错误响应格式。
- 完成技术选型 ADR。

待决策：

- HTTP router。
- 数据访问方式：`database/sql` + SQL builder、`sqlc`、`gorm` 或其他。
- 数据迁移工具。
- 校验库、日志库、测试断言和 mock 策略。

验收标准：

- Go 服务本地可启动。
- 健康检查可访问。
- 单元测试和 lint 命令可执行。
- Docker 本地构建方案明确。

### P2 数据访问与迁移体系

目标：

- 建立 PostgreSQL 连接池、事务封装、Repository 接口。
- 建立 Redis 客户端和缓存基础能力。
- 明确 Alembic 到 Go 迁移工具的衔接策略。
- 先迁移只读查询能力，再迁移写入能力。

验收标准：

- Repository 集成测试可连接测试库。
- 数据库迁移命令可重复执行。
- 回滚策略已记录。
- 事务边界有统一模式。

### P3 鉴权与用户域

目标：

- 迁移登录、注册、刷新、登出、当前用户信息。
- 兼容现有 JWT claims、过期策略和 Cookie 行为。
- 迁移密码强度校验、密码哈希、管理员初始化。
- 迁移用户权限和角色判断。

验收标准：

- `/auth/*` 契约测试通过。
- 旧 token 兼容策略明确。
- 权限失败、账号禁用、弱密码等错误路径覆盖。

### P4 核心学习域

目标：

- 迁移学习会话、练习、错题本、学习进度、学生画像。
- 先实现同步接口等价，再处理流式聊天、任务取消等异步能力。
- 统一诊断报告、练习提交、DKT 更新的事务边界。

验收标准：

- 核心学生端流程端到端通过。
- 练习提交和错题生成数据一致。
- 会话历史和消息顺序一致。
- 关键用例有单元测试和集成测试。

### P5 内容与教学管理域

目标：

- 迁移题库、资源中心、班级、教师统计和知识点管理。
- 保持批量发布、批量删除、导入、收藏、ACL 语义一致。
- 知识节点和关系维护必须保护图结构完整性。

验收标准：

- 教师端和管理员知识点核心流程通过。
- 批量接口具备部分失败记录或原子性说明。
- 资源权限和收藏行为与旧服务一致。

### P6 AI 与 Agent 能力

目标：

- 不承接旧 Python LLM provider、模型、Agent 配置管理实现，也不回退到 legacy LangGraph/LiteLLM/SymPy/OCR 工作流。
- 已落地 Go/Eino 会话 Tutor Agent：`/session/{id}/chat` 优先读取持久化 `tutor` Agent 配置调用 Eino ADK Tutor Agent；没有后台配置时再使用 `EINO_ENABLED=true`、`EINO_API_KEY`、`EINO_MODEL` 兼容配置；未配置或调用失败时保持明确降级回复。
- 已落地 Go/Eino 画像 Portrait Agent：`/portrait/generate` 优先读取持久化 `portrait` Agent 配置调用 Eino ADK Portrait Agent；没有后台配置、模型不可用或生成空内容时保留模板画像降级。
- 已落地 Go/Eino 数学等价判定 Math Solver Agent：`/exercise/submit` 对文本答案优先读取持久化 `math_solver` Agent 配置生成结构化答案比较；没有后台配置、模型不可用、JSON 格式无效或置信度不合规时保留本地规范化比较降级。
- 已落地 Go/Eino 错因诊断 Diagnostician Agent：`/exercise/submit` 对错误答案优先读取持久化 `diagnostician` Agent 配置生成结构化 C/P/L/S-Type 诊断；没有后台配置、模型不可用、JSON 格式无效或生成空内容时保留本地基础诊断降级。
- 已落地 Go/Eino 题目解析 Question Parser Agent：`/questions/ai-parse` 优先读取持久化 `question_parser` Agent 配置抽取题目候选；没有后台配置、模型不可用、JSON 格式无效或必填字段缺失时保留确定性形状兼容解析降级。
- `/admin/ai-config/*` 已实现管理员鉴权后的 provider/model/Agent 配置闭环，支持 provider/model CRUD、provider test、模型拉取、provider 模型列表替换和 Agent 配置 CRUD；前端 AI 模型设置页已具备后端闭环。
- OCR 和更完整的通用数学求解能力仍需继续用 Go 侧或明确批准的独立服务实现。

原则：

- 旧 Python 服务已从当前工作区删除，禁止把 legacy Python 作为运行时回退。
- 最终线上业务能力必须由 Go 后端或明确批准的独立服务承载。
- 所有临时桥接都必须记录退出条件。
- 每一个 AI slice 都必须同步更新接口边界、验收证据和剩余风险。

验收标准：

- Eino Tutor Agent 配置校验、会话历史拼接、附件上下文和未配置降级路径测试通过。
- Eino Portrait Agent 配置选择、画像 prompt 构造、模板降级路径和保存路径测试通过。
- Eino Math Solver Agent 配置选择、答案比较 prompt 构造、结构化 JSON 解析、置信度校验和本地比较降级路径测试通过。
- Eino Diagnostician Agent 配置选择、诊断 prompt 构造、结构化 JSON 解析、taxonomy 校验和本地降级路径测试通过。
- Eino Question Parser Agent 配置选择、题目解析 prompt 构造、结构化 JSON 解析、必填字段校验和确定性解析降级路径测试通过。
- LLM provider/model CRUD、provider test/fetch、Agent 配置 CRUD 与前端 AI 模型设置页联通。
- Agent 配置可被运行时选择使用，而不是仅依赖 `EINO_*` 单组环境变量。
- OCR、通用数学求解和教学反馈 Agent 的正常路径、失败路径、超时、限流、降级路径测试通过。

### P7 集成与运维域

目标：

- 迁移西电教务集成、文件上传、对象存储、系统设置、安全日志。
- 迁移请求超时、安全头、限流、CORS、GZip、指标和日志脱敏。
- 对齐 Docker、Nginx、生产环境配置。

验收标准：

- `/xidian/*`、`/upload/*`、管理员设置和安全日志契约测试通过。
- Prometheus 指标可采集。
- 日志不泄露 token、密码、Cookie、API key。
- 生产配置校验具备等价能力。

### P8 双跑与契约验证

目标：

- 对 Python 和 Go 后端执行同一批契约测试。
- 对关键读接口做响应快照比较。
- 对关键写接口在隔离数据库中做状态变化比较。
- 记录性能、内存和错误率基线。

验收标准：

- P0 冻结的高优先级接口全部通过。
- 已知差异有迁移说明和前端影响评估。
- 回滚流程演练完成。

### P9 流量切换与 Python 下线

目标：

- 调整部署入口，将生产流量切到 Go 服务。
- 保留 Python 服务回滚窗口。
- 清理不再使用的 Python 运行时、依赖、镜像和启动脚本。
- 更新 README、部署文档和开发文档。

验收标准：

- 生产健康检查、业务 smoke test、指标和日志正常。
- 回滚手册验证过。
- Python 后端下线范围明确。
- 文档完成最终更新。

---

## 7. API 迁移清单

| 优先级 | 模块 | 状态 | 备注 |
|--------|------|------|------|
| P0 | `/health`、`/metrics` | DONE | Go P1 骨架已承接 `/health`、`/health/detailed`、`/metrics`；2026-06-01 起 `/health/detailed` 和 `/metrics` 默认限制在 `MANAGEMENT_ALLOWED_CIDRS` 内访问 |
| P1 | `/auth` | DONE | Go P3 已承接登录、注册、刷新、登出、当前用户、修改密码、注册状态、忘记密码公开申请/状态查询 |
| P1 | `/admin/users` | DONE | Go P3 追加承接管理员用户统计、列表、创建、更新、状态切换、删除、CSV 导入导出 |
| P1 | `/admin/settings` | DONE | Go P7 已承接注册开关、通用信息、可导出表、数据库 JSON 导入导出和数据库监控 |
| P2 | `/session` | DONE | Go P4 已承接会话创建、历史、列表、结束、模式、删除、批删、任务取消和 SSE 响应；P6 已接入可配置的 Eino Tutor Agent 第一片能力，token 级流式和资源推荐仍待后续 slice |
| P2 | `/exercise` | DONE | Go P4 已承接下一题、提交答案、题目详情、题目解析；P6 已接入可配置 Eino Math Solver 和 Diagnostician Agent，本地比较/诊断保留为降级；OCR 留到 P6 后续 |
| P2 | `/mistakes` | DONE | Go P4 已承接列表、统计、详情、标记掌握、删除和复习推荐 |
| P2 | `/progress` | DONE | Go P4 首轮已承接 overview、mastery、statistics、path、knowledge-graph、class-ranking、chapters |
| P2 | `/portrait` | DONE | Go P4 已承接读取、清除和模板画像生成；P6 已接入可配置的 Eino Portrait Agent，模型不可用时保留模板画像降级 |
| P3 | `/questions` | DONE | Go P5 已承接题目 CRUD、列表/分组/统计、批量发布/删除/复制、批量导入；P6 已接入可配置 Eino Question Parser Agent，确定性解析保留为降级；`/generate-isomorphic` 已有本地 solver 校验模板，通用求解仍留到 P6 |
| P3 | `/resources` | DONE | Go P5 已承接资源列表、详情、创建、更新、软删除、统计、收藏列表和收藏切换；资源文件上传已由 P7 `/upload` 承接 |
| P3 | `/classes` | DONE | Go P5 已承接教师创建/列表/详情/移除学生/解散班级，以及学生查询、加入、退出、当前班级 |
| P3 | `/teacher` | DONE | Go P5 已承接教师工作台统计、学生管理统计、教师数据分析、班级分析和教师视角学生详情 |
| P3 | `/admin/knowledge` | DONE | Go P5 已承接知识节点/关系 CRUD、分页筛选、章节、简要节点列表和统计 |
| P4 | `/admin/ai-config` | DONE | AI 配置；Go 已承接 provider/model/Agent 配置、provider test/fetch、模型列表替换和运行时 Tutor/Portrait/Diagnostician/Math Solver/Question Parser 配置选择；后续 OCR 与通用求解仍留在 P6 |
| P5 | `/xidian` | DONE | Go P7 已承接绑定状态、验证码挑战、绑定完成、解绑、课表/考试/成绩同步和快照读取；外部门户 live 验证留到有西电凭证的集成环境 |
| P5 | `/upload` | DONE | Go P7 已承接图片上传、教师资源文件上传、本地 `/uploads` 文件落盘、S3 兼容对象存储和七牛云对象存储适配；2026-06-01 起图片上传要求登录、增加用户/IP 速率限制、按真实图片内容校验，本地 `/uploads` 禁止目录索引 |
| P5 | `/admin/security-logs` | DONE | Go P7 已承接列表筛选/分页/日期分组、统计、删除、JSON/CSV 导出、归档、每日报告、清理和容量查询 |
| P5 | `/admin/inbox` | DONE | Go P7 已承接密码重置申请列表、待处理计数和审批通过/拒绝；审批通过会重置用户密码并清理登录失败计数 |
| P5 | `/admin/stats` | DONE | Go P7 已承接总览、用户增长、最近活动和系统状态 |

---

## 8. 数据迁移原则

1. 先读兼容，后写兼容。
2. Go 代码必须尊重现有表名、枚举值、索引和外键语义。
3. 数据迁移必须有备份、回滚和验证 SQL。
4. 不允许无记录地重命名字段或改变字段含义。
5. 大表索引、NOT NULL 约束、枚举变更必须拆成可回滚步骤。
6. 对于历史 Alembic 迁移，先冻结为基线，再决定是否用 Go 迁移工具重建后续迁移。

---

## 9. 测试与验收策略

### 9.1 测试层级

| 层级 | 目标 |
|------|------|
| 单元测试 | 覆盖领域规则、错误分支、输入校验 |
| Repository 集成测试 | 验证 SQL、事务、约束、回滚 |
| API 契约测试 | 验证请求、响应、状态码、错误码 |
| 双跑测试 | 同输入比较 Python 与 Go 输出 |
| 端到端测试 | 覆盖学生、教师、管理员核心流程 |
| 性能测试 | 对比延迟、吞吐、内存、连接池行为 |

### 9.2 最低验收门槛

- 新增 Go 公共函数必须有测试。
- 每个迁移阶段至少包含对应 API 契约测试。
- 核心业务模块覆盖正常路径、权限失败、输入错误和依赖失败。
- 外部依赖必须 mock 或使用隔离测试环境。
- 覆盖率目标沿用项目要求：核心逻辑 80%+。

### 9.3 建议验证命令占位

具体命令在 P1 技术选型后补齐：

```powershell
# Go 单元测试
go test ./...

# Go 格式化检查
gofmt -w .

# Go 静态检查
go vet ./...

# Go 数据迁移
go run ./cmd/migrate

# PostgreSQL Repository/迁移集成测试（需要测试库）
$env:MSP_GO_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/math_platform_test"
go test ./internal/platform/migration -run TestPostgresStoreIntegration

# Python 基线测试
pytest

# API 契约测试
# TODO: P0/P1 后补齐
```

---

## 10. 风险清单

| 编号 | 风险 | 影响 | 当前状态 | 缓解方案 |
|------|------|------|----------|----------|
| R1 | API 文档与实际 FastAPI 路由存在差异 | 前端兼容失败 | OPEN | P0 导出实际 OpenAPI 并冻结真实契约 |
| R2 | SQLAlchemy 模型语义迁移到 Go 后丢失隐含默认值 | 数据不一致 | OPEN | Repository 测试覆盖默认值、枚举、级联行为 |
| R3 | LangGraph/SymPy 能力在 Go 中无直接等价实现 | AI 功能延期 | ACCEPTED | 本轮明确排除旧 Python AI/Agent/OCR/LLM 工作流；P6 改走 Eino 新架构，当前 Tutor/Portrait/Diagnostician/Math Solver/Question Parser Agent 首片已落地，OCR 和通用求解仍需后续 slice |
| R4 | JWT/Cookie 兼容不完整 | 用户登录失效 | MITIGATED | P3 已实现 Python 兼容 JWT claims、HMAC 算法校验、refresh token HttpOnly Cookie 行为和轮换测试；后续 P8 继续做 Python/Go 双跑契约验证 |
| R5 | Alembic 历史与 Go 迁移工具并存 | 迁移历史混乱 | MITIGATED | P2 已生成 Go 单步初始 schema；后续生产迁移由 Go runner 负责；legacy Alembic 源码已随 `backend/` 清理，需从 git 历史查看 |
| R6 | 上传文件路径或对象存储 key 改变 | 历史资源不可访问 | MITIGATED | P7 `/upload` 已保持 `images/`、`videos/`、`documents/` key 规则和本地 `/uploads/{key}` 访问路径，并补充本地/S3/七牛适配测试；P8 继续做历史文件访问回归 |
| R7 | 缺少 git 元数据时无法做变更边界检查 | 并行任务冲突风险增加 | OPEN | 修改前后记录文件清单，避免触碰无关文件 |
| R8 | 默认入口已切到 Go，但 TODO 或未知 `/api/v1/*` 接口会收到占位响应 | AI 配置或非基线接口不可用 | MITIGATED | 非 AI Python v1 路由已按清单迁移；`/admin/ai-config` 已承接 provider/model/Agent 配置；`/session/{id}/chat` 和 `/portrait/generate` 可读取持久化 Agent 配置或 `EINO_*` 兼容配置调用 Eino Agent；未迁移接口禁止静默回落 Python |
| R10 | 前端 AI 模型设置页功能完整度高于 Go 后端能力 | 管理员以为可配置 LLM/Agent，但请求返回 501 | MITIGATED | P6 已实现 provider/model/Agent 配置 CRUD、provider test/fetch、凭据 Fernet 加密存储和 Tutor/Portrait/Diagnostician/Math Solver/Question Parser 运行时配置注入；剩余风险转为 OCR 与通用求解未完成 |
| R9 | 当前机器未配置可连接 PostgreSQL 测试库且 Docker CLI 不可用 | P2 数据库迁移/Repository 集成验收不能在本机闭环 | CLOSED | 已使用本地 PostgreSQL `math_platform` 执行清库、Go 迁移、重复迁移和迁移集成测试；Docker CLI 仍不可用但不阻塞 P2 |

---

## 11. 架构决策记录

| ADR | 日期 | 决策 | 状态 | 说明 |
|-----|------|------|------|------|
| ADR-001 | 2026-04-18 | Go HTTP router 选择 `net/http` `ServeMux` | DONE | P1 使用标准库，避免框架迁移期额外抽象 |
| ADR-002 | 2026-04-18 | Go 数据访问方式选择 `pgx/v5` + `go-redis/v9` | DONE | P1 先建立连接和健康检查；Repository 在 P2 补齐 |
| ADR-003 | 2026-04-18 | 从 Python Alembic head 生成 Go 单步初始 schema，后续由 Go forward migration runner 承接 | DONE | `0001_initial_schema.up.sql` 由临时库执行 Alembic 至 `0019_performance_indexes_phase3` 后 `pg_dump` 生成；包含表、枚举、索引、外键和种子数据；`go_schema_migrations` 记录 Go 迁移状态 |
| ADR-004 | 2026-05-08 | AI/Agent uses a new Eino architecture instead of bridging legacy Python LangGraph/LiteLLM workflows | IN_PROGRESS | First slices wire Session Tutor Agent, Portrait Agent, Diagnostician Agent, Math Solver Agent, Question Parser Agent, admin provider/model/Agent config, encrypted provider keys, provider test/fetch and runtime selection; LLM/Agent is not complete because OCR and broader math tools remain pending |
| ADR-005 | 2026-04-18 | 默认启动和 Nginx 分流先切到 Go | DONE | Python 代码保留为迁移参考，不再由默认启动、compose、Nginx split routing 承流 |
| ADR-006 | 2026-04-18 | P3 JWT 兼容层使用标准库 HMAC 实现，不引入额外 JWT 框架 | DONE | Go 侧显式验证 `alg`、`iss`、`aud`、`exp`、`iat`、`jti` 和 `type`，生成与 Python PyJWT 兼容的 HS256/HS384/HS512 token；密码哈希使用 `bcrypt` 保持兼容 |

---

## 12. 阶段完成记录

### 12.1 P0 基线冻结

- 状态：TODO
- 开始日期：TODO
- 完成日期：TODO
- 负责人：TODO
- 验证命令：TODO
- 验证结果：TODO
- 交付物链接：TODO
- 遗留风险：TODO

### 12.2 P1 Go 技术选型与骨架

- 状态：DONE
- 开始日期：2026-04-18
- 完成日期：2026-04-18
- 负责人：Codex
- 验证命令：`go test ./...`、`go vet ./...`、`go run ./cmd/api` 后访问 `/health`、`/metrics`、`/api/v1/auth/login`
- 验证结果：Go 单元测试和 vet 通过；本地 Go 服务可启动；`/health` 返回 healthy；`/metrics` 返回 Prometheus text；未迁移 `/api/v1/*` 返回 501 `NOT_IMPLEMENTED`
- 交付物链接：`backend-go/`、`backend-go/Dockerfile`、`start.bat`、`docker-compose.yml`、`.env.example`、`nginx-site.conf`
- 遗留风险：Docker CLI 在当前机器不可用，Docker 镜像构建未本地验证；业务 API 尚未迁移，默认 Go 入口会对未迁移接口返回 501

### 12.3 P2 数据访问与迁移体系

- 状态：DONE
- 开始日期：2026-04-18
- 完成日期：2026-04-18
- 负责人：Codex
- 验证命令：`alembic upgrade head` 临时库、`pg_dump` 生成 Go 初始 schema、`DROP SCHEMA public CASCADE` 清空 `math_platform`、`go run ./cmd/migrate`、重复 `go run ./cmd/migrate`、`MSP_GO_TEST_DATABASE_URL=postgres://.../math_platform go test ./internal/platform/migration -run TestPostgresStoreIntegration -count=1 -v`、`go test ./...`、`go vet ./...`
- 验证结果：本地 `math_platform` 已清空旧结构和数据并由 Go 单步迁移重建；首次迁移 applied_count=1；重复迁移 applied_count=0；迁移后 30 张 public 表，其中 `go_schema_migrations=1`、`alembic_version=0019_performance_indexes_phase3`、`knowledge_nodes=8`、`knowledge_relations=7`、`system_settings=2`；迁移集成测试通过；Go 单元测试和 vet 通过
- 交付物链接：`backend-go/internal/platform/postgres/`、`backend-go/internal/platform/redis/`、`backend-go/internal/platform/migration/`、`backend-go/internal/adapter/postgres/`、`backend-go/cmd/migrate/`、`backend-go/migrations/`
- 遗留风险：Docker CLI 仍不可用，Docker 镜像构建未本地验证；业务 API 仍未迁移，P3 需要基于新 Repository/事务模式迁移 `/auth` 和用户域

### 12.4 P3 鉴权与用户域

- 状态：DONE
- 开始日期：2026-04-18
- 完成日期：2026-04-18
- 负责人：Codex
- 验证命令：`gofmt -w ...`、`go test ./... -count=1`、`go vet ./...`、`MSP_GO_TEST_DATABASE_URL=postgres://.../math_platform?sslmode=disable go test ./internal/adapter/postgres -run TestUserRepositoryIntegration -count=1 -v`、`go test ./internal/application/adminuser ./internal/adapter/http/adminuser ./internal/adapter/postgres`。2026-05-07 安全加固追加验证：`go test ./internal/application/auth ./internal/adapter/http/auth ./internal/application/adminuser ./internal/application/admininbox ./internal/platform/config -count=1`、`npm test -- --run src/libs/validation/__tests__/schemas.test.ts`、`go test ./... -count=1`、`go vet ./...`、`npm run build`
- 验证结果：Go 全量单元/契约测试通过；Go vet 通过；PostgreSQL 用户仓储集成测试在事务内通过并回滚；覆盖 JWT claims、bcrypt 密码、注册开关、登录失败锁定、refresh cookie 设置/清理、用户角色判断、用户仓储枚举映射和密码重置公开申请/状态查询。2026-05-01 追加覆盖 `/admin/users` 管理员鉴权、账户统计、用户分页筛选、创建/重复校验、更新密码和显示名、状态切换、物理删除关联清理、CSV UTF-8/GBK 导入解析和 CSV 导出。2026-05-07 追加认证安全加固：refresh token 由服务端 Redis/本地降级会话存储登记并一次性消费轮换，登出撤销 refresh session；生产/非开发环境拒绝占位 JWT secret、短 JWT secret、默认/弱管理员密码；注册、改密、管理员创建/重置/导入用户统一强密码策略并拒绝 bcrypt 72 字节截断范围；密码重置临时密码保证包含大小写字母、数字和特殊字符；前端 access token 从长期 `localStorage` 改为标签页会话级存储并可用 HttpOnly refresh cookie 静默恢复；`/auth/refresh` 和 `/auth/logout` 增加双提交 CSRF cookie/header 校验并在登录、注册、刷新时轮换 CSRF token；生产/非开发环境启动时要求 Redis ping 通过，refresh session store 严格使用 Redis，不再静默本地降级。2026-05-07 定向 Go 测试、前端校验测试、`go vet ./...` 和 `npm run build` 通过；`go test ./... -count=1` 仅 `tests/contract/TestLegacyPythonBackendDirectoryIsAbsent` 失败，原因是当前工作区存在 legacy `backend/` 目录，非本次认证改动导致。
- 交付物链接：`backend-go/internal/domain/user/`、`backend-go/internal/application/auth/`、`backend-go/internal/application/adminuser/`、`backend-go/internal/application/admininbox/`、`backend-go/internal/adapter/postgres/user_repository.go`、`backend-go/internal/adapter/postgres/admin_user_repository.go`、`backend-go/internal/adapter/http/auth/`、`backend-go/internal/adapter/http/adminuser/`、`backend-go/cmd/api/main.go`、`backend-go/internal/platform/config/`、`frontend/src/libs/auth/tokenStorage.ts`、`frontend/src/modules/auth/`、`frontend/src/libs/http/`
- 遗留风险：邮箱绑定/验证码接口已确认为废弃功能，不再纳入 Go 迁移欠账；尚未实现 Casdoor/OIDC 风格的授权码 + PKCE 外部身份提供方、TOTP/WebAuthn MFA、用户自助会话设备管理和细粒度策略引擎；P8 仍需执行 Python/Go 双跑契约验证

### 12.5 P4 核心学习域

- 状态：IN_PROGRESS
- 开始日期：2026-04-25
- 完成日期：TODO
- 负责人：Codex
- 验证命令（阶段进行中）：`gofmt -w ...`、`go test ./... -count=1`、`go vet ./...`
- 验证结果（阶段进行中）：Go 全量单元/契约测试通过；Go vet 通过；覆盖 `/progress` 鉴权、overview、mastery、statistics、path、knowledge-graph、class-ranking、chapters 的应用层和 HTTP 层主要路径；覆盖 `/portrait` 鉴权、读取、清除和模板画像生成的应用层与 HTTP 层主要路径；覆盖 `/mistakes` 鉴权、列表筛选/排序/分页、统计、详情、标记掌握、删除和复习推荐的应用层与 HTTP 层主要路径；覆盖 `/exercise` 鉴权、下一题选择、提交答案、DKT/profile 更新、题目详情和解析权限的应用层主要路径；覆盖 `/session` 鉴权、创建、历史、列表、结束、模式、删除、批删、任务取消和 SSE 形状兼容降级的应用层与 HTTP 层主要路径。2026-06-01 本轮 `go test ./... -count=1` 通过。
- 交付物链接：`backend-go/internal/application/progress/`、`backend-go/internal/adapter/http/progress/`、`backend-go/internal/adapter/postgres/progress_repository.go`、`backend-go/internal/application/portrait/`、`backend-go/internal/adapter/http/portrait/`、`backend-go/internal/adapter/postgres/portrait_repository.go`、`backend-go/internal/application/mistake/`、`backend-go/internal/adapter/http/mistake/`、`backend-go/internal/adapter/postgres/mistake_repository.go`、`backend-go/internal/application/exercise/`、`backend-go/internal/adapter/http/exercise/`、`backend-go/internal/adapter/postgres/exercise_repository.go`、`backend-go/internal/application/session/`、`backend-go/internal/adapter/http/session/`、`backend-go/internal/adapter/postgres/session_repository.go`、`backend-go/cmd/api/main.go`、`backend-go/migrations/0002_replace_bkt_with_dkt.up.sql`（进行中）
- Residual risks: /session/{id}/chat now has an Eino-first Tutor Agent path when EINO_* is configured and keeps an explicit fallback when not configured; resource recommendation fallback, portrait updates, LLM portrait quality, OCR, math equivalence and LLM diagnosis still need follow-up Eino slices. Repository integration tests and runtime user acceptance remain separate validation work.

### 12.6 P5 内容与教学管理域

- 状态：DONE
- 开始日期：2026-04-26
- 完成日期：2026-04-27
- 负责人：Codex
- 验证命令：`gofmt -w ...`、`go test ./internal/application/teacher ./internal/application/knowledge ./internal/adapter/http/teacher ./internal/adapter/http/knowledge ./internal/adapter/postgres`、`go vet ./...`、`go test ./...`、`go test ./internal/adapter/postgres -run TestClassRepositoryIntegration -count=1 -v`
- 验证结果：Go vet 通过；P5 新增和相关 PostgreSQL adapter 定向测试通过；覆盖 `/resources` 鉴权、列表筛选/分页、详情、教师权限校验、创建默认值、统计/收藏字面量路由、404 映射和软删除响应；PostgreSQL adapter 编译通过并实现 `contents`、`content_assets`、`user_favorites` 的资源中心读写语义。覆盖 `/classes` 鉴权、教师创建/列表/详情/移除学生/解散班级、学生 lookup/join/leave/me、班级号规范化、角色限制、404/409/422 映射；PostgreSQL adapter 编译通过并实现 `classes`、`class_enrollments`、`users` 的班级管理读写语义，解散班级时显式先删成员再删班级以保持外键兼容；`TestClassRepositoryIntegration` 已补充真实 PostgreSQL 验证入口，本轮未设置 `MSP_GO_TEST_DATABASE_URL` 时按预期跳过。覆盖 `/questions` 鉴权、列表筛选/分页/排序、分组、统计、详情、创建、更新、软删除、批量发布/删除/复制、批量导入和 `/ai-parse` 形状兼容占位；PostgreSQL adapter 编译通过并实现 `contents`、`content_attempts`、`knowledge_nodes`、`content_audit`、`outbox_events` 的题库读写语义。覆盖 `/teacher` 教师工作台统计、学生统计、数据分析、班级分析、学生详情的应用层聚合和 HTTP 鉴权/参数/路径转发；PostgreSQL adapter 编译通过并实现 `classes`、`class_enrollments`、`content_attempts`、`learning_sessions`、`student_profiles`、`diagnosis_reports`、`knowledge_nodes` 读模型聚合。覆盖 `/admin/knowledge` 管理员鉴权、节点分页筛选/详情/创建/更新/删除、关系列表/创建/更新/删除、章节/统计/简要节点列表；PostgreSQL adapter 编译通过并实现 `knowledge_nodes`、`knowledge_relations` CRUD 与删除节点前清理关系。2026-04-27 本轮 `go test ./...` 除 `internal/platform/redis` 既有 miniredis socket 初始化失败外，其余包通过。
- 交付物链接：`backend-go/internal/application/resource/`、`backend-go/internal/adapter/http/resource/`、`backend-go/internal/adapter/postgres/resource_repository.go`、`backend-go/internal/application/classroom/`、`backend-go/internal/adapter/http/classroom/`、`backend-go/internal/adapter/postgres/class_repository.go`、`backend-go/internal/application/question/`、`backend-go/internal/adapter/http/question/`、`backend-go/internal/adapter/postgres/question_repository.go`、`backend-go/internal/application/teacher/`、`backend-go/internal/adapter/http/teacher/`、`backend-go/internal/adapter/postgres/teacher_repository.go`、`backend-go/internal/application/knowledge/`、`backend-go/internal/adapter/http/knowledge/`、`backend-go/internal/adapter/postgres/knowledge_repository.go`、`backend-go/cmd/api/main.go`
- 遗留风险：`/questions/ai-parse` 当前是非 LLM 形状兼容占位，AI 识别质量等价留到 P6；`/resources`、`/questions`、`/teacher`、`/admin/knowledge` 仍需补充真实 PostgreSQL Repository 集成测试；`/classes` 仓储集成测试入口已补充但本轮未连接真实 PostgreSQL 测试库执行；上述模块仍需在 P8 做 Python/Go 双跑契约验证；资源文件上传和对象存储能力不属于本切片，仍留到 P7 `/upload`；当前 Windows 环境下 `internal/platform/redis` 的 miniredis 测试因 socket 初始化失败阻塞全量测试绿灯。

### 12.7 P6 AI 与 Agent 能力

- Status: IN_PROGRESS
- Start date: 2026-05-08 (Go 501 placeholders were added on 2026-05-06; Eino framework integration started on 2026-05-08 under /goal)
- Completion date: TODO
- Owner: Codex
- Verification commands: gofmt targeted Go files; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/exercise ./internal/adapter/llm/einoagent ./cmd/api ./tests/contract -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/portrait ./internal/adapter/llm/einoagent ./cmd/api -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminaiconfig ./internal/adapter/http/adminaiconfig ./internal/adapter/llm/einoagent ./internal/application/session ./internal/application/portrait ./internal/adapter/postgres ./cmd/api ./tests/contract -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1; GOCACHE=E:\code\msp-go\.gocache go vet ./...; pnpm.cmd run build in frontend; earlier slice also ran go test ./internal/platform/config ./internal/application/session ./internal/adapter/llm/einoagent ./tests/contract -count=1; go mod tidy; go test ./cmd/api ./internal/... -count=1
- Verification results: Added EINO_* runtime config, Eino ADK Tutor Agent adapter, Session ChatAgent abstraction, /session/{id}/chat Eino-first path, provider/model/Agent config service, PostgreSQL adapter, admin HTTP routes, provider test/fetch, Fernet-protected provider API keys, Tutor runtime selection from persisted `tutor` Agent config, Portrait runtime selection from persisted `portrait` Agent config with template fallback, Diagnostician runtime selection from persisted `diagnostician` Agent config with local diagnosis fallback, Math Solver runtime selection from persisted `math_solver` Agent config with local answer-check fallback, and Question Parser runtime selection from persisted `question_parser` Agent config with deterministic parse fallback. Config, session, portrait, exercise, question, Eino adapter, admin AI config application/HTTP/Postgres, AI boundary contracts, cmd/api, targeted contract tests, full Go test suite, go vet, and frontend build passed. Current 2026-06-29 implementation confirms LLM/Agent management is no longer a 501 placeholder, while OCR and broader math tools remain incomplete.
- Deliverables: backend-go/internal/application/adminaiconfig/; backend-go/internal/adapter/http/adminaiconfig/; backend-go/internal/adapter/postgres/admin_ai_config_repository.go; backend-go/internal/adapter/llm/einoagent/; backend-go/tests/contract/ai_boundary_surface_test.go; backend-go/tests/contract/route_surface_test.go; backend-go/internal/application/question/service.go; backend-go/internal/application/session/service.go; backend-go/internal/application/portrait/service.go; backend-go/internal/application/exercise/service.go; backend-go/internal/platform/config/config.go; backend-go/cmd/api/main.go; .env.example
- Residual risks: token-level streaming, broader math solving, OCR and teaching feedback still need follow-up Eino slices; provider test/fetch uses OpenAI-compatible `/v1` endpoints and may need provider-specific adapters for non-compatible vendors. The Go backend must not silently fall back to legacy Python.
- Design precondition: AI workflows are rebuilt on Eino rather than ported from legacy Python; endpoint boundaries and acceptance evidence must be updated with every slice.

### 12.8 P7 集成与运维域

- 状态：IN_PROGRESS
- 开始日期：2026-05-01
- 完成日期：TODO
- 负责人：Codex
- 验证命令（阶段进行中）：`gofmt -w ...`、`go test ./internal/application/admininbox ./internal/adapter/http/admininbox ./internal/adapter/postgres -count=1`、`go test ./internal/application/adminstats ./internal/adapter/http/adminstats ./internal/application/adminsettings ./internal/adapter/http/adminsettings ./internal/application/securitylog ./internal/adapter/http/securitylog ./internal/adapter/postgres ./internal/platform/config -count=1`、`go test ./internal/application/upload ./internal/adapter/http/upload ./internal/adapter/storage ./internal/platform/config -count=1`、`go test ./internal/application/xidian ./internal/adapter/http/xidian ./internal/adapter/postgres ./internal/integration/xidian ./internal/platform/secret ./internal/platform/config -count=1`、`go test ./... -count=1`、`go vet ./...`
- 验证结果（阶段进行中）：Go 全量单元/契约测试通过；Go vet 通过；覆盖 `/admin/inbox` 管理员鉴权、密码重置申请列表分页/状态筛选、待处理计数、审批通过生成临时密码并更新用户密码哈希、审批拒绝记录原因、已处理/不存在/用户缺失等业务分支；已补充 PostgreSQL 集成测试入口覆盖列表、计数和事务审批路径，未设置 `MSP_GO_TEST_DATABASE_URL` 时按既有模式跳过。2026-05-03 追加覆盖 `/admin/stats` 管理员鉴权、概览统计、用户增长序列、最近活动和 PostgreSQL/Redis 状态聚合；覆盖 `/admin/settings` 注册开关、通用系统信息、系统设置 upsert、可导出表、数据库 JSON 导入导出、敏感字段排除、连接池/表统计监控；覆盖 `/admin/security-logs` 管理员鉴权、事件/级别/时间筛选、分页日期分组、统计、删除、JSON/CSV Base64 导出、归档、每日报告生成、批量清理和容量阈值检查。2026-05-03 本轮 `go test ./... -count=1` 和 `go vet ./...` 通过。2026-05-06 追加覆盖 `/upload/image` 公开图片上传、`/upload/resource` 教师/管理员权限、multipart 解析、415/413/500 错误映射、图片/视频/文档 MIME 白名单、10MB/500MB 大小限制、本地 `/uploads` 落盘、S3 path-style 签名上传/私有桶预签名 URL、七牛上传 token/私有下载 URL；追加覆盖 `/xidian/binding` 鉴权和状态、验证码挑战、绑定完成、解绑、同步、快照和 Python 兼容错误映射，以及 Fernet 兼容密码加密、西电 IDS 登录页/验证码解析、登录表单提交和快照降级路径；本轮 `go test ./internal/application/xidian ./internal/adapter/http/xidian ./internal/adapter/postgres ./internal/integration/xidian ./internal/platform/secret ./internal/platform/config -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- 交付物链接：`backend-go/internal/application/admininbox/`、`backend-go/internal/adapter/http/admininbox/`、`backend-go/internal/adapter/postgres/password_reset_admin_repository.go`、`backend-go/internal/application/adminstats/`、`backend-go/internal/adapter/http/adminstats/`、`backend-go/internal/adapter/postgres/admin_stats_repository.go`、`backend-go/internal/application/adminsettings/`、`backend-go/internal/adapter/http/adminsettings/`、`backend-go/internal/adapter/postgres/admin_settings_repository.go`、`backend-go/internal/application/securitylog/`、`backend-go/internal/adapter/http/securitylog/`、`backend-go/internal/adapter/postgres/security_log_repository.go`、`backend-go/internal/application/upload/`、`backend-go/internal/adapter/http/upload/`、`backend-go/internal/adapter/storage/`、`backend-go/internal/application/xidian/`、`backend-go/internal/adapter/http/xidian/`、`backend-go/internal/adapter/postgres/xidian_repository.go`、`backend-go/internal/integration/xidian/`、`backend-go/internal/platform/secret/`、`backend-go/cmd/api/main.go`、`backend-go/internal/platform/config/`
- 遗留风险：P7 仍有更完整的生产运维检查未收敛；`/admin/settings` 数据库导入导出、`/admin/security-logs` 清理归档、对象存储真实云端写入和西电门户 live 同步仍需在可用外部环境中补充集成测试；上述接口仍需在 P8 做 Python/Go 双跑契约验证。

### 12.9 P8 静态契约验证与用户验收交接

- 状态：DONE
- 开始日期：2026-05-06
- 完成日期：2026-05-07
- 负责人：Codex
- 验证命令：`gofmt -w backend-go/tests/contract/response_shape_surface_test.go`、`gofmt -w backend-go/tests/contract/request_shape_surface_test.go`、`gofmt -w backend-go/tests/contract/error_body_surface_test.go`、`gofmt -w backend-go/internal/application/mistake/service.go backend-go/internal/application/mistake/service_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go backend-go/tests/contract/response_shape_surface_test.go`、`go test ./internal/application/mistake ./internal/adapter/http/mistake ./tests/contract -count=1`、`go test ./tests/contract -count=1`、`go test ./... -count=1`、`go vet ./...`、`git diff --check -- ...`、`npm install`、`npm run build`
- 验证结果：新增 route-surface contract test，静态解析 legacy FastAPI `@router.*` 装饰器和 Go `mux.HandleFunc` 注册，逐模块比较非 AI `/api/v1` 路由；当时 `/admin/ai-config` 作为 AI 范围跳过等价实现检查，并要求 Go 存在精确路径和子路径 TODO placeholder（2026-06-29 已替换为真实配置路由）。追加 success-status、explicit error-status、error-body、frontend route audit、response-shape 和 request-shape contract tests；覆盖 Go 路由注册、前端 API 调用、稳定错误字段、非 AI 顶层请求/响应字段和 AI TODO 边界。2026-05-07 用户确认运行时双跑和比对由用户自行测试，P8 不再阻塞 Python 后端删除。
- 交付物链接：`backend-go/tests/contract/route_surface_test.go`、`backend-go/tests/contract/error_body_surface_test.go`、`backend-go/tests/contract/frontend_route_surface_test.go`、`docs/backend-go-migration-completion-audit.md`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/mistake/service_test.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/mistake/handler_test.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`
- 遗留风险：当前 P8 保留 Go 路由注册、前端 API 覆盖、错误响应字段和 AI TODO 边界等静态守卫。2026-05-07 用户明确确认“不用双跑，不用比对，用户自行测试”，因此真实运行时 smoke、Docker/Compose 实机验证、业务流程回归、性能基线、动态响应细节和外部服务 live 集成不再阻塞 Python 下线，由用户验收承担。前端显式分类清单中的邮箱验证、作业、旧题库导入导出模板和 browser remote log endpoint 不是 legacy Python v1 可迁移接口，仍需产品侧决定删除、隐藏、补新 Go 功能或保持降级。

### 12.10 P9 流量切换与 Python 下线

- 状态：DONE
- 开始日期：2026-05-06
- 完成日期：2026-05-07
- 负责人：Codex
- 验证命令：`rg -n "backend/|backend-go|uvicorn|FastAPI|Python|python|pyproject|poetry|alembic|go run|cmd/api|cmd/migrate|docker compose|Docker" README.md docs scripts start.bat docker-compose.yml nginx-site.conf frontend/nginx.conf .env.example -S`、`bash -n scripts/deploy.sh scripts/update.sh`、`gofmt -w backend-go/tests/contract/runtime_entry_surface_test.go`、`go test ./tests/contract -count=1`、`go test ./... -count=1`、`go vet ./...`、`docker --version`、`rm -rf backend`、`test -e backend; echo $?`
- 验证结果：默认启动与部署入口已核查：`start.bat`、`docker-compose.yml`、`frontend/nginx.conf`、`nginx-site.conf`、`scripts/deploy.sh` 和 `scripts/update.sh` 均指向 Go backend；runtime-entry contract test 静态保护默认本地启动、Compose、Go Dockerfile、部署/更新脚本和 Nginx 入口，要求保留 Go backend/`msp-migrate` 信号并禁止重新引入 `uvicorn`、`alembic upgrade` 或 Python backend Dockerfile 运行入口；Go 后端镜像新增 `msp-migrate`，生产部署和更新脚本在启动应用容器前运行 Go migration runner，不再提示手动执行 legacy Alembic；2026-05-07 用户明确确认“不用双跑，不用比对，用户自行测试”，并确认删除 `backend/` 时包含 ignored 的 `.venv`、缓存和旧 `uploads`；已执行 `rm -rf backend`，`test -e backend; echo $?` 返回 `1`，确认目录不存在。本轮当前 shell 无 `go` 命令，无法重新运行 Go 测试；此前记录的 Go 测试和 vet 通过结果保留为静态证据。
- 交付物链接：`README.md`、`backend-go/Dockerfile`、`backend-go/tests/contract/runtime_entry_surface_test.go`、`docs/backend-go-migration-completion-audit.md`、`scripts/deploy.sh`、`scripts/update.sh`、`start.bat`、`docker-compose.yml`、`frontend/nginx.conf`、`nginx-site.conf`、本文 [Python 下线记录](#14-python-下线记录p9)
- 遗留风险：`backend/` 已删除，无法再从当前工作区进行 Python/Go 双跑或读取 legacy Python 源码对照；真实 Docker/Compose 环境演练、浏览器/API flow smoke、性能基线和业务验收由用户自行执行。

---

## 13. 后续待补充

- P0 后补充实际 OpenAPI 导出路径和契约测试入口。
- P1 后补充 Go 技术栈版本和标准命令。
- P2 已补充数据库迁移工具、目录和回滚命令；后续变更按 Go forward migration 追加。
- P6 后补充 AI/Agent 详细设计。
- 运行时 smoke、Docker/Compose 实机烟测和业务流程测试由用户自行执行并记录。

---

## 14. Python 下线记录（P9）

### 14.1 当前下线范围

- 默认本地启动：`start.bat` 启动 `backend-go/cmd/api`，不启动 `backend/app/main.py`。
- 默认容器部署：`docker-compose.yml` 的 `backend` 服务构建和运行 `backend-go/Dockerfile`。
- 默认前端代理：`frontend/vite.config.ts`、`frontend/nginx.conf` 和根目录 `nginx-site.conf` 均代理到 Go API 端口 `8000`。
- 默认部署脚本：`scripts/deploy.sh`、`scripts/update.sh` 启动 Go backend，并通过 `msp-migrate` 执行 Go migration runner。
- Legacy Python：`backend/` 已按用户确认删除，包括 ignored 的 `.venv`、缓存和旧 `uploads`。

### 14.2 用户验收范围

用户明确表示“不用双跑，不用比对，我自己会测试”，因此以下运行时验收不再阻塞 `backend/` 清理：

1. Go 后端全量测试、vet 和集成测试。
2. Docker/Compose 镜像构建、`msp-migrate` 实机烟测和迁移幂等验证。
3. 浏览器/API flow smoke、核心业务读写流程和性能基线。
4. 外部对象存储、西电门户 live 同步等外部服务集成验证。

### 14.3 回滚策略

- 数据库回滚以备份恢复或补偿型 Go forward migration 为准，不使用 Alembic downgrade 作为默认生产回滚路径；旧 Alembic 源码已不在当前工作区。
- 应用回滚以镜像版本回滚为准：保留上一版 `backend-go`、`frontend` 镜像标签和 `.env` 备份。
- 如果 Go migration 已应用不可逆数据变更，先评估补偿迁移或恢复备份，再回滚应用镜像。
- 当前默认部署链路不提供自动切回 Python backend 的脚本；如需查看旧 Python 实现，需从 git 历史或外部归档恢复。

### 14.4 删除确认

- 2026-05-07 用户确认删除 `backend/`，并确认连 ignored 的 `.venv`、缓存和旧 `uploads` 一起删除。
- 已执行 `rm -rf backend`。
- `test -e backend; echo $?` 返回 `1`，确认目录不存在。

---

## 15. 更新记录

### 2026-04-18

- 创建后端 Python 到 Go 重构迁移主文档。
- 建立阶段总览、完成标记规则、风险清单和阶段完成记录模板。
- 完成 P1 首轮 Go API 骨架，默认启动、Docker 后端服务和 Nginx split routing 切到 Go；Python 后端保留为迁移参考，不再默认承流。
- 清理废弃配置入口：删除 legacy Python `backend/Dockerfile` 和旧 split routing 配置，统一为根目录 `.env`/`.env.example` 与 `docker-compose.yml`。
- P2 数据访问与迁移体系开始：拆分 PostgreSQL/Redis 基础设施、事务模式、Repository 基础和 Go 迁移衔接策略。
- P2 完成：新增 pgx 连接池配置、事务封装、Repository 查询基类、Redis cache/LRU 降级、Go forward migration runner 和 `cmd/migrate`；从 Python Alembic head 生成 `0001_initial_schema.up.sql`；本地 `math_platform` 已清库并由 Go 迁移重建，重复迁移和迁移集成测试通过。
- P3 完成：新增 Go 用户领域模型、Auth service、bcrypt 密码策略、Python 兼容 HMAC JWT、refresh token Cookie 轮换、登录失败锁定、用户上下文/角色判断、PostgreSQL 用户仓储、`/api/v1/auth/*` 核心路由和启动期管理员初始化。

### 2026-04-25

- P4 核心学习域开始：优先迁移 `/api/v1/progress/*` 学生端进度查询链路，避开 LLM 画像生成和流式会话等后续高耦合能力。
- P4 `/progress` 首轮完成：新增 Go progress application service、PostgreSQL read repository 和 HTTP handler，承接 overview、mastery、statistics、path、knowledge-graph、class-ranking、chapters；`go test ./... -count=1` 和 `go vet ./...` 通过。
- P4 `/portrait` 首轮完成：新增 Go portrait application service、PostgreSQL repository 和 HTTP handler，承接 GET `/portrait`、POST `/portrait/generate`、DELETE `/portrait`；生成入口先产出基于学习数据的模板画像，LLM 画像质量等价留到 P6；`go test ./... -count=1` 通过。
- P4 `/mistakes` 首轮完成：新增 Go mistake application service、PostgreSQL repository 和 HTTP handler，承接列表、统计、详情、标记掌握、删除和复习推荐；保持 Python 的掌握度过滤、错误类型统计和复习优先级语义；`go test ./... -count=1` 和 `go vet ./...` 通过。
- P4 `/exercise` 首轮完成：新增 Go exercise application service、PostgreSQL repository 和 HTTP handler，承接下一题、提交答案、题目详情和解析；提交答案在事务内写入 attempt、基础 diagnosis、learning session、BKT 状态和 student profile；AI OCR、数学等价判定和 LLM 诊断质量等价留到 P6；`go test ./... -count=1` 和 `go vet ./...` 通过。
- P4 `/session` 首轮完成：新增 Go session application service、PostgreSQL repository 和 HTTP handler，承接创建、历史、列表、结束、模式更新、删除、批删、任务取消和 `/chat` SSE 形状兼容降级；完整 Agent 流式工作流、资源推荐和画像写回质量等价留到 P6；`go test ./... -count=1` 和 `go vet ./...` 通过。

### 2026-04-26

- P5 内容与教学管理域开始：优先迁移不依赖 AI/Agent 的 `/api/v1/resources` 资源中心链路。
- P5 `/resources` 首轮完成：新增 Go resource application service、PostgreSQL repository 和 HTTP handler，承接列表、统计、收藏列表、详情、创建、更新、软删除和收藏切换；保持视频/文档到 `contents` 类型、附件到 `content_assets`、收藏到 `user_favorites` 的 Python 存储语义；`go test ./... -count=1` 和 `go vet ./...` 通过。

### 2026-04-27

- P5 `/classes` 首轮完成：新增 Go classroom application service、PostgreSQL repository 和 HTTP handler，承接教师创建班级、班级列表、班级详情、移除学生、解散班级，以及学生班级号查询、加入班级、退出班级、当前班级查询；保持 `classes`、`class_enrollments` 存储语义和教师/学生角色限制；`go test ./... -count=1` 和 `go vet ./...` 通过；`TestClassRepositoryIntegration` 已补充真实 PostgreSQL 验证入口，当前未设置 `MSP_GO_TEST_DATABASE_URL` 时按预期跳过。
- P5 `/questions` 首轮完成：新增 Go question application service、PostgreSQL repository 和 HTTP handler，承接题目 CRUD、列表、分组、统计、批量发布、批量删除、批量复制和批量导入；保持 `contents` 中 `PROBLEM` 类型、题型/答案信息写入 `meta`、使用统计来自 `content_attempts`、标题自动匹配 `knowledge_nodes` 的 Python 存储语义；`/questions/ai-parse` 先提供非 LLM 形状兼容占位，LLM 解析质量留到 P6；`go test ./... -count=1` 和 `go vet ./...` 通过。
- P5 `/teacher` 与 `/admin/knowledge` 首轮完成：新增 Go teacher/knowledge application service、PostgreSQL repository 和 HTTP handler，承接教师统计分析与知识图谱管理全路径；`go test ./internal/application/teacher ./internal/application/knowledge ./internal/adapter/http/teacher ./internal/adapter/http/knowledge ./internal/adapter/postgres` 和 `go vet ./...` 通过；本轮 `go test ./...` 除既有 `internal/platform/redis` miniredis socket 初始化失败外，其余包通过。

### 2026-05-01

- P4 `/admin/bkt` 首轮完成：新增 Go BKT 参数 application service、PostgreSQL repository 和 HTTP handler，承接参数分页列表、单项概率更新、默认重置和缺失知识点参数种子化；保持 Python 的默认参数 `p_l0=0.25`、`p_t=0.12`、`p_g=0.20`、`p_s=0.10` 及概率校验边界；`go test ./... -count=1` 和 `go vet ./...` 通过。
- P3 `/admin/users` 追加完成：新增 Go admin user application service、PostgreSQL repository 方法和 HTTP handler，承接账户统计、用户分页筛选、创建、更新、状态切换、物理删除、CSV 导入导出；删除用户时显式清理学习会话、画像、班级、内容、导入任务和西电账号/快照等非级联依赖；本轮定向 `go test ./internal/application/adminuser ./internal/adapter/http/adminuser ./internal/adapter/postgres` 通过。
- P7 `/admin/inbox` 首轮完成：新增 Go admin inbox application service、PostgreSQL repository 方法和 HTTP handler，承接密码重置申请列表、待处理计数、审批通过/拒绝；审批通过在事务内更新用户密码哈希并清理登录失败计数；补充可选 PostgreSQL 集成测试入口；本轮 `go test ./... -count=1` 和 `go vet ./...` 通过。

### 2026-05-03

- P7 `/admin/stats` 首轮完成：新增 Go admin stats application service、PostgreSQL read repository 和 HTTP handler，承接概览统计、用户增长、最近活动和系统状态；系统状态聚合 PostgreSQL/Redis ping 与延迟。
- P7 `/admin/settings` 首轮完成：新增 Go admin settings application service、PostgreSQL repository 和 HTTP handler，承接注册开关、通用系统信息、数据库可导出表、JSON Base64 导出、JSON 导入和数据库监控；导出时排除密码、加密密码和会话 Cookie 等敏感字段。
- P7 `/admin/security-logs` 首轮完成：新增 Go security log application service、PostgreSQL repository 和 HTTP handler，承接日志列表筛选/分页/日期分组、统计、删除、JSON/CSV 导出、归档、每日报告、自动清理和容量查询；本轮 `go test ./... -count=1` 和 `go vet ./...` 通过。

### 2026-05-06

- P7 `/upload` 首轮完成：新增 Go upload application service、HTTP handler 和 storage adapter，承接公开图片上传、教师/管理员资源文件上传、本地 `/uploads` 文件落盘、S3 兼容对象存储和七牛云对象存储适配；保持 Python 的 MIME 白名单、10MB 图片限制、500MB 资源文件限制、`images/`、`videos/`、`documents/` key 规则和响应字段；本轮 `go test ./internal/application/upload ./internal/adapter/http/upload ./internal/adapter/storage ./internal/platform/config -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P7 `/xidian` 首轮完成：新增 Go xidian application service、PostgreSQL repository、HTTP handler、Fernet 兼容加密和 IDS/Ehall/Yjspt integration client，承接绑定状态、验证码挑战、绑定完成、解绑、课表/考试/成绩同步和快照读取；保留 Python 的快照降级、`CAPTCHA_REQUIRED`、`NO_SNAPSHOT` 等错误语义；本轮 `go test ./internal/application/xidian ./internal/adapter/http/xidian ./internal/adapter/postgres ./internal/integration/xidian ./internal/platform/secret ./internal/platform/config -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过；西电门户 live 同步仍需在有真实凭证和网络的集成环境验证。
- P6 范围调整与 `/admin/ai-config` 占位完成：本轮非 AI 迁移明确不移植旧 Python AI/Agent/OCR/LLM 工作流；当时新增 Go admin AI config placeholder handler 并接入 `cmd/api`，保留 `/api/v1/admin/ai-config/*` 管理员鉴权接口边界，管理员访问返回 501 `AI_CONFIG_TODO`；本轮 `go test ./internal/adapter/http/adminaiconfig -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。2026-06-29 该占位已被 provider/model/Agent 配置闭环替换。
- P8 双跑与契约验证开始：新增 `backend-go/tests/contract/route_surface_test.go`，静态比较 legacy FastAPI v1 route surface 与 Go `ServeMux` route surface；非 AI 路由表面等价通过，当时 `/admin/ai-config` 要求 Go 保留 TODO placeholder（2026-06-29 已更新为真实路由存在性检查）；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过；P8 仍需请求/响应字段、状态码、数据状态变化、性能和真实双跑验证。
- P8 前端 API 覆盖审计追加：新增 `backend-go/tests/contract/frontend_route_surface_test.go`，静态提取前端 `apiClient`、`axios`、`fetch`、`createSSEConnection` 和默认 remote log endpoint，要求所有前端 API 调用被 Go 路由覆盖或显式分类；当前分类清单记录邮箱验证、作业管理、旧题库导入导出模板和 browser remote log endpoint 为 legacy Python v1 不存在的前端侧遗留/未实现路径；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 成功状态码契约追加：新增 `backend-go/tests/contract/status_surface_test.go`，静态比较 legacy FastAPI decorator 成功状态码和 Go handler 成功状态码，覆盖非 AI 路由的 200/201/204 等成功响应状态；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过，后续仍需错误状态/错误码、响应字段、数据状态变化和真实双跑验证。
- P8 显式错误状态码契约追加：新增 `backend-go/tests/contract/error_status_surface_test.go`，静态比较 legacy route body 中显式 `HTTPException(status_code=...)` 与 Go handler/helper 暴露的错误状态；修复 Go `/questions/{id}` 详情/更新/删除对 `ErrBadRequest` 的 400 映射，以及 `/admin/knowledge/nodes/{id}` 删除对 `ErrBadRequest` 的 400 映射；本轮 `go test ./tests/contract -count=1`、`go test ./internal/adapter/http/question ./internal/adapter/http/knowledge -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过，后续仍需错误码/错误响应字段、响应字段、数据状态变化和真实双跑验证。
- P8 前端 build smoke 追加：当前 node_modules 缺少 Rollup Linux optional native package，首次 `npm run build` 失败；执行 `npm install` 补齐 optional dependency 后，`npm run build` 通过，Vite 输出大 chunk 警告但无构建错误，且未产生 tracked frontend 文件内容变更。后续仍需真实浏览器 runtime smoke 和 API 流程 smoke。
- P8 `/auth` 响应字段契约追加：新增 `backend-go/tests/contract/response_shape_surface_test.go`，静态提取 legacy `auth.py` route `response_model`、Pydantic `BaseModel` 字段和 Go auth response struct JSON tag，覆盖登录、注册、刷新、登出、当前用户、注册状态和忘记密码接口的响应顶层字段集合；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过，后续仍需扩展到其他模块、请求字段、错误响应字段、数据状态变化和真实双跑验证。
- P8 `/auth` 请求字段契约追加：新增 `backend-go/tests/contract/request_shape_surface_test.go`，静态提取 legacy `auth.py` route 函数签名中的 Pydantic body model，并与 Go auth request struct JSON tag 比较，覆盖登录、注册、修改密码和忘记密码 JSON 请求字段集合；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过，后续仍需扩展到其他模块、错误响应字段、数据状态变化和真实双跑验证。
- P8 `/session` 请求/响应字段契约追加：扩展 `backend-go/tests/contract/response_shape_surface_test.go` 和 `backend-go/tests/contract/request_shape_surface_test.go`，静态比较 legacy `session.py`、`schemas/session.py` 与 Go session handler/application DTO 的请求 body 字段和响应顶层字段；覆盖 start、chat、history、list、mode、delete、batch-delete 和 task cancel 的字段集合。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过，后续仍需扩展其他模块、错误响应字段、数据状态变化和真实双跑验证。
- P8 `/exercise` 请求/响应字段契约追加：扩展字段契约到 legacy `exercise.py` 与 Go exercise handler/application DTO，覆盖 next、submit、detail 和 solution 的响应顶层字段，以及 submit JSON 请求字段；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 `/progress` 与 `/portrait` 响应字段契约追加：扩展字段契约到 `/progress/class-ranking` 和 `/portrait` 三个画像接口；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。`/mistakes/{attempt_id}/master` 字段契约暂缓，因 Python 声明模型与实际失败分支/Go 响应结构在 `message` 字段上存在基线差异，需先做兼容决策。
- P8 `/mistakes` 响应字段契约部分追加：扩展字段契约到错题列表、统计、详情和复习推荐响应；`/mistakes/{attempt_id}/master` 保留显式例外并带 stale guard，原因是 Python 声明模型与实际失败分支/Go 响应结构在 `message` 字段上存在基线差异；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 `/resources` 请求/响应字段契约追加：扩展字段契约到资源列表、统计、收藏列表、详情、创建、更新和收藏切换响应，以及创建/更新 JSON 请求字段；本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 `/classes` 与 `/teacher` 字段契约追加：扩展字段契约到班级创建、教师班级列表、班级详情、移除学生、解散班级、班级号查询、加入/退出班级、当前班级响应，以及班级创建/加入 JSON 请求字段；扩展教师分析、班级分析和学生详情响应字段契约。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 剩余非 AI DTO 字段契约追加：扩展响应字段契约到 `/questions`、`/admin/knowledge`、`/admin/users`、`/admin/settings`、`/admin/stats`、`/admin/security-logs`、`/admin/inbox`、`/admin/bkt`、`/upload` 和 `/xidian`；扩展请求字段契约到 `/questions`、`/admin/knowledge`、`/admin/users`、`/admin/settings`、`/admin/security-logs`、`/admin/inbox`、`/admin/bkt` 和 `/xidian`；字段契约 helper 追加 `response_model=list[...]`、多 Python schema 文件和多 Go DTO 文件合并支持。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P8 错误响应字段契约追加：新增 `backend-go/tests/contract/error_body_surface_test.go`，要求每个 Go HTTP 模块保留稳定错误响应字段；FastAPI `HTTPException` 兼容模块要求 `detail`、`code`、`message`，Xidian 自定义 JSON 错误要求 `code`、`message`。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过；逐路由错误 `code`/`message` 具体取值和框架隐式 validation error 仍留待后续审计。
- P9 默认入口防回归契约追加：新增 `backend-go/tests/contract/runtime_entry_surface_test.go`，静态保护 `start.bat`、`docker-compose.yml`、`backend-go/Dockerfile`、`scripts/deploy.sh`、`scripts/update.sh`、`frontend/nginx.conf` 和 `nginx-site.conf` 的默认 Go backend/Go migration runner 入口，禁止重新引入 `uvicorn`、`alembic upgrade` 或 Python backend Dockerfile 运行入口。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过；Docker/Compose 实机验证仍因当前环境无 Docker 保持未完成。
- P6/P8 AI 边界防回归契约追加：新增 `backend-go/tests/contract/ai_boundary_surface_test.go`，要求 `/admin/ai-config`、`/questions/ai-parse`、`/session/{id}/chat`、`/portrait/generate` 和 `/exercise/submit` 的 Go 边界保留显式 TODO/占位/降级标记，并禁止 Go `cmd`/`internal` 重新引入 LangChain、LangGraph、LiteLLM、OpenAI、SymPy、Tesseract、PaddleOCR 等 legacy AI 工作流栈 token；补充 `/questions/ai-parse` 和 `/portrait/generate` 注释，明确 LLM 能力为 P6 TODO。本轮 `go test ./tests/contract -count=1`、`go test ./... -count=1` 和 `go vet ./...` 通过。
- P9 Python 下线开始：核查默认启动、Compose、Nginx、部署和更新脚本均不再承载 Python backend；更新 README，将 Go 后端标为唯一默认运行入口，AI/Agent/OCR/LLM 标为 TODO/占位接口，数据库迁移说明切换到 Go migration runner；Go Docker 镜像新增 `msp-migrate`，部署和更新脚本改为运行 Go migration runner，不再提示手动执行 legacy Alembic；新增 P9 Python 下线运行手册草案，覆盖当前下线范围、发布前检查、回滚策略和下线检查项；`bash -n scripts/deploy.sh scripts/update.sh` 通过。

### 2026-05-07

- P8/P9 完成审计补充：新增 `docs/backend-go-migration-completion-audit.md`，将“完整迁移到 Go、排除 AI 质量工作、AI/Agent/OCR/LLM/math-solver 保留 TODO/占位、Python 从默认运行时下线”的目标逐项映射到当时证据、缺口和结论；初始审计曾明确 P8 仍缺真实 Python/Go 双跑、数据库状态变化对比、精确错误码/错误消息 parity、嵌套 DTO/dynamic dict/StreamingResponse/multipart parity、浏览器运行时/API flow smoke 和性能基线，P9 仍缺 Docker/Compose `msp-migrate` 实机烟测、运行手册演练和 legacy `backend/` 归档/删除策略；后续同日用户确认跳过双跑/比对并删除 `backend/`。
- P8 `/mistakes/{attempt_id}/master` 字段契约例外关闭：Go `MarkAsMasteredResponse` 对齐 legacy Python 声明模型和前端类型，移除额外 `message` 字段；缺失学生画像改为 `ErrProfileNotFound` 并由 HTTP 层返回 404 `NOT_FOUND`/`学生画像不存在`，避免返回不符合声明成功 DTO 的 200 body；补充应用层和 HTTP 层测试，并将 response-shape contract 纳入该路由。
- P9 Python 后端清理完成：用户明确确认“不用双跑，不用比对，我自己会测试”，并确认删除 `backend/` 时包含 ignored 的 `.venv`、缓存和旧 `uploads`；已执行 `rm -rf backend`，`test -e backend; echo $?` 返回 `1`；README 和 completion audit 更新为 cleanup complete / user-owned runtime validation；新增 runtime-entry contract guard，防止 legacy Python backend 目录重新出现。
### 2026-05-09

- P4/P5/P7 性能修正完成：`/upload` 存储接口从 `[]byte` 改为 `io.Reader` 流式写入/上传，本地落盘改用 `io.Copy`，S3 PUT 使用 streaming request body，七牛 multipart 通过 `io.Pipe` 写入，保留 MIME 白名单、10MB 图片上限、500MB 资源文件上限、key 规则和响应字段；`/teacher/students` 新增教师学生聚合分页接口，PostgreSQL 下推班级/学生 join、筛选、分页和总数计算，前端学生管理页改为后端分页和服务端搜索/班级筛选，移除按班级并发详情 fan-out；`/mistakes` 列表页新增 PostgreSQL `ListMistakePage`，将错误类型/知识点/难度/日期/掌握度筛选、error_count 聚合、排序和分页下推到 SQL，应用层仅组装响应 DTO 和画像统计。
- 验证命令：`gofmt -w backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go backend-go/internal/adapter/storage/local.go backend-go/internal/adapter/storage/local_test.go backend-go/internal/adapter/storage/s3.go backend-go/internal/adapter/storage/s3_test.go backend-go/internal/adapter/storage/qiniu.go backend-go/internal/adapter/storage/qiniu_test.go backend-go/internal/application/teacher/service.go backend-go/internal/application/teacher/service_test.go backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/teacher/handler_test.go backend-go/internal/adapter/postgres/teacher_repository.go backend-go/internal/application/mistake/service.go backend-go/internal/application/mistake/service_test.go backend-go/internal/adapter/postgres/mistake_repository.go`；`go test ./internal/application/upload ./internal/adapter/storage ./internal/application/teacher ./internal/adapter/http/teacher ./internal/application/mistake ./internal/adapter/postgres -count=1`；`go test ./tests/contract -count=1`；`npm run build`；`git diff --check -- <本轮修改文件>`。
- 验证结果：定向 Go 测试通过，覆盖上传服务/本地/S3/七牛 adapter、教师应用与 HTTP 层、错题应用层，并通过 PostgreSQL adapter 包编译和既有跳过式集成入口；contract 测试通过，确认 legacy Python `backend` 目录已不存在；前端生产构建通过，保留既有 Vite large chunk 警告；本轮修改文件 whitespace check 通过。
- 交付物链接：`backend-go/internal/application/upload/`、`backend-go/internal/adapter/storage/`、`backend-go/internal/application/teacher/`、`backend-go/internal/adapter/http/teacher/`、`backend-go/internal/adapter/postgres/teacher_repository.go`、`backend-go/internal/application/mistake/`、`backend-go/internal/adapter/postgres/mistake_repository.go`、`frontend/src/pages/teacher/StudentsPage.tsx`、`frontend/src/modules/teacher/`。
- 遗留风险：S3 兼容服务需在真实对象存储环境确认 `UNSIGNED-PAYLOAD` PUT 兼容性；七牛流式 multipart 已由本地 httptest 覆盖但仍需真实云端 smoke；新增 `/teacher/students` 是 Go 前端侧聚合接口，legacy Python route surface 无同名基线，P8 如继续做严格双跑需将该优化列为 Go-only frontend API；错题 SQL 下推已编译并由应用层假仓储覆盖语义，仍需连接真实 PostgreSQL 数据集做 explain/性能基线。

### 2026-06-01

- P4 学习智能升级开始并完成本轮后端落地：移除 `/admin/bkt` 管理参数服务、HTTP handler、PostgreSQL repository 和启动注册；新增 Go forward migration `0002_replace_bkt_with_dkt.up.sql`，将学生知识点状态迁移到 `student_concept_dkt_states`，删除概念级 BKT 参数表，增加序列长度、注意力权重和最近题目字段。
- DKT 首轮完成：`/exercise/submit` 改为 `dkt-sakt-lite` 掌握度模型，基于最近练习序列、哈希题目/概念嵌入、答题结果嵌入、位置编码和 scaled dot-product attention 执行轻量自注意力式实时更新；`/progress/mastery` 和学习路径读侧改为 DKT 状态，目标掌握阈值调整为 0.85。
- 动态路径规划完成：`/progress/path?target=...` 支持按目标节点、章节、名称或描述匹配目标范围，保留未达标节点及其先修节点，按拓扑顺序输出；先修未达标时目标节点返回 `locked`、`locked_by` 和先修学习建议。
- 自适应题目生成首轮完成：`POST /questions/generate-isomorphic` 新增 Solver 校验的本地变式题模板，当前覆盖 `integral_power_exp`（$\int x^n e^{ax} dx$），按能力/难度调整参数并返回闭式解、步骤、标签和验证结果；后续可接 pyKT/LLM/Solver 服务替换生成策略。
- 错题诊断分类完成：练习提交的基础诊断内置 C/P/L/S-Type 错误分类，写入 `error_type` 和 `error_subtype`，前端练习服务类型同步暴露 `taxonomyCode` 和 `errorSubtype`。
- 验证命令：`go test ./internal/application/exercise ./internal/adapter/postgres ./internal/application/progress ./internal/adapter/http/progress ./internal/application/question ./internal/adapter/http/question ./tests/contract -count=1`、`npm run build`。
- 验证结果：上述定向 Go 测试通过，覆盖 DKT 更新、PostgreSQL adapter 编译、动态路径规划、题目生成和路由契约；前端生产构建通过，保留既有 Vite large chunk 警告。
- 交付物链接：`backend-go/internal/application/exercise/`、`backend-go/internal/adapter/postgres/exercise_repository.go`、`backend-go/internal/application/progress/`、`backend-go/internal/adapter/postgres/progress_repository.go`、`backend-go/internal/application/question/`、`backend-go/internal/adapter/http/question/`、`backend-go/migrations/0002_replace_bkt_with_dkt.up.sql`、`frontend/src/modules/exercise/services/exerciseService.ts`。
- 遗留风险：当前无学生历史训练数据，DKT 为可替换的本地 SAKT-lite 估算器，不是离线训练后的 Transformer 权重；推荐后续以 pyKT 训练 AKT/SAKT 模型并通过独立推理服务接入，真实 PostgreSQL migration 仍需在集成环境执行 `go run ./cmd/migrate` 验证。
- P7 安全评估修复完成：根据 `output/security-assessment-2026-06-01.md` 修复上传、注册默认权限、运维端点暴露、安全响应头和依赖漏洞；`POST /api/v1/upload/image` 改为要求登录并加用户/IP 本地速率限制，图片内容改为 `http.DetectContentType` + 图片解码/WEBP 签名校验；本地 `/uploads` 静态访问禁止目录索引，仅服务具体文件；`allow_teacher_registration` 初始迁移和缺省读取改为 `false`；`/health/detailed`、`/metrics` 默认限制在 `MANAGEMENT_ALLOWED_CIDRS` 内访问；Go/Nginx 补充 CSP、Permissions-Policy 等安全头；前端移除西电密码 localStorage 持久化并清理旧键；Go toolchain 锁定 `go1.25.10`，Docker builder 升级到 `golang:1.25.10-alpine`；前端生产/开发依赖漏洞清零。
- 验证命令：`go test ./...`、`go vet ./...`、`go version`、`go run golang.org/x/vuln/cmd/govulncheck@latest ./...`、`npm test -- --run`、`npm run build`、`npm audit --json`、`npm audit --omit=dev --json`。
- 验证结果：Go 全量测试和 vet 通过；`go version` 返回 `go1.25.10 windows/amd64`；`govulncheck` 报告代码路径 0 个漏洞；前端 157 个 Vitest 测试通过；生产构建通过，保留既有 Vite large chunk 警告；npm full/prod audit 均为 0 漏洞。
- 交付物链接：`backend-go/internal/adapter/http/upload/`、`backend-go/internal/application/upload/`、`backend-go/internal/platform/httpserver/`、`backend-go/internal/platform/middleware/middleware.go`、`backend-go/internal/platform/config/config.go`、`backend-go/migrations/0001_initial_schema.up.sql`、`frontend/src/modules/xidian/`、`frontend/src/pages/common/ProfilePage.tsx`、`frontend/nginx.conf`、`frontend/package.json`、`frontend/package-lock.json`、`backend-go/go.mod`、`backend-go/Dockerfile`。
- 遗留风险：上传速率限制为进程内本地限制，横向扩容时仍建议接 Redis/网关级限流；`/metrics` 和 `/health/detailed` 的实际生产可达性还需按部署网络与 `MANAGEMENT_ALLOWED_CIDRS` 做 smoke；HSTS 仍应在最终 HTTPS 入口启用并验证。

### 2026-06-28

- P6 AI/Agent 文档审计完成：复核 `backend-go/internal/adapter/llm/einoagent/`、`backend-go/internal/application/session/`、`backend-go/internal/adapter/http/adminaiconfig/`、`backend-go/internal/application/question/`、`backend-go/internal/application/exercise/`、`backend-go/internal/application/portrait/`、`frontend/src/modules/ai-config/` 和 `frontend/src/pages/admin/AIModelSettingsPage.tsx`。结论：`/session/{id}/chat` 已具备 `EINO_*` 环境变量驱动的 Eino Tutor Agent 第一片实现，但整体 LLM/Agent 功能尚未完善。
- 当时确认的未完成项：`/admin/ai-config/*` 后端仍为 501 `AI_CONFIG_TODO`，前端渠道/模型/Agent 配置页尚未后端闭环；Tutor Agent 尚未从持久化 Agent 配置中选择模型；题目 LLM 解析、诊断 Agent、画像 Agent、OCR、数学等价判定、通用数学求解和 token 级流式输出仍需后续 P6 slice。2026-06-29 已完成其中的 admin AI config、Tutor 持久化配置闭环、Portrait 运行时配置闭环、Diagnostician 运行时配置闭环、Math Solver 等价判定闭环与 Question Parser 题目解析闭环。
- 文档更新：README 和本文档已从“AI 全部 TODO”修正为“Eino Tutor Agent 已接入但 LLM/Agent 管理和 AI 质量能力未完善”，并新增 R10 风险记录前端 AI 配置页与 Go 后端能力不一致。
- 验证命令：`go test ./internal/platform/config ./internal/application/session ./internal/adapter/llm/einoagent ./tests/contract -count=1`。
- 验证结果：上述定向 Go 测试通过，覆盖 Eino 配置校验、Session ChatAgent 抽象、Eino adapter 消息转换和 AI 边界契约。
- 交付物链接：`README.md`、`docs/backend-python-to-go-refactor.md`。
- 遗留风险：本次仅做审计和文档更新，未实现新的 AI 后端能力；P6 仍需按上方未完成项继续拆分交付。

### 2026-06-29

- P6 admin LLM/Agent 配置闭环完成：新增 `backend-go/internal/application/adminaiconfig/`、`backend-go/internal/adapter/postgres/admin_ai_config_repository.go`，并将 `backend-go/internal/adapter/http/adminaiconfig/` 从 501 placeholder 替换为真实 provider/model/Agent 配置路由。支持 provider/model CRUD、provider test、OpenAI-compatible 模型拉取、provider 模型列表替换、Agent 配置 CRUD 和管理员鉴权错误映射。
- Tutor 运行时选择完成：`backend-go/internal/adapter/llm/einoagent/` 新增可配置 Tutor Agent，`backend-go/cmd/api/main.go` 接入 `adminaiconfig.Service`，`/session/{id}/chat` 优先读取持久化 `tutor` Agent 配置，未配置时兼容 `EINO_*` 环境变量；模型调用失败时保存用户消息并返回明确降级回复。
- Portrait 运行时选择完成：`backend-go/internal/application/portrait/` 新增可选 `Generator` 抽象，`backend-go/internal/adapter/llm/einoagent/` 新增可配置 Portrait Agent，`/portrait/generate` 优先读取持久化 `portrait` Agent 配置，未配置、模型不可用或生成空内容时回退到模板画像并继续保存。
- Diagnostician 运行时选择完成：`backend-go/internal/application/exercise/` 新增可选 `Diagnostician` 抽象，`backend-go/internal/adapter/llm/einoagent/` 新增可配置 Diagnostician Agent，`/exercise/submit` 对错误答案优先读取持久化 `diagnostician` Agent 配置，模型不可用、JSON 格式无效或 taxonomy 不匹配时回退到本地 C/P/L/S-Type 基础诊断。
- Math Solver 运行时选择完成：`backend-go/internal/application/exercise/` 新增 `MathSolver`/`SolverAnswerChecker` 抽象，`backend-go/internal/adapter/llm/einoagent/` 新增可配置 Math Solver Agent，`/exercise/submit` 对文本答案优先读取持久化 `math_solver` Agent 配置做结构化等价判定，模型不可用、JSON 格式无效或置信度不合规时回退本地规范化比较。
- Question Parser 运行时选择完成：`backend-go/internal/application/question/` 新增可选 `Parser` 抽象，`backend-go/internal/adapter/llm/einoagent/` 新增可配置 Question Parser Agent，`/questions/ai-parse` 优先读取持久化 `question_parser` Agent 配置抽取题目候选，模型不可用、JSON 格式无效或必填字段缺失时回退确定性形状兼容解析。
- 契约更新：`backend-go/tests/contract/route_surface_test.go` 不再要求 `/admin/ai-config` 保留 TODO 占位，`backend-go/tests/contract/ai_boundary_surface_test.go` 改为守住 admin AI config 真路由、Tutor/Portrait/Diagnostician/Math Solver/Question Parser 可配置运行时，同时继续禁止 legacy Python AI workflow 栈。
- 验证命令：`gofmt -w ...`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/exercise ./internal/adapter/llm/einoagent ./cmd/api ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/portrait ./internal/adapter/llm/einoagent ./cmd/api -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminaiconfig ./internal/adapter/http/adminaiconfig ./internal/adapter/llm/einoagent ./internal/application/session ./internal/application/portrait ./internal/adapter/postgres ./cmd/api ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1`；`GOCACHE=E:\code\msp-go\.gocache go vet ./...`；`pnpm.cmd run build`（frontend）。
- 验证结果：上述定向 Go 测试、全量 Go 测试、go vet 和前端构建通过，覆盖 admin AI config 应用层校验、HTTP 路由、Eino Tutor/Portrait/Diagnostician/Math Solver/Question Parser 配置选择、画像/诊断/判题/题目解析 prompt 构造、结构化诊断/判题/题目解析 JSON 校验、Session 降级、Exercise 判题与诊断降级、Question 解析降级、Postgres adapter 编译和 AI/route 契约。
- 交付物链接：`backend-go/internal/application/adminaiconfig/`、`backend-go/internal/adapter/http/adminaiconfig/`、`backend-go/internal/adapter/postgres/admin_ai_config_repository.go`、`backend-go/internal/adapter/llm/einoagent/agent.go`、`backend-go/internal/application/session/service.go`、`backend-go/internal/application/portrait/service.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/question/service.go`、`backend-go/cmd/api/main.go`、`backend-go/tests/contract/ai_boundary_surface_test.go`、`backend-go/tests/contract/route_surface_test.go`、`README.md`。
- 遗留风险：provider test/fetch 当前按 OpenAI-compatible `/v1` 端点实现，非兼容 provider 需要后续适配；token 级流式、OCR 和更完整的通用数学求解仍未完成。
