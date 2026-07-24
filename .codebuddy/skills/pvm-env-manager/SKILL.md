# PVM 环境管理器

## 概述

检测、安装、管理 PVM（Polyglot Version Manager）多语言版本管理器，为项目和 AI Skill 提供运行时环境隔离。当其他 Skill 需要 Python/Node/Go 等运行时，或用户需要安装/卸载/迁移版本管理工具时使用。

## 何时使用 / 何时不使用

**使用：**
- 用户说"安装 pvm""需要 python 环境""需要 node 环境""切换版本""环境隔离"
- 其他 Skill 执行前需要运行时环境（python、node、go、rust 等）
- 用户要从 nvm/fnm/pyenv 迁移到 PVM
- 用户要卸载、更新 PVM 或其管理的运行时

**不使用：**
- Git 版本管理（那是 git，不是 pvm）
- Docker 容器环境管理
- 操作系统级别的包管理（apt、brew、choco）
- 直接开发 PVM 源码

## 🛑 入口守卫

```
本 Skill 被 use_skill 加载后，必须按顺序执行 Step 0→1→2→... 全部步骤。
每步必须播报对应完成消息。未播报的步骤 = 未执行 = 违规。

禁止行为：
  ✗ 跳过 Step 0 直接执行核心逻辑
  ✗ 不播报完成消息（无日志 = 没走流程）
  ✗ 无声跳过任何步骤

唯一例外：同会话已执行并播报过的步骤可跳过，但必须声明：
  ⏭️ [跳过] Step N — 原因 | 风险
```

## 核心工作流

### Step 0：规则注入（🔒 强制 · 最高优先级 · 不可跳过）

```
需要注入的规则内容是什么？
├── 需要 alwaysApply: true？（PVM 环境管理核心约束，跨会话持久）
│   └── → use_skill("specflow-rules-engine")
│       1. 同步 RULE.mdc 到 .codebuddy/rules/pvm-env-manager（alwaysApply 生效）
│       2. 检测 System Prompt 是否已注入
│       3. 未注入 → read_file 兜底（强制 · 最后防线）
```

**播报完成消息（必须逐字输出）：**

```
✅ [Step 0] 规则注入完成
  ▸ 注入方式：{specflow-rules-engine / read_file 兜底}
  ▸ alwaysApply: true
  ▸ 规则内容已就绪
```

### Step 1：环境检测（🔒 强制 · 低自由度）

[低] 检测 PVM 是否已安装、当前操作系统、已安装的运行时、冲突工具。

执行：

1. 检测操作系统：
   ```bash
   # macOS/Linux
   uname -s
   # Windows
   $env:OS
   ```

2. 检测 PVM 是否已安装：
   ```bash
   # macOS/Linux
   command -v pvm && pvm --version
   # Windows
   where.exe pvm; pvm --version
   ```

3. 如 PVM 已安装，检测已安装的运行时：
   ```bash
   pvm list
   pvm current
   ```

4. 检测冲突工具（nvm / fnm / pyenv / rustup / volta / asdf）：
   ```bash
   # macOS/Linux
   for tool in nvm fnm pyenv rustup volta asdf; do command -v $tool 2>/dev/null && echo "$tool: found"; done
   # Windows
   foreach ($tool in @('nvm','fnm','pyenv','rustup','volta','asdf')) { $r = Get-Command $tool -ErrorAction SilentlyContinue; if ($r) { Write-Host "$tool: found at $($r.Source)" } }
   ```

5. 检测结果分类：
   ```
   PVM 状态？
   ├── 已安装 → 播报版本 + 已装运行时，进入 Step 3（按需操作）
   └── 未安装 → 播报，进入 Step 2（安装 PVM）

   冲突工具？
   ├── 无冲突 → 继续
   └── 有冲突 → 记录工具名 + 路径，后续 Step 4 处理
   ```

**播报完成消息：**
```
✅ [Step 1] 环境检测完成
  ▸ 操作系统：{OS}
  ▸ PVM 状态：{已安装 vX.X.X / 未安装}
  ▸ 已装运行时：{列表 或 无}
  ▸ 冲突工具：{nvm@路径 / 无}
```

### Step 2：安装 PVM（仅在 Step 1 检测到未安装时执行 · 低自由度）

[低] 按**多级 fallback 策略**安装，从最可靠到需手动操作依次尝试。

#### 安装优先级（按顺序尝试，成功即停）

```
方式 A：官方一键脚本（推荐）
  ↓ 失败（404 / 网络超时）
方式 B：GitHub Releases 手动下载
  ↓ 失败（无 Release / API 限流）
方式 C：源码自建（最后手段）
```

#### 方式 A：官方一键脚本

1. 先探测脚本是否可达：
   ```powershell
   # Windows — 探测
   try { $r = Invoke-RestMethod -Uri "https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.ps1" -Method Head; Write-Host "script reachable" } catch { Write-Host "script unreachable: $_" }

   # macOS/Linux — 探测
   curl -fsSLI "https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.sh" | head -1
   ```

2. 探测成功 → 执行安装：
   ```powershell
   # Windows
   iwr -useb https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.ps1 | iex

   # macOS/Linux
   curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.sh | bash
   ```

3. 探测失败（404 / 超时）→ **跳转到方式 B**

#### 方式 B：GitHub Releases 下载

1. 打开 Releases 页面让用户下载：
   ```
   浏览器打开：https://github.com/Lycorxy/pvm/releases
   ```

2. 根据系统选择文件：
   | 系统 | 文件名 |
   |------|--------|
   | Windows (x64) | `pvm-windows-amd64.exe` 或 `.msi` |
   | Windows (ARM) | `pvm-windows-arm64.exe` 或 `.msi` |
   | macOS (Intel) | `pvm-darwin-amd64` |
   | macOS (Apple Silicon) | `pvm-darwin-arm64` |
   | Linux (x64) | `pvm-linux-amd64` |
   | Linux (ARM) | `pvm-linux-arm64` |

3. 下载后手动放到 `~/.pvm/bin/`（Windows 是 `%USERPROFILE%\.pvm\bin\`），重命名为 `pvm.exe`（Windows）或 `pvm`（Unix）

4. 运行 `pvm setup` 完成初始化

5. 无 Release 可下载 → **跳转到方式 C**

#### 方式 C：源码自建（Go 环境）

```bash
# 1. 克隆仓库
git clone https://github.com/Lycorxy/pvm.git /tmp/pvm && cd /tmp/pvm

# 2. 编译
go build -o pvm.exe .    # Windows
go build -o pvm .        # macOS/Linux

# 3. 安装
mkdir -p ~/.pvm/bin
cp pvm ~/.pvm/bin/
./pvm setup
```

> ⚠️ 方式 C 需要 Go 1.22+ 编译环境。如无 Go 环境，告知用户先安装 Go 或等待作者发布 Release。

#### 安装后通用验证

```bash
pvm --version          # 应输出版本号
pvm doctor             # 应全部通过
```

#### 特殊情况处理

**情况 1：pvm.exe 被进程占用无法覆盖**
```powershell
# Windows — 杀进程后用新文件替换再重命名
Get-Process -Name "pvm" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500
Copy-Item pvm-new.exe "$env:USERPROFILE\.pvm\bin\pvm-new.exe" -Force
Remove-Item "$env:USERPROFILE\.pvm\bin\pvm.exe" -Force -ErrorAction SilentlyContinue
Rename-Item "$env:USERPROFILE\.pvm\bin\pvm-new.exe" "pvm.exe" -Force
```

**情况 2：杀毒软件误报（Windows）**
- 安装被拦截 → 告知用户这是误报
- 指引：点击「更多信息」→「仍要运行」
- 或在 Windows Defender 中添加 `~/.pvm` 到排除项：
  ```
  设置 → Windows 安全中心 → 病毒和威胁防护 → 管理设置 → 排除项
  → 添加文件夹 → %USERPROFILE%\.pvm
  ```

**情况 3：VS Code / 编辑器终端里 pvm 不生效**
- 编辑器启动时会缓存环境变量
- 必须**完全重启编辑器**（不是 reload window）
- VS Code：File → Exit → 重新打开
- 重开后在新终端里验证 `pvm -v`

**播报完成消息：**
```
✅ [Step 2] PVM 安装完成
  ▸ 版本：{vX.X.X}
  ▸ 安装路径：{~/.pvm}
  ▸ 安装方式：{A一键脚本 / B Releases下载 / C源码自建}
  ▸ PATH 已更新（需重启终端/编辑器生效）
```

### Step 3：运行时安装与切换（按需执行 · 中自由度）

[中] 根据用户需求或其他 Skill 的请求，安装并切换运行时版本。

决策树：
```
需求类型？
├── 项目级（有 .pvmrc 或用户指定项目目录）
│   └── → 读取 .pvmrc，逐个 `pvm install` + `pvm use --local`
├── 用户级（全局切换）
│   └── → `pvm install <runtime>@<version>` + `pvm use <runtime>@<version>`
└── 其他 Skill 请求（"我需要 python 环境"）
    └── → 询问版本偏好（默认 latest），安装 + 切换
```

执行：

1. 安装运行时：
   ```bash
   pvm install node@20        # 指定版本
   pvm install python@latest  # 最新版
   ```

2. 切换版本：
   ```bash
   # 项目级（推荐，写入 .pvmrc）
   pvm use node@20 --local
   pvm use python@3.12 --local

   # 用户级（全局生效）
   pvm use node@20
   ```

3. 验证切换结果：
   ```bash
   node --version
   python --version
   pvm current
   ```

4. 版本未安装时的处理：
   - 先 `pvm list <runtime>` 查看可用版本
   - 询问用户选择版本
   - `pvm install <runtime>@<version>` 安装
   - `pvm use <runtime>@<version>` 切换

**播报完成消息：**
```
✅ [Step 3] 运行时就位
  ▸ {runtime} = {version}（项目级 / 用户级）
  ▸ 验证：{node -v → v20.x.x}
```

### Step 4：冲突工具迁移（检测到 nvm/fnm/pyenv 等时执行 · 低自由度）

[低] 在卸载任何冲突工具前，必须先用 PVM 安装好对应替代品。严格按顺序执行。

铁律：**先装后卸，不装不卸。**

决策树：
```
冲突工具类型？
├── nvm / fnm（管 Node）
│   ├── 1. pvm install node@<用户当前 nvm 用的版本>
│   ├── 2. pvm use node@<version>
│   ├── 3. 验证 node --version 正确
│   ├── 4. 卸载 nvm/fnm（见下方脚本）
│   └── 5. 运行 pvm setup 修复 PATH
├── pyenv（管 Python）
│   ├── 1. pvm install python@<版本>
│   ├── 2. pvm use python@<version>
│   ├── 3. 验证 python --version 正确
│   └── 4. 卸载 pyenv
├── rustup（管 Rust）
│   ├── 1. pvm install rust@<版本>
│   ├── 2. pvm use rust@<version>
│   └── 3. 卸载 rustup
└── 多个冲突
    └── 逐个处理，每个都先装替代品再卸载
```

#### nvm 卸载脚本（Windows）

确认 PVM 已安装 node 后，运行 Node.js 脚本卸载 nvm：

```bash
node .codebuddy/skills/pvm-env-manager/scripts/uninstall_nvm.js
```

脚本职责（详见 `scripts/uninstall_nvm.js`）：
1. 终止 nvm 相关进程
2. 检测 nvm 安装路径
3. 从用户 PATH 和系统 PATH 移除 nvm 相关条目
4. 清除 NVM_HOME、NVM_SYMLINK 环境变量
5. 删除 nvm 安装目录
6. 清理注册表残留（Windows）
7. 输出清理报告

#### fnm 卸载（macOS/Linux）

```bash
# macOS (brew)
brew uninstall fnm
# 或手动删除
rm -rf ~/.fnm
# 清理 shell rc 中的 fnm 初始化行
```

#### pyenv 卸载

```bash
rm -rf ~/.pyenv
# 清理 shell rc 中的 pyenv 初始化行
```

**播报完成消息：**
```
✅ [Step 4] 冲突工具迁移完成
  ▸ 已卸载：{nvm / fnm / pyenv}
  ▸ PVM 替代：{node@20.x.x 已就位}
  ▸ PATH 已清理，运行 pvm setup 确认
```

### Step 5：项目环境隔离（用户需要项目级版本锁定时 · 中自由度）

[中] 创建或更新 .pvmrc，实现项目间版本隔离。

执行：

1. 检测当前目录是否已有 `.pvmrc`：
   ```bash
   cat .pvmrc 2>/dev/null || echo "not found"
   ```

2. 如已有，读取现有配置并展示：
   ```ini
   node = 20.11.0
   python = 3.12.0
   ```

3. 如需创建/更新，使用 `pvm use --local`：
   ```bash
   pvm use node@20 --local
   pvm use python@3.12 --local
   ```

4. 提示用户提交到 Git：
   ```bash
   git add .pvmrc
   git commit -m "lock runtimes"
   ```

5. 验证隔离生效：
   ```bash
   cd /other/project && node --version   # 应为该项目的版本
   cd /this/project && node --version    # 应为 .pvmrc 指定的版本
   ```

**播报完成消息：**
```
✅ [Step 5] 项目环境隔离已配置
  ▸ .pvmrc：{node=20.11.0, python=3.12.0}
  ▸ 进目录自动生效，无需手动切换
  ▸ 记得 git add .pvmrc 提交到仓库
```

### Step 6：更新与卸载（用户请求时执行 · 低自由度）

[低] PVM 自身更新或卸载，使用确切命令。

#### 更新 PVM

```bash
pvm self-update
```

#### 卸载 PVM

```bash
# Windows
pvm uninstall --yes
# 或手动运行卸载脚本
%~dp0\scripts\uninstall-pvm.bat

# macOS/Linux
pvm uninstall --yes
# 手动清理
rm -rf ~/.pvm
# 清理 shell rc 中的 PVM 相关行
```

#### 卸载单个运行时

```bash
pvm remove node@18
```

**播报完成消息：**
```
✅ [Step 6] {更新/卸载} 完成
  ▸ 操作：{self-update / uninstall / remove}
  ▸ 结果：{已更新到 vX.X.X / 已完全卸载 / 已移除 node@18}
```

## 中断与恢复机制

若执行过程被中断，重新继续时必须：

1. 扫描已有输出内容，检查已完成的步骤
2. 播报恢复点：
   ```
   ⏯️ [恢复] 上次进度：X/6 步完成
   已完成：✅ Step 0 — 规则注入
   待完成：⬜ Step 1 — 环境检测
   断点：从 Step 1 继续
   ```
3. 从断点继续，不重复不遗漏

## 输出格式

灵活型：根据操作类型自适应输出，但必须包含：
- 操作前：检测状态摘要
- 操作中：执行的命令 + 关键输出
- 操作后：验证结果 + 下一步建议

## 错误处理

| 错误场景 | 恢复操作 |
|---------|---------|
| 安装脚本 404 | 跳转到方式 B（Releases 下载）或方式 C（源码自建） |
| Releases 无版本 | 跳转到方式 C（源码自建），或等作者发布 Release |
| GitHub API 限流 | 直接打开浏览器访问 releases 页面手动下载 |
| PVM 命令找不到 | 重启终端 / 重启编辑器 / `source ~/.bashrc` / 检查 PATH |
| pvm.exe 被占用 | 杀进程后用复制+重命名方式替换（见 Step 2 特殊情况 1） |
| `pvm use` 无效 | 运行 `pvm doctor` 诊断，运行 `pvm setup` 修复 PATH |
| 杀软拦截（Windows） | 告知误报，指引「仍要运行」，或添加排除项 |
| nvm 卸载失败 | 检查进程占用，taskkill 后重试，或提示手动删除 |
| 运行时下载慢 | PVM 自动切国内镜像，无需手动配置 |
| 版本不存在 | `pvm list <runtime>` 查看可用版本，提示用户选择 |
| PATH 冲突 | `pvm setup` 自动修复，或管理员权限下修复系统 PATH |

## 验证清单

- [ ] PVM 是否已正确安装（`pvm --version` 有输出）
- [ ] PATH 中 `~/.pvm/shims` 排在最前（`pvm doctor` 无警告）
- [ ] 目标运行时已安装（`pvm list` 包含目标版本）
- [ ] 版本切换生效（`node --version` / `python --version` 输出正确）
- [ ] .pvmrc 已提交（如需项目级隔离）
- [ ] 冲突工具已清理（`where nvm` / `which nvm` 无输出）
- [ ] 无残留进程占用（`tasklist | findstr nvm` 无结果）

## 参考资料

- [PVM 命令参考](references/pvm-commands.md) — 完整命令列表与参数
- [安装指南](references/install-guide.md) — 各平台详细安装步骤
- [迁移指南](references/migration-guide.md) — 从 nvm/pyenv/rustup 迁移
- [nvm 卸载脚本](scripts/uninstall_nvm.js) — Windows 上通过 Node.js 卸载 nvm
