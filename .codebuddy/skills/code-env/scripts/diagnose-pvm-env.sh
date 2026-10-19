#!/bin/bash
# ============================================================================
# PVM 环境管理器 - 环境诊断脚本 (macOS/Linux)
# ============================================================================
# 功能：诊断 PVM 环境，检测冲突、环境变量问题、端口占用等
# 用法：./diagnose-pvm-env.sh [--fix]
#   --fix : 自动修复发现的问题
# ============================================================================

set -e

echo "========================================"
echo "  PVM 环境诊断工具"
echo "========================================"
echo

FIX_MODE="${1:-no}"
if [ "$FIX_MODE" = "--fix" ]; then
    FIX_MODE="yes"
else
    FIX_MODE="no"
fi

PASS_COUNT=0
FAIL_COUNT=0
WARNING_COUNT=0

# ============================================================================
# 检查 1: PVM 安装状态
# ============================================================================
echo "[检查 1] PVM 安装状态"
if command -v pvm >/dev/null 2>&1; then
    PVM_VERSION=$(pvm --version 2>/dev/null || echo "unknown")
    echo "[√] PVM 已安装: $PVM_VERSION"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "[X] PVM 未安装"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi
echo

# ============================================================================
# 检查 2: 环境变量
# ============================================================================
echo "[检查 2] 环境变量配置"

# 检查 PVM_HOME
if [ -n "$PVM_HOME" ]; then
    echo "[√] PVM_HOME = $PVM_HOME"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "[!] PVM_HOME 未设置（可选）"
    WARNING_COUNT=$((WARNING_COUNT + 1))
fi

# 检查 PATH
PATH_ISSUE=0
if echo "$PATH" | grep -q "\.pvm/shims"; then
    echo "[√] PATH 包含 .pvm/shims"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "[X] PATH 缺少 .pvm/shims"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    PATH_ISSUE=1
    
    if [ "$FIX_MODE" = "yes" ]; then
        echo "[修复] 正在添加 .pvm/shims 到 PATH..."
        
        # 检测 shell 类型并更新配置文件
        SHELL_RC=""
        if [ -n "$ZSH_VERSION" ]; then
            SHELL_RC="$HOME/.zshrc"
        elif [ -n "$BASH_VERSION" ]; then
            SHELL_RC="$HOME/.bashrc"
        fi
        
        if [ -n "$SHELL_RC" ]; then
            echo "export PATH=\"$HOME/.pvm/shims:\$PATH\"" >> "$SHELL_RC"
            echo "[完成] PATH 已更新，请运行: source $SHELL_RC"
        fi
    fi
fi

echo

# ============================================================================
# 检查 3: 冲突工具
# ============================================================================
echo "[检查 3] 冲突工具检测"

CONFLICT_TOOLS=""
for tool in nvm fnm pyenv rustup volta asdf; do
    if command -v "$tool" >/dev/null 2>&1; then
        echo "[X] 检测到冲突工具: $tool"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        CONFLICT_TOOLS="$CONFLICT_TOOLS $tool"
    fi
done

if [ -z "$CONFLICT_TOOLS" ]; then
    echo "[√] 无冲突工具"
    PASS_COUNT=$((PASS_COUNT + 1))
fi

if [ -n "$CONFLICT_TOOLS" ]; then
    echo
    echo "[警告] 发现冲突工具: $CONFLICT_TOOLS"
    echo "[建议] 运行: pvm migrate 或手动卸载冲突工具"
fi

echo

# ============================================================================
# 检查 4: PVM 目录结构
# ============================================================================
echo "[检查 4] PVM 目录结构"

PVM_DIR="$HOME/.pvm"
if [ -d "$PVM_DIR" ]; then
    echo "[√] PVM 目录存在: $PVM_DIR"
    PASS_COUNT=$((PASS_COUNT + 1))
    
    # 检查子目录
    if [ -d "$PVM_DIR/bin" ]; then
        echo "[√] bin 目录存在"
    else
        echo "[X] bin 目录缺失"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    
    if [ -d "$PVM_DIR/shims" ]; then
        echo "[√] shims 目录存在"
    else
        echo "[X] shims 目录缺失"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        
        if [ "$FIX_MODE" = "yes" ]; then
            echo "[修复] 正在创建 shims 目录并生成 shim..."
            pvm reshim
        fi
    fi
else
    echo "[X] PVM 目录不存在"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo

# ============================================================================
# 检查 5: 运行时安装状态
# ============================================================================
echo "[检查 5] 运行时安装状态"

if [ -d "$PVM_DIR/installs" ]; then
    RUNTIME_COUNT=$(find "$PVM_DIR/installs" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
    
    if [ "$RUNTIME_COUNT" -gt 0 ]; then
        echo "[√] 已安装 $RUNTIME_COUNT 个运行时"
        find "$PVM_DIR/installs" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | while read -r runtime; do
            echo "    - $runtime"
        done
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "[!] 未安装任何运行时"
        WARNING_COUNT=$((WARNING_COUNT + 1))
    fi
else
    echo "[!] installs 目录不存在"
    WARNING_COUNT=$((WARNING_COUNT + 1))
fi

echo

# ============================================================================
# 检查 6: 端口冲突
# ============================================================================
echo "[检查 6] 端口冲突检测"

# 常见的开发端口
PORTS="3000 8080 8000 5000 4000 9000"
for port in $PORTS; do
    if lsof -i ":$port" >/dev/null 2>&1; then
        echo "[!] 端口 $port 已被占用"
        WARNING_COUNT=$((WARNING_COUNT + 1))
        
        if [ "$FIX_MODE" = "yes" ]; then
            echo "[提示] 请手动处理端口冲突或使用: lsof -i :$port"
        fi
    fi
done

echo

# ============================================================================
# 检查 7: .npmrc 配置
# ============================================================================
echo "[检查 7] .npmrc 配置"

if [ -f "$HOME/.npmrc" ]; then
    echo "[√] .npmrc 文件存在"
    
    # 检查是否配置了国内镜像
    if grep -q "registry" "$HOME/.npmrc"; then
        echo "[√] 已配置 registry"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "[!] 未配置 registry（建议配置国内镜像）"
        WARNING_COUNT=$((WARNING_COUNT + 1))
    fi
else
    echo "[!] .npmrc 文件不存在"
    WARNING_COUNT=$((WARNING_COUNT + 1))
    
    if [ "$FIX_MODE" = "yes" ]; then
        echo "[修复] 正在创建 .npmrc..."
        create_npmrc
    fi
fi

echo

# ============================================================================
# 检查 8: Shell 配置文件
# ============================================================================
echo "[检查 8] Shell 配置文件"

# 检测当前使用的 shell 配置文件
SHELL_RC=""
if [ -n "$ZSH_VERSION" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -n "$BASH_VERSION" ]; then
    if [ -f "$HOME/.bashrc" ]; then
        SHELL_RC="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        SHELL_RC="$HOME/.bash_profile"
    fi
fi

if [ -n "$SHELL_RC" ] && [ -f "$SHELL_RC" ]; then
    echo "[√] Shell 配置文件存在: $SHELL_RC"
    
    if grep -q "pvm" "$SHELL_RC"; then
        echo "[√] Shell 配置文件包含 PVM 配置"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "[!] Shell 配置文件缺少 PVM 配置（可选）"
        WARNING_COUNT=$((WARNING_COUNT + 1))
    fi
else
    echo "[!] Shell 配置文件不存在（可选）"
    WARNING_COUNT=$((WARNING_COUNT + 1))
fi

echo

# ============================================================================
# 总结
# ============================================================================
echo "========================================"
echo "  诊断结果"
echo "========================================"
echo
echo "通过: $PASS_COUNT 项"
echo "失败: $FAIL_COUNT 项"
echo "警告: $WARNING_COUNT 项"
echo

if [ $FAIL_COUNT -gt 0 ]; then
    echo "[状态] 发现 $FAIL_COUNT 个问题需要处理"
    if [ "$FIX_MODE" = "no" ]; then
        echo "[建议] 运行: $0 --fix 自动修复问题"
    fi
elif [ $WARNING_COUNT -gt 0 ]; then
    echo "[状态] 环境基本正常，有 $WARNING_COUNT 个警告"
else
    echo "[状态] 环境完全正常 ✓"
fi

exit 0

# ============================================================================
# 创建 .npmrc
# ============================================================================
create_npmrc() {
    cat > "$HOME/.npmrc" << 'EOF'
# pnpm 核心配置
registry=https://registry.npmmirror.com/
save-prefix=""
auto-install-peers=true

# lockfile 冻结策略
strict-peer-dependencies=false
prefer-frozen-lockfile=true

# 版本与冲突处理
resolution-mode=time-based
prefer-higher-version=true

# 依赖结构
shamefully-hoist=false
strict-store-content=true
EOF

    echo "[完成] .npmrc 已创建"
    PASS_COUNT=$((PASS_COUNT + 1))
}