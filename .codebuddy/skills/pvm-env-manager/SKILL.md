---
name: pvm-env-manager
description: >
  管理 PVM（多语言版本管理器）的安装与运行时环境：未安装时自动检测操作系统与
  CPU 架构并安装匹配版本；已安装时直接执行对应的 pvm 命令。当用户提到 pvm、
  安装pvm、pvm use、切换版本、需要 node/python/go 等运行时环境、版本管理器、
  环境诊断、EPERM 权限错误等相关问题时使用。不要用于开发 PVM 源码本身、
  git 版本回滚或与运行时版本无关的包依赖问题。
version: 2.0.0
dependencies: []
conflict-with: []
allowed-tools: [read_file, execute_command, search_content]
max-token: 2000
---

# PVM 环境管理器

## 概述

PVM 是多语言运行时版本管理器。本 skill 负责：未装 PVM 时检测环境并安装匹配版本；已装时直接执行 pvm 命令完成用户的运行时需求。所有 pvm 命令均在本项目中（以 `references/pvm-commands.md` 为准），可按需调用。

## 何时使用 / 何时不使用

**使用：**
- 安装、更新、卸载 PVM 本身
- 安装/切换/锁定 node、python、go 等运行时版本
- 任何关于 pvm、环境、版本的问题（先调本 skill，再做其他）

**不使用：**
- 开发 PVM 源码（负向触发示例第 4 条）
- git 操作、Docker 容器环境
- 与运行时版本无关的 npm 包依赖问题

## 🛑 入口守卫

```
用户提出 pvm / 环境 / 版本相关请求 → 必须走 Step 1 → 2 → 3 顺序，禁止跳步。
禁止未检测就安装；禁止已安装还重复安装；禁止凭经验猜架构。
```

## 核心工作流

### Step 1：检测环境与安装状态（必走）

运行 Skill 自带检测脚本，得出唯一结论：

- Windows：`& "<SKILL_ROOT>/scripts/detect-env.ps1"`
- macOS / Linux：`bash "<SKILL_ROOT>/scripts/detect-env.sh"`（Git Bash 下也能用）

输出键值（以此为准，禁止自行推断）：

```text
os=windows|darwin|linux|windows-gitbash
arch=amd64|arm64
rosetta=true|false          # Rosetta 终端已折算为原生 arm64
pvm_installed=true|false
pvm_version=0.0.1           # 已装时输出
asset=pvm-windows-amd64.exe # 匹配的 Release 资产
installer=install-pvm.ps1|.sh
```

退出码：`0` 受支持，`1` 无对应 Release（停止并告知用户，不得猜架构），`2` 参数错误。

播报：`✅ [Step 1] os=X arch=Y pvm_installed=Z`

### Step 2：安装或跳过

按 `pvm_installed` 分流：

- **`pvm_installed=true`** → 跳过安装，直接进 Step 3。升级需求用 `pvm self-update`，不重装。
- **`pvm_installed=false`** → 执行 `<SKILL_ROOT>/scripts/install-pvm.ps1`（Windows）或
  `bash <SKILL_ROOT>/scripts/install-pvm.sh`（macOS/Linux）。用户指定版本时传
  `-Version vX.Y.Z` / `--version vX.Y.Z`。
- 安装脚本自动完成：Gitee 优先回退 GitHub → 下载 `asset` 对应资产 → 校验 → `pvm setup`。
- PATH 未刷新时用绝对路径验证：Windows `& "$env:USERPROFILE/.pvm/bin/pvm.exe" version`；
  Unix `~/.pvm/bin/pvm version`。
- 安装写入用户主目录并修改 PATH/shell profile，遵循客户端权限审批，不默认提权。

播报：`✅ [Step 2] 已安装 vX.Y.Z / 已存在 vX.Y.Z（跳过安装）`

### Step 3：执行用户请求的 pvm 命令

按意图映射命令（完整命令集见 `references/pvm-commands.md`）：

| 用户意图 | 命令 |
|----------|------|
| 需要某运行时 | `pvm install <rt>@<version>` && `pvm use <rt>@<version>` |
| 切版本 | `pvm use <rt>@<version>`（项目级加 `--local` 写 .pvmrc） |
| 查看版本 | `pvm list` / `pvm current` / `pvm list-remote <rt>` |
| 环境问题 | `pvm doctor` → `pvm setup` 修复 |
| 更新 PVM | `pvm self-update` |
| 卸载 | `pvm remove <rt>@<v>` / `pvm uninstall` |

验证：安装/切换后执行 `node --version` 等确认生效；环境类操作后执行 `pvm doctor`。

播报：`✅ [Step 3] <命令> → <关键结果>`

## 场景分流（细节见 references/）

| 场景 | 参考文档 | 何时读 |
|------|----------|--------|
| 安装细节 / 手动下载 / 源码构建 | `references/install-guide.md` | Step 2 安装失败降级时 |
| 从 nvm/pyenv 等迁移 | `references/migration-guide.md` | 用户提"换掉 nvm"时 |
| 全部 pvm 命令 | `references/pvm-commands.md` | Step 3 映射命令时 |
| pnpm 锁版本 / EPERM / lockfile 冲突 | `references/pvm-commands.md` + 场景速查（下） | 对应报错出现时 |

### 场景速查

- **pnpm 锁版本**：`pvm use pnpm@9 --local` → `scripts/setup-npmrc.bat|.sh --force` → 验证 `pnpm --version`
- **pnpm install EPERM（有管理员权限仍报 unlink）**：`scripts/fix-pnpm-eperm.bat|.sh --reinstall`（杀 esbuild/rollup/swc 占用进程，不杀 node.exe）
- **冲突工具清理（先装后卸）**：先 `pvm install node@<版本>` 替代，再 `scripts/uninstall-tool.bat|.sh nvm --yes`，最后 `pvm setup` 修复 PATH
- **lock 文件合并冲突**：`scripts/resolve-lockfile-conflict.bat|.sh`（checkout --theirs + `pvm install --lockfile-only` 重新生成）
- **环境异常排查**：`scripts/diagnose-pvm-env.bat|.sh --fix`

## 中断与恢复机制

- **中断点**：任一 Step 失败或用户审批被拒时，记录当前 Step 编号和检测输出，停止后续步骤。
- **恢复**：从中断的 Step 重新开始，不重复已完成的 Step。Step 1 的检测输出可复用（同一会话内环境未变时）。
- **重装恢复**：`pvm version` 可正常输出即视为已安装，直接进入 Step 3。
- **半安装状态**：`pvm_installed=false` 但 `~/.pvm/bin/pvm` 存在 → 运行 `pvm setup` 补全配置，不重新下载。

## 输出格式

```text
✅ [Step 1] os=darwin arch=arm64 pvm_installed=false
✅ [Step 2] 已安装 v0.0.1 (pvm-darwin-arm64)
✅ [Step 3] pvm install node@20.11.0 → node v20.11.0 已生效
```

失败时：

```text
⛔ [Step N] <失败原因> → <恢复建议>
```

## 错误处理

| 错误 | 处理 |
|------|------|
| detect-env 退出码 1（无对应 Release） | 停止，告知用户当前 OS/CPU 无匹配版本，不猜架构 |
| 安装脚本失败 | 按 `references/install-guide.md` 降级：手动 Release → 源码构建 |
| `pvm: command not found`（刚装完） | 重启终端/编辑器，或用绝对路径 `~/.pvm/bin/pvm` |
| `pvm use` 无效 | `pvm doctor` → `pvm setup` 修复 PATH |
| Windows 无 Git Bash（verify 命令失败） | 手动运行 `scripts/detect-env.ps1`，或安装 Git Bash 后重试 |
| pvm.exe 被占用 | 提示关闭 PVM 进程后重试，不擅自杀无关进程 |

## 验证清单

- [ ] Step 1 检测输出来自 detect-env 脚本（非模型推断）
- [ ] `pvm_installed=true` 时未重复安装
- [ ] 安装资产与检测的 os/arch 一致
- [ ] 安装后 `pvm version` 正常输出
- [ ] 用户意图映射为正确 pvm 命令
- [ ] 执行结果已播报，安装/切换后验证生效

## 参考资料

- `references/pvm-commands.md` — 全部 pvm 命令
- `references/install-guide.md` — 安装细节与降级方案
- `references/migration-guide.md` — 版本管理器迁移
- `references/floor-ceiling-guide.md` — 脚本/模型分工判定表
- `scripts/README.md` — 脚本用法
