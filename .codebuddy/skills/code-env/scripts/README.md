# 实用工具脚本

所有脚本位于 `.codebuddy/skills/pvm-env-manager/scripts/`，支持 Windows、macOS 和 Linux。

---

## 环境检测（判断该执行哪个安装）

**脚本：** `detect-env.ps1` / `detect-env.sh`

自动检测当前操作系统、CPU 架构（含 Rosetta 识别原生 Apple Silicon）、默认 shell、是否已安装 PVM，并输出匹配的 Release 资产和推荐安装命令。

```powershell
# Windows
& "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\detect-env.ps1"
```

```bash
# macOS / Linux（Git Bash 下会识别为 Windows 并推荐 PowerShell 安装器）
bash .codebuddy/skills/pvm-env-manager/scripts/detect-env.sh
```

输出示例：

```text
os=darwin
arch=arm64
rosetta=false
shell=zsh
supported=true
pvm_installed=false
asset=pvm-darwin-arm64
installer=install-pvm.sh

Detected darwin/arm64. Recommended install:
  bash "<SKILL_ROOT>/install-pvm.sh"
Or the one-line remote installer:
  curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
```

退出码：`0` 环境受支持（无论是否已安装），`1` 无对应 Release，`2` 参数错误。已安装时会提示用 `pvm self-update` 升级而不是重装。

---

## 自动安装 PVM

**脚本：** `install-pvm.ps1` / `install-pvm.sh`

自动检测当前 OS 和原生 CPU 架构，并映射到对应 Release 资产。未指定版本时安装最新稳定版；Gitee 下载失败时自动回退 GitHub。

```powershell
# Windows：复制即可执行，自动识别 x64 / ARM64
& "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\install-pvm.ps1"

# 指定版本 / 仅查看将选择的安装包
& "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\install-pvm.ps1" -Version v0.0.1
& "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\install-pvm.ps1" -DetectOnly
```

```bash
# macOS / Linux：Skill 在当前仓库时，自动识别 Intel、Apple Silicon、x64、ARM64
bash .codebuddy/skills/pvm-env-manager/scripts/install-pvm.sh
bash .codebuddy/skills/pvm-env-manager/scripts/install-pvm.sh --version v0.0.1
bash .codebuddy/skills/pvm-env-manager/scripts/install-pvm.sh --detect-only
```

直接使用远程正式安装器：

```powershell
# Windows
irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex
```

```bash
# macOS / Linux
curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
```

探测结果示例：`platform=darwin architecture=arm64 asset=pvm-darwin-arm64`。不支持的 OS/CPU 会停止，不会猜测或下载其他架构。

---

## 软件彻底卸载

**脚本**：`uninstall-tool.bat` / `uninstall-tool.sh`

**用法**：
```bash
# Windows
scripts\uninstall-tool.bat <软件名> --yes

# macOS/Linux
./scripts/uninstall-tool.sh <软件名> --yes
```

**支持 18 种软件：**

| 分类 | 软件 | 说明 |
|------|------|------|
| PVM运行时 | `node` | Node.js |
| | `git` | Git |
| | `python` | Python |
| | `rust` | Rust (rustc + cargo) |
| | `go` | Go |
| | `bun` | Bun |
| | `deno` | Deno |
| | `pnpm` | PNPM |
| | `yarn` | Yarn |
| | `pvm` | Polyglot Version Manager |
| 冲突工具 | `nvm` | Node Version Manager |
| | `volta` | Volta |
| | `fnm` | Fast Node Manager |
| | `nodenv` | nodenv |
| | `pyenv` | pyenv |
| | `rustup` | rustup |
| | `asdf` | asdf |
| | `conda` | Anaconda / Miniconda |

**每个软件的卸载流程：**
1. 终止相关进程
2. 删除安装目录 / 全局目录
3. 清理 PATH 环境变量（用户级 + 系统级）
4. 清理工具专属环境变量（如 NVM_HOME、PYENV_ROOT、GOROOT 等）
5. 清理注册表（Windows） / shell rc（macOS/Linux）

---

## .npmrc 自动配置（pnpm 锁版本）

**脚本**：`setup-npmrc.bat` / `setup-npmrc.sh`

**用法**：
```bash
scripts\setup-npmrc.bat --force     # Windows
./scripts/setup-npmrc.sh --force    # macOS/Linux
```

**配置内容**：
- 国内镜像源（npmmirror）
- 精确版本锁定（`save-prefix=""`，不加 `^` `~`）
- 自动安装 peer 依赖
- lockfile 冻结策略（`prefer-frozen-lockfile=true`）
- 版本解析策略（`resolution-mode=time-based`）

---

## PVM 环境诊断

**脚本**：`diagnose-pvm-env.bat` / `diagnose-pvm-env.sh`

**用法**：
```bash
scripts\diagnose-pvm-env.bat --fix      # Windows
./scripts/diagnose-pvm-env.sh --fix     # macOS/Linux
```

**诊断 8 项**：
1. PVM 安装状态
2. 环境变量（PVM_HOME、PATH）
3. 冲突工具检测
4. PVM 目录结构
5. 运行时安装状态
6. 端口冲突
7. .npmrc 配置
8. Shell 配置文件

---

## Git local 文件冲突处理

**脚本**：`resolve-local-conflict.bat` / `resolve-local-conflict.sh`

```bash
scripts\resolve-local-conflict.bat --remote   # 用远程版本
scripts\resolve-local-conflict.bat --local    # 保留本地版本
```

---

## 从远程替换 local 文件

**脚本**：`replace-local-from-remote.bat` / `replace-local-from-remote.sh`

```bash
scripts\replace-local-from-remote.bat --backup    # 替换前备份
scripts\replace-local-from-remote.bat --list      # 仅列出，不替换
```

---

## pnpm EPERM 权限错误修复

**两种 EPERM 场景，先分清再处理：**

| 场景 | 报错时机 | 根因 | 解法 |
|------|---------|------|------|
| **权限不足**（最常见） | `pvm install pnpm` | IDE 无管理员权限，写入 shim 被拒 | **管理员权限运行 IDE**，无需脚本 |
| **进程占用**（补充） | `pnpm install`（已有管理员权限） | esbuild/rollup/swc 进程占用文件 | 用下方 `fix-pnpm-eperm` 脚本 |

> **先试管理员权限！** 大多数 EPERM 是权限不足导致，管理员权限运行 IDE 即可解决。仅当已有管理员权限仍报 `unlink` 错误时，才用脚本处理进程占用。

**脚本**：`fix-pnpm-eperm.bat` / `fix-pnpm-eperm.sh`

**适用场景**：已有管理员权限，`pnpm install` 仍报 `ERR_PNPM_EPERM` / `EPERM: operation not permitted, unlink '...esbuild.exe'`（进程占用）

**原因**：esbuild.exe / rollup.exe / swc.exe 等原生二进制被进程占用（开发服务器、IDE 索引、杀毒软件），pnpm 无法删除/替换。

**用法**：

```bash
# Windows
scripts\fix-pnpm-eperm.bat --reinstall    # 完整修复：杀进程 + 删 node_modules + pnpm install（默认）
scripts\fix-pnpm-eperm.bat --kill-only    # 仅杀占用进程，不重装
scripts\fix-pnpm-eperm.bat --check        # 仅检测占用进程，不操作

# macOS/Linux
./scripts/fix-pnpm-eperm.sh --reinstall
```

**处理流程（--reinstall 模式）**：

1. 扫描占用进程：`esbuild.exe` `rollup.exe` `swc.exe` `vite.exe` `webpack.exe`
2. `taskkill /F` 终止这些进程，等待 2 秒释放文件句柄
3. 删除 `node_modules`（失败则强制清除只读属性后重试）
4. 运行 `pnpm install` 重新安装依赖

**注意**：

- 脚本**不杀 `node.exe`**，避免误杀 IDE。如开发服务器通过 node 启动（如 `vite`），需手动关闭。
- 如脚本仍失败：关闭 IDE → 添加杀毒软件排除项 → 以管理员身份运行终端。
