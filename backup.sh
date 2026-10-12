#!/bin/sh
# NRLLink 自动化数据库备份与轮转脚本
# 可配置环境变量：
#   BACKUP_DIR:  备份目录（默认：/nrllink/backups 或 ./backups）
#   NRL_DBFILE:  SQLite 数据库文件路径（默认：/nrllink/data/udphub.sqlite3）
#   KEEP_COUNT:  保留的备份快照份数（默认：7 份）
#   COMPRESS:    是否启用 gzip 压缩 (1=开启, 0=关闭，默认 1)

set -e

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_DIR="${BACKUP_DIR:-/nrllink/backups}"
DB_FILE="${NRL_DBFILE:-/nrllink/data/udphub.sqlite3}"
KEEP_COUNT="${KEEP_COUNT:-7}"
COMPRESS="${COMPRESS:-1}"

# 若处于本地源码环境，调整缺省目录
if [ ! -d "/nrllink" ]; then
    BACKUP_DIR="${BACKUP_DIR:-./backups}"
    DB_FILE="${NRL_DBFILE:-./udphub.sqlite3}"
fi

mkdir -p "$BACKUP_DIR"

SNAPSHOT_NAME="nrllink_backup_${TIMESTAMP}.sqlite3"
SNAPSHOT_PATH="${BACKUP_DIR}/${SNAPSHOT_NAME}"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] 开始执行 NRLLink 数据库热备份..."

# 1. 优先调用 udphub 原生 -backup 参数（自动处理 SQLCipher 加密与 DSN）
if command -v udphub >/dev/null 2>&1; then
    udphub -backup "$SNAPSHOT_PATH"
elif [ -x "/nrllink/udphub/udphub" ]; then
    /nrllink/udphub/udphub -backup "$SNAPSHOT_PATH"
elif [ -x "./udphub" ]; then
    ./udphub -backup "$SNAPSHOT_PATH"
elif command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$DB_FILE" "VACUUM INTO '$SNAPSHOT_PATH';"
else
    echo "错误: 未找到 udphub 或 sqlite3 命令，无法生成备份快照！" >&2
    exit 1
fi

if [ ! -f "$SNAPSHOT_PATH" ]; then
    echo "错误: 备份文件生成失败！" >&2
    exit 1
fi

# 2. 可选压缩归档
FINAL_FILE="$SNAPSHOT_PATH"
if [ "$COMPRESS" = "1" ] && command -v gzip >/dev/null 2>&1; then
    gzip -f "$SNAPSHOT_PATH"
    FINAL_FILE="${SNAPSHOT_PATH}.gz"
fi

FILE_SIZE=$(ls -lh "$FINAL_FILE" | awk '{print $5}')
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 数据库备份成功: ${FINAL_FILE} (${FILE_SIZE})"

# 3. 轮转清理旧备份，保留最近 $KEEP_COUNT 份
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 检查并清理旧备份（保留最近 ${KEEP_COUNT} 份）..."
TOTAL_BACKUPS=$(ls -1t "${BACKUP_DIR}"/nrllink_backup_*.sqlite3* 2>/dev/null | wc -l || true)
if [ "$TOTAL_BACKUPS" -gt "$KEEP_COUNT" ]; then
    REMOVE_COUNT=$((TOTAL_BACKUPS - KEEP_COUNT))
    ls -1t "${BACKUP_DIR}"/nrllink_backup_*.sqlite3* | tail -n "$REMOVE_COUNT" | while read -r old_file; do
        rm -f "$old_file"
        echo "  - 已删除历史备份: $(basename "$old_file")"
    done
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] 备份任务完成。"
