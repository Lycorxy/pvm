---
name: skill-creator-pro
description: >
  创建高质量 Agent Skill，保证触发准确性和稳定性。
  当用户想要创建、优化、验证或调试任何 skill 时调用。
  通过 7 阶段门控工作流 + 4 层验证，确保 skill 具有最优的触发准确性、
  确定性逻辑和最小上下文占用。支持单 skill 创建和多 skill 编排。
  生成的 skill 为"经验步骤器"：每步强制执行、声明所需知识/上下文、
  通过数据契约调用 MCP（知识库/上下文/记忆），MCP 缺失时降级保持独立可用。
  不要用于通用编码任务、git 操作或非 skill 文件编辑。
---

# Skill Creator Pro v3

> 本版核心：skill = 经验步骤器。每步强制执行 + 声明知识需求 + 数据契约调用 MCP + 降级保独立。

## 🛑 入口守卫

```
本 Skill 加载后必须按 Phase 1→...→7 门控工作流执行全部阶段。
每阶段通过后才可进入下一阶段，禁止跳阶段。
如需跳过某阶段，必须逐条给出充足理由：①为什么可跳过 ②跳过不影响最终输出。禁止批量跳过。
```

创建专业级 Agent Skill，具备最优可发现性、确定性执行和最小上下文占用。

## 快速决策树

```
用户需要 skill 相关操作？
├── 创建新 skill？      → Phase 1→7 完整工作流
├── 优化现有 skill？    → Phase 5(验证) → 6(迭代) → 7(交付)
├── 多 skill 系统？     → 添加 config.yaml + 编排器章节
├── 需要 MCP 集成？     → Phase 2 识别契约需求 → Phase 4 设计 step-contracts
└─── 调试触发问题？     → Layer 1 测试 → 修复 description
```

## 7 阶段门控工作流

```
CAPTURE ──→ RESEARCH ──→ ARCHITECT ──→ DRAFT ──→ VALIDATE ──→ ITERATE ──→ DELIVER
```

**规则：** 在任何阶段的退出条件满足之前，不得进入下一阶段。

---

### Phase 1：捕获意图

首先从对话中提取信息，然后追问缺口：

| # | 问题 | 是否必须？ |
|---|------|-----------|
| 1 | **做什么**具体能力？（不是领域名称） | 必须 |
| 2 | **何时**触发？（确切短语、上下文） | 必须 |
| 3 | **何时不**触发？（相似但错误的上下文） | 必须 |
| 4 | 预期输出格式？ | 必须 |
| 5 | 2-3 个真实示例提示词 | 必须 |
| 6 | 单 skill 还是多 skill 编排？ | 多 skill 时必须 |
| 7 | 是否需要 MCP 提供的知识库/上下文/记忆？哪些步骤需要？ | 有 MCP 需求时必须 |

**退出条件：** 用户确认触发词 + 反触发词 + 2 个示例提示词。

### Phase 2：调研

1. 检查工作区中已有 skill 是否有重叠
2. 识别边界情况：畸形输入、缺失依赖、环境差异
3. 决定资源结构：scripts / references / assets
4. 将假设告知用户并确认
5. 确定 **自由度等级** — 参见 [references/freedom-levels.md](references/freedom-levels.md)
6. **识别数据契约需求** — 逐 Step 分析：
   - 该步需要什么知识/上下文/记忆？
   - 本地 references 能否兜底？（能 → 软依赖 `preferred`）
   - 是否为运行时实时数据，本地无法生成？（是 → 硬依赖 `required`）
   - 可用 MCP 服务器有哪些？提供什么工具？
   - 参见 [references/contract-pattern.md](references/contract-pattern.md)

**退出条件：** 边界情况已列出，自由度已决定，结构已规划，契约需求已识别。

### Phase 3：架构设计

**标准结构（必须包含 RULE.mdc）：**
```
skill-name/                   # kebab-case，与 name 字段一致
├── RULE.mdc                  # 🔥 必须：强制规则（alwaysApply: true），注入 System Prompt
├── SKILL.md                  # 必须：frontmatter + 正文（<500 行）
├── skill.config.yaml         # 必须：priority、deps、triggers、step-contracts，depends-on 必须含 specflow-rules-engine
├── scripts/                  # 确定性 CLI 工具（不是库）
├── references/               # 按需加载的文档（仅一层深度）
├── examples/                 # 正向 + 负向触发示例
└── assets/                   # 输出模板、schema
```

**结构规则：**
- 正文 > 500 行 → 拆分到 references/
- 易出错操作 → 打包为 scripts
- 固定输出格式 → 创建 asset 模板
- 多个领域 → 按领域组织 references
- 多 skill → 添加 skill.config.yaml
- references > 100 行 → 在顶部添加目录
- 有 MCP 需求 → skill.config.yaml 必须含 `step-contracts`（见 Phase 4.5）
- ❸ 以上 **RULE.mdc + skill.config.yaml.depends-on** 缺失任意一项 = 结构违规

**退出条件：** 结构设计完成，内容分配决定完毕。必须确认包含 RULE.mdc 和 depends-on 声明。

### Phase 4：起草

#### 4.1 Frontmatter（唯一预加载到全局注册表的内容）

```yaml
---
name: skill-name              # ≤64 字符，仅小写/连字符
description: >                # ≤1024 字符，第三人称，无 XML 标签
  [功能描述]。当 [触发关键词 + 上下文] 时使用。
  不要用于 [反触发场景]。
version: 1.0.0
dependencies: [specflow-rules-engine]  # 🔒 强制依赖：规则注入引擎
conflict-with: []             # 互斥的 skill
allowed-tools: [read_file]    # 显式工具白名单（含 MCP 调用所需工具）
max-token: 1500               # 上下文预算上限
---
```

**description 结构（3 个必需部分）：**

| 部分 | 规则 | 示例 |
|------|------|------|
| **做什么** | 第三人称、现在时 | "执行代码评审，检查 bug..." |
| **何时用** | 触发关键词 + 确切短语 | "当用户说 代码评审、review code..." |
| **何时不用** | 相似但错误的反触发词 | "不要用于编写新代码..." |

**错误示例：** `"代码评审 skill。"`（太模糊，无触发词）
**正确示例：** `"执行代码评审，检查 bug、命名、安全问题。当用户请求 代码评审、review code、检查 bug、优化现有代码时使用。不要用于编写新功能、回答语法问题或审查非代码文档。"`

#### 4.2 正文结构（经验步骤器）

> 完整 SKILL.md 正文模板参见 `references/writing-patterns.md` → **Skill 正文清单** 章节。

SKILL.md 正文必须包含以下组件，以编号祈使句步骤组织：

```
1. 概述（1-2 句话）
2. 何时使用 / 何时不使用
3. 🛑 入口守卫
4. 核心工作流（Step 0 + Step 1...N）
   └─ 每个 Step 含：知识需求声明 + 契约获取子流程 + 核心逻辑
5. 中断与恢复机制
6. 输出格式
7. 错误处理
8. 验证清单
9. 参考资料
```

每个组件的内容规范参考 `references/writing-patterns.md`。

#### 4.3 自由度匹配（关键）

将指令具体程度与任务脆弱性匹配：

| 等级 | 使用场景 | 指令风格 | 示例 |
|-------|---------|---------|------|
| **高** | 多条有效路径，上下文相关决策 | 文本指南、启发式规则 | 代码评审流程 |
| **中** | 存在首选模式，有一定灵活性 | 伪代码、参数化模板 | 报告生成 |
| **低** | 脆弱操作，顺序至关重要 | 确切脚本，最少参数 | 数据库迁移 |

完整指南：[references/freedom-levels.md](references/freedom-levels.md)

#### 4.4 写作规则

- **祈使句**："读取文件..." 而非 "你应该读取..."
- **编号步骤**：严格排序，无歧义
- **一个概念一个术语**：选一个，全篇统一（不要混用 "endpoint"/"URL"/"route"）
- **无魔法数字**：每个值都有文档化理由
- **仅使用正斜杠路径**：`references/guide.md` 绝不 `references\guide.md`
- **无时间信息**：不要写 "2025 年 8 月之后"；改用 "遗留" 章节
- **假设 Claude 很聪明**：只添加 Claude 真正缺乏的上下文
- **❸ MCP 调用只出现在契约获取子流程**：核心逻辑里零个 MCP 工具名

#### 4.5 数据契约设计（有 MCP 需求时必须）

> 完整规范：[references/contract-pattern.md](references/contract-pattern.md)

**核心原则：** skill 依赖"数据契约"，不依赖"MCP 工具"。MCP 是契约的一个实现，缺失时降级。

在 `skill.config.yaml` 的 `step-contracts` 中为每个需要 MCP 的 Step 声明契约：

```yaml
step-contracts:
  - id: code-impact-analysis       # 契约标识
    step: "1.2"                     # 对应步骤号
    criticality: required           # required(硬依赖) | preferred(软依赖)
    input: { change_points: "..." } # 契约输入
    output: { affected_files: "...", impact_level: "..." }  # 注入给 skill 的数据
    providers:                      # 提供者优先级链
      - mcp: { server: "knowledge-base", tool: "analyze_impact", params: { changes: "{input.change_points}" } }
      - builtin: { strategy: "search_content + code-explorer" }
      - fallback: "ask-user"
    on-missing: block               # block | degrade | ask
```

**SKILL.md 正文对应 Step 写法（标准模板）：**

```markdown
### Step 1.2：获取代码影响范围

🔒 数据契约：code-impact-analysis（criticality: required）

#### 契约获取子流程
按 providers 链获取 {affected_files, impact_level}：
  1. MCP knowledge-base:analyze_impact（优先，存在时必调用）
  2. 内置 search_content + code-explorer（降级）
  3. fallback: ask-user（最终）
播报：✅ [Step 1.2·契约] 已就绪（来源：{MCP/内置/用户}）
全部失败且 on-missing=block → ⛔ 阻断

#### 核心逻辑（消费契约数据，零 MCP 工具名）
1. 按 impact_level 分级处理...
2. 播报：✅ [Step 1.2] 完成
```

**设计要点：**
- 软依赖（preferred）：本地 references 能兜底 → `on-missing: degrade`
- 硬依赖（required）：运行时实时数据 → `on-missing: block` 或 `ask`
- providers 至少两级（mcp → builtin/fallback），保证 MCP 挂了仍可用
- 核心逻辑只消费 `output` 字段，不判断数据来源

**退出条件：** SKILL.md 起草完成（<500 行），所有资源已创建，契约已声明。

---

### Phase 5：验证（5 层）

**Layer 1 — 可发现性：**
- 生成 3 个必须触发的提示词（100% 置信度）
- 生成 3 个必须不触发的提示词（相似但错误）
- 如有失败 → 立即修复 frontmatter 中的 description

**Layer 2 — 逻辑：**
- 针对真实测试提示词模拟逐步执行
- 标记 agent 必须"猜测"或解释歧义的任何步骤
- 验证每个错误路径都有显式处理

**Layer 3 — 边界情况：**
- 畸形输入？缺失依赖？部分匹配？
- 操作系统差异（Windows/Linux/macOS）？编码问题？

**Layer 4 — 架构：**
- SKILL.md ≤ 500 行？
- 所有大段内容已卸载到 references/？
- references 是扁平的（无嵌套目录）？
- 所有路径使用 `/`？
- 无 README/CHANGELOG 人工文档？

**Layer 5 — 契约/MCP 解耦验证（有 MCP 需求时必须）：**
- 每个声明了契约的 Step，SKILL.md 正文是否含"契约获取子流程"？
- 核心逻辑里是否出现 MCP 工具名？（必须为零）
- providers 链是否至少两级（mcp + 降级）？
- 硬依赖 on-missing 是否为 block/ask？（不得 degrade）
- 软依赖 on-missing 是否为 degrade？（不得 block）
- 模拟 MCP 不可用场景：skill 是否能通过降级链继续/优雅阻断？
- 模拟 MCP 可用场景：契约获取子流程是否能正确调用 MCP？

详细测试模板：[references/validation-framework.md](references/validation-framework.md)

**退出条件：** 全部 5 层通过。记录任何豁免项及理由。

### Phase 6：迭代

优先级顺序：**关键**（逻辑缺口）→ **必需**（歧义步骤）→ **可选**（风格）

修复后，仅重新运行受影响的验证层。

**Claude 辅助迭代模式**（最有效的方法）：

1. **Claude-A**（本次会话）：起草/完善 skill
2. **Claude-B**（全新会话，已加载 skill）：在真实任务上测试
3. **观察 Claude-B**：它漏掉了什么？在哪里卡住了？意外路径？
4. **返回 Claude-A** 并附上具体观察："做 X 时，它忘记了 Y"
5. **基于行为证据迭代，而非假设**

完整迭代指南：[references/evaluation-driven-dev.md](references/evaluation-driven-dev.md)

**退出条件：** 所有关键 + 必需问题已解决，重新验证通过。

### Phase 7：交付

1. 创建 `.codebuddy/skills/<name>/` 目录
2. 写入所有文件，结构正确
3. 最终清单：
   - [ ] 🔥 RULE.mdc 存在且含 `alwaysApply: true`
   - [ ] 🔥 skill.config.yaml 中 `depends-on` 包含 `specflow-rules-engine`
   - [ ] 🔥 SKILL.md 正文包含「Step 0：规则注入」章节
   - [ ] 🔥 SKILL.md 正文包含「中断与恢复机制」章节
   - [ ] 目录名与 `name` 字段一致（kebab-case）
   - [ ] description 包含触发词 + 反触发词 + 关键词
   - [ ] 正文 ≤ 500 行
   - [ ] 无非 agent 面向的 README/CHANGELOG/docs
   - [ ] 所有路径使用正斜杠 `/`
   - [ ] skill.config.yaml 存在（即使是单 skill）
   - [ ] examples 包含正向 AND 负向触发示例
   - [ ] ❸ 有 MCP 需求时：step-contracts 已声明且 providers≥2 级
   - [ ] ❸ 有 MCP 需求时：核心逻辑零 MCP 工具名（Layer 5 通过）
4. 向用户展示结构 + 快速上手指南

---

## 多 Skill 协作与编排

### skill.config.yaml（多 skill 系统中每个 skill 都必须）

```yaml
# ── 触发与优先级 ──
priority: 80                  # 越高 = 多个匹配时优先
auto-trigger: true            # 关键词匹配时自动激活
trigger-keywords:
  - 代码评审
  - review code
  - 查bug

# ── 依赖关系 ──
depends-on: []                # 在此 skill 之前加载这些
mutual-exclude: [quick-check] # 永不共存的 skill
require-tools: [read_file, write_file]

# ── 生命周期 ──
load-strategy: lazy           # lazy=触发时加载 | eager=预加载
unload-after-run: true        # 执行后释放上下文
max-duration: 120             # 超时秒数

# ── 触发权重（用于冲突解决）──
trigger-weight:
  代码评审: 90
  review代码: 85
  查bug: 75                  # 越低 = 需要更多上下文
```

参见完整模板：`examples/skill.config.template.yaml`

### 协作规则

1. **优先级解决冲突** — 最高优先级在 2+ 个 skill 匹配时激活
2. **depends-on 确保顺序** — 调度器先加载依赖
3. **mutual-exclude 防止冲突** — 互斥 skill 永不共存
4. **lazy load 节省 token** — 仅在实际触发时加载 SKILL.md
5. **unload-after-run 回收** — 任务完成后释放上下文窗口

### 扩展策略

| Skill 数量 | 方法 |
|-------------|------|
| 1-5 | 直接加载，无需编排器 |
| 5-20 | 添加带关键词匹配的编排器 |
| 20-50 | 添加 `trigger-weight` 评分 + 阈值过滤 |
| 50+ | 向量相似度预过滤 → Top-K 召回 |

### 编排器模式（10+ 个 skill）

创建一个轻量级调度 skill，首先运行：

```yaml
name: skill-orchestrator
description: >
  中央 skill 调度器。启动时扫描所有 skill 元数据，
  通过关键词/相似度匹配用户意图，加载最优 skill。
  探测可用 MCP 服务器，按 step-contracts 路由知识获取。
  每次请求时使用，确保正确的 skill 选择。
priority: 0                   # 最低 — 首先运行以选择目标
load-strategy: eager          # 始终加载，最小占用（约 100 行）
```

编排器职责：
- 启动时扫描所有 skill 元数据
- 构建 keyword→skill 索引
- 每次用户请求：按 priority + weight 排序候选
- 处理 depends-on / mutual-exclude 解决
- 强制执行总 token 预算（防止溢出）
- **探测可用 MCP 服务器，构建能力清单**
- **读取各 skill 的 step-contracts，构建契约路由表**
- **skill 执行某 Step 前，按 providers 链预先获取数据注入**
- **在关键节点调用 MCP 存记忆（skill 不感知）**

---

## 数据契约模式（MCP 集成标准）

> 完整规范：[references/contract-pattern.md](references/contract-pattern.md)

让 skill 能调用 MCP 提供的知识库/上下文/记忆，同时保持 skill 独立可用。

**核心原则：** skill 依赖"数据契约"，不依赖"MCP 工具"。MCP 是契约的一个实现。

```
skill 声明：Step N 需要 {contract-id} 数据（契约）
skill 不写：Step N 调用 mcp__xxx__yyy（实现绑定）

MCP 存在 → 契约获取子流程调用 MCP，注入数据给 skill
MCP 缺失 → 降级到内置工具 / 本地 references / 问用户
```

**两种依赖：**
| 类型 | criticality | on-missing |
|------|-------------|------------|
| 软依赖（增强型） | preferred/optional | degrade（本地兜底） |
| 硬依赖（数据输入型） | required | block（阻断）或 ask（问用户） |

**契约获取子流程（skill 正文唯一允许 MCP 调用的地方，标准化）：**
1. 按 providers 链尝试：mcp → builtin → fallback
2. MCP 可用 → 调用 `mcp__{server}__{tool}({params})` → 数据就绪
3. MCP 不可用 → 降级 builtin / fallback
4. 播报来源；全部失败 → 执行 on-missing 策略

**铁律：**
- 核心逻辑里**零个 MCP 工具名**，只消费契约 output 字段
- providers 至少两级（mcp + 降级），保证 MCP 挂了仍可用
- 硬依赖不得 degrade（缺关键数据会出错）
- 软依赖不得 block（MCP 挂了就阻断，过度刚性）

---

## 高级模式

针对需要专门行为的复杂 skill：

| 模式 | 使用场景 | 参见 |
|------|---------|------|
| **反馈循环** | 有关键质量验证器的任务 | [references/advanced-patterns.md](references/advanced-patterns.md) |
| **条件详情** | 基础内容 + 高级主题链接 | [references/advanced-patterns.md](references/advanced-patterns.md) |
| **视觉分析** | 输入可渲染为图像 | [references/advanced-patterns.md](references/advanced-patterns.md) |
| **可验证中间输出** | 复杂批量操作、高风险变更 | [references/advanced-patterns.md](references/advanced-patterns.md) |
| **数据契约/MCP 集成** | skill 调用 MCP 知识库/上下文/记忆 | [references/contract-pattern.md](references/contract-pattern.md) |

---

## 反模式快速参考

| 反模式 | 修复 |
|--------|------|
| 模糊 description（"帮助写代码"） | 3 部分：做什么 + 何时触发 + 何时不触发 |
| 散文式指令（"你可以..."） | 编号祈使句步骤 |
| 正文 > 500 行 | 卸载到 references/ |
| 嵌套 reference 目录（`ref/a/b.md`） | 扁平结构，仅一层深度 |
| scripts/ 中有库代码 | scripts = 小型 CLI 工具 only |
| 存在 README/CHANGELOG | 删除所有人工面向文档 |
| 缺失错误处理 | 每个步骤有显式错误路径 + 恢复操作 |
| Windows 反斜杠路径 | 仅正斜杠 `/` |
| 无验证/清单 | 在工作流末尾添加清单 |
| 过度工程化结构 | 满足需求的最简方案 |
| 缺失 config.yaml | 每个 skill 都获得 config 以备未来扩展 |
| ❸ **缺失 RULE.mdc** | 必须创建 RULE.mdc（alwaysApply: true） |
| ❸ **缺失 depends-on: [specflow-rules-engine]** | config.yaml 必须声明依赖 |
| ❸ **缺失 Step 0 规则注入** | SKILL.md 必须包含规则注入步骤 |
| ❸ **无中断恢复机制** | SKILL.md 必须包含中断恢复章节 |
| 脚本中的魔法数字 | 记录每个常量的理由 |
| 时间信息（"v2 之后"） | 改用遗留/弃用章节 |
| 术语混用 | 选一个术语，全篇统一 |
| 假设包已安装 | 显式列出依赖并附安装命令 |
| ❸ **核心逻辑里出现 MCP 工具名** | 移到契约获取子流程，核心逻辑只消费 output |
| ❸ **契约 providers 只有一级（仅 mcp）** | 至少 mcp + builtin/fallback 两级 |
| ❸ **硬依赖用 on-missing: degrade** | 改为 block 或 ask |
| ❸ **软依赖用 on-missing: block** | 改为 degrade |

---

## 质量评分卡

| 维度 | 优秀 (5/5) |
|------|------------|
| **可发现性** | 触发词 + 反触发词 + 确切关键词 + 正向/负向示例 |
| **确定性** | 入口守卫 + 所有步骤编号 + 决策树 + 零歧义词汇 + 每步播报检查点 |
| **上下文效率** | 元数据精简，正文 <500 行，资源懒加载，RULE.mdc 替代长文本 |
| **错误处理** | 每个步骤有显式错误路径 + 恢复操作 |
| **可验证性** | 清单 + 每阶段退出条件 + 评估模板 |
| **协作就绪** | config.yaml 完整，deps 含 specflow-rules-engine，mutual-exclude 已定义 |
| **执行力** | 规则注入（Step 0）+ 任务式流程 + 中断恢复机制 |
| **自由度匹配** | 指令与任务脆弱性等级匹配（高/中/低） |
| **契约解耦** | step-contracts 声明完整 + providers≥2 级 + 核心逻辑零 MCP 名 + 降级链可用 |

---

## 文件参考

| 文件 | 内容 |
|------|------|
| [references/freedom-levels.md](references/freedom-levels.md) | 自由度匹配指南，含决策矩阵 |
| [references/writing-patterns.md](references/writing-patterns.md) | 正文模板、输出格式、决策树、步骤器写法 |
| [references/contract-pattern.md](references/contract-pattern.md) | 数据契约完整规范、MCP 集成标准、示例 |
| [references/validation-framework.md](references/validation-framework.md) | 完整 5 层验证测试提示词和清单 |
| [references/evaluation-driven-dev.md](references/evaluation-driven-dev.md) | 评估驱动开发和 Claude-A/B 迭代 |
| [references/advanced-patterns.md](references/advanced-patterns.md) | 反馈循环、视觉分析、条件详情 |
| [examples/skill.config.template.yaml](examples/skill.config.template.yaml) | 可直接复制的 config 模板（含契约） |
| [examples/complete-skill-example.md](examples/complete-skill-example.md) | 完整可用 skill 示例（含契约） |
