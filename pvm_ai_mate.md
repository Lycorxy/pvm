# AI 助手越能干，你的开发环境越危险：pvm 给每个项目一个隔离的家

> **pvm × AI Mate**：本地项目隔离 —— 让 AI 助手随便切项目、装依赖、跑流水线、发 5 平台产物，而不弄脏你的机器。

AI 编程助手（AI Mate 这类 Coding Agent）正在成为默认生产力。但很少有人讨论一个正在发生的事实：

**环境破坏的主力，正在从"粗心的人"变成"不知疲倦的 AI"。**

人一天切两三次项目，AI 一天几十次；人装错了会停下来问，AI 会自动"修复"——
升级全局包、改 PATH、覆盖版本、重写 lockfile，全部静默完成。你早上还好好的环境，
晚上就莫名其妙挂了——不是玄学，是 AI 在"帮忙"。

问题出在环境模型本身：**nvm 那一代工具是为"人"设计的**——人记得切版本，AI 不该记，也不该切。
给 AI 自由的前提是隔离：**环境不能装在机器上，必须跟着项目走。**

这正是 pvm（Polyglot Version Manager）的核心设计：一个单二进制、跨平台的运行时管理器，
管 9 种运行时（node/python/go/git/bun/deno/pnpm/yarn/rust），做的是**本地项目隔离**——
每个项目一个独立环境，互不可见，互不污染。

---

## 一、没有隔离，AI 助手的四种翻车

**翻车一：版本错乱。** 全局装的是 Node 22，老项目要 Node 18。AI 直接 `npm install` —— lockfile 被静默重写，CI 红了。你以为是代码问题，查了半天是环境。

**翻车二：环境试错循环。** 脚本写好了，`python: command not found`。AI 开始自动"修复"：装环境、换版本、修 PATH。一分钟的活折腾半小时——**每一步都发生在你的全局环境里**。

**翻车三：命令靠猜。** A 项目 `npm run build`，B 项目 `make release`，C 项目 `go build ./cmd/... && zip ...`。AI 接手陌生项目，构建命令全靠猜：猜错 → 翻 README → 再猜。

**翻车四：全局污染连带崩盘。** AI 为修 B 升了全局包、改了 PATH。B 是好了，A 和 C 晚上神秘挂了。

四个翻车，一个病根：**环境状态没有跟着项目走，AI 每次都在盲猜当前环境。**

![一个 AI 助手，四个项目，零环境翻车：问题与解法全景](diagrams/ai-mate-01-value.png)

---

## 二、隔离方案：AI 只管干活，环境交给配置

pvm 和 AI 助手的配合分四层，一层解决一类翻车：

![pvm × AI Mate：四层配合](diagrams/ai-mate-02-architecture.png)

### 第一层：声明式隔离 —— cd 进项目，环境自动就位

每个项目根目录放一个 `.pvmrc`，声明这个项目要什么：

```ini
node 20.20.2
pnpm 8.15.9
```

提交进 Git，环境声明从此属于项目，不属于机器。pvm 的 shim 是**原生硬链接**（不是 bash 脚本，Windows 原生可用），每次调用实时读取当前目录的 `.pvmrc`——

**AI（或任何人）cd 进项目的那一刻，node 就是 20.20.2；cd 去隔壁项目，自动变。** 不用 `use`、不用切终端、不用问。

> AI 的视角：我永远不需要知道"全局装的什么"，我只管 cd。环境是项目的事，不是我的事。

### 第二层：隔离地干活 —— 本地流水线，一条命令跑完全程

环境隔离了，流程也要跟着项目走。把构建/测试/打包/发布写成项目根的 `pvm.yaml`——项目的"操作说明书"：

```yaml
name: my-app
build:
  pro:
    - pnpm run build:pro
pkg:
  go:
    pro:
      - pvm pack go --entry . --output dist/app
release:
  pro:
    - "@build:pro"           # ① 串行引用：构建完才继续
    - "@test:unit & @lint"   # ② 后台并发：同时跑，取最慢
    - "@pkg:go:pro"          # ③ 交付：一条命令出 5 平台产物
```

AI 接手项目，一条命令看到全部能力：

```
$ pvm run --list
build:pro      → 构建
pkg:go:pro     → 打包
release:pro    → 构建 → 测试∥lint → 打包
```

![本地流水线：@ 引用串行 · & 并发 · 零手工引导](diagrams/ai-mate-03-pipeline.png)

三个关键设计，都是为"AI 执行者"准备的：

- **`@` 引用串行，`&` 后台并发**：流水线在本地编排，测试和 lint 同时跑，总耗时取最慢的一个而不是相加；
- **零手工引导**：执行前 pvm 自动装齐 `.pvmrc` 声明的运行时——流水线里只有业务命令，AI 连"先装 node"都不用写；
- **先看后跑**：`pvm run --dry-run` 预览将执行的命令序列，AI 的每一步透明可控。

执行 `pvm run release:pro`，一条命令跑完全程。

### 第三层：隔离环境坏了，AI 自己修

pvm 官方 skill 让 AI 获得环境自诊断能力。跨平台 harness 脚本，五项体检：

```
[pvm] windows | cwd=D:\my-app | PVM_HOME=C:\Users\me\.pvm
[1] pvm 安装 → ✅ pvm 0.2.0
[2] shims PATH → ✅ ~/.pvm/shims 已在 PATH
[3] nvm 干扰 → ✅ nvm 存在，node 已由 pvm 接管
[4] pvm doctor → ✅ 全部检查通过
[5] 项目配置 → ✅ .pvmrc 可解析
    ℹ️ pvm.yaml 不存在（如需流水线: 在项目根创建 pvm.yaml）
------
结果: 4 通过, 0 失败 | exit=0
```

每项失败都自带「下一步」：pvm 没装就装、shims 不在 PATH 就前置、nvm 劫持 node 就接管。AI 按输出执行修复，复跑体检直到全绿——**闭环在 AI 手里完成，用户只看到"已就绪"**。机器上原来有 nvm？shims 前置接管 node，两套共存不打架。

### 第四层：隔离地产出 —— 一条命令出 5 平台产物

项目要发给用户？AI 不用研究 GOOS/GOARCH、不用懂 PyInstaller：

```
$ pvm pack go --entry . --output dist/pvm
==> go build: 5 cross-platform targets (windows/amd64, linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
==> [1/5] go build (windows/amd64) -> dist\pvm-windows-amd64.exe
==> packed go: dist\pvm-windows-amd64.exe (8.8 MB, from .)
...
==> [5/5] go build (darwin/arm64) -> dist\pvm-darwin-arm64
```

node 项目走官方 SEA（自动下载各目标平台的 node 发行版做底座），python 项目走 PyInstaller。**默认跨平台全量，`--platform` 可指定单目标**——产物命名与 `pvm self-update` 的更新源约定一致，发完用户直接自更新。

---

## 三、实战：AI 助手的一天

| 时间 | 项目 | 任务 | AI 用到的 pvm 能力 |
|------|------|------|------|
| 09:00 | A（Node 18 老项目） | 修线上 bug | cd 即切 18；`pvm run test:all` 跑回归 |
| 13:00 | B（Python + Node） | 加功能 | `.pvmrc` 双运行时自动就位；装依赖零污染 |
| 16:00 | C（Go 工具） | 升依赖 + 发版 | `pvm run release:pro`；`pvm pack go` 出 5 平台产物 |
| 20:00 | D（全新项目） | 初始化 | `pvm init --local node@22` 秒起步；流水线从模板开始 |

全程零全局污染：A 的 18 和 D 的 22 同时存在于同一台机器，互不知晓，互不干扰。
**这就是本地项目隔离的意义——AI 可以放开手脚干活，因为它破坏不了任何项目边界之外的东西。**

---

## 四、价值总账

| 你遇到的问题 | pvm 的解法 | 你的收益 |
|------|------|------|
| 项目间版本打架 | `.pvmrc` 声明式隔离环境 | 版本跟着项目走，零切换成本 |
| AI 折腾环境半小时 | pvm skill 五项体检 + 自愈 | 故障自闭环，不折腾人 |
| 构建命令靠猜 | `pvm.yaml` 本地流水线 | `--list` 即能力清单，接手即上手 |
| 流程慢、步骤串行 | `@` 串行 + `&` 并发 | 并发段取最慢，本地流水线提速 |
| 环境步骤多、启动慢 | 单二进制 · 无守护进程 · 自动装齐运行时 | 零手工引导，执行即开始 |
| 一个项目出 5 平台 | `pvm pack` 跨平台打包 | 一条命令交付，命名对齐自更新 |
| 多项目互相污染 | 项目级隔离，环境按目录生效 | 修 A 不炸 C |

---

## 五、边界（诚实清单）

pvm 不是银弹，这些边界 AI 和你都要知道：

- **python 打包不能交叉平台**（PyInstaller 的硬限制）——要发多平台，在各平台分别跑或走 CI matrix
- **node 打包要求 Node ≥ 20**（官方 SEA 的要求）；darwin 产物在非 macOS 机器上打包后未签名，分发前要在 macOS 补签
- **go 交叉编译要求纯 Go 代码**（自动 `CGO_ENABLED=0`，依赖 cgo 的项目交叉会失败）
- **go/git/rust 全局唯一**，不进项目级 `.pvmrc`
- **环境变量按进程缓存**——切完版本要新开终端，IDE 要完全重启
- **pvm 只管运行时版本，不管应用依赖**——node_modules 归 npm/pnpm，分工要清楚

边界写清楚，AI 才不会越界硬试——这也是"AI 助手多项目开发"能稳定运转的前提。

---

## 六、现在，给你的 AI 助手一个隔离的家

AI 编程的下半场，拼的不是谁的环境更干净，而是**谁的环境不怕 AI 折腾**。

**给 pvm 一个 Star**，然后三步体验本地项目隔离：

1. **安装**（一行命令）：

```bash
# Windows
irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex
# macOS / Linux
curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
```

2. **声明项目环境**：项目根写一个 `.pvmrc`，提交进 Git
3. **把流水线交给 AI**：写好 `pvm.yaml`，让 AI 助手 `pvm run release:pro`

从此你的 AI 助手 cd 到哪个项目，哪个项目的环境就是对的——**它只管干活，环境的事，配置说了算。**

---

*pvm：Polyglot Version Manager — 一个二进制，管 9 种运行时，Windows 原生。*
*GitHub：<https://github.com/Lycorxy/pvm> — 觉得有用就点个 Star ⭐*
