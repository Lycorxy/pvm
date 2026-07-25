# Skill + PVM 绝了，AI 直接帮你搞定环境问题了

> 九个运行时，一个命令搞定。冲突清理、权限修复、依赖锁定，AI 全包了。

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

| 运行时 | 支持情况 |
|--------|----------|
| Node.js | ✅ 安装 / 切换 / 项目级锁定 |
| Python | ✅ 安装 / 切换 / 项目级锁定 |
| Go | ✅ 安装 / 切换 |
| Rust | ✅ 安装 / 切换 |
| Bun / Deno | ✅ 安装 / 切换 / 项目级锁定 |
| pnpm / Yarn | ✅ 安装 / 切换 / 项目级锁定 |
| Git | ✅ 安装 / 切换 |

**CodeBuddy Skill**—— 一个让 AI 能直接执行 PVM 命令的扩展包。它不是给你文档让你自己看，而是 **AI 直接帮你跑命令**。

---

## 到底有多"绝"？真实场景说话

### 场景一：新电脑，零环境，一句搞定

```
用户：帮我配好 Node 20、Python 3.12、pnpm 9
AI：  pvm install node@20 --mirror
      pvm use node@20
      ✓ Node.js 20.18.0 安装完成

      pvm install python@3.12 --mirror
      pvm use python@3.12
      ✓ Python 3.12.4 安装完成

      pvm install pnpm@9 --mirror
      pvm use pnpm@9
      ✓ pnpm 9.15.0 安装完成
```

以前：查三个工具的安装命令 → 各下载安装 → 配环境变量 → 验证。现在：**一句话，30 秒。**

---

### 场景二：冲突工具，自动清理

```
用户：我电脑有 nvm，帮我换成 pvm 管理 node
AI：  [检测冲突] 发现 nvm，Node 当前版本 18.17.0
      1. pvm install node@18
      2. node --version → ✓ 18.17.0
      3. uninstall-tool.bat nvm --yes
         → 杀进程 → 清 PATH → 删目录 → 清注册表
      4. pvm doctor → ✓ 全绿

      nvm 已彻底卸载，Node 18 已切换到 PVM 管理
```

以前：手动删 nvm 目录 → 删环境变量 → 删 PATH → 重装 node。现在：**一句话，全自动。**

支持卸载 18 种工具：nvm、volta、fnm、pyenv、rustup、asdf、conda...

---

### 场景三：pnpm EPERM 权限错误，自动修复

```
用户：pnpm install 报 EPERM，esbuild.exe 删不掉
AI：  [检测] 发现 2 个 esbuild.exe 进程 (PID: 30392, 59340)
      fix-pnpm-eperm.bat --reinstall
      → 杀进程 → 删 node_modules → pnpm install
      ✓ 依赖重装完成，EPERM 解决
```

以前：Google 搜报错 → 找到 taskkill 命令 → 手动执行 → 手动删 node_modules → 重装。现在：**AI 自己检测、自己修。**

---

### 场景四：lock 文件合并冲突，AI 帮解

```
用户：git merge 后 pnpm-lock.yaml 冲突了
AI：  [扫描] 发现 pnpm-lock.yaml 冲突
      resolve-lockfile-conflict.bat
      → git checkout --theirs (清除冲突标记)
      → pnpm install --lockfile-only (重新生成)
      → git add pnpm-lock.yaml
      ✓ lockfile 冲突已解决
```

lock 文件手动合并 = 灾难。但 AI **不手动合并**，而是让包管理器重新生成，100% 正确。

---

### 场景五：项目环境隔离，进目录自动切

```
用户：这个项目锁定 Node 20 + pnpm 9
AI：  pvm use node@20 --local
      pvm use pnpm@9 --local
      ✓ .pvmrc 已写入
```

效果：你 `cd` 进项目目录，Node 自动切到 20；`cd` 到另一个需要 Node 18 的项目，自动切到 18。**零思考，零手动。**

---

### 场景六：环境诊断，一键排查

```
用户：pvm 好像有问题
AI：  pvm doctor
      ✓ PVM 安装状态    [OK]
      ✓ 环境变量 PATH    [OK]
      ✓ 冲突工具检测     [OK]
      ✓ 目录结构         [OK]
      ✓ 运行时状态       [OK]
      ✓ .npmrc 配置      [OK]
      ✓ Shell 配置       [OK]
```

8 项检查，30 秒出结果。有问题的自动标红，告诉你修什么。

---

## PVM Skill 能力全图

```
┌─────────────────────────────────────────────────────┐
│                    PVM Skill                        │
├───────────────┬─────────────┬───────────────────────┤
│  运行时管理    │  依赖安全    │  环境修复              │
├───────────────┼─────────────┼───────────────────────┤
│ 安装/切换/卸载 │ 版本锁定     │ 冲突工具清理          │
│ 项目隔离       │ lockfile 修复│ EPERM 权限修复        │
│ 多版本并存     │ .npmrc 配置  │ 环境诊断              │
│ 国内镜像加速   │ 依赖不漂移   │ PATH 修复             │
├───────────────┴─────────────┴───────────────────────┤
│  支持 9 种运行时 + 18 种工具卸载 + 6 个自动化脚本     │
└─────────────────────────────────────────────────────┘
```

---

## 最后

Skill + PVM 这套组合，本质上是把"人查文档、人敲命令、人排查问题"变成了"人说需求、AI 直接干"。

**你只管写代码，环境问题 AI 来。**
