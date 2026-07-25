#!/bin/bash
# ============================================================================
# PVM 环境管理器 - .npmrc 自动配置脚本 (macOS/Linux)
# ============================================================================
# 功能：自动创建或更新 .npmrc 文件，配置 pnpm 最佳实践
# 用法：./setup-npmrc.sh [--force]
#   --force : 强制覆盖现有配置
# ============================================================================

set -e

echo "========================================"
echo "  .npmrc 自动配置工具"
echo "========================================"
echo

FORCE_MODE="${1:-no}"
if [ "$FORCE_MODE" = "--force" ]; then
    FORCE_MODE="yes"
else
    FORCE_MODE="no"
fi

NPMRC_PATH="$HOME/.npmrc"

# 检查文件是否已存在
if [ -f "$NPMRC_PATH" ]; then
    if [ "$FORCE_MODE" = "no" ]; then
        echo "[提示] .npmrc 文件已存在"
        echo
        echo "当前配置:"
        echo "----------------------------------------"
        cat "$NPMRC_PATH"
        echo "----------------------------------------"
        echo
        read -p "是否覆盖? (yes/no): " OVERWRITE
        if [ "$OVERWRITE" != "yes" ]; then
            echo "[取消] 保留现有配置"
            exit 0
        fi
    fi
    
    echo "[备份] 正在备份现有配置..."
    cp "$NPMRC_PATH" "$NPMRC_PATH.backup-$(date +%Y%m%d)" 2>/dev/null || true
fi

echo
echo "[创建] 正在生成 .npmrc 配置..."

# 写入配置
cat > "$NPMRC_PATH" << 'EOF'
# ============================================
# pnpm 核心配置
# ============================================
registry=https://registry.npmmirror.com/
# 保存依赖时不添加版本前缀（^ ~），直接锁定精确版本
save-prefix=""

# 自动安装 peer 依赖（无需手动安装）
auto-install-peers=true

# ============================================
# lockfile 冻结策略
# ============================================

# 本地开发：严格按 lockfile 安装，不自动更新
# （CI 环境用 --frozen-lockfile 参数）
strict-peer-dependencies=false

# 优先使用现有 lockfile，避免意外更新
prefer-frozen-lockfile=true

# ============================================
# 版本与冲突处理
# ============================================

# 版本解析策略：基于时间，保持一致性
resolution-mode=time-based

# peer 依赖冲突时取最高版本
prefer-higher-version=true

# ============================================
# 依赖结构
# ============================================

shamefully-hoist=false
strict-store-content=true
EOF

if [ -f "$NPMRC_PATH" ]; then
    echo
    echo "========================================"
    echo "  配置完成"
    echo "========================================"
    echo
    echo "[√] .npmrc 已创建: $NPMRC_PATH"
    echo
    echo "配置内容:"
    echo "----------------------------------------"
    cat "$NPMRC_PATH"
    echo "----------------------------------------"
    echo
    echo "[说明] 此配置已包含:"
    echo "  - 国内镜像源（npmmirror）"
    echo "  - 精确版本锁定（save-prefix=\"\"）"
    echo "  - 自动安装 peer 依赖"
    echo "  - lockfile 冻结策略"
    echo "  - 版本解析策略"
    echo "  - 依赖结构优化"
    echo
    echo "[建议] 配合 .pvmrc 文件使用，确保团队环境一致"
else
    echo "[X] 创建失败"
    exit 1
fi

exit 0