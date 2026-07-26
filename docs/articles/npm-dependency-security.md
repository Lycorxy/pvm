# 本地能跑,CI 炸了,现网出 Bug?—— 依赖安全三道防线

> **灵魂连问:**
>
> - 本地没问题,CI 之后,现网出 Bug,甚至有些隐藏的 Bug 很久才发现,心里直冒冷汗?
> - 是不是正着急,死活跑不通,人家运行好好的,Node 版本和 pnpm 版本都一样也不行?
> - 甚至出现合并 lockfile 文件有种要疯的感觉?
>
> ⭐ [如果这篇文章帮到你了,给个 Star 吧](https://github.com/Lycorxy/pvm) | 💬 [评论区说说你的依赖坑](#评论区)
>
> 接下来,我会用 3 个真实案例 + 3 道防线,告诉你:**如何让依赖永不漂移。**

---

## 一、pnpm 依赖管理原理:为什么会有版本漂移?

前端圈有个经典段子:代码 push 上去,CI 炸了,你喊一声"我本地没问题啊"。

问题出在哪?根源在于 pnpm 的依赖管理机制。

### 1.1 为什么会有版本漂移问题?

依赖漂移的本质是:**版本范围约束允许自动升级**。

看一个真实案例:

```json
// package.json 里写的
"lodash": "^4.17.20"

// 实际装的是
"lodash": "4.17.21"   // ← 悄悄更新了
```

今天跑 `pnpm install`,装了 4.17.21;明天新队友 clone 项目,装了 4.17.22 —— 多了一个 bugfix,但可能引入了一个你没测过的行为。

### 1.2 `^` `~` `*` 符号的含义和风险

这些符号是语义化版本(SemVer)的范围约束:

| 符号 | 含义 | 示例 | 风险等级 |
|------|------|------|----------|
| `^` | 兼容版本(允许次版本号升级) | `^4.17.20` → 允许 4.x.x | 🔴 高风险 |
| `~` | 补丁版本(仅允许补丁号升级) | `~4.17.20` → 允许 4.17.x | 🟡 中风险 |
| `*` | 任意最新版本 | `*` → 最新版 | 🔴 极高风险 |
| 无符号 | 精确版本 | `4.17.20` → 必须 4.17.20 | 🟢 安全 |

**风险本质:** 版本范围约束让依赖包有了"自由意志",每次安装都可能装到不同的版本。

### 1.3 lockfile 的作用和重要性

`pnpm-lock.yaml` 是依赖树的"快照",记录了所有依赖的精确版本。

**lockfile 解决的问题:**
- ✅ 锁定依赖的精确版本
- ✅ 锁定依赖树结构(包括子依赖)
- ✅ 确保团队安装相同的依赖

**lockfile 的局限:**
- ⚠️ **同一个 lockfile,不同 pnpm 版本可能解析出不同的依赖树**
- ⚠️ 不同版本的 pnpm 有不同的依赖解析算法
- ⚠️ lockfile 格式本身可能因 pnpm 版本而异

看一个真实案例:

```yaml
# 你本地的 pnpm-lock.yaml
lodash: 4.17.21

# CI 上跑的 pnpm-lock.yaml (因为 pnpm 版本不同)
lodash: 4.17.21
  dependencies:
    some-sub-dep: 1.0.0  ← CI 上解析出不同的子依赖
```

### 1.4 peer dependency 地狱

版本不兼容报错还算好的。更可怕的是:**不报错,但运行时表现异常**。

```bash
# React 18 项目,装了某个基于 React 17 的 UI 库
npm ERR! peer react@"^17.0.0" from some-ui-lib@3.0.0
npm ERR! Found: react@18.2.0
```

---

## 二、我们的解决方案:三道防线

理解了问题根源,我们构建三道防线彻底锁死依赖。

### 第一道防线:锁定 pnpm 版本(.pvmrc)

**核心问题:** 不同人用不同版本的 pnpm,`pnpm-lock.yaml` 的解析结果可能不一样。

**解决方案:** 用 PVM 锁死 pnpm 版本。

```bash
# 在项目根目录执行
pvm install pnpm@9       # 安装 pnpm 9
pvm use pnpm@9 --local   # 写入 .pvmrc,项目级锁定
```

执行后,项目根目录多了一个 `.pvmrc` 文件:

```ini
# .pvmrc
pnpm = 9.15.0
```

这个文件提交到 git 后,任何队友 `cd` 进项目,pnpm 自动锁定 9.15.0。**版本不统一?不存在了。**

> 💡 **想了解 PVM 如何实现项目级环境隔离?**
> 查看 [Skill + PVM 绝了,AI 直接帮你搞定环境问题了](./skill-pvm-ai-environment.md)

---

### 第二道防线:冻结依赖策略(.npmrc)

**核心问题:** `^` `~` `*` 这些语义化版本符号是依赖漂移的根源。

**解决方案:** 配置 `.npmrc` 锁死安装策略。

```bash
# PVM Skill 一键配置
.codebuddy\skills\pvm-env-manager\scripts\setup-npmrc.bat --force
```

这会在你全局 `~/.npmrc` 中写入(不影响其他项目的话可用项目级 `.npmrc`):

```ini
# 1. 精确版本,不加 ^ ~
save-prefix=""

# 2. 优先使用现有 lockfile,不自动更新
prefer-frozen-lockfile=true

# 3. 自动安装 peer 依赖
auto-install-peers=true

# 4. 基于时间的版本解析(可复现)
resolution-mode=time-based

# 5. 国内镜像加速
registry=https://registry.npmmirror.com
```

**效果对比:**

```bash
# 配置前
pnpm add lodash
# package.json: "lodash": "^4.17.21"  ← 带 ^,会漂移

# 配置后
pnpm add lodash
# package.json: "lodash": "4.17.21"    ← 精确版本,锁死
```

---

### 第三道防线:lockfile 正确使用 + 自动冲突合并

**核心问题:** lockfile 是依赖的"唯一真相来源",不是装饰品。

**正确姿势:**

```bash
# 开发环境:正常安装(尊重 lockfile)
pnpm install

# 提交前:格式化 lockfile(可选)
pnpm install --lockfile-only

# CI 环境:严格模式,lockfile 不一致 = 直接报错
pnpm install --frozen-lockfile
```

**lockfile 必须提交到 git!** 看到 `.gitignore` 里有 `pnpm-lock.yaml`?立刻删掉那条规则。

**lockfile 合并冲突怎么办?**

```bash
# 不要手动编辑 lockfile!用 PVM Skill 自动处理
.codebuddy\skills\pvm-env-manager\scripts\resolve-lockfile-conflict.bat

# 脚本自动做的事:
# 1. git checkout --theirs(清除冲突标记)
# 2. pnpm install --lockfile-only(基于 package.json 重新生成)
# 3. git add pnpm-lock.yaml
```

> 💡 **想了解更多 AI 自动化场景?**
> 查看 [Skill + PVM 绝了,AI 直接帮你搞定环境问题了](./skill-pvm-ai-environment.md)

---

## 三、完整配置模板

提交到 git 的三个文件,构成依赖安全的三角形:

### 3.1 .pvmrc — 锁定工具链版本

```ini
node = 20.18.0
pnpm = 9.15.0
```

**作用:**
- 锁定 Node.js 版本
- 锁定 pnpm 版本
- 团队成员进入项目自动切换版本

### 3.2 .npmrc — 锁定安装策略

```ini
save-prefix=""
prefer-frozen-lockfile=true
auto-install-peers=true
resolution-mode=time-based
```

**作用:**
- 新增依赖使用精确版本
- 优先使用 lockfile,不自动更新
- 确保依赖解析可复现

### 3.3 pnpm-lock.yaml — 依赖唯一真相

```
自动生成,必须提交,禁止手动编辑,冲突用脚本解。
```

**作用:**
- 锁定所有依赖的精确版本
- 锁定依赖树结构
- 确保团队依赖一致

---

## 四、CI/CD 配置

### 4.1 GitHub Actions 示例

```yaml
- name: Setup Node & pnpm
  run: |
    # PVM 会自动读取 .pvmrc 锁定版本
    pvm use

- name: Install dependencies
  run: pnpm install --frozen-lockfile
  # 如果 lockfile 与 package.json 不一致 → 直接报错,拦截上线
```

### 4.2 为什么 --frozen-lockfile 很重要?

| 场景 | `pnpm install` | `pnpm install --frozen-lockfile` |
|------|----------------|----------------------------------|
| lockfile 与 package.json 一致 | ✅ 正常安装 | ✅ 正常安装 |
| lockfile 与 package.json 不一致 | ⚠️ 静默更新 lockfile | ❌ 直接报错 |
| 某个依赖有新版本 | ⚠️ 可能更新 | ❌ 严格使用 lockfile 版本 |
| 适合场景 | 本地开发 | **CI / 生产部署** |

**CI 环境使用 `--frozen-lockfile` 的好处:**
- 拦截不一致的 lockfile 提交
- 确保部署环境依赖版本一致
- 避免"在我机器上能跑"的问题

---

## 五、常见问题

### Q1: 如何检查当前项目依赖是否安全?

```bash
# 检查是否有 ^ ~ 等版本符号
grep -E '"[~^]' package.json

# 检查 lockfile 是否与 package.json 一致
pnpm install --frozen-lockfile --dry-run

# 检查依赖是否有已知漏洞
pnpm audit
```

### Q2: 想升级某个依赖怎么办?

```bash
# 手动升级(不自动使用 ^ ~)
pnpm add lodash@4.17.22

# 或:
pnpm update lodash@4.17.22

# 升级后 lockfile 会更新,把变更一起提交
git add package.json pnpm-lock.yaml
git commit -m "upgrade lodash to 4.17.22"
```

### Q3: 如何批量检查所有依赖的安全状态?

```bash
# 检查过时的依赖
pnpm outdated

# 检查依赖树结构
pnpm list --depth=0

# 检查 lockfile 完整性
pnpm install --lockfile-only
```

### Q4: 项目已经有版本漂移了怎么办?

```bash
# 1. 锁定工具链版本
pvm use pnpm@9 --local

# 2. 配置 .npmrc
# (使用 setup-npmrc.bat 脚本)

# 3. 重新生成 lockfile
rm pnpm-lock.yaml
pnpm install

# 4. 提交变更
git add .pvmrc .npmrc pnpm-lock.yaml
git commit -m "lock dependencies"
```

---

## 总结

依赖安全的本质就三件事:

| 层次 | 锁定目标 | 工具 | 配置文件 |
|------|---------|------|----------|
| **工具链** | pnpm 版本 | PVM | `.pvmrc` |
| **策略** | 安装行为 | .npmrc | `~/.npmrc` |
| **依赖** | 每个包的精确版本 | lockfile | `pnpm-lock.yaml` |

三道防线都锁死之后,你的项目:

- 今天能跑 → 明天能跑 ✅
- 你的电脑能跑 → 队友的电脑能跑 ✅
- 本地能跑 → CI 能跑 ✅
- CI 能跑 → 现网能跑 ✅

**"在我机器上能跑",从借口变成陈述。**

---

**如果这篇文章对你有帮助,别忘了:**
- ⭐ [给 GitHub 仓库点个 Star](https://github.com/Lycorxy/pvm)
- 💬 在评论区说说你遇到的依赖版本坑
- 🔗 转发给还在为"在我机器上能跑"困扰的队友