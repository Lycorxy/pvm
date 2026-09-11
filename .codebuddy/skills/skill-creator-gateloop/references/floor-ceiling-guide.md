# 脚本（地板）vs 模型（天花板）分工判定指南

> **核心哲学**：脚本划地板（收敛型质量，exit code，不可协商），模型冲天花板（发散型质量，自由创造，鼓励更好）。脚本只陈述客观事实（度量），模型独占主观判断（设计）。脚本卡"思考的完备性"，绝不碰"思考的结果"。
>
> **v2 新增**：主观可归类维度不再"模型自评放水"——走 `review.model` 结构化评审（四道门禁：完整性/script重跑防伪/evidence/阻断），把"自评"升级为"填一张跑不掉的表"。

## 入口：用户检测清单（两条路径）

任何 skill（新建或改造）的质量要求入口都是**用户提供的检测清单**——逐条走下面的判定流程，分流到两条路径：

| 路径 | 适用 | 落地 | 准确性 |
|---|---|---|---|
| **路径1 脚本** | 清单项是固定可执行的脚本/CLI/接口命令 | `verify.custom` / `produceScripts`（exit 0 = pass）；需模型声明结论的用 `review.mode:'script'`（引擎重跑 command 核对） | **100%**（exit code，模型改不了） |
| **路径2 模型** | 清单项需要模型理解（写不出判定脚本） | `review.checklist`（`mode:'model'`）——模型一次过一遍，按清单逐条核对，每条 verdict+evidence | 形式由四道门禁保证（漏评/空话/造假/fail放行 均 FAIL） |

**路径1 优先**：凡是能写成固定脚本的检测，一律固化到 `scripts/` CLI 进 verify，绝不留给模型口述。**路径2 兜底**：写不出脚本的才交给模型，且必须逐条核对留痕。

## 判定流程

拿到检测清单的任何一条，问 5 个问题：

```
Q1: 违反了是否有唯一对错？
  是 → 类型 A（收敛型·脚本 exit code）
  否 → 进入 Q2

Q2: 是否能拆成可度量的客观指标？
  能 → 类型 C（度量回灌·脚本粗筛→模型精判→artifact）
  否 → 进入 Q3

Q3: 是否影响"思考完备性"？
  是 → 类型 B（完备性·schema 字段存在校验）
  否 → 进入 Q4

Q4: 是否有可归类的质量维度（能列 checklist 逐条评）？
  是 → 类型 D（主观可评审·review.model 四道门禁）★ v2 新增
  否 → 类型 E（纯审美·完全交给模型，不进 verify）
```

## 速查表

| 质量要求 | 类型 | 归属 | verify 行为 |
|---|---|---|---|
| 能编译 / 无 `any` / 无硬编码色值 | A 收敛型 | 脚本 exit code | FAIL |
| 圈复杂度 ≤ 10 / 函数 ≤ 25 行 / 文件 ≤ 360 行 | A 收敛型 | 脚本 exit code（带逃生口） | FAIL |
| 组件内无 fetch / 无数据加工 | A 收敛型 | 脚本 custom | FAIL |
| SCSS-TSX 类名一致 | A 收敛型 | 脚本 custom（`style-crosscheck`） | FAIL |
| 子任务从 plan 派生 | A 收敛型 | 脚本 custom（`subtask-derivation`） | FAIL |
| 目录结构合法 | A 收敛型 | 脚本 custom | FAIL |
| 设计覆盖边界 / 错误态 | B 完备性 | 脚本 schema 字段存在 | FAIL（缺字段） |
| 设计布局决策理由 | B 完备性 | 脚本 schema 字段存在（突破默认时必填） | FAIL（突破默认但未声明） |
| 循环依赖数 / useState 数量 | C 度量回灌 | 脚本输出 facts → 模型精判 artifact | 不 FAIL，artifact 传递 |
| **组件拆分好不好** | **D 主观可评审** | **review.model（abstraction-level）** | **blocking fail 禁推进** |
| **数据流是否优雅** | **D 主观可评审** | **review.model（data-flow）** | **blocking fail 禁推进** |
| **布局决策是否合理** | **D 主观可评审** | **review.model（layout-decision）** | **blocking fail 禁推进** |
| **交互是否消除鸿沟** | **D 主观可评审** | **review.model（interaction-design）** | **blocking fail 禁推进** |
| **信息层级是否清晰** | **D 主观可评审** | **review.model（information-hierarchy）** | **blocking fail 禁推进** |
| **容错四态是否覆盖** | **D 主观可评审** | **review.model（error-robustness）** | **blocking fail 禁推进** |
| 视觉效果是否达标 | E 纯审美 | 模型自由 | 不进 verify |
| 交互动线是否自然 | E 纯审美 | 模型自由 | 不进 verify |

## 5 类质量归属

### 类型 A — 收敛型（脚本 exit code 判定）

**特征**：违反了就是错，没有"更好"。

**示例**：
- TypeScript 编译错误（TS2304/TS2345 等）
- ESLint 错误（未使用变量、`any`、`console.log`）
- 圈复杂度 > 10、函数 > 25 行、文件 > 360 行
- SCSS 中硬编码颜色 `#ffffff`
- 组件内 `index.tsx` 出现 `fetch`/`axios`
- SCSS 类名在 TSX 中无引用（或反之）

**归属**：脚本通过 exit code 判定，0 = 通过，非 0 = FAIL。

**逃生口（仅复杂度）**：11-13 允许 `// @complexity-justified: 理由` 注释突破；>13 无逃生口。

### 类型 B — 完备性（脚本 schema 字段存在校验）

**特征**：脚本不管你怎么想，只管你"想全了没"。

**示例**：
- 设计文档必须包含 `edgeCases`（边界情况清单）
- 设计文档必须包含 `errorStates`（错误态处理方案）
- 突破默认布局决策树时必须声明 `layoutRationale`
- frontendPlan 必须包含 4 张表（functionOwnership/stateClassification/efficiencyAudit/moduleBoundary）

**归属**：脚本通过 JSON Schema 校验字段是否存在（不校验内容质量），缺失 = FAIL。

**关键**：模型可以设计任何天才方案，只要它"想全了"。脚本卡的是"思考的完备性"，不是"思考的结果"。

### 类型 C — 度量回灌（脚本粗筛 → 模型精判 → artifact 传递）

**特征**：能客观度量但不能客观判定"好不好"。

**示例**：
- 循环依赖数（脚本能数出来，但 1 个循环依赖是否必须修要结合业务判断）
- 文件被引用次数（脚本能数出来，但是否过度耦合要结合场景判断）
- useState 数量（≥8 提示审视，但是否必须拆要看组件复杂度）
- useEffect 依赖重复（提示合并，但是否合并要看副作用语义）

**归属**：脚本输出 `facts` 数组（客观陈述），不 exit 1。模型按 checklist 逐条精判，产出 artifact。

**关键模式（度量→精判→artifact）**：
```
脚本粗筛（启发式，会误报）
  ↓ 输出 facts 候选问题清单
模型精判（按 checklist 做语义判断）
  ↓ 对每条 fact 给 verdict（真问题/误报/不确定）+ reason
产出 artifact（architectureReview）
  ↓ schema 校验 artifact 存在 + verdicts 非空
下游阶段 consumes artifact
```

**与类型 D 的区别**：C 的评审项是**脚本动态产出的 facts**（每次不同），D 的评审项是**静态固定的 checklist 维度**。C 用 schema+custom 机制，D 用 review 机制。

### 类型 D — 主观可评审（review.model 四道门禁）★ v2 新增

**特征**：主观判断，但有**可归类的质量维度**（能列成 checklist 逐条评）。既不能 exit code 判定，也不是纯审美。

**示例**：
- 组件拆分是否合理（abstraction-level）——能评"过度抽象/欠抽象"
- 数据流是否清晰（data-flow）——能评"props 单向/无隐式依赖"
- 布局决策是否合理（layout-decision）——能评"方案对比/定律校验"
- 交互是否消除鸿沟（interaction-design）——能评"执行/评估鸿沟/微交互"
- 信息层级是否清晰（information-hierarchy）——能评"P0-P3 分级/视觉手段"
- 容错是否覆盖（error-robustness）——能评"四策略/四态"

**归属**：`review.model` 结构化评审。模型按 checklist 逐条给 verdict(pass/fail)+evidence，写入 review 产物 JSON。引擎四道门禁卡形式：
1. **完整性**：结论必须覆盖全部 checklist id（漏评 FAIL）
2. **script 防伪**：`mode:'script'` 的项引擎重跑 command 核对（模型造假 FAIL）
3. **evidence 校验**：`mode:'model'` 每项 evidence 必须含行号/符号引用（空话 FAIL）
4. **阻断判定**：blocking 项 verdict=fail → 禁止推进

**为什么不能只靠"模型自评"（v1 的类型 D 缺陷）**：
- v1 类型 D"完全交给模型，不进 verify" → 模型自评放水，"我觉得没问题"就过
- v2 类型 D 升级为 review.model → 必须逐条评+带证据+评出 fail 必须修，把"自评"逼成"填跑不掉的表"
- 脚本管不了"质量好不好"，但能管住"评审动作被强制+留痕+不可伪造"

**anchor.config 配置**：
```json
"review": {
  "output": ".specflow/context/xxx-review.json",
  "requireEvidence": true,
  "checklist": [
    { "id": "维度ID", "rule": "评审标准描述", "mode": "model" },
    { "id": "可脚本核对项", "rule": "...", "mode": "script", "command": "npx xxx" }
  ]
}
```

**两种 mode**：
- `mode: 'model'`：主观维度，模型给 verdict+evidence，脚本卡 evidence 形式
- `mode: 'script'`：确定项但需模型声明，引擎重跑 command 核对（防伪）

**实际案例**：
- frontend-codegen code anchor：9 项 model（6代码质量+3样式质量）
- frontend-codegen design anchor：6 项 model（设计决策质量）
- web-ui-design：6 项 model（需求/功能/布局/交互/信息层级/容错）
- code-quality-guard：**不配 review**（全是类型 A 收敛型，无主观维度）
- figma-dom-style：6 项 model（还原保真度：尺寸/间距/颜色/字体/边框/对齐），reduce anchor 的 review 门禁

### 类型 E — 纯审美（完全交给模型，不进 verify）

**特征**：既不能客观判定，也不可归类成 checklist，完全是"审美/直觉"范畴。

**示例**：
- 视觉效果是否达到设计稿意图
- 交互动线是否自然
- 这个配色是否有品牌感

**归属**：模型自由发挥，脚本一个字都不该管，也不进 review。

**与类型 D 的区分**：D 能列成"布局合理性/交互消除鸿沟"等可评维度（有判定标准），E 是纯直觉无法列标准。拿不准时问："这条能写出评审标准吗？"——能→D，不能→E。

## 约定 vs 模板的关键区分

| | 约定（Convention） | 模板（Template） |
|---|---|---|
| 性质 | 收敛型 | 发散型 |
| 用法 | 统一规范，模型必须遵守 | 参考样本，模型可突破 |
| 例子 | 命名规则、目录结构、状态三态 | "标准列表页"完整代码 |
| 归属 | 脚本（exit code）+ 内联约定 | 模型（自由创造） |

**错误用法**：把模板当约定，让模型照抄"标准列表页"——扼杀创新。
**正确用法**：约定层统一规范（命名/目录/三态），设计层模型自由发挥。

## 实操：拿到一条新规则怎么归类？

```
例：用户说"组件文件不应该超过 200 行"
Q1: 违反了是否有唯一对错？ 是 → 类型 A（收敛型）

例：用户说"组件拆分要合理"
Q1: 否 → Q2: 能拆成客观指标？ 能（行数/props数/状态数）→ 类型 C（度量回灌）

例：用户说"必须考虑边界情况"
Q1: 否 → Q2: 否 → Q3: 影响"思考完备性"？ 是 → 类型 B（完备性 schema）

例：用户说"布局决策要合理"
Q1: 否 → Q2: 否 → Q3: 否 → Q4: 有可归类维度？ 是（方案对比/定律校验/淘汰理由）→ 类型 D（review.model）

例：用户说"视觉效果要有品牌感"
Q1: 否 → Q2: 否 → Q3: 否 → Q4: 否（写不出评审标准）→ 类型 E（纯审美不进 verify）
```

## 在 anchor.config.json 中的落地

```json
"verify": {
  // 类型 A — 收敛型 exit code（脚本现成）
  "tsc": true,
  "lint": true,
  "fileSize": { "maxFileLines": 360, "maxFunctionLines": 25, "maxComplexity": 10 },

  // 类型 A — 收敛型 custom（自定义脚本）
  "custom": [
    "npx nut gateloop metric style-crosscheck --dir src/",
    "npx nut gateloop metric architecture --input .specflow/context/current.json --state .specflow/anchor-state.json"
  ],

  // 类型 B — 完备性 schema 字段（不校验内容）
  "schema": {
    "required": ["frontendPlan"],
    "properties": {
      "frontendPlan": {
        "required": [
          "functionOwnership", "stateClassification", "efficiencyAudit", "moduleBoundary",
          "edgeCases", "errorStates"
        ]
      }
    }
  },

  // 类型 D — 主观可评审 review.model（v2 新增）
  "review": {
    "output": ".specflow/context/code-review.json",
    "requireEvidence": true,
    "checklist": [
      { "id": "single-responsibility", "rule": "组件单一职责无副作用", "mode": "model" },
      { "id": "no-duplicate", "rule": "无重复逻辑（jscpd 阈值5%）", "mode": "script", "command": "npx jscpd src --threshold 5" }
    ]
  }

  // 类型 E — 纯审美：不进 verify，模型自由
}
```

## 反模式

| 反模式 | 错在哪 | 正确做法 |
|---|---|---|
| "代码必须优雅" → 写进 verify exit code | 主观项进 exit code = 模型自由度被扼杀 | 拆成客观指标（行数/复杂度）类型 C + 主观维度类型 D review |
| "组件拆分照抄 StandardList.tsx" | 模板当约定，扼杀创新 | 约定层（命名/目录/三态）统一，设计层自由 |
| "圈复杂度必须 ≤ 10" 硬性 | 本质复杂度无逃生口 | 11-13 允许 `@complexity-justified` 注释突破 |
| 把"是否优雅"交给脚本 judge | 脚本无法主观判断 | 类型 D review.model，模型评+脚本卡形式 |
| 完备性字段不存在但 verify PASS | 完备性是收敛型 | schema required 强制 |
| ★ 主观维度"模型自评不进 verify"（v1 缺陷） | 自评放水，"我觉得没问题"就过 | 类型 D review.model 四道门禁，逐条评+带证据+fail必改 |
| ★ 全收敛型 skill 硬配 review | 没有主观维度硬塞 review 项 | 全收敛型（如 code-quality-guard）不配 review，全 exit code 判定 |
| ★ review checklist 项过多（>10） | 模型负担过重，低价值打卡 | 聚焦关键维度 4-9 项，细节归 RULE.mdc 规则 |