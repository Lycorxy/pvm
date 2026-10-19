#!/usr/bin/env bash
# ============================================================
# PVM 统一卸载脚本 (macOS / Linux)
# 用法: ./uninstall-tool.sh <软件名> [--yes]
# 支持: node git python rust go bun deno pnpm yarn pvm
#       nvm volta fnm nodenv pyenv rustup asdf conda (18种)
# ============================================================

set -euo pipefail

TOOL_NAME="${1:-}"
SKIP_CONFIRM=""

if [ -z "$TOOL_NAME" ]; then
    echo ""
    echo "  PVM卸载工具 - 支持的软件:"
    echo "  运行时: node git python rust go bun deno pnpm yarn pvm"
    echo "  冲突:   nvm volta fnm nodenv pyenv rustup asdf conda"
    echo ""
    echo "  用法: ./uninstall-tool.sh <名称> [--yes]"
    exit 1
fi

[ "${2:-}" = "--yes" ] && SKIP_CONFIRM="y"

# 标准化名称 (小写)
TOOL_NAME=$(echo "$TOOL_NAME" | tr '[:upper:]' '[:lower:]')

# 验证支持的软件
SUPPORTED="node git python rust go bun deno pnpm yarn pvm nvm volta fnm nodenv pyenv rustup asdf conda"
if ! echo "$SUPPORTED" | grep -qw "$TOOL_NAME"; then
    echo "[错误] 不支持: $TOOL_NAME"
    echo "支持: $SUPPORTED"
    exit 1
fi

echo ""
echo "[目标] 卸载: $TOOL_NAME"

if [ -z "$SKIP_CONFIRM" ]; then
    read -rp "确认? (y/n): " CONFIRM
    if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
        echo "已取消"; exit 0
    fi
fi

# ---- 通用函数 ----
rm_dir() { for d in "$@"; do [ -d "$d" ] && rm -rf "$d" && echo "  删除: $d" || true; }
kill_proc() { pkill -f "$1" 2>/dev/null || true; }

clean_rc() {
    local pattern="$1"
    # 清理 shell rc 文件中的相关行
    for rc in ~/.bashrc ~/.bash_profile ~/.zshrc ~/.profile; do
        [ -f "$rc" ] || continue
        if grep -qE "$pattern" "$rc" 2>/dev/null; then
            local tmp=$(mktemp)
            grep -vE "$pattern" "$rc" > "$tmp" 2>/dev/null || true
            mv "$tmp" "$rc"
            echo "  清理: $rc ($pattern)"
        fi
    done
}

# ===================== Node.js =====================
uninstall_node() {
    echo "[1/3] 终止进程..."
    kill_proc "node|npm|npx"
    echo "[2/3] 删除目录..."
    rm_dir /usr/local/lib/node_modules ~/.npm ~/.nvm "$HOME/.npm"
    [ -d "$(brew --prefix 2>/dev/null)/lib/node_modules" ] && brew uninstall node 2>/dev/null || true
    echo "[3/3] 完成"
}

# ===================== Git =====================
uninstall_git() {
    echo "[1/2] 终止进程..."
    kill_proc "git"
    echo "[2/2] 删除目录..."
    rm_dir /usr/local/git /opt/homebrew/opt/git-cellar
    clean_rc "git"
}

# ===================== Python =====================
uninstall_python() {
    echo "[1/3] 终止进程..."
    kill_proc "python|pip"
    echo "[2/3] 删除目录..."
    rm_dir ~/.local/lib/python* ~/Library/Python "$HOME/.pyenv/versions"/* 2>/dev/null || true
    echo "[3/3] 完成"
}

# ===================== Rust =====================
uninstall_rust() {
    echo "[1/2] 终止进程 + 删除 .cargo/.rustup..."
    kill_proc "rustc|cargo|rustup"
    rm_dir "$HOME/.cargo" "$HOME/.rustup"
    echo "[2/2] 清理环境变量..."
    clean_rc "cargo|rustup|\.rust"
}

# ===================== Go =====================
uninstall_go() {
    echo "[1/3] 终止进程..."
    kill_proc "go"
    echo "[2/3] 删除目录..."
    rm_dir /usr/local/go /opt/homebrew/opt/go "$HOME/go" "$HOME/.go"
    echo "[3/3] 完成"
}

# ===================== Bun =====================
uninstall_bun() {
    echo "[1/2] 终止进程 + 删除 .bun..."
    kill_proc "bun"
    rm_dir "$HOME/.bun" ~/.bun
    echo "[2/2] 清理 shell 配置..."
    clean_rc "bun"
}

# ===================== Deno =====================
uninstall_deno() {
    echo "[1/2] 终止进程 + 删除 .deno..."
    kill_proc "deno"
    rm_dir "$HOME/.deno" ~/.deno ~/.cache/deno ~/.local/share/deno
    echo "[2/2] 完成"
}

# ===================== PNPM =====================
uninstall_pnpm() {
    echo "[1/2] 终止进程 + 删除缓存..."
    kill_proc "pnpm"
    rm_dir "$HOME/Library/pnpm" "$HOME/.pnpm-store" "$HOME/.pnpm-global" ~/.local/share/pnpm
    echo "[2/2] 完成"
}

# ===================== Yarn =====================
uninstall_yarn() {
    echo "[1/2] 终止进程 + 删除 .yarn..."
    kill_proc "yarn|yarnpkg"
    rm_dir "$HOME/.yarn" "$HOME/.yarn/config" ~/.config/yarn
    echo "[2/2] 完成"
}

# ===================== PVM =====================
uninstall_pvm() {
    echo "[1/2] 调用 pvm 自身卸载..."
    command -v pvm >/dev/null 2>&1 && pvm uninstall --yes 2>/dev/null || true
    kill_proc "pvm"
    rm_dir "$HOME/.pvm"
    echo "[2/2] 完成"
}

# ===================== NVM =====================
uninstall_nvm() {
    echo "[1/2] 删除 .nvm..."
    kill_proc "nvm"
    rm_dir "$HOME/.nvm" "$HOME/nvm" ~/.nvm
    echo "[2/2] 清理 shell 配置..."
    clean_rc "NVM_DIR|nvm"
}

# ===================== Volta =====================
uninstall_volta() {
    echo "[1/2] 删除 .volta..."
    kill_proc "volta"
    rm_dir "$HOME/.volta" ~/.volta
    echo "[2/2] 清理环境变量..."
    clean_rc "volta|VOLTA"
}

# ===================== FNM =====================
uninstall_fnm() {
    echo "[1/2] 删除 .fnm..."
    kill_proc "fnm"
    rm_dir "$HOME/.fnm" ~/.fnm "$LOCALAPPDATA/fnm" "$APPDATA/fnm"
    echo "[2/2] 清理 shell 配置..."
    clean_rc "fnm"
}

# ===================== nodenv =====================
uninstall_nodenv() {
    echo "[1/2] 删除 .nodenv..."
    kill_proc "nodenv"
    rm_dir "$HOME/.nodenv"
    echo "[2/2] 清理环境变量..."
    clean_rc "nodenv|NODENV"
}

# ===================== pyenv =====================
uninstall_pyenv() {
    echo "[1/2] 删除 .pyenv..."
    kill_proc "pyenv"
    rm_dir "$HOME/.pyenv"
    echo "[2/2] 清理环境变量..."
    clean_rc "pyenv|PYENV"
}

# ===================== rustup =====================
uninstall_rustup() {
    echo "[1/2] 终止进程 + 删除 .rustup/.cargo..."
    kill_proc "rustup|rustc|cargo"
    rm_dir "$HOME/.rustup" "$HOME/.cargo"
    echo "[2/2] 清理环境变量..."
    clean_rc "rustup|cargo|\.rust"
}

# ===================== asdf =====================
uninstall_asdf() {
    echo "[1/2] 删除 .asdf..."
    rm_dir "$HOME/.asdf"
    command -v brew >/dev/null 2>&1 && brew uninstall asdf 2>/dev/null || true
    echo "[2/2] 清理环境变量..."
    clean_rc "asdf|ASDF"
}

# ===================== Conda =====================
uninstall_conda() {
    echo "[1/3] 终止进程..."
    kill_proc "conda"
    echo "[2/3] 删除 Anaconda/Miniconda..."
    rm_dir "$HOME/anaconda3" "$HOME/miniconda3" "$HOME/miniforge3" "$HOME/mambaforge" \
          "/opt/anaconda3" "/opt/miniconda3" "$HOME/.condarc" "$HOME/.conda"
    command -v conda >/dev/null 2>&1 && conda init --reverse --all 2>/dev/null || true
    echo "[3/3] 清理环境变量..."
    clean_rc "conda|anaconda|miniconda|CONDA_"
}

# ===================== 分发 =====================
case "$TOOL_NAME" in
    node)   uninstall_node ;;
    git)    uninstall_git ;;
    python) uninstall_python ;;
    rust)   uninstall_rust ;;
    go)     uninstall_go ;;
    bun)    uninstall_bun ;;
    deno)   uninstall_deno ;;
    pnpm)   uninstall_pnpm ;;
    yarn)   uninstall_yarn ;;
    pvm)    uninstall_pvm ;;
    nvm)    uninstall_nvm ;;
    volta)  uninstall_volta ;;
    fnm)    uninstall_fnm ;;
    nodenv) uninstall_nodenv ;;
    pyenv)  uninstall_pyenv ;;
    rustup) uninstall_rustup ;;
    asdf)   uninstall_asdf ;;
    conda)  uninstall_conda ;;
esac

echo ""
echo "[done] $TOOL_NAME 已卸载，重启终端生效"
