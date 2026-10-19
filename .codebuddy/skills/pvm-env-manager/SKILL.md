---
name: pvm-env-manager
description: This skill should be used when users ask to install, update, configure, diagnose, migrate to, or manage runtimes with PVM. For PVM installation, it detects the current operating system and native CPU architecture, selects the matching stable Release, installs it at user scope, configures PATH, and verifies the result.
---

# PVM 环境管理器（移动手册）

## 定位

用户提出安装或使用 PVM 时触发。安装时自动检测当前机器，不询问用户选择 Windows/macOS/Linux、Intel/Apple Silicon 或 x64/ARM64；有明确版本就安装该版本，否则安装最新稳定版。

## 执行原则

1. **直接执行** — 检测到什么就跑什么命令，不等用户确认细节
2. **少废话** — 不解释原理，不输出大段说明，只报关键结果
3. **一步到位** — 能一条命令解决的不拆成两步
4. **失败降级** — A 方式不行换 B，B 不行换 C，不卡死
5. **脚本优先** — 冲突清理、卸载等操作有现成脚本，直接调，不手写

## 内置命令速查（不查文档）

```
# 安装与切换
pvm install <runtime>@<version>     # 安装运行时（默认用户级）
pvm use <runtime>@<version>         # 切换（默认用户级 --user）
pvm use <runtime>@<version> --local # 锁定到项目（写 .pvmrc）
pvm use                              # 按 .pvmrc 自动切
pvm use --system <runtime>          # 使用系统版本
pvm init                             # 创建 .pvmrc（当前目录）
pvm init --local                     # 同上（显式项目级）

# 查询
pvm list                            # 已装运行时
pvm list <runtime>                  # 某运行时的已装版本
pvm list-remote <runtime>           # 远程可用版本
pvm current                         # 当前激活版本
pvm which <command>                 # 可执行文件真实路径
pvm where <runtime>                 # 运行时安装目录
pvm doctor                          # 环境健康检查（7项）
pvm validate [--auto-fix]           # 深度验证 shim + 版本一致性
pvm diagnostics <runtime>           # 单个运行时详细诊断
pvm --version                       # PVM 版本

# 配置管理（.pvmrc 操作）
pvm config init                     # 从当前活跃版本创建 .pvmrc
pvm config show                     # 显示 .pvmrc 内容
pvm config set <runtime>@<version>  # 设置/更新版本
pvm config remove <runtime>         # 移除某运行时

# 维护
pvm setup                           # 首次设置（目录+PATH+shims）
pvm setup-path                      # 检查/修复 PATH 配置
pvm reshim                          # 重建 shim
pvm self-update                     # 更新 PVM
pvm remove <runtime>@<version>      # 卸载某个已装版本
pvm uninstall                       # 卸载 PVM 自身

# 全局标志
--user, -u                          # 用户级操作 [默认]
--local, -l                         # 项目级操作（写 .pvmrc）
--system, -s                        # 使用系统安装版本
--mirror, -m                        # 下载用国内镜像
--official, -o                      # 下载用官方源
--force, -f                         # 强制重装
--verbose, -V                       # 详细输出
--quiet, -q                         # 静默模式

# 支持的运行时（9种）
node / python / rust / go / bun / deno / git / pnpm / yarn

# 全局唯一运行时（不支持项目级 .pvmrc）
go / git / rust（只能 --user 或 --system）
```

## 脚本速查（冲突清理直接调）

```
脚本目录: .codebuddy/skills/pvm-env-manager/scripts/

# 自动检测机器并安装 PVM
当前 Windows 仓库: & "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\install-pvm.ps1"
Windows 通用:      & "<SKILL_ROOT>\scripts\install-pvm.ps1" [-Version vX.Y.Z] [-Source auto|gitee|github]
macOS/Linux:       bash "<SKILL_ROOT>/scripts/install-pvm.sh" [--version vX.Y.Z] [--source auto|gitee|github]
远程 Windows:      irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex
远程 macOS/Linux:  curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
探测:              Windows 加 -DetectOnly；macOS/Linux 加 --detect-only

# 软件卸载（16种：PVM运行时 + 冲突工具）
Windows: scripts\uninstall-tool.bat <名称> --yes
Unix:    ./scripts/uninstall-tool.sh <名称> --yes

# pnpm .npmrc 配置（锁版本 + 国内镜像）
Windows: scripts\setup-npmrc.bat --force
Unix:    ./scripts/setup-npmrc.sh --force

# 环境诊断
Windows: scripts\diagnose-pvm-env.bat --fix
Unix:    ./scripts/diagnose-pvm-env.sh --fix

# pnpm EPERM 权限错误修复（esbuild/rollup/swc 进程占用）
Windows: scripts\fix-pnpm-eperm.bat --reinstall   # 杀进程+删node_modules+重装
         scripts\fix-pnpm-eperm.bat --kill-only   # 仅杀占用进程
         scripts\fix-pnpm-eperm.bat --check       # 仅检测
Unix:    ./scripts/fix-pnpm-eperm.sh --reinstall
```

---

## 场景执行流

### 场景 1：安装 PVM

**目标：** 用户只需说“安装 PVM”。自动检测当前操作系统、原生 CPU 架构和最新稳定版本，不让用户手动选择资产。

#### 固定执行流程

1. **检查是否已经安装。**
   - Windows：检查 `Get-Command pvm`，再检查 `${env:PVM_HOME}\bin\pvm.exe` 和 `$env:USERPROFILE\.pvm\bin\pvm.exe`。
   - macOS/Linux：检查 `command -v pvm`，再检查 `${PVM_HOME:-$HOME/.pvm}/bin/pvm`。
   - 找到后执行绝对路径的 `version`。版本可正常输出时不重复安装；用户明确要求指定版本或修复安装时除外。

2. **解析 Skill 根目录。** 使用当前已加载 Skill 的绝对目录作为 `<SKILL_ROOT>`，不得假设当前工作目录就是仓库根目录。

3. **自动检测并安装。**
   - Windows：执行 `<SKILL_ROOT>\scripts\install-pvm.ps1`。
   - macOS/Linux：用 Bash 执行 `<SKILL_ROOT>/scripts/install-pvm.sh`。
   - 脚本会检测平台和架构、解析最新稳定 Release、先尝试 Gitee 再回退 GitHub、下载对应资产、验证并执行 `pvm setup`。
   - 用户指定版本时传 `-Version vX.Y.Z` 或 `--version vX.Y.Z`；未指定时不传版本参数。
   - 安装会写入用户主目录并修改 PATH/shell profile。Windows 正式安装器使用 `pvm setup --no-elevate`，机器级 PATH 冲突只报告；调用执行工具时遵循客户端权限审批，不绕过审批，也不默认申请管理员/root 权限。

4. **验证安装。** 新终端 PATH 尚未刷新时，计算实际 `PVM_HOME`、临时前置 PATH，并使用带引号的绝对路径：

   Windows：
   ```powershell
   $pvmHome = if ($env:PVM_HOME) { [Environment]::ExpandEnvironmentVariables($env:PVM_HOME) } else { Join-Path $env:USERPROFILE '.pvm' }
   $pvmPath = Join-Path $pvmHome 'bin\pvm.exe'
   $env:PVM_HOME = $pvmHome
   $env:Path = "$(Join-Path $pvmHome 'shims');$(Join-Path $pvmHome 'bin');$env:Path"
   & $pvmPath version
   & $pvmPath doctor
   ```

   macOS/Linux：
   ```bash
   pvm_home="${PVM_HOME:-$HOME/.pvm}"
   PVM_HOME="$pvm_home" PATH="$pvm_home/shims:$pvm_home/bin:$PATH" "$pvm_home/bin/pvm" version
   PVM_HOME="$pvm_home" PATH="$pvm_home/shims:$pvm_home/bin:$PATH" "$pvm_home/bin/pvm" doctor
   ```

   `doctor` 发现冲突时只报告；安装 PVM 的请求不等于授权卸载其他软件。

5. **失败降级。** 自动脚本失败后读取 `references/install-guide.md`。仅在对应平台没有 Release 且本机已有兼容 Go 工具链时考虑源码构建；不得猜测资产、下载其他架构或把裸二进制当作双击安装包。

#### 自动映射

| 当前机器 | Release 资产 |
|---|---|
| Windows x64 | `pvm-windows-amd64.exe` |
| Windows ARM64 | `pvm-windows-arm64.exe` |
| macOS Intel | `pvm-darwin-amd64` |
| macOS Apple Silicon | `pvm-darwin-arm64` |
| macOS Apple Silicon 的 Rosetta 终端 | `pvm-darwin-arm64` |
| Linux x64 | `pvm-linux-amd64` |
| Linux ARM64 | `pvm-linux-arm64` |

不支持的 OS/CPU 必须停止并说明没有对应 Release，不能擅自选择最接近的架构。

**特殊情况：**

- **Windows 文件被占用** → 提示关闭正在运行的 PVM 进程后重试，不擅自终止无关进程。
- **编辑器终端不生效** → 完全重启编辑器（不是 reload）。
- **macOS Gatekeeper 拦截** → 先核验下载来源和校验值，再提示到“隐私与安全性”中允许；不得未经说明移除 quarantine。
- **杀软拦截** → 先核验来源，不自动添加安全软件排除项。

---

### 场景 2：安装/切换运行时

```
需求？
├─ "我需要 python 环境" → pvm install python@latest && pvm use python@latest
├─ "切到 node 20"       → pvm install node@20 && pvm use node@20
├─ "这个项目用指定版本" → 见场景 5（.pvmrc）
└─ 其他 Skill 请求      → 问版本（默认 latest）→ install + use

验证 → node --version / python --version / pvm current
```

---

### 场景 3：pnpm 版本锁定（解决依赖问题）

**目标：** 项目内 pnpm 版本固定 + lockfile 冻结 + 依赖不漂移。

```
Step 1: pvm 管理 pnpm 版本
  pvm install pnpm@9
  pvm use pnpm@9 --local          # 写入 .pvmrc，项目级锁定

Step 2: 配置 .npmrc（锁版本策略）
  Windows: .codebuddy\skills\pvm-env-manager\scripts\setup-npmrc.bat --force
  Unix:    ./.codebuddy/skills/pvm-env-manager/scripts/setup-npmrc.sh --force

  → 自动配置：
    - registry=npmmirror（国内镜像）
    - save-prefix=""（精确版本，不加 ^ ~）
    - prefer-frozen-lockfile=true（优先用现有 lockfile）
    - auto-install-peers=true（自动装 peer 依赖）

Step 3: 验证
  pnpm --version                   # 应为 .pvmrc 指定版本
  cat .pvmrc                       # 应有 pnpm = 9
  cat ~/.npmrc                     # 应有锁版本配置

Step 4: 提交到仓库
  git add .pvmrc .npmrc
  git commit -m "lock pnpm version and deps"
```

**.pvmrc 最终效果：**

```ini
node = 20.11.0
pnpm = 9
```

**CI 环境额外加：**

```bash
pnpm install --frozen-lockfile   # CI 严格按 lockfile，不更新
```

**pnpm 安装报错（权限问题）—— 最常见：**

`pvm install pnpm` 报 `ERR_PNPM_EPERM` / `EPERM: operation not permitted` / `EACCES`，**根因是 IDE 没有管理员权限**，pvm 写入 shim/全局目录被系统拒绝。

**解决（一步到位）：** 用管理员权限重新打开 IDE：

```
右键 IDE 图标 → 以管理员身份运行 → pvm install pnpm@9 && pvm use pnpm@9
```

> 这是 PVM 安装 pnpm 的典型问题，管理员权限即可解决，无需其他操作。

---

### 场景 3B：pnpm install EPERM 权限错误修复（补充方案）

**触发：** 已有管理员权限，但 `pnpm install` 仍报 `ERR_PNPM_EPERM` / `EPERM: operation not permitted, unlink '...esbuild.exe'`

**原因：** esbuild.exe / rollup.exe / swc.exe 等原生二进制文件被进程占用（开发服务器 vite/webpack 在运行、IDE 索引服务、杀毒软件扫描），pnpm 无法删除/替换。

**首选排查（先试这步）：**

```
1. 确认已用管理员权限运行 IDE（见场景 3 权限问题）
2. 如仍报错 → 进程占用，用下方脚本处理
```

**自动处理流程（进程占用场景）：**

```
1. 扫描占用进程（esbuild.exe, rollup.exe, swc.exe, vite.exe, webpack.exe）
2. taskkill /F 终止这些进程
3. 删除 node_modules（含强制删除只读文件）
4. pnpm install 重新安装
```

**调用方式：**

```
# 完整修复（杀进程 + 删 node_modules + 重装）— 默认
Windows: .codebuddy\skills\pvm-env-manager\scripts\fix-pnpm-eperm.bat --reinstall
Unix:    ./.codebuddy/skills/pvm-env-manager/scripts/fix-pnpm-eperm.sh --reinstall

# 仅杀占用进程（不想删 node_modules 时用）
Windows: scripts\fix-pnpm-eperm.bat --kill-only

# 仅检测占用进程（不操作）
Windows: scripts\fix-pnpm-eperm.bat --check
```

**手动处理（脚本无效时）：**

```
1. 关闭开发服务器（Ctrl+C 停止 vite dev / npm run dev）
2. 完全关闭 IDE（不是 reload，是退出）
3. 终端手动执行：
   taskkill /F /IM esbuild.exe
   rmdir /s /q node_modules
   pnpm install
4. 如仍失败 → 杀毒软件添加项目目录到排除项
5. 如仍失败 → 以管理员身份运行终端重试
```

**注意：** 脚本只杀 esbuild/rollup/swc/vite/webpack 进程，**不杀 node.exe**（避免误杀 IDE）。如开发服务器是 node 进程（如 `vite` 通过 node 启动），需手动关闭。

---

### 场景 4：冲突工具清理

**铁律：先装后卸。卸 nvm 前先 `pvm install node`，卸 pyenv 前先 `pvm install python`。**

```
检测冲突 → pvm doctor（或手动 where.exe nvm / which nvm）
  ├─ 无冲突 → 跳过
  └─ 有冲突 →
      1. 先装替代品：
         nvm  → pvm install node@<当前版本> && pvm use node@<版本>
         pyenv → pvm install python@<版本> && pvm use python@<版本>
      2. 验证替代品：node --version / python --version
      3. 跑卸载脚本（直接调，不解释）：
         Windows: scripts\uninstall-tool.bat nvm --yes
         Unix:    ./scripts/uninstall-tool.sh nvm --yes
      4. 修复 PATH：pvm setup
      5. 验证：pvm doctor
```

**支持的卸载目标（18种）：**

- PVM运行时：`node` `git` `python` `rust` `go` `bun` `deno` `pnpm` `yarn` `pvm`
- 冲突工具：`nvm` `volta` `fnm` `nodenv` `pyenv` `rustup` `asdf` `conda`

---

### 场景 4B：lock 文件合并冲突

**触发：** `pnpm-lock.yaml` / `package-lock.json` / `yarn.lock` 在 git merge/rebase 时出现冲突标记（`<<<<<<<` / `>>>>>>>`）。

**为什么不能手动合并：** lock 文件是包管理器根据 package.json 自动生成的，包含了依赖树、完整性校验 hash、peer dependency 解析结果等。手动编辑极易导致：
- 依赖版本错乱
- 完整性校验失败
- pnpm install 后 lockfile 再次漂移

**自动处理流程：**

```
1. 脚本扫描所有冲突文件，识别出 lock 文件
2. git checkout --theirs   （用远程版本清除冲突标记）
3. pnpm install --lockfile-only   （基于本地 package.json 重新生成）
4. git add pnpm-lock.yaml   （暂存正确版本）

调用方式：
  Windows: .codebuddy\skills\pvm-env-manager\scripts\resolve-lockfile-conflict.bat
  Unix:    ./.codebuddy/skills/pvm-env-manager/scripts/resolve-lockfile-conflict.sh

选项：
  --regenerate（默认）：清除冲突 + pnpm install 重新生成 + git add
  --no-regenerate     ：仅清除冲突标记，不重新生成
```

---

### 场景 5：项目环境隔离

```
1. 进项目目录
2. pvm use node@20 --local        # 自动写 .pvmrc
   pvm use python@3.12 --local
   pvm use pnpm@9 --local
3. 验证 → cat .pvmrc
4. 提交 → git add .pvmrc && git commit -m "lock runtimes"
```

**.pvmrc 格式：**

```ini
node = 20.11.0
python = 3.12.0
pnpm = 9
```

进目录自动生效，切到别的项目自动换。不用记，不用手动 `use`。

---

### 场景 6：更新与卸载

```
更新 PVM → pvm self-update
卸载运行时 → pvm remove node@18
卸载 PVM → pvm uninstall
彻底清理 → scripts\uninstall-tool.bat pvm --yes
```

---

## 错误速查

| 现象 | 动作 |
|------|------|
| `pvm: command not found` | 重启终端 / 重启编辑器 / `pvm setup` |
| 安装脚本 404 | 换 Releases 下载（方式 B）或源码编译（方式 C） |
| `pvm use` 无效 | `pvm doctor` → `pvm setup` 修复 PATH |
| pvm.exe 被占用 | 杀进程 + 复制重命名替换 |
| 杀软拦截 | 「仍要运行」或加排除项 |
| 下载慢 | PVM 自动切国内镜像，无需配置 |
| 版本不存在 | `pvm list <runtime>` 看可用版本 |
| `ERR_PNPM_EPERM` / `EPERM` / `EACCES`（pvm install pnpm） | **管理员权限运行 IDE**（场景 3 权限问题，最常见） |
| `ERR_PNPM_EPERM` unlink（pnpm install，已有管理员权限） | 场景 3B：`fix-pnpm-eperm.bat --reinstall`（进程占用） |

---

## 脚本职责（AI 只需知道何时调，不需解释内部逻辑）

| 脚本 | 何时调 | 干什么 |
|------|--------|--------|
| `install-pvm.ps1/.sh` | 用户要求安装 PVM | 检测 OS/CPU → 选择匹配 Release → 调用正式安装器 → setup |
| `uninstall-tool.bat/.sh` | 冲突工具清理 / 彻底卸载 | 16种软件：杀进程→清PATH→删目录→清注册表 |
| `setup-npmrc.bat/.sh` | pnpm 版本锁定场景 | 配置 .npmrc 锁版本策略 |
| `diagnose-pvm-env.bat/.sh` | 环境异常排查 | 8 项诊断 + 可选自动修复 |
| `resolve-lockfile-conflict.bat/.sh` | lock 文件合并冲突 | 清除冲突标记 → pnpm install 重新生成 → git add |
| `replace-local-from-remote.bat/.sh` | local 配置文件同步远程 | 从远程拉取替换本地 |
| `fix-pnpm-eperm.bat/.sh` | pnpm install EPERM 权限错误 | 杀占用进程 → 删 node_modules → pnpm install |
