#!/bin/bash
# ============================================================================
# PVM 环境管理器 - local 文件合并冲突处理脚本 (macOS/Linux)
# ============================================================================
# 功能：自动处理 git 合并时产生的 local 文件冲突
# 用法：./resolve-local-conflict.sh [--local|--remote|--auto]
#   --local  : 保留本地版本
#   --remote : 使用远程版本
#   --auto   : 自动选择（默认：保留远程）
# ============================================================================

set -e

echo "========================================"
echo "  Local 文件合并冲突处理工具"
echo "========================================"
echo

# 参数解析
MODE="${1:-auto}"
if [[ "$MODE" != "local" && "$MODE" != "remote" && "$MODE" != "auto" ]]; then
    echo "[错误] 无效的参数: $MODE"
    echo "用法: $0 [--local|--remote|--auto]"
    exit 1
fi

# 检查是否在 git 仓库中
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "[错误] 当前目录不是 Git 仓库"
    exit 1
fi

# 检查是否有冲突
if ! git diff --name-only --diff-filter=U >/dev/null 2>&1; then
    echo "[提示] 没有发现合并冲突"
    exit 0
fi

echo "[扫描] 正在检测合并冲突..."
echo

# 统计计数
CONFLICT_COUNT=0
LOCAL_CONFLICT_COUNT=0

# 获取所有冲突文件
while IFS= read -r file; do
    CONFLICT_COUNT=$((CONFLICT_COUNT + 1))
    
    # 检查是否是 local 文件（文件名包含 local，不区分大小写）
    if echo "$file" | grep -iq "local"; then
        LOCAL_CONFLICT_COUNT=$((LOCAL_CONFLICT_COUNT + 1))
        echo "[发现] $file"
        
        # 解决冲突
        if [ "$MODE" = "local" ]; then
            echo "[处理] $file -> 保留本地版本"
            git checkout --ours "$file"
            git add "$file"
        elif [ "$MODE" = "remote" ]; then
            echo "[处理] $file -> 使用远程版本"
            git checkout --theirs "$file"
            git add "$file"
        else
            # auto 模式：默认使用远程版本
            echo "[处理] $file -> 自动使用远程版本"
            git checkout --theirs "$file"
            git add "$file"
        fi
    fi
done < <(git diff --name-only --diff-filter=U)

echo
echo "========================================"
echo "  处理结果"
echo "========================================"
echo "总冲突文件数: $CONFLICT_COUNT"
echo "Local 文件冲突数: $LOCAL_CONFLICT_COUNT"
echo

if [ $LOCAL_CONFLICT_COUNT -eq 0 ]; then
    echo "[完成] 未发现 local 文件冲突"
else
    echo "[完成] 已处理 $LOCAL_CONFLICT_COUNT 个 local 文件冲突"
fi

exit 0