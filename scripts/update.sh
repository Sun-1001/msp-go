#!/bin/bash
# 生产环境更新脚本
# 用法: ./update.sh [版本号]

set -Eeuo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
VERSION=${1:-latest}
COMPOSE_FILE="docker-compose.yml"
ENV_FILE=".env"
BACKUP_ROOT="${BACKUP_ROOT:-backups}"
BACKUP_DIR="${BACKUP_ROOT}/$(date +%Y%m%d_%H%M%S)"
UPLOADS_DIR="${MSP_UPLOADS_BACKUP_DIR:-uploads}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." > /dev/null 2>&1 && pwd)"

cd "$PROJECT_ROOT"
# shellcheck source=deployment-common.sh
source "${SCRIPT_DIR}/deployment-common.sh"
umask 077

echo -e "${GREEN}=== MathStudyPlatform 更新部署 ===${NC}"
echo -e "${YELLOW}版本: ${VERSION}${NC}"

# 检查环境变量
if [ -z "${DOCKER_USERNAME:-}" ]; then
    echo -e "${RED}错误: DOCKER_USERNAME 环境变量未设置${NC}"
    echo "请运行: export DOCKER_USERNAME=your-dockerhub-username"
    exit 1
fi

# 检查 docker-compose 文件
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}错误: 找不到 ${COMPOSE_FILE}${NC}"
    exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}错误: 找不到 ${ENV_FILE}${NC}"
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

# 记录当前配置和运行镜像。数据库与上传目录在停止应用写入后备份。
echo -e "${BLUE}[1/9] 准备备份目录并记录当前版本...${NC}"
BACKEND_WAS_RUNNING=false
FRONTEND_WAS_RUNNING=false
if service_is_running backend; then
    BACKEND_WAS_RUNNING=true
fi
if service_is_running frontend; then
    FRONTEND_WAS_RUNNING=true
fi
mkdir -p "$BACKUP_ROOT"
if ! mkdir "$BACKUP_DIR"; then
    echo -e "${RED}错误: 无法创建唯一备份目录 ${BACKUP_DIR}${NC}"
    exit 1
fi
cp -- "$ENV_FILE" "$BACKUP_DIR/.env"
compose config > "$BACKUP_DIR/docker-compose.resolved.yml"
{
    printf 'backend=%s\n' "$(service_image backend)"
    printf 'backend_image_id=%s\n' "$(service_image_id backend)"
    printf 'frontend=%s\n' "$(service_image frontend)"
    printf 'frontend_image_id=%s\n' "$(service_image_id frontend)"
    printf 'backend_was_running=%s\n' "$BACKEND_WAS_RUNNING"
    printf 'frontend_was_running=%s\n' "$FRONTEND_WAS_RUNNING"
} > "$BACKUP_DIR/previous-images.txt"
echo -e "${GREEN}✓ 配置与当前镜像已记录到 ${BACKUP_DIR}${NC}"

# 确保数据库可访问；旧应用保持运行，直到新镜像拉取完成。
echo -e "${BLUE}[2/9] 检查 PostgreSQL...${NC}"
compose up -d postgres
if ! wait_for_postgres "${POSTGRES_WAIT_ATTEMPTS:-30}"; then
    echo -e "${RED}✗ PostgreSQL 未就绪，更新已中止，应用未停止${NC}"
    exit 1
fi
echo -e "${GREEN}✓ PostgreSQL 已就绪${NC}"

# 拉取最新镜像
echo -e "${BLUE}[3/9] 拉取最新镜像...${NC}"
docker pull "${DOCKER_USERNAME}/backend-go:${VERSION}"
docker pull "${DOCKER_USERNAME}/frontend:${VERSION}"
echo -e "${GREEN}✓ 镜像拉取完成${NC}"

# 导出环境变量
echo -e "${BLUE}[4/9] 设置目标版本...${NC}"
export DOCKER_USERNAME
export IMAGE_VERSION="$VERSION"

# 只停止应用，保持 PostgreSQL/Redis 容器和数据卷在线。
echo -e "${BLUE}[5/9] 停止应用写入...${NC}"
compose stop backend frontend
echo -e "${GREEN}✓ 后端与前端已停止${NC}"

restart_previous_apps_after_backup_failure() {
    local services=()

    echo -e "${YELLOW}备份失败，尝试按原容器配置恢复应用...${NC}"
    if [ "$BACKEND_WAS_RUNNING" = true ]; then
        services+=(backend)
    fi
    if [ "$FRONTEND_WAS_RUNNING" = true ]; then
        services+=(frontend)
    fi
    if [ "${#services[@]}" -gt 0 ]; then
        compose start "${services[@]}" || true
    fi
}

# 数据库与 uploads 在应用停止后形成同一维护窗口内的可恢复快照。
echo -e "${BLUE}[6/9] 备份 PostgreSQL 与上传目录...${NC}"
if ! backup_postgres "$BACKUP_DIR/postgres.dump"; then
    echo -e "${RED}✗ PostgreSQL 备份失败，未执行迁移${NC}"
    restart_previous_apps_after_backup_failure
    exit 1
fi
if [ -d "$UPLOADS_DIR" ]; then
    if ! tar -czf "$BACKUP_DIR/uploads.tar.gz" -C "$(dirname -- "$UPLOADS_DIR")" "$(basename -- "$UPLOADS_DIR")"; then
        echo -e "${RED}✗ 上传目录备份失败，未执行迁移${NC}"
        restart_previous_apps_after_backup_failure
        exit 1
    fi
else
    printf 'Upload directory did not exist at backup time: %s\n' "$UPLOADS_DIR" > "$BACKUP_DIR/uploads.absent.txt"
fi
echo -e "${GREEN}✓ 数据备份完成: ${BACKUP_DIR}${NC}"

# 启动基础依赖
echo -e "${BLUE}[7/9] 确认基础容器...${NC}"
compose up -d postgres redis
echo -e "${GREEN}✓ 基础容器已启动${NC}"

if ! wait_for_postgres "${POSTGRES_WAIT_ATTEMPTS:-30}"; then
    echo -e "${RED}✗ PostgreSQL 未就绪，未执行迁移；备份位于 ${BACKUP_DIR}${NC}"
    restart_previous_apps_after_backup_failure
    exit 1
fi

# 运行数据库迁移
echo -e "${BLUE}[8/9] 执行 Go 数据库迁移...${NC}"
echo -e "${YELLOW}更新脚本不运行 Python Alembic，改由 Go migration runner 应用数据库迁移。${NC}"
if ! compose run --rm --no-deps backend msp-migrate; then
    echo -e "${RED}✗ 数据库迁移失败，应用保持停止；请检查日志和 ${BACKUP_DIR}/postgres.dump${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go 数据库迁移完成${NC}"

# 启动应用容器
echo -e "${BLUE}[9/9] 启动并检查应用...${NC}"
compose up -d backend frontend
echo -e "${GREEN}✓ 应用容器已启动${NC}"

# 健康检查
if wait_for_service backend "${BACKEND_WAIT_ATTEMPTS:-45}" && wait_for_service frontend "${FRONTEND_WAIT_ATTEMPTS:-30}"; then
    echo -e "${GREEN}✓ 服务启动成功${NC}"
    compose ps
else
    echo -e "${RED}✗ 服务启动失败；数据库备份位于 ${BACKUP_DIR}/postgres.dump${NC}"
    compose logs --tail=50 backend frontend
    compose stop backend frontend || true
    echo -e "${YELLOW}后端与前端已停止，避免不健康的新版本继续提供服务。${NC}"
    echo -e "${YELLOW}不要只回滚镜像：先确认旧版本兼容当前 schema，必要时按部署文档恢复数据库和 uploads。${NC}"
    exit 1
fi

echo -e "${GREEN}=== 更新部署完成 ===${NC}"
echo -e "${GREEN}备份目录: ${BACKUP_DIR}${NC}"
echo -e "${BLUE}常用命令:${NC}"
echo "  查看日志: ${DOCKER_COMPOSE[*]} -f ${COMPOSE_FILE} logs -f"
echo "  查看旧镜像: cat ${BACKUP_DIR}/previous-images.txt"
echo "  恢复数据: 参见 docs/technical/deployment.md 的“更新与回滚”"
