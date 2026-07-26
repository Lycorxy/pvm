# Skill + PVM 绝了，AI 直接帮你搞定环境问题了

> **你是否遇到过这些场景：**
>
> - **后端开发者**：克隆前端项目，`npm install` 报错 —— "Node 版本不对，你本地装个 18"
> - **AI 模型部署**：跑了好久，Token 都烧了好几万，Python 还没装上
> - **包管理器打架**：你用 pnpm，他用 yarn，lockfile 冲突改半天
> - **开发迭代翻车**：环境问题导致新任务跑不起来，查半天才发现是依赖版本漂移
>
> **我们真的有必要把 Token 和精力浪费在重试上吗？每次都要和环境较真吗？**
>
> ⭐ [如果这篇文章帮到你了，给个 Star 吧](https://github.com/Lycorxy/pvm) | 💬 [评论区说说你的环境痛点](#评论区)
>
> 接下来，我会用 6 个真实场景，告诉你：**AI 怎么帮你从环境地狱里爬出来。**

---

## 你还在为环境问题折腾吗？

装个 Node 要用 nvm，切 Python 版本要配 pyenv，换个 Go 版本还得手动下载。每个工具一套命令、一套配置，光记命令就要掉半条命。更别提冲突的时候——nvm 和 volta 打架，pyenv 和 conda 互顶，PATH 乱成一锅粥。

**现在，你只需要说一句话。**

```
"帮我装 Node 20，Python 3.12，外加 pnpm 9"
```

AI 直接执行，30 秒搞定。

---

## 这背后是什么？

**PVM（Polyglot Version Manager）**—— 一个统一管理 9 种运行时的版本管理器：

| 运行时 | 安装 | 用户级切换 | 项目级锁定 | 国内镜像 |
|--------|:---:|:---:|:---:|:---:|
| Node.js | ✅ | ✅ | ✅ .pvmrc | npmmirror |
| Python | ✅ | ✅ | ✅ .pvmrc | npmmirror |
| Bun / Deno | ✅ | ✅ | ✅ .pvmrc | npmmirror |
| pnpm / Yarn | ✅ | ✅ | ✅ .pvmrc | npmmirror |
| Go | ✅ | ✅ | — 仅用户级 | golang.google.cn |
| Rust | ✅ | ✅ | — 仅用户级 | rsproxy.cn |
| Git | ✅ | ✅ | — 仅用户级 | npmmirror |

**CodeBuddy Skill**—— 一个让 AI 能直接执行 PVM 命令的扩展包。它不是给你文档让你自己看，而是 **AI 直接帮你跑命令**。

---

## 到底有多"绝"？真实场景说话

### 场景一：新电脑，零环境，一句搞定

```
用户：帮我配好 Node 20、Python 3.12、pnpm 9
AI：  pvm install node@20 --mirror
      ✓ Node.js 20.18.0 安装完成，已设为默认

      pvm install python@3.12 --mirror
      ✓ Python 3.12.4 安装完成，已设为默认

      pvm install pnpm@9 --mirror
      ✓ pnpm 9.15.0 安装完成，已设为默认
```

以前：查三个工具的安装命令 → 各下载安装 → 配环境变量 → 验证。现在：**一句话，30 秒。**

---

### 场景二：冲突工具，自动清理

```
用户：我电脑有 nvm，帮我换成 pvm 管理 node
AI：  [检测冲突] 发现 nvm，Node 当前版本 18.17.0
      1. pvm install node@18
      2. node --version → ✓ 18.17.0
      3. 执行 Skill 脚本：uninstall-tool nvm --yes
         → 杀进程 → 清 PATH → 删目录 → 清注册表
      4. pvm doctor → ✓ 全绿

      nvm 已彻底卸载，Node 18 已切换到 PVM 管理
```

以前：手动删 nvm 目录 → 删环境变量 → 删 PATH → 重装 node。现在：**一句话，全自动。**

支持卸载 18 种工具：nvm、volta、fnm、pyenv、rustup、asdf、conda...

---

### 场景三：lock 文件合并冲突，AI 帮解

```
用户：git merge 后 pnpm-lock.yaml 冲突了
AI：  [扫描] 发现 pnpm-lock.yaml 冲突
      执行 Skill 脚本：resolve-lockfile-conflict
      → git checkout --theirs (清除冲突标记)
      → pnpm install --lockfile-only (重新生成)
      → git add pnpm-lock.yaml
      ✓ lockfile 冲突已解决
```

lock 文件手动合并 = 灾难。但 AI **不手动合并**，而是让包管理器重新生成，100% 正确。

---

### 场景四：项目环境隔离，进目录自动切

```
用户：这个项目锁定 Node 20 + pnpm 9
AI：  pvm install node@20 --local
      pvm install pnpm@9 --local
      ✓ .pvmrc 已写入，版本已安装
```

效果：你 `cd` 进项目目录，Node 自动切到 20；`cd` 到另一个需要 Node 18 的项目，自动切到 18。**零思考，零手动。**

---

### 场景五：项目环境配置，一行命令

```
用户：这个项目需要配置开发环境
AI：  [1] pvm install node@20 --local
      [2] pvm install pnpm@9 --local
      [3] 执行 Skill 脚本：setup-npmrc
          → 配置国内镜像
          → 启用镜像加速
      ✓ .pvmrc 和 .npmrc 已配置完成
```

效果：
- `.pvmrc`：锁定 Node 和 pnpm 版本，团队成员进入目录自动切换
- `.npmrc`：配置镜像源，安装依赖速度提升 5-10 倍
- 提交到 git 后，团队 clone 项目就能开箱即用

> 💡 **想深入了解依赖安全？** 查看 [锁版本、冻依赖、防漂移 —— pnpm 依赖管理安全三部曲](./npm-dependency-security.md)

---

### 场景六：环境诊断，一键排查

```
用户：pvm 好像有问题
AI：  pvm doctor
      ✓ PVM_HOME 目录       [OK]
      ✓ shims 目录          [OK]
      ✓ shims 在 PATH 中    [OK]
      ✓ pvm 二进制位置      [OK]
      ✓ PVM_HOME 可写权限   [OK]
      ✓ 无旧版 .ps1 shim    [OK]
      ✓ 无冲突版本管理器    [OK]
```

7 项核心检查，30 秒出结果。有问题的自动标红，告诉你修什么。

---

## PVM Skill 能力全图

| 能力维度 | 运行时管理 | 依赖安全 | 环境修复 |
|---------|-----------|---------|---------|
| **核心功能** | 安装/切换/卸载 | 版本锁定 | 冲突工具清理 |
| | 项目隔离 | lockfile 修复 | 环境诊断 |
| | 多版本并存 | .npmrc 配置 | PATH 修复 |
| | 国内镜像加速 | 依赖不漂移 | |
| **支持范围** | 9 种运行时 | 项目级锁定 | 7 个自动化脚本 |
| | Node/Python/Go/Rust/Bun/Deno/Git/pnpm/Yarn | .pvmrc 文件 | 18 种工具卸载 |

**一句话总结**：支持 9 种运行时 + 18 种工具卸载 + 7 个自动化脚本

---

## 最后

Skill + PVM 这套组合，本质上是把"人查文档、人敲命令、人排查问题"变成了"人说需求、AI 直接干"。

**你只管写代码，环境问题 AI 来。**

---

## 怎么用？

1. **安装 PVM**：一行命令搞定
   ```bash
   # Windows（默认从 Gitee 镜像下载，国内更快）
   iwr -useb https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex

   # macOS/Linux（默认从 Gitee 镜像下载）
   curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash

   # 国际用户（从 GitHub 下载）
   # iwr -useb https://github.com/Lycorxy/pvm/raw/master/scripts/install.ps1 | iex
   ```

   > 💡 **提示**：国内用户推荐从 Gitee 镜像下载，速度更快。国际用户可直接从 GitHub 下载。

2. **获取 Skill**：两种方式任选

   **方式一：直接下载（推荐）**

   - [从 GitHub Release 下载](https://github.com/Lycorxy/pvm/releases/download/v0.0.1/pvm-env-manager.zip)
   - [从 Gitee Release 下载（国内更快）](https://gitee.com/lycorxy/pvm/releases/download/v0.0.1/pvm-env-manager.zip)

   下载后解压到项目的 `.codebuddy/skills/` 目录即可。

   **方式二：Clone 仓库**

   ```bash
   # Clone PVM 仓库（包含 Skill）
   git clone https://github.com/Lycorxy/pvm.git
   # 或从 Gitee 克隆（国内更快）
   git clone https://gitee.com/lycorxy/pvm.git
   ```

   Skill 位于 `.codebuddy/skills/pvm-env-manager` 目录，支持 CodeBuddy Skill 的 IDE 会自动加载。

3. **体验 Skill**：在 Trae IDE 中打开项目，直接对话即可
   ```
   "帮我装 Node 20"
   "项目锁定 Python 3.12"
   "清理 nvm，换成 pvm 管理"
   ```

   > 遇到问题？👉 [GitHub Issues](https://github.com/Lycorxy/pvm/issues) | [Gitee Issues](https://gitee.com/lycorxy/pvm/issues)

---

**如果这篇文章对你有帮助，别忘了：**
- ⭐ [给 GitHub 仓库点个 Star](https://github.com/Lycorxy/pvm)
- 💬 在评论区说说你的环境配置痛点
- � 转发给还在为环境折腾的朋友
