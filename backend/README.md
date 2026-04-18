# 高等数学智能学习平台 - 后端

FastAPI 后端服务，提供 REST API 和基于 LangGraph 的多智能体系统。

## 技术栈

- **框架**: FastAPI + Python 3.11+
- **ORM**: SQLAlchemy (async) + Alembic
- **数据库**: PostgreSQL + pgvector + Redis
- **AI/Agent**: LangGraph + LangChain + SymPy
- **认证**: JWT (PyJWT)

## 快速开始

```bash
# 进入 backend 目录
cd backend

# 创建并激活虚拟环境（Python 3.11+）
uv venv --python 3.11 venv
# Windows PowerShell
.\venv\Scripts\Activate.ps1
# Linux/macOS
source venv/bin/activate

# 安装依赖（开发依赖）
uv pip install -e ".[dev,ai]"
# 或使用 pip
pip install -e ".[dev,ai]"

# 配置环境变量
# 复制 .env.example -> .env，并修改配置
# 注意：CORS_ORIGINS 需要 JSON 数组格式，例如：
# CORS_ORIGINS=["http://localhost:5173","http://127.0.0.1:5173"]

# 数据库迁移
alembic upgrade head

# 启动服务
uvicorn app.main:app --reload
```

服务默认运行在 http://localhost:8000

## API 文档

- **Swagger UI**: http://localhost:8000/api/v1/docs
- **ReDoc**: http://localhost:8000/api/v1/redoc
- **OpenAPI JSON**: http://localhost:8000/api/v1/openapi.json

## 目录结构

```
app/
├── main.py               # FastAPI 应用入口
├── config.py             # 配置管理
├── api/                  # API 接口层
│   ├── deps.py           # 依赖注入
│   └── v1/
│       ├── router.py     # 路由聚合
│       ├── auth.py       # 认证接口
│       ├── session.py    # 学习会话接口
│       ├── exercise.py   # 练习接口
│       ├── schemas/      # Pydantic 模型
│       └── admin/        # 管理员接口
├── domain/               # 领域层 (DDD)
│   ├── models/           # 领域模型
│   └── services/         # 领域服务
├── services/             # 应用服务层
├── infrastructure/       # 基础设施层
│   ├── database/         # 数据库配置和 ORM 模型
│   ├── repositories/     # 仓储模式
│   └── cache/            # Redis 缓存
├── agents/               # 多智能体系统
│   ├── core/             # 核心组件
│   ├── roles/            # 智能体角色 (9个)
│   └── workflow/         # LangGraph 工作流
└── core/                 # 核心工具
```

## 主要 API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 用户登录 |
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/session/start` | 开始学习会话 |
| POST | `/api/v1/session/message` | 发送消息 |
| GET | `/api/v1/exercise/next` | 获取下一题 |
| POST | `/api/v1/exercise/submit` | 提交答案 |
| GET | `/api/v1/mistakes/list` | 错题列表 |
| GET | `/api/v1/progress/overview` | 学习进度 |
| GET | `/api/v1/admin/stats/overview` | 统计概览 |
| GET | `/api/v1/admin/users` | 用户列表 |

## 智能体系统

基于 LangGraph 的多智能体协作：

| 智能体 | 职责 |
|--------|------|
| Orchestrator | 意图识别、任务路由 |
| Solver | 数学问题求解 (SymPy) |
| Tutor | 概念讲解、苏格拉底式教学 |
| Diagnostician | 错误诊断、步骤比对 |
| Planner | 学习路径规划 |
| Emotion Detector | 情感检测、安抚 |
| Verifier | 答案验证 |
| Reflection | 反思总结 |
| Safety | 安全检查 |

## 开发命令

```bash
# 代码检查
ruff check .
ruff check --fix .    # 自动修复

# 代码格式化
ruff format .

# 类型检查
mypy app

# 运行测试
pytest
pytest --cov=app      # 带覆盖率

# 数据库迁移
alembic revision --autogenerate -m "描述"
alembic upgrade head
alembic downgrade -1
```

## 环境变量

主要配置项（详见 `.env.example`）：

| 变量 | 说明 |
|------|------|
| `POSTGRES_*` | PostgreSQL 连接配置 |
| `REDIS_*` | Redis 连接配置 |
| `JWT_SECRET_KEY` | JWT 签名密钥 |
| `LLM_API_BASE` | LLM API 地址 |
| `LLM_API_KEY` | LLM API 密钥 |
| `LLM_MODEL_NAME` | 默认模型名称 |

## 相关文档

- [模块详细文档](./CLAUDE.md)
- [后端 Python 到 Go 重构迁移文档](../docs/backend-python-to-go-refactor.md)
- [根目录 README](../README.md)

## 内存基线与门禁

```bash
# 导入增量 + 1/5/10 分钟空闲 RSS 采样
python scripts/memory_probe.py \
  --intervals 60,300,600 \
  --command-template "{python} -m uvicorn app.main:app --host 127.0.0.1 --port {port} --lifespan off"

# 连续 3 次取中位数，超过阈值则退出码非 0
python scripts/memory_gate.py \
  --threshold-mb 250 \
  --repeats 3 \
  --intervals 60,300,600
```

> 生产环境建议设置：`LLM_POOL_WARMUP_ENABLED=false`
