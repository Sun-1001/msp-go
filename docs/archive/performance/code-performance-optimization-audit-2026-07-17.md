# MSP-Go 全仓代码与性能优化审计报告

> 归档说明：本文是 2026-07-17 的时间点审计，只用于记录当时的代码、测试和构建证据。当前未完成工作统一以 [项目待办](../../TODO.md) 为准；后端迁移阶段状态仍以 [迁移跟踪](../../backend-python-to-go-refactor.md) 为准。

审计日期：2026-07-17  
审计范围：Go API、React 前端、PostgreSQL/Redis 使用方式、Nginx 与 Docker 配置  
审计方式：静态代码审查、单元测试与覆盖率、竞态检查、Go 微基准、前端生产构建及压缩体积分析、依赖漏洞扫描  
运行环境：Windows/amd64，Intel Core i9-14900HX；未连接生产 PostgreSQL、Redis 或外部 AI provider

## 结论摘要

- 未发现需要立即阻断合并的 Critical 或 High 问题；治理等级为 **Caution**，原因是中优先级性能、测试和架构债务较集中。
- 当前最有价值的工作不是直接增加缓存，而是先补齐按路由统计的请求时延、数据库连接池与查询次数观测，再优化教师分析、学习进度等读模型的串行查询扇出。
- 前端首屏资源约为 702,470 B 原始大小、200,201 B Gzip；1.39 MB 的 G6 图谱块不在首屏，但进入管理端知识管理页时会随静态图谱依赖加载，仍有延迟加载空间。
- Go 现有微基准表现稳定且分配较低，但只覆盖 Gzip、文本读取和本地限流，不能代表登录、题目列表、练习提交、会话或教师聚合接口的端到端性能。
- 前端整体语句覆盖率为 29.64%，Go 总语句覆盖率为 58.1%，其中 PostgreSQL adapter 为 12.4%；核心业务已有较多测试，但数据库读模型和大部分前端页面/服务仍缺少回归保护。
- 本轮没有修改源代码、API、数据库 schema、部署行为或迁移阶段状态。

## 审查概览

| 项目 | Go 后端 | React 前端 |
|------|---------|------------|
| 生产文件 | 136 个 `*.go` | 278 个 `*.ts`/`*.tsx` |
| 生产代码物理行 | 38,872 | 48,546 |
| 测试文件 | 125 | 60 |
| 测试代码物理行 | 26,569 | 7,655 |
| 覆盖率 | 58.1% statements | 29.64% statements / 30.74% lines |
| 主要技术 | Go 1.25、net/http、pgx、go-redis、Eino | React 19、TypeScript 5.9、Vite 7、Redux Toolkit |

统计只包含 Git 跟踪的 Go/TypeScript 源文件，不包含 `dist`、`coverage`、缓存和生成文件。

## 问题统计

| 严重程度 | 数量 |
|----------|-----:|
| Critical | 0 |
| High | 0 |
| Medium | 13 |
| Low | 1 |
| **总计** | **14** |

| 维度 | 数量 |
|------|-----:|
| Correctness | 2 |
| Security | 1 |
| Performance | 6 |
| Readability | 1 |
| Testing | 3 |
| Architecture | 1 |

严重程度表示风险，P0/P1/P2 表示建议排期，两者不混用。证据类型分为：

- **实测**：由本轮命令、测试或构建产物直接证明。
- **静态高概率**：调用路径明确，但收益仍需真实数据和负载验证。
- **待验证假设**：配置具备优化空间，未达到建议立即变更的证据强度。

## 高风险区域

| 区域 | 主要原因 | 建议优先级 |
|------|----------|------------|
| `internal/application/teacher` | 聚合接口包含 10-12 次串行 Repository 调用 | P1 |
| `internal/application/progress` | 概览查询扇出；筛选图谱仍读取全量关系 | P1 |
| `internal/platform/metrics` | 只有无标签总请求计数，无法计算路由 P95/P99 | P0 |
| `internal/adapter/postgres` | 覆盖率 12.4%；多处前后通配模糊搜索无法利用普通 B-tree | P1 |
| `frontend/src/pages/admin/KnowledgeManagementPage` | 图谱运行时和全量节点请求在非图谱首选项卡也会准备 | P1 |
| Redis 运行配置 | 可淘汰缓存与认证状态共享 `allkeys-lru` 实例 | P1 |

## 详细发现

### Correctness

#### [CORR-001] 遗留 `backend/uploads` 目录持续破坏 Go-only 契约

- **严重程度 / 排期 / 证据**：Medium / P0 / 实测
- **位置**：`backend-go/tests/contract/runtime_entry_surface_test.go:123`；仓库根目录 `backend/uploads/`
- **证据**：`go test ./... -count=1` 的唯一失败是 `TestLegacyPythonBackendDirectoryIsAbsent`，其余 Go 包通过。当前目录为空，但其存在与迁移文档声明的“旧 Python 后端已删除”不一致。
- **影响**：全仓测试无法绿色通过，CI 无法区分真实回归与已知残留，迁移完成契约失去持续保护。
- **建议**：确认目录没有需要保留的上传数据后删除整个 `backend/` 残留；上传继续使用根目录 `uploads/`。不要放宽契约测试。
- **验收指标**：`go test ./tests/contract -count=1` 和 `go test ./... -count=1` 全部通过。
- **实施成本**：S；需要一次数据归属确认。

#### [CORR-002] Go 工具链声明、镜像与文档版本不一致

- **严重程度 / 排期 / 证据**：Medium / P1 / 实测
- **位置**：`backend-go/go.mod:5`、`backend-go/Dockerfile:3`、`README.md:27`、`docs/technical/development.md:5`
- **证据**：`go.mod` 声明 `toolchain go1.25.12`，Docker builder 和文档仍固定为 1.25.10。
- **影响**：Docker 构建可能在固定镜像内再次自动下载 1.25.12，离线构建可能失败，开发机与镜像也可能使用不同补丁版本。
- **建议**：统一为同一补丁版本，并在 Docker 构建中显式验证 `go version`；版本变更时同步 `go.mod`、Dockerfile、README 和开发指南。
- **验收指标**：本地、CI、builder 的 `go version` 一致，Docker 构建不发生隐式工具链下载。
- **实施成本**：S。

### Security

#### [SEC-001] 安全状态与可淘汰缓存共享 `allkeys-lru` Redis

- **严重程度 / 排期 / 证据**：Medium / P1 / 静态高概率
- **位置**：`docker-compose.yml:60`、`backend-go/internal/platform/redis/client.go:9`、`backend-go/internal/application/auth/limiter.go:15`、`backend-go/internal/application/auth/refresh_session.go:14`
- **证据**：同一 Redis client 承载 cache、rate limit 和短期状态，实例设置 512 MB `allkeys-lru`。登录失败计数、锁定键、验证码和一次性 refresh session 都可能参与淘汰。
- **影响**：内存压力下，锁定键被淘汰会缩短登录保护；refresh session 被淘汰会导致用户被动登出。淘汰返回“键不存在”而不是 Redis 错误，现有本地 fallback 不会接管该状态。
- **建议**：将认证/限流状态放入独立的 no-eviction Redis 实例，或至少为安全状态建立容量告警、淘汰计数告警和压测验收；普通缓存继续使用 LRU 实例。
- **验收指标**：压力测试中 `evicted_keys=0`（安全状态实例），多实例锁定和 refresh rotation 在缓存满载时仍保持契约。
- **实施成本**：M；涉及部署配置和容量规划。

依赖扫描结果：`npm audit --omit=dev` 为 0 个漏洞；`govulncheck ./...` 未发现代码路径可达漏洞。后者仍报告依赖模块中存在未调用的漏洞，需持续升级和复扫。

### Performance

#### [PERF-001] 教师聚合接口存在串行查询扇出

- **严重程度 / 排期 / 证据**：Medium / P1 / 静态高概率
- **位置**：`backend-go/internal/application/teacher/service.go:404`、`:463`、`:525`、`:631`、`:662`、`:702`、`:725`
- **证据**：`GetAnalytics` 在非空数据下最多串行执行约 10 次 Repository 调用；`GetClassAnalytics` 约 12 次，并重复加载学生、画像、名称、分数、活跃度和排行数据。
- **影响**：接口延迟近似累加数据库往返时间；并发教师访问时还会增加连接池占用和数据库调度开销。
- **建议**：先记录每路由 SQL 次数和耗时，再为教师仪表盘建立少量专用 read-model 查询，用 CTE/聚合一次返回相关指标。只有无法合并且彼此独立的查询才考虑有上限的并发，避免简单并发放大连接池压力。
- **验收指标**：固定数据集下 SQL 往返次数至少下降 50%，接口 P95 降低且数据库 CPU、锁等待和连接池等待不恶化。
- **实施成本**：M-L。

#### [PERF-002] 学习概览重复扫描同一学生的作答数据

- **严重程度 / 排期 / 证据**：Medium / P1 / 静态高概率
- **位置**：`backend-go/internal/application/progress/service.go:249`、`:275`、`:281`、`:285`、`:289`、`:293`
- **证据**：`GetOverview` 分别查询累计学习时长、今日时长、今日作答数、连续天数和最后作答；连同 profile/mastery，正常路径约 7 次串行调用。
- **影响**：`content_attempts` 会被同一请求多次按 student/time 范围扫描，学生历史增长后查询成本和连接占用同步增长。
- **建议**：用一个聚合 read model 同时返回累计/今日/最近作答指标；连续天数可保留独立的有界查询或维护按日汇总表，先以 `EXPLAIN (ANALYZE, BUFFERS)` 决定。
- **验收指标**：概览 SQL 次数压缩到 2-3 次；在 1 万、10 万次历史作答数据量下记录 P50/P95 和 buffer hit/read。
- **实施成本**：M。

#### [PERF-003] 筛选知识图谱仍读取全量关系后在 Go 中丢弃

- **严重程度 / 排期 / 证据**：Medium / P1 / 静态高概率
- **位置**：`backend-go/internal/application/progress/service.go:450`、`:464`；`backend-go/internal/adapter/postgres/progress_repository.go:227`
- **证据**：节点查询支持章节、类型和搜索过滤，但 `ListKnowledgeRelations` 无过滤条件并返回所有关系；应用层随后按已选节点 ID 过滤边。
- **影响**：图谱扩大后，数据库传输、Go 分配和遍历成本与全图边数相关，而不是与用户筛选结果相关。
- **建议**：先取得筛选节点 ID，再通过 `source_id = ANY(...) AND target_id = ANY(...)` 查询关系，或在一个 CTE 中完成节点和边筛选。继续保留全图查询给学习路径用例。
- **验收指标**：筛选请求返回边数与扫描/传输边数同阶；10 万边数据集下记录延迟和分配量。
- **实施成本**：M。

#### [PERF-004] Prometheus 指标不足以建立性能基线

- **严重程度 / 排期 / 证据**：Medium / P0 / 实测
- **位置**：`backend-go/internal/platform/metrics/metrics.go:12`、`:34`；`backend-go/internal/platform/middleware/middleware.go:182`、`:203`
- **证据**：指标只包含无标签的进程总请求数；请求时长仅写入日志，没有规范化路由、方法、状态码、时延直方图、响应大小或数据库连接池指标。
- **影响**：无法计算登录、练习提交、教师聚合等接口的 P50/P95/P99，也无法把延迟回归关联到错误率或连接池饱和。
- **建议**：在路由模板确定后记录低基数的 `{route,method,status_class}` 计数和时延直方图，并暴露 pgx pool 的 acquired/idle/max、acquire duration 与 Redis pool/命令错误。禁止使用原始 URL、用户 ID 或 request ID 作为 label。
- **验收指标**：Prometheus 可直接查询五个核心流程的吞吐、错误率和 P95/P99；压力测试期间 label 基数保持有界。
- **实施成本**：M。

#### [PERF-005] 前后通配搜索缺少可用索引策略

- **严重程度 / 排期 / 证据**：Medium / P1 / 静态高概率
- **位置**：`backend-go/internal/adapter/postgres/teacher_repository.go:78`、`progress_repository.go:201`、`admin_user_repository.go:205`；`backend-go/migrations/0001_initial_schema.up.sql:586`
- **证据**：用户、知识点、题目和资源搜索普遍使用 `ILIKE '%term%'` 或 `lower(column) LIKE '%term%'`，当前相关索引是普通 B-tree，且迁移中没有 `pg_trgm`。
- **影响**：数据量增大后高概率退化为顺序扫描；分页 count 和 data 查询会重复承担搜索成本。
- **建议**：先采集慢查询并用真实选择性执行 `EXPLAIN (ANALYZE, BUFFERS)`；确认热点后启用 `pg_trgm` 并为标准化表达式建立 GIN/GiST 索引。短前缀或低选择性搜索应限制最小长度，不能无条件增加所有索引。
- **验收指标**：目标查询不再对大表执行全表扫描，写入开销和索引体积处于容量预算内。
- **实施成本**：M。

#### [PERF-006] 所有 API 都使用 SSE 级代理配置

- **严重程度 / 排期 / 证据**：Low / P2 / 待验证假设
- **位置**：`frontend/nginx.conf:33`、`:43`；`nginx-site.conf:30`、`:40`；SSE 路径实现位于 `backend-go/internal/adapter/http/session/handler.go:295`
- **证据**：整个 `/api/` 关闭响应缓冲、请求缓冲和 Nginx gzip，并使用长超时；实际只有会话流式接口需要 SSE 语义。Go 中间件仍可压缩普通响应，因此本轮没有测得实际损失。
- **影响**：普通 JSON 接口可能失去 Nginx 缓冲带来的慢客户端隔离和更高效的代理写出；全局长超时也会放大异常连接占用。
- **建议**：仅在运行态基线证明代理层是瓶颈后，将 SSE/上传路径拆成专用 location，普通 JSON 恢复默认缓冲和较短超时。未经压测不直接修改。
- **验收指标**：普通 JSON 吞吐或上游连接占用改善，同时 SSE 首 token 延迟、断连和上传行为不回归。
- **实施成本**：S-M。

### Readability

#### [READ-001] 核心用例和页面文件承担过多职责

- **严重程度 / 排期 / 证据**：Medium / P1 / 实测结构证据
- **位置**：`backend-go/internal/application/exercise/service.go`（1,937 行）、`teacher/service.go`（1,078 行）、`frontend/src/pages/admin/SystemSettingsPage.tsx`（915 行）
- **证据**：练习 service 同时包含选题、AI 生成、OCR、判题、解题验证、诊断、事务写入和 DKT 更新；教师 service 同时承载多个统计 read model；系统设置页组合多个独立管理域。
- **影响**：性能优化容易触碰无关契约，review 和测试定位成本高，也难以为单个热点建立窄基准。
- **建议**：保持现有 package 与公开接口，先按用例拆分同包文件和私有协作者，例如 `next_exercise`、`submission`、`solution`、`tracking`；前端按设置域拆出受控 section/hook。不要为了行数引入新框架或跨层抽象。
- **验收指标**：公共 API 与测试契约不变；热点用例可单独测试/基准；单次修改影响文件和 mock surface 明显缩小。
- **实施成本**：M。

### Testing

#### [TEST-001] 前端覆盖率低且没有门槛

- **严重程度 / 排期 / 证据**：Medium / P1 / 实测
- **位置**：`frontend/vitest.config.ts:17`；覆盖率报告中的 modules/services/stores/pages
- **证据**：407 项测试全部通过，但整体 statements 29.64%、branches 25.71%、functions 21.63%、lines 30.74%；Vitest 配置没有 thresholds。多个管理端、教师端页面和 service 接近 0%。
- **影响**：拆包、请求去重、selector 和异步流程优化缺少回归保护，覆盖率可在 CI 中继续下降而不失败。
- **建议**：不要立即要求全仓 80%；先为认证、练习、会话、题库导入、图谱转换和公共 HTTP 层建立分目录门槛，再逐步提高到核心业务 80% 以上。
- **验收指标**：CI 阻止核心目录覆盖率下降；新增/修改公共函数包含成功、边界和失败路径测试。
- **实施成本**：M-L。

#### [TEST-002] PostgreSQL Repository 是后端最大覆盖缺口

- **严重程度 / 排期 / 证据**：Medium / P1 / 实测
- **位置**：`backend-go/internal/adapter/postgres/`；`docs/TODO.md` 的 PostgreSQL Repository 集成测试项
- **证据**：Go 总 statements 58.1%，application/exercise 82.5%，但 PostgreSQL adapter 只有 12.4%；现有少量 integration test 依赖外部 DSN，未覆盖多数聚合查询、默认值和索引行为。
- **影响**：最需要优化的 SQL read model 缺少真实 schema、扫描类型、事务和执行计划保护，mock 测试不能发现这些问题。
- **建议**：在 CI 启动 PostgreSQL/pgvector，按高风险查询逐步补集成测试；性能断言使用独立基准/计划快照，避免把毫秒阈值写入普通单元测试。
- **验收指标**：教师、进度、练习核心 Repository 的公共行为覆盖达到 80% 左右，并验证 schema、事务和错误路径。
- **实施成本**：L。

#### [TEST-003] 现有 benchmark 没有覆盖核心用户流程

- **严重程度 / 排期 / 证据**：Medium / P0 / 实测
- **位置**：`middleware_test.go:201`、`upload/service_test.go:98`、`ratelimit/limiter_test.go:189`；`docs/TODO.md` 的性能基线项
- **证据**：仓库只有 3 个 Go benchmark；没有登录、题目列表、练习提交、学习会话、教师聚合或前端交互性能基线。
- **影响**：无法量化 query consolidation、缓存、JSON 编码或组件拆分的收益，也无法发现后续性能回归。
- **建议**：建立固定规模数据夹具、API 负载脚本和 `benchstat` 基线；结果记录 P50/P95/P99、吞吐、错误率、SQL 次数、分配与连接池等待。外部 AI 路径用可控 fake，另做真实 provider 验收。
- **验收指标**：五个核心流程具备可重复命令、环境说明和历史基线，优化 PR 必须给出前后对比。
- **实施成本**：M-L。

### Architecture

#### [ARCH-001] 可选图谱运行时没有隔离到页内异步边界

- **严重程度 / 排期 / 证据**：Medium / P1 / 实测构建产物
- **位置**：`frontend/src/app/routes/adminRoutes.ts:10`、`KnowledgeManagementPage/index.tsx:51`、`:85`、`:307`
- **证据**：管理页面本身已路由懒加载，这是正确边界；但页面静态导入 `KnowledgeGraphEditor`，且进入默认节点 tab 时就请求 `fetchAllNodesSimple`。构建产物的 G6 主块为 1,390,113 B raw / 392,334 B Gzip。
- **影响**：只管理节点列表的用户也需要下载/解析 G6 主块并提前加载全量简单节点，低端设备和慢网络的管理页开销偏高。
- **建议**：在 `activeTab === 'graph'` 内使用 `React.lazy`/动态 import 加载 editor，并在关系 modal 或图谱 tab 首次打开时再加载全量节点。保留路由级懒加载和错误边界。
- **验收指标**：默认知识节点 tab 不请求 G6 chunks 和全量节点；切换图谱 tab 后功能、加载态和重试正常；首屏与路由 chunk 预算进入 CI。
- **实施成本**：S-M。

## 性能基线

### Go 微基准

每项在同一 Windows/amd64 主机运行 3 次：

| Benchmark | 结果范围 | 分配 |
|-----------|----------|------|
| `BenchmarkGzipMiddleware-32` | 21,797-21,947 ns/op | 56-71 B/op，3 allocs/op |
| `BenchmarkTextResourceReader-32` | 116,410-117,347 ns/op；767.91-774.09 MB/s | 48 B/op，1 alloc/op |
| `BenchmarkLimiterLocalParallel-32` | 25.29-26.50 ns/op | 0 B/op，0 allocs/op |

这些结果证明局部实现没有明显分配异常，不代表 API、数据库或网络性能。

### 前端生产资源

`npm run build` 后，从 `dist/index.html` 引用关系计算：

| 资源集合 | Raw | Gzip | Brotli |
|----------|----:|-----:|-------:|
| 初始 HTML 引用的 JS/CSS 合计 | 702,470 B | 200,201 B | 172,667 B |
| G6 图谱主块 | 1,390,113 B | 392,334 B | 315,215 B |
| ECharts Canvas renderer（分析/仪表盘异步块） | 507,053 B | 169,335 B | 143,498 B |
| KaTeX 主块 | 266,427 B | 76,625 B | 62,785 B |
| Markdown 内容块 | 173,714 B | 51,237 B | 44,062 B |

资源 hash 会随构建变化。G6、ECharts、KaTeX 和 Markdown 均为异步路由/功能块，不能与首屏大小相加。

## 已通过的控制

- `go vet ./...` 通过。
- 认证、会话、练习、Redis 和限流包的 `go test -race` 通过。
- Go 配置了连接池上限、连接/statement/idle transaction timeout；常见 attempt 时间范围也已有组合索引。
- Repository 查询使用参数占位符；本轮未发现直接拼接用户值的 SQL 注入路径。
- 前端按页面使用 `React.lazy`，重型图谱与数学/Markdown 依赖没有进入初始资源集合。
- 前端 60 个测试文件、407 项测试通过；ESLint 0 error；TypeScript/Vite 生产构建通过。
- `npm audit --omit=dev` 为 0；`govulncheck` 未发现代码路径可达漏洞。

## 建议顺序

1. **P0：建立可观测性能基线。** 先补路由时延/状态、SQL 次数与连接池指标，再对五个核心流程压测；同时清理遗留 `backend/` 目录使全仓测试恢复绿色。
2. **P1：优化已定位读模型。** 合并教师分析和进度概览查询，限制图谱关系读取范围，并在真实数据上验证 trigram 索引。
3. **P1：降低前端可选功能成本。** 图谱 editor 和全量节点请求延迟到使用时加载，建立首屏/路由块预算。
4. **P1：补回归保护。** 优先补 PostgreSQL 集成测试和前端核心 services/stores 覆盖率，再进行大文件内聚拆分。
5. **P1：隔离 Redis 安全状态。** 在缓存容量压测和淘汰指标基础上决定独立实例与 no-eviction 策略。
6. **P2：验证代理层机会。** 只有实测表明普通 JSON API 受全局 SSE 配置影响时，才拆分 Nginx location。

## 验证记录

| 命令 | 结果 |
|------|------|
| `go test ./... -count=1` | FAIL：仅 `TestLegacyPythonBackendDirectoryIsAbsent`；其余包通过 |
| 排除已知 contract 的 `go test ... -coverprofile` | PASS：total statements 58.1%，PostgreSQL adapter 12.4% |
| `go vet ./...` | PASS |
| 五个高并发/状态包 `go test -race` | PASS |
| 三个现有 benchmark `-benchmem -count=3` | PASS，结果见性能基线 |
| `npm test` | PASS：60 files / 407 tests |
| `npm run test:coverage` | PASS：statements 29.64%，lines 30.74% |
| `npm run lint` | PASS：0 errors；6 个 warning 均来自生成的 `coverage/` 报告 |
| `npm run build` | PASS |
| `npm audit --omit=dev` | PASS：0 vulnerabilities |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | PASS：0 个代码路径可达漏洞 |

## 边界与残余风险

- 本轮没有真实 PostgreSQL 数据量、`EXPLAIN ANALYZE`、API 并发、浏览器网络瀑布或生产 Prometheus 数据，因此不声称具体 P95/P99、吞吐提升或索引收益。
- 微基准和压缩体积来自单台 Windows 开发机；跨平台绝对值不可直接比较，后续应在固定 CI/Linux 环境建立基线。
- 外部 AI、对象存储和西电教务集成没有在线压测；相关 provider 质量和 token 流式验收仍按现有 TODO 与迁移跟踪执行。
- 本报告不是完整渗透测试；安全维度只覆盖与代码/性能架构直接相关的静态检查和依赖扫描。
