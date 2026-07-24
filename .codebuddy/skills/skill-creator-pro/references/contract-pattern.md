# 数据契约模式（MCP 集成标准）

让 skill 能调用 MCP 提供的知识库/上下文/记忆，同时保持 skill 独立可用的标准机制。

## 目录
- [核心原则](#核心原则)
- [两种依赖类型](#两种依赖类型)
- [契约格式](#契约格式)
- [契约获取子流程（标准化）](#契约获取子流程标准化)
- [on-missing 策略](#on-missing-策略)
- [编排器适配职责](#编排器适配职责)
- [完整示例](#完整示例)
- [反模式](#反模式)

---

## 核心原则

> **skill 依赖"数据契约"，不依赖"MCP 工具"。MCP 只是契约的一个实现。**

```
skill 写：Step N 需要 {contract-id} 数据（契约声明）
skill 不写：Step N 调用 mcp__xxx__yyy（实现绑定）

MCP 存在 → 契约获取子流程调用 MCP，注入数据给 skill
MCP 缺失 → 降级到内置工具 / 本地 references / 问用户
```

skill 正文里**零个 MCP 工具名**。所有 MCP 调用集中在"契约获取子流程"中，按 providers 优先级链执行。

---

## 两种依赖类型

| 类型 | criticality | 特征 | on-missing |
|------|-------------|------|------------|
| **软依赖（增强型）** | `preferred` / `optional` | 本地有基础版，MCP 有更好版 | `degrade`（降级用本地） |
| **硬依赖（数据输入型）** | `required` | 没这数据 skill 无法正确执行 | `block`（阻断）或 `ask`（问用户） |

判断方法：
- 这个知识/数据，本地 references 或内联能否兜底？能 → 软依赖
- 这是运行时实时数据（影响分析、历史记忆、跨项目上下文），本地无法生成？→ 硬依赖

---

## 契约格式

在 `skill.config.yaml` 的 `step-contracts` 中声明：

```yaml
step-contracts:
  - id: code-impact-analysis          # 契约唯一标识（kebab-case）
    step: "1.2"                        # 对应 SKILL.md 的步骤号
    name: "获取代码影响范围"            # 人类可读名称
    criticality: required              # required | preferred | optional
    input:                             # 契约输入（skill 已有的数据）
      change_points: "本次变更点列表"
    output:                            # 契约输出（注入给 skill 的数据结构）
      affected_files: "受影响文件列表"
      impact_level: "high | medium | low"
      call_chain: "调用链路"
    providers:                         # 提供者优先级链（从上到下尝试）
      - mcp:                           # 优先级 1：MCP（存在时必调用）
          server: "knowledge-base"
          tool: "analyze_impact"
          params:                      # 参数可用 {input.xxx} 引用契约输入
            changes: "{input.change_points}"
      - builtin:                       # 优先级 2：内置工具降级
          strategy: "search_content + code-explorer"
          description: "搜索调用点 + 子代理扫调用链"
      - fallback: "ask-user"           # 优先级 3：最终降级
    on-missing: block                  # block | degrade | ask

  - id: historical-decisions
    step: "1.3"
    name: "获取同类需求历史决策"
    criticality: preferred
    input:
      requirement_keywords: "需求关键词"
    output:
      past_decisions: "历史决策记录"
      pitfalls: "踩坑记录"
    providers:
      - mcp:
          server: "memory-store"
          tool: "recall"
          params:
            query: "{input.requirement_keywords}"
      - fallback: "local"              # 降级到本地 references
    on-missing: degrade
    degrade-source: "references/pitfalls.md"
```

### providers 字段说明

| 字段 | 含义 | 必填 |
|------|------|------|
| `mcp` | MCP 调用：`server` + `tool` + `params` | 否（但有 MCP 时首选） |
| `builtin` | 内置工具策略（search_content / code-explorer 等） | 否 |
| `fallback` | 最终降级：`ask-user` / `local` / `none` | 否 |

`params` 中可用 `{input.xxx}` 引用契约 `input` 字段，执行时由契约获取子流程替换。

---

## 契约获取子流程（标准化）

每个声明了契约的 Step，在执行核心逻辑**之前**必须运行此子流程。这是 skill 正文里唯一允许出现 MCP 调用的地方，且格式标准化、可复用。

```
对于本 Step 声明的每个契约 contract：
  1. 按 providers 顺序尝试获取数据：
     ├─ providers[0] = mcp ?
     │   ├─ 探测 MCP server 是否可用
     │   ├─ 可用 → 调用 mcp__{server}__{tool}({params})
     │   │        → 成功 → 数据就绪，记录来源=MCP，结束
     │   │        → 失败 → 降级 providers[1]
     │   └─ 不可用 → 降级 providers[1]
     ├─ providers[1] = builtin ?
     │   └─ 按 strategy 执行内置工具 → 成功 → 数据就绪，来源=内置
     └─ providers[2] = fallback ?
         ├─ ask-user → 向用户询问所需数据
         ├─ local → read_file(degrade-source)
         └─ none → 无数据
  2. 播报：
     ✅ [Step N·契约] {contract-id} 已就绪（来源：MCP / 内置 / 本地 / 用户）
  3. 若全部 provider 失败 → 执行 on-missing 策略
```

### SKILL.md 正文写法（标准模板）

```markdown
### Step 1.2：获取代码影响范围

🔒 数据契约：code-impact-analysis（criticality: required）
   详见 skill.config.yaml → step-contracts

#### 契约获取子流程
按 providers 链获取 {affected_files, impact_level, call_chain}：
  1. MCP knowledge-base:analyze_impact（优先）
  2. 内置 search_content + code-explorer（降级）
  3. fallback: ask-user（最终）
播报：✅ [Step 1.2·契约] code-impact-analysis 已就绪（来源：{MCP/内置/用户}）
若全部失败且 on-missing=block → ⛔ 阻断，不得继续

#### 核心逻辑（消费契约数据）
1. 按 impact_level 分级处理：
   - high → 必须出迁移方案
   - medium → 标注风险点
   - low → 记录即可
2. 播报：✅ [Step 1.2] 影响分析完成，受影响 N 个文件
```

**关键**：核心逻辑只消费契约 output 字段，不感知数据来源。

---

## on-missing 策略

| 策略 | 行为 | 适用 |
|------|------|------|
| `block` | 阻断当前 Step，播报 ⛔ 缺失必需契约，不得继续 | 硬依赖，缺数据会出错 |
| `degrade` | 用 `degrade-source` 指定的本地文件兜底，继续执行 | 软依赖，有本地基础版 |
| `ask` | 向用户询问所需数据，用户给出后继续 | 可交互场景 |

---

## 编排器适配职责

多 skill 系统中，编排器（orchestrator）是唯一全局感知 MCP 的角色：

```
编排器做什么：
  ✓ 启动时探测可用 MCP 服务器，构建能力清单
  ✓ 读取各 skill 的 step-contracts，构建契约路由表
  ✓ skill 执行某 Step 前，按 providers 链预先获取数据注入
  ✓ 在 Step 完成时，调用 MCP 存记忆（skill 不感知）
  ✓ skill 结束时，存会话摘要

编排器不做什么：
  ✗ 不修改 skill 的执行逻辑与领域判断
  ✗ 不让 skill 感知编排器存在
  ✗ 不让 skill 正文出现 MCP 工具名
```

**无编排器时（单 skill）**：skill 自身的"契约获取子流程"承担获取职责，直接探测并调用 MCP。skill 仍独立可用。

---

## 完整示例

需求分析 skill 调用 MCP 获取影响分析 + 历史记忆：

### skill.config.yaml（契约声明）

```yaml
step-contracts:
  - id: code-impact-analysis
    step: "1.2"
    name: "获取代码影响范围"
    criticality: required
    input:
      change_points: "变更点列表"
    output:
      affected_files: "受影响文件列表"
      impact_level: "影响等级"
    providers:
      - mcp:
          server: "knowledge-base"
          tool: "analyze_impact"
          params:
            changes: "{input.change_points}"
      - builtin:
          strategy: "search_content + code-explorer"
      - fallback: "ask-user"
    on-missing: block

  - id: historical-decisions
    step: "1.3"
    name: "获取同类需求历史决策"
    criticality: preferred
    input:
      requirement_keywords: "需求关键词"
    output:
      past_decisions: "历史决策记录"
      pitfalls: "踩坑记录"
    providers:
      - mcp:
          server: "memory-store"
          tool: "recall"
          params:
            query: "{input.requirement_keywords}"
      - fallback: "local"
    on-missing: degrade
    degrade-source: "references/pitfalls.md"

memory-hooks:
  on-step-complete:
    - event: "design-decision-made"
      mcp-store:
        server: "memory-store"
        tool: "save_memory"
  on-skill-end:
    - event: "session-summary"
      mcp-store:
        server: "memory-store"
        tool: "save_session"
```

### SKILL.md 正文（零 MCP 工具名）

```markdown
### Step 1.2：获取代码影响范围

🔒 数据契约：code-impact-analysis（required）

#### 契约获取子流程
按 providers 链获取 {affected_files, impact_level}。
播报来源。失败且 on-missing=block → 阻断。

#### 核心逻辑
1. 按 impact_level 分级...
2. 播报完成

### Step 1.3：获取历史决策

🔒 数据契约：historical-decisions（preferred）

#### 契约获取子流程
按 providers 链获取 {past_decisions, pitfalls}。
MCP 不可用 → read_file(references/pitfalls.md) 兜底。

#### 核心逻辑
1. 对比当前需求与历史决策找差异...
2. 播报完成（来源：MCP / 本地兜底）
```

---

## 反模式

| 反模式 | 问题 | 正确做法 |
|--------|------|----------|
| skill 正文写 `mcp__xxx__yyy()` | 绑定 MCP 实现，不可移植 | 声明契约，获取子流程标准化调用 |
| 契约无 providers 降级链 | MCP 挂了 skill 就废 | 至少 mcp → builtin/fallback 两级 |
| 硬依赖用 on-missing: degrade | 缺关键数据仍跑，出错 | 硬依赖用 block 或 ask |
| 软依赖用 on-missing: block | MCP 挂了就阻断，过度刚性 | 软依赖用 degrade |
| 核心逻辑里判断数据来源 | 逻辑与实现耦合 | 核心逻辑只消费 output 字段 |
| 契约 output 字段模糊（"一些数据"） | skill 无法稳定消费 | 明确字段名 + 类型描述 |
| params 里硬编码而非 {input.xxx} | 无法复用 | 用 {input.xxx} 引用契约输入 |
