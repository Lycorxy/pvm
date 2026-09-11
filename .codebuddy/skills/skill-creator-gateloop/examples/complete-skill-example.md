# 完整 Skill 示例：code-review-check v3（GateLoop Harness 模式）

一个展示 v3 最佳实践（GateLoop harness + RULE.mdc 内联 + 任务式流程 + floor-ceiling 分工）的生产级 skill。

> v3 关键变更：不再依赖 specflow-rules-engine。规则注入由 GateLoop harness `reads` + `rulesInline` 自动完成（或独立 skill 时由 `RULE.mdc` `alwaysApply: true` 直接注入 System Prompt）。

## 文件：RULE.mdc

```markdown
---
description: 代码评审铁律。当执行代码评审、检查 bug、代码审查时必须加载此规则。包含：安全审计铁律、逻辑检查铁律、质量检查铁律、输出格式铁律、验证清单。
alwaysApply: true
enabled: true
updatedAt: 2026-08-05T10:00:00.000Z
provider:
---

# 代码评审 — 强制约束（System Prompt 级 · 独立 skill 注入方式 B）

## 铁律 0 — 评审流程（强制 · 最高优先级）

评审流程中的每个 Step 必须按序执行，不得跳过。
规则已由 `alwaysApply: true` 注入 System Prompt，无需 Step 0 显式调用。

## 铁律 1 — 安全审计（所有文件必须检查）

| 检查项 | 要查找的内容 |
|-------|------------|
| 注入 | SQL/NoSQL/命令/HTML 注入向量 |
| 密钥 | 源代码中硬编码的密钥、令牌 |
| XSS | 响应中未转义的用户输入 |

## 铁律 2 — 输出格式（强制）

所有发现必须按以下格式报告：
[文件]:[行号] — [严重程度] — [问题标题]
```

## 文件：SKILL.md

```markdown
---
name: code-review-check
description: >
  执行彻底的代码评审，检查 bug、安全漏洞、
  命名违规和性能问题。当用户请求
  代码评审、review code、code review、检查 bug、查找问题、
  优化现有代码或检查 PR 变更时使用。
  不要用于编写新功能、回答语法问题、
  生成样板代码或评审非代码文档。
version: 3.0.0
dependencies: []
conflict-with: []
allowed-tools: [read_file, search_content]
max-token: 2000
---

# 代码评审检查器 v3（独立 skill · 注入方式 B）

> 核心定位：规则由 RULE.mdc `alwaysApply: true` 注入 System Prompt（无需 Step 0），
> 系统化评审，逐条验证交付。
> 脚本/模型分工遵循 floor-ceiling-guide.md（脚本管收敛型，模型管发散型）。

## 🛑 入口守卫

```
本 Skill 加载后必须按顺序执行 Step 1→2→3→4 全部步骤。
每步必须播报对应完成消息。未播报的步骤 = 未执行 = 违规。
```

## 工作流总览

```
（规则已由 RULE.mdc alwaysApply 注入，无需 Step 0）
Step 1: 读取并映射文件
Step 2: 安全审计
Step 3: 逻辑检查
Step 4: 质量检查
Step 5: 输出报告并验证
```

## 核心工作流

### Step 1：读取并映射文件

[低自由度] 完整读取所有目标文件...

### Step 5：输出报告

⚠️ 交付前必须逐一验证（RULE.mdc 铁律 2 约束）：
- [ ] 每个问题都有具体行号
- [ ] 无假阳性
- [ ] 总结与详细发现匹配

## 中断与恢复机制

若中断后恢复，扫描已有输出，播报恢复点，从断点继续。

## 错误处理

[同 v2]
```

## 文件：skill.config.yaml

```yaml
priority: 85
auto-trigger: true
trigger-keywords:
  - 代码评审
  - review code
  - code review
  - check for bugs
  - 查bug
  - 代码优化

# 规则注入方式 B：RULE.mdc alwaysApply: true 自动注入 System Prompt
# 不再强制 depends-on: [specflow-rules-engine]
depends-on: []
mutual-exclude: []
require-tools: [read_file, search_content]
load-strategy: lazy
unload-after-run: true
max-duration: 180
trigger-weight:
  代码评审: 95
  review code: 90
  查bug: 80
  代码优化: 60
```

## 文件：references/floor-ceiling-guide.md（🔥 必须产出）

```markdown
# 脚本/模型分工判定表

| 质量要求 | 类型 | 归属 |
|---|---|---|
| 安全漏洞是否存在（注入/密钥/XSS） | 收敛型 | 模型判断（无可脚本化的客观指标，但有客观事实陈述） |
| 评审是否覆盖所有目标文件 | 完备性 | 脚本 schema 字段（要求列出审查文件清单） |
| 评审输出格式（[文件]:[行号]—[严重程度]—[标题]） | 收敛型 | 脚本 custom（正则扫输出格式） |
| bug 严重程度评级 | 发散型 | 模型自评 |
| 修复建议是否合理 | 发散型 | 模型自评 |
```

## 文件：examples/triggers.json

```json
{
  "should_trigger": [
    "帮我review一下这个文件",
    "检查这段代码有没有隐藏bug",
    "review src/auth/login.ts"
  ],
  "should_not_trigger": [
    "帮我写一个登录接口",
    "JS for循环语法怎么用"
  ]
}
```

## v2 → v3 关键变化

| 维度 | v2（旧） | v3（新） |
|------|---------|---------|
| 规则注入 | use_skill("specflow-rules-engine") | RULE.mdc alwaysApply: true 自动注入 |
| dependencies | `[specflow-rules-engine]` | `[]`（不强制） |
| Step 0 | 强制调用 use_skill | 已删除（规则已注入） |
| 脚本/模型分工 | 无明确定义 | floor-ceiling-guide.md 必产出 |
| 入口守卫 | ✅ 强制步骤执行 | ✅ 保留 |
| 中断恢复 | ✅ 扫描 + 播报 + 续接 | ✅ 保留 |
| 每步播报 | ✅ 播报完成消息作为检查点 | ✅ 保留 |

---

## 带 MCP 数据契约的 Step 示例

当 skill 某步骤需要 MCP 提供的知识库/上下文/记忆时，用数据契约声明，核心逻辑零 MCP 工具名。

### skill.config.yaml 契约声明

```yaml
step-contracts:
  - id: similar-bug-history
    step: "2"
    name: "获取同类 bug 历史记录"
    criticality: preferred           # 软依赖：本地经验可兜底
    input:
      bug_signature: "bug 特征签名"
    output:
      past_bugs: "历史相似 bug 列表"
      fixes: "历史修复方案"
    providers:
      - mcp:                         # 优先级 1：MCP（存在时必调用）
          server: "memory-store"
          tool: "recall"
          params:
            query: "{input.bug_signature}"
      - fallback: "local"            # 优先级 2：本地兜底
    on-missing: degrade
    degrade-source: "references/common-bugs.md"

  - id: full-codebase-context
    step: "1"
    name: "获取完整代码库上下文"
    criticality: required            # 硬依赖：缺则阻断
    input:
      target_files: "目标文件列表"
    output:
      dependency_graph: "依赖关系图"
      module_summary: "模块摘要"
    providers:
      - mcp:
          server: "knowledge-base"
          tool: "get_context"
          params:
            files: "{input.target_files}"
      - builtin:                     # 降级：内置工具
          strategy: "search_content + code-explorer"
      - fallback: "ask-user"
    on-missing: block

memory-hooks:
  on-step-complete:
    - event: "bug-found"
      mcp-store:
        server: "memory-store"
        tool: "save_memory"
```

### SKILL.md 正文（零 MCP 工具名）

```markdown
### Step 1：获取代码库上下文

🔒 数据契约：full-codebase-context（criticality: required）

#### 契约获取子流程
按 providers 链获取 {dependency_graph, module_summary}：
  1. MCP knowledge-base:get_context（优先，存在时必调用）
  2. 内置 search_content + code-explorer（降级）
  3. fallback: ask-user（最终）
播报：✅ [Step 1·契约] full-codebase-context 已就绪（来源：{MCP/内置/用户}）
全部失败且 on-missing=block → ⛔ 阻断，不得继续

#### 核心逻辑
1. 基于 dependency_graph 建立影响范围
2. 按 module_summary 聚焦审查重点
3. 播报：✅ [Step 1] 上下文建立完成

### Step 2：安全审计

🔒 数据契约：similar-bug-history（criticality: preferred）

#### 契约获取子流程
按 providers 链获取 {past_bugs, fixes}：
  1. MCP memory-store:recall（优先）
  2. fallback: local → read_file(references/common-bugs.md)
播报来源。MCP 不可用 → 本地兜底继续。

#### 核心逻辑
1. 逐文件检查注入/密钥/XSS（参照 RULE.mdc 铁律 1）
2. 对照 past_bugs 排查同类问题
3. 对照 fixes 验证是否已修复
4. 播报：✅ [Step 2] 安全审计完成（来源：{MCP/本地}）

---

## GateLoop harness 模式示例（anchor.config.json · 以 figma-dom-style 为蓝本）

上半部分展示「独立 skill 注入方式 B」。下面补全本 skill 主要推荐的 **GateLoop harness（anchor.config.json）** 形态——把 Step 0→N 映射为 anchors，由引擎门控推进。完整 JSON 见 `SKILL.md` Phase 3 与 `references/gateloop-engine-guide.md` §16。

### anchor.config.json（精简版）

```json
{
  "name": "figma-dom-style",
  "schemaVersion": "1.0",
  "version": "14.0.0",
  "entry": "deliver",
  "reads": ["RULE.mdc"],
  "anchors": [
    { "id": "materials", "dependsOn": [], "output": ".specflow/context/origin-dsl.json",
      "verify": { "producesExist": true, "producesFresh": true, "schema": {},
        "custom": ["npx nut gateloop metric design-system"] } },
    { "id": "layout", "dependsOn": ["materials"], "output": ".specflow/context/layout-plan.json",
      "verify": { "producesExist": true, "producesFresh": true, "schema": {},
        "custom": ["npx nut gateloop metric spacing"] } },
    { "id": "reduce", "dependsOn": ["layout"], "output": ".specflow/context/final-dsl.json",
      "requiresSubtasks": [
        { "id": "b1", "desc": "基线 HTML 提取真实内容", "dependsOn": [] },
        { "id": "b5", "desc": "逐元素保真 + 渲染对比", "dependsOn": ["b1"] },
        { "id": "b6", "desc": "gate 5 道门禁", "dependsOn": ["b5"] }
      ],
      "verify": { "producesExist": true, "producesFresh": true, "schema": {},
        "custom": ["npx nut gateloop metric ssim"],
        "review": { "output": ".specflow/context/restore-report.json", "requireEvidence": true,
          "checklist": [
            { "id": "size", "rule": "尺寸还原", "mode": "model" },
            { "id": "spacing", "rule": "间距还原", "mode": "model" }
          ] } } },
    { "id": "deliver", "dependsOn": ["reduce"],
      "verify": { "review": { "output": ".specflow/context/deliver-report.json", "requireEvidence": true,
        "checklist": [ { "id": "structure", "rule": "HTML 结构合法", "mode": "model" } ] } } }
  ]
}
```

**关键差异（对照 注入方式 B）：**
- 规则注入靠 `reads: ["RULE.mdc"]` 由引擎内联，不再需要 `alwaysApply` + Step 0。
- 4 个 anchor 由 `dependsOn` 拓扑驱动，verify 不过卡住跳不了。
- `requiresSubtasks` 把 `reduce` 拆成任务队列，全绿才推进（见 `references/gateloop-engine-guide.md` §7）。
- 启动命令：`npx nut gateloop init` → `npx nut gateloop next` → `npx nut gateloop status`（`nut ads` 已废弃，命令为 `nut gateloop`）。执行环境：ai-ultra 本仓库内直接 `node dist/cli/index.cjs gateloop ...`（改过 src 先 `npm run build`）；目标工作区须已装 `ai-ultra`（未装先 `npm i -D ai-ultra`，验证 `npx nut gateloop --help`，禁止 `-g` 全局安装）。
```
