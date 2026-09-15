#!/bin/sh
# 容器启动入口。
# 配置优先级：/nrllink/conf/udphub.yaml（挂载的自定义配置）> 镜像内置的 /nrllink/udphub/udphub.yaml
set -e

mkdir -p /nrllink/data /nrllink/www /nrllink/www/uploads

if [ -f /nrllink/conf/udphub.yaml ]; then
  echo "[start] 使用自定义配置 /nrllink/conf/udphub.yaml"
  exec /nrllink/udphub/udphub -c /nrllink/conf/udphub.yaml
fi

echo "[start] 使用镜像内置默认配置"
exec /nrllink/udphub/udphub