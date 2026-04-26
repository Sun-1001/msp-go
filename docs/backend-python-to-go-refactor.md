# 后端 Python 到 Go 重构迁移文档

**文档状态**：P4 核心学习域进行中，P5 内容与教学管理域进行中（资源中心已迁移）
**最后更新**：2026-04-26
**适用范围**：`backend/` Python FastAPI 后端整体迁移到 Go 后端
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

- Go 后端承接所有 `/api/v1` 业务接口、健康检查、监控指标和文件访问入口。
- Python 后端不再承载线上业务流量。
- 数据库迁移、缓存、对象存储、AI 调用、鉴权和审计能力在 Go 后端中具备等价实现。
- 旧 Python 服务仅保留为历史参考或迁移对照，最终可归档或删除。

### 2.2 非目标

- 不在第一阶段重写前端业务。
- 不在未冻结 API 契约前改变前端请求路径和响应结构。
- 不在缺少数据备份和回滚方案时修改生产数据库结构。
- 不把临时兼容代码视为最终架构。

---

## 3. 当前 Python 后端基线

### 3.1 技术栈

当前后端位于 `backend/`，核心技术栈如下：

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
| 管理员 BKT | `/admin/bkt` | BKT 参数维护和种子数据 |

### 3.4 数据模型范围

当前 SQLAlchemy 模型至少覆盖：

- 用户、学生画像、认证与密码重置。
- BKT 参数和学生知识点掌握状态。
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
| P5 内容与教学管理域 | IN_PROGRESS | 迁移题库、资源、班级、教师统计、知识点 | `/questions`、`/resources`、`/classes`、`/teacher`、`/admin/knowledge` | 12.6 |
| P6 AI 与 Agent 能力 | TODO | 迁移 LLM 配置、Agent 调用、数学求解、诊断 | `/admin/ai-config`、Agent 抽象、数学工具链 | 12.7 |
| P7 集成与运维域 | TODO | 迁移西电集成、上传、系统设置、安全日志、监控 | `/xidian`、`/upload`、`/admin/settings`、`/admin/security-logs`、`/metrics` | 12.8 |
| P8 双跑与契约验证 | TODO | Python/Go 并行验证，确认接口和数据等价 | Contract tests、回归报告、性能报告 | 12.9 |
| P9 流量切换与 Python 下线 | TODO | 切换生产入口，保留回滚窗口，下线 Python 服务 | 部署配置、回滚手册、归档清单 | 12.10 |

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
- 统一诊断报告、练习提交、BKT 更新的事务边界。

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

- 迁移 LLM provider、模型、Agent 配置管理。
- 建立 Go 侧 AI 调用抽象，隐藏 provider 差异。
- 迁移数学求解、答案等价判断、诊断、教学反馈能力。
- 明确 LangGraph/SymPy 的替代、重写或临时桥接方案。

原则：

- 可以在迁移期保留 Python 服务作为对照或双跑参考。
- 最终线上业务能力必须由 Go 后端或明确批准的非 Python 独立服务承载。
- 所有临时桥接都必须记录退出条件。

验收标准：

- Agent 配置接口等价。
- 数学求解和答案判定测试通过。
- LLM 调用失败、超时、限流、降级路径覆盖。

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
| P0 | `/health`、`/metrics` | DONE | Go P1 骨架已承接 `/health`、`/health/detailed`、`/metrics` |
| P1 | `/auth` | DONE | Go P3 已承接登录、注册、刷新、登出、当前用户、修改密码、注册状态、忘记密码公开申请/状态查询 |
| P1 | `/admin/users` | TODO | 管理员和权限基础能力 |
| P1 | `/admin/settings` | TODO | 系统配置影响运行时行为 |
| P2 | `/session` | DONE | Go P4 已承接会话创建、历史、列表、结束、模式、删除、批删、任务取消和 SSE 形状兼容降级；Agent 流式质量等价留到 P6 |
| P2 | `/exercise` | DONE | Go P4 已承接下一题、提交答案、题目详情、题目解析；AI OCR/LLM 诊断质量等价留到 P6 |
| P2 | `/mistakes` | DONE | Go P4 已承接列表、统计、详情、标记掌握、删除和复习推荐 |
| P2 | `/progress` | DONE | Go P4 首轮已承接 overview、mastery、statistics、path、knowledge-graph、class-ranking、chapters |
| P2 | `/portrait` | DONE | Go P4 已承接读取、清除和模板画像生成；LLM 画像质量等价留到 P6 AI 能力收敛 |
| P3 | `/questions` | TODO | 教师题库 |
| P3 | `/resources` | DONE | Go P5 已承接资源列表、详情、创建、更新、软删除、统计、收藏列表和收藏切换；上传/对象存储仍归 P7 `/upload` |
| P3 | `/classes` | TODO | 班级管理 |
| P3 | `/teacher` | TODO | 教师统计 |
| P3 | `/admin/knowledge` | TODO | 知识图谱维护 |
| P4 | `/admin/ai-config` | TODO | AI 配置 |
| P4 | `/admin/bkt` | TODO | BKT 参数维护 |
| P5 | `/xidian` | TODO | 外部系统集成 |
| P5 | `/upload` | TODO | 文件上传和对象存储 |
| P5 | `/admin/security-logs` | TODO | 审计、安全日志 |
| P5 | `/admin/stats`、`/admin/inbox` | TODO | 管理员辅助能力 |

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
| R3 | LangGraph/SymPy 能力在 Go 中无直接等价实现 | AI 功能延期 | OPEN | P6 单独 ADR，允许临时双跑但必须定义退出条件 |
| R4 | JWT/Cookie 兼容不完整 | 用户登录失效 | MITIGATED | P3 已实现 Python 兼容 JWT claims、HMAC 算法校验、refresh token HttpOnly Cookie 行为和轮换测试；后续 P8 继续做 Python/Go 双跑契约验证 |
| R5 | Alembic 历史与 Go 迁移工具并存 | 迁移历史混乱 | MITIGATED | P2 已生成 Go 单步初始 schema；后续生产迁移由 Go runner 负责，Alembic 保留为历史参考 |
| R6 | 上传文件路径或对象存储 key 改变 | 历史资源不可访问 | OPEN | P7 做历史文件访问回归 |
| R7 | 缺少 git 元数据时无法做变更边界检查 | 并行任务冲突风险增加 | OPEN | 修改前后记录文件清单，避免触碰无关文件 |
| R8 | 默认入口已切到 Go，但业务 `/api/v1/*` 尚未迁移 | 前端业务调用会收到 501 占位响应 | OPEN | 按 P3-P7 逐模块迁移；未迁移接口禁止静默回落 Python |
| R9 | 当前机器未配置可连接 PostgreSQL 测试库且 Docker CLI 不可用 | P2 数据库迁移/Repository 集成验收不能在本机闭环 | CLOSED | 已使用本地 PostgreSQL `math_platform` 执行清库、Go 迁移、重复迁移和迁移集成测试；Docker CLI 仍不可用但不阻塞 P2 |

---

## 11. 架构决策记录

| ADR | 日期 | 决策 | 状态 | 说明 |
|-----|------|------|------|------|
| ADR-001 | 2026-04-18 | Go HTTP router 选择 `net/http` `ServeMux` | DONE | P1 使用标准库，避免框架迁移期额外抽象 |
| ADR-002 | 2026-04-18 | Go 数据访问方式选择 `pgx/v5` + `go-redis/v9` | DONE | P1 先建立连接和健康检查；Repository 在 P2 补齐 |
| ADR-003 | 2026-04-18 | 从 Python Alembic head 生成 Go 单步初始 schema，后续由 Go forward migration runner 承接 | DONE | `0001_initial_schema.up.sql` 由临时库执行 Alembic 至 `0019_performance_indexes_phase3` 后 `pg_dump` 生成；包含表、枚举、索引、外键和种子数据；`go_schema_migrations` 记录 Go 迁移状态 |
| ADR-004 | TODO | AI/Agent 重写或桥接方案 | TODO | P6 决策 |
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
- 验证命令：`gofmt -w ...`、`go test ./... -count=1`、`go vet ./...`、`MSP_GO_TEST_DATABASE_URL=postgres://.../math_platform?sslmode=disable go test ./internal/adapter/postgres -run TestUserRepositoryIntegration -count=1 -v`
- 验证结果：Go 全量单元/契约测试通过；Go vet 通过；PostgreSQL 用户仓储集成测试在事务内通过并回滚；覆盖 JWT claims、bcrypt 密码、注册开关、登录失败锁定、refresh cookie 设置/清理、用户角色判断、用户仓储枚举映射和密码重置公开申请/状态查询
- 交付物链接：`backend-go/internal/domain/user/`、`backend-go/internal/application/auth/`、`backend-go/internal/adapter/postgres/user_repository.go`、`backend-go/internal/adapter/http/auth/`、`backend-go/cmd/api/main.go`、`backend-go/internal/platform/config/`
- 遗留风险：`/admin/users` 管理员用户 CRUD、用户导入导出、邮箱绑定/验证码接口仍未由 Go 承接；现有前端若调用这些未迁移接口仍会收到 501，占位风险将在后续用户管理/管理员域切片中处理；P8 仍需执行 Python/Go 双跑契约验证

### 12.5 P4 核心学习域

- 状态：IN_PROGRESS
- 开始日期：2026-04-25
- 完成日期：TODO
- 负责人：Codex
- 验证命令（阶段进行中）：`gofmt -w ...`、`go test ./... -count=1`、`go vet ./...`
- 验证结果（阶段进行中）：Go 全量单元/契约测试通过；Go vet 通过；覆盖 `/progress` 鉴权、overview、mastery、statistics、path、knowledge-graph、class-ranking、chapters 的应用层和 HTTP 层主要路径；覆盖 `/portrait` 鉴权、读取、清除和模板画像生成的应用层与 HTTP 层主要路径；覆盖 `/mistakes` 鉴权、列表筛选/排序/分页、统计、详情、标记掌握、删除和复习推荐的应用层与 HTTP 层主要路径；覆盖 `/exercise` 鉴权、下一题选择、提交答案、BKT/profile 更新、题目详情和解析权限的应用层与 HTTP 层主要路径；覆盖 `/session` 鉴权、创建、历史、列表、结束、模式、删除、批删、任务取消和 SSE 形状兼容降级的应用层与 HTTP 层主要路径
- 交付物链接：`backend-go/internal/application/progress/`、`backend-go/internal/adapter/http/progress/`、`backend-go/internal/adapter/postgres/progress_repository.go`、`backend-go/internal/application/portrait/`、`backend-go/internal/adapter/http/portrait/`、`backend-go/internal/adapter/postgres/portrait_repository.go`、`backend-go/internal/application/mistake/`、`backend-go/internal/adapter/http/mistake/`、`backend-go/internal/adapter/postgres/mistake_repository.go`、`backend-go/internal/application/exercise/`、`backend-go/internal/adapter/http/exercise/`、`backend-go/internal/adapter/postgres/exercise_repository.go`、`backend-go/internal/application/session/`、`backend-go/internal/adapter/http/session/`、`backend-go/internal/adapter/postgres/session_repository.go`、`backend-go/cmd/api/main.go`（进行中）
- 遗留风险：`/session/{id}/chat` 当前保存用户消息并返回 SSE 形状兼容的导师占位回复，尚未恢复 Python 侧 Agent 工作流、资源推荐兜底和画像更新；`/portrait/generate` 当前由 Go 模板报告生成承接，尚未恢复 Python 侧 LLM 画像质量；`/exercise/submit` 当前使用规范化文本比对和基础诊断承接，图片 OCR、数学等价多层判定和 LLM 诊断质量需在 P6 AI 能力中替换或补充双跑验证；`/progress`、`/portrait`、`/mistakes`、`/exercise` 和 `/session` 仍需在可用 PostgreSQL 测试库中补充 Repository 集成测试，并在 P8 做 Python/Go 双跑契约验证

### 12.6 P5 内容与教学管理域

- 状态：IN_PROGRESS
- 开始日期：2026-04-26
- 完成日期：TODO
- 负责人：Codex
- 验证命令（阶段进行中）：`gofmt -w ...`、`go test ./... -count=1`、`go vet ./...`
- 验证结果（阶段进行中）：Go 全量单元/契约测试通过；Go vet 通过；覆盖 `/resources` 鉴权、列表筛选/分页、详情、教师权限校验、创建默认值、统计/收藏字面量路由、404 映射和软删除响应；PostgreSQL adapter 编译通过并实现 `contents`、`content_assets`、`user_favorites` 的资源中心读写语义。
- 交付物链接：`backend-go/internal/application/resource/`、`backend-go/internal/adapter/http/resource/`、`backend-go/internal/adapter/postgres/resource_repository.go`、`backend-go/cmd/api/main.go`（进行中）
- 遗留风险：`/questions`、`/classes`、`/teacher`、`/admin/knowledge` 仍未由 Go 承接；`/resources` 仍需在可用 PostgreSQL 测试库中补充 Repository 集成测试，并在 P8 做 Python/Go 双跑契约验证；资源文件上传和对象存储能力不属于本切片，仍留到 P7 `/upload`。

### 12.7 P6 AI 与 Agent 能力

- 状态：TODO
- 开始日期：TODO
- 完成日期：TODO
- 负责人：TODO
- 验证命令：TODO
- 验证结果：TODO
- 交付物链接：TODO
- 遗留风险：TODO

### 12.8 P7 集成与运维域

- 状态：TODO
- 开始日期：TODO
- 完成日期：TODO
- 负责人：TODO
- 验证命令：TODO
- 验证结果：TODO
- 交付物链接：TODO
- 遗留风险：TODO

### 12.9 P8 双跑与契约验证

- 状态：TODO
- 开始日期：TODO
- 完成日期：TODO
- 负责人：TODO
- 验证命令：TODO
- 验证结果：TODO
- 交付物链接：TODO
- 遗留风险：TODO

### 12.10 P9 流量切换与 Python 下线

- 状态：TODO
- 开始日期：TODO
- 完成日期：TODO
- 负责人：TODO
- 验证命令：TODO
- 验证结果：TODO
- 交付物链接：TODO
- 遗留风险：TODO

---

## 13. 后续待补充

- P0 后补充实际 OpenAPI 导出路径和契约测试入口。
- P1 后补充 Go 技术栈版本和标准命令。
- P2 已补充数据库迁移工具、目录和回滚命令；后续变更按 Go forward migration 追加。
- P6 后补充 AI/Agent 详细设计。
- P8 后补充双跑报告模板。

---

## 14. 更新记录

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
