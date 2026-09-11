# GateLoop 引擎用法指南（门控闭环引擎）

> 本文讲清 GateLoop 引擎的**完整用法**：三支柱思想、两层架构（小 skill vs 大流程）、三层次使用（单功能/流程/命令）、Manifest/Anchor 字段、verify 门禁、通用生命周期钩子（含 onEvent 事件流）、边界（开发测试自动/发布人工）、如何改造现有 skill。
>
> GateLoop 源码：`src/gateloop/`。引擎入口类为 `GateLoop`（由历史代号 `Ads` 重命名而来）。任何 skill 通过声明 `AnchorManifest` 即可获得 GateLoop 能力——**门控推进 + 验证判定 + 数据/上下文内联 + 防信息衰减 + 事件通知**。
>
> **执行环境（二分）**：`nut gateloop` CLI 由 npm 包 `ai-ultra` 提供。
> - **在 ai-ultra 本仓库内**：直接从本地构建执行——`node dist/cli/index.cjs gateloop init/next/status <manifest>`，或 npm scripts（`guard` / `guard:status` / `guard:retry` / `guard:reset`）；改过 `src/` 先 `npm run build`。**禁止在本仓库内再从 npm 安装 ai-ultra。**
> - **在用户目标工作区**：未安装时**先装到本地** devDependency：`npm i -D ai-ultra`（pnpm：`pnpm add -D ai-ultra`；yarn：`yarn add -D ai-ultra`；裸目录先 `npm init -y`），验证 `npx nut gateloop --help`。禁止 `-g` 全局安装——版本随项目锁定，多项目互不污染。npm `latest` tag 即当前版本（0.0.0-beta.8），无需指定 beta tag。

---

## 目录
- [核心定位：三支柱思想](#核心定位三支柱思想)
- [边界：开发测试自动，发布人工](#边界开发测试自动发布人工)
- [两层架构：小 skill vs 大流程](#两层架构小-skill-vs-大流程)
- [三层次使用：单功能 / 流程 / 命令](#三层次使用单功能--流程--命令)
- [决策树：我该用哪一层？](#决策树我该用哪一层)
- [AnchorManifest 字段](#anchormanifest-字段)
- [AnchorDefinition 字段](#anchordefinition-字段)
- [verify 门禁（4 阶段 + 3 前置门禁）](#verify-门禁4-阶段--3-前置门禁)
- [通用生命周期钩子 AnchorHooks](#通用生命周期钩子-anchorhooks)
- [统一事件流 onEvent（通知钩子）](#统一事件流-onevent通知钩子)
- [四条信息管道（防衰减）](#四条信息管道防衰减)
- [如何改造现有 skill 到 GateLoop harness 模式](#如何改造现有-skill-到-gateloop-harness-模式)
- [完整示例](#完整示例)

---

## 核心定位：三支柱思想

GateLoop 的本质是一句话：

```
引擎只信「脚本 exit code」，不信「模型说做完了」。
模型能做的只有"声称做完并调 next"，但能否推进由 verify 的 exit code 说了算。
```

它由**三个不可分割的支柱**构成：

```
① 门控（Gate）    —— 每阶段有客观验证门，exit code 守着，"确定的交给脚本"
② 闭环（Loop）    —— 失败重试 + 熔断回滚 + 环检测，"执行-验证-修复"逼收敛
③ 底座（Harness） —— 引擎强制驱动 + 数据自动内联，不信模型自述，防信息衰减
```

- **确定的**（能编译 / 无 any / 文件 ≤ 360 行 / 产物存在 / schema 字段齐全）→ 脚本 exit code，模型改不了
- **理解的**（拆分合不合理 / 数据流优不优雅）→ `review.model` 结构化评审，模型评 + 脚本卡形式（逐条评 + 带证据 + fail 必改）
- **纯审美**（视觉效果 / 品牌感）→ 完全交给模型，不进 verify

命名关系：**GateLoop 是引擎名，Anchor（锚点）是它的核心数据结构名**——锚点描述"把模型执行锚定在可验证步骤上"的机制，闭环描述"执行-验证-修复"的运行哲学，两者互补不冲突。

> 5 类分工判定表见 [floor-ceiling-guide.md](floor-ceiling-guide.md)。

---

## 边界：开发测试自动，发布人工

GateLoop 自动闭环有明确边界——它只覆盖**开发 + 测试**这段，到发布就停下：

```
需求 → 设计 → 开发 → 验证 → 测试 → 验证 ──→ 停在这里，交给人
        └──────── 全自动闭环（GateLoop）────────┘   └─ 人工手动发布 ─┘
                                                    （发布才知道对不对）
```

**为什么发布不进自动闭环？**

```
开发测试可以自动  → 脚本能验证对不对（tsc/lint/test exit code）→ 交给脚本门控
发布才能知道对不对 → 脚本验证不了"生产环境对不对" → 交给人的经验
生产环节人工手动  → 不可逆 + 无法脚本验证 → 交给人的判断
```

三层分工哲学至此完整闭合：

```
脚本（确定的）  → exit code 门禁，自动 —— 验证能不能跑通
模型（理解的）  → 写代码、做设计，自动 —— 把需求变成代码
人（不可逆的）  → 发布上线，手动 —— 只有人知道"对不对"
```

**落地**：GateLoop 的 manifest 到**测试通过**就结束（`done=true`），发布不是 manifest 里的 anchor。引擎跑完 test，输出"可发布"状态 + 完整审计链，然后停，人手动发布。

---

## 两层架构：小 skill vs 大流程

GateLoop 引擎同一套状态机/verify/钩子，分两层使用，**不换引擎、不换核心类**：

| | Anchor 层（小 skill） | Flow 层（大流程） |
|---|---|---|
| 配置文件 | `anchor.config.json` | `flow.config.json` |
| 解决的问题 | 单个 skill 内部防跳过步骤 | 多 skill / 多 agent 协作防信息衰减 |
| anchors 是什么 | 方法论骨架（design → code → verify） | 流程阶段（需求 → 设计 → 开发 → 质检） |
| 关键字段 | `brief` + `produces` + `verify` | 额外用 `delegateSkill` / `readsMemory` / `writesMemory` / `readsContext` / `onEscalate` |
| 核心诉求 | "确定的交给脚本，理解的交给模型" | "流程完美协调、上下文不衰减、步骤不跳过" |

**关键区分：**

```
Anchor 层（小 skill）
  例：frontend-codegen 内部 design→code→verify 三个 anchor
  每个 anchor = 一个强制步骤，verify 门禁防模型跳步

Flow 层（大流程）
  例：一个工程化流程 需求→设计→开发→质检 四个 anchor（无 deploy，发布人工）
  每个 anchor 用 delegateSkill 委派一个子 skill
  用 readsContext 读上游上下文存档（防衰减）
  用 writesMemory 回写决策（跨任务）
  用 onEscalate 回滚（质检失败 → 回开发）
```

---

## 三层次使用：单功能 / 流程 / 命令

GateLoop 是 skill 的执行底座，同一底座有三种使用层次（不是三个东西）：

```
                    ┌─────────────────────────────────┐
                    │   GateLoop 底座（门控+闭环+内联）   │
                    └───────────────┬─────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        ↓                           ↓                           ↓
  层次1：做单一功能            层次2：做流程                层次3：当命令
  （根·最常用）              （扩展·最大价值）            （入口·操作形态）
        │                           │                           │
  anchor.config.json         flow.config.json            nut gateloop init/next/...
  单 skill 内部阶段           跨多 skill 编排             手动驱动/审计/续跑
  门控闭环防「跳步」          上下文防「衰减」             断点恢复/状态查询
```

- **做单一功能**（根）：一个 skill 内部把方法论拆成阶段，门控防跳步——"确定的交给脚本"首先在单 skill 内部落地
- **做流程**（扩展）：多 skill 接力，跨 skill 上下文防衰减——这是单一 skill 做不到的
- **当命令**（入口）：`nut gateloop` CLI 驱动，手动推进/审计/断点续跑——不管做单功能还是做流程，最终都通过命令驱动

---

## 决策树：我该用哪一层？

```
拿到一个 skill 需求，先问：这是一个领域，还是一条跨领域流水线？

├── 单一领域、单一 skill 内部的方法论
│   → Anchor 层（小 skill，anchor.config.json）
│   → 例：css-style-writer（写样式）、code-quality-guard（质检）
│   → anchors = 该领域的固定步骤（如：扫描→设计→产出）
│
├── 跨多个 skill / 多个 agent 协作的工程化流水线
│   → Flow 层（大流程，flow.config.json）
│   → 例：前端从需求到可发布代码的全流程（发布人工，不进流程）
│   → 每个 anchor 用 delegateSkill 委派子 skill
│   → 用 readsContext / writesMemory / onEscalate 做跨 skill 协调
│
└── 既是单 skill 又有内部阶段？
    → 仍然 Anchor 层。Flow 层只在「跨 skill」时用。
```

**判断口诀：**
- 一个 skill 自己就能干完 → Anchor 层
- 需要 2+ 个 skill 接力 + 上下文要跨 skill 传递 → Flow 层

---

## AnchorManifest 字段

```typescript
export interface AnchorManifest {
    name: string                    // skill 名称
    description: string
    anchors: AnchorDefinition[]     // 阶段列表，按执行顺序
    stateFile?: string              // 默认 .specflow/anchor-state.json
    logFile?: string                // 默认 .specflow/anchor-log.jsonl
    scratchDir?: string             // 默认 .specflow/scratch
    scratchGcHours?: number         // 默认 72
    maxAttempts?: number            // 单阶段最大重试，默认 5
    cwd?: string                    // 默认 process.cwd()
    memory?: MemoryConfig           // 决策记忆配置（跨任务）
    maxJumps?: number               // 非线性跳转环检测阈值，默认 3
}

export interface MemoryConfig {
    localStore?: string             // 默认 .specflow/memory/store.json
    mcp?: boolean                   // 是否接 MCP（未注入降级本地）
}
```

---

## AnchorDefinition 字段

```typescript
export interface AnchorDefinition {
    id: string                      // 唯一标识（design / code / verify）
    name?: string                   // 人类可读名
    brief?: string                  // 给 LLM 的简报（目标+约束）

    // —— 输入内联（引擎读进来给模型，模型不用自己 read）——
    reads?: string[]                // 规则文件路径，内容内联进指令
    consumes?: string[]             // 依赖上游 artifact key，内容内联
    readsMemory?: string[]          // 读决策记忆 key（精确命中，非语义搜索）
    readsContext?: string[]         // 读上游 anchor 的上下文存档（防衰减）★

    // —— 输出约束（引擎校验，模型必须产出）——
    produces?: string[]             // 必须产出的文件（glob）
    emits?: string[]                // 必须产出的 artifact key
    writesMemory?: string[]         // 必须回写的决策记忆 key（门禁）

    // —— 执行控制 ——
    allowedTools?: string[]         // 工具白名单
    maxAttempts?: number            // 覆盖 manifest.maxAttempts
    requiresSubtasks?: boolean      // 必须拆子任务（全绿才推进）
    subagentPrompt?: string         // 派 code-explorer 子上下文
    delegateSkill?: string          // 委派子 skill（Flow 层用）★

    // —— 验证 ——
    verify?: AnchorVerify

    // —— 非线性跳转（Flow 层用）★ ——
    onPass?: string                 // 验证通过后跳转目标（默认下一个）
    onEscalate?: string             // 熔断后跳转目标（回滚/降级）
}
```

---

## verify 门禁（4 阶段 + 3 前置门禁）

### verify() 内部 4 阶段（AND 关系，全过才 PASS）

```
阶段0  配置自检
  └ modelWritable:false 必须配 produceScripts

阶段1  产出型脚本 produceScripts（串行，先于文件类）
  └ 脚本产出 produces 声明的文件 → autoEmit 自动产出 artifact

阶段2  文件类校验（串行·快速失败）
  ├ producesExist   产物文件存在
  ├ producesFresh   产物 mtime > 本轮开始（防复用旧文件）
  └ schema          JSON Schema 校验产物结构

阶段3a 修复类（同步）
  └ lintFix         ESLint 自动修复

阶段3b 命令类校验（并行）
  ├ lint / tsc / stylelint / build / test
  ├ fileSize        行数/函数行数/圈复杂度
  ├ prettier        格式
  └ custom          自定义命令（exit 0 = 通过）

阶段3c 结构化质量评审 review
  ├ ① 完整性    结论覆盖全部 checklist id
  ├ ② script防伪 引擎重跑 command 核对（不信模型）
  ├ ③ model要证据 evidence 含行号/符号引用
  └ ④ 阻断判定   blocking 项 fail → 整体 FAIL
```

### doNext 的 3 前置门禁

```
门禁1  子任务闭环   requiresSubtasks 的阶段，子任务未全完成不推进
门禁2  emits 校验   声明的 artifact 未产出不推进
门禁3  writesMemory 声明的决策记忆未回写不推进
```

### AnchorVerify 字段

```typescript
export interface AnchorVerify {
    schema?: JSONSchema
    lint?: boolean | { files?: string[] }
    lintFix?: boolean | { files?: string[] }
    tsc?: boolean
    stylelint?: boolean | { files?: string[] }
    build?: boolean | { command?: string }
    test?: boolean | { command?: string }
    custom?: string | string[]
    producesExist?: boolean
    producesFresh?: boolean
    fileSize?: boolean | { maxFileLines?; maxFunctionLines?; maxComplexity? }
    prettier?: boolean | { files?: string[] }
    review?: ReviewConfig          // checklist 由 skill 注入
    produceScripts?: string[]      // 产出型脚本（先于文件类）
    autoEmit?: Record<string, string>  // 脚本成功后自动 emit
    modelWritable?: boolean        // false = 产物由脚本产出
}
```

---

## 通用生命周期钩子 AnchorHooks

**定位**：GateLoop 引擎的统一接入点。记忆系统、上下文系统、知识库、figma-to-code、代码影响分析等**所有外部能力**，都通过这些钩子接入流程，保证多 skill / 多 agent 协作时信息不衰减。

```typescript
export interface AnchorHooks {
    /** anchor 开始前调用，返回值内联进指令（注入类） */
    beforeAnchor?: (ctx: HookContext) => Promise<ContextItem[] | undefined>
    /** anchor 完成后调用（verify 通过后），供外部存档（存档类） */
    afterAnchor?: (ctx: HookContext, result: VerifyResult, archive: ContextArchive) => Promise<void>
    /** 统一事件流（通知类），见下一节 */
    onEvent?: (event: AnchorEvent) => Promise<void>
}

export interface ContextItem {
    source: string      // 来源标记（impact-analysis / knowledge-base / figma / memory）
    content: string     // 内容（内联进指令）
    kind?: string       // 类型标签
}

export interface HookContext {
    anchorId: string
    anchorName?: string
    cwd: string
    state: AnchorState
    artifacts: Record<string, Artifact>
    anchor: AnchorDefinition
}

export interface ContextArchive {
    anchorId: string
    anchorName?: string
    consumed: string[]      // 消费了哪些 artifact
    emitted: string[]       // 产出了哪些 artifact
    produced: string[]      // 产出了哪些文件
    verifyChecks: VerifyCheck[]
    completedAt: string
}
```

**三种钩子各有职责，互不重叠：**

| 钩子 | 类型 | 职责 | 能否改流程 |
|---|---|---|---|
| `beforeAnchor` | 注入类 | 返回 ContextItem 内联进指令（影响分析/知识库/figma） | 能（注入上下文） |
| `afterAnchor` | 存档类 | 接收 archive 供外部存档（经验记忆/知识库更新） | 否（事后存档） |
| `onEvent` | 通知类 | 只读推送流程进度/异常（发消息/写监控/调接口） | 否（纯通知） |

**注入方式**（与 MemoryHooks 一致，可选增强）：

```typescript
const gateLoop = new GateLoop(manifest, memoryHooks, {
    async beforeAnchor(ctx) {
        // 可做：代码影响分析、知识库检索、figma 解析、经验记忆语义搜索
        return [{ source: 'impact-analysis', content: '改 auth.ts 影响 5 个文件...' }]
    },
    async afterAnchor(ctx, result, archive) {
        // 可做：经验记忆写入、知识库更新、上下文存到外部系统
    }
})
```

**设计原则：**
- **可选注入**：不注入 `anchorHooks` 时引擎降级跳过，skill 独立可用
- **不绑定特定服务**：上层注入什么系统就接什么系统
- **引擎不含业务语义**：钩子做什么由上层决定，引擎只管调用 + 内联 + 存档 + 通知

---

## 统一事件流 onEvent（通知钩子）

`onEvent` 是 GateLoop 的**统一通知机制**——外部系统实现这一个接口，即可实时追踪"每个节点执行到哪、是否完成、是否出错"。

### 5 种事件类型

| 事件 | 触发时机 | 业务用途 |
|---|---|---|
| `stage_start` | 进入新节点 | "现在执行到哪个节点了" |
| `stage_verify_fail` | 节点验证失败（将重试） | "出错了，卡在哪个节点" |
| `stage_complete` | 节点完成并推进 | "这个节点做完了，下一个是 X" |
| `escalate` | 熔断（含回滚跳转） | "连续失败，需要人工介入" |
| `done` | 流程全部完成 | "整个流程结束了" |

### 事件结构

```typescript
export type AnchorEventType =
    | 'stage_start' | 'stage_verify_fail' | 'stage_complete' | 'escalate' | 'done'

export interface AnchorEvent {
    type: AnchorEventType
    stageId: string          // 当前节点 id
    stageName?: string       // 节点名称
    timestamp: string
    attempts?: number        // 尝试次数
    errors?: string[]        // 错误信息
    nextStage?: string       // 推进到的下一阶段
}
```

### 关键设计

- **fire-and-forget**：通知失败（钩子抛错/系统挂了）静默吞掉，绝不阻断主流程
- **纯通知（只读）**：外部不能通过 onEvent 改流程，只能"观察"
- **业务可后续补充**：不改引擎，只要实现 onEvent 接口，就能接任意通知

### 使用示例

```typescript
const gateLoop = new GateLoop(manifest, undefined, {
    async onEvent(event) {
        // 业务方补充通知：发消息 / 写监控 / 调接口 / 调 MCP
        if (event.type === 'escalate' || event.type === 'stage_verify_fail') {
            notifyTeam(`${event.stageId} 出错: ${event.errors?.join('; ')}`)
        }
        if (event.type === 'done') {
            notifyTeam('流程完成，可人工发布')
        }
    }
})
```

---

## 四条信息管道（防衰减）

多 skill 协作时信息衰减的根本原因：每经过一个 skill，上下文只传"文件内容"，"为什么这么做""验证了什么"会丢。GateLoop 用四条管道解决：

```
管道1  artifact         → 传「结论」（文件内容/值）        consumes/emits
管道2  reads            → 传「规则」（静态事实）          reads → rulesInline
管道3  memory           → 传「决策」（跨任务精确 key）     readsMemory/writesMemory
管道4  contextArchive   → 传「上下文链」（防衰减）★       readsContext → contextArchiveInline
+      AnchorHooks      → 调用前注入/调用后存档/事件通知   beforeAnchor/afterAnchor/onEvent ★
```

**关键机制：**
- `readsContext` 让下游 anchor 读到上游的**完整执行上下文**（consumed/emitted/verifyChecks），不只文件内容
- `beforeAnchor` 注入影响分析等动态上下文
- `onEvent` 把流程进度/异常实时推给外部（发消息/写监控）
- 存档是引擎自动写的本地 JSONL（`.specflow/context/archive.jsonl`），不依赖 MCP

---

## 16. 完整示例：Anchor 层 4 阶段（以 figma-dom-style 为蓝本）

`figma-dom-style`（Figma 设计稿还原）是 Anchor 层多 anchor 门控的典型实现，完整展示了本指南前述全部机制。完整 JSON 亦见 `SKILL.md` Phase 3，此处给出带注解的精简版，重点说明 **anchor DAG + requiresSubtasks 任务队列 + review 三层门禁** 的组合。

```json
{
  "name": "figma-dom-style",
  "schemaVersion": "1.0",
  "version": "14.0.0",
  "entry": "deliver",
  "reads": ["RULE.mdc"],
  "anchors": [
    { "id": "materials", "dependsOn": [], "output": ".specflow/context/origin-dsl.json",
      "verify": { "producesExist": true, "producesFresh": true, "schema": { "required": ["designMeta"] },
        "custom": ["npx nut gateloop metric design-system ...", "npx nut gateloop metric accessibility ..."] } },
    { "id": "layout", "dependsOn": ["materials"], "output": ".specflow/context/layout-plan.json",
      "verify": { "producesExist": true, "producesFresh": true, "schema": { "required": ["regions"] },
        "custom": ["npx nut gateloop metric spacing ..."] } },
    { "id": "reduce", "dependsOn": ["layout"], "output": ".specflow/context/final-dsl.json",
      "requiresSubtasks": [
        { "id": "b1", "desc": "基线 HTML 提取真实内容", "dependsOn": [] },
        { "id": "b2", "desc": "自适应三级拆分", "dependsOn": ["b1"] },
        { "id": "b3", "desc": "DSL 转换 25 技巧", "dependsOn": ["b1"] },
        { "id": "b4", "desc": "绝对定位转 flex 硬规则", "dependsOn": ["b3"] },
        { "id": "b5", "desc": "逐元素保真 + 渲染对比", "dependsOn": ["b2","b4"] },
        { "id": "b6", "desc": "gate 5 道门禁", "dependsOn": ["b5"] }
      ],
      "verify": { "producesExist": true, "producesFresh": true, "schema": { "required": ["elements"] },
        "custom": ["npx nut gateloop metric ssim ..."],
        "review": { "output": ".specflow/context/restore-report.json", "requireEvidence": true,
          "checklist": [
            { "id": "ssim", "rule": "渲染相似度≥95%（脚本判定）", "mode": "script", "command": "npx nut gateloop metric ssim --input .specflow/context/final-dsl.json" },
            { "id": "size", "rule": "尺寸还原", "mode": "model" },
            { "id": "spacing", "rule": "间距还原", "mode": "model" },
            { "id": "color", "rule": "颜色还原", "mode": "model" },
            { "id": "font", "rule": "字体还原", "mode": "model" },
            { "id": "border", "rule": "边框还原", "mode": "model" },
            { "id": "align", "rule": "对齐还原", "mode": "model" }
          ] } } },
    { "id": "deliver", "dependsOn": ["reduce"],
      "verify": { "review": { "output": ".specflow/context/deliver-report.json", "requireEvidence": true,
        "checklist": [ { "id": "structure", "rule": "HTML 结构合法", "mode": "model" } ] } } }
  ]
}
```

**机制落点：**
- **anchor DAG**：`materials → layout → reduce → deliver` 由 `dependsOn` 拓扑排序；引擎从 `entry`（deliver）逆推起点，逐 anchor 推进。每个 anchor 的 `output` 落 `.specflow/context/`，下游用 `consumes` 引用（防衰减数据管道，§8）。
- **`reads: ["RULE.mdc"]`**：引擎把 RULE.mdc 自动内联进每个 anchor 上下文，等价于「harness 内联」，无需 SKILL.md Step 0（§3）。
- **`requiresSubtasks`（§7）**：`reduce` 这一重活被拆成任务队列（b1→b6，含依赖边），由 GateLoop SubTask 驱动执行，**全部 done 才允许 `doNext` 推进**；把"一个动作一个 anchor"反模式化解为"一个阶段内多子任务"。
- **verify 三道质量闸（§4）**：`schema`（B 完备性，字段存在即过）+ `custom`（A 收敛型，exit code 判错）+ `review`（D 主观，mode:model 带 evidence 四道门禁）。`deliver` 仅留 `review` 做最终人工级卡点。
- **前置门禁（§5）**：`producesFresh` 防复用旧产物、`producesExist` 防空产出，与 `schema` 共同构成 3 道硬闸。

> 启动命令（§1）：`nut gateloop init` → `nut gateloop next` → `nut gateloop status`。注意部分早期稿写 `nut ads ...`——`ads` 为历史代号，引擎已重命名为 **GateLoop**；`nut ads` 命令**已被移除、不再可用**，规范命令为 `nut gateloop`。

## 如何改造现有 skill 到 GateLoop harness 模式

**适用对象**：已存在的 skill（有 SKILL.md 的步骤器），想获得 GateLoop 的防跳步 + 防衰减能力。

### 改造步骤

```
Step 0  收集检测清单（改造的第一件事）
  向用户要"如何判定做完了/做对了"的检测清单
  （SKILL.md 原有"验证清单"章节的条目逐条并入）
  逐条分流：
    固定脚本/CLI/接口可执行 → 路径1（脚本，准确 100%）
    需要模型理解          → 路径2（模型，一次过一遍逐条核对）

Step 1  拆步骤 → 拆 anchor
  把 SKILL.md 正文的 Step 0→N 映射为 anchors[]
  每个强制步骤 = 一个 anchor，brief = 该步骤的目标+约束

Step 2  定输入输出
  上游产出 → produces + emits
  下游依赖 → consumes
  规则文件 → reads（引擎自动内联，删掉 SKILL.md 里的 Step 0 规则注入）

Step 3  配 verify 门禁（检测清单逐条分流落地）
  路径1 脚本项：现成命令 → custom / produceScripts（exit code，100% 准确）
             需模型声明结论 → review.mode:'script'（引擎重跑 command 防伪）
             可写成脚本的 → 固化到 scripts/ CLI 再进 custom
  路径2 模型项 → review.checklist mode:'model'（一次过一遍，逐条 verdict+evidence）
  类型 B 完备 → schema；类型 E 审美 → 不进 verify
  引擎能力逐项评估（producesExist/Fresh、schema、命令类并行、review 四道门禁、
  requiresSubtasks、emits/writesMemory、produceScripts+autoEmit）——
  能落进 verify 的检测绝不留给模型自觉

Step 4  声明数据契约（有 MCP 需求时）
  每个需要外部数据的 Step → step-contracts 声明
  SKILL.md 正文保留"契约获取子流程"，核心逻辑零 MCP 名

Step 5  接通用钩子（可选）
  影响分析/知识库/figma → beforeAnchor
  经验记忆/知识库更新 → afterAnchor
  进度追踪/出错通知 → onEvent

Step 6  定边界
  开发 + 测试 → 进自动闭环（manifest 到 test 结束）
  发布 → 不进流程，人工手动

Step 7  验证
  跑一遍 anchor.config.json，确认每个 anchor 的 verify 都能拦下"跳步"
```

### 改造前后对比

```
改造前（纯 SKILL.md 步骤器）
  SKILL.md 正文 Step 0→N，靠模型自觉执行，跳步靠"播报"约束

改造后（GateLoop harness 模式）
  anchor.config.json 声明 anchors，引擎强制推进
  每个 anchor 的 verify 门禁：模型不产出 → 不推进
  跳步 = verify 不过 = 引擎卡住，模型改不了 exit code
```

### 关键：SKILL.md 正文保留什么？

改造后 SKILL.md 正文**仍然保留**（作为 anchor 的 brief 详细说明 + 契约获取子流程），但：
- 删除"Step 0 规则注入"（改为 `reads` 声明，引擎自动内联）
- 删除"播报防跳步"（改为 verify 门禁，引擎强制）
- 保留"契约获取子流程"（数据契约声明在 step-contracts）

---

## 完整示例

### Anchor 层（小 skill）— anchor.config.json

```json
{
  "name": "css-style-writer",
  "description": "写样式的小 skill",
  "anchors": [
    {
      "id": "scan",
      "name": "扫描已有样式",
      "brief": "扫描项目现有 SCSS，识别可复用的 token 和模式",
      "allowedTools": ["read_file", "search_content"],
      "produces": [".specflow/context/scan-report.json"],
      "verify": { "producesExist": true, "producesFresh": true }
    },
    {
      "id": "write",
      "name": "产出样式",
      "brief": "按扫描结果产出 module.scss",
      "produces": ["src/**/*.module.scss"],
      "verify": {
        "stylelint": true,
        "producesExist": true,
        "producesFresh": true
      }
    }
  ]
}
```

### Flow 层（大流程）— flow.config.json

注意：到 `test` 结束，**没有 deploy anchor**——发布人工手动。

```json
{
  "name": "flow",
  "description": "大工程全流程编排（需求→可发布代码，发布人工）",
  "maxJumps": 3,
  "memory": { "localStore": ".specflow/memory/store.json", "mcp": true },
  "anchors": [
    {
      "id": "context",
      "name": "上下文准备",
      "brief": "读入决策记忆 + 组装上下文包",
      "readsMemory": ["projectFacts", "lastDecisions"],
      "produces": [".specflow/context/context-pkg.json"]
    },
    {
      "id": "design",
      "name": "技术设计",
      "delegateSkill": "frontend-design",
      "consumes": ["spec"],
      "onEscalate": "requirement",
      "produces": [".specflow/context/frontend-plan.json"],
      "verify": { "producesExist": true }
    },
    {
      "id": "dev",
      "name": "代码生成",
      "delegateSkill": "frontend-codegen",
      "consumes": ["frontendPlan"],
      "verify": { "lint": true, "tsc": true, "producesExist": true }
    },
    {
      "id": "test",
      "name": "测试验证",
      "delegateSkill": "code-quality-guard",
      "readsContext": ["design", "dev"],
      "writesMemory": ["lastDecisions"],
      "verify": { "lint": true, "tsc": true, "test": true }
    }
  ]
}
```

**注意 Flow 层的协调字段：**
- `delegateSkill`：委派子 skill（硬门禁仍靠 verify 校验产物）
- `readsContext`：读上游上下文存档（防衰减）
- `writesMemory`：回写决策（跨任务，门禁）
- `onEscalate`：熔断回滚（带 maxJumps 环检测）
- **无 deploy**：发布不进流程，`test` 通过 = done，交人工发布
