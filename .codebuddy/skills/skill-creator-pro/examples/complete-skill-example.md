# 完整 Skill 示例：code-review-check v2

一个展示 v2 最佳实践（含 RULE.mdc + 规则注入 + 任务式流程）的生产级 skill。

## 文件：RULE.mdc

```markdown
---
description: 代码评审铁律。当执行代码评审、检查 bug、代码审查时必须加载此规则。包含：安全审计铁律、逻辑检查铁律、质量检查铁律、输出格式铁律、验证清单。
alwaysApply: true
enabled: true
updatedAt: 2026-07-03T10:00:00.000Z
provider:
---

# 代码评审 — 强制约束（System Prompt 级）

## 铁律 0 — 评审流程（强制 · 最高优先级）

🔒 评审流程中的每个 Step 必须按序执行，不得跳过。

### Step 0：规则注入（🔒 强制 · 不可跳过）

评审开始前必须先调用 use_skill("specflow-rules-engine") 注入本规则。

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
version: 2.0.0
dependencies: [specflow-rules-engine]    # 🔒 强制依赖
conflict-with: []
allowed-tools: [read_file, search_content]
max-token: 2000
---

# 代码评审检查器 v2

> 核心定位：先注入规则、再系统化评审、最后逐条验证交付。

## 🛑 入口守卫

```
本 Skill 加载后必须按顺序执行 Step 0→1→2→3→4→5 全部步骤。
每步必须播报对应完成消息。未播报的步骤 = 未执行 = 违规。
```

## 工作流总览

```
🔒 Step 0: 规则注入（强制·不可跳过）
    ↓ use_skill("specflow-rules-engine")
Step 1: 读取并映射文件
Step 2: 安全审计
Step 3: 逻辑检查
Step 4: 质量检查
Step 5: 输出报告并验证
```

## 核心工作流

### Step 0：规则注入（🔒 强制 · 不可跳过）

调用 `use_skill("specflow-rules-engine")` → 同步 RULE.mdc → 检测注入 → 兜底。

**播报：** `"✅ [Step 0] 规则注入完成"`

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

[同 v1]
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
depends-on:
  - specflow-rules-engine              # 🔒 强制依赖
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

## v1 → v2 关键变化

| 维度 | v1（旧） | v2（新） |
|------|---------|---------|
| RULE.mdc | ❌ 无 | ✅ 有（alwaysApply: true） |
| Step 0 规则注入 | ❌ 无 | ✅ use_skill("specflow-rules-engine") |
| dependencies | `[]` | `[specflow-rules-engine]` |
| 入口守卫 | ❌ 无 | ✅ 强制步骤执行 |
| 中断恢复 | ❌ 无 | ✅ 扫描 + 播报 + 续接 |
| 每步播报 | ❌ 无 | ✅ 播报完成消息作为检查点 |

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
```

### v2 → v3（契约模式）关键变化

| 维度 | v2 | v3（契约模式） |
|------|----|---------------|
| MCP 调用 | 直接写 `ServerName:tool` | 契约声明 + 获取子流程标准化 |
| 核心逻辑 | 可能含 MCP 工具名 | 零 MCP 工具名，只消费 output |
| MCP 缺失 | skill 可能报错 | 降级链兜底，仍可用 |
| 可移植性 | 绑定特定 MCP | 契约解耦，换 MCP 不改 skill |
