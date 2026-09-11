# 脚本 vs 模型分工判定表（floor-ceiling）

本 skill 的检测项按 5 类分流：能交给脚本的绝不留给模型自觉。

| 类别 | 判定 | 本 skill 中的检测项 | 落地位置 |
|------|------|--------------------|----------|
| **A 收敛型**（有确定命令，exit code 即结论） | 脚本执行 | 环境检测（OS/架构/已装状态）→ `detect-env.ps1/.sh`；安装后可执行 → `pvm version`；环境健康 → `pvm doctor` | `verify.custom`（exit 0 = pass，模型改不了） |
| **B 完备型**（产物字段齐全） | schema | 环境报告须含 `os`/`arch`/`pvm_installed`/`asset`/`installer` | `producesExist` + `producesFresh` + custom 重跑防伪 |
| **C 度量型**（数值回灌） | 不适用 | 本 skill 无度量指标 | — |
| **D 主观理解型**（需模型判断） | review.model 逐条核对 | 已装时未重复安装；资产与检测架构一致；用户意图→pvm 命令映射正确；结果已播报 | `review.checklist`（mode: model，逐条 verdict+evidence，漏评/空话/造假/fail 放行均 FAIL） |
| **E 纯审美型** | 不进 verify | CLI 输出文案风格 | — |

## 防伪规则

- A 类检测优先用 `review.mode: 'script'`（引擎重跑 command），模型口述「装好了」不算数
- 环境检测结论必须来自 `detect-env` 脚本输出，禁止模型凭 `uname` 片段自行推断架构
- 已安装判定必须来自 `pvm_installed=true/false` 键值，禁止只看 `command -v` 单一信号

## 分流速查

遇到新检测项时问一句：「是否有现成脚本/CLI，exit code 能给出唯一结论？」
- 是 → A/B 类，进 verify（custom / schema / script review）
- 否，需理解用户意图或判断合理性 → D 类，进 review.model
- 纯风格 → E 类，不进
