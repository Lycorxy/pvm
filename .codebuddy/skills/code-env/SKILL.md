---
name: code-env
description: >
  环境检测流程（项目环境准备闭环）：检测并安装 git 与 pvm → 按给的 git 地址
  拉取主分支代码并创建修复缺陷分支 → pvm 安装 node/pnpm（按需 go/python）→
  pnpm i 安装依赖（自动识别单仓/monorepo/散仓）→ 启动验证前端项目可正常执行。
  由 GateLoop harness 驱动，5 anchor 门禁（schema+custom+review），引擎 exit code 强制。
  自包含可分发：pvm 检测/安装脚本与命令手册在本 skill 内；同级有 pvm / flowcli
  skill 时以其为增强。
  触发词：环境检测、准备环境、搭建开发环境、初始化项目环境、拉代码建分支、
  装依赖、环境搭建流程、修复缺陷环境准备。
  不要用于：单独安装某个运行时（→ pvm skill）、流水线/MR/tag 操作（→ flowcli）、
  与环境无关的普通开发任务。
version: 3.0.0
dependencies: []
conflict-with: []
allowed-tools: [read_file, write_file, replace_in_file, search_content, search_file, execute_command, list_dir]
max-token: 3000
---

# 环境检测流程 v3.0 — GateLoop Harness 驱动（5 anchor 闭环）

> 环境检测完整流程：git/pvm 检测→拉码建分支→运行时→依赖→启动验证。
> **自包含可分发（knote）**：detect-env / install-pvm / fix-pnpm-eperm 脚本与
> pvm-commands.md 命令手册都在本 skill 内（`scripts/` + `references/`），复制目录即用；
> 全部相对路径 + `${skillDir}`；同级存在 `../pvm` / `../flowcli` skill 时以其为增强。

## 🔧 执行环境

- 本 skill 可放任意目录（knote/任意项目 `.codebuddy/skills/`）：manifest 即 `<skillDir>/anchor.config.json`
- **自包含**：`scripts/detect-env.*`、`scripts/install-pvm.*`、`scripts/check-env.mjs`、`references/pvm-commands.md` 均在本 skill 内
- **可选增强**：同级 `../pvm`（pvm 能力底座）存在时以其脚本/手册为准；同级 `../flowcli`（git/流水线命令手册）存在时 gitflow 场景先查其指南

**驱动 CLI 前置（ai-ultra，未装先装）**——本 skill 靠 `nut gateloop` 驱动，执行前必须确认：

```
1. 检测：npx nut --version 执行成功（exit 0）= 已安装
2. 未安装 → 先装（devDependency，按项目包管理器）：
   npm install ai-ultra --save-dev | pnpm add -D ai-ultra | yarn add -D ai-ultra
   （裸目录先 npm init -y；禁止 -g 全局安装）
3. 验证：npx nut gateloop --help 能列出子命令 = 可用
4. ai-ultra 本仓库内：直接 node dist/cli/index.cjs gateloop ...（改过 src 先 npm run build），禁止再 npm 安装
```

## GateLoop 驱动模式

```bash
MANIFEST=<skillDir>/anchor.config.json
nut gateloop init $MANIFEST --task "按 <git地址> 准备修复缺陷的开发环境"
# 引擎按 anchor 依次推进，每个 anchor：模型执行 → nut gateloop next → 引擎校验（exit 0 才过）
nut gateloop status $MANIFEST   # 断点恢复
```

### 模型只做三件事

1. **读指令** — rulesInline（RULE.mdc 已内联）+ brief（含各步执行细节）
2. **执行当前 anchor** — 按分工规矩用对应命令（见下）
3. **推进** — 写报告 JSON + `nut gateloop next`，引擎 custom/review 校验

## 5 Anchor 流程

| # | Anchor | 做什么 | 引擎验证（检测清单两条路径） |
|---|--------|--------|------------------------------|
| 1 | **env-check** | 检测 git（有=不动）+ pvm（未装则装，确保成功） | 路径1：`git --version` + `pvm version` exit 0；schema 卡报告结构 |
| 2 | **repo-setup** | 按 git 地址拉主分支 + 创建修复缺陷分支（fix/xxx） | 路径1：仓库分支状态 = 报告声明分支 |
| 3 | **toolchain** | pvm 安装 node/pnpm，检测 go/python 需求一并安装 | 路径1：全部运行时版本命令 exit 0 + 版本一致；路径2：review 核对语言需求无遗漏 |
| 4 | **deps-install** | pnpm i（识别 single/monorepo/multi-repo，逐目录装） | 路径1：每处 node_modules 存在；路径2：review 核对形态判断与覆盖 |
| 5 | **verify-run** | 启动 dev server 验证可正常执行（HTTP <500 / build 兜底），过后停服务 | 路径1：真实请求 url 或重跑构建；路径2：review 核对启动方式/证据/清理 |

**产出报告链**（artifact 管道）：`env-report.json` → `repo-report.json` → `toolchain-report.json` → `deps-report.json` → `run-report.json`（全部落 `.specflow/context/`，下游 anchor 由引擎 artifactsInline 自动内联）。

## 命令分工（硬规矩）

```
pvm 相关命令（检测/安装运行时/切版本/doctor/锁 pnpm）：
  → 脚本：${skillDir}/scripts/detect-env.* / install-pvm.* / fix-pnpm-eperm.*（同级 ../pvm 存在时以其为准）
  → 命令手册：${skillDir}/references/pvm-commands.md（禁止凭记忆拼 pvm 命令）
git / 流水线相关命令：
  → 同级存在 ${skillDir}/../flowcli 时：先查其 references/gitflow-guide.md（或 pipeline-guide.md）确认语法与前置条件；
    需要流水线/工蜂规范但 flowcli 未安装 → 按 flowcli skill Step 0 安装（npm install -g @tencent/flowcli，详见其 references/environment.md）
  → 不存在 flowcli：普通原生 git（fetch/checkout/pull/branch）直接执行；
    工蜂/gitflow 专有规范（版本分支/MR/tag/回退）缺失指南时向用户确认规范，禁止凭记忆拼
任何流程中需要 pvm / git / 流水线相关命令 → 必须走上述对应 skill/CLI，不手搓
```

## 中断与恢复

```
nut gateloop status $MANIFEST → 当前 anchor + attempt → 从断点继续
环境已变化（如分支被切走）→ nut gateloop retry 后重做当前 anchor
verify-run 的 dev server 跨轮中断 → 重启服务再 next（mode=http 校验需要服务在线）
```

## 错误处理

| 场景 | 处理 |
|------|------|
| detect-env 退出码 1（无匹配 Release） | 停止并告知用户，不猜架构 |
| pvm 刚装完 command not found | 绝对路径 `~/.pvm/bin/pvm` 验证；重启终端 |
| pnpm i EPERM | `${skillDir}/scripts/fix-pnpm-eperm.* --reinstall` |
| clone 失败 / 权限错误 | 检查 git 地址与凭证；工蜂场景按 flowcli 指南处理 |
| dev server 端口占用 | 换端口（--port）重启，更新 run-report.url |
| verify-run HTTP 校验失败 | 服务必须保持运行到 next 通过；通过后立即停服务 |

## 验证清单（引擎门禁，非模型自评）

- [ ] env-check：git + pvm 命令 exit 0（引擎 custom 重跑）
- [ ] repo-setup：当前分支 = 报告声明分支，origin 远端存在
- [ ] toolchain：声明运行时全部可用且版本一致；review 语言需求无遗漏
- [ ] deps-install：每处 installs[].dir 下 node_modules 存在；review 形态判断正确
- [ ] verify-run：HTTP <500 或构建 exit 0；review 启动方式合理 + 服务已清理

## 参考资料

| 文件 | 内容 | 加载方式 |
|------|------|---------|
| `RULE.mdc` | 流程铁律（分工规矩/禁跳步/脚本优先） | GateLoop reads 内联 |
| `scripts/check-env.mjs` | 5 phase 客观校验（引擎 custom 调用） | 引擎 custom |
| `scripts/detect-env.*` / `install-pvm.*` | 环境检测 / pvm 自动安装 | env-check 执行时 |
| `references/pvm-commands.md` | pvm 命令映射手册 | toolchain 执行时 |
| `references/install-guide.md` | 安装失败降级（手动/源码构建） | 安装失败时 |
| `../flowcli/references/gitflow-guide.md` | 分支/MR/tag/回退命令（存在 flowcli 时） | repo-setup 执行时 |
