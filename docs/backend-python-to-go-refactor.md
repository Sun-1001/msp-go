# 后端 Python 到 Go 重构迁移文档

**Document status**: P4 in progress; P5 done; P6 AI/Agent in progress (Eino Tutor/Portrait/Diagnostician/Math Solver/Question Parser Agent and admin LLM/Agent config loop wired; OCR and broader math tools pending); P7 in progress; P8 static contract handoff done; P9 Python backend removed by user confirmation
**Last updated**: 2026-07-04
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
| P5 | `/upload` | DONE | Go P7 已承接图片上传、教师资源文件上传、本地 `/uploads` 文件落盘、S3 兼容对象存储和七牛云对象存储适配；2026-06-01 起图片上传要求登录、增加用户/IP 速率限制、按真实图片内容校验，本地 `/uploads` 禁止目录索引；2026-07-02 本地静态上传访问收紧到规范化的 `images/`、`documents/`、`videos/` 文件路径 |
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
| R11 | 可配置 AI provider `base_url`、七牛 `QINIU_UPLOAD_URL` 或西电门户 base URL 指向本机、内网、保留地址或经重定向/代理探测内部网络 | SSRF、内网信息探测、provider API key/上传 token/文件/校园账号凭据误发 | MITIGATED | 2026-07-01 新增出站 HTTP 防护：AI provider `base_url`、七牛上传 URL 和西电门户 base URL 仅允许公网 HTTPS，拒绝 userinfo/query/fragment、本机/内网/保留地址，默认 provider/Qiniu/Xidian HTTP client 禁用代理和重定向，并在拨号前校验 DNS 解析后的 IP；西电手动重定向只允许跳转到已配置 IDS/Ehall/Yjspt 主机；admin provider test/fetch、运行时 Agent 配置、Eino OpenAI-compatible 调用、七牛上传和西电门户同步均复用该防护 |
| R12 | 生产环境 CORS 配置为 `*` 且接口使用 Authorization/refresh cookie | 跨站凭据边界混乱、生产误配置被静默接受 | MITIGATED | 2026-07-01 CORS 中间件改为仅精确 origin 返回 `Access-Control-Allow-Credentials: true`，通配 origin 不再允许 credentials；生产/非开发环境启动时拒绝 `CORS_ORIGINS=*` |
| R13 | S3-compatible endpoint/public URL base 或七牛下载域名包含 userinfo/query/fragment | 对象存储签名目标和公开/私有下载 URL 混淆，凭据或静态参数被误拼接 | MITIGATED | 2026-07-01 S3 adapter 规范化 endpoint/public URL base，七牛 adapter 规范化 `QINIU_DOMAIN`，仅允许 http/https 且必须包含 scheme/host，拒绝 username/password、query 和 fragment，同时保留 path-style endpoint、CDN base path 和私有下载签名兼容 |
| R14 | 资源文件上传仅信任 multipart Content-Type | 可上传伪装 PDF/Office/text 的非预期内容，后续预览、下载或解析链路风险上升 | MITIGATED | 2026-07-01 资源上传新增轻量内容校验：PDF 校验 `%PDF-`，旧版 doc/ppt 校验 OLE magic，docx/pptx 校验 ZIP magic，text/markdown 要求 UTF-8 且不含 NUL；校验后将已读取前缀拼回 reader，避免存储内容被截断；视频暂保持非空校验以避免误伤合法编码容器 |
| R15 | 本地 `/uploads/documents/*` 文档资源按同源内容内联打开 | 教师上传的 PDF/text/Office 类资源若被浏览器插件或预览链路内联处理，会扩大同源文档执行/内容嗅探暴露面 | MITIGATED | 2026-07-01 本地上传静态服务对 `documents/` 路径设置 `Content-Disposition: attachment`，让文档资源默认下载；图片和视频路径保持无下载处置，避免破坏聊天附件和答题图片预览 |
| R16 | `/session/{id}/chat` 附件数组接受任意字符串 | 外链、非图片上传路径、路径穿越或带 query/fragment 的 URL 会被存入消息、写入 LLM prompt 并回显到前端 `<img>` | MITIGATED | 2026-07-01 Session 应用层新增附件边界校验，仅允许最多 5 个 `/uploads/images/...` 相对路径，拒绝外链、文档路径、路径穿越、反斜杠、query、fragment 和 encoded traversal；HTTP 层将非法附件映射为 422 `VALIDATION_ERROR` |
| R17 | `/exercise/submit` 的 `answer_image_url` 仅做 `/uploads/` 前缀判断 | 学生可提交文档路径、路径穿越或带 query/fragment 的异常上传 URL，进入作答记录、诊断输入和后续展示链路 | MITIGATED | 2026-07-01 抽出 `upload.IsSafeImagePath` 统一校验本地上传图片路径，练习图片答案和 Session 聊天附件复用同一规则，仅允许 `/uploads/images/...`，拒绝外链、非图片路径、路径穿越、反斜杠、query、fragment 和 encoded traversal |
| R18 | 管理员用户导出、安全日志 CSV 导出和前端教学报告 CSV 导出未处理公式前缀 | 管理员/教师下载 CSV 后用 Excel/表格软件打开时，用户名、显示名、日志标题/描述、知识点名、学生姓名等可控字段可能触发公式注入 | MITIGATED | 2026-07-01 新增 Go `internal/platform/csvsafe` 并加固前端 `dashboardExporter`，CSV 导出字段在去除前导空白后若以 `=`, `+`, `-`, `@` 开头则前置单引号；管理员用户 CSV、安全日志 CSV 和前端教学报告 CSV 均覆盖该规则 |
| R19 | 资源中心 `url` 字段和前端打开资源逻辑接受任意 URL 字符串 | 教师可保存 `javascript:`/`data:`/userinfo/异常本地上传路径或内网 external 链接，学生/教师点击资源时触发危险 scheme、异常同源路径或内网地址暴露；资源编辑链接也可能未同步更新 `content_assets` | MITIGATED | 2026-07-01 资源应用层统一规范化 create/update URL：仅允许 `http`/`https` 外链或本地 `/uploads/documents|videos/...` 路径，拒绝危险 scheme、userinfo、空白/控制字符、反斜杠、路径穿越、query/fragment 本地路径和 external 本机/内网/保留地址；HTTP 层将应用校验错误映射为 422；PostgreSQL 更新资源时在同一事务内替换或清空 `content_assets`；前端抽出 `openResourceUrl`，统一补全裸域名、拒绝危险 URL，并用 `noopener,noreferrer` 打开 |
| R20 | 聊天 Markdown 链接和消息附件链接直接渲染 href/src | LLM 或用户可控内容可能生成危险 scheme、异常附件路径或缺少 tabnabbing 防护的外链，进入聊天消息点击链路或图片加载链路 | MITIGATED | 2026-07-01 新增前端 `safeUrl` 工具：Markdown 链接仅允许 http/https/mailto，拒绝危险 scheme、userinfo、协议相对 URL、控制字符和本地路径；不安全链接退化为普通文本；聊天附件仅允许 `/uploads/images/...` 本地图片路径并拒绝路径穿越、encoded traversal、query/fragment；所有新窗口链接保留 `noopener noreferrer` |
| R21 | 数学文本渲染将题目/导入预览文本拼接为 HTML 后写入 DOM | 教师题库、导入预览或练习题文本中的 HTML/异常 LaTeX 可能进入 `innerHTML` 渲染链路，扩大 XSS 与错误兜底注入边界 | MITIGATED | 2026-07-01 `MathText` 改为先拆分普通文本和公式段，普通文本由 React 文本节点渲染，公式段交给 KaTeX 写入独立容器；错误兜底使用 `textContent`，并设置 KaTeX `trust: false`；新增 Vitest 覆盖 HTML 文本惰性渲染、块级公式渲染和异常公式不生成真实 HTML 节点 |
| R22 | 前端日志系统脱敏规则过窄且递归对象缺少循环保护 | 远程日志或控制台日志可能携带 `api_key`、Bearer/JWT、URL/query token、错误字符串中的凭据；循环对象也可能打断日志记录路径 | MITIGATED | 2026-07-01 `logger` 抽出可测试 `sanitizeLogData`，统一递归脱敏敏感字段变体、Bearer/JWT、敏感 query 和 `key=value`/`key: value` 字符串；Error stack 仅开发模式保留且同样脱敏；循环引用和过深对象使用占位符，避免日志路径抛错 |
| R23 | 前端生产默认向 `/api/v1/logs` 发送远程日志但 Go 后端没有该路由 | 浏览器会持续向未承接端点发送日志负载，造成生产噪声、契约例外和潜在日志数据暴露面 | MITIGATED | 2026-07-01 前端 logger 改为仅在显式配置 `VITE_LOG_REMOTE_ENDPOINT` 时启用远程日志；endpoint 只接受同源 `/api/...` 路径并拒绝外站、协议相对、反斜杠、空白和控制字符；移除 frontend route contract 中 `POST /logs` 例外，保留硬编码 `remoteEndpoint` 扫描防回归 |
| R24 | 管理员数据库备份导入只做外层 JSON 校验并用泛用 `INSERT` 导入 `users` | 异常大 JSON 备份可造成内存/数据库压力；`users` 行可能绕过管理员用户服务约束尝试导入管理员角色、异常角色或矛盾账号状态 | MITIGATED | 2026-07-01 `/admin/settings/database/import` 应用层新增导入规模配额，限制表数量、单表/总行数、单行字段数、字段名/字符串长度、数组长度和嵌套深度；PostgreSQL 导入层对 `users` 表仅接受显式 student/teacher role 与合法 status，拒绝 admin、异常或缺失角色/状态，并将 role/status 规范化为数据库枚举值、由 status 派生 `is_active` |
| R25 | 管理员数据库备份导出使用泛用 `SELECT *`，自由 JSON/文本字段可能带凭据片段 | `security_logs.metadata`、异常描述或未来扩展的设置值可能将 Bearer/JWT、query token、`api_key/password/secret` 等带入备份文件；`users` 表也会导出管理员账号非密码字段 | MITIGATED | 2026-07-01 PostgreSQL 备份导出层对 `users` 追加 `role <> 'ADMIN'` 过滤；导出时继续剔除密码/加密密码/session cookie，并对敏感字段名和嵌套 JSON 递归写入 `[REDACTED]`；文本值统一脱敏 Bearer、JWT、敏感 query 和 `key=value`/`key: value` 凭据片段；`security_logs.ip_address` 默认不进入数据库备份 |
| R26 | `/admin/security-logs/export` JSON/CSV 导出原样输出日志描述、用户名、IP 和 extra_data | 安全日志可能记录请求错误上下文、Authorization、token query、api key、refresh token、管理员 IP 或 CSV 公式前缀，管理员下载后造成凭据扩散或表格公式注入 | MITIGATED | 2026-07-01 新增 Go `internal/platform/redact` 公共脱敏包，安全日志 JSON/CSV 导出复用同一规则：title/description/username 字符串脱敏 Bearer/JWT/query token/assignment token，extra_data 递归脱敏敏感 key，IP 字段导出为 `[REDACTED]`；CSV 导出继续叠加 `csvsafe.Row` 公式注入防护 |
| R27 | `/admin/ai-config` provider test/fetch 和错误处理路径可能回显底层错误字符串 | 上游 HTTP client、代理或仓储错误若包含 Authorization、api_key、token query、JWT 等片段，可能被返回给管理员 UI 或写入服务端日志，扩大 provider API key 和访问令牌扩散面 | MITIGATED | 2026-07-01 AI 配置应用层 provider test/fetch 的外部错误消息、HTTP 层可公开 BadRequest/Conflict 错误和内部错误日志统一复用 `internal/platform/redact.String`；新增应用层和 HTTP 层测试覆盖响应体与日志不泄漏 Bearer、api_key 和 query token |
| R28 | `/xidian` 绑定/同步链路可能透传外部门户错误消息或日志中的凭据片段 | 西电 IDS/Ehall/Yjspt 登录失败页、重定向/HTTP client 错误或仓储错误若夹带 Authorization、cookie、session id、token query、api_key 等信息，可能返回给学生用户或写入服务端日志，造成校园账号会话和访问令牌扩散 | MITIGATED | 2026-07-01 Xidian 应用层 `ServiceError` 规范化、`ServiceError.Error()` 和 HTTP 层响应/日志统一复用 `internal/platform/redact.String`；公共脱敏规则同步覆盖 cookie/session/session_id query 与 assignment 片段；新增应用层和 HTTP 层测试覆盖门户错误、响应体和日志不泄漏 Bearer、api_key、query token、cookie/session 片段 |
| R29 | `/admin/users` 导入行结果和错误日志可能携带底层创建错误细节 | 管理员 CSV 导入包含明文临时密码，仓储/数据库/解析错误若带 SQL 参数、Authorization、api_key、token 或 password 片段，可能被写入导入结果详情或服务端日志，扩大账号导入凭据暴露面 | MITIGATED | 2026-07-01 管理员用户导入行级创建失败消息、HTTP 层 BadRequest/NotFound 可公开错误、文件读取/CSV 解析错误和 admin user 内部错误日志统一复用 `internal/platform/redact.String`；保留既有 CSV 公式注入防护，新增应用层和 HTTP 层测试覆盖导入详情、响应体和日志不泄漏 Bearer、api_key、query token、password 片段 |
| R30 | `/auth` 登录、注册、刷新、改密、登出、当前用户和密码重置路径的内部错误日志可能携带凭据片段 | 仓储/JWT/session store 或密码重置错误若包含 Authorization、api_key、token query、password、session id 等内容，可能写入服务端日志，扩大账号凭据和会话标识暴露面 | MITIGATED | 2026-07-01 Auth HTTP 层所有直接记录底层错误的 Error/Warn 日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 login/register/change-password/refresh/logout/me/registration-status/forgot-password/status 内部错误日志和响应体不泄漏 Bearer、api_key、query token、password、session_id 片段 |
| R31 | `/resources` 创建/更新校验错误和内部错误日志可能携带凭据片段 | 资源 URL 校验、仓储或收藏/统计错误若带 Authorization、api_key、token query、password 等片段，可能返回给教师/学生或写入服务端日志，扩大资源中心点击链路与账号凭据暴露面 | MITIGATED | 2026-07-01 Resource HTTP 层 ErrBadRequest 公开消息和所有内部 Error 日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 create/update 公开校验响应，以及 list/stats/favorites/detail/create/update/delete/favorite 内部错误日志与响应体不泄漏 Bearer、api_key、query token、password 片段 |
| R32 | `/admin/settings` 数据库导入/导出错误响应和内部日志可能携带凭据片段 | 管理员备份 JSON、仓储错误或 multipart 解析错误若带 Authorization、api_key、token query、password 等片段，可能返回给管理员 UI 或写入服务端日志，扩大备份文件与系统设置管理链路的凭据暴露面 | MITIGATED | 2026-07-01 Admin settings HTTP 层文件读取错误、ErrBadRequest 公开消息和内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 database export/import BadRequest 响应体和 database monitor 内部错误日志不泄漏 Bearer、api_key、query token、password 片段 |
| R33 | `/admin/stats`、`/admin/inbox` 和 `/admin/security-logs` 错误响应与内部日志可能携带凭据片段 | 管理统计、密码重置审批或安全日志管理的仓储/校验错误若夹带 Authorization、api_key、token query、password 等片段，可能返回给管理员 UI 或写入服务端日志，扩大运营管理面凭据扩散 | MITIGATED | 2026-07-01 Admin stats、admin inbox 和 security log HTTP 层 ErrBadRequest 公开消息与内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖公开响应体和服务端日志不泄漏 Bearer、api_key、query token、password 片段 |
| R34 | `/admin/knowledge` 知识节点/关系错误响应与内部日志可能携带凭据片段 | 知识图谱节点/关系的校验、NotFound 或仓储错误若夹带 Authorization、api_key、token query、password 等片段，可能返回给管理员 UI 或写入服务端日志，扩大知识管理面凭据扩散 | MITIGATED | 2026-07-01 Knowledge HTTP 层 BadRequest/NotFound 公开消息和内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖所有原 `err.Error()` 公开分支和内部日志不泄漏 Bearer、api_key、query token、password 片段 |
| R35 | `/upload` 和 `/exercise` 内部错误日志可能携带凭据片段 | 上传存储、题目推荐/提交/解析等底层错误若夹带 Authorization、api_key、token query、password 等片段，可能写入服务端日志，扩大文件上传和练习入口凭据暴露面 | MITIGATED | 2026-07-02 Upload 与 exercise HTTP 层内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖上传和练习内部错误响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R36 | `/session` 会话和聊天内部错误日志可能携带凭据片段 | 会话创建、聊天处理、历史/列表、模式更新、删除或任务取消的底层错误若夹带 Authorization、api_key、token query、password 或 LLM/tool 错误上下文，可能写入服务端日志，扩大聊天与会话管理面凭据暴露 | MITIGATED | 2026-07-02 Session HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 start/chat/history/list/end/mode/delete/batch-delete/cancel-task 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R37 | `/questions` 题库管理内部错误日志可能携带凭据片段 | 题目列表、分组、统计、详情、创建、更新、删除、批量操作、AI 解析和变式题生成的底层错误若夹带 Authorization、api_key、token query、password 或 LLM/tool 错误上下文，可能写入服务端日志，扩大题库与 AI 题目生成链路凭据暴露面 | MITIGATED | 2026-07-02 Question HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 list/groups/stats/detail/create/update/delete/batch-publish/batch-import/ai-parse/generate-isomorphic 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R38 | `/portrait` 学生画像内部错误日志可能携带凭据片段 | 画像读取、生成或清除的仓储/LLM provider 错误若夹带 Authorization、api_key、token query、password 或 Agent 错误上下文，可能写入服务端日志，扩大画像生成链路和模型调用凭据暴露面 | MITIGATED | 2026-07-02 Portrait HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 get/generate/clear 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R39 | `/teacher` 教师工作台内部错误日志可能携带凭据片段 | 教师工作台统计、学生统计/列表、教师分析、班级分析和学生详情的仓储/聚合错误若夹带 Authorization、api_key、token query、password 或 SQL/上游错误上下文，可能写入服务端日志，扩大教师管理面凭据暴露 | MITIGATED | 2026-07-02 Teacher HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 dashboard stats/students stats/students list/analytics/class analytics/student detail 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R40 | `/progress` 学习进度内部错误日志可能携带凭据片段 | 学习概览、掌握度、路径、知识图谱、统计、班级排名和章节列表的仓储/聚合错误若夹带 Authorization、api_key、token query、password 或 SQL/上游错误上下文，可能写入服务端日志，扩大学习进度读侧凭据暴露面 | MITIGATED | 2026-07-02 Progress HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 overview/mastery/path/knowledge-graph/statistics/class-ranking/chapters 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R41 | `/classes` 班级管理内部错误日志可能携带凭据片段 | 教师建班、班级列表/详情、移除学生、解散班级、班级号查询、加入/退出班级和当前班级读取的仓储错误若夹带 Authorization、api_key、token query、password 或邀请码/SQL 上下文，可能写入服务端日志，扩大班级管理面凭据与班级号暴露 | MITIGATED | 2026-07-02 Classroom HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 create/list/detail/remove/disband/lookup/join/leave/my-class 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R42 | `/mistakes` 错题本内部错误日志可能携带凭据片段 | 错题列表、统计、详情、掌握标记、删除和复习推荐的仓储/诊断错误若夹带 Authorization、api_key、token query、password 或 SQL/诊断上下文，可能写入服务端日志，扩大错题诊断与复习链路凭据暴露面 | MITIGATED | 2026-07-02 Mistake HTTP 层所有内部错误日志统一复用 `internal/platform/redact.String`；新增 HTTP 层测试覆盖 list/statistics/detail/master/delete/review-next 的响应体与日志不泄漏 Bearer、api_key、query token、password 片段 |
| R43 | `/classes` 建班和入班 JSON 请求体没有显式大小上限 | 恶意客户端可向建班或入班接口发送超大 JSON body，占用内存/CPU 并在进入业务校验前造成 HTTP 层资源压力 | MITIGATED | 2026-07-02 Classroom HTTP 层 `decodeRequest` 改为 `http.MaxBytesReader` 1MiB 上限；超限请求返回固定 400 `BAD_REQUEST` 文案且不调用 service；新增 HTTP 层测试覆盖超大建班 body 被拒绝 |
| R44 | `/xidian/binding/complete` 绑定完成 JSON 请求体没有显式大小上限 | 恶意客户端可向绑定完成接口发送超大账号/密码 JSON body，占用内存/CPU 并在进入校园门户绑定逻辑前造成 HTTP 层资源压力 | MITIGATED | 2026-07-02 Xidian HTTP 层 `decodeRequest` 改为 `http.MaxBytesReader` 1MiB 上限；超限请求返回固定 400 `BAD_REQUEST` 文案且不调用 service；新增 HTTP 层测试覆盖超大绑定完成 body 被拒绝 |
| R45 | `/resources` 创建/更新 JSON 请求体没有显式大小上限 | 恶意教师账号或被盗用凭据可向资源创建/更新接口发送超大正文、标签或 URL JSON body，占用内存/CPU 并在进入资源 URL/字段校验前造成 HTTP 层资源压力 | MITIGATED | 2026-07-02 Resource HTTP 层 `decodeRequest` 改为 `http.MaxBytesReader` 2MiB 上限；超限请求返回固定 400 `BAD_REQUEST` 文案且不调用 service；新增 HTTP 层测试覆盖超大 create/update body 被拒绝 |
| R46 | `/resources` 创建/更新 JSON 请求体允许合法 JSON 后追加尾随内容 | 恶意客户端可发送首个合法资源 JSON 后追加第二个 JSON 或非空垃圾，使代理、审计日志、WAF 与应用实际解析内容不一致，增加请求走私式歧义和审计绕过风险 | MITIGATED | 2026-07-02 Resource HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 create/update 尾随 JSON 被固定 400 `BAD_REQUEST` 拒绝且不调用 service |
| R47 | `/xidian/binding/complete` 绑定完成 JSON 请求体允许合法 JSON 后追加尾随内容 | 绑定请求包含校园门户用户名和密码，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大账号绑定链路的审计绕过和请求歧义风险 | MITIGATED | 2026-07-02 Xidian HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖绑定完成请求尾随 JSON 被固定 400 `BAD_REQUEST` 拒绝且不调用 service |
| R48 | `/classes` 建班和入班 JSON 请求体允许合法 JSON 后追加尾随内容 | 恶意客户端可在建班或入班请求首个合法 JSON 后追加第二个 JSON 或非空垃圾，使代理、审计日志、WAF 与应用实际解析内容不一致，扩大班级创建/邀请码加入链路的请求歧义和审计绕过风险 | MITIGATED | 2026-07-02 Classroom HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 create/join 尾随 JSON 被固定 400 `BAD_REQUEST` 拒绝且不调用 service |
| R49 | `/auth` 登录、注册、改密和忘记密码 JSON 请求体允许合法 JSON 后追加尾随内容 | 认证入口会处理密码、角色、邮箱和重置原因，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大认证链路的请求歧义、角色审计绕过和敏感字段误判风险 | MITIGATED | 2026-07-02 Auth HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 login/register/change-password/forgot-password 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝 |
| R50 | `/session` 会话创建、聊天、模式切换和批量删除 JSON 请求体允许合法 JSON 后追加尾随内容 | 会话入口会处理聊天消息、附件路径、会话模式和批量删除 ID，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大会话/LLM 链路的请求歧义和审计绕过风险 | MITIGATED | 2026-07-02 Session HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 start/chat/mode/batch-delete 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R51 | `/exercise/submit` 提交答案 JSON 请求体允许合法 JSON 后追加尾随内容 | 练习提交入口会处理文本答案、图片答案、解题步骤和耗时，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大答案提交和诊断链路的请求歧义与审计绕过风险 | MITIGATED | 2026-07-02 Exercise HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 submit 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R52 | `/questions` 题目写入、批量操作、AI 解析和变式生成 JSON 请求体允许合法 JSON 后追加尾随内容 | 题库入口会处理题干、答案、解题步骤、批量 ID、AI 解析文本和变式生成参数，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大题库和 AI 出题链路的请求歧义与审计绕过风险 | MITIGATED | 2026-07-02 Question HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 create/update/batch/import/ai-parse/generate-isomorphic 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R53 | `/admin/knowledge` 知识节点和关系写入 JSON 请求体允许合法 JSON 后追加尾随内容 | 知识管理入口会处理知识节点、章节、公式、标签和关系边，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大管理员知识图谱变更链路的请求歧义和审计绕过风险 | MITIGATED | 2026-07-02 Knowledge HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 create/update node/relation 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R54 | `/admin/settings` 注册配置、系统配置和数据库导出 JSON 请求体允许合法 JSON 后追加尾随内容 | 管理配置入口会处理注册开关、系统展示信息和导出表清单，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大系统配置和数据导出链路的请求歧义与审计绕过风险 | MITIGATED | 2026-07-02 Admin settings HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 registration/general/database export 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R55 | `/admin/ai-config` provider、model、agent 配置 JSON 请求体允许合法 JSON 后追加尾随内容 | AI 配置入口会处理 provider endpoint、API key、模型列表、默认模型和 agent runtime 参数，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大 LLM 出站配置链路的请求歧义、凭据误判和审计绕过风险 | MITIGATED | 2026-07-02 Admin AI config HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 provider/model/credentials/provider-models/agent 配置写入尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R56 | `/admin/users` 创建用户、状态变更和用户更新 JSON 请求体允许合法 JSON 后追加尾随内容 | 用户管理入口会处理账号、邮箱、密码、角色、状态和显示名，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大账号管理链路的请求歧义、角色审计绕过和敏感字段误判风险 | MITIGATED | 2026-07-02 Admin user HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 create/status/update 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R57 | `/admin/inbox/{request_id}/review` 密码重置审批 JSON 请求体允许合法 JSON 后追加尾随内容 | 管理员审批入口会处理 approve/reject 动作和拒绝原因，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大密码重置审批链路的请求歧义和审计绕过风险 | MITIGATED | 2026-07-02 Admin inbox HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 review 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R58 | `/admin/security-logs` 删除、导出和归档 JSON 请求体允许合法 JSON 后追加尾随内容 | 安全日志管理入口会处理删除 ID、导出筛选和归档时间，若客户端在首个合法 JSON 后追加第二个 JSON 或非空垃圾，可能造成代理、审计日志、WAF 与应用实际解析内容不一致，扩大安全审计数据管理链路的请求歧义和审计绕过风险 | MITIGATED | 2026-07-02 Security log HTTP 层 `decodeRequest` 在首个 JSON 后追加 EOF 检查，只接受单个 JSON 文档；新增 HTTP 层测试覆盖 delete/export/archive 尾随 JSON 被固定 422 `VALIDATION_ERROR` 拒绝且不调用 service |
| R59 | HTTP 严格 JSON 单文档解析逻辑在多个 adapter 包重复实现 | 重复的 `MaxBytesReader`、首个 JSON decode 和 EOF 检查分散在多个 handler 中，后续新增入口容易遗漏严格单文档校验或形成错误响应不一致，增加边界回归风险和维护成本 | MITIGATED | 2026-07-02 新增 `internal/platform/httpjson.DecodeStrict`，集中实现 body 大小限制、单 JSON 文档解析和尾随内容拒绝；各 HTTP 包保留本地错误映射包装，统一复用公共 helper；新增公共 helper 单元测试覆盖正常 JSON、尾随第二个 JSON、尾随垃圾和超限 body |
| R60 | 本地 `/uploads/` 静态文件服务会清理请求路径后继续打开文件，且未限制已知上传前缀 | 带 `..`、编码片段、反斜杠或非标准路径的请求可能被不同层解释不一致；即使不能越过 uploads 根目录，也可能扩大 uploads 根目录下意外文件的公开读取范围，增加静态资源边界绕过和审计歧义 | MITIGATED | 2026-07-02 `uploadsFileHandler` 改为先规范校验再打开文件，只接受已规范化且位于 `images/`、`documents/`、`videos/` 下的文件路径，拒绝目录、根目录文件、`..`、重复分隔符、反斜杠和 `%` 编码形态；补充静态上传路径单元测试和完整 handler 回归 |
| R61 | 本地上传路径安全校验规则散落在 upload、resource 和 httpserver 包中 | 图片附件、练习答案图、资源 URL 和静态文件服务若各自维护 `/uploads/*` 路径判断，后续新增目录或修补边界时容易出现规则漂移，导致某些入口接受 query/fragment、编码穿越、反斜杠或非预期上传目录 | MITIGATED | 2026-07-02 新增 `internal/platform/uploadpath` 作为本地上传 URL/key 的公共校验边界，统一提供图片路径、文档/视频资源路径、静态可服务 key 和文档 key 判断；`upload.IsSafeImagePath` 保留为兼容包装，resource 和 httpserver 直接复用公共 helper；新增平台单元测试并补资源 URL 集成用例 |
| R62 | HTTP query 整数解析逻辑在多个 adapter 包重复实现 | 多个 handler 各自使用 `strconv.Atoi`、fallback 和范围判断，后续新增分页/limit/offset 参数时容易出现错误文案漂移、范围校验遗漏或 service fake 测试无法发现超大查询参数进入应用层的问题 | MITIGATED | 2026-07-02 新增 `internal/platform/httpquery`，集中提供可选整数和有界整数解析；session limit/offset、admin stats limit 和 security log page/page_size 复用公共 helper；security log `page_size` 超界在 HTTP 层提前 422 拒绝且不调用 service；新增公共 helper 单元测试和 HTTP 层边界回归 |
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
- Verification commands: gofmt targeted Go files; GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/outbound ./internal/application/adminaiconfig ./internal/application/auth ./internal/adapter/llm/einoagent ./internal/adapter/storage ./internal/integration/xidian -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/exercise ./internal/adapter/llm/einoagent ./cmd/api ./tests/contract -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/portrait ./internal/adapter/llm/einoagent ./cmd/api -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminaiconfig ./internal/adapter/http/adminaiconfig ./internal/adapter/llm/einoagent ./internal/application/session ./internal/application/portrait ./internal/adapter/postgres ./cmd/api ./tests/contract -count=1; GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1; GOCACHE=E:\code\msp-go\.gocache go vet ./...; pnpm.cmd run build in frontend; earlier slice also ran go test ./internal/platform/config ./internal/application/session ./internal/adapter/llm/einoagent ./tests/contract -count=1; go mod tidy; go test ./cmd/api ./internal/... -count=1
- Verification results: Added EINO_* runtime config, Eino ADK Tutor Agent adapter, Session ChatAgent abstraction, /session/{id}/chat Eino-first path, provider/model/Agent config service, PostgreSQL adapter, admin HTTP routes, provider test/fetch, Fernet-protected provider API keys, Tutor runtime selection from persisted `tutor` Agent config, Portrait runtime selection from persisted `portrait` Agent config with template fallback, Diagnostician runtime selection from persisted `diagnostician` Agent config with local diagnosis fallback, Math Solver runtime selection from persisted `math_solver` Agent config with local answer-check fallback, Question Parser runtime selection from persisted `question_parser` Agent config with deterministic parse fallback, provider outbound URL/HTTP guard for admin test/fetch plus Eino runtime calls, Qiniu upload URL outbound guard, and Xidian portal base URL outbound guard. Config, outbound HTTP guard, session, portrait, exercise, question, Eino adapter, admin AI config application/HTTP/Postgres, storage adapter, Xidian integration, AI boundary contracts, cmd/api, targeted contract tests, full Go test suite, go vet, and frontend build passed for the recorded slices. Current implementation confirms LLM/Agent management is no longer a 501 placeholder, while OCR and broader math tools remain incomplete.
- Deliverables: backend-go/internal/application/adminaiconfig/; backend-go/internal/adapter/http/adminaiconfig/; backend-go/internal/adapter/postgres/admin_ai_config_repository.go; backend-go/internal/adapter/llm/einoagent/; backend-go/internal/platform/outbound/; backend-go/tests/contract/ai_boundary_surface_test.go; backend-go/tests/contract/route_surface_test.go; backend-go/internal/application/question/service.go; backend-go/internal/application/session/service.go; backend-go/internal/application/portrait/service.go; backend-go/internal/application/exercise/service.go; backend-go/internal/platform/config/config.go; backend-go/cmd/api/main.go; .env.example
- Residual risks: token-level streaming, broader math solving, OCR and teaching feedback still need follow-up Eino slices; provider test/fetch uses OpenAI-compatible `/v1` endpoints and may need provider-specific adapters for non-compatible vendors; local/on-prem model gateways now require an explicit future allow-list or deployment proxy decision instead of default localhost/private-network access. The Go backend must not silently fall back to legacy Python.
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

### 2026-07-01

- P6/P7 安全边界加固完成：新增 `backend-go/internal/platform/outbound/`，统一保护可配置 AI provider、七牛上传和西电门户出站 HTTP。AI `base_url`、`QINIU_UPLOAD_URL` 和 `XIDIAN_*_BASE` 仅允许公网 HTTPS，拒绝 userinfo、query、fragment、本机、内网和保留地址；默认出站 client 禁用环境代理和跨站重定向，并在拨号前解析 DNS、拒绝解析到非公网地址；西电手动重定向只允许跳转到已配置 IDS/Ehall/Yjspt 主机，降低 admin AI config provider test/fetch、Eino OpenAI-compatible runtime、七牛上传 token/文件误发和西电校园账号凭据误发的 SSRF/内网探测风险。CORS 边界同步加固：通配 origin 不再返回 credentials，生产/非开发环境拒绝 `CORS_ORIGINS=*`。S3-compatible 存储同步规范化 endpoint/public URL base，七牛下载域名同步规范化 `QINIU_DOMAIN`，拒绝 userinfo/query/fragment 但保留私有 HTTP endpoint、path-style endpoint、CDN base path 和私有下载签名兼容。上传资源文件同步新增轻量内容校验：PDF、Office、文本类不再只信任 multipart Content-Type，校验后拼回已读取前缀以保持存储内容完整。本地 `/uploads/documents/*` 静态响应设置 `Content-Disposition: attachment`，降低同源文档内联打开风险，同时保持图片/视频预览路径不变。Session 聊天附件和练习图片答案统一复用 `upload.IsSafeImagePath`，限制为本地 `/uploads/images/...` 路径，非法附件/答案图在应用层拒绝，避免外链或异常 URL 进入消息历史、LLM prompt、作答记录、诊断输入和前端图片渲染链路。管理员用户 CSV、安全日志 CSV 和前端教学报告 CSV 导出新增公式注入防护，表格软件可执行前缀字段会前置单引号。
- Auth refresh token 默认轮换加固完成：`auth.NewService` 默认创建本地 refresh session store，`WithRefreshSessionStore(nil)` 不再覆盖默认 store，避免漏传 option 时退化为可重复使用 refresh token；生产环境仍通过 strict Redis store 保持共享会话要求。
- 资源中心链接边界加固完成：`/resources` create/update 在应用层统一规范化 URL，危险 scheme、userinfo、异常本地上传路径、路径穿越、反斜杠、控制字符、external 内网/本机/保留地址均会被拒绝；HTTP 层返回 422 `VALIDATION_ERROR`；PostgreSQL 更新资源时在同一事务内同步替换或清空 `content_assets`，修复编辑资源链接不落库的行为洞；前端学生/教师资源打开逻辑统一收敛到 `openResourceUrl`，仅打开规范化后的 `http/https` 或本地上传文档/视频路径，并附带 `noopener,noreferrer`。聊天渲染链路同步新增 `safeUrl`，Markdown 链接仅允许 http/https/mailto，危险链接退化为不可点击文本；学生聊天附件仅渲染 `/uploads/images/...` 本地图片路径，拒绝外链、文档路径、路径穿越和 encoded traversal。数学文本渲染链路移除整段 `innerHTML` 拼接写入，普通文本由 React 文本节点承载，KaTeX 仅渲染到独立公式容器，异常公式兜底使用 `textContent`。前端日志链路抽出 `sanitizeLogData`，补齐敏感字段变体、Bearer/JWT、URL/query token、错误字符串凭据和循环对象保护；远程日志改为显式配置才启用，并限制为同源 `/api/...` endpoint，移除无后端承接的默认 `/api/v1/logs`。
- P7 `/admin/settings` 数据库导入边界加固完成：应用层 `/admin/settings/database/import` 在进入仓储前限制备份表数量、单表行数、总行数、单行字段数、字段名长度、字符串长度、数组长度和 JSON 嵌套深度，超限返回 `ErrBadRequest`，避免 100MB JSON 在业务层形成过大解析/写库压力；PostgreSQL 泛用导入对 `users` 表新增纯函数过滤，只允许显式 student/teacher role 与合法 status，拒绝 admin、异常角色、异常状态和缺失 role/status 的用户行，并将 role/status 转成 PostgreSQL 枚举值、由 status 派生 `is_active`，避免数据库备份导入绕过管理员用户服务的角色/状态边界。
- P7 `/admin/settings` 数据库导出脱敏加固完成：PostgreSQL 备份导出 `users` 表时排除管理员账号行，降低备份文件散布管理员账号元数据的风险；泛用导出层保留密码、加密密码、session cookie 剔除，并新增字段名/JSON key 级递归脱敏，命中 password/passwd/secret/api key/access key/token/authorization/credential 等敏感键时写入 `[REDACTED]`；普通字符串同样脱敏 Bearer、JWT、敏感 query 参数和 `key=value`/`key: value` 凭据片段；`security_logs.ip_address` 不进入数据库备份，减少安全日志导出中的 IP 暴露面。
- P7 `/admin/security-logs/export` 导出脱敏加固完成：新增 `backend-go/internal/platform/redact/` 公共脱敏包，数据库备份导出和安全日志导出复用同一套敏感 key、Bearer、JWT、敏感 query 和 assignment token 脱敏规则；安全日志 JSON 导出对 title/description/username 做字符串脱敏，extra_data 递归脱敏，ip_address 输出 `[REDACTED]`；CSV 导出在脱敏后继续走 `csvsafe.Row`，同时覆盖公式注入防护和凭据/IP 不落盘。
- P6/P7 `/admin/ai-config` 错误回显脱敏加固完成：provider test 和 fetch models 的外部 HTTP 错误消息、AI 配置 HTTP 层可公开 BadRequest/Conflict 错误，以及内部错误日志统一复用 `backend-go/internal/platform/redact.String`；当底层错误包含 `Authorization`、Bearer、`api_key`、query token 或 JWT 片段时，返回给管理员 UI 和写入服务端日志的内容只保留 `[REDACTED]` 标记。
- P7 `/xidian` 错误回显脱敏加固完成：绑定和同步链路的 `ServiceError` 进入应用层、HTTP 响应和服务端日志前统一复用 `backend-go/internal/platform/redact.String`；公共脱敏规则同步覆盖 cookie/session/session_id 的 query 与 assignment 片段，避免外部门户错误、重定向/HTTP client 错误或仓储错误将校园会话、Bearer、api_key、query token 片段返回给学生用户或写入日志。
- P3/P7 `/admin/users` 导入错误脱敏加固完成：管理员用户 CSV 导入的行级创建失败消息、HTTP 层可公开 BadRequest/NotFound 错误、文件读取/CSV 解析错误和内部错误日志统一复用 `backend-go/internal/platform/redact.String`；当底层错误包含 Authorization、Bearer、api_key、query token 或 password 片段时，导入结果详情、响应体和服务端日志只保留 `[REDACTED]` 标记。
- 验证命令：`gofmt -w backend-go/internal/platform/outbound/http_client.go backend-go/internal/platform/outbound/http_client_test.go backend-go/internal/platform/csvsafe/csvsafe.go backend-go/internal/platform/csvsafe/csvsafe_test.go backend-go/internal/application/adminaiconfig/service.go backend-go/internal/application/adminaiconfig/service_test.go backend-go/internal/application/auth/service.go backend-go/internal/application/auth/service_test.go backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go backend-go/internal/application/session/service.go backend-go/internal/application/session/service_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/exercise/service_test.go backend-go/internal/application/resource/service.go backend-go/internal/application/resource/service_test.go backend-go/internal/application/securitylog/service.go backend-go/internal/application/securitylog/service_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go backend-go/internal/adapter/http/exercise/handler_test.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go backend-go/internal/adapter/llm/einoagent/agent.go backend-go/internal/adapter/llm/einoagent/agent_test.go backend-go/internal/adapter/postgres/resource_repository.go backend-go/internal/adapter/storage/factory.go backend-go/internal/adapter/storage/qiniu.go backend-go/internal/adapter/storage/qiniu_test.go backend-go/internal/adapter/storage/s3.go backend-go/internal/adapter/storage/s3_test.go backend-go/internal/integration/xidian/client.go backend-go/internal/integration/xidian/client_test.go backend-go/internal/integration/xidian/session.go backend-go/internal/platform/middleware/middleware.go backend-go/internal/platform/middleware/middleware_test.go backend-go/internal/platform/config/config.go backend-go/internal/platform/config/config_test.go backend-go/internal/platform/httpserver/server.go backend-go/internal/platform/httpserver/server_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/outbound ./internal/platform/csvsafe ./internal/application/adminaiconfig ./internal/application/auth ./internal/application/upload ./internal/application/session ./internal/application/exercise ./internal/application/resource ./internal/application/securitylog ./internal/adapter/http/adminuser ./internal/adapter/http/session ./internal/adapter/http/exercise ./internal/adapter/http/resource ./internal/adapter/llm/einoagent ./internal/adapter/postgres ./internal/adapter/storage ./internal/integration/xidian ./internal/platform/middleware ./internal/platform/config ./internal/platform/httpserver -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/upload ./internal/adapter/http/upload ./internal/adapter/storage ./internal/platform/httpserver -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1`；`GOCACHE=E:\code\msp-go\.gocache go vet ./...`；`GOCACHE=E:\code\msp-go\.gocache go test ./tests/contract -run TestFrontendAPICallsAreCoveredByGoOrExplicitlyClassified -count=1`；`pnpm.cmd exec vitest run src/libs/export/__tests__/dashboardExporter.test.ts`；`pnpm.cmd exec vitest run src/pages/student/__tests__/ResourcesPage.test.ts`；`pnpm.cmd exec vitest run src/libs/utils/__tests__/safeUrl.test.ts src/components/chat/__tests__/MarkdownContent.test.tsx src/components/chat/__tests__/MessageItem.test.tsx`；`pnpm.cmd exec vitest run src/libs/math/__tests__/MathText.test.tsx src/libs/utils/__tests__/safeUrl.test.ts src/components/chat/__tests__/MarkdownContent.test.tsx src/components/chat/__tests__/MessageItem.test.tsx`；`pnpm.cmd exec vitest run src/libs/utils/__tests__/logger.test.ts src/libs/utils/__tests__/safeUrl.test.ts src/libs/math/__tests__/MathText.test.tsx src/components/chat/__tests__/MarkdownContent.test.tsx src/components/chat/__tests__/MessageItem.test.tsx`；`pnpm.cmd run build`。
- 验证结果：定向测试、全量 Go 测试、go vet、frontend route surface 契约、前端导出器 Vitest、资源 URL Vitest、安全 URL/聊天 Markdown/消息附件 Vitest、MathText Vitest、logger Vitest 和前端生产构建通过，覆盖 provider URL 安全校验、admin AI config OpenAI-compatible 模型拉取、Eino config 校验、七牛上传 URL 安全校验、七牛下载域名混淆输入拒绝、S3 endpoint/public URL base 混淆输入拒绝、西电门户 base URL 安全校验、西电跨域重定向拒绝、outbound IPv4/IPv6 地址族边界、CORS wildcard credentials 拒绝、生产 CORS wildcard 配置拒绝、auth refresh token 默认本地轮换和 refresh token 重放拒绝、资源上传伪造 PDF 拒绝、文本 NUL 拒绝、校验后前缀字节完整存储、本地 `/uploads/documents/*` 下载处置头、图片上传静态访问不加下载处置、Session 聊天附件外链/文档路径/路径穿越/query/encoded traversal/超量拒绝、非法附件 HTTP 422 映射、练习图片答案外链/文档路径/路径穿越/query/encoded traversal 拒绝、资源中心 URL 危险 scheme/userinfo/异常本地路径/external 内网地址拒绝、资源链接编辑落库路径编译覆盖、前端资源打开危险 URL 拒绝和 noopener/noreferrer 打开、聊天 Markdown 危险 URL 不可点击、聊天图片附件只渲染上传图片路径、MathText 普通 HTML 仅按文本渲染、块级公式保留 KaTeX 渲染、异常公式不生成真实 HTML 节点、logger 敏感字段/字符串脱敏、循环对象保护、生产 Error stack 移除、远程日志 endpoint 仅显式同源 API 配置启用且无 `/logs` 契约例外、管理员用户/安全日志 CSV 公式字段转义，以及前端教学报告 CSV 知识点/学生名/星期字段公式前缀转义。
- 数据库导入边界验证命令：`gofmt -w backend-go/internal/application/adminsettings/service.go backend-go/internal/application/adminsettings/service_test.go backend-go/internal/adapter/postgres/admin_settings_repository.go backend-go/internal/adapter/postgres/admin_settings_repository_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminsettings ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminsettings ./internal/adapter/http/adminsettings ./internal/adapter/postgres -count=1`；`git diff --check -- backend-go/internal/application/adminsettings/service.go backend-go/internal/application/adminsettings/service_test.go backend-go/internal/adapter/postgres/admin_settings_repository.go backend-go/internal/adapter/postgres/admin_settings_repository_test.go docs/backend-python-to-go-refactor.md`。
- 数据库导入边界验证结果：应用层和 HTTP/adminsettings/PostgreSQL adapter 定向测试通过；覆盖备份导入未知表跳过、超量表、超量行、超宽行、超大字符串、过深 JSON 拒绝且不调用仓储，以及 `users` 导入过滤敏感字段/不安全列、student/teacher role 与 status 枚举规范化、admin role/异常 role/异常 status/缺失 role/status 用户行跳过；`git diff --check` 仅报告 Windows LF/CRLF 提示，无 whitespace 错误。
- 数据库导出脱敏验证命令：`gofmt -w backend-go/internal/adapter/postgres/admin_settings_repository.go backend-go/internal/adapter/postgres/admin_settings_repository_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminsettings ./internal/adapter/http/adminsettings ./internal/adapter/postgres -count=1`。
- 数据库导出脱敏验证结果：PostgreSQL adapter 与 adminsettings 应用/HTTP/adapter 定向测试通过；覆盖 `users` 导出 SQL 排除 admin role、`security_logs.ip_address` 字段剔除、敏感字段名/嵌套 JSON key 递归 `[REDACTED]`、Bearer/JWT/query token/assignment token 字符串脱敏，以及普通安全字段保持原值。
- 安全日志导出脱敏验证命令：`gofmt -w backend-go/internal/platform/redact/redact.go backend-go/internal/platform/redact/redact_test.go backend-go/internal/adapter/postgres/admin_settings_repository.go backend-go/internal/adapter/postgres/admin_settings_repository_test.go backend-go/internal/application/securitylog/service.go backend-go/internal/application/securitylog/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/redact ./internal/application/securitylog ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/securitylog ./internal/adapter/http/securitylog ./internal/application/adminsettings ./internal/adapter/http/adminsettings ./internal/adapter/postgres ./internal/platform/redact -count=1`。
- 安全日志导出脱敏验证结果：公共脱敏包、securitylog 应用/HTTP、adminsettings 应用/HTTP 和 PostgreSQL adapter 定向测试通过；覆盖 security log JSON/CSV 导出不泄漏 Bearer、query token、api key、refresh token、IP，extra_data 安全字段保留、敏感字段写 `[REDACTED]`，CSV 仍保留公式注入转义。
- AI 配置错误脱敏验证命令：`gofmt -w backend-go/internal/application/adminaiconfig/service.go backend-go/internal/application/adminaiconfig/service_test.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminaiconfig/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminaiconfig ./internal/adapter/http/adminaiconfig ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/application/adminaiconfig/service.go backend-go/internal/application/adminaiconfig/service_test.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminaiconfig/handler_test.go docs/backend-python-to-go-refactor.md`。
- AI 配置错误脱敏验证结果：adminaiconfig 应用层、HTTP 层和公共 redact 定向测试通过；覆盖 provider fetch/test 外部错误消息不泄漏 Bearer、api_key、query token，BadRequest 响应体不泄漏凭据片段，InternalError 服务端日志与响应体均不泄漏原始 token；`git diff --check` 仅报告 Windows LF/CRLF 提示，无 whitespace 错误。
- 西电错误脱敏验证命令：`gofmt -w backend-go/internal/application/xidian/service.go backend-go/internal/application/xidian/service_test.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go backend-go/internal/platform/redact/redact.go backend-go/internal/platform/redact/redact_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/xidian ./internal/adapter/http/xidian ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/application/xidian/service.go backend-go/internal/application/xidian/service_test.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go docs/backend-python-to-go-refactor.md`；`Select-String -Path backend-go\internal\platform\redact\redact.go,backend-go\internal\platform\redact\redact_test.go -Pattern "[ \t]+$"`。
- 西电错误脱敏验证结果：xidian 应用层、HTTP 层和公共 redact 定向测试通过；覆盖门户 `ServiceError` message/Error 不泄漏 Bearer、query token、api_key、cookie，HTTP ServiceError 响应体不泄漏凭据片段，InternalError 服务端日志与响应体均不泄漏原始 token；`git diff --check` 仅报告 Windows LF/CRLF 提示，未跟踪 redact 文件尾随空白扫描无输出，无 whitespace 错误。
- 管理员用户导入错误脱敏验证命令：`gofmt -w backend-go/internal/application/adminuser/service.go backend-go/internal/application/adminuser/service_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminuser ./internal/adapter/http/adminuser ./internal/platform/redact ./internal/platform/csvsafe -count=1`；`git diff --check -- backend-go/internal/application/adminuser/service.go backend-go/internal/application/adminuser/service_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go docs/backend-python-to-go-refactor.md`。
- 管理员用户导入错误脱敏验证结果：adminuser 应用层、HTTP 层、公共 redact 与 csvsafe 定向测试通过；覆盖导入行级创建失败消息不泄漏 Bearer、api_key、password，BadRequest 响应体不泄漏凭据片段，stats/InternalError 服务端日志与响应体均不泄漏原始 token，同时保持管理员用户 CSV 导出公式字段转义；`git diff --check` 仅报告 Windows LF/CRLF 提示，无 whitespace 错误。
- Auth 日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/auth ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go docs/backend-python-to-go-refactor.md`。
- Auth 日志脱敏验证结果：auth HTTP 层和公共 redact 定向测试通过；覆盖 login/register/change-password/refresh/logout/me/registration-status/forgot-password/status 内部错误日志不泄漏 Bearer、api_key、query token、password、session_id，响应体继续只返回通用错误或安全消息。
- 资源中心错误脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/resource ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go docs/backend-python-to-go-refactor.md`。
- 资源中心错误脱敏验证结果：resource HTTP 层和公共 redact 定向测试通过；覆盖 create/update 应用校验错误响应体不泄漏 Bearer、api_key、query token，list/stats/favorites/detail/create/update/delete/favorite 内部错误日志与响应体均不泄漏原始 token/password，同时保留资源 URL/分页/权限/NotFound 既有映射。
- Admin settings 错误脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminsettings/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminsettings ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminsettings/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin settings 错误脱敏验证结果：adminsettings HTTP 层和公共 redact 定向测试通过；覆盖 database export/import BadRequest 响应体不泄漏 Bearer、api_key、query token、password，database monitor 内部错误日志与响应体均不泄漏原始 token/password，同时保留备份文件格式、大小限制和管理员权限既有映射。
- 管理运营接口错误脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminstats/handler_test.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminstats ./internal/adapter/http/admininbox ./internal/adapter/http/securitylog ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminstats/handler_test.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go docs/backend-python-to-go-refactor.md`。
- 管理运营接口错误脱敏验证结果：adminstats、admininbox、securitylog HTTP 层和公共 redact 定向测试通过；覆盖 ErrBadRequest 响应体不泄漏 Bearer、api_key、query token、password，内部错误日志与响应体均不泄漏原始 token/password，同时保留管理接口权限、分页、格式校验和既有状态码映射。
- 知识图谱错误脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/knowledge ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go docs/backend-python-to-go-refactor.md`。
- 知识图谱错误脱敏验证结果：knowledge HTTP 层和公共 redact 定向测试通过；覆盖 list/get/create/update/delete node、create/update/delete relation 的 BadRequest/NotFound 公开响应体不泄漏 Bearer、api_key、query token、password，stats 内部错误日志与响应体均不泄漏原始 token/password，同时保留知识图谱权限、输入校验和既有状态码映射。
- 上传/练习内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/upload/handler_test.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/upload ./internal/adapter/http/exercise ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/upload/handler_test.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go docs/backend-python-to-go-refactor.md`。
- 上传/练习内部日志脱敏验证结果：upload、exercise HTTP 层和公共 redact 定向测试通过；覆盖上传存储错误和练习获取下一题内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留上传类型/大小、练习 BadRequest/NotFound/Forbidden 既有状态码映射。
- Session 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/session ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go docs/backend-python-to-go-refactor.md`。
- Session 内部日志脱敏验证结果：session HTTP 层和公共 redact 定向测试通过；覆盖 start/chat/history/list/end/mode/delete/batch-delete/cancel-task 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留聊天 SSE 错误、附件校验、NotFound 和分页校验既有映射。
- Question 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/question ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go docs/backend-python-to-go-refactor.md`。
- Question 内部日志脱敏验证结果：question HTTP 层和公共 redact 定向测试通过；覆盖 list/groups/stats/detail/create/update/delete/batch-publish/batch-import/ai-parse/generate-isomorphic 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留题库权限、输入校验、BadRequest/NotFound/Forbidden 和 AI 解析/变式题既有映射。
- Portrait 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/portrait/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/portrait ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/portrait/handler_test.go docs/backend-python-to-go-refactor.md`。
- Portrait 内部日志脱敏验证结果：portrait HTTP 层和公共 redact 定向测试通过；覆盖 get/generate/clear 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留画像鉴权和固定公开错误文案既有映射。
- Teacher 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/teacher/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/teacher ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/teacher/handler_test.go docs/backend-python-to-go-refactor.md`。
- Teacher 内部日志脱敏验证结果：teacher HTTP 层和公共 redact 定向测试通过；覆盖 dashboard stats/students stats/students list/analytics/class analytics/student detail 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留教师权限、分页校验、time_range 校验和 NotFound 既有映射。
- Progress 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/progress/handler.go backend-go/internal/adapter/http/progress/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/progress ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/progress/handler.go backend-go/internal/adapter/http/progress/handler_test.go docs/backend-python-to-go-refactor.md`。
- Progress 内部日志脱敏验证结果：progress HTTP 层和公共 redact 定向测试通过；覆盖 overview/mastery/path/knowledge-graph/statistics/class-ranking/chapters 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留鉴权、查询参数转发和章节响应包装既有映射。
- Classroom 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/classroom ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go docs/backend-python-to-go-refactor.md`。
- Classroom 内部日志脱敏验证结果：classroom HTTP 层和公共 redact 定向测试通过；覆盖 create/list/detail/remove/disband/lookup/join/leave/my-class 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留教师/学生权限、NotFound/Forbidden/Conflict 和输入校验既有映射。
- Mistake 内部日志脱敏验证命令：`gofmt -w backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/mistake ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go docs/backend-python-to-go-refactor.md`。
- Mistake 内部日志脱敏验证结果：mistake HTTP 层和公共 redact 定向测试通过；覆盖 list/statistics/detail/master/delete/review-next 内部错误的响应体与日志不泄漏 Bearer、api_key、query token、password，同时保留鉴权、列表参数校验、NotFound/ProfileNotFound 和 literal review route 既有映射。
- Classroom JSON body 边界验证命令：`gofmt -w backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/classroom ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go docs/backend-python-to-go-refactor.md`。
- Classroom JSON body 边界验证结果：classroom HTTP 层和公共 redact 定向测试通过；覆盖超大建班 JSON body 被 `MaxBytesReader` 拒绝、返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留班级管理日志脱敏和既有权限/业务错误映射。
- Xidian JSON body 边界验证命令：`gofmt -w backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/xidian ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go docs/backend-python-to-go-refactor.md`。
- Xidian JSON body 边界验证结果：xidian HTTP 层和公共 redact 定向测试通过；覆盖超大绑定完成 JSON body 被 `MaxBytesReader` 拒绝、返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留绑定/同步/快照和错误脱敏既有映射。
- Resource JSON body 边界验证命令：`gofmt -w backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/resource ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go docs/backend-python-to-go-refactor.md`。
- Resource JSON body 边界验证结果：resource HTTP 层和公共 redact 定向测试通过；覆盖超大 create/update JSON body 被 `MaxBytesReader` 拒绝、返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留资源权限、URL 校验、错误脱敏和 NotFound 既有映射。
- Resource 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/resource ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go docs/backend-python-to-go-refactor.md`。
- Resource 严格 JSON 验证结果：resource HTTP 层和公共 redact 定向测试通过；覆盖 create/update 请求首个合法 JSON 后追加第二个 JSON 时返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留 2MiB body 上限、资源权限、URL 校验、错误脱敏和 NotFound 既有映射。
- Xidian 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/xidian ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/xidian/handler_test.go docs/backend-python-to-go-refactor.md`。
- Xidian 严格 JSON 验证结果：xidian HTTP 层和公共 redact 定向测试通过；覆盖绑定完成请求首个合法 JSON 后追加第二个 JSON 时返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留 1MiB body 上限、绑定/同步/快照和错误脱敏既有映射。
- Classroom 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/classroom ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/classroom/handler_test.go docs/backend-python-to-go-refactor.md`。
- Classroom 严格 JSON 验证结果：classroom HTTP 层和公共 redact 定向测试通过；覆盖 create/join 请求首个合法 JSON 后追加第二个 JSON 时返回固定 400 `BAD_REQUEST` 且不调用 service，同时保留 1MiB body 上限、教师/学生权限、业务错误映射和错误脱敏既有行为。
- Auth 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/auth ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go docs/backend-python-to-go-refactor.md`。
- Auth 严格 JSON 验证结果：auth HTTP 层和公共 redact 定向测试通过；覆盖 login/register/change-password/forgot-password 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR`，同时保留 1MiB body 上限、CSRF、cookie、鉴权和错误脱敏既有行为。
- Session 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/session ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go docs/backend-python-to-go-refactor.md`。
- Session 严格 JSON 验证结果：session HTTP 层和公共 redact 定向测试通过；覆盖 start/chat/mode/batch-delete 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、SSE chat、鉴权、查询参数边界和错误脱敏既有行为。
- Exercise 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/exercise ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go docs/backend-python-to-go-refactor.md`。
- Exercise 严格 JSON 验证结果：exercise HTTP 层和公共 redact 定向测试通过；覆盖 submit 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、答案文本/图片路径校验、NotFound/BadRequest 映射和错误脱敏既有行为。
- Question 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/question ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go docs/backend-python-to-go-refactor.md`。
- Question 严格 JSON 验证结果：question HTTP 层和公共 redact 定向测试通过；覆盖 create/update/batch publish/batch import/ai-parse/generate-isomorphic 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 2MiB body 上限、教师权限、题目字段校验、AI 输入边界和错误脱敏既有行为。
- Knowledge 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/knowledge ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go docs/backend-python-to-go-refactor.md`。
- Knowledge 严格 JSON 验证结果：knowledge HTTP 层和公共 redact 定向测试通过；覆盖 create/update node/relation 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 2MiB body 上限、管理员权限、节点/关系字段校验、BadRequest/NotFound 映射和错误脱敏既有行为。
- Admin settings 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminsettings/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminsettings ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminsettings/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin settings 严格 JSON 验证结果：adminsettings HTTP 层和公共 redact 定向测试通过；覆盖 registration/general/database export 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、100MiB multipart 导入上限、管理员权限、BadRequest 映射和错误脱敏既有行为。
- Admin AI config 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminaiconfig/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminaiconfig ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminaiconfig/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin AI config 严格 JSON 验证结果：adminaiconfig HTTP 层和公共 redact 定向测试通过；覆盖 create/update provider、create provider with models、fetch models by credentials、update provider models、create/update model、update agent config 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、管理员权限、NotFound/Conflict 映射和错误脱敏既有行为。
- Admin user 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminuser ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin user 严格 JSON 验证结果：adminuser HTTP 层和公共 redact 定向测试通过；覆盖 create/status/update 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB JSON body 上限、5MiB CSV 导入上限、管理员权限、CSV 公式转义和错误脱敏既有行为。
- Admin inbox 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/admininbox ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin inbox 严格 JSON 验证结果：admininbox HTTP 层和公共 redact 定向测试通过；覆盖 review 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、管理员权限、审批动作校验、列表参数边界和错误脱敏既有行为。
- Security log 严格 JSON 验证命令：`gofmt -w backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/securitylog ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go docs/backend-python-to-go-refactor.md`。
- Security log 严格 JSON 验证结果：securitylog HTTP 层和公共 redact 定向测试通过；覆盖 delete/export/archive 请求首个合法 JSON 后追加第二个 JSON 时返回固定 422 `VALIDATION_ERROR` 且不调用 service，同时保留 1MiB body 上限、管理员权限、查询参数边界、BadRequest 映射和错误脱敏既有行为。
- HTTP JSON helper 重构验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/xidian/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/adapter/http/... ./internal/platform/redact -count=1`；`git diff --check -- backend-go/internal/platform/httpjson backend-go/internal/adapter/http docs/backend-python-to-go-refactor.md`。
- HTTP JSON helper 重构验证结果：公共 httpjson helper、HTTP adapter 全包和公共 redact 定向测试通过；扫描确认 HTTP adapter 中不再残留本地 `json.NewDecoder(http.MaxBytesReader(...))` 严格解析实现，所有 JSON helper 统一经 `httpjson.DecodeStrict` 处理，同时保留各包原有错误状态码、错误文案、body 上限和尾随 JSON 测试。
- 本地上传静态路径边界验证命令：`gofmt -w backend-go/internal/platform/httpserver/server.go backend-go/internal/platform/httpserver/server_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpserver ./internal/adapter/storage ./internal/application/upload -count=1`；`git diff --check -- backend-go/internal/platform/httpserver/server.go backend-go/internal/platform/httpserver/server_test.go docs/backend-python-to-go-refactor.md`。
- 本地上传静态路径边界验证结果：httpserver、storage adapter 和 upload application 定向测试通过；覆盖 `/uploads/images/*`、`/uploads/documents/*`、`/uploads/videos/*` 正常访问，文档附件 Content-Disposition 保持 attachment，同时拒绝 uploads 根目录文件、目录请求、`..`、编码路径、反斜杠、重复分隔符和 `.` 片段。
- 上传路径 helper 重构验证命令：`gofmt -w backend-go/internal/platform/uploadpath/uploadpath.go backend-go/internal/platform/uploadpath/uploadpath_test.go backend-go/internal/application/upload/service.go backend-go/internal/application/resource/service.go backend-go/internal/application/resource/service_test.go backend-go/internal/platform/httpserver/server.go backend-go/internal/platform/httpserver/server_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/uploadpath ./internal/application/upload ./internal/application/resource ./internal/application/session ./internal/application/exercise ./internal/platform/httpserver -count=1`；`git diff --check -- backend-go/internal/platform/uploadpath backend-go/internal/application/upload/service.go backend-go/internal/application/resource/service.go backend-go/internal/application/resource/service_test.go backend-go/internal/platform/httpserver/server.go backend-go/internal/platform/httpserver/server_test.go docs/backend-python-to-go-refactor.md`。
- 上传路径 helper 重构验证结果：公共 uploadpath、上传应用、资源应用、会话应用、练习应用和 httpserver 定向测试通过；扫描确认 httpserver 不再保留本地 `cleanUploadsPath`，resource 不再保留 `isSafeResourceUploadPath`，本地上传路径规则统一经 `uploadpath` 处理，同时保留 `upload.IsSafeImagePath` 兼容入口。
- HTTP query helper 重构验证命令：`gofmt -w backend-go/internal/platform/httpquery/int.go backend-go/internal/platform/httpquery/int_test.go backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/session ./internal/adapter/http/adminstats ./internal/adapter/http/securitylog -count=1`；`git diff --check -- backend-go/internal/platform/httpquery backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/session/handler_test.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go docs/backend-python-to-go-refactor.md`。
- HTTP query helper 重构验证结果：公共 httpquery、session/adminstats/securitylog HTTP 定向测试通过；覆盖 session history `limit` 超界和 security log `page_size` 超界被 HTTP 层 422 拒绝且不调用 service，同时保留各包原有错误状态码和错误文案。
- HTTP query helper 收敛补充验证命令：`gofmt -w backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/admininbox/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`。
- HTTP query helper 收敛补充验证结果：后端全量测试通过；adminuser、knowledge、mistake、question、resource、admininbox 的整数 query 解析统一改走 `httpquery.Int`，teacher 正整数分页解析在保留空白默认值语义的同时改走 `httpquery.Int`，扫描确认 HTTP adapter 中不再残留本地 `strconv.Atoi` 整数 query 解析实现，同时保留各包原有错误状态码、错误文案和范围校验。
- 前端资源批量导入 URL 边界验证命令：`npm test -- --run src/libs/utils/__tests__/safeUrl.test.ts src/pages/student/__tests__/ResourcesPage.test.ts`；`npm test -- --run`。
- 前端资源批量导入 URL 边界验证结果：safeUrl 与资源页面定向 Vitest 通过，前端全量 Vitest 14 files / 179 tests 通过；资源批量链接解析改为复用 `normalizeSafeHttpUrl`，只接受 http/https 或裸域名规范化结果，拒绝 `javascript:`、`data:`、`mailto:`、userinfo、协议相对 URL 和本地上传相对路径，同时资源打开逻辑复用同一 http/https 规范化 helper，减少前端 URL 边界重复实现。
- 前端图片预览对象 URL 生命周期验证命令：`npm test -- --run src/pages/student/SessionChatPage/hooks/__tests__/useImageUpload.test.ts src/libs/utils/__tests__/safeUrl.test.ts src/pages/student/__tests__/ResourcesPage.test.ts`；`npm test -- --run`。
- 前端图片预览对象 URL 生命周期验证结果：聊天图片上传 hook、安全 URL 和资源页面定向 Vitest 通过，前端全量 Vitest 15 files / 180 tests 通过；练习答题图片预览在替换、移除和组件卸载时回收旧 `blob:` 对象 URL，聊天图片上传 hook 改用 ref 跟踪当前预览 URL，追加新图片时不再提前 revoke 仍在使用的旧预览，删除、清空和卸载时统一回收，降低长会话内存泄漏和预览失效风险。
- 前端下载 helper 收敛验证命令：`npm test -- --run src/libs/utils/__tests__/download.test.ts src/pages/student/SessionChatPage/hooks/__tests__/useImageUpload.test.ts`；`npm test -- --run`；`git diff --check -- frontend/src/libs/utils/download.ts frontend/src/libs/utils/__tests__/download.test.ts frontend/src/pages/admin/AccountManagementPage.tsx frontend/src/pages/admin/SystemSettingsPage.tsx frontend/src/modules/admin/components/ImportUsersModal.tsx frontend/src/modules/admin/services/securityLogService.ts`。
- 前端下载 helper 收敛验证结果：下载工具与聊天图片上传 hook 定向 Vitest 通过，前端全量 Vitest 16 files / 182 tests 通过，whitespace check 仅保留 Windows LF/CRLF 提示；新增 `downloadBlob`/`sanitizeDownloadFilename`，统一管理员用户导出、数据库导出、用户导入模板下载和安全日志导出的 blob URL 创建、隐藏链接点击、finally 回收与文件名清洗，拒绝下载文件名中的控制字符、路径分隔符、Windows 保留字符和 bidi 控制字符，降低服务端文件名污染和导出下载资源泄漏风险。
- 外部 JSON 响应受限解码验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/integration/xidian/client.go backend-go/internal/integration/xidian/session.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/integration/xidian -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/integration/xidian/client.go backend-go/internal/integration/xidian/session.go`。
- 外部 JSON 响应受限解码验证结果：公共 httpjson、西电集成定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；新增 `httpjson.DecodeLimited`，在保留外部响应 unknown field 兼容性的同时限制最大响应体、拒绝尾随第二个 JSON 文档；西电验证码、滑块校验和通用 `getJSON` 响应解码统一改走 5MiB 上限，降低异常外部门户响应造成的内存压力和多文档解析歧义。
- 西电 HTML 响应受限读取验证命令：`gofmt -w backend-go/internal/integration/xidian/client.go backend-go/internal/integration/xidian/client_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/integration/xidian -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/integration/xidian/client.go backend-go/internal/integration/xidian/client_test.go`。
- 西电 HTML 响应受限读取验证结果：西电集成定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；新增 2MiB `readLimitedString` 边界，StartBinding 登录页、登录继续表单页和 401 错误页解析前统一限制外部门户 HTML/text 响应大小，补充超大登录页回归测试，降低异常门户响应造成的内存压力。
- AI provider 模型列表响应解码收敛验证命令：`gofmt -w backend-go/internal/application/adminaiconfig/service.go backend-go/internal/application/adminaiconfig/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminaiconfig ./internal/platform/httpjson -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/application/adminaiconfig/service.go backend-go/internal/application/adminaiconfig/service_test.go`。
- AI provider 模型列表响应解码收敛验证结果：adminaiconfig 应用层、公共 httpjson 定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；OpenAI-compatible `/v1/models` 响应从手写 `io.LimitReader` + `json.Unmarshal` 收敛到 `httpjson.DecodeLimited`，保留 4MiB 上限并拒绝尾随第二个 JSON 文档，补充 trailing response data 回归测试，降低异常 provider 响应造成的解析歧义。
- `httpjson.DecodeLimited` 总响应体上限收紧验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/application/adminaiconfig ./internal/integration/xidian -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go`。
- `httpjson.DecodeLimited` 总响应体上限收紧验证结果：公共 httpjson、adminaiconfig、西电集成定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；`DecodeLimited` 从 `io.LimitReader(max+1)` 改为计数 reader，超过上限时返回 `ErrBodyTooLarge`，补充超大 JSON 文档和“合法 JSON 后跟超量空白”测试，确保外部 JSON 响应的总字节数也受上限约束。
- 上传资源视频内容嗅探验证命令：`gofmt -w backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/upload ./internal/adapter/http/upload -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go`。
- 上传资源视频内容嗅探验证结果：上传应用层、上传 HTTP 层定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；资源文件内容校验从文档/文本扩展到视频容器，`video/mp4`/`video/quicktime` 要求 ISO BMFF `ftyp` 签名，`video/avi` 要求 RIFF/AVI 签名，`video/webm`/`video/x-matroska` 要求 EBML 签名，补充伪造 video/mp4 拒绝测试，避免任意非空内容冒充视频资源进入存储。
- 上传文本资源流式内容校验验证命令：`gofmt -w backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/upload ./internal/adapter/http/upload -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/application/upload/service.go backend-go/internal/application/upload/service_test.go`。
- 上传文本资源流式内容校验验证结果：上传应用层、上传 HTTP 层定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；`text/plain` 和 `text/markdown` 从仅校验前 512 字节扩展为存储读取过程中的流式 UTF-8/NUL 校验，补充“前缀为合法文本、后续夹带 NUL 二进制”拒绝测试，避免伪造文本资源绕过前缀检查后落入存储。
- 管理员用户 CSV 导入形状边界验证命令：`gofmt -w backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminuser -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go`。
- 管理员用户 CSV 导入形状边界验证结果：adminuser HTTP 定向测试和后端全量测试通过，whitespace check 仅保留 Windows LF/CRLF 提示；管理员用户 CSV 导入在 5MiB multipart 总量限制之外新增 5000 行、20 列和单字段 1000 rune 的解析边界，补充超量行、超量列和超长字段回归测试，避免极端 CSV 形状造成过多导入操作或解析压力。
- JWT segment 严格 JSON 边界验证命令：`gofmt -w backend-go/internal/application/auth/token.go backend-go/internal/application/auth/token_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/auth -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/application/auth/token.go backend-go/internal/application/auth/token_test.go docs/backend-python-to-go-refactor.md`。
- JWT segment 严格 JSON 边界验证结果：auth 应用层定向测试和后端全量测试通过；JWT header/payload base64 segment 解码在首个 JSON 文档后要求 EOF，补充“签名有效但 header 或 claims 追加第二个 JSON 文档”的回归测试，避免 token 解码边界与 HTTP/外部 JSON 严格单文档规则不一致。
- 对象存储 key 边界验证命令：`gofmt -w backend-go/internal/adapter/storage/local.go backend-go/internal/adapter/storage/local_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/storage -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/upload ./internal/adapter/http/upload ./internal/adapter/storage ./internal/platform/httpserver -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./...`；`git diff --check -- backend-go/internal/adapter/storage/local.go backend-go/internal/adapter/storage/local_test.go docs/backend-python-to-go-refactor.md`。
- 对象存储 key 边界验证结果：storage 适配层、上传应用/HTTP/静态服务组合测试和后端全量测试通过；本地/S3/七牛共用的 object key 清理入口在保留重复斜杠和 `.` 段归一化兼容的同时，新增 `images/documents/videos` 一级目录限制，并拒绝反斜杠、query/fragment 分隔符、百分号编码、Windows 盘符/ADS 冒号和控制字符，补充异常 key 回归测试，降低存储 URL 混淆和本地路径歧义风险。
- 前端认证缓存边界验证命令：`npm.cmd test -- --run src/modules/auth/store/authSlice.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/auth/store/authSlice.ts frontend/src/modules/auth/store/authSlice.test.ts docs/backend-python-to-go-refactor.md`。
- 前端认证缓存边界验证结果：authSlice 定向 Vitest、前端全量 Vitest（17 files / 185 tests）和生产构建通过；`auth_user_cache` 恢复时新增运行时白名单校验，只接受非空 `id/name`、`student/teacher/admin` 角色和类型正确的可选字段，非法或 malformed 缓存会被清理且不进入 Redux 初始用户状态，降低本地缓存篡改导致的 UI 角色污染风险；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端 tokenStorage 异常安全验证命令：`npm.cmd test -- --run src/libs/auth/tokenStorage.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/auth/tokenStorage.ts frontend/src/libs/auth/tokenStorage.test.ts docs/backend-python-to-go-refactor.md`。
- 前端 tokenStorage 异常安全验证结果：tokenStorage 定向 Vitest、前端全量 Vitest（18 files / 189 tests）和生产构建通过；`authTokenStorage.get/set/clear` 对 sessionStorage/localStorage 的读写和旧 localStorage token 清理均增加异常保护，存储被浏览器策略阻断时不再让初始化、登录态刷新或 401 清理链路抛出未处理异常，同时保持正常 sessionStorage token 读取和旧 token 清理行为。
- 前端主题缓存边界验证命令：`npm.cmd test -- --run src/store/slices/uiSlice.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/store/slices/uiSlice.ts frontend/src/store/slices/uiSlice.test.ts docs/backend-python-to-go-refactor.md`。
- 前端主题缓存边界验证结果：uiSlice 定向 Vitest、前端全量 Vitest（19 files / 193 tests）和生产构建通过；`localStorage.theme` 恢复时只接受 `light/dark`，非法值会清理并回落系统偏好，读写 localStorage 被浏览器策略阻断时不再让全局 UI state 初始化或主题切换抛出未处理异常，同时保留正常主题持久化行为；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端西电课表缓存边界验证命令：`npm.cmd test -- --run src/modules/classroom/store/classtableSlice.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/classroom/store/classtableSlice.ts frontend/src/modules/classroom/store/classtableSlice.test.ts docs/backend-python-to-go-refactor.md`。
- 前端西电课表缓存边界验证结果：classtableSlice 定向 Vitest、前端全量 Vitest（20 files / 197 tests）和生产构建通过；`xidian_classtable_cache` 恢复与同步写入前新增运行时结构校验，限制课表数组规模、学期长度、课程详情字段、上课节次、星期、周次标记和必要字符串字段，畸形缓存会被清理且不进入 Redux state，畸形同步响应返回“课表数据格式异常”并不写缓存，降低外部门户异常数据或本地缓存篡改造成页面状态污染和过大渲染压力的风险；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端我的班级教师 URL 边界验证命令：`npm.cmd test -- --run src/libs/utils/__tests__/safeUrl.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/utils/safeUrl.ts frontend/src/libs/utils/__tests__/safeUrl.test.ts frontend/src/pages/student/MyClassPage.tsx docs/backend-python-to-go-refactor.md`。
- 前端我的班级教师 URL 边界验证结果：safeUrl 定向 Vitest、前端全量 Vitest（20 files / 198 tests）和生产构建通过；`MyClassPage` 教师头像从直接渲染 `teacher_avatar_url` 改为仅允许规范化后的本地 `/uploads/images/...` 图片路径，教师邮箱链接从字符串拼接 `mailto:` 改为 `normalizeSafeMailtoUrl`，只接受普通邮箱地址并拒绝 `mailto:` 伪 URL、query/header 注入、控制字符和非邮箱输入，降低班级详情中服务端返回异常 URL 污染 DOM 的风险。
- 前端通用 localStorage Hook 边界验证命令：`npm.cmd test -- --run src/hooks/__tests__/useLocalStorage.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/hooks/useLocalStorage.ts frontend/src/hooks/__tests__/useLocalStorage.test.ts docs/backend-python-to-go-refactor.md`。
- 前端通用 localStorage Hook 边界验证结果：useLocalStorage 定向 Vitest、前端全量 Vitest（21 files / 204 tests）和生产构建通过；`useLocalStorage` 读取路径新增 storage 访问异常保护、畸形 JSON 清理和可选运行时 validator，写入/删除路径在浏览器阻断 localStorage 时不再抛出未处理异常，日志不再拼接原始 storage key，降低本地缓存篡改、解析歧义、隐私 key 泄漏和浏览器策略导致页面崩溃的风险；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端通用 localStorage Hook 交付物：`frontend/src/hooks/useLocalStorage.ts`、`frontend/src/hooks/__tests__/useLocalStorage.test.ts`。
- 前端 SSE 流式事件边界验证命令：`npm.cmd test -- --run src/libs/http/__tests__/sseClient.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/http/sseClient.ts frontend/src/libs/http/__tests__/sseClient.test.ts docs/backend-python-to-go-refactor.md`。
- 前端 SSE 流式事件边界验证结果：sseClient 定向 Vitest、前端全量 Vitest（22 files / 210 tests）和生产构建通过；`createSSEConnection` 对未完成行缓冲和单个 SSE data 事件新增 1MiB 上限，解析失败日志只记录事件名、dataLength 和错误对象，不再写入原始 data，事件 payload 运行时收敛为普通对象且只向回调传递字符串字段，畸形字段回落安全空值；`cancelTask` 对 task id path segment 进行 trim 和 `encodeURIComponent`，降低异常流响应造成前端内存压力、日志泄漏、回调类型污染和取消任务路径混淆的风险；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端 SSE 流式事件交付物：`frontend/src/libs/http/sseClient.ts`、`frontend/src/libs/http/__tests__/sseClient.test.ts`。
- 前端日期展示边界验证命令：`npm.cmd test -- --run src/libs/utils/__tests__/dateFormat.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/utils/dateFormat.ts frontend/src/libs/utils/__tests__/dateFormat.test.ts frontend/src/pages/student/MyClassPage.tsx docs/backend-python-to-go-refactor.md`。
- 前端日期展示边界验证结果：dateFormat 定向 Vitest、前端全量 Vitest（23 files / 213 tests）和生产构建通过；新增 `formatDateOrFallback` 统一处理空日期、非法日期和 date-fns pattern 异常，`MyClassPage` 的班级创建/加入日期从直接 `format(new Date(...))` 改为安全格式化并在异常数据下显示“未知”，避免服务端异常日期或缓存污染导致学生班级页渲染崩溃；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端日期展示交付物：`frontend/src/libs/utils/dateFormat.ts`、`frontend/src/libs/utils/__tests__/dateFormat.test.ts`、`frontend/src/pages/student/MyClassPage.tsx`。
- 前端聊天文档解析边界验证命令：`npm.cmd test -- --run src/libs/utils/__tests__/documentParser.test.ts`；`rg -n "mammoth" frontend/src/libs/utils frontend/src/libs/parsers -S`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/utils/documentParser.ts frontend/src/libs/utils/__tests__/documentParser.test.ts docs/backend-python-to-go-refactor.md`。
- 前端聊天文档解析边界验证结果：documentParser 定向 Vitest、前端全量 Vitest（24 files / 218 tests）和生产构建通过；`frontend/src/libs/utils/documentParser.ts` 移除静态 `mammoth` import，复用既有 `parseTxtFile` 和动态 `parseDocxFile`，减少 TXT/DOCX 解析重复实现并保留 DOCX 按需加载；解析后的附件文件名新增控制字符、路径分隔符、空白折叠和 120 字符上限规范化，文档内容统一 CRLF 换行并移除 NUL/不可见控制字符，`formatDocumentAsContext` 对传入的 ParsedDocument 再次执行文件名和内容清理，降低聊天附件文件名/内容污染 prompt 上下文、控制字符穿透和不必要首屏 bundle 压力；构建仍保留按需 DOCX 解析所需的 `vendor-mammoth` 懒加载 chunk，以及既有 Browserslist 数据过期和大 chunk 提示。
- 前端聊天文档解析交付物：`frontend/src/libs/utils/documentParser.ts`、`frontend/src/libs/utils/__tests__/documentParser.test.ts`。
- 前端题库导入批量边界验证命令：`npm.cmd test -- --run src/pages/teacher/QuestionBankPage/utils/__tests__/importBoundaries.test.ts`；`npm.cmd test -- --run src/libs/utils/__tests__/documentParser.test.ts src/pages/teacher/QuestionBankPage/utils/__tests__/importBoundaries.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/question/types/questionImport.ts frontend/src/pages/teacher/QuestionBankPage/components/QuestionImportModal.tsx frontend/src/pages/teacher/QuestionBankPage/utils/importBoundaries.ts frontend/src/pages/teacher/QuestionBankPage/utils/__tests__/importBoundaries.test.ts docs/backend-python-to-go-refactor.md`。
- 前端题库导入批量边界验证结果：importBoundaries 定向 Vitest、文档解析/导入边界组合 Vitest、前端全量 Vitest（25 files / 222 tests）和生产构建通过；新增 `MAX_PARSED_IMPORT_QUESTIONS=1000`，教师题库导入在本地解析后只让前 1000 道题进入预览并提示拆分文件，避免 10MB 文档解析出超大量题目拖垮预览/选择状态；AI 辅助识别复用已定义的 `MAX_AI_PARSE_TEXTS=10` 和 `MAX_AI_PARSE_TEXT_LENGTH=3000`，一次只处理前 10 道且只为实际处理的题目显示解析状态，降低误选大批题目造成的前端状态压力和请求体膨胀风险；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端题库导入批量边界交付物：`frontend/src/modules/question/types/questionImport.ts`、`frontend/src/pages/teacher/QuestionBankPage/components/QuestionImportModal.tsx`、`frontend/src/pages/teacher/QuestionBankPage/utils/importBoundaries.ts`、`frontend/src/pages/teacher/QuestionBankPage/utils/__tests__/importBoundaries.test.ts`。
- 后端题库导入/解析边界验证命令：`gofmt -w backend-go/internal/application/question/service.go backend-go/internal/application/question/service_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/tests/contract/frontend_route_surface_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/question ./internal/adapter/http/question -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1`；`git diff --check -- backend-go/internal/application/question/service.go backend-go/internal/application/question/service_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/tests/contract/frontend_route_surface_test.go docs/backend-python-to-go-refactor.md`。
- 后端题库导入/解析边界验证结果：题库应用层、题库 HTTP 层、前后端路由契约包和后端全量测试通过，whitespace check 仅报告 Windows LF/CRLF 提示；`question` 应用层新增共享边界常量并在服务入口兜底限制批量操作 ID 数、批量导入题数、AI raw_texts 数量/长度、解析器返回题数、题目 title/body/answer 长度、options/tags/hints/solution_steps/concept_ids 数量与单项长度、预计用时上限，避免绕过 HTTP handler 的内部调用把超大字段或数组送入仓储层；HTTP 层复用同一组常量并在调用 service 前拒绝超量 create/update/batch import/batch operation/ai-parse 请求；AI parser 返回超量题目或超大字段时回退确定性解析，避免异常模型输出扩大响应面；前端 route surface 契约改为忽略 `__tests__`、`.test.ts(x)` 和 `.spec.ts(x)` 测试桩，避免 SSE 单测中的 `/stream` 假 endpoint 污染真实前后端 API 覆盖面。
- 后端题库导入/解析边界交付物：`backend-go/internal/application/question/service.go`、`backend-go/internal/application/question/service_test.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/question/handler_test.go`、`backend-go/tests/contract/frontend_route_surface_test.go`。
- 后端题库导入/解析边界残余风险：本轮边界采用 Go 层字节长度和数组数量限制，未新增数据库 CHECK 约束；如后续产品需要按中文字符数、LaTeX 富文本结构或题型差异给出更细粒度提示，需要同步前端文案、后端校验和迁移约束。
- 题库旧 API 面清理验证命令：`rg -n "importQuestions\\(|downloadTemplate\\(|/questions/import|/questions/export|/questions/template|deprecated frontend helper" frontend/src backend-go/tests/contract`；`gofmt -w backend-go/tests/contract/route_surface_test.go backend-go/tests/contract/frontend_route_surface_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1`；`npm.cmd test -- --run src/pages/teacher/QuestionBankPage/utils/__tests__/importBoundaries.test.ts`；`npm.cmd test -- --run`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/question/services/questionService.ts backend-go/tests/contract/frontend_route_surface_test.go backend-go/tests/contract/route_surface_test.go docs/backend-python-to-go-refactor.md`。
- 题库旧 API 面清理验证结果：旧 `questionService.importQuestions/exportQuestions/downloadTemplate` 删除后，前端源码不再调用无 Go 路由承接的 `/questions/import`、`/questions/export`、`/questions/template`；前端 route surface 契约移除对应例外，仍通过真实 Go route 覆盖检查；`repoRoot` 从固定工作目录层级推断改为从当前目录和 `runtime.Caller` 路径向上查找 `backend-go/go.mod` 与 `frontend/package.json`，修复 `go test ./...` 在 Codex 沙盒中解析到不存在 cwd 镜像目录的问题；题库导入边界定向 Vitest、前端全量 Vitest（25 files / 222 tests）、前端生产构建、契约包和后端全量测试通过；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 题库旧 API 面清理交付物：`frontend/src/modules/question/services/questionService.ts`、`backend-go/tests/contract/frontend_route_surface_test.go`、`backend-go/tests/contract/route_surface_test.go`。
- 题库旧 API 面清理当时残余风险：`/auth/verify-email`、`/auth/bind-email` 和教师 assignments 相关前端-only/未迁移接口仍保留显式契约例外；教师 assignments 例外已在后续作业管理切片关闭，auth 邮箱例外已在 2026-07-03 邮箱验证/绑定 API 面清理切片关闭。
- 作业管理前端-only API 面清理验证命令：`rg -n "apiClient\\.(get|post|put|patch|delete).*teacher/assignments|BASE_PATH = '/teacher/assignments'|assignments/stats" frontend/src backend-go/tests/contract`；`gofmt -w backend-go/tests/contract/frontend_route_surface_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache go test ./... -count=1`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/modules/teacher/services/assignmentService.test.ts`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/teacher/services/assignmentService.ts frontend/src/modules/teacher/services/assignmentService.test.ts backend-go/tests/contract/frontend_route_surface_test.go docs/backend-python-to-go-refactor.md`。
- 作业管理前端-only API 面清理验证结果：`assignmentService` 从请求不存在的 `/teacher/assignments` 和 `/teacher/assignments/stats` 改为显式返回空列表/零统计，保留作业管理页面当前空状态但不再制造运行时 404 或错误日志；移除 route surface 契约中 `GET /teacher/assignments` 和 `GET /teacher/assignments/stats` 例外，契约包和后端全量测试通过；新增 assignmentService 单测并随前端全量 Vitest 执行通过（26 files / 224 tests）；前端生产构建通过，仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 作业管理前端-only API 面清理交付物：`frontend/src/modules/teacher/services/assignmentService.ts`、`frontend/src/modules/teacher/services/assignmentService.test.ts`、`backend-go/tests/contract/frontend_route_surface_test.go`。
- 作业管理前端-only API 面清理残余风险：页面仍保留作业管理入口和空状态，占位功能尚未迁移到 Go 后端；后续若要恢复真实作业管理，需要新增 Go 路由、契约和端到端测试，而不是直接恢复前端裸 API 调用。
- Auth 邮箱验证/绑定 API 面清理验证命令：`rg -n "apiClient\\.(get|post|put|patch|delete).*['\"]/auth/(bind-email|verify-email|verify-email-by-code)|fetch\\(.*['\"]/auth/(bind-email|verify-email|verify-email-by-code)|/auth/bind-email|/auth/verify-email-by-code" frontend/src backend-go/tests/contract`；`gofmt -w backend-go/tests/contract/route_surface_test.go backend-go/tests/contract/frontend_route_surface_test.go`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/modules/auth/services/authService.test.ts`；`npm.cmd run build`。
- Auth 邮箱验证/绑定 API 面清理验证结果：前端源码不再向 `/auth/bind-email`、`/auth/verify-email` 或 `/auth/verify-email-by-code` 发起 HTTP 请求；`bindEmail` 改为本地显式抛出“邮箱绑定与验证功能暂未接入”，无调用方的 `verifyEmailByCode` 旧 helper 删除，注册成功页和个人中心邮箱弹窗都改为清晰的未接入/可直接登录提示；无路由引用的 `VerifyEmailPage` 删除，避免保留永远失败的邮箱链接验证入口；前端 route surface 契约移除 auth 邮箱三条例外且契约包通过。新增 `authService` 单测覆盖本地拒绝路径并断言不会调用 `apiClient`；本轮 Vitest 命令在当前配置下执行前端全量测试（27 files / 225 tests）并通过，后端全量测试和前端生产构建通过；构建仅保留 Browserslist 数据过期和既有大 chunk 提示。`repoRoot` 增加 `MSP_REPO_ROOT`/`REPO_ROOT`/`PROJECT_ROOT` 候选入口，用于 Codex 沙盒只映射 `backend-go` 模块时仍能定位真实仓库根；正常工作区/CI 仍保留自动向上查找 `backend-go/go.mod` 与 `frontend/package.json` 的路径。
- Auth 邮箱验证/绑定 API 面清理交付物：`frontend/src/modules/auth/services/authService.ts`、`frontend/src/modules/auth/services/authService.test.ts`、`frontend/src/modules/auth/components/RegisterForm.tsx`、`frontend/src/pages/common/ProfilePage.tsx`、`backend-go/tests/contract/frontend_route_surface_test.go`、`backend-go/tests/contract/route_surface_test.go`。
- Auth 邮箱验证/绑定 API 面清理残余风险：邮箱绑定、邮件链接验证和验证码验证仍未在 Go 后端实现；当前产品行为是显式未接入且不再请求不存在接口。后续如恢复邮箱验证能力，需要新增 Go 路由、邮件/验证码存储、频率限制、防枚举策略、过期/重放保护、契约测试和端到端验证，而不是恢复前端裸 API 调用。
- 前端 CSRF cookie 解析边界验证命令：`rg -n -e "dangerouslySetInnerHTML" -e "innerHTML" -e "window\\.open\\(" -e 'target="_blank"' -e "target='_blank'" -e "eval\\(" -e "new Function\\(" -e "document\\.cookie" frontend/src -g "*.ts" -g "*.tsx"`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/libs/auth/csrfToken.test.ts`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/auth/csrfToken.ts frontend/src/libs/auth/csrfToken.test.ts docs/backend-python-to-go-refactor.md`。
- 前端 CSRF cookie 解析边界验证结果：`getCsrfToken` 对空值、超过 4096 字符的值和畸形 percent-encoding 返回 `null`，`csrfHeader` 在这些情况下返回空对象，避免异常 cookie 让 axios request interceptor 在构造变更类请求头时抛出未处理异常；新增单测覆盖正常 decode、多 cookie、畸形编码和超长 token。前端危险 DOM/打开链接扫描确认剩余 `target="_blank"` 均带 `noopener noreferrer` 且聊天/Markdown/资源链接走安全 URL 规范化；CSRF 定向 Vitest 命令在当前配置下执行前端全量测试（28 files / 228 tests）并通过；前端生产构建通过，仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端 CSRF cookie 解析边界交付物：`frontend/src/libs/auth/csrfToken.ts`、`frontend/src/libs/auth/csrfToken.test.ts`。
- 前端西电旧凭证清理边界验证命令：`rg -n "credentialStorage|clearCredential|saveCredential|getCredential|localStorage" frontend/src/modules/xidian frontend/src/pages/common/ProfilePage.tsx -g "*.ts" -g "*.tsx"`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/modules/xidian/services/credentialStorage.test.ts`；`npm.cmd run build`；`git diff --check -- frontend/src/modules/xidian/services/credentialStorage.ts frontend/src/modules/xidian/services/credentialStorage.test.ts docs/backend-python-to-go-refactor.md`。
- 前端西电旧凭证清理边界验证结果：`credentialStorage` 仍保持“不再持久化西电密码、调用时清理旧版 `xidian_cred`”的兼容语义，但 `clearCredential` 在 `localStorage` 不存在或浏览器策略阻断 removeItem 时不再抛出异常，避免打开绑定弹窗、解绑和重新验证流程被旧缓存清理失败打断；新增单测覆盖旧凭证删除、不持久化新凭证、storage blocked 时 `clearCredential/saveCredential/loadCredential/hasCredential` 均无抛出。定向 Vitest 命令在当前配置下执行前端全量测试（29 files / 230 tests）并通过；前端生产构建通过，仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端西电旧凭证清理边界交付物：`frontend/src/modules/xidian/services/credentialStorage.ts`、`frontend/src/modules/xidian/services/credentialStorage.test.ts`。
- 前端资源 URL 展示边界验证命令：`rg -n "decodeURIComponent\\(|atob\\(|new URLSearchParams\\(|window\\.open\\(" frontend/src -g "*.ts" -g "*.tsx"`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/pages/student/__tests__/ResourcesPage.test.ts`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/utils/resourceUtils.ts frontend/src/pages/student/__tests__/ResourcesPage.test.ts docs/backend-python-to-go-refactor.md`。
- 前端资源 URL 展示边界验证结果：`extractTitleFromUrl` 将 URL 路径段 decode 收敛为安全解码，畸形 percent-encoding 不再让资源批量导入标题退化为整段 URL，而是回退到清理后的原始路径段；资源标题去除控制字符并限制为 100 字符。`getInitialResourceSearch` 对资源页地址栏 query 总长度设置 4096 字符上限，并将初始搜索词清理控制字符、trim 后限制为 100 字符，避免异常长地址栏参数放大页面初始状态和搜索输入压力。资源页定向 Vitest 命令在当前配置下执行前端全量测试（29 files / 233 tests）并通过；前端生产构建通过，仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端资源 URL 展示边界交付物：`frontend/src/libs/utils/resourceUtils.ts`、`frontend/src/pages/student/__tests__/ResourcesPage.test.ts`。
- 前端导出 base64 解码边界验证命令：`rg -n "atob\\(|base64ToBlob|导出文件内容异常" frontend/src -g "*.ts" -g "*.tsx"`；`npm.cmd exec vitest run --root E:/code/msp-go/frontend src/libs/utils/__tests__/download.test.ts`；`npm.cmd run build`；`git diff --check -- frontend/src/libs/utils/download.ts frontend/src/libs/utils/__tests__/download.test.ts frontend/src/pages/admin/SystemSettingsPage.tsx frontend/src/modules/admin/services/securityLogService.ts docs/backend-python-to-go-refactor.md`。
- 前端导出 base64 解码边界验证结果：新增 `base64ToBlob` 统一处理导出文件 base64 响应，先移除空白、限制 encoded 长度为 64MiB、校验 base64 字符和长度余数，再捕获 `atob` 失败并返回固定“导出文件内容异常”；系统设置数据库导出和安全日志导出复用该 helper，移除重复的手写 `atob`/`Uint8Array` 转换，避免后端异常响应或代理污染造成浏览器 `InvalidCharacterError` 直冒。下载工具测试新增合法 JSON blob、畸形 base64、非法长度和超长内容覆盖；定向 Vitest 命令在当前配置下执行前端全量测试（29 files / 235 tests）并通过；前端生产构建通过，仅保留 Browserslist 数据过期和既有大 chunk 提示。
- 前端导出 base64 解码边界交付物：`frontend/src/libs/utils/download.ts`、`frontend/src/libs/utils/__tests__/download.test.ts`、`frontend/src/pages/admin/SystemSettingsPage.tsx`、`frontend/src/modules/admin/services/securityLogService.ts`。
- 西电 UUID 生成 panic 边界验证命令：`gofmt -w backend-go/internal/application/xidian/uuid.go backend-go/internal/application/xidian/service.go backend-go/internal/application/xidian/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/xidian -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/application/xidian/uuid.go backend-go/internal/application/xidian/service.go backend-go/internal/application/xidian/service_test.go docs/backend-python-to-go-refactor.md`。
- 西电 UUID 生成 panic 边界验证结果（2026-07-03）：`newUUID` 从 `panic` 改为显式返回错误，`Service.newID` 与开始绑定、完成绑定、同步保存快照三处调用点统一改为 `func() (string, error)`；当系统随机源失败时，业务路径返回错误并交由 HTTP 层按既有 500 fallback 处理，不再让请求 goroutine 直接崩溃。补充回归测试覆盖 `StartBinding`、`CompleteBinding` 和 `SyncScores` 的 ID 生成失败路径；同步路径在生成快照 ID 失败时不会先更新门户 cookie，减少失败路径的半截副作用。西电应用层定向测试和后端全量测试通过。
- 西电 UUID 生成 panic 边界交付物：`backend-go/internal/application/xidian/uuid.go`、`backend-go/internal/application/xidian/service.go`、`backend-go/internal/application/xidian/service_test.go`。
- 西电 UUID 生成 panic 边界残余风险：本轮只收敛西电应用层 UUID 生成失败边界；代码库内仍存在少量独立的随机数/UUID 生成入口，后续应继续逐包审计并按业务边界决定是否返回错误、降级或启动期失败。
- 管理后台统计展示 ID 去随机化验证命令：`rg -n "newID|newUUID|rand\\.|crypto/rand|activity-" backend-go/internal/application/adminstats -g "*.go"`；`gofmt -w backend-go/internal/application/adminstats/service.go backend-go/internal/application/adminstats/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/adminstats -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/application/adminstats/service.go backend-go/internal/application/adminstats/service_test.go docs/backend-python-to-go-refactor.md`。
- 管理后台统计展示 ID 去随机化验证结果（2026-07-03）：`adminstats` 最近活动和系统告警的展示 ID 不再依赖 `crypto/rand`，删除随机 UUID helper 与 `activity-<UnixNano>` 弱随机 fallback；最近活动 ID 改为基于用户 ID 的稳定 `user-*`，系统告警 ID 改为基于服务名与状态的稳定 `service-*`，正常状态告警固定为 `system-ok`。新增/更新单测断言活动和告警 ID 输出；检索确认 `adminstats` 包不再残留随机 ID 生成，adminstats 定向测试和后端全量测试通过。
- 管理后台统计展示 ID 去随机化交付物：`backend-go/internal/application/adminstats/service.go`、`backend-go/internal/application/adminstats/service_test.go`。
- 管理后台统计展示 ID 去随机化残余风险：本轮仅处理 dashboard 展示用 ID，不改变数据库用户 ID 或接口排序语义；若未来前端把这些展示 ID 用作可持久引用，需要升级为后端明确的资源标识并补充契约约束，而不是继续依赖派生展示键。
- 请求 ID 随机失败 fallback 边界验证命令：`gofmt -w backend-go/internal/platform/middleware/middleware.go backend-go/internal/platform/middleware/middleware_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/middleware -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/middleware/middleware.go backend-go/internal/platform/middleware/middleware_test.go docs/backend-python-to-go-refactor.md`。
- 请求 ID 随机失败 fallback 边界验证结果（2026-07-03）：`newRequestID` 保持优先使用 16 字节随机值；当随机源返回错误时，不再把 RFC3339Nano 时间字符串 hex 编码为请求 ID，而是使用纳秒时间戳加原子递增序号组成 32 位十六进制 fallback，降低极端并发下的 request id 碰撞风险并保持响应不中断。新增单测注入随机源失败，断言连续 fallback request id 非空、长度稳定且不重复；middleware 定向测试和后端全量测试通过。
- 请求 ID 随机失败 fallback 边界交付物：`backend-go/internal/platform/middleware/middleware.go`、`backend-go/internal/platform/middleware/middleware_test.go`。
- 请求 ID 随机失败 fallback 边界残余风险：fallback request id 仍不是安全令牌，不应用于认证、授权、重放保护或外部可验证资源标识；它只用于日志关联和响应头追踪。
- UUID 生成 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/identifier/uuid.go backend-go/internal/platform/identifier/uuid_test.go backend-go/internal/adapter/postgres/uuid.go backend-go/internal/adapter/postgres/user_repository.go backend-go/internal/application/xidian/uuid.go backend-go/internal/application/session/uuid.go backend-go/internal/application/exercise/uuid.go backend-go/internal/application/adminaiconfig/uuid.go backend-go/internal/application/upload/service.go backend-go/internal/application/teacher/service.go`；`rg -n -e 'fmt\\.Sprintf\\("%08x-%04x-%04x-%04x-%012x' -e 'func newUUID\\(' -e 'func NewUUID\\(' backend-go/internal -g '*.go'`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/identifier ./internal/application/session ./internal/application/exercise ./internal/application/adminaiconfig ./internal/application/upload ./internal/application/teacher ./internal/application/xidian ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/identifier/uuid.go backend-go/internal/platform/identifier/uuid_test.go backend-go/internal/adapter/postgres/uuid.go backend-go/internal/adapter/postgres/user_repository.go backend-go/internal/application/xidian/uuid.go backend-go/internal/application/session/uuid.go backend-go/internal/application/exercise/uuid.go backend-go/internal/application/adminaiconfig/uuid.go backend-go/internal/application/upload/service.go backend-go/internal/application/teacher/service.go docs/backend-python-to-go-refactor.md`。
- UUID 生成 helper 收敛验证结果（2026-07-04）：新增 `backend-go/internal/platform/identifier.NewUUID` 作为统一 RFC 4122 v4 UUID 生成入口，使用 `io.ReadFull` 读取 16 字节随机源并集中设置 version/variant 位，补充确定性格式、随机源错误和短读错误单测；`session`、`exercise`、`adminaiconfig`、`xidian`、`upload`、`teacher` 和 PostgreSQL adapter 现有 UUID wrapper/default factory 均改为复用公共 helper，删除多处手写 `crypto/rand` + `fmt.Sprintf` 重复实现。检索确认 UUID 格式化模板只保留在 `identifier` 包，保留各业务包函数名以避免 API/测试调用面震荡；定向测试和后端全量测试通过。
- UUID 生成 helper 收敛交付物：`backend-go/internal/platform/identifier/uuid.go`、`backend-go/internal/platform/identifier/uuid_test.go`、`backend-go/internal/adapter/postgres/uuid.go`、`backend-go/internal/adapter/postgres/user_repository.go`、`backend-go/internal/application/xidian/uuid.go`、`backend-go/internal/application/session/uuid.go`、`backend-go/internal/application/exercise/uuid.go`、`backend-go/internal/application/adminaiconfig/uuid.go`、`backend-go/internal/application/upload/service.go`、`backend-go/internal/application/teacher/service.go`。
- UUID 生成 helper 收敛残余风险：本轮只收敛 RFC 4122 v4 UUID 生成；CSRF token、refresh token、管理员信箱重置码、班级邀请码和请求 ID 等非 UUID 随机值仍按各自安全语义保留独立实现，后续若继续统一随机字节/令牌 helper，需要逐项验证长度、编码、可预测性和兼容性。
- 安全随机字符串 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/securerand/string.go backend-go/internal/platform/securerand/string_test.go backend-go/internal/application/admininbox/service.go backend-go/internal/application/classroom/service.go`；`rg -n "crypto/rand|math/big|rand\\.Int|randomByte|shufflePassword|generateClassCode" backend-go/internal/application/admininbox backend-go/internal/application/classroom backend-go/internal/platform/securerand -g "*.go"`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/securerand ./internal/application/admininbox ./internal/application/classroom -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/securerand/string.go backend-go/internal/platform/securerand/string_test.go backend-go/internal/application/admininbox/service.go backend-go/internal/application/classroom/service.go docs/backend-python-to-go-refactor.md`。
- 安全随机字符串 helper 收敛验证结果（2026-07-04）：新增 `backend-go/internal/platform/securerand`，集中提供基于 `crypto/rand.Reader` 的 ASCII 字符集随机取字节、随机字符串和 Fisher-Yates 字节洗牌；索引生成采用拒绝采样避免 modulo bias，补充确定性输出、非法长度/空字符集/超大字符集、拒绝采样、随机源错误和洗牌顺序测试。管理员信箱临时密码保留“至少包含大小写、数字和特殊字符后再洗牌”的强度语义，班级邀请码保留 6 位大写字母/数字语义，两处均删除本地 `rand.Int`/`math/big` 重复实现；定向测试和后端全量测试通过。
- 安全随机字符串 helper 收敛交付物：`backend-go/internal/platform/securerand/string.go`、`backend-go/internal/platform/securerand/string_test.go`、`backend-go/internal/application/admininbox/service.go`、`backend-go/internal/application/classroom/service.go`。
- 安全随机字符串 helper 收敛残余风险：本轮只收敛 ASCII 字符集随机字符串和密码洗牌；JWT/refresh token、CSRF token、Fernet key、S3/Qiniu 签名和请求 ID 等二进制令牌或签名路径仍保留各自实现，后续若继续统一随机字节 helper，需要避免改变令牌长度、编码和兼容语义。
- 随机 hex token helper 收敛验证命令：`gofmt -w backend-go/internal/platform/securerand/string.go backend-go/internal/platform/securerand/string_test.go backend-go/internal/application/auth/token.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/securerand ./internal/application/auth ./internal/adapter/http/auth -count=1`；`rg -n "randomHex|crypto/rand|encoding/hex|securerand.Hex" backend-go/internal/application/auth backend-go/internal/adapter/http/auth backend-go/internal/platform/securerand -g "*.go"`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/securerand/string.go backend-go/internal/platform/securerand/string_test.go backend-go/internal/application/auth/token.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/auth/handler_test.go docs/backend-python-to-go-refactor.md`。
- 随机 hex token helper 收敛验证结果（2026-07-04）：`securerand` 新增 `Bytes` 和 `Hex`，统一用 `io.ReadFull` 读取随机源并返回小写 hex 文本，补充确定性字节/hex、负长度和随机源错误测试；JWT `jti` 从本地 `randomHex(16)` 改为 `securerand.Hex(16)`，CSRF cookie 从 HTTP handler 本地 `rand.Read` + `hex.EncodeToString` 改为 `securerand.Hex(32)`，删除两处认证域重复随机 hex 实现。回归测试确认 access token `jti` 仍为 32 个 hex 字符，登录/刷新设置的 CSRF cookie 仍为 64 个 hex 字符；定向测试和后端全量测试通过。
- 随机 hex token helper 收敛交付物：`backend-go/internal/platform/securerand/string.go`、`backend-go/internal/platform/securerand/string_test.go`、`backend-go/internal/application/auth/token.go`、`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/adapter/http/auth/handler_test.go`。
- 随机 hex token helper 收敛残余风险：本轮只收敛认证域中“随机字节转 hex 文本”的内部实现，不改变 JWT 签名、refresh token 结构、cookie 属性或 CSRF 校验语义；Fernet key、S3/Qiniu 签名、请求 ID fallback 和非 hex 编码令牌仍保留各自实现。
- 配置列表解析收敛验证命令：`gofmt -w backend-go/internal/platform/config/config.go backend-go/internal/platform/config/config_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/config -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/config/config.go backend-go/internal/platform/config/config_test.go docs/backend-python-to-go-refactor.md`。
- 配置列表解析收敛验证结果（2026-07-04）：`envList` 拆分为 `parseEnvList`、严格 JSON 数组解析、逗号列表拆分和统一 fallback 小函数；JSON 列表使用 `json.Decoder` 确认无尾随 JSON 数据，普通逗号列表先清理空白再剥离外层引号/方括号并丢弃空项。新增测试覆盖 CORS/管理 CIDR 的环境变量集成、JSON 数组 trim、空 JSON 列表回退、畸形 JSON 列表兼容旧逗号拆分，以及合法 JSON 数组后追加数据时回退默认值；config 定向测试和后端全量测试通过。
- 配置列表解析收敛交付物：`backend-go/internal/platform/config/config.go`、`backend-go/internal/platform/config/config_test.go`。
- 配置列表解析收敛残余风险：本轮只规范进程启动期的字符串列表配置，不改变配置键名、默认值或 CORS/管理 CIDR 校验策略；JSON 数组中的空白项现在会和逗号列表一样被 trim 后丢弃，若某个部署依赖空字符串作为有效列表项，需要改为显式配置有效值。
- HTTP 查询列表解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpquery/list.go backend-go/internal/platform/httpquery/list_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/question ./internal/adapter/http/securitylog -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg 'strings\\.Split\\(value, ","\\)' backend-go/internal/adapter/http backend-go/internal/platform/httpquery -n`；`git diff --check -- backend-go/internal/platform/httpquery/list.go backend-go/internal/platform/httpquery/list_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/securitylog/handler_test.go docs/backend-python-to-go-refactor.md`。
- HTTP 查询列表解析 helper 收敛验证结果（2026-07-04）：`httpquery` 新增 `StringList` 和 `NamedStringList`，统一处理重复 query 值、逗号分隔值、空白 trim 和空项丢弃；question 列表筛选的 `tags/tags[]` 与 securitylog 的 `event_types/severities` 删除本地重复拆分循环并复用公共 helper。新增 `httpquery` 单测覆盖重复值、逗号拆分、空输入和 `name[]` 数组风格参数；强化 question/securitylog HTTP 集成测试，断言筛选值顺序和内容。扫描确认 HTTP adapter 中的 `strings.Split(value, ",")` 已收敛到 `httpquery` helper；定向测试和后端全量测试通过。
- HTTP 查询列表解析 helper 收敛交付物：`backend-go/internal/platform/httpquery/list.go`、`backend-go/internal/platform/httpquery/list_test.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/question/handler_test.go`、`backend-go/internal/adapter/http/securitylog/handler.go`、`backend-go/internal/adapter/http/securitylog/handler_test.go`。
- HTTP 查询列表解析 helper 收敛残余风险：本轮只收敛 HTTP 查询字符串中的非校验型字符串列表解析，不改变 question/securitylog 的分页、日期、枚举合法性或 service 层校验语义；若后续更多 handler 需要数组风格参数，应优先复用 `httpquery.NamedStringList` 并补对应路由测试。
- 配置 duration 解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/config/config.go backend-go/internal/platform/config/config_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/config -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func envSeconds|func envMilliseconds|func envDuration|ParseDuration|ParseFloat\\(value" backend-go/internal/platform/config/config.go -n`；`git diff --check -- backend-go/internal/platform/config/config.go backend-go/internal/platform/config/config_test.go docs/backend-python-to-go-refactor.md`。
- 配置 duration 解析 helper 收敛验证结果（2026-07-04）：`envSeconds` 和 `envMilliseconds` 保留原入口，内部统一委托 `envDuration(key, fallback, unit)`；Go duration 字符串仍优先由 `time.ParseDuration` 解析，纯数字仍按调用入口分别解释为秒或毫秒，非法值和空值继续回退默认配置。新增单测覆盖数字秒、duration 字符串秒入口、数字毫秒、duration 字符串毫秒入口、非法值和空值回退；扫描确认 duration 解析中的 `ParseDuration`/数值单位换算已收敛到 `envDuration`，config 定向测试和后端全量测试通过。
- 配置 duration 解析 helper 收敛交付物：`backend-go/internal/platform/config/config.go`、`backend-go/internal/platform/config/config_test.go`。
- 配置 duration 解析 helper 收敛残余风险：本轮只减少配置加载内部重复，不改变任何环境变量名、默认超时、校验阈值或部署配置格式；纯数字 duration 仍沿用原来的秒/毫秒入口语义，后续若新增分钟或字节等单位配置应避免复用该 helper 造成语义混淆。
- HTTP Bearer token 解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpauth/bearer.go backend-go/internal/platform/httpauth/bearer_test.go backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/progress/handler.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/upload/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpauth ./internal/adapter/http/session ./internal/adapter/http/mistake ./internal/adapter/http/progress ./internal/adapter/http/portrait ./internal/adapter/http/xidian ./internal/adapter/http/exercise ./internal/adapter/http/upload -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg 'strings\\.Fields\\(authHeader\\)' backend-go/internal/adapter/http backend-go/internal/platform/httpauth -n`；`git diff --check -- backend-go/internal/platform/httpauth backend-go/internal/adapter/http/session/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/progress/handler.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/upload/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP Bearer token 解析 helper 收敛验证结果（2026-07-04）：新增 `backend-go/internal/platform/httpauth.BearerToken`，集中处理 `Authorization` header trim、字段数量校验和大小写不敏感的 `Bearer` scheme 判断，补充缺失 header、大小写 scheme、额外空白、错误 scheme、缺失 token 和多字段 token 单测。session、mistake、progress、portrait、xidian、exercise、upload 七个 HTTP adapter 的 principal 解析入口改为复用该 helper，保留各自 `WWW-Authenticate`、未认证响应体和权限判断逻辑；定向测试和后端全量测试通过。
- HTTP Bearer token 解析 helper 收敛交付物：`backend-go/internal/platform/httpauth/bearer.go`、`backend-go/internal/platform/httpauth/bearer_test.go`、`backend-go/internal/adapter/http/session/handler.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/progress/handler.go`、`backend-go/internal/adapter/http/portrait/handler.go`、`backend-go/internal/adapter/http/xidian/handler.go`、`backend-go/internal/adapter/http/exercise/handler.go`、`backend-go/internal/adapter/http/upload/handler.go`。
- HTTP Bearer token 解析 helper 收敛残余风险：本轮只收敛 7 个同构 principal 入口的 header 解析，不改变 JWT 解码、角色鉴权、CSRF、cookie 或错误响应格式；admin、teacher、knowledge、question、resource、securitylog、classroom、auth 等剩余 adapter 仍有手写 `strings.Fields(authHeader)`，后续可分阶段继续替换并跑各包路由测试。
- HTTP Bearer token 解析剩余 adapter 收敛验证命令：`gofmt -w backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/teacher/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpauth ./internal/adapter/http/... -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg 'strings\\.Fields\\(authHeader\\)' backend-go/internal/adapter/http backend-go/internal/platform/httpauth -n`；`git diff --check -- backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/teacher/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP Bearer token 解析剩余 adapter 收敛验证结果（2026-07-04）：adminaiconfig、admininbox、adminsettings、adminstats、adminuser、auth、classroom、knowledge、question、resource、securitylog、teacher 十二个剩余 HTTP adapter 的 Bearer header 解析改为复用 `httpauth.BearerToken`；各 handler 仍保留自身 JWT 解码入口、角色判断、`WWW-Authenticate` header 和错误响应格式。扫描确认 `backend-go/internal/adapter/http` 下不再直接调用 `strings.Fields(authHeader)`，该解析逻辑只保留在 `platform/httpauth`；HTTP adapter 全包定向测试和后端全量测试通过。
- HTTP Bearer token 解析剩余 adapter 收敛交付物：`backend-go/internal/adapter/http/adminaiconfig/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/adminsettings/handler.go`、`backend-go/internal/adapter/http/adminstats/handler.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/adapter/http/classroom/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`、`backend-go/internal/adapter/http/teacher/handler.go`。
- HTTP Bearer token 解析剩余 adapter 收敛残余风险：本轮只统一 header 解析层，不合并各 adapter 的 principal/role helper 或错误响应结构；未来若继续抽象认证中间件，需要先统一 handler 级错误契约和角色策略，避免改变现有前端可见错误码。
- HTTP 分页查询解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpquery/pagination.go backend-go/internal/platform/httpquery/pagination_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/question ./internal/adapter/http/resource ./internal/adapter/http/securitylog -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func parseIntQuery|httpquery\\.Pagination|PaginationError" backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/platform/httpquery -n`；`git diff --check -- backend-go/internal/platform/httpquery/pagination.go backend-go/internal/platform/httpquery/pagination_test.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/question/handler_test.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/resource/handler_test.go backend-go/internal/adapter/http/securitylog/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP 分页查询解析 helper 收敛验证结果（2026-07-04）：`httpquery` 新增 `Pagination`、`PaginationParams` 和结构化 `PaginationError`，统一解析 `page`/`page_size`、默认页码 1、默认 page_size、最大 page_size 和范围错误；新增单测覆盖默认值、显式值、非法整数、page 越界和 page_size 越界。question、resource、securitylog 三个 handler 删除本地 page/page_size `parseIntQuery` + 手写范围判断，改为复用公共 helper，并保留原有中文错误文案和 422 `VALIDATION_ERROR` 响应；question/resource 新增路由测试确认非法分页不会调用 service。定向测试和后端全量测试通过。
- HTTP 分页查询解析 helper 收敛交付物：`backend-go/internal/platform/httpquery/pagination.go`、`backend-go/internal/platform/httpquery/pagination_test.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/question/handler_test.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/resource/handler_test.go`、`backend-go/internal/adapter/http/securitylog/handler.go`。
- HTTP 分页查询解析 helper 收敛残余风险：本轮先收敛 question/resource/securitylog 三个代表性列表入口；adminuser、admininbox、knowledge、mistake 等仍保留同类本地分页解析，后续可继续逐包替换，并在替换时确保各自默认 page_size 和错误文案保持兼容。
- HTTP 分页查询解析 helper 扩展验证命令：`gofmt -w backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/adminuser ./internal/adapter/http/admininbox ./internal/adapter/http/knowledge ./internal/adapter/http/mistake -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func parseIntQuery|httpquery\\.Pagination|PaginationError" backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go -n`；`git diff --check -- backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminuser/handler_test.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/admininbox/handler_test.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/knowledge/handler_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go docs/backend-python-to-go-refactor.md`。
- HTTP 分页查询解析 helper 扩展验证结果（2026-07-04）：adminuser、admininbox、knowledge、mistake 四个 HTTP adapter 的 `page`/`page_size` 解析改为复用 `httpquery.Pagination`，删除各自本地 `parseIntQuery` 和手写 page/page_size 范围判断；保留 adminuser 默认 page_size=10，其余三个默认 page_size=20，并保留原有中文错误文案和 422 `VALIDATION_ERROR` 响应。四个包新增非法分页路由测试，确认错误分页不会调用 service；扫描确认这四个 handler 中不再定义本地 `parseIntQuery`，定向测试和后端全量测试通过。
- HTTP 分页查询解析 helper 扩展交付物：`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/adminuser/handler_test.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/admininbox/handler_test.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/knowledge/handler_test.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/mistake/handler_test.go`。
- HTTP 分页查询解析 helper 扩展残余风险：本轮只覆盖同构 page/page_size 列表入口；teacher 的正整数 page/page_size、session 的 limit/offset bounded 查询和 adminstats 的 limit 仍按不同参数语义保留本地解析，后续若收敛应使用更贴合各自字段名和范围的 helper，避免把分页语义误套到非分页参数。
- Admin stats limit 查询解析收敛验证命令：`gofmt -w backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminstats/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/adminstats -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/adminstats/handler_test.go docs/backend-python-to-go-refactor.md`。
- Admin stats limit 查询解析收敛验证结果（2026-07-04）：`/admin/stats/recent-activities` 的 `limit` 解析改为直接复用 `httpquery.BoundedInt`，删除本地 `parseIntQuery` 包装和分散范围判断；保留默认 limit=10、合法范围 1..50、非法整数与越界两类原有中文错误文案和 422 `VALIDATION_ERROR` 响应。HTTP 测试追加断言 `limit=bad`、`limit=0` 均在路由层拒绝且不会调用 service；定向测试和后端全量测试通过。
- Admin stats limit 查询解析收敛交付物：`backend-go/internal/adapter/http/adminstats/handler.go`、`backend-go/internal/adapter/http/adminstats/handler_test.go`。
- Admin stats limit 查询解析收敛残余风险：本轮只收敛 adminstats 单字段 limit 参数；session 的 limit/offset 仍保留本地错误文案包装，teacher 的 page/page_size 仍需保留空白字符串回退默认值语义，后续若继续统一应先补公共 helper 对字段名、范围和空白输入策略的显式表达。
- HTTP JSON 响应写出 helper 试点验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/progress/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/adapter/http/portrait ./internal/adapter/http/progress -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func writeJSON|httpjson\\.Write" backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/progress/handler.go backend-go/internal/platform/httpjson/decode.go -n`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/progress/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON 响应写出 helper 试点验证结果（2026-07-04）：`httpjson` 新增 `Write`，集中设置 `application/json; charset=utf-8`、状态码和 JSON encode；新增公共 helper 单测覆盖状态码、Content-Type 和响应体。portrait/progress 两个较小 HTTP adapter 删除本地 `writeJSON` 与 `encoding/json` import，成功响应和错误响应统一委托 `httpjson.Write`，同时保留各自 `errorResponse` 字段、状态码和错误文案；定向测试和后端全量测试通过。
- HTTP JSON 响应写出 helper 试点交付物：`backend-go/internal/platform/httpjson/decode.go`、`backend-go/internal/platform/httpjson/decode_test.go`、`backend-go/internal/adapter/http/portrait/handler.go`、`backend-go/internal/adapter/http/progress/handler.go`。
- HTTP JSON 响应写出 helper 试点残余风险：本轮只替换 portrait/progress 两个低复杂度 adapter；其余 HTTP adapter 仍保留本地 `writeJSON` 包装以降低一次性改动面。后续扩展时应继续保留各包错误响应 DTO 和 SSE/multipart 等非普通 JSON 输出路径，避免改变前端可见响应形状。
- HTTP JSON 响应写出 helper 扩展验证命令：`gofmt -w backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/admininbox/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/upload ./internal/adapter/http/xidian ./internal/adapter/http/exercise ./internal/adapter/http/adminstats ./internal/adapter/http/admininbox ./internal/platform/httpjson -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func writeJSON|encoding/json|httpjson\\.Write" backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/admininbox/handler.go -n`；`git diff --check -- backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/xidian/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/admininbox/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON 响应写出 helper 扩展验证结果（2026-07-04）：upload、xidian、exercise、adminstats、admininbox 五个普通 JSON HTTP adapter 删除本地 `writeJSON` 和不再需要的 `encoding/json` import，成功响应与错误响应统一委托 `httpjson.Write`；保留 multipart 上传、严格 JSON decode、分页校验、Xidian ServiceError 映射和各包错误响应 DTO 不变。扫描确认这五个 handler 不再定义本地 `writeJSON`，定向测试和后端全量测试通过。
- HTTP JSON 响应写出 helper 扩展交付物：`backend-go/internal/adapter/http/upload/handler.go`、`backend-go/internal/adapter/http/xidian/handler.go`、`backend-go/internal/adapter/http/exercise/handler.go`、`backend-go/internal/adapter/http/adminstats/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`。
- HTTP JSON 响应写出 helper 扩展残余风险：仍有 auth、session、question、resource、securitylog、teacher、knowledge、classroom、adminuser、adminsettings、adminaiconfig、mistake 等 adapter 保留本地 `writeJSON`；其中 session 还包含 SSE，后续扩展需按包验证特殊响应路径和错误契约。
- HTTP JSON 响应写出 helper 管理与教学接口扩展验证命令：`gofmt -w backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/teacher/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminaiconfig ./internal/adapter/http/adminsettings ./internal/adapter/http/adminuser ./internal/adapter/http/classroom ./internal/adapter/http/securitylog ./internal/adapter/http/teacher ./internal/platform/httpjson -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func writeJSON|encoding/json|httpjson\\.Write" backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/teacher/handler.go -n`；`git diff --check -- backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/teacher/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON 响应写出 helper 管理与教学接口扩展验证结果（2026-07-04）：adminaiconfig、adminsettings、adminuser、classroom、securitylog、teacher 六个 HTTP adapter 的成功响应与错误响应改为统一委托 `httpjson.Write`，删除各自本地 `writeJSON` 和不再需要的 `encoding/json` import；保留严格 JSON decode、CSV 导入导出、安全日志分页/导出、班级路由和教师分页校验等既有行为。扫描确认这六个 handler 不再定义本地 `writeJSON`，定向测试和后端全量测试通过。
- HTTP JSON 响应写出 helper 管理与教学接口扩展交付物：`backend-go/internal/adapter/http/adminaiconfig/handler.go`、`backend-go/internal/adapter/http/adminsettings/handler.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/classroom/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`、`backend-go/internal/adapter/http/teacher/handler.go`。
- HTTP JSON 响应写出 helper 管理与教学接口扩展残余风险：普通 JSON 写出仍剩 auth、knowledge、mistake、question、resource 和 session；其中 session 还使用 `json.Marshal` 生成 SSE event，auth 仍含 cookie/CSRF 路径，后续应按响应类型分批替换并单独验证。
- HTTP JSON 响应写出 helper 全 adapter 收敛验证命令：`gofmt -w backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/session/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/auth ./internal/adapter/http/knowledge ./internal/adapter/http/mistake ./internal/adapter/http/question ./internal/adapter/http/resource ./internal/adapter/http/session ./internal/platform/httpjson -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func writeJSON|encoding/json|json\\.|httpjson\\.Write" backend-go/internal/adapter/http -g "handler.go" -n`；`git diff --check -- backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/session/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON 响应写出 helper 全 adapter 收敛验证结果（2026-07-04）：auth、knowledge、mistake、question、resource、session 六个剩余 HTTP adapter 的普通 JSON 成功响应和错误响应改为统一委托 `httpjson.Write`，删除本地 `writeJSON`；auth/knowledge/mistake/question/resource 删除不再需要的 `encoding/json` import，session 保留 `encoding/json` 仅用于 SSE event 的 `json.Marshal`。扫描确认 HTTP adapter 下不再定义本地 `writeJSON`，定向测试和后端全量测试通过。
- HTTP JSON 响应写出 helper 全 adapter 收敛交付物：`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/session/handler.go`。
- HTTP JSON 响应写出 helper 全 adapter 收敛残余风险：本轮只统一普通 JSON response writer；session 的 SSE `json.Marshal`、导出/下载类响应 header、multipart 上传和 CSV/文件内容仍按各自协议保留独立路径，后续若继续抽象需要按协议类型分别验证。
- HTTP 时间查询解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpquery/time.go backend-go/internal/platform/httpquery/time_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/securitylog/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/mistake ./internal/adapter/http/securitylog -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "OptionalTime|parseISOTime|parseTimeQuery|parseOptionalTimeQuery|time\\.Parse" backend-go/internal/platform/httpquery backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/securitylog/handler.go -n`；`git diff --check -- backend-go/internal/platform/httpquery/time.go backend-go/internal/platform/httpquery/time_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/securitylog/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP 时间查询解析 helper 收敛验证结果（2026-07-04）：`httpquery` 新增 `OptionalTime` 和 `ErrInvalidTime`，集中处理空白值回退 nil、多 layout 顺序尝试和格式错误；新增单测覆盖空值、RFC3339、日期 fallback 和非法时间。mistake 列表的 `date_from/date_to` 与 securitylog 列表的 `start_date/end_date` 改为复用公共 helper，同时由各 handler 显式传入原有允许 layout 并保留原中文错误文案与状态码；定向测试和后端全量测试通过。
- HTTP 时间查询解析 helper 收敛交付物：`backend-go/internal/platform/httpquery/time.go`、`backend-go/internal/platform/httpquery/time_test.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`。
- HTTP 时间查询解析 helper 收敛残余风险：本轮只统一查询字符串中的可选时间解析，不改变业务层时间范围校验、时区解释或归档请求体时间解析；后续如要支持更多格式应通过调用方显式 layout 扩展并补路由测试。
- HTTP 浮点查询解析 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpquery/float.go backend-go/internal/platform/httpquery/float_test.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/exercise ./internal/adapter/http/mistake -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "Float\\(|BoundedFloat|ParseFloat|ErrInvalidFloat|ErrFloatOutOfRange" backend-go/internal/platform/httpquery backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/mistake/handler.go -n`；`git diff --check -- backend-go/internal/platform/httpquery/float.go backend-go/internal/platform/httpquery/float_test.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/exercise/handler_test.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/mistake/handler_test.go docs/backend-python-to-go-refactor.md`。
- HTTP 浮点查询解析 helper 收敛验证结果（2026-07-04）：`httpquery` 新增 `Float`、`BoundedFloat`、`ErrInvalidFloat` 和 `ErrFloatOutOfRange`，统一处理可选浮点默认值、解析错误和闭区间范围校验；新增单测覆盖空值、合法浮点、非法浮点和上下界越界。exercise `/next` 的 `difficulty` 改为复用 `BoundedFloat` 并保留 0..1 越界/格式错误文案；mistake 列表的 `difficulty_min/difficulty_max` 改为复用 `Float`，保留原有“必须是数字”和后续统一 0..1 范围校验。两个 HTTP 包新增非法 difficulty 路由测试，确认错误参数不会调用 service；定向测试和后端全量测试通过。
- HTTP 浮点查询解析 helper 收敛交付物：`backend-go/internal/platform/httpquery/float.go`、`backend-go/internal/platform/httpquery/float_test.go`、`backend-go/internal/adapter/http/exercise/handler.go`、`backend-go/internal/adapter/http/exercise/handler_test.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/mistake/handler_test.go`。
- HTTP 浮点查询解析 helper 收敛残余风险：本轮只统一 HTTP query 浮点解析；资源中心布尔 query、teacher/session 的特殊整数包装和业务层 DTO 校验仍保留各自语义，后续如继续收敛需先确认空白输入、错误文案和状态码兼容性。
- HTTP detail 错误响应 helper 试点验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/progress/handler.go backend-go/tests/contract/error_body_surface_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/adapter/http/portrait ./internal/adapter/http/progress ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/portrait/handler.go backend-go/internal/adapter/http/progress/handler.go backend-go/tests/contract/error_body_surface_test.go docs/backend-python-to-go-refactor.md`。
- HTTP detail 错误响应 helper 试点验证结果（2026-07-04）：`httpjson` 新增 `DetailError` 和 `WriteDetailError`，集中承载兼容字段 `detail/code/message`；公共 helper 单测覆盖状态码、Content-Type 和响应体字段。portrait/progress 删除本地 `errorResponse` 结构体，错误写出改为复用 `httpjson.WriteDetailError`，保留各自 wrapper、状态码和中文错误文案。错误响应契约测试升级为既支持 handler 本地 `errorResponse`，也支持通过 `httpjson.WriteDetailError` 复用公共 `DetailError` 并继续校验字段完整性；定向测试、契约测试和后端全量测试通过。
- HTTP detail 错误响应 helper 试点交付物：`backend-go/internal/platform/httpjson/decode.go`、`backend-go/internal/platform/httpjson/decode_test.go`、`backend-go/internal/adapter/http/portrait/handler.go`、`backend-go/internal/adapter/http/progress/handler.go`、`backend-go/tests/contract/error_body_surface_test.go`。
- HTTP detail 错误响应 helper 试点残余风险：本轮只在 portrait/progress 两个低复杂度 adapter 删除本地错误 DTO；其余 handler 仍保留本地 `errorResponse`，后续扩展时需避开 `/xidian` 的 `code/message` 双字段形状，并继续让契约测试证明字段兼容。
- HTTP detail 错误响应 helper 小型 adapter 扩展验证命令：`gofmt -w backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/exercise/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminstats ./internal/adapter/http/admininbox ./internal/adapter/http/upload ./internal/adapter/http/exercise ./internal/platform/httpjson ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminstats/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/upload/handler.go backend-go/internal/adapter/http/exercise/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP detail 错误响应 helper 小型 adapter 扩展验证结果（2026-07-04）：adminstats、admininbox、upload、exercise 四个 HTTP adapter 删除本地三字段 `errorResponse`，错误写出统一改为 `httpjson.WriteDetailError`；保留各自鉴权、分页/参数校验、multipart 上传和业务错误映射的状态码与中文文案。定向测试、错误响应字段契约测试和后端全量测试通过。
- HTTP detail 错误响应 helper 小型 adapter 扩展交付物：`backend-go/internal/adapter/http/adminstats/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/upload/handler.go`、`backend-go/internal/adapter/http/exercise/handler.go`。
- HTTP detail 错误响应 helper 小型 adapter 扩展残余风险：本轮继续只覆盖 `detail/code/message` 兼容形状；/xidian 的双字段错误体仍必须保持独立，其余较复杂 adapter 后续扩展时需逐包跑路由测试和契约测试。
- HTTP detail 错误响应 helper 中等 adapter 扩展验证命令：`gofmt -w backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/securitylog/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/adminsettings ./internal/adapter/http/adminuser ./internal/adapter/http/adminaiconfig ./internal/adapter/http/classroom ./internal/adapter/http/teacher ./internal/adapter/http/securitylog ./internal/platform/httpjson ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`git diff --check -- backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/teacher/handler.go backend-go/internal/adapter/http/securitylog/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP detail 错误响应 helper 中等 adapter 扩展验证结果（2026-07-04）：adminsettings、adminuser、adminaiconfig、classroom、teacher、securitylog 六个 HTTP adapter 删除本地三字段 `errorResponse`，错误写出统一改为 `httpjson.WriteDetailError`；保留严格 JSON decode、CSV 导入导出、安全日志分页/导出、班级路由、教师分页校验和 AI 配置业务错误映射的状态码与中文文案。定向测试、错误响应字段契约测试和后端全量测试通过。
- HTTP detail 错误响应 helper 中等 adapter 扩展交付物：`backend-go/internal/adapter/http/adminsettings/handler.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/adminaiconfig/handler.go`、`backend-go/internal/adapter/http/classroom/handler.go`、`backend-go/internal/adapter/http/teacher/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`。
- HTTP detail 错误响应 helper 中等 adapter 扩展残余风险：仍有 auth、knowledge、mistake、question、resource、session 等较复杂 adapter 保留本地三字段 `errorResponse`；/xidian 继续保持特殊双字段错误体。后续扩展仍需分批验证鉴权、SSE、分页和导出路径。
- HTTP detail 错误响应 helper 全普通三字段 adapter 收敛验证命令：`gofmt -w backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/session/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/adapter/http/auth ./internal/adapter/http/knowledge ./internal/adapter/http/mistake ./internal/adapter/http/question ./internal/adapter/http/resource ./internal/adapter/http/session ./internal/platform/httpjson ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "type errorResponse|Write\(w, status, errorResponse|WriteDetailError" backend-go/internal/adapter/http -g "handler.go" -n`；`git diff --check -- backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/session/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP detail 错误响应 helper 全普通三字段 adapter 收敛验证结果（2026-07-04）：auth、knowledge、mistake、question、resource、session 六个剩余普通三字段错误响应 adapter 删除本地 `errorResponse`，错误写出统一改为 `httpjson.WriteDetailError`；保留登录/注册 cookie 与 CSRF 路径、知识点 CRUD、错题筛选、题库导入/解析、资源列表/收藏、Session SSE 和任务取消等既有状态码与中文文案。定向 adapter 测试、错误响应字段契约测试和后端全量测试通过；错误响应扫描确认仅 `/xidian` 继续保留双字段 `code/message` 兼容体。
- HTTP detail 错误响应 helper 全普通三字段 adapter 收敛交付物：`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/session/handler.go`。
- HTTP detail 错误响应 helper 全普通三字段 adapter 收敛残余风险：`/xidian` 的双字段错误体是兼容契约，不纳入三字段 `DetailError` 合并；session 的 SSE 错误事件仍按事件流 JSON 形状写出，不走普通 HTTP detail helper；后续若新增 adapter 应继续由契约测试守住 `detail/code/message` 字段。
- HTTP query pagination 错误文案 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpquery/pagination.go backend-go/internal/platform/httpquery/pagination_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpquery ./internal/adapter/http/adminuser ./internal/adapter/http/admininbox ./internal/adapter/http/question ./internal/adapter/http/knowledge ./internal/adapter/http/mistake ./internal/adapter/http/resource ./internal/adapter/http/securitylog ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func paginationErrorMessage|PaginationErrorMessage" backend-go/internal -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/httpquery/pagination.go backend-go/internal/platform/httpquery/pagination_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/mistake/handler.go backend-go/internal/adapter/http/resource/handler.go backend-go/internal/adapter/http/securitylog/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP query pagination 错误文案 helper 收敛验证结果（2026-07-04）：adminuser、admininbox、question、knowledge、mistake、resource、securitylog 七个 adapter 删除本地重复 `paginationErrorMessage`，统一调用 `httpquery.PaginationErrorMessage(err, 100)`；公共 helper 新增未知错误、非整数、page 下界和 page_size 上界文案单测，保留原有 `detail/code/message` 响应形状、422 状态码和中文提示。定向 adapter 测试、`httpquery` 单测、契约测试和后端全量测试通过。
- HTTP query pagination 错误文案 helper 收敛交付物：`backend-go/internal/platform/httpquery/pagination.go`、`backend-go/internal/platform/httpquery/pagination_test.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`。
- HTTP query pagination 错误文案 helper 收敛残余风险：当前收敛只覆盖已使用 `httpquery.Pagination(..., maxPageSize=100)` 的 HTTP 列表端点；若后续端点使用不同上限，应继续传入实际 max 值并通过 handler 测试锁定兼容文案。
- HTTP JSON strict decode detail helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/session/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/adapter/http/adminaiconfig ./internal/adapter/http/admininbox ./internal/adapter/http/adminsettings ./internal/adapter/http/adminuser ./internal/adapter/http/auth ./internal/adapter/http/exercise ./internal/adapter/http/knowledge ./internal/adapter/http/question ./internal/adapter/http/securitylog ./internal/adapter/http/session ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "DecodeStrictOrDetailError|DecodeStrict\\(w, r" backend-go/internal/platform/httpjson backend-go/internal/adapter/http -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/adminuser/handler.go backend-go/internal/adapter/http/admininbox/handler.go backend-go/internal/adapter/http/auth/handler.go backend-go/internal/adapter/http/adminaiconfig/handler.go backend-go/internal/adapter/http/adminsettings/handler.go backend-go/internal/adapter/http/exercise/handler.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/securitylog/handler.go backend-go/internal/adapter/http/session/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON strict decode detail helper 收敛验证结果（2026-07-04）：新增 `httpjson.DecodeStrictOrDetailError`，统一写出普通 JSON 请求体解析失败的 `422 VALIDATION_ERROR 请求体格式错误` detail 响应；adminaiconfig、admininbox、adminsettings、adminuser、auth、exercise、knowledge、question、securitylog、session 十个 adapter 的 `decodeRequest` 改为委托公共 helper。`classroom`、`resource`、`/xidian` 保留本地 `400 BAD_REQUEST 请求体不是有效 JSON` 兼容契约。公共 helper 成功/失败路径单测、定向 adapter 测试、契约测试和后端全量测试通过。
- HTTP JSON strict decode detail helper 收敛交付物：`backend-go/internal/platform/httpjson/decode.go`、`backend-go/internal/platform/httpjson/decode_test.go`、`backend-go/internal/adapter/http/adminaiconfig/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/adminsettings/handler.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/adapter/http/exercise/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`、`backend-go/internal/adapter/http/session/handler.go`。
- HTTP JSON strict decode detail helper 收敛残余风险：本轮仅收敛普通三字段 detail 错误响应的 JSON body 解析失败路径；保留 400 坏 JSON 兼容的 adapter 后续如需统一，必须先更新契约测试和前端/调用方预期。
- HTTP JSON strict decode bad request helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/resource/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpjson ./internal/adapter/http/classroom ./internal/adapter/http/resource ./internal/adapter/http/xidian ./tests/contract -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "DecodeStrictOrBadRequest|DecodeStrictOrDetailError|DecodeStrict\\(w, r" backend-go/internal/platform/httpjson backend-go/internal/adapter/http -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/httpjson/decode.go backend-go/internal/platform/httpjson/decode_test.go backend-go/internal/adapter/http/classroom/handler.go backend-go/internal/adapter/http/resource/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP JSON strict decode bad request helper 收敛验证结果（2026-07-04）：新增 `httpjson.DecodeStrictOrBadRequest`，统一写出三字段 detail 形状的 `400 BAD_REQUEST 请求体不是有效 JSON`；classroom、resource 的 `decodeRequest` 改为委托公共 helper。`/xidian` 保留本地双字段 `code/message` 坏 JSON 响应。公共 helper 成功/失败路径单测、classroom/resource/xidian 定向测试、契约测试和后端全量测试通过。
- HTTP JSON strict decode bad request helper 收敛交付物：`backend-go/internal/platform/httpjson/decode.go`、`backend-go/internal/platform/httpjson/decode_test.go`、`backend-go/internal/adapter/http/classroom/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`。
- HTTP JSON strict decode bad request helper 收敛残余风险：`/xidian` 的坏 JSON 响应仍是双字段兼容契约，不能直接使用 detail helper；如后续新增 400 坏 JSON detail 响应 adapter，应优先复用 `DecodeStrictOrBadRequest` 并补 handler 测试。
- 字符串切片复制 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/sliceutil/strings.go backend-go/internal/platform/sliceutil/strings_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/session/service.go backend-go/internal/adapter/postgres/question_repository.go backend-go/internal/adapter/postgres/resource_repository.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/sliceutil ./internal/application/exercise ./internal/application/mistake ./internal/application/session ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func copyStrings|func copyStringSlice|copyStrings\\(|copyStringSlice\\(|CloneStrings" backend-go/internal/application backend-go/internal/adapter/postgres backend-go/internal/platform/sliceutil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/sliceutil/strings.go backend-go/internal/platform/sliceutil/strings_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/session/service.go backend-go/internal/adapter/postgres/question_repository.go backend-go/internal/adapter/postgres/resource_repository.go docs/backend-python-to-go-refactor.md`。
- 字符串切片复制 helper 收敛验证结果（2026-07-04）：新增 `sliceutil.CloneStrings`，保持既有 DTO/Repository 约定：复制字符串切片且 `nil` 输入返回非 nil 空切片；exercise、mistake、session 三个应用服务和 question/resource PostgreSQL adapter 删除本地 `copyStrings`/`copyStringSlice` 重复实现并统一调用公共 helper。公共 helper 单测覆盖 backing array 隔离和 nil->empty 行为；定向服务/adapter 测试和后端全量测试通过。
- 字符串切片复制 helper 收敛交付物：`backend-go/internal/platform/sliceutil/strings.go`、`backend-go/internal/platform/sliceutil/strings_test.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/session/service.go`、`backend-go/internal/adapter/postgres/question_repository.go`、`backend-go/internal/adapter/postgres/resource_repository.go`。
- 字符串切片复制 helper 收敛残余风险：本轮只收敛字符串切片复制语义；其他 map 复制、可选指针复制和领域去重/排序函数仍保留在各自模块，后续必须按 nil/empty JSON 形状和排序稳定性逐项验证后再抽取。
- 可选指针复制 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/ptrutil/ptr.go backend-go/internal/platform/ptrutil/ptr_test.go backend-go/internal/application/mistake/service.go backend-go/internal/application/session/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/ptrutil ./internal/application/mistake ./internal/application/session -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "copyOptional|ptrutil\\.Clone|func Clone" backend-go/internal/application backend-go/internal/platform/ptrutil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/ptrutil/ptr.go backend-go/internal/platform/ptrutil/ptr_test.go backend-go/internal/application/mistake/service.go backend-go/internal/application/session/service.go docs/backend-python-to-go-refactor.md`。
- 可选指针复制 helper 收敛验证结果（2026-07-04）：新增 `ptrutil.Clone` 泛型 helper，保持既有可选 DTO 指针复制语义：`nil` 输入返回 `nil`，非 nil 输入返回独立副本；mistake、session 应用服务删除本地 `copyOptionalString`/`copyOptionalInt` 重复实现并统一调用公共 helper。公共 helper 单测覆盖字符串复制隔离和 nil 输入；定向应用服务测试和后端全量测试通过。
- 可选指针复制 helper 收敛交付物：`backend-go/internal/platform/ptrutil/ptr.go`、`backend-go/internal/platform/ptrutil/ptr_test.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/session/service.go`。
- 可选指针复制 helper 收敛残余风险：本轮仅收敛纯值指针复制；领域内带校验、默认值、时间格式化或所有权语义的可选值处理仍保留在模块内，后续不应直接泛化。
- float map key 排序 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/maputil/float.go backend-go/internal/platform/maputil/float_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/progress/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/maputil ./internal/application/exercise ./internal/application/progress -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func sortedKeys|sortedKeys\\(|SortedFloatKeys" backend-go/internal/application backend-go/internal/platform/maputil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/maputil/float.go backend-go/internal/platform/maputil/float_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/progress/service.go docs/backend-python-to-go-refactor.md`。
- float map key 排序 helper 收敛验证结果（2026-07-04）：新增 `maputil.SortedFloatKeys`，保持 `map[string]float64` key 字典序输出且 nil map 返回非 nil 空切片；exercise、progress 应用服务删除本地重复 `sortedKeys` 并统一调用公共 helper。公共 helper 单测覆盖排序和 nil->empty 行为；定向应用服务测试和后端全量测试通过。
- float map key 排序 helper 收敛交付物：`backend-go/internal/platform/maputil/float.go`、`backend-go/internal/platform/maputil/float_test.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/progress/service.go`。
- float map key 排序 helper 收敛残余风险：本轮只收敛普通 key 字典序输出；teacher、portrait、mistake 等模块里的按值排序、稳定排序和领域 tie-breaker 逻辑继续保留在本地，避免误改推荐/排名语义。
- float map 复制 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/maputil/float.go backend-go/internal/platform/maputil/float_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/progress/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/mistake/service_test.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/maputil ./internal/application/exercise ./internal/application/progress ./internal/application/mistake -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "normalizeFloatMap|copyFloatMap|normalizeMastery|CloneFloatMap" backend-go/internal/application backend-go/internal/platform/maputil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/maputil/float.go backend-go/internal/platform/maputil/float_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/progress/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/mistake/service_test.go docs/backend-python-to-go-refactor.md`。
- float map 复制 helper 收敛验证结果（2026-07-04）：新增 `maputil.CloneFloatMap`，保持 `map[string]float64` 复制且 nil 输入返回非 nil 空 map；exercise 的 mastery/error tendency 复制、progress 的 mastery fallback 复制、mistake 的 mastery 复制和 fake repo 测试复制均改为公共 helper，删除本地 `normalizeFloatMap`、`copyFloatMap`、`normalizeMastery` 重复实现。公共 helper 单测覆盖 backing map 隔离和 nil->empty 行为；定向应用服务测试和后端全量测试通过。
- float map 复制 helper 收敛交付物：`backend-go/internal/platform/maputil/float.go`、`backend-go/internal/platform/maputil/float_test.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/progress/service.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/mistake/service_test.go`。
- float map 复制 helper 收敛残余风险：本轮只收敛无业务变换的 float map 拷贝；带 clamp、归一化、排序、聚合或默认值语义的 map 处理仍保留在各模块内，后续不可直接替换。
- teacher/question 集合 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/sliceutil/strings.go backend-go/internal/platform/sliceutil/strings_test.go backend-go/internal/application/teacher/service.go backend-go/internal/application/question/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/sliceutil ./internal/platform/maputil ./internal/application/teacher ./internal/application/question -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func sortedStringKeys|sortedStringKeys\\(|func uniqueStrings|uniqueStrings\\(|func appendUniqueStrings|appendUniqueStrings\\(|AppendUniqueNonEmptyStrings|SortedFloatKeys" backend-go/internal/application/teacher backend-go/internal/application/question backend-go/internal/platform/sliceutil backend-go/internal/platform/maputil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/sliceutil/strings.go backend-go/internal/platform/sliceutil/strings_test.go backend-go/internal/application/teacher/service.go backend-go/internal/application/question/service.go docs/backend-python-to-go-refactor.md`。
- teacher/question 集合 helper 收敛验证结果（2026-07-04）：teacher 的低分学生 key 排序改用既有 `maputil.SortedFloatKeys`，删除本地 `sortedStringKeys`；新增 `sliceutil.AppendUniqueNonEmptyStrings`，保持 trim、去空、按首次出现顺序去重语义，teacher 的 student/alert ID 去重和 question 的 AI 解析标签补充去重改为公共 helper。公共 helper 单测覆盖 trim、去空、去重、顺序和 nil->empty 行为；定向应用服务测试和后端全量测试通过。
- teacher/question 集合 helper 收敛交付物：`backend-go/internal/platform/sliceutil/strings.go`、`backend-go/internal/platform/sliceutil/strings_test.go`、`backend-go/internal/application/teacher/service.go`、`backend-go/internal/application/question/service.go`。
- teacher/question 集合 helper 收敛残余风险：exercise 的 `uniqueNonEmpty` 仍会排序，和本轮顺序保留 helper 语义不同，继续保留本地；后续若要统一需明确排序/稳定顺序契约并补推荐路径测试。
- HTTP 请求字符串校验 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/httpvalidate/string.go backend-go/internal/platform/httpvalidate/string_test.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/httpvalidate ./internal/adapter/http/knowledge ./internal/adapter/http/question ./internal/adapter/http/resource -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func validateRequiredString|func validateStringLength|func validateOptionalString|func validateOptionalStringMinMax|func validateRequiredField|func validateStringSlice|func validateOptionalStringSlice" backend-go/internal/adapter/http/knowledge backend-go/internal/adapter/http/question backend-go/internal/adapter/http/resource -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/httpvalidate/string.go backend-go/internal/platform/httpvalidate/string_test.go backend-go/internal/adapter/http/knowledge/handler.go backend-go/internal/adapter/http/question/handler.go backend-go/internal/adapter/http/resource/handler.go docs/backend-python-to-go-refactor.md`。
- HTTP 请求字符串校验 helper 收敛验证结果（2026-07-04）：新增 `httpvalidate` 平台包，统一 required trimmed string、普通字符串长度、可选字符串、必填指针字段和字符串切片长度校验，保持 `422 VALIDATION_ERROR` detail 响应文案不变；knowledge、question、resource 三个 HTTP adapter 删除本地重复字符串校验实现并改用公共 helper。公共 helper 单测覆盖通过、空白、超长、nil 可选值、必填字段缺失、列表数量和列表单项长度边界；三组定向 adapter 测试和后端全量测试通过；旧本地校验函数检索无残留。
- HTTP 请求字符串校验 helper 收敛交付物：`backend-go/internal/platform/httpvalidate/string.go`、`backend-go/internal/platform/httpvalidate/string_test.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/resource/handler.go`。
- HTTP 请求字符串校验 helper 收敛残余风险：本轮只收敛 HTTP 请求层中错误形状一致的字符串/字符串切片校验；节点类型、关系类型、难度、页数、题目预计用时等业务校验继续保留在各 handler 内，避免把领域文案和数值边界误泛化。
- 元数据读取 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/metautil/meta.go backend-go/internal/platform/metautil/meta_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/adapter/postgres/question_repository.go backend-go/internal/adapter/postgres/resource_repository.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/metautil ./internal/application/exercise ./internal/application/mistake ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func metaString|func metaStringSlice|func metaOptionalStringSlice|func readStringSlice|func metaStringPointer|func stringSliceFromMeta" backend-go/internal/application/exercise backend-go/internal/application/mistake backend-go/internal/adapter/postgres -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/metautil/meta.go backend-go/internal/platform/metautil/meta_test.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/adapter/postgres/question_repository.go backend-go/internal/adapter/postgres/resource_repository.go docs/backend-python-to-go-refactor.md`。
- 元数据读取 helper 收敛验证结果（2026-07-04）：新增 `metautil` 平台包，统一 `map[string]any` 元数据中的 string、string pointer、`[]string` 和 `[]any` 字符串切片读取；exercise、mistake 应用服务和 question/resource PostgreSQL adapter 删除本地重复读取实现并改用公共 helper。公共 helper 单测覆盖 nil/missing/non-string、空字符串存在性、slice backing array 隔离、`[]any` 非字符串过滤、missing 返回空切片或 nil 可选切片等边界；定向包测试和后端全量测试通过；旧底层读取 helper 检索仅剩保留默认值策略的 `metaStringDefault`。
- 元数据读取 helper 收敛交付物：`backend-go/internal/platform/metautil/meta.go`、`backend-go/internal/platform/metautil/meta_test.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/adapter/postgres/question_repository.go`、`backend-go/internal/adapter/postgres/resource_repository.go`。
- 元数据读取 helper 收敛残余风险：本轮只抽取无业务转换的 metadata 读取；exercise/resource 的 `metaStringDefault`、question 的 `stringFromMeta` 仍保留本地，因为它们对空字符串、空白字符串和 fallback 的处理语义不同，后续不能直接合并。
- 时间格式 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/timefmt/timefmt.go backend-go/internal/platform/timefmt/timefmt_test.go backend-go/internal/application/session/service.go backend-go/internal/application/portrait/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/teacher/service.go backend-go/internal/application/progress/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/timefmt ./internal/application/session ./internal/application/portrait ./internal/application/mistake ./internal/application/teacher ./internal/application/progress -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func formatTime|func optionalFormattedTime|func optionalTimeString|func dateString|func formatDateTime|2006-01-02T15:04:05\\.999999|2006-01-02\\\"" backend-go/internal/application/session backend-go/internal/application/portrait backend-go/internal/application/mistake backend-go/internal/application/teacher backend-go/internal/application/progress backend-go/internal/platform/timefmt -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/timefmt/timefmt.go backend-go/internal/platform/timefmt/timefmt_test.go backend-go/internal/application/session/service.go backend-go/internal/application/portrait/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/teacher/service.go backend-go/internal/application/progress/service.go docs/backend-python-to-go-refactor.md`。
- 时间格式 helper 收敛验证结果（2026-07-04）：新增 `timefmt` 平台包，统一响应层常用日期 `2006-01-02`、微秒时间戳 `2006-01-02T15:04:05.999999` 和可选微秒时间戳指针格式化；session、portrait、mistake、teacher、progress 五个应用服务删除本地重复时间格式 helper 并改用公共 helper。公共 helper 单测覆盖日期格式、微秒截断和 nil 可选时间；定向应用服务测试和后端全量测试通过；应用层目标范围内旧本地 helper 和裸 layout 检索无残留。
- 时间格式 helper 收敛交付物：`backend-go/internal/platform/timefmt/timefmt.go`、`backend-go/internal/platform/timefmt/timefmt_test.go`、`backend-go/internal/application/session/service.go`、`backend-go/internal/application/portrait/service.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/teacher/service.go`、`backend-go/internal/application/progress/service.go`。
- 时间格式 helper 收敛残余风险：本轮只收敛应用服务响应格式；`time.RFC3339`、管理员 CSV 导出 `2006-01-02 15:04:05`、HTTP query 解析 layout、仓储读模型内部兼容格式和 `startOfDay` 本地日期边界计算语义不同，继续保留原地，后续需按接口契约逐项验证后再处理。
- 日期边界 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/timefmt/timefmt.go backend-go/internal/platform/timefmt/timefmt_test.go backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go backend-go/internal/application/teacher/service_test.go backend-go/internal/application/adminstats/service.go backend-go/internal/application/securitylog/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/timefmt ./internal/application/adminstats ./internal/application/securitylog ./internal/application/progress ./internal/application/teacher -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func startOfDay|startOfDay\\(" backend-go/internal/application backend-go/internal/adapter backend-go/internal/platform -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/timefmt/timefmt.go backend-go/internal/platform/timefmt/timefmt_test.go backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go backend-go/internal/application/teacher/service_test.go backend-go/internal/application/adminstats/service.go backend-go/internal/application/securitylog/service.go docs/backend-python-to-go-refactor.md`。
- 日期边界 helper 收敛验证结果（2026-07-04）：`timefmt` 新增 `StartOfDay`，统一按输入时间所在 location 取当天零点；adminstats、securitylog、progress、teacher 四个应用服务删除本地重复 `startOfDay` 实现并改用公共 helper，teacher 单测同步改为断言公共 helper 输出。`timefmt` 单测覆盖 location 保留；定向应用服务测试和后端全量测试通过；全仓目标范围内 `func startOfDay`/`startOfDay(` 检索无残留。
- 日期边界 helper 收敛交付物：`backend-go/internal/platform/timefmt/timefmt.go`、`backend-go/internal/platform/timefmt/timefmt_test.go`、`backend-go/internal/application/adminstats/service.go`、`backend-go/internal/application/securitylog/service.go`、`backend-go/internal/application/progress/service.go`、`backend-go/internal/application/teacher/service.go`、`backend-go/internal/application/teacher/service_test.go`。
- 日期边界 helper 收敛残余风险：本轮只收敛“当天零点”基础计算；`weekdayMondayIndex`、`timeRangeStart`、安全日志归档/删除阈值、统计 lookback 窗口和仓储 SQL 日期聚合仍是各领域语义，继续保留在模块内。
- 整数 min/max helper 收敛验证命令：`gofmt -w backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go backend-go/internal/adapter/postgres/question_repository.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/progress ./internal/application/teacher ./internal/adapter/postgres -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func minInt|func maxInt|minInt\\(|maxInt\\(" backend-go/internal -g "*.go" -n`；`git diff --check -- backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go backend-go/internal/adapter/postgres/question_repository.go docs/backend-python-to-go-refactor.md`。
- 整数 min/max helper 收敛验证结果（2026-07-04）：progress、teacher 和 question PostgreSQL adapter 的本地 `minInt`/`maxInt` 纯重复实现改用 Go 1.25 内建 `min`/`max`，删除三处无业务语义的 helper。定向包测试和后端全量测试通过；全仓 `minInt`/`maxInt` 函数和调用检索无残留。
- 整数 min/max helper 收敛交付物：`backend-go/internal/application/progress/service.go`、`backend-go/internal/application/teacher/service.go`、`backend-go/internal/adapter/postgres/question_repository.go`。
- 整数 min/max helper 收敛残余风险：本轮只替换普通二元整数比较；`clampInt`、分页总页数、分数四舍五入、排序 tie-breaker 和仓储 SQL placeholder 生成仍有各自语义，继续保留原地。
- session 整数格式化 helper 收敛验证命令：`gofmt -w backend-go/internal/application/session/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/application/session -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func intString|intString\\(" backend-go/internal/application/session -g "*.go" -n`；`git diff --check -- backend-go/internal/application/session/service.go docs/backend-python-to-go-refactor.md`。
- session 整数格式化 helper 收敛验证结果（2026-07-04）：`BatchDeleteSessions` 成功文案的删除数量改用标准库 `strconv.Itoa`，删除本地手写 `intString` 实现；既有 session 单测覆盖“成功删除 2 个会话”文案，定向测试和后端全量测试通过；session 包内 `intString` 函数和调用检索无残留。
- session 整数格式化 helper 收敛交付物：`backend-go/internal/application/session/service.go`。
- session 整数格式化 helper 收敛残余风险：本轮只替换单一手写整数转字符串 helper；其他 `strconv.Itoa`、`fmt.Sprintf` 调用多为 URL 参数、SQL placeholder、错误文案或 LLM prompt 结构化输出，语义不同，继续保留。
- 浮点保留位数 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/numutil/round.go backend-go/internal/platform/numutil/round_test.go backend-go/internal/application/adminstats/service.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/portrait/service.go backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/numutil ./internal/application/adminstats ./internal/application/exercise ./internal/application/mistake ./internal/application/portrait ./internal/application/progress ./internal/application/teacher -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func round[0-9]?\\(|round1\\(|round2\\(|round3\\(|round4\\(|round\\(" backend-go/internal/application/exercise/service.go backend-go/internal/application/teacher/service.go backend-go/internal/application/adminstats/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/progress/service.go backend-go/internal/application/portrait/service.go -n`；`git diff --check -- backend-go/internal/platform/numutil/round.go backend-go/internal/platform/numutil/round_test.go backend-go/internal/application/adminstats/service.go backend-go/internal/application/exercise/service.go backend-go/internal/application/mistake/service.go backend-go/internal/application/portrait/service.go backend-go/internal/application/progress/service.go backend-go/internal/application/teacher/service.go docs/backend-python-to-go-refactor.md`。
- 浮点保留位数 helper 收敛验证结果（2026-07-04）：新增 `numutil.RoundPlaces`，统一 `math.Round(value*scale)/scale` 保留小数位逻辑；adminstats、exercise、mistake、portrait、progress、teacher 六个应用服务删除本地 `round`/`round1`/`round2`/`round3`/`round4` 薄封装并改用公共 helper。公共 helper 单测覆盖 0/1/2/4 位、正负数和四舍五入边界；定向应用服务测试和后端全量测试通过；目标应用服务范围内旧 round helper 检索无残留。
- 浮点保留位数 helper 收敛交付物：`backend-go/internal/platform/numutil/round.go`、`backend-go/internal/platform/numutil/round_test.go`、`backend-go/internal/application/adminstats/service.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/portrait/service.go`、`backend-go/internal/application/progress/service.go`、`backend-go/internal/application/teacher/service.go`。
- 浮点保留位数 helper 收敛残余风险：本轮只收敛纯数值四舍五入；teacher 的 `formatScore`、portrait 的 `formatNumber`、question 的题目生成参数计算以及 math.Min/Max/Exp 等业务数学逻辑继续保留在模块内，避免改变展示文本或模型语义。
- 百分比计算 helper 收敛验证命令：`gofmt -w backend-go/internal/platform/numutil/round.go backend-go/internal/platform/numutil/round_test.go backend-go/internal/application/mistake/service.go backend-go/internal/application/progress/service.go`；`GOCACHE=E:\code\msp-go\.gocache go test ./internal/platform/numutil ./internal/application/mistake ./internal/application/progress -count=1`；`GOCACHE=E:\code\msp-go\.gocache MSP_REPO_ROOT=E:\code\msp-go go test ./... -count=1`；`rg "func percent|percent\\(" backend-go/internal/application/mistake/service.go backend-go/internal/application/progress/service.go backend-go/internal/platform/numutil -g "*.go" -n`；`git diff --check -- backend-go/internal/platform/numutil/round.go backend-go/internal/platform/numutil/round_test.go backend-go/internal/application/mistake/service.go backend-go/internal/application/progress/service.go docs/backend-python-to-go-refactor.md`。
- 百分比计算 helper 收敛验证结果（2026-07-04）：`numutil` 新增 `Percent(total,count)`，统一 `total<=0` 返回 0、否则 `count/total*100` 的百分比计算；mistake 和 progress 应用服务删除本地重复 `percent` helper 并改用公共 helper。公共 helper 单测覆盖正数、零 total 和负 total；已有 mistake/progress 测试继续覆盖 60%/70% 业务响应；定向测试和后端全量测试通过，目标范围内本地 `percent` 检索无残留。
- 百分比计算 helper 收敛交付物：`backend-go/internal/platform/numutil/round.go`、`backend-go/internal/platform/numutil/round_test.go`、`backend-go/internal/application/mistake/service.go`、`backend-go/internal/application/progress/service.go`。
- 百分比计算 helper 收敛残余风险：portrait 的 `ratio` 返回 0..1 比例而不是百分比，teacher 多处百分比计算紧邻额外四舍五入和领域上下文，暂不强行合并；后续若继续收敛，应先确认响应单位和展示精度。
- 交付物链接：`backend-go/internal/platform/httpjson/`、`backend-go/internal/platform/httpquery/`、`backend-go/internal/platform/uploadpath/`、`backend-go/internal/platform/outbound/`、`backend-go/internal/platform/csvsafe/`、`backend-go/internal/application/adminaiconfig/service.go`、`backend-go/internal/adapter/llm/einoagent/agent.go`、`backend-go/internal/application/auth/service.go`、`backend-go/internal/application/auth/token.go`、`backend-go/internal/application/auth/token_test.go`、`backend-go/internal/adapter/http/auth/handler.go`、`backend-go/internal/application/upload/service.go`、`backend-go/internal/adapter/http/upload/handler.go`、`backend-go/internal/application/session/service.go`、`backend-go/internal/adapter/http/session/handler.go`、`backend-go/internal/application/exercise/service.go`、`backend-go/internal/adapter/http/exercise/handler.go`、`backend-go/internal/application/resource/service.go`、`backend-go/internal/application/securitylog/service.go`、`backend-go/internal/adapter/http/adminuser/handler.go`、`backend-go/internal/adapter/http/adminsettings/handler.go`、`backend-go/internal/adapter/http/adminstats/handler.go`、`backend-go/internal/adapter/http/admininbox/handler.go`、`backend-go/internal/adapter/http/securitylog/handler.go`、`backend-go/internal/adapter/http/knowledge/handler.go`、`backend-go/internal/adapter/http/question/handler.go`、`backend-go/internal/adapter/http/question/handler_test.go`、`backend-go/internal/adapter/http/portrait/handler.go`、`backend-go/internal/adapter/http/portrait/handler_test.go`、`backend-go/internal/adapter/http/teacher/handler.go`、`backend-go/internal/adapter/http/teacher/handler_test.go`、`backend-go/internal/adapter/http/progress/handler.go`、`backend-go/internal/adapter/http/progress/handler_test.go`、`backend-go/internal/adapter/http/classroom/handler.go`、`backend-go/internal/adapter/http/classroom/handler_test.go`、`backend-go/internal/adapter/http/xidian/handler.go`、`backend-go/internal/adapter/http/xidian/handler_test.go`、`backend-go/internal/adapter/http/mistake/handler.go`、`backend-go/internal/adapter/http/mistake/handler_test.go`、`backend-go/internal/adapter/http/exercise/handler_test.go`、`backend-go/internal/adapter/http/resource/handler.go`、`backend-go/internal/adapter/http/resource/handler_test.go`、`backend-go/internal/adapter/postgres/resource_repository.go`、`backend-go/internal/adapter/storage/local.go`、`backend-go/internal/adapter/storage/local_test.go`、`backend-go/internal/adapter/storage/qiniu.go`、`backend-go/internal/adapter/storage/s3.go`、`backend-go/internal/adapter/storage/factory.go`、`backend-go/internal/integration/xidian/client.go`、`backend-go/internal/integration/xidian/session.go`、`backend-go/internal/platform/middleware/middleware.go`、`backend-go/internal/platform/config/config.go`、`backend-go/internal/platform/httpserver/server.go`、`backend-go/tests/contract/frontend_route_surface_test.go`、`frontend/src/modules/auth/store/authSlice.ts`、`frontend/src/modules/auth/store/authSlice.test.ts`、`frontend/src/libs/auth/tokenStorage.ts`、`frontend/src/libs/auth/tokenStorage.test.ts`、`frontend/src/store/slices/uiSlice.ts`、`frontend/src/store/slices/uiSlice.test.ts`、`frontend/src/modules/classroom/store/classtableSlice.ts`、`frontend/src/modules/classroom/store/classtableSlice.test.ts`、`frontend/src/libs/export/dashboardExporter.ts`、`frontend/src/libs/utils/resourceUtils.ts`、`frontend/src/libs/utils/safeUrl.ts`、`frontend/src/libs/utils/download.ts`、`frontend/src/libs/utils/logger.ts`、`frontend/src/libs/math/MathText.tsx`、`frontend/src/components/chat/markdownRenderConfig.tsx`、`frontend/src/components/chat/MessageItem.tsx`、`frontend/src/pages/student/MyClassPage.tsx`、`frontend/src/pages/teacher/TeacherResourcesPage/components/ResourceCard.tsx`、`frontend/src/pages/teacher/TeacherResourcesPage/components/ResourceDetailModal.tsx`、`frontend/src/pages/teacher/TeacherResourcesPage/components/ResourceListView.tsx`、`frontend/src/pages/teacher/TeacherResourcesPage/components/ResourceEditModal.tsx`、`frontend/src/pages/student/ResourcesPage.tsx`、`.gitignore`。
- 遗留风险：本次默认阻断本地和内网 AI provider、七牛上传地址、西电门户地址和 external 资源链接；如需接入 Ollama、内网 OpenAI-compatible 网关、专有云私网 endpoint、私有对象存储上传网关、校园网私有代理或仅内网可访问的教学资源链接，后续应新增显式 allow-list/部署代理配置，并配套审计日志与测试，而不是放宽默认行为。资源仓储资产替换当前由 adapter 包编译测试覆盖，仍建议在可用 PostgreSQL 集成环境补一条真实更新资源 URL 的事务回归测试。
