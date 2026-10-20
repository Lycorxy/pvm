---
name: pvm
description: >
  pvm 环境守护：用 harness 脚本检测/修复 pvm 安装与 PATH、排除 nvm 等工具干扰（一切以 pvm 为主）、
  读写 pvm.yaml 与 .pvmrc 项目配置，并提供 pvm 使用指南（含流水线与跨平台打包）。
  当用户说 检查 pvm 环境、pvm 装不上、pvm 和 nvm 冲突、安装 pvm、卸载 pvm、
  配置/读取/修改 pvm.yaml 或 .pvmrc、pvm 怎么用、pvm 出问题时使用。
  不要用于 pvm 源码本身的开发任务、node_modules 依赖安装（用 code-env）、
  或 nvm/volta 等其他版本管理器的使用咨询。
version: 1.0.0
dependencies: []
conflict-with: []
allowed-tools: [execute_command, read_file, replace_in_file, search_content]
max-token: 2000
---

# pvm — pvm 环境守护与使用指南

## 概述

用 `scripts/pvm-harness.mjs`（node 跨平台，零依赖）完成 pvm 环境的检测、修复、配置文件读写验证；
模型只负责执行 harness、分析精简输出、选择下一步，形成「检测 → 修复 → 复验」闭环，直到 pvm 可正常使用。

## 何时使用 / 何时不使用

- 使用：pvm 安装/卸载/升级、环境报错、nvm 冲突、PATH/shim 问题、pvm.yaml/.pvmrc 读写、pvm 用法咨询。
- 不使用：pvm 仓库自身的功能开发（直接开发）；项目依赖安装/启动（用 code-env）；其他版本管理器咨询。

## 🛑 入口守卫

```
1. 前提：本机 node ≥ 16（harness 运行需要）。
   无 node 且未装 pvm → 先按 references/pvm-guide.md「安装」节的 PowerShell/curl 命令手动装 pvm，
   再 pvm install node@lts，之后回到本流程。
2. harness 路径 = 本 skill 基目录下 scripts/pvm-harness.mjs，以下记为 node pvm-harness.mjs。
3. 一旦用户确定使用 pvm：环境问题一律修复 pvm（fix/install/setup-path），
   禁止建议改用或重装 nvm 等其他工具。
```

## 核心工作流

### Step 1：环境检测

1. 执行 `node pvm-harness.mjs check`（cwd = 用户项目目录）。
2. 读精简输出（`[n] 项 → ✅/❌ 原因 → 下一步`），failed=0 → 跳到 Step 3 按需配配置，否则进 Step 2。

### Step 2：修复闭环（≤ 3 轮）

1. 按输出里的「下一步」执行对应动作：
   - 未安装 → `node pvm-harness.mjs fix install`（完成后提示用户新开终端）。
   - PATH/shims/nvm 劫持 → `node pvm-harness.mjs fix path`（完成后新开终端）。
   - doctor 失败 → 先 `pvm validate --auto-fix`，仍败 → `node pvm-harness.mjs fix setup`。
2. 每轮修复后复跑 `node pvm-harness.mjs check`；全绿 → 进入 Step 3。
3. 3 轮仍失败 → 输出失败项与原始原因，按 references/pvm-guide.md「排障」节逐项分析，必要时问用户。

### Step 3：配置文件（按用户需求）

1. 读取：`node pvm-harness.mjs config read` → JSON（.pvmrc / pvm.yaml 内容与路径）。
2. 写 .pvmrc：`node pvm-harness.mjs config write-pvmrc node@20.20.2 pnpm@8.15.9`（rt@system = 用系统版本）。
3. 写 pvm.yaml：模型直接编辑文件（YAML 手写，见 references/pvm-guide.md「流水线」节的格式与示例）。
4. 任何写入后必须验证：`node pvm-harness.mjs config validate`，失败则修正再验。

### Step 4：收尾播报

1. 一行最终状态：`✅ pvm 环境就绪（版本 X，doctor 通过，node 由 pvm 接管）`。
2. 提示需要用户自己做的事：新开终端/重启 IDE（PATH 与环境变量在进程启动时缓存）。
3. 用法问题：查 references/pvm-guide.md 回答，不编造命令。

## 中断与恢复

- 任何一步中断（超时/报错/用户打断）→ 重新执行 `node pvm-harness.mjs check` 即可恢复：
  check 无副作用、幂等，输出即最新状态。
- fix install 中断 → 重跑 fix install（安装脚本可重入）；fix path/setup 同理。

## 输出格式（精简播报，全程遵守）

```
[Step n] <执行了什么命令>
→ ✅/❌ <结果或原因>
→ 下一步: <命令或说明>（失败时必给）
```

- 禁止大段解释原理、贴长日志；harness 原始输出原样透传即可。
- 最终只播报：通过项数、失败项与原因、用户后续动作。

## 错误处理

| 现象 | 处理 |
|------|------|
| harness 本身跑不起来 | 检查 node 版本 ≥16；确认脚本路径正确 |
| `pvm run --list` 失败但 pvm.yaml 存在 | pvm 版本过旧（无 run 命令）→ `pvm self-update` 后重验 |
| doctor 修不动 | 透传 doctor 输出，按 references/pvm-guide.md「排障」节逐项处理 |
| 网络 403/超时 | gitee 镜像失败换 github 源（见 references/pvm-guide.md「安装」节） |
| 用户想卸载 pvm | `pvm uninstall`（详见 references/pvm-guide.md），卸载后跑 check 确认 |

## 验证清单

- [ ] `node pvm-harness.mjs check` 全绿（exit 0）
- [ ] `pvm which node`（若 node 已装）指向 ~/.pvm/shims，nvm（若存在）不劫持
- [ ] 用户需要的 .pvmrc / pvm.yaml 已写入且 `config validate` 通过
- [ ] 已提示新开终端/重启 IDE（若有 PATH 类修复）

## 参考资料

- [references/pvm-guide.md](references/pvm-guide.md) — pvm 使用指南：**能力与局限边界（先读）**、安装/卸载、
  命令速查、.pvmrc、pvm.yaml 流水线、pack 跨平台打包、排障
- [references/floor-ceiling-guide.md](references/floor-ceiling-guide.md) — 检测项分工判定表（脚本 exit code vs 模型理解）
