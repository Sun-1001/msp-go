#!/usr/bin/env bash
# First production deployment for MathStudyPlatform.

set -Eeuo pipefail
umask 077

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

VERSION="latest"
DOMAIN=""
COMPOSE_FILE="docker-compose.yml"
ENV_FILE=".env"
BACKEND_IMAGE="ghcr.io/fraternity-z/mathstudyplatform-backend"
FRONTEND_IMAGE="ghcr.io/fraternity-z/mathstudyplatform-frontend"
NGINX_CONF_DIR="/etc/nginx/sites-available"
NGINX_ENABLED_DIR="/etc/nginx/sites-enabled"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." > /dev/null 2>&1 && pwd)"

usage() {
    cat <<'EOF'
用法: sudo bash ./scripts/deploy.sh [选项]

选项:
  --version <标签>  部署 latest、v1.0.0 或 sha-abcdef1，默认 latest
  --domain <域名>   配置宿主机 Nginx 反向代理
  -h, --help        显示帮助

私有 GHCR 镜像可通过 GHCR_USERNAME 和 GHCR_TOKEN 环境变量登录。
此脚本只用于首次部署；已有应用请使用 scripts/update.sh。
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            [ "$#" -ge 2 ] || { echo "--version 缺少参数" >&2; exit 2; }
            VERSION="$2"
            shift 2
            ;;
        --domain)
            [ "$#" -ge 2 ] || { echo "--domain 缺少参数" >&2; exit 2; }
            DOMAIN="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "未知参数: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if ! [[ "$VERSION" =~ ^(latest|v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)|sha-[0-9a-f]{7,40})$ ]]; then
    echo -e "${RED}版本格式无效，仅接受 latest、v1.0.0 或 sha-abcdef1${NC}" >&2
    exit 1
fi
if [ -n "$DOMAIN" ] && ! [[ "$DOMAIN" =~ ^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}$ ]]; then
    echo -e "${RED}域名格式无效，只接受标准 DNS 域名${NC}" >&2
    exit 1
fi
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请使用 root 权限运行此脚本${NC}" >&2
    exit 1
fi

cd "$PROJECT_ROOT"

compose() {
    "${DOCKER_COMPOSE[@]}" -f "$COMPOSE_FILE" "$@"
}

wait_for_postgres() {
    local max_attempts="${1:-30}"
    local attempt
    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        if compose exec -T postgres sh -ec 'pg_isready -q -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"' > /dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done
    echo "PostgreSQL 在 $((max_attempts * 2)) 秒内未就绪" >&2
    compose logs --tail=50 postgres >&2 || true
    return 1
}

wait_for_service() {
    local service="$1" max_attempts="$2"
    local attempt container_id state
    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        container_id="$(compose ps -q "$service" 2>/dev/null || true)"
        if [ -n "$container_id" ]; then
            state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id" 2>/dev/null || true)"
            case "$state" in
                healthy|running) return 0 ;;
                unhealthy|dead|exited)
                    compose logs --tail=50 "$service" >&2 || true
                    return 1
                    ;;
            esac
        fi
        sleep 2
    done
    compose logs --tail=50 "$service" >&2 || true
    return 1
}

persist_image_version() {
    if grep -q '^IMAGE_VERSION=' "$ENV_FILE"; then
        sed -i "s/^IMAGE_VERSION=.*/IMAGE_VERSION=${VERSION}/" "$ENV_FILE"
    else
        printf '\nIMAGE_VERSION=%s\n' "$VERSION" >> "$ENV_FILE"
    fi
}

configure_nginx() {
    [ -n "$DOMAIN" ] || return 0
    if ! command -v nginx > /dev/null 2>&1; then
        apt-get update
        apt-get install -y nginx
    fi
    mkdir -p "$NGINX_CONF_DIR" "$NGINX_ENABLED_DIR"
    cat > "${NGINX_CONF_DIR}/mathplatform.conf" <<EOF
server {
    listen 80;
    server_name ${DOMAIN};
    client_max_body_size 50M;

    location / {
        proxy_pass http://localhost:9000;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /api/ {
        proxy_pass http://localhost:8000;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 300s;
    }

    location /health {
        proxy_pass http://localhost:8000/health;
        access_log off;
    }
}
EOF
    ln -sf "${NGINX_CONF_DIR}/mathplatform.conf" "${NGINX_ENABLED_DIR}/mathplatform.conf"
    nginx -t
    systemctl reload nginx
}

echo -e "${GREEN}=== MathStudyPlatform 首次部署 ===${NC}"

command -v docker > /dev/null 2>&1 || { echo -e "${RED}Docker 未安装${NC}" >&2; exit 1; }
if docker compose version > /dev/null 2>&1; then
    DOCKER_COMPOSE=(docker compose)
elif command -v docker-compose > /dev/null 2>&1; then
    DOCKER_COMPOSE=(docker-compose)
else
    echo -e "${RED}Docker Compose 未安装${NC}" >&2
    exit 1
fi
[ -f "$COMPOSE_FILE" ] || { echo -e "${RED}找不到 ${COMPOSE_FILE}${NC}" >&2; exit 1; }
if [ ! -f "$ENV_FILE" ]; then
    [ -f ".env.example" ] || { echo -e "${RED}找不到 .env.example${NC}" >&2; exit 1; }
    cp .env.example "$ENV_FILE"
    echo -e "${YELLOW}已创建 ${ENV_FILE}，请完成生产配置后重新运行脚本${NC}"
    exit 1
fi
if [ -n "$(compose ps -a -q backend 2>/dev/null || true)" ] || [ -n "$(compose ps -a -q frontend 2>/dev/null || true)" ]; then
    echo -e "${RED}检测到已有应用容器，请改用 scripts/update.sh${NC}" >&2
    exit 1
fi
if [ -n "${GHCR_TOKEN:-}" ]; then
    [ -n "${GHCR_USERNAME:-}" ] || { echo -e "${RED}设置 GHCR_TOKEN 时必须同时设置 GHCR_USERNAME${NC}" >&2; exit 1; }
    printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$GHCR_USERNAME" --password-stdin
fi

export BACKEND_IMAGE FRONTEND_IMAGE IMAGE_VERSION="$VERSION"

echo -e "${BLUE}[1/5] 拉取应用镜像...${NC}"
docker pull "${BACKEND_IMAGE}:${VERSION}"
docker pull "${FRONTEND_IMAGE}:${VERSION}"

echo -e "${BLUE}[2/5] 启动基础服务...${NC}"
compose up -d postgres redis
wait_for_postgres "${POSTGRES_WAIT_ATTEMPTS:-30}"

echo -e "${BLUE}[3/5] 初始化数据库并执行迁移...${NC}"
compose exec -T postgres sh -ec 'psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}" -c "CREATE EXTENSION IF NOT EXISTS vector"'
compose run --rm --no-deps backend msp-migrate

echo -e "${BLUE}[4/5] 启动并检查应用服务...${NC}"
compose up -d backend frontend
if ! wait_for_service backend "${BACKEND_WAIT_ATTEMPTS:-45}" || ! wait_for_service frontend "${FRONTEND_WAIT_ATTEMPTS:-30}"; then
    compose stop backend frontend || true
    echo -e "${RED}应用健康检查失败，应用已停止${NC}" >&2
    exit 1
fi

echo -e "${BLUE}[5/5] 配置反向代理...${NC}"
configure_nginx
persist_image_version

echo -e "${GREEN}=== 首次部署完成 ===${NC}"
compose ps
[ -n "$DOMAIN" ] && echo -e "${GREEN}访问地址: http://${DOMAIN}${NC}"
echo "后续更新: sudo bash ./scripts/update.sh --version <标签>"
