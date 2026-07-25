---
name: pvm-env-manager
description: >
  检测、安装、管理 PVM 多语言版本管理器，提供运行时环境隔离。
  当用户需要安装/卸载/迁移版本管理器、处理local文件冲突、
  环境诊断、配置npmrc时使用。
version: 3.0.0
dependencies: [specflow-rules-engine]
conflict-with: []
allowed-tools: [read_file, execute_command, search_content]
max-token: 3000
---

# PVM 环境管理器

## 概述

检测、安装、管理 PVM（Polyglot Version Manager）多语言版本管理器，为项目和 AI Skill 提供运行时环境隔离。

**使用场景**：
- 安装/卸载/更新 PVM 或其管理的运行时
- 需要运行时环境（python、node、go、rust 等）
- 从 nvm/fnm/pyenv 迁移到 PVM
- 项目级版本锁定（.pvmrc）
- 环境问题诊断与修复
- local 文件冲突处理

**不使用**：Git 版本管理、Docker 环境、系统包管理（apt、brew、choco）、开发 PVM 源码

---

## 🛑 入口守卫

```
必须按顺序执行 Step 0→1→2→... 全部步骤。
每步必须播报完成消息。未播报 = 未执行 = 违规。

禁止：✗ 跳过 Step 0 / ✗ 不播报 / ✗ 无声跳过

例外：同会话已执行步骤可跳过，但必须声明：
  ⏭️ [跳过] Step N — 原因 | 风险
```

---

## 核心工作流

### Step 0：规则注入（🔒 强制 · 最高优先级）

```
alwaysApply: true → use_skill("specflow-rules-engine")
  1. 同步 RULE.mdc 到 .codebuddy/rules/pvm-env-manager
  2. 检测 System Prompt 是否已注入
  3. 未注入 → read_file 兜底
```

```
✅ [Step 0] 规则注入完成
  ▸ 注入方式：{specflow-rules-engine / read_file 兜底}
  ▸ alwaysApply: true
```

---

### Step 1：环境检测（🔒 强制）

检测 PVM 安装状态、操作系统、已安装运行时、冲突工具。

**执行**：
```bash
# 检测 OS
uname -s                    # macOS/Linux
$env:OS                     # Windows

# 检测 PVM
command -v pvm && pvm --version   # macOS/Linux
where.exe pvm; pvm --version       # Windows

# 检测冲突工具
for tool in nvm fnm pyenv rustup volta asdf; do command -v $tool; done  # macOS/Linux
foreach ($tool in @('nvm','fnm','pyenv','rustup','volta','asdf')) { Get-Command $tool }  # Windows
```

```
✅ [Step 1] 环境检测完成
  ▸ 操作系统：{OS}
  ▸ PVM 状态：{已安装 vX.X.X / 未安装}
  ▸ 已装运行时：{列表 / 无}
  ▸ 冲突工具：{nvm@路径 / 无}
```

---

### Step 2：安装 PVM（未安装时执行）

**安装策略**（按顺序尝试，成功即停）：
```
A. 一键脚本（推荐）
   Windows: iwr -useb https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.ps1 | iex
   macOS/Linux: curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.sh | bash
   ↓ 失败（404）

B. Releases 下载
   浏览器打开：https://github.com/Lycorxy/pvm/releases
   ↓ 失败（无 Release）

C. 源码自建（需 Go 1.22+）
   git clone https://github.com/Lycorxy/pvm.git && cd pvm && go build
```

**验证**：`pvm --version` + `pvm doctor`

**特殊情况**：
- pvm.exe 被占用 → 杀进程后复制+重命名替换
- 杀软误报（Windows）→ 告知误报，指引「仍要运行」
- 编辑器终端不生效 → 完全重启编辑器

```
✅ [Step 2] PVM 安装完成
  ▸ 版本：{vX.X.X}
  ▸ 安装路径：{~/.pvm}
  ▸ 安装方式：{A一键 / B Releases / C源码}
```

---

### Step 3：运行时安装与切换（按需执行）

**决策树**：
- 项目级（有 .pvmrc）→ 读取并安装 + `pvm use --local`
- 用户级（全局）→ `pvm install <runtime>@<version>` + `pvm use`
- Skill 请求 → 询问版本偏好，安装 + 切换

**执行**：
```bash
pvm install node@20        # 安装
pvm use node@20 --local    # 项目级
pvm use node@20            # 用户级
```

**验证**：`node --version` / `python --version` / `pvm current`

```
✅ [Step 3] 运行时就位
  ▸ {runtime} = {version}（项目级 / 用户级）
```

---

### Step 4：冲突工具迁移（检测到冲突时执行）

**铁律**：先装后卸，不装不卸。

**决策树**：
```
nvm/fnm → 1. pvm install node@版本
          2. pvm use node@版本
          3. 验证 node --version
          4. 卸载 nvm（脚本或手动）
          5. pvm setup 修复 PATH

pyenv → 1. pvm install python@版本
        2. pvm use python@版本
        3. 卸载 pyenv

rustup → 1. pvm install rust@版本
         2. pvm use rust@版本
         3. 卸载 rustup
```

**nvm 卸载**（Windows）：
```bash
node .codebuddy/skills/pvm-env-manager/scripts/uninstall_nvm.js --yes
```

```
✅ [Step 4] 冲突工具迁移完成
  ▸ 已卸载：{nvm / fnm / pyenv}
  ▸ PVM 替代：{node@20.x.x 已就位}
```

---

### Step 5：项目环境隔离（按需执行）

创建或更新 .pvmrc，实现项目间版本隔离。

**执行**：
```bash
cat .pvmrc                    # 查看现有配置
pvm use node@20 --local      # 写入 .pvmrc
pvm use python@3.12 --local
git add .pvmrc                # 提交到仓库
```

**验证**：`cd /other/project && node --version`（切换目录自动生效）

```
✅ [Step 5] 项目环境隔离已配置
  ▸ .pvmrc：{node=20.11.0, python=3.12.0}
  ▸ 进目录自动生效
```

---

### Step 6：更新与卸载（按需执行）

```bash
pvm self-update              # 更新 PVM
pvm uninstall --yes          # 卸载 PVM
pvm remove node@18           # 卸载运行时
```

```
✅ [Step 6] {更新/卸载} 完成
  ▸ 操作：{self-update / uninstall / remove}
  ▸ 结果：{已更新到 vX.X.X / 已完全卸载}
```

---

### Step 7：实用工具脚本（按需执行）

使用跨平台脚本解决常见环境问题。

**脚本执行路径**：`.codebuddy/skills/pvm-env-manager/scripts/`

#### 7.1 local 文件合并冲突处理

**场景**：用户说"local文件冲突"、"合并冲突"

**执行**：
```bash
# Windows
.\.codebuddy\skills\pvm-env-manager\scripts\resolve-local-conflict.bat --remote

# macOS/Linux
./.codebuddy/skills/pvm-env-manager/scripts/resolve-local-conflict.sh --remote
```

**参数**：`--local`（保留本地）/ `--remote`（使用远程，默认）/ `--auto`

#### 7.2 从远程替换 local 文件

**场景**：用户说"从远程替换"、"替换local文件"

**执行**：
```bash
# Windows
.\.codebuddy\skills\pvm-env-manager\scripts\replace-local-from-remote.bat --backup

# macOS/Linux
./.codebuddy/skills/pvm-env-manager/scripts/replace-local-from-remote.sh --backup
```

**参数**：`--backup`（备份，默认）/ `--no-backup`（不备份）/ `--list`（仅列出）

#### 7.3 软件彻底卸载

**场景**：用户说"彻底卸载"、"卸载node/git/nvm"等

**执行**：
```bash
# Windows
.\.codebuddy\skills\pvm-env-manager\scripts\uninstall-tool.bat <软件名> --yes

# macOS/Linux
./.codebuddy/skills/pvm-env-manager/scripts/uninstall-tool.sh <软件名> --yes
```

**支持**：node / git / nvm / pvm / python / yarn / pnpm

#### 7.4 PVM 环境诊断

**场景**：用户说"环境诊断"、"诊断pvm"、"端口冲突"

**执行**：
```bash
# Windows
.\.codebuddy\skills\pvm-env-manager\scripts\diagnose-pvm-env.bat --fix

# macOS/Linux
./.codebuddy/skills/pvm-env-manager/scripts/diagnose-pvm-env.sh --fix
```

**参数**：`--fix`（自动修复）

#### 7.5 .npmrc 自动配置

**场景**：用户说"配置npmrc"、"npmrc配置"

**执行**：
```bash
# Windows
.\.codebuddy\skills\pvm-env-manager\scripts\setup-npmrc.bat --force

# macOS/Linux
./.codebuddy/skills/pvm-env-manager/scripts/setup-npmrc.sh --force
```

**参数**：`--force`（强制覆盖）

**播报完成消息**：
```
✅ [Step 7] {脚本功能} 完成
  ▸ 脚本：{脚本名称}
  ▸ 参数：{使用的参数}
```

---

## 中断与恢复

```
⏯️ [恢复] 上次进度：X/7 步完成
已完成：✅ Step 0 — 规则注入
待完成：⬜ Step 1 — 环境检测
断点：从 Step 1 继续
```

---

## 错误处理

| 错误场景 | 恢复操作 |
|---------|---------|
| 安装脚本 404 | 跳转到方式 B（Releases）或方式 C（源码） |
| Releases 无版本 | 等待作者发布或源码自建 |
| PVM 命令找不到 | 重启终端/编辑器，检查 PATH |
| `pvm use` 无效 | 运行 `pvm doctor` + `pvm setup` |
| 杀软拦截 | 告知误报，指引「仍要运行」 |

---

## 验证清单

- [ ] PVM 已正确安装（`pvm --version`）
- [ ] PATH 中 `~/.pvm/shims` 排在最前
- [ ] 目标运行时已安装（`pvm list`）
- [ ] 版本切换生效（`node --version`）
- [ ] .pvmrc 已提交（项目级隔离）
- [ ] 冲突工具已清理

---

## 参考资料

- [PVM 命令参考](references/pvm-commands.md)
- [安装指南](references/install-guide.md)
- [迁移指南](references/migration-guide.md)
- [实用脚本详解](scripts/README.md)