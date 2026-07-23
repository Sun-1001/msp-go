#!/bin/bash
# 生产环境首次部署脚本
# 用法: ./deploy.sh [域名]

set -Eeuo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
DOMAIN=${1:-""}
COMPOSE_FILE="docker-compose.yml"
ENV_FILE=".env"
NGINX_CONF_DIR="/etc/nginx/sites-available"
NGINX_ENABLED_DIR="/etc/nginx/sites-enabled"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." > /dev/null 2>&1 && pwd)"

cd "$PROJECT_ROOT"
# shellcheck source=deployment-common.sh
source "${SCRIPT_DIR}/deployment-common.sh"

echo -e "${GREEN}=== MathStudyPlatform 生产环境部署 ===${NC}"

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请使用 root 权限运行此脚本${NC}"
    echo "sudo ./deploy.sh [域名]"
    exit 1
fi

# 检查域名参数
if [ -z "$DOMAIN" ]; then
    echo -e "${YELLOW}未指定域名，将使用 IP 地址访问${NC}"
    read -p "是否继续？(y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
elif ! [[ "$DOMAIN" =~ ^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}$ ]]; then
    echo -e "${RED}域名格式无效，只接受标准 DNS 域名${NC}"
    exit 1
fi

# 检查 Docker 和 Docker Compose
echo -e "${BLUE}[1/8] 检查环境依赖...${NC}"
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker 未安装，请先安装 Docker${NC}"
    exit 1
fi

# 优先使用 docker compose (v2)，回退到 docker-compose (v1)
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE=(docker compose)
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE=(docker-compose)
else
    echo -e "${RED}Docker Compose 未安装，请先安装 Docker Compose${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker 和 Docker Compose 已安装${NC}"

# 检查 Nginx
if ! command -v nginx &> /dev/null; then
    echo -e "${YELLOW}Nginx 未安装，正在安装...${NC}"
    apt-get update && apt-get install -y nginx
fi
echo -e "${GREEN}✓ Nginx 已安装${NC}"

# 配置环境变量
echo -e "${BLUE}[2/8] 配置环境变量...${NC}"
if [ ! -f "$ENV_FILE" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example "$ENV_FILE"
        echo -e "${YELLOW}已创建 ${ENV_FILE}，请编辑配置文件${NC}"
        read -p "按回车键继续编辑配置文件..."
        ${EDITOR:-nano} "$ENV_FILE"
    else
        echo -e "${RED}找不到 .env.example${NC}"
        exit 1
    fi
fi

# 设置 Docker Hub 用户名
echo -e "${BLUE}[3/8] 配置 Docker Hub 用户名...${NC}"
read -r -p "请输入 Docker Hub 用户名: " DOCKER_USERNAME
if [ -z "$DOCKER_USERNAME" ]; then
    echo -e "${RED}Docker Hub 用户名不能为空${NC}"
    exit 1
fi
if ! [[ "$DOCKER_USERNAME" =~ ^[a-z0-9]+([._-][a-z0-9]+)*$ ]]; then
    echo -e "${RED}Docker Hub 用户名格式无效${NC}"
    exit 1
fi
export DOCKER_USERNAME
echo -e "${GREEN}✓ Docker Hub 用户名已设置${NC}"

# 登录 Docker Hub（如果是私有镜像）
read -p "镜像是否为私有？(y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}请登录 Docker Hub...${NC}"
    docker login
fi

# 拉取镜像
echo -e "${BLUE}[4/8] 拉取 Docker 镜像...${NC}"
docker pull "${DOCKER_USERNAME}/backend-go:latest"
docker pull "${DOCKER_USERNAME}/frontend:latest"
echo -e "${GREEN}✓ 镜像拉取完成${NC}"

# 启动基础依赖
echo -e "${BLUE}[5/8] 启动基础容器...${NC}"
compose up -d postgres redis
echo -e "${GREEN}✓ 基础容器启动完成${NC}"

# 等待数据库接受连接
echo -e "${BLUE}[6/8] 等待 PostgreSQL 就绪...${NC}"
if ! wait_for_postgres "${POSTGRES_WAIT_ATTEMPTS:-30}"; then
    echo -e "${RED}✗ PostgreSQL 未就绪，部署已中止${NC}"
    exit 1
fi

# 数据库迁移
echo -e "${BLUE}[7/8] 数据库迁移...${NC}"
echo -e "${YELLOW}默认部署不运行 Python Alembic，改由 Go migration runner 应用数据库迁移。${NC}"
compose run --rm --no-deps backend msp-migrate
echo -e "${GREEN}✓ Go 数据库迁移完成${NC}"

# 启动应用容器
echo -e "${BLUE}启动应用容器...${NC}"
compose up -d backend frontend
echo -e "${GREEN}✓ 应用容器启动完成${NC}"

if ! wait_for_service backend "${BACKEND_WAIT_ATTEMPTS:-45}" || ! wait_for_service frontend "${FRONTEND_WAIT_ATTEMPTS:-30}"; then
    echo -e "${RED}✗ 应用服务未正常启动${NC}"
    compose logs --tail=50 backend frontend || true
    compose stop backend frontend || true
    echo -e "${YELLOW}后端与前端已停止，请修复问题后重新部署。${NC}"
    exit 1
fi

# 配置 Nginx
echo -e "${BLUE}[8/8] 配置 Nginx 反向代理...${NC}"
if [ -n "$DOMAIN" ]; then
    cat > ${NGINX_CONF_DIR}/mathplatform.conf <<EOF
# MathStudyPlatform Nginx 配置
server {
    listen 80;
    server_name ${DOMAIN};

    # 客户端最大上传大小
    client_max_body_size 50M;

    # 前端静态文件
    location / {
        proxy_pass http://localhost:9000;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Go 后端 API
    location /api/ {
        proxy_pass http://localhost:8000;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";

        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # 健康检查
    location /health {
        proxy_pass http://localhost:8000/health;
        access_log off;
    }
}
EOF

    # 启用站点
    ln -sf ${NGINX_CONF_DIR}/mathplatform.conf ${NGINX_ENABLED_DIR}/

    # 测试配置
    nginx -t

    # 重载 Nginx
    systemctl reload nginx

    echo -e "${GREEN}✓ Nginx 配置完成${NC}"
    echo -e "${YELLOW}访问地址: http://${DOMAIN}${NC}"

    # 提示 SSL 配置
    echo -e "${BLUE}提示: 建议使用 Certbot 配置 HTTPS${NC}"
    echo "sudo apt-get install certbot python3-certbot-nginx"
    echo "sudo certbot --nginx -d ${DOMAIN}"
else
    echo -e "${YELLOW}未配置域名，跳过 Nginx 配置${NC}"
    echo -e "${YELLOW}前端访问: http://服务器IP:9000${NC}"
    echo -e "${YELLOW}后端访问: http://服务器IP:8000${NC}"
fi

# 显示服务状态
echo -e "${GREEN}=== 部署完成 ===${NC}"
compose ps

echo -e "${BLUE}常用命令:${NC}"
echo "  查看日志: ${DOCKER_COMPOSE[*]} -f $COMPOSE_FILE logs -f"
echo "  停止服务: ${DOCKER_COMPOSE[*]} -f $COMPOSE_FILE down"
echo "  重启服务: ${DOCKER_COMPOSE[*]} -f $COMPOSE_FILE restart"
echo "  更新服务: ./update.sh"
