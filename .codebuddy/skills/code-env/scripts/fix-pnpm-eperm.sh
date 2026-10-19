#!/usr/bin/env bash
# ============================================================
# PVM 环境管理器 - pnpm EPERM 权限错误修复脚本 (macOS/Linux)
# ============================================================
# 功能：修复 pnpm install 时的 ERR_PNPM_EPERM / EPERM unlink 错误
# 原因：esbuild / rollup / swc 等原生二进制被进程占用
# 用法：./fix-pnpm-eperm.sh [--reinstall|--kill-only|--check]
#   --reinstall  : 杀进程 + 删 node_modules + pnpm install（默认）
#   --kill-only  : 仅杀占用进程，不重装
#   --check      : 仅检测占用进程，不操作
# ============================================================

set -euo pipefail

echo "========================================"
echo "  pnpm EPERM 权限错误修复工具"
echo "========================================"
echo ""

# 参数解析
MODE="reinstall"
[ "${1:-}" = "--reinstall" ] && MODE="reinstall"
[ "${1:-}" = "--kill-only" ] && MODE="kill-only"
[ "${1:-}" = "--check" ]     && MODE="check"

# 检查是否在项目目录
if [ ! -f "package.json" ]; then
    echo "[错误] 当前目录没有 package.json，请在项目根目录运行"
    exit 1
fi

echo "[模式] $MODE"
echo ""

# ============================================================
# Step 1: 检测占用进程
# ============================================================
echo "[1/4] 扫描占用 node_modules 的进程..."

KILL_COUNT=0
PIDS=""

# 常见占用 node_modules 的进程
for proc in esbuild rollup swc vite webpack node; do
    if pgrep -x "$proc" >/dev/null 2>&1; then
        for pid in $(pgrep -x "$proc"); do
            echo "  [发现] $proc (PID: $pid)"
            PIDS="$PIDS $pid"
            KILL_COUNT=$((KILL_COUNT + 1))
        done
    fi
done

# 也检测占用 node_modules 目录的进程 (lsof)
if command -v lsof >/dev/null 2>&1; then
    NODE_MODULES_PIDS=$(lsof +D ./node_modules 2>/dev/null | awk 'NR>1 {print $2}' | sort -u)
    for pid in $NODE_MODULES_PIDS; do
        PROC_NAME=$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")
        # 避免重复
        echo "$PIDS" | grep -qw "$pid" && continue
        echo "  [发现] $PROC_NAME (PID: $pid) - 占用 node_modules"
        PIDS="$PIDS $pid"
        KILL_COUNT=$((KILL_COUNT + 1))
    done
fi

if [ "$KILL_COUNT" -eq 0 ]; then
    echo "  [OK] 未发现占用进程"
else
    echo "  共发现 $KILL_COUNT 个占用进程"
fi
echo ""

# check 模式
if [ "$MODE" = "check" ]; then
    echo "[完成] 仅检测模式，未执行修复"
    if [ "$KILL_COUNT" -gt 0 ]; then
        echo "[建议] 运行 ./fix-pnpm-eperm.sh --kill-only 杀死占用进程"
        echo "       或运行 ./fix-pnpm-eperm.sh --reinstall 完整修复"
    fi
    exit 0
fi

# ============================================================
# Step 2: 杀死占用进程
# ============================================================
if [ "$KILL_COUNT" -gt 0 ]; then
    echo "[2/4] 终止占用进程..."
    for pid in $PIDS; do
        PROC_NAME=$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")
        kill -9 "$pid" 2>/dev/null && echo "  [已杀] $PROC_NAME (PID: $pid)" || true
    done
    sleep 2
else
    echo "[2/4] 无需杀进程"
fi
echo ""

# kill-only 模式
if [ "$MODE" = "kill-only" ]; then
    echo "[完成] 已终止占用进程，请重新运行 pnpm install"
    exit 0
fi

# ============================================================
# Step 3: 清理 node_modules
# ============================================================
echo "[3/4] 清理 node_modules..."

if [ -d "node_modules" ]; then
    rm -rf node_modules 2>/dev/null || true
    if [ -d "node_modules" ]; then
        echo "  [重试] 强制删除..."
        chmod -R u+w node_modules 2>/dev/null || true
        rm -rf node_modules 2>/dev/null || true
    fi
    if [ -d "node_modules" ]; then
        echo "  [警告] node_modules 仍无法删除"
        echo "  [提示] 请关闭编辑器后重试，或手动删除: sudo rm -rf node_modules"
        exit 1
    fi
    echo "  [OK] node_modules 已删除"
else
    echo "  [跳过] node_modules 不存在"
fi

# 清理 pnpm store 缓存
if [ -d ".pnpm-store" ]; then
    rm -rf .pnpm-store 2>/dev/null || true
    echo "  [OK] .pnpm-store 已清理"
fi
echo ""

# ============================================================
# Step 4: 重新安装依赖
# ============================================================
echo "[4/4] 重新安装依赖..."
echo ""

if command -v pnpm >/dev/null 2>&1; then
    echo "  [运行] pnpm install"
    if pnpm install; then
        echo ""
        echo "========================================"
        echo "  修复完成"
        echo "========================================"
        echo "依赖已重新安装，EPERM 错误应已解决"
    else
        echo ""
        echo "[警告] pnpm install 仍报错，可能原因："
        echo "  1. 权限不足 - 尝试 sudo chown -R \$USER:\$USER ."
        echo "  2. IDE 占用 - 完全关闭 IDE 后重试"
        echo "  3. 磁盘错误 - 检查磁盘空间和权限"
        exit 1
    fi
else
    echo "[错误] pnpm 命令不可用，请先安装: pvm install pnpm@latest"
    exit 1
fi

exit 0
