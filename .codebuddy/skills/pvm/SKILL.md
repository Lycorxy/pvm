---
name: pvm
description: >
  pvm 环境守护（pvm 仓库自用）：用 harness 脚本检测/修复 pvm 安装与 PATH、排除 nvm 等工具干扰、
  校验 pvm.yaml 与 .pvmrc，并提供 pvm 用法速查（流水线、打包、压缩）。
  当用户说 检查 pvm 环境、pvm 装不上、pvm 和 nvm 冲突、安装 pvm、卸载 pvm、
  配置/读取/修改 pvm.yaml 或 .pvmrc、pvm 怎么用、pvm 出问题时使用。
  不要用于 pvm 源码本身的开发任务、node_modules 依赖安装（用 code-env）、
  或 nvm/volta 等其他版本管理器的使用咨询。
version: 1.1.0
dependencies: []
conflict-with: []
allowed-tools: [execute_command, read_file, replace_in_file, search_content]
max-token: 1200
---

# pvm — 环境体检与配置

面向本仓库（pvm 自身）的日常使用：跑一条命令看状态，有红色才处理。

## 三条命令

```bash
node scripts/pvm-harness.mjs check            # 环境体检
node scripts/pvm-harness.mjs fix path         # 修 PATH（仅 check 报错时）
node scripts/pvm-harness.mjs config validate  # 配置文件校验
```

## 输出（已精简，照抄即可）

全绿只有一行：

```
✅ pvm 0.0.2 | shims ok | nvm ok | doctor ok | .pvmrc ok | pvm.yaml 6 行清单
```

有失败才展开：

```
❌ 1/6 项失败
❌ shims PATH: C:\Users\me\.pvm\shims 不在 PATH（node 等命令不会被 pvm 接管）
   下一步: node pvm-harness.mjs fix path，完成后新开终端再 check
```

播报规则：只回传这一行（或失败项），**不贴原始日志、不复述原理**。

## 流程

1. `check` 全绿 → 直接回答用户；有红 → 下一步。
2. 按失败项的「下一步」执行：`fix install` / `fix path` / `fix setup`；
   `doctor` 失败先 `pvm validate --auto-fix`。每轮后复跑 `check`，最多 3 轮。
3. 改配置：`config read` → 编辑 `pvm.yaml`（四条规则：`名字: 命令`、列表=顺序、
   `parallel:`=并行、步骤名=引用）→ `config validate`。
4. 收尾一行状态 + 需要用户做的事（新开终端/重启 IDE）。

## 本仓库常用

| 命令 | 作用 |
|------|------|
| `pvm run` | 列出全部流水线 |
| `pvm run release:pro` | test → build → pkg |
| `pvm pkg go --entry . --output dist/pvm --type all` | 5 平台二进制 |
| `pvm pkg node --entry src/app.ts --output dist/app --type all --libc legacy` | node 单文件（兼容老 glibc） |
| `pvm zip <src> -o x.zip --reproducible` | 可复现压缩 |

## 错误处理

| 现象 | 处理 |
|------|------|
| harness 跑不起来 | node ≥ 16；确认脚本路径 |
| `pvm run` 报 Unknown command | pvm 过旧（无 run/pkg）→ `pvm self-update` 或重装 |
| `pvm run` 报 circular reference | 流水线互相引用，按报错链路拆开 |
| doctor 修不动 | 透传输出，按 pvm-guide.md「排障」逐项处理 |
| 普通终端装不了、管理员可以 | `~/.pvm` 所有权被污染 → `pvm repair-owner`（Windows 弹 UAC） |
| 网络 403/超时 | 默认国内镜像；`--official` 切官方源 |

## 验证清单

- [ ] `node pvm-harness.mjs check` 全绿（exit 0）
- [ ] `pvm which node` 指向 `~/.pvm/shims`（若装了 node）
- [ ] `pvm.yaml` / `.pvmrc` 改动后 `config validate` 通过
- [ ] 有 PATH 类修复时已提示新开终端

## 参考资料

- [references/pvm-guide.md](references/pvm-guide.md) — 使用指南：边界、安装/卸载、命令速查、
  `.pvmrc`（含 `.nvmrc` 兼容）、`pvm.yaml` 四条规则、`pkg`（`--type`/`--include`/`--libc`）、`zip`、排障
- [references/floor-ceiling-guide.md](references/floor-ceiling-guide.md) — 检测项分工判定表
