# 高级模式参考

复杂 skill 场景的专门模式。仅在所创建的 skill 需要这些能力时加载。

## 目录
- [反馈循环模式](#反馈循环模式)
- [条件详情模式](#条件详情模式)
- [视觉分析模式](#视觉分析模式)
- [可验证中间输出](#可验证中间输出)
- [MCP 工具集成（契约模式）](#mcp-工具集成契约模式)
- [包依赖管理](#包依赖管理)

---

## 反馈循环模式

用于有关键质量验证器的任务。

### 结构

```
步骤 N：执行操作
  ↓
步骤 N+1：运行验证器
  ↓
  ┌─ 验证器通过 → 继续到步骤 N+2
  └─ 验证器失败  → 审查错误 → 修复每个错误 → 重新运行验证器 → 重复
```

### 错误实现
```markdown
修复任何错误并继续下一步。
```
问题：无验证修复是否真正有效。

### 正确实现
```markdown
运行 `python scripts/validate.py output.json`：
- 如果输出为 "OK" → 继续下一步
- 如果列出了错误 → 修复每个错误，然后重新运行验证器
- 仅当验证器返回 "OK" 时才继续
```

### 何时使用此模式
- 带 lint/类型检查的代码生成
- 带 schema 验证的文档转换
- 带字段约束检查的表单填写
- 带格式验证的数据转换

### 验证也可以是非脚本的

对于非代码 skill，"验证"意味着与参考进行比较：

```markdown
1. 遵循 STYLE_GUIDE.md 起草内容
2. 对照清单审查：
   - [ ] 术语与指南一致？
   - [ ] 示例匹配标准格式？
   - [ ] 所有必需章节都存在？
3. 如果发现问题：记录每个问题并附章节引用，修订，重新检查
4. 仅当所有清单项通过时才继续
```

---

## 条件详情模式

当基础用法简单但存在针对高级用户的进阶功能时使用。

### 结构

将核心内容内联显示，有条件地链接到进阶内容：

```markdown
## 核心功能（始终可见）

此处为基础指令。足以应对 80% 的使用场景。

**进阶选项：**
- 功能 X：参见 [ADVANCED-X.md](references/ADVANCED-X.md)
- 自定义：参见 [CUSTOMIZE.md](references/CUSTOMIZE.md)
- 遗留行为：参见下方
```

### 示例：文档处理 Skill

```markdown
## 创建文档

对新文档使用 docx-js 库。默认模板处理大多数情况。

**自定义模板：** 参见 [TEMPLATES.md](references/TEMPLATES.md)

## 编辑文档

对于简单文本更改，直接修改段落内容。

**复杂编辑：**
- 修订跟踪 → 参见 [TRACKING.md](references/TRACKING.md)
- OOXML 操作 → 参见 [OOXML.md](references/OOXML.md)

## 遗留格式支持

<details>
<summary>.doc 格式（已弃用，不推荐）</summary>

对于遗留 .doc 文件（2007 年之前），使用 antiword 转换器。
参见 [LEGACY.md](references/LEGACY.md) 获取完整兼容性矩阵。
</details>
```

### 关键规则
- 主 SKILL.md 清晰显示 80% 路径
- 进阶路径被链接，而非展开
- 仅一层链接（无嵌套引用）
- 对很少需要的遗留信息使用 `<details>` 折叠

---

## 视觉分析模式

当输入可渲染为图像且空间/布局理解有帮助时使用。

### 视觉分析有帮助的场景
- PDF 表单字段识别（位置重要）
- UI 截图分析（布局、层级）
- 图表/图形解释（空间关系）
- 文档布局理解（表格、列）

### 实现模式

```markdown
## 布局分析

1. 将源转换为图像：
   ```bash
   python scripts/to_images.py input.pdf output_dir/
   ```

2. 分析每个页面图像以识别：
   - 字段位置和类型
   - 元素之间的空间关系
   - 视觉层级和分组

3. 以结构化格式记录发现：
   ```json
   {
     "fields": [{"name": "signature", "type": "sig", "x": 150, "y": 500}]
   }
   ```

注意：你可以看到图像。将此能力用于布局理解，
而不仅仅是文本提取。
```

### 前置条件
- 必须在 `scripts/` 中包含图像转换脚本
- 指令应显式告诉 agent 去**查看**图像
- 将视觉发现与文本提取结合以获完整信息

---

## 可验证中间输出模式

用于早期错误会级联的复杂多步操作。

### 它解决的问题

无中间验证时：
1. Agent 在内存中处理 50 个表单字段
2. 犯 15 个错误（错误字段、冲突、缺失必需项）
3. 写入最终输出
4. 用户太晚才发现错误
5. Agent 必须从零重做所有事情

使用计划-验证-执行：
1. Agent 创建结构化计划文件（changes.json）
2. 验证器脚本检查计划有无错误
3. Agent 获得具体错误列表，修复计划
4. 重新验证直到干净
5. 执行已验证的计划
6. 首次尝试最终输出即正确

### 实现

```markdown
## 批量更新工作流

### 步骤 1：创建计划（尚未执行）
生成映射每个更新的 `changes.json`：
```json
[
  {"field": "customer_name", "value": "Acme Corp", "action": "set"},
  {"field": "total_amount", "value": 1500, "action": "set"}
]
```

### 步骤 2：验证计划
运行：`python scripts/validate_plan.js changes.json schema.json`

预期输出：
- `"OK"` → 继续步骤 3
- 错误列表 → 修复每个错误，重新运行验证

### 步骤 3：执行已验证计划
运行：`python scripts/execute.js changes.json input.pdf output.pdf`

### 步骤 4：验证输出
运行：`python scripts/verify_output.js output.pdf`
```

### 何时使用
- 批量操作（10+ 项）
- 破坏性变更（难以撤销）
- 复杂验证规则
- 高风险操作（财务、安全）
- 多文件协调变更

### 验证器设计技巧
- 输出**具体**错误消息：`"字段 'sig_date' 未找到。可用字段：customer_name, sig_date_signed"`
- 在错误中包含可用选项（帮助 agent 自我纠正）
- 返回结构化 JSON，而非人工散文（更易解析）
- 检查每项既**存在**又**有效**

---

## MCP 工具集成（契约模式）

当 skill 需要 MCP 提供的知识库/上下文/记忆时，**不直接调用 MCP 工具**，而是通过数据契约声明 + 标准化获取子流程。

> 完整规范：[contract-pattern.md](contract-pattern.md)

### 核心原则

```
skill 声明：Step N 需要 {contract-id} 数据（契约）
skill 不写：Step N 调用 mcp__xxx__yyy（实现绑定）

MCP 存在 → 契约获取子流程调用 MCP，注入数据
MCP 缺失 → 降级到内置工具 / 本地 / 问用户
```

### 铁律

| 规则 | 要求 |
|------|------|
| 核心逻辑零 MCP 工具名 | MCP 调用只在"契约获取子流程"中，且格式标准化 |
| providers 至少两级 | mcp + builtin/fallback，保证 MCP 挂了仍可用 |
| 软依赖 on-missing: degrade | 本地能兜底的，MCP 缺失不阻断 |
| 硬依赖 on-missing: block/ask | 运行时实时数据，缺则阻断或问用户 |
| 核心逻辑只消费 output | 不判断数据来源，逻辑与实现解耦 |

### 契约声明（skill.config.yaml）

```yaml
step-contracts:
  - id: schema-lookup
    step: "1"
    criticality: preferred
    input: { table_name: "表名" }
    output: { schema: "表结构", columns: "列定义" }
    providers:
      - mcp:
          server: "BigQuery"
          tool: "biquery_schema"
          params: { table: "{input.table_name}" }
      - fallback: "ask-user"
    on-missing: degrade
```

### SKILL.md 正文写法

```markdown
### Step 1：获取表结构

🔒 数据契约：schema-lookup（preferred）

#### 契约获取子流程
按 providers 链获取 {schema, columns}：
  1. MCP BigQuery:biquery_schema（优先）
  2. fallback: ask-user
播报来源。

#### 核心逻辑
1. 基于 schema 构建 SELECT 字段...
2. 播报完成
```

### 为什么不直接调用 MCP

| 直接调用（❌） | 契约模式（✅） |
|---------------|---------------|
| `使用 BigQuery:run_query 执行 SQL` 写在核心逻辑里 | 核心逻辑只消费 output，MCP 调用在获取子流程 |
| MCP 缺失 → skill 报错 | MCP 缺失 → 降级链兜底，仍可用 |
| 换 MCP 服务器 → 改 skill | 换 MCP → 只改 config 的 providers |
| skill 绑定特定 MCP | skill 契约解耦，可移植 |

### 唯一例外：工具命名限定

当契约获取子流程中需要调用 MCP 时，工具名必须完全限定为 `mcp__{server}__{tool}` 或 `{server}:{tool}` 格式，避免多 MCP 服务器歧义。但此调用仅限获取子流程，不进入核心逻辑。

---

## 包依赖管理

Skill 在不同环境中运行，能力不同。

### 环境矩阵

| 能力 | claude.ai | Claude API | CodeBuddy 本地 |
|-----------|-----------|------------|-----------------|
| 安装 npm 包 | 是 | 否 | 是 |
| 安装 pip 包 | 是 | 否 | 是 |
| 克隆 GitHub 仓库 | 是 | 否 | 是 |
| 运行时网络访问 | 是 | 有限 | 是 |
| 预安装包 | 各异 | 固定集合 | 项目依赖 |

### 规则：绝不假设包已安装

**错误：**
```markdown
使用 pdf 库处理文件。
```

**正确：**
```markdown
安装所需依赖：
```bash
pip install pypdf
```

然后使用：
```python
from pypdf import PdfReader
reader = PdfReader("file.pdf")
```
```

### 最佳实践

1. **列出所有依赖** 在 SKILL.md frontmatter 注释或专门章节中
2. **提供安装命令** 可复制粘贴
3. **固定版本** 以确保可重现性（`pypdf==3.10.0`）
4. **检查可用性** 在推荐包之前在目标环境中
5. **提供回退** 当主包不可用时：
   ```markdown
   主要：pdfplumber（pip install pdfplumber）
   扫描版 PDF 回退：pytesseract + pdf2image
   ```

### Scripts 目录依赖

如果你的 skill 包含可执行脚本：

1. 创建 `scripts/requirements.txt` 并附上脚本特定依赖
2. 在脚本使用章节顶部添加安装说明：
   ```bash
   pip install -r scripts/requirements.txt
   python scripts/analyze.py input.pdf
   ```
3. 在脚本中优雅处理导入错误（不要崩溃 agent 的会话）
