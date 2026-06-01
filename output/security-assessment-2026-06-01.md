# MSP-Go 本地安全评估报告

评估时间：2026-06-01  
范围：`E:\code\msp-go` 本地 Go API、Vite 前端、PostgreSQL、Redis。  
方式：非破坏性本地动态探测、代码审计、依赖漏洞扫描、构建/测试验证。

## 本地运行状态

- Go API：`http://localhost:8000`，健康检查 `GET /health` 返回 `200 {"status":"healthy","version":"0.1.0"}`。
- 前端：`http://127.0.0.1:5173`，首页返回 `200`。
- PostgreSQL：`localhost:5432` 可达，迁移执行成功，应用了 `0002_replace_bkt_with_dkt`。
- Redis：`localhost:6379` 可达。
- 本轮为避免触碰真实对象存储，API 进程临时使用 `STORAGE_BACKEND=local` 和 `UPLOADS_DIR=E:\code\msp-go\uploads-test`。

## 高风险发现

### 1. 未认证用户可上传图片并公开访问

证据：

- 未带 `Authorization` 调用 `POST /api/v1/upload/image` 返回 `200`。
- 服务端返回公开 URL，例如 `/uploads/images/<uuid>.png`。
- 随后未认证 `GET /uploads/images/<uuid>.png` 返回 `200`，内容就是上传的测试文本。
- 代码位置：`backend-go/internal/adapter/http/upload/handler.go:62-63` 的 `image` 入口未调用鉴权；资源文件上传在 `handler.go:66-70` 才要求教师权限。
- 代码位置：`backend-go/internal/application/upload/service.go:83-86` 仅信任 multipart 的 `Content-Type`，未校验文件魔数或实际图片解码。

影响：

- 任意匿名访问者可消耗磁盘/对象存储容量。
- 攻击者可上传伪装成图片的任意字节内容，作为公开文件分发点使用。
- 如果后续代理/CDN 或浏览器 MIME 处理配置变弱，可能扩大为内容嗅探、钓鱼或存储型内容投放风险。

建议：

- 给 `POST /api/v1/upload/image` 增加至少登录用户鉴权，并记录上传者。
- 增加 per-user/IP 上传频率和容量限制。
- 使用 `http.DetectContentType`、图片解码或文件签名校验，不只信任客户端提交的 MIME。
- 对上传对象做隔离命名、审计日志、可选病毒扫描和过期清理。

### 2. 默认允许教师自助注册，普通访客可获得教师权限

证据：

- 动态注册教师账号 `POST /api/v1/auth/register {"role":"teacher"}` 返回 `200`。
- 该教师令牌随后可成功调用教师资源上传 `POST /api/v1/upload/resource`，返回 `200`。
- 数据库初始设置：`backend-go/migrations/0001_initial_schema.up.sql:477` 将 `allow_teacher_registration` 初始化为 `true`。
- 注册逻辑允许 student/teacher，只有 admin 被拒绝：`backend-go/internal/application/auth/service.go:179-192`。

影响：

- 公开部署时，任意访客可成为教师角色，进入教师功能面，例如班级、题库、资源、学生相关视图中所有由教师角色保护的接口。
- 这不是单个接口越权，而是默认权限模型过宽。

建议：

- 生产默认将 `allow_teacher_registration=false`。
- 教师注册改为管理员审批、邀请码、学校域名验证或后台创建。
- 首次部署脚本或迁移中避免默认开启高权限角色自助注册。

### 3. 西电教务账号密码以可逆混淆保存在浏览器 localStorage

证据：

- `frontend/src/modules/xidian/services/credentialStorage.ts:17-20` 将 `{ username, password }` 做 Base64 + 反转后写入 `localStorage`。
- `frontend/src/modules/xidian/services/credentialStorage.ts:26-33` 可直接反向恢复明文用户名和密码。

影响：

- 任意 XSS、恶意浏览器扩展、共享设备其他本地脚本都可以读取并还原教务密码。
- 这是第三方真实账号密码，风险高于普通站内 token 缓存。

建议：

- 不在前端持久化教务明文密码。
- 如必须记住绑定状态，应只保存服务器侧绑定 ID 或短期挑战状态。
- 密码只在提交绑定时传输一次，后端使用 Fernet/密钥管理加密保存；前端只显示“已绑定/需重新认证”。

### 4. 依赖与运行时存在已知漏洞

证据：

- `go version` 为 `go1.25.0 windows/amd64`。
- `govulncheck ./...` 报告代码路径受 19 个 Go 标准库漏洞影响，修复版本集中在 `go1.25.2` 至 `go1.25.10`，包括 `net/http`、`crypto/tls`、`crypto/x509`、`net/url`、`os`。
- `npm audit --omit=dev --json` 报告生产依赖 8 个漏洞：5 high、3 moderate。
- 直接依赖 `axios@1.13.6` 命中多个 advisory；传递依赖包括 `@xmldom/xmldom@0.8.11`、`fast-uri`、`follow-redirects`、`lodash@4.17.23`、`underscore@1.13.7`、`serialize-javascript`。

影响：

- Go 标准库漏洞覆盖 HTTP/TLS/URL/文件系统路径，属于运行时基础风险。
- 前端依赖漏洞会影响浏览器端请求处理、文档解析、图结构/导入链路或构建链路，具体利用取决于入口是否处理攻击者可控数据。

建议：

- 将 Go 工具链升级到至少 `go1.25.10`，重新 `go test ./...`、`go vet ./...`、`govulncheck ./...`。
- 升级 `axios` 到 npm audit 给出的安全版本范围。
- 跟进 `mammoth`、`@xmldom/xmldom`、`@antv/g6`/`dagre` 相关传递依赖升级；无法升级时隔离高风险解析入口并限制文件大小/类型。

## 中风险发现

### 5. `/uploads/` 公开目录索引泄露上传对象清单

证据：

- 未认证 `GET /uploads/` 返回 `200`，HTML 中列出 `images/`。
- 未认证 `GET /uploads/images/` 返回 `200`，HTML 中列出上传文件名。
- 代码位置：`backend-go/internal/platform/httpserver/server.go:73` 使用 `http.FileServer(http.Dir(uploadsDir))`，默认会生成目录索引。

影响：

- 攻击者可枚举已上传文件名和目录结构。
- 与匿名图片上传叠加后，可把平台变成公开文件投放与枚举站点。

建议：

- 替换为自定义静态文件 handler：目录请求返回 `404/403`，只允许访问具体文件。
- 如果上传内容包含隐私或作业资料，改为鉴权下载或短期签名 URL。
- Nginx/对象存储侧同步关闭目录列表。

### 6. 详细健康检查和 Prometheus 指标公开暴露

证据：

- 未认证 `GET /health/detailed` 返回 `200`，包含 `postgres`、`redis` 组件状态。
- 未认证 `GET /metrics` 返回 `200`，包含版本和环境标签，例如 `version="0.1.0", environment="development"`。
- 代码位置：`backend-go/internal/platform/httpserver/server.go:56-70` 直接注册公开路由。

影响：

- 公开环境会泄露服务组件、环境、版本和请求量信息，降低攻击成本。

建议：

- 保留简单 `/health` 作为负载均衡探针。
- `/health/detailed`、`/metrics` 仅内网、VPN、管理鉴权或 Prometheus 专用网络可访问。
- 生产环境避免暴露 `environment=development`。

## 低风险/加固项

### 7. 安全响应头不完整

证据：

- `GET /health` 响应包含 `X-Content-Type-Options=nosniff`、`X-Frame-Options=SAMEORIGIN`、`Referrer-Policy=strict-origin-when-cross-origin`。
- 缺少 `Content-Security-Policy`、`Strict-Transport-Security`、`Permissions-Policy`。
- 代码位置：`backend-go/internal/platform/middleware/middleware.go:39-45`。
- `frontend/nginx.conf` 也未设置 CSP；`nginx-site.conf` 的 HTTPS/HSTS 块处于注释示例状态。

建议：

- 生产 Nginx 增加 CSP，至少限制 `default-src 'self'`，再按 KaTeX、图表、API、图片源逐项放开。
- HTTPS 站点启用 HSTS；HTTP 仅做 301 到 HTTPS。
- 增加 `Permissions-Policy`，禁用不需要的传感器、摄像头、麦克风等能力。

### 8. 认证 access token 存在 sessionStorage，需与 CSP/XSS 防护一起考虑

证据：

- `frontend/src/libs/auth/tokenStorage.ts:5-9` 从 `sessionStorage` 读取/写入 `auth_token`。

影响：

- `sessionStorage` 优于长期 `localStorage`，但一旦出现 XSS，access token 仍可被读取。

建议：

- 优先补 CSP 和依赖漏洞。
- 可评估将 access token 缩短生命周期，并使用内存态保存；刷新 token 已使用 HttpOnly cookie + CSRF，是更好的方向。

## 已通过的安全控制

- 未认证访问 `/api/v1/auth/me`、`/api/v1/admin/users`、`/api/v1/progress/overview` 均返回 `401`。
- 学生访问管理员和教师接口分别返回 `403`。
- 教师访问管理员接口返回 `403`。
- `alg=none` 伪造管理员 JWT 被拒绝，返回 `401`。
- 简单 SQL 注入登录负载未绕过认证，返回 `401`。
- refresh token 缺少 CSRF header 时返回 `403`；带匹配 CSRF token 时返回 `200`。
- 登录失败限流生效：第 6 次同用户名错误登录返回临时锁定消息。
- 未认证资源文件上传被拒绝 `401`；学生资源文件上传被拒绝 `403`。
- CORS 当前配置拒绝未列入来源，允许 `http://localhost:5173` 并带 credentials。
- 前端生产构建通过。
- `go vet ./...` 通过。
- 清理本轮临时误生成的 `backend/uploads` 空目录后，`go test ./tests/contract` 通过。

## 验证命令摘要

- `go run ./cmd/migrate`
- `go build -o ..\output\msp-api.exe ./cmd/api`
- `npm run dev -- --host 127.0.0.1 --port 5173`
- 本地动态探测脚本：覆盖认证、角色、JWT、CSRF、CORS、上传、公开目录、metrics、health。
- `go test ./...`：除本轮临时生成 `backend/uploads` 导致契约测试失败外，其余包通过；清理后 `go test ./tests/contract` 通过。
- `go vet ./...`：通过。
- `npm run build`：通过。
- `npm audit --omit=dev --json`：发现 8 个生产依赖漏洞。
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`：发现当前 Go 标准库版本受 19 个漏洞影响。

## 优先修复顺序

1. 关闭教师自助注册或改为审批/邀请码。
2. 给图片上传加登录鉴权、限流、真实类型校验，并关闭 `/uploads/` 目录索引。
3. 移除前端 localStorage 中的教务密码持久化。
4. 升级 Go 到 `1.25.10+`，升级前端高风险依赖。
5. 将 `/metrics` 和 `/health/detailed` 收到内网或管理鉴权后面。
6. 补 CSP/HSTS/Permissions-Policy，并确认生产 `ENVIRONMENT`、密钥、CORS 配置。
