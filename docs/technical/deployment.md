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

迁移 runner 不随 API 自动执行。合并包含 `backend-go/migrations/*.up.sql` 的变更后，必须在启动新版本 API 前执行一次迁移，并确认重复执行没有待应用版本：

```powershell
Set-Location backend-go
go run ./cmd/migrate
go run ./cmd/migrate
Set-Location ..
```

首次命令应记录实际 `applied_count`，第二次应为 `applied_count=0`。也可在数据库中核对已应用版本：

```sql
SELECT version, name, applied_at
FROM public.go_schema_migrations
ORDER BY version;
```

## 反向代理

`frontend/nginx.conf` 负责前端容器内的静态资源和 API 转发，根目录 `nginx-site.conf` 可用于站点级反向代理。部署时应确认：

默认请求超时按工作负载分层：

| 层级 | 默认超时 |
| --- | ---: |
| 普通 Go API 总请求预算 | 30 秒 |
| `POST /api/v1/exercise/generate` 总请求预算 | 55 秒 |
| 前端生成请求 Axios 超时 | 60 秒 |
| Nginx `/api/` 上游响应读取超时 | 300 秒 |
| Go HTTP `WriteTimeout` | 310 秒 |

`EXERCISE_GENERATION_REQUEST_TIMEOUT_SECONDS` 是生成、独立求解、验证及最多一次重试共享的总预算。生产环境通常应保持默认 55 秒；如需配置到 60 秒以上，必须同步调整前端请求超时，如需超过 300 秒还必须同步调整所有边缘代理。Nginx 的 300 秒仅是代理安全上限，不代表业务请求应持续运行 300 秒。

- `/api/` 指向 Go API；
- SSE 路径关闭不必要的代理缓冲并保留足够超时；
- 上传大小限制与后端配置一致；
- TLS、HSTS、CSP 和其他安全响应头由边缘代理统一设置；
- `/metrics` 和详细健康信息只对管理网络开放。

`scripts/deploy.sh` 只会为首次部署或重新执行部署脚本时生成站点配置。已经部署的服务器不会因代码更新自动改写 `/etc/nginx`；应先用 `sudo nginx -T` 确认实际生效的站点文件，再将 `/api/` location 中的 `proxy_read_timeout` 调整为 `300s`，随后执行：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

不要在通用更新脚本中自动编辑系统 Nginx；自定义域名、面板托管、Ingress、云负载均衡或其他边缘代理应在各自配置入口应用相同的响应读取上限。

## 监控指标

`GET /metrics` 使用 Prometheus text exposition，并保留既有无标签总计 `msp_http_requests_total`。新增指标包括：

- `msp_http_server_requests_total{method,route,status_class}`：按 HTTP 方法、ServeMux 路由模板和状态类别统计请求量。
- `msp_http_server_request_duration_seconds`：使用相同低基数标签的请求时延直方图。
- `msp_postgres_pool_*`：pgx 连接上限、当前 total/acquired/idle/constructing、获取次数、等待和取消。
- `msp_redis_pool_*`：go-redis 当前连接、连接复用命中/未命中、等待、超时和不可用连接。

`route` 只使用注册路由模板；未匹配请求和 CORS preflight 使用固定占位符。不要把原始 URL、用户 ID、request ID 或错误文本加入 label。常用查询示例：

```promql
# 各路由 5 分钟 P95
histogram_quantile(
  0.95,
  sum by (le, method, route) (
    rate(msp_http_server_request_duration_seconds_bucket[5m])
  )
)

# PostgreSQL 连接池占用率
msp_postgres_pool_connections{state="acquired"}
  / msp_postgres_pool_max_connections
```

部署告警至少应覆盖 HTTP 5xx 比例、核心路由 P95/P99、PostgreSQL canceled/empty acquire 增长，以及 Redis pool timeout/wait 增长。

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
5. 文件上传、对象存储、外部 AI provider 和西电账户绑定按部署配置进行连通性验证；`ocr` Agent 必须选择支持图片输入的模型。
6. 分别提交真实 PNG、JPEG 图片和空白/低对比图片，确认成功路径只产生一次 attempt，并各执行一次 session、DKT 和 profile 更新；OCR/数学不确定或失败路径的这些写入均为零。图片 OCR 当前只接受 PNG、JPEG 和 GIF。
7. 验证通用数学判定的 `correct`、`incorrect`、`indeterminate` 响应，以及解析生成不可用、超时、取消、无效输出和验证失败的 `failure.stage`、`failure.code`、`retryable` 契约。

仓库不永久保留验收测试源码。发布前按 [开发指南](development.md) 临时创建非网络验收用例，覆盖真实 PNG/JPEG 的上传、存储回读、多模态 Base64 传递和学习状态写入边界，运行并记录结果后删除：

```powershell
Set-Location backend-go
go test ./internal/adapter/llm/einoagent -run 'TestAnswerImageSubmission' -count=1 -v
```

发布环境还应使用目标视觉 provider 执行 live OCR 质量验收。临时用例包含 `x+1`、`42`、空白 PNG 和低对比 JPEG；凭据只通过环境变量提供，不写入仓库，用例通过后立即删除：

```powershell
$env:MSP_LIVE_OCR_ACCEPTANCE = '1'
$env:MSP_OCR_ACCEPTANCE_BASE_URL = 'https://provider.example.com/v1'
$env:MSP_OCR_ACCEPTANCE_API_KEY = '<secret>'
$env:MSP_OCR_ACCEPTANCE_MODEL = '<vision-model>'
go test ./internal/adapter/llm/einoagent -run 'TestLiveAnswerOCR' -count=1 -v
```

目标 Math Solver provider 的通用题型质量验收使用独立开关和临时用例，覆盖三角恒等、极限、不定积分、方程解集、矩阵、证明和错误步骤拒绝；记录结果后删除用例源码：

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

- 使用 `scripts/update.sh <镜像标签>` 或按“确认数据库、拉取镜像、停止应用写入、备份数据、迁移、启动新应用”的顺序更新。脚本确认 PostgreSQL 可用并拉取新镜像时旧应用仍保持运行，随后只停止 backend/frontend，不停止数据库和 Redis。
- 更新脚本在迁移前创建权限收紧的 `backups/<时间戳>/`，保存 `.env`、解析后的 Compose 配置、旧镜像引用及不可变镜像 ID、PostgreSQL custom-format dump，以及存在时的 `uploads.tar.gz`。可通过 `BACKUP_ROOT` 修改备份根目录。该目录被 Git 忽略，但包含生产凭据和业务数据，仍需限制访问并按运维保留策略清理。
- `postgres.dump` 失败或为空时脚本不会执行迁移，并尝试重新启动原应用容器；迁移失败或新版本健康检查失败时应用保持停止，避免在未知 schema 或不健康状态下继续提供服务。
- `uploads/` 不在默认路径时，通过 `MSP_UPLOADS_BACKUP_DIR` 指定宿主机持久化目录。使用 S3/七牛等外部对象存储时，仍需遵循对应供应商的版本与备份策略。
- 数据迁移不提供自动 down migration；失败时恢复备份，或发布经过评审的补偿性 forward migration。
- 应用镜像回滚前必须确认旧版本能够读取当前数据库结构。
- 回滚后重新执行健康检查、认证和核心业务 smoke。

需要恢复备份时，先停止应用并保留故障现场，再使用对应备份目录。以下命令会覆盖当前数据库与本地上传目录，只能在确认目标目录和恢复窗口后执行：

```bash
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
BACKUP_ROOT="${BACKUP_ROOT:-backups}"
BACKUP_DIR="${BACKUP_ROOT}/20260723_120000"
docker compose -f "${COMPOSE_FILE}" stop backend frontend
docker compose -f "${COMPOSE_FILE}" up -d postgres
for attempt in {1..30}; do
    if docker compose -f "${COMPOSE_FILE}" exec -T postgres sh -ec 'pg_isready -q -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"'; then
        break
    fi
    if [ "$attempt" -eq 30 ]; then
        echo "PostgreSQL 未在 60 秒内就绪" >&2
        exit 1
    fi
    sleep 2
done

# 先验证 archive，再清空项目 schema；仅使用 --clean 无法删除备份后新增的表。
docker compose -f "${COMPOSE_FILE}" exec -T postgres sh -ec 'exec pg_restore --list >/dev/null' < "${BACKUP_DIR}/postgres.dump"
docker compose -f "${COMPOSE_FILE}" exec -T postgres sh -ec 'exec psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}" -c "DROP SCHEMA public CASCADE"'
docker compose -f "${COMPOSE_FILE}" exec -T postgres sh -ec 'exec pg_restore --exit-on-error --clean --if-exists --no-owner --no-privileges -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"' < "${BACKUP_DIR}/postgres.dump"

# 仅当备份中存在 uploads.tar.gz 且使用本地存储时执行。
UPLOADS_DIR="${MSP_UPLOADS_BACKUP_DIR:-uploads}"
if [ -f "${BACKUP_DIR}/uploads.tar.gz" ]; then
    if [ -e "${UPLOADS_DIR}" ]; then
        mv "${UPLOADS_DIR}" "${UPLOADS_DIR}.failed-$(date +%Y%m%d_%H%M%S)"
    fi
    mkdir -p "$(dirname -- "${UPLOADS_DIR}")"
    tar -xzf "${BACKUP_DIR}/uploads.tar.gz" -C "$(dirname -- "${UPLOADS_DIR}")"
elif [ -f "${BACKUP_DIR}/uploads.absent.txt" ]; then
    if [ -e "${UPLOADS_DIR}" ]; then
        mv "${UPLOADS_DIR}" "${UPLOADS_DIR}.failed-$(date +%Y%m%d_%H%M%S)"
    fi
    echo "备份时上传目录不存在，当前目录已保留为故障现场"
fi
```

`previous-images.txt` 同时保存原镜像引用和容器实际使用的镜像 ID。需要回滚镜像时，先确认两个 ID 在本机仍存在，再把它们标记为同一个临时版本并启动应用：

```bash
: "${DOCKER_USERNAME:?请先导出原镜像仓库用户名}"
export DOCKER_USERNAME
ROLLBACK_TAG="rollback-20260723_120000"
BACKEND_IMAGE_ID="$(sed -n 's/^backend_image_id=//p' "${BACKUP_DIR}/previous-images.txt")"
FRONTEND_IMAGE_ID="$(sed -n 's/^frontend_image_id=//p' "${BACKUP_DIR}/previous-images.txt")"
docker image inspect "${BACKEND_IMAGE_ID}" "${FRONTEND_IMAGE_ID}" >/dev/null
docker tag "${BACKEND_IMAGE_ID}" "${DOCKER_USERNAME}/backend-go:${ROLLBACK_TAG}"
docker tag "${FRONTEND_IMAGE_ID}" "${DOCKER_USERNAME}/frontend:${ROLLBACK_TAG}"
export IMAGE_VERSION="${ROLLBACK_TAG}"
docker compose -f "${COMPOSE_FILE}" up -d backend frontend
```

恢复备份中的 `.env` 后再使用上述旧镜像。数据库已成功完成 forward migration、且旧镜像确认兼容新 schema 时，可以只回滚镜像；否则必须先恢复数据库或发布补偿迁移，不能只复制旧 `.env` 后直接启动。
