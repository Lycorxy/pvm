#!/bin/bash
# ============================================================================
# PVM 环境管理器 - 从远程替换本地 local 文件脚本 (macOS/Linux)
# ============================================================================
# 功能：从远程仓库强制替换本地 local 文件
# 用法：./replace-local-from-remote.sh [--backup|--no-backup|--list]
#   --backup    : 替换前备份原有文件（默认）
#   --no-backup : 不备份直接替换
#   --list      : 仅列出 local 文件，不替换
# ============================================================================

set -e

echo "========================================"
echo "  从远程替换本地 local 文件"
echo "========================================"
echo

# 参数解析
MODE="${1:-backup}"
if [[ "$MODE" != "backup" && "$MODE" != "no-backup" && "$MODE" != "list" ]]; then
    echo "[错误] 无效的参数: $MODE"
    echo "用法: $0 [--backup|--no-backup|--list]"
    exit 1
fi

# 检查是否在 git 仓库中
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "[错误] 当前目录不是 Git 仓库"
    exit 1
fi

# 确保远程信息是最新的
echo "[同步] 正在从远程获取最新信息..."
git fetch origin 2>/dev/null || echo "[警告] 无法从远程获取信息，将使用本地缓存的远程信息"

echo
echo "[扫描] 正在查找 local 文件..."
echo

# 备份目录
BACKUP_DIR=".local-backup-$(date +%Y%m%d_%H%M%S)"
LOCAL_COUNT=0

# 查找所有包含 local 的文件（不区分大小写）
while IFS= read -r file; do
    LOCAL_COUNT=$((LOCAL_COUNT + 1))
    echo "[$LOCAL_COUNT] $file"
    
    if [ "$MODE" = "list" ]; then
        # 仅列出，不操作
        continue
    fi
    
    # 检查文件是否存在
    if [ ! -f "$file" ]; then
        echo "[跳过] $file - 文件不存在"
        continue
    fi
    
    # 备份原有文件
    if [ "$MODE" = "backup" ]; then
        mkdir -p "$BACKUP_DIR"
        
        # 保持目录结构
        backup_path="$BACKUP_DIR/$file"
        backup_dir=$(dirname "$backup_path")
        mkdir -p "$backup_dir"
        
        if cp "$file" "$backup_path" 2>/dev/null; then
            echo "[备份] $file -> $backup_path"
        else
            echo "[警告] 备份失败: $file"
        fi
    fi
    
    # 从远程获取文件
    if git checkout origin/HEAD -- "$file" 2>/dev/null; then
        echo "[替换] $file - 完成"
    else
        # 尝试从当前分支的远程获取
        branch=$(git rev-parse --abbrev-ref HEAD)
        if git checkout "origin/$branch" -- "$file" 2>/dev/null; then
            echo "[替换] $file - 完成"
        else
            echo "[失败] $file - 无法从远程获取"
        fi
    fi
done < <(git ls-files | grep -i "local")

echo
echo "========================================"
echo "  处理结果"
echo "========================================"
echo "找到 local 文件数: $LOCAL_COUNT"

if [ "$MODE" = "list" ]; then
    echo "[完成] 仅列出文件，未执行替换"
elif [ $LOCAL_COUNT -gt 0 ]; then
    echo "[完成] 已从远程替换 $LOCAL_COUNT 个文件"
    if [ "$MODE" = "backup" ]; then
        echo "[备份] 备份文件位于: $BACKUP_DIR"
    fi
else
    echo "[完成] 未找到 local 文件"
fi

exit 0