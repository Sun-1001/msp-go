# 开发指南

## 环境要求

- Go 1.25.12（`go.mod` 声明 `go 1.25.0` 和 `toolchain go1.25.12`）
- Node.js 20 和 npm
- PostgreSQL 18 + pgvector
- Redis 7

版本变化时以 [go.mod](../../backend-go/go.mod)、[package.json](../../frontend/package.json) 和 [docker-compose.yml](../../docker-compose.yml) 为准。

## 首次启动

在仓库根目录创建本地环境文件：

```powershell
Copy-Item .env.example .env
```

启动后端前先执行迁移：

```powershell
Set-Location backend-go
go mod download
go run ./cmd/migrate
go run ./cmd/api
```

另开终端启动前端：

```powershell
Set-Location frontend
npm install
npm run dev
```

根目录 `start.bat` 可在 Windows 上同时打开前后端进程，但不会替代首次数据库迁移。

## 常用验证命令

Go 后端：

```powershell
Set-Location backend-go
go vet ./...
go build ./...
gofmt -w <changed-go-files>
```

前端：

```powershell
Set-Location frontend
npm run lint
npm run build
```

提交前在仓库根目录运行：

```powershell
git diff --check
git status --short
```

## 临时测试规则

仓库不永久保留或提交测试用例源码。生产代码完成后，才按本次变更创建临时 `*_test.go`、`*.test.ts(x)` 或 `*.spec.ts(x)`；测试范围覆盖公共行为、边界输入、错误条件和外部依赖降级，修改共享契约时同时做 Go 与前端临时契约验证。

测试运行器配置和依赖可以保留。临时测试存在时按需运行：

```powershell
# Go：只运行受影响包，必要时再扩大范围
Set-Location backend-go
go test <affected-packages> -count=1

# 前端：传入本次创建的临时测试文件
Set-Location ../frontend
npm test -- <temporary-test-path>
npm run test:coverage -- <temporary-test-path>
```

测试通过后先记录命令、结果和必要覆盖率，再按明确路径删除本次临时测试及其专用 fixture/mock；禁止使用宽泛递归删除。提交前在仓库根目录确认以下命令没有输出：

```powershell
git ls-files "*_test.go" "*.test.ts" "*.test.tsx" "*.test.js" "*.test.jsx" "*.spec.ts" "*.spec.tsx" "*.spec.js" "*.spec.jsx" "test_*.py" "*_test.py"
git diff --cached --name-only --diff-filter=ACMR | Select-String -Pattern '(_test\.go|\.(test|spec)\.(ts|tsx|js|jsx)|(^|/)test_.*\.py|_test\.py)$'
```

## 代码组织

### 前端

- 页面只负责布局和业务模块组合。
- API 调用放在 `src/modules/*/services/`，交互状态和业务流程放在模块 Hook 或 Store。
- 模块通过 `index.ts` 暴露公共接口，外部代码避免深层导入。
- 通用 UI 放入 `src/components/`，与业务绑定的组件留在对应模块。

### 后端

- `application` 表达用例、事务和业务规则。
- `adapter/http` 负责请求解析、鉴权、响应和协议错误映射。
- `adapter/postgres` 负责 SQL、扫描和持久化语义。
- `platform` 只承载跨领域基础能力，不放业务规则。
- 新外部依赖通过接口和适配器接入，并在临时测试中替换为 fake 或 mock。

完整协作约束见 [AGENTS.md](../../AGENTS.md)。

## 数据库迁移

新增迁移文件使用 `NNNN_description.up.sql` 命名，并放在 `backend-go/migrations/`。当前只使用 forward migration：

```powershell
Set-Location backend-go
go run ./cmd/migrate
go run ./cmd/migrate  # 重复执行应无待应用版本
```

不要重建或修改 `0001_initial_schema.up.sql` 来承载增量变化。生产回滚依赖备份恢复或经过评审的补偿性 forward migration，详见 [迁移策略](../../backend-go/migrations/README.md)。

## 环境配置

仓库根目录 `.env` 是本地和部署环境的统一文件名，`.env.example` 是唯一模板。至少应按环境修改：

- PostgreSQL、Redis 和连接池配置
- JWT、Fernet、管理员初始化凭据
- CORS 和管理端允许网段
- Eino provider 的兼容配置
- Local、Qiniu 或 S3 存储配置
- 西电账户绑定端点和超时

不要提交 `.env`、API key、密码或真实用户数据。
