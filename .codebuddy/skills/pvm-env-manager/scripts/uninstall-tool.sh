#!/bin/bash
# ============================================================================
# PVM 环境管理器 - 软件彻底卸载脚本 (macOS/Linux)
# ============================================================================
# 功能：彻底卸载指定软件，包括文件、环境变量、配置文件
# 用法：./uninstall-tool.sh <软件名> [--yes]
#   软件名: node, git, nvm, pvm, python, yarn, pnpm
#   --yes   : 跳过确认直接卸载
# ============================================================================

set -e

echo "========================================"
echo "  软件彻底卸载工具"
echo "========================================"
echo

# 参数检查
if [ $# -eq 0 ]; then
    echo "[错误] 请指定要卸载的软件名称"
    echo "用法: $0 <软件名> [--yes]"
    echo
    echo "支持的软件:"
    echo "  node   - Node.js"
    echo "  git    - Git"
    echo "  nvm    - Node Version Manager"
    echo "  pvm    - Polyglot Version Manager"
    echo "  python - Python"
    echo "  yarn   - Yarn"
    echo "  pnpm   - PNPM"
    exit 1
fi

TOOL_NAME=$(echo "$1" | tr '[:upper:]' '[:lower:]')
SKIP_CONFIRM="${2:-no}"

if [ "$SKIP_CONFIRM" != "--yes" ]; then
    SKIP_CONFIRM="no"
fi

echo "[目标] 正在准备卸载: $TOOL_NAME"
echo

# 确认操作
if [ "$SKIP_CONFIRM" = "no" ]; then
    echo "[警告] 此操作将彻底删除 $TOOL_NAME 及其所有相关文件、环境变量。"
    echo "        此操作不可逆！"
    echo
    read -p "确认卸载? (yes/no): " CONFIRM
    if [ "$CONFIRM" != "yes" ]; then
        echo "[取消] 用户取消操作"
        exit 0
    fi
fi

echo
echo "========================================"
echo "  开始卸载: $TOOL_NAME"
echo "========================================"
echo

# 根据软件类型执行不同的卸载流程
case $TOOL_NAME in
    node)   uninstall_node ;;
    git)    uninstall_git ;;
    nvm)    uninstall_nvm ;;
    pvm)    uninstall_pvm ;;
    python) uninstall_python ;;
    yarn)   uninstall_yarn ;;
    pnpm)   uninstall_pnpm ;;
    *)
        echo "[错误] 不支持的软件: $TOOL_NAME"
        exit 1
        ;;
esac

echo
echo "========================================"
echo "  卸载完成: $TOOL_NAME"
echo "========================================"
echo
echo "[建议] 请重启终端或运行 'source ~/.bashrc' 以使环境变量生效"

exit 0

# ============================================================================
# 卸载 Node.js
# ============================================================================
uninstall_node() {
    echo "[步骤 1] 查找 Node.js 安装路径"
    NODE_PATHS=$(which -a node 2>/dev/null || true)
    
    if [ -z "$NODE_PATHS" ]; then
        echo "[提示] Node.js 未安装或未在 PATH 中"
        return
    fi
    
    echo "$NODE_PATHS" | while read -r node_path; do
        echo "[发现] $node_path"
    done
    
    echo
    echo "[步骤 2] 终止 Node.js 进程"
    pkill -f node || true
    pkill -f npm || true
    pkill -f npx || true
    echo "[完成] 进程已终止"
    
    echo
    echo "[步骤 3] 删除安装目录"
    # 删除常见的 Node.js 安装位置
    for dir in \
        "/usr/local/bin/node" \
        "/usr/local/lib/node_modules" \
        "/usr/local/include/node" \
        "$HOME/.npm" \
        "$HOME/.node-gyp" \
        "$HOME/.nvm"
    do
        if [ -e "$dir" ]; then
            echo "[删除] $dir"
            rm -rf "$dir" 2>/dev/null || sudo rm -rf "$dir" 2>/dev/null || echo "[警告] 无法删除: $dir"
        fi
    done
    
    echo
    echo "[步骤 4] 清理 shell 配置"
    for rc_file in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$rc_file" ]; then
            # 删除 Node.js 相关的 PATH 配置
            sed -i.bak '/node/d' "$rc_file" 2>/dev/null || true
            echo "[清理] $rc_file"
        fi
    done
    
    echo
    echo "[完成] Node.js 已卸载"
}

# ============================================================================
# 卸载 Git
# ============================================================================
uninstall_git() {
    echo "[步骤 1] 查找 Git 安装路径"
    GIT_PATH=$(which git 2>/dev/null || true)
    
    if [ -z "$GIT_PATH" ]; then
        echo "[提示] Git 未安装或未在 PATH 中"
        return
    fi
    
    echo "[发现] $GIT_PATH"
    
    echo
    echo "[步骤 2] 删除安装目录"
    
    # macOS: 通过 brew 安装
    if command -v brew >/dev/null 2>&1; then
        echo "[卸载] 通过 Homebrew 卸载"
        brew uninstall git 2>/dev/null || true
    fi
    
    # Linux: 通过包管理器卸载
    if command -v apt-get >/dev/null 2>&1; then
        echo "[卸载] 通过 apt 卸载"
        sudo apt-get remove -y git 2>/dev/null || true
    elif command -v yum >/dev/null 2>&1; then
        echo "[卸载] 通过 yum 卸载"
        sudo yum remove -y git 2>/dev/null || true
    fi
    
    echo
    echo "[步骤 3] 清理配置文件"
    if [ -d "$HOME/.gitconfig" ]; then
        echo "[删除] $HOME/.gitconfig"
        rm -rf "$HOME/.gitconfig"
    fi
    
    echo
    echo "[完成] Git 已卸载"
}

# ============================================================================
# 卸载 NVM
# ============================================================================
uninstall_nvm() {
    echo "[步骤 1] 检查 NVM 安装"
    if [ ! -d "$HOME/.nvm" ] && [ ! -d "$HOME/.nvm" ]; then
        echo "[提示] NVM 未安装"
        return
    fi
    
    echo
    echo "[步骤 2] 删除 NVM 目录"
    if [ -d "$HOME/.nvm" ]; then
        echo "[删除] $HOME/.nvm"
        rm -rf "$HOME/.nvm"
    fi
    
    echo
    echo "[步骤 3] 清理 shell 配置"
    for rc_file in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$rc_file" ]; then
            # 删除 NVM 相关配置
            sed -i.bak '/NVM_DIR/d' "$rc_file" 2>/dev/null || true
            sed -i.bak '/nvm.sh/d' "$rc_file" 2>/dev/null || true
            echo "[清理] $rc_file"
        fi
    done
    
    echo
    echo "[完成] NVM 已卸载"
}

# ============================================================================
# 卸载 PVM
# ============================================================================
uninstall_pvm() {
    echo "[步骤 1] 检查 PVM 安装"
    if [ ! -d "$HOME/.pvm" ]; then
        echo "[提示] PVM 未安装"
        return
    fi
    
    echo
    echo "[步骤 2] 使用 PVM 自卸载命令"
    if command -v pvm >/dev/null 2>&1; then
        pvm uninstall --yes 2>/dev/null || true
    fi
    
    echo
    echo "[步骤 3] 删除 PVM 目录"
    if [ -d "$HOME/.pvm" ]; then
        echo "[删除] $HOME/.pvm"
        rm -rf "$HOME/.pvm"
    fi
    
    echo
    echo "[步骤 4] 清理 shell 配置"
    for rc_file in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$rc_file" ]; then
            # 删除 PVM 相关配置
            sed -i.bak '/\.pvm/d' "$rc_file" 2>/dev/null || true
            echo "[清理] $rc_file"
        fi
    done
    
    echo
    echo "[完成] PVM 已卸载"
}

# ============================================================================
# 卸载 Python
# ============================================================================
uninstall_python() {
    echo "[步骤 1] 查找 Python 安装路径"
    PYTHON_PATH=$(which python 2>/dev/null || true)
    PYTHON3_PATH=$(which python3 2>/dev/null || true)
    
    if [ -z "$PYTHON_PATH" ] && [ -z "$PYTHON3_PATH" ]; then
        echo "[提示] Python 未安装或未在 PATH 中"
        return
    fi
    
    [ -n "$PYTHON_PATH" ] && echo "[发现] $PYTHON_PATH"
    [ -n "$PYTHON3_PATH" ] && echo "[发现] $PYTHON3_PATH"
    
    echo
    echo "[步骤 2] 通过包管理器卸载"
    
    # macOS: 通过 brew 安装
    if command -v brew >/dev/null 2>&1; then
        echo "[卸载] 通过 Homebrew 卸载"
        brew uninstall python@3 2>/dev/null || true
        brew uninstall python 2>/dev/null || true
    fi
    
    # Linux: 通过包管理器卸载
    if command -v apt-get >/dev/null 2>&1; then
        echo "[卸载] 通过 apt 卸载"
        sudo apt-get remove -y python3 2>/dev/null || true
    elif command -v yum >/dev/null 2>&1; then
        echo "[卸载] 通过 yum 卸载"
        sudo yum remove -y python3 2>/dev/null || true
    fi
    
    echo
    echo "[步骤 3] 清理配置文件"
    for dir in "$HOME/.python_history" "$HOME/.pip"; do
        if [ -e "$dir" ]; then
            echo "[删除] $dir"
            rm -rf "$dir"
        fi
    done
    
    echo
    echo "[完成] Python 已卸载"
}

# ============================================================================
# 卸载 Yarn
# ============================================================================
uninstall_yarn() {
    echo "[步骤 1] 检查 Yarn 安装"
    if ! command -v yarn >/dev/null 2>&1; then
        echo "[提示] Yarn 未安装"
        return
    fi
    
    echo
    echo "[步骤 2] 通过包管理器卸载"
    
    # macOS: 通过 brew 安装
    if command -v brew >/dev/null 2>&1; then
        echo "[卸载] 通过 Homebrew 卸载"
        brew uninstall yarn 2>/dev/null || true
    fi
    
    # 通过 npm 全局卸载
    if command -v npm >/dev/null 2>&1; then
        echo "[卸载] 通过 npm 卸载"
        npm uninstall -g yarn 2>/dev/null || true
    fi
    
    echo
    echo "[步骤 3] 删除 Yarn 全局目录"
    if [ -d "$HOME/.yarn" ]; then
        echo "[删除] $HOME/.yarn"
        rm -rf "$HOME/.yarn"
    fi
    
    if [ -d "$HOME/.config/yarn" ]; then
        echo "[删除] $HOME/.config/yarn"
        rm -rf "$HOME/.config/yarn"
    fi
    
    echo
    echo "[完成] Yarn 已卸载"
}

# ============================================================================
# 卸载 PNPM
# ============================================================================
uninstall_pnpm() {
    echo "[步骤 1] 检查 PNPM 安装"
    if ! command -v pnpm >/dev/null 2>&1; then
        echo "[提示] PNPM 未安装"
        return
    fi
    
    echo
    echo "[步骤 2] 通过 npm 全局卸载"
    if command -v npm >/dev/null 2>&1; then
        echo "[卸载] 通过 npm 卸载"
        npm uninstall -g pnpm 2>/dev/null || true
    fi
    
    echo
    echo "[步骤 3] 删除 PNPM 全局目录"
    for dir in "$HOME/.pnpm-store" "$HOME/.pnpm-global" "$HOME/Library/pnpm"; do
        if [ -d "$dir" ]; then
            echo "[删除] $dir"
            rm -rf "$dir"
        fi
    done
    
    echo
    echo "[步骤 4] 清理 shell 配置"
    for rc_file in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$rc_file" ]; then
            # 删除 PNPM 相关配置
            sed -i.bak '/pnpm/d' "$rc_file" 2>/dev/null || true
            echo "[清理] $rc_file"
        fi
    done
    
    echo
    echo "[完成] PNPM 已卸载"
}