#!/bin/sh
# NRL Link 后端部署脚本（单容器，不含反向代理、不含前端）
#
# 注意：生产前端请用 nrllink-web 仓库的编排（会把完整前端挂到 /nrllink/www）。
# 本仓库的 www/ 只是开发用测试前端，不随镜像分发。
#
# 两种用法：
#   1) 仓库内执行：  ./install.sh            拉镜像启动
#                    ./install.sh --build    本地构建镜像
#   2) 服务器一行命令： curl -fsSL https://raw.githubusercontent.com/78ham/nrllink-78ham/sqlite/install.sh | sh
#      （下载编排文件到 ./nrllink 并启动；可用 NRL_DIR 指定目录、NRL_BRANCH 指定分支）
set -e

REPO="78ham/nrllink-78ham"
BRANCH="${NRL_BRANCH:-sqlite}"
RAW="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() { printf "${GREEN}[OK]${NC}   %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
fail() { printf "${RED}[FAIL]${NC} %s\n" "$1"; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# ---- 环境检查 ----
have docker || fail "未找到 docker，请先安装 Docker"
docker info >/dev/null 2>&1 || fail "Docker 未运行"
info "Docker 就绪"

docker compose version >/dev/null 2>&1 || fail "需要 Docker Compose V2（docker compose 子命令）"
DC="docker compose"
info "Compose 就绪"

# ---- 定位工作目录 ----
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || pwd)
if [ -f "${SCRIPT_DIR}/docker-compose.yml" ]; then
  cd "${SCRIPT_DIR}"
  info "在仓库目录部署：$(pwd)"
else
  TARGET="${NRL_DIR:-./nrllink}"
  mkdir -p "${TARGET}"
  cd "${TARGET}"
  info "引导模式，部署目录：$(pwd)"
  have curl || fail "引导模式需要 curl 下载编排文件"
  curl -fsSL "${RAW}/docker-compose.yml" -o docker-compose.yml || fail "下载 docker-compose.yml 失败"
  curl -fsSL "${RAW}/.env.example" -o .env.example || fail "下载 .env.example 失败"
  info "编排文件已下载"
fi

# ---- .env ----
if [ ! -f .env ]; then
  [ -f .env.example ] || fail "缺少 .env.example"
  cp .env.example .env
  info "已生成 .env（端口 / 镜像可在此修改）"
else
  info ".env 已存在"
fi

# ---- GHCR 登录（可选，仅私有镜像需要） ----
if [ -f .ghcr-token ]; then
  if docker login ghcr.io -u 78ham --password-stdin < .ghcr-token >/dev/null 2>&1; then
    info "GHCR 已登录"
  else
    warn "GHCR 登录失败，改为匿名拉取"
  fi
else
  info "匿名拉取镜像（公开镜像无需登录）"
fi

# ---- 获取镜像 ----
if [ "${1:-}" = "--build" ]; then
  info "本地构建镜像（含 codec2，首次较慢）"
  $DC build || fail "镜像构建失败"
else
  info "拉取镜像..."
  $DC pull || fail "镜像拉取失败，可改用：./install.sh --build"
fi

# ---- 启动 ----
$DC up -d --remove-orphans || fail "启动失败"

info "等待后端就绪..."
i=30
while [ $i -gt 0 ]; do
  $DC ps nrllink 2>/dev/null | grep -q "healthy" && break
  i=$((i-1)); sleep 2
done

echo ""
$DC ps
echo ""

port="$(grep -E '^API_PORT=' .env 2>/dev/null | tail -n1 | cut -d= -f2)"
[ -n "$port" ] || port=9000
bind="$(grep -E '^API_BIND=' .env 2>/dev/null | tail -n1 | cut -d= -f2)"
[ -n "$bind" ] || bind=127.0.0.1

info "完成！后端接口 http://${bind}:${port}"
info "首次部署的管理员密码： $DC logs nrllink | grep -A3 默认管理员"
warn "请确认防火墙已放通 UDP 60050（设备接入）"
if [ "$bind" = "127.0.0.1" ]; then
  info "接口当前只监听本机；要让局域网访问，把 .env 里 API_BIND 改成 0.0.0.0 或内网网卡地址"
fi
warn "此仓库只跑后端；完整部署（前端容器 + 后端容器）请用 nrllink-web 仓库"