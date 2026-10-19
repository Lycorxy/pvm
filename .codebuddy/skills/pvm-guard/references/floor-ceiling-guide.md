# 分工判定表（pvm-guard）

原则：能被脚本/CLI 用 exit code 判定的一律走 harness（准确 100%）；模型只做意图理解与下一步选择。

| # | 检测/动作 | 类型 | 执行方 | 判定方式 |
|---|-----------|------|--------|----------|
| 1 | pvm 是否安装 / 版本 | A 收敛 | `check` 第 1 项 | `pvm --version` exit code + `~/.pvm/bin` 存在性 |
| 2 | shims 是否在 PATH | A 收敛 | `check` 第 2 项 | PATH 目录比对（脚本） |
| 3 | nvm 是否劫持 node | A 收敛 | `check` 第 3 项 | `pvm which node` 输出路径判定（脚本） |
| 4 | pvm 环境健康 | A 收敛 | `check` 第 4 项 | `pvm doctor` exit code |
| 5 | .pvmrc / pvm.yaml 可解析 | A 收敛 | `config validate` | `pvm current` / `pvm run --list` exit code |
| 6 | 安装 / 修复动作 | A 收敛 | `fix install/path/setup` | 安装脚本 / pvm 命令 exit code + 复跑 check |
| 7 | .pvmrc 写入 | A 收敛 | `config write-pvmrc` | 写入后 `pvm current` exit code |
| 8 | pvm.yaml 内容（流水线写什么命令） | D 主观 | 模型 | 理解用户意图 + references/pvm-guide.md 格式约束，写后必须 `config validate` |
| 9 | 装哪些运行时版本 | D 主观 | 模型 | 问用户 / 按项目栈推断，写 .pvmrc 后验证 |
| 10 | 输出措辞与解释详略 | E 审美 | — | 不进任何门禁（遵守 RULE.mdc 精简播报即可） |

- A 收敛（1-7）：禁止模型口述"应该没问题"，必须以 harness exit code / 输出为准。
- D 主观（8-9）：模型一次过一遍，逐条给出结论；产物必须再过 A 类验证（validate）兜底。
- E 审美（10）：不设检测。
