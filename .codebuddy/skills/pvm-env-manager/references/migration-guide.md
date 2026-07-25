# 迁移指南：从其他版本管理器迁移到 PVM

## 通用迁移流程

```
1. 安装 PVM（如未安装）
2. 查看旧工具管理的版本
3. 用 PVM 安装相同版本
4. 用 PVM 切换到该版本
5. 验证版本正确
6. 卸载旧工具
7. 运行 pvm setup 修复 PATH
```

---

## 从 nvm 迁移

### Windows

```powershell
# 1. 安装 PVM（如未安装）
# 方式 A：一键脚本（可能 404 则用方式 B）
iwr -useb https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.ps1 | iex
# 方式 B：从 https://github.com/Lycorxy/pvm/releases 下载 pvm-windows-amd64.exe
#       放到 %USERPROFILE%\.pvm\bin\pvm.exe，然后运行 pvm setup

# 2. 查看 nvm 当前版本
nvm list

# 3. 用 PVM 安装相同版本
pvm install node@20

# 4. 切换
pvm use node@20

# 5. 验证
node --version

# 6. 卸载 nvm（通过统一卸载脚本）
scripts\uninstall-tool.bat nvm --yes

# 7. 修复 PATH
pvm setup
```

### macOS / Linux

```bash
# 1. 安装 PVM
# 方式 A：一键脚本（可能 404 则用方式 B）
curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.sh | bash
# 方式 B：从 https://github.com/Lycorxy/pvm/releases 下载对应平台二进制
#       放到 ~/.pvm/bin/pvm，chmod +x，然后运行 ~/.pvm/bin/pvm setup

# 2. 查看 nvm 版本
nvm ls

# 3. 安装 + 切换
pvm install node@20
pvm use node@20

# 4. 验证
node --version

# 5. 卸载 nvm
rm -rf ~/.nvm

# 6. 清理 shell rc 中的 nvm 初始化行
# 手动编辑 ~/.bashrc 或 ~/.zshrc，删除以下内容：
# export NVM_DIR="$HOME/.nvm"
# [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
# [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"

# 7. 修复 PATH
pvm setup
```

---

## 从 fnm 迁移

```bash
# 查看 fnm 版本
fnm list

# PVM 安装相同版本
pvm install node@20
pvm use node@20

# 卸载 fnm
# macOS (brew)
brew uninstall fnm
rm -rf ~/.fnm

# 或手动
rm -rf ~/.fnm

# 清理 shell rc 中的 fnm 初始化行
# eval "$(fnm env)"  ← 删除这行
```

---

## 从 pyenv 迁移

```bash
# 查看 pyenv 版本
pyenv versions

# PVM 安装相同版本
pvm install python@3.12
pvm use python@3.12

# 验证
python --version

# 卸载 pyenv
rm -rf ~/.pyenv

# 清理 shell rc 中的 pyenv 初始化行
# export PYENV_ROOT="$HOME/.pyenv"  ← 删除
# export PATH="$PYENV_ROOT/bin:$PATH"  ← 删除
# eval "$(pyenv init -)"  ← 删除
```

---

## 从 rustup 迁移

```bash
# 查看 rust 版本
rustup show

# PVM 安装相同版本
pvm install rust@1.78
pvm use rust@1.78

# 验证
rustc --version

# 卸载 rustup
rustup self uninstall

# 或手动
rm -rf ~/.rustup ~/.cargo
```

---

## 从 Volta 迁移

```bash
# 查看 volta 版本
volta list

# PVM 安装
pvm install node@20
pvm install pnpm@9
pvm use node@20
pvm use pnpm@9

# 卸载 volta
# macOS
brew uninstall volta
rm -rf ~/.volta

# Windows
# 通过「添加或删除程序」卸载 Volta
# 然后删除 %LOCALAPPDATA%\Volta
```

---

## 从 asdf 迁移

```bash
# 查看 asdf 版本
asdf current

# PVM 逐个安装
pvm install node@20
pvm install python@3.12
pvm install rust@1.78
pvm use node@20
pvm use python@3.12
pvm use rust@1.78

# 卸载 asdf
rm -rf ~/.asdf

# 清理 shell rc 中的 asdf 初始化行
# . $HOME/.asdf/asdf.sh  ← 删除
```

---

## 迁移后验证

```bash
# 确认所有旧工具已清除
which nvm        # 应无输出
which fnm        # 应无输出
which pyenv      # 应无输出
which rustup     # 应无输出
which volta      # 应无输出
which asdf       # 应无输出

# 确认 PVM 管理的运行时正常
pvm current
pvm list
pvm doctor

# 确认版本正确
node --version
python --version
```

---

## 常见问题

### Q: 迁移后 `node --version` 还是旧版本？

A: 系统 PATH 中有冲突路径。运行 `pvm setup` 修复，或手动检查：

```bash
# Windows
where.exe node

# macOS/Linux
which -a node
```

确保 `~/.pvm/shims/node` 排在最前面。

### Q: 迁移后 npm 找不到？

A: 运行 `pvm reshim` 重新生成 shim，然后重启终端。

### Q: 可以保留旧工具和 PVM 共存吗？

A: 不建议。多个版本管理器共存会导致 PATH 冲突，`pvm use` 可能无效。建议彻底迁移。
