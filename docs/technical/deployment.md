# 部署指南

## 部署组成

根目录 `docker-compose.yml` 编排四个服务：

| 服务 | 默认实现 | 容器端口 |
|------|----------|----------|
| PostgreSQL | `pgvector/pgvector:pg18-trixie` | 5432 |
| Redis | `redis:7-alpine` | 6379 |
| Backend | `backend-go/Dockerfile` 构建的 Go API | 8000 |
| Frontend | `frontend/Dockerfile` 构建的 Nginx 静态站点 | 80 |

PostgreSQL、Redis 和 Go API 默认只绑定宿主机回环地址；前端默认发布到宿主机 `9000` 端口。

## 准备环境

```powershell
Copy-Item .env.example .env
```

生产环境至少要替换数据库密码、`JWT_SECRET_KEY`、`FERNET_SECRET_KEY`、初始管理员密码、CORS、管理网段和对象存储凭据。设置 `ENVIRONMENT=production`，不要把开发密钥或真实 `.env` 提交到仓库。

## 构建与启动

```powershell
docker compose build
docker compose up -d postgres redis
```

数据库健康后执行 Go migration runner，再启动应用服务。仓库脚本 `scripts/deploy.sh` 和 `scripts/update.sh` 已按这一顺序处理；手工部署时可使用：

```powershell
Set-Location backend-go
go run ./cmd/migrate
Set-Location ..
docker compose up -d backend frontend
```

默认生产链路不运行 Python 或 Alembic。

## 反向代理

`frontend/nginx.conf` 负责前端容器内的静态资源和 API 转发，根目录 `nginx-site.conf` 可用于站点级反向代理。部署时应确认：

- `/api/` 指向 Go API；
- SSE 路径关闭不必要的代理缓冲并保留足够超时；
- 上传大小限制与后端配置一致；
- TLS、HSTS、CSP 和其他安全响应头由边缘代理统一设置；
- `/metrics` 和详细健康信息只对管理网络开放。

## 上线验证

```powershell
docker compose ps
docker compose logs --tail 200 backend
```

至少验证：

1. `/health` 返回成功，数据库和 Redis 容器健康。
2. 前端页面可以加载并调用 `/api/v1`。
3. 登录、刷新令牌和角色权限符合预期。

4. 数据库迁移首次执行有新增版本，重复执行无待应用版本。
5. 文件上传、对象存储、外部 AI provider 和西电教务按部署配置进行连通性验证；`ocr` Agent 必须选择支持图片输入的模型。
6. 分别提交真实 PNG、JPEG 图片和空白/低对比图片，确认成功路径只产生一次 attempt，并各执行一次 session、DKT 和 profile 更新；OCR/数学不确定或失败路径的这些写入均为零。图片 OCR 当前只接受 PNG、JPEG 和 GIF。
7. 验证通用数学判定的 `correct`、`incorrect`、`indeterminate` 响应，以及解析生成不可用、超时、取消、无效输出和验证失败的 `failure.stage`、`failure.code`、`retryable` 契约。

仓库内非网络验收覆盖真实 PNG/JPEG 的上传、存储回读、多模态 Base64 传递和学习状态写入边界：

```powershell
Set-Location backend-go
go test ./internal/adapter/llm/einoagent -run 'TestAnswerImageSubmission' -count=1 -v
```

发布环境还应使用目标视觉 provider 执行 live OCR 质量验收。测试包含 `x+1`、`42`、空白 PNG 和低对比 JPEG；凭据只通过环境变量提供，不写入仓库：

```powershell
$env:MSP_LIVE_OCR_ACCEPTANCE = '1'
$env:MSP_OCR_ACCEPTANCE_BASE_URL = 'https://provider.example.com/v1'
$env:MSP_OCR_ACCEPTANCE_API_KEY = '<secret>'
$env:MSP_OCR_ACCEPTANCE_MODEL = '<vision-model>'
go test ./internal/adapter/llm/einoagent -run 'TestLiveAnswerOCR' -count=1 -v
```

目标 Math Solver provider 的通用题型质量验收使用独立开关，覆盖三角恒等、极限、不定积分、方程解集、矩阵、证明和错误步骤拒绝：

```powershell
$env:MSP_LIVE_MATH_ACCEPTANCE = '1'
$env:MSP_MATH_ACCEPTANCE_BASE_URL = 'https://provider.example.com/v1'
$env:MSP_MATH_ACCEPTANCE_API_KEY = '<secret>'
$env:MSP_MATH_ACCEPTANCE_MODEL = '<math-model>'
go test ./internal/adapter/llm/einoagent -run 'TestLiveMathSolver' -count=1 -v
```

登录安全验证使用 Redis 保存短时一次性票据。生产环境必须保持 Redis 可用，并可通过以下环境变量调整策略：

- `LOGIN_CAPTCHA_TTL_SECONDS`：拼图挑战有效期，默认 120 秒。
- `LOGIN_CAPTCHA_PROOF_TTL_SECONDS`：验证通过后登录票据有效期，默认 120 秒。
- `LOGIN_CAPTCHA_TOLERANCE_PIXELS`：拼图位置容差，默认 6 像素。
- `LOGIN_CAPTCHA_ISSUE_LIMIT`：单客户端在窗口内最多签发数量，默认 10。
- `LOGIN_CAPTCHA_ISSUE_WINDOW_SECONDS`：签发限频窗口，默认 60 秒。

反向代理需要覆盖写入 `X-Real-IP`；仓库内 Nginx 配置已包含该请求头。验证码图片和校验响应均禁止缓存。

尚未完成的运行时验收范围记录在 [项目待办](../TODO.md)。

## 更新与回滚

- 更新前备份 PostgreSQL 和持久化上传目录。
- 使用 `scripts/update.sh` 或按“迁移后启动应用”的顺序滚动更新。
- 数据迁移不提供自动 down migration；失败时恢复备份，或发布经过评审的补偿性 forward migration。
- 应用镜像回滚前必须确认旧版本能够读取当前数据库结构。
- 回滚后重新执行健康检查、认证和核心业务 smoke。
