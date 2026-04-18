#!/bin/bash
# 生产环境更新脚本
# 用法: ./update.sh [版本号]

set -e

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
BACKUP_DIR="backups/$(date +%Y%m%d_%H%M%S)"

echo -e "${GREEN}=== MathStudyPlatform 更新部署 ===${NC}"
echo -e "${YELLOW}版本: ${VERSION}${NC}"

# 检查环境变量
if [ -z "$DOCKER_USERNAME" ]; then
    echo -e "${RED}错误: DOCKER_USERNAME 环境变量未设置${NC}"
    echo "请运行: export DOCKER_USERNAME=your-dockerhub-username"
    exit 1
fi

# 检查 docker-compose 文件
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}错误: 找不到 ${COMPOSE_FILE}${NC}"
    exit 1
fi

# 优先使用 docker compose (v2)，回退到 docker-compose (v1)
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
else
    echo -e "${RED}Docker Compose 未安装，请先安装 Docker Compose${NC}"
    exit 1
fi

# 备份当前配置
echo -e "${BLUE}[1/7] 备份当前配置...${NC}"
mkdir -p "$BACKUP_DIR"
cp -r "$ENV_FILE" "$BACKUP_DIR/" 2>/dev/null || true
$DOCKER_COMPOSE -f "$COMPOSE_FILE" config > "$BACKUP_DIR/docker-compose.yml" 2>/dev/null || true
echo -e "${GREEN}✓ 配置已备份到 ${BACKUP_DIR}${NC}"

# 拉取最新镜像
echo -e "${BLUE}[2/7] 拉取最新镜像...${NC}"
docker pull ${DOCKER_USERNAME}/backend-go:${VERSION}
docker pull ${DOCKER_USERNAME}/frontend:${VERSION}
echo -e "${GREEN}✓ 镜像拉取完成${NC}"

# 导出环境变量
echo -e "${BLUE}[3/7] 设置环境变量...${NC}"
export DOCKER_USERNAME=${DOCKER_USERNAME}
export IMAGE_VERSION=${VERSION}

# 停止旧容器
echo -e "${BLUE}[4/7] 停止旧容器...${NC}"
$DOCKER_COMPOSE -f "$COMPOSE_FILE" down
echo -e "${GREEN}✓ 旧容器已停止${NC}"

# 启动基础依赖
echo -e "${BLUE}[5/7] 启动基础容器...${NC}"
$DOCKER_COMPOSE -f "$COMPOSE_FILE" up -d postgres redis
echo -e "${GREEN}✓ 基础容器已启动${NC}"

# 等待服务启动
echo -e "${BLUE}[6/7] 等待服务启动...${NC}"
sleep 10

# 运行数据库迁移
echo -e "${BLUE}[7/7] 数据库迁移...${NC}"
echo -e "${YELLOW}Go 后端迁移执行器尚未接入；更新脚本不再运行 Python Alembic。${NC}"

# 启动应用容器
echo -e "${BLUE}启动应用容器...${NC}"
$DOCKER_COMPOSE -f "$COMPOSE_FILE" up -d backend frontend
echo -e "${GREEN}✓ 应用容器已启动${NC}"

# 健康检查
echo -e "${BLUE}检查服务状态...${NC}"
if $DOCKER_COMPOSE -f "$COMPOSE_FILE" ps | grep -q "Up"; then
    echo -e "${GREEN}✓ 服务启动成功${NC}"
    $DOCKER_COMPOSE -f "$COMPOSE_FILE" ps
else
    echo -e "${RED}✗ 服务启动失败，查看日志:${NC}"
    $DOCKER_COMPOSE -f "$COMPOSE_FILE" logs --tail=50
    echo -e "${YELLOW}回滚命令: $DOCKER_COMPOSE -f ${COMPOSE_FILE} down && cp ${BACKUP_DIR}/.env . && $DOCKER_COMPOSE -f ${COMPOSE_FILE} up -d${NC}"
    exit 1
fi

# 清理旧镜像
echo -e "${BLUE}清理未使用的镜像...${NC}"
docker image prune -f

echo -e "${GREEN}=== 更新部署完成 ===${NC}"
echo -e "${BLUE}常用命令:${NC}"
echo "  查看日志: $DOCKER_COMPOSE -f ${COMPOSE_FILE} logs -f"
echo "  回滚版本: $DOCKER_COMPOSE -f ${COMPOSE_FILE} down && cp ${BACKUP_DIR}/.env . && $DOCKER_COMPOSE -f ${COMPOSE_FILE} up -d"
