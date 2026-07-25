# 锁版本、冻依赖、防漂移 —— pnpm 依赖管理安全三部曲

> 你的项目昨天能跑，今天能跑，明天也能跑。不管换电脑、换 CI、换队友，都一样。

---

## 一、为什么"在我机器上能跑"是最危险的谎言

前端圈有个经典段子：代码 push 上去，CI 炸了，你喊一声"我本地没问题啊"。

问题出在哪？看几个真实案例：

### 案例 1：隐性版本升级

```json
// package.json 里写的
"lodash": "^4.17.20"

// 实际装的是
"lodash": "4.17.21"   // ← 悄悄更新了
```

`^` 符号允许安装 4.x.x 的最新版。今天跑 `pnpm install`，装了 4.17.21；明天新队友 clone 项目，装了 4.17.22 —— 多了一个 bugfix，但可能引入了一个你没测过的行为。

### 案例 2：lockfile 漂移

```yaml
# 你本地的 pnpm-lock.yaml
lodash: 4.17.21

# CI 上跑的 pnpm-lock.yaml (因为 pnpm 版本不同)
lodash: 4.17.21
  dependencies:
    some-sub-dep: 1.0.0  ← CI 上解析出不同的子依赖
```

**同一个 lockfile，不同 pnpm 版本可能解析出不同的依赖树。**

### 案例 3：peer dependency 地狱

```bash
# React 18 项目，装了某个基于 React 17 的 UI 库
npm ERR! peer react@"^17.0.0" from some-ui-lib@3.0.0
npm ERR! Found: react@18.2.0
```

版本不兼容报错还算好的。更可怕的是：**不报错，但运行时表现异常**。

---

## 二、三步锁死，让依赖永不漂移

### 第一步：锁 pnpm 版本（.pvmrc）

**核心问题：** 不同人用不同版本的 pnpm，`pnpm-lock.yaml` 的解析结果可能不一样。

**解决方案：** 用 PVM 锁死 pnpm 版本。

```bash
# 在项目根目录执行
pvm install pnpm@9       # 安装 pnpm 9
pvm use pnpm@9 --local   # 写入 .pvmrc，项目级锁定
```

执行后，项目根目录多了一个 `.pvmrc` 文件：

```ini
# .pvmrc
pnpm = 9.15.0
```

这个文件提交到 git 后，任何队友 `cd` 进项目，pnpm 自动锁定 9.15.0。**版本不统一？不存在了。**

---

### 第二步：冻依赖策略（.npmrc）

**核心问题：** `^` `~` `*` 这些语义化版本符号是依赖漂移的根源。

**解决方案：** 配置 `.npmrc` 锁死安装策略。

```bash
# PVM Skill 一键配置
.codebuddy\skills\pvm-env-manager\scripts\setup-npmrc.bat --force
```

这会在你全局 `~/.npmrc` 中写入（不影响其他项目的话可用项目级 `.npmrc`）：

```ini
# 1. 精确版本，不加 ^ ~
save-prefix=""

# 2. 优先使用现有 lockfile，不自动更新
prefer-frozen-lockfile=true

# 3. 自动安装 peer 依赖
auto-install-peers=true

# 4. 基于时间的版本解析（可复现）
resolution-mode=time-based

# 5. 国内镜像加速
registry=https://registry.npmmirror.com
```

**效果对比：**

```bash
# 配置前
pnpm add lodash
# package.json: "lodash": "^4.17.21"  ← 带 ^，会漂移

# 配置后
pnpm add lodash
# package.json: "lodash": "4.17.21"    ← 精确版本，锁死
```

---

### 第三步：正确使用 lockfile

**核心问题：** lockfile 是依赖的"唯一真相来源"，不是装饰品。

**正确姿势：**

```bash
# 开发环境：正常安装（尊重 lockfile）
pnpm install

# 提交前：格式化 lockfile（可选）
pnpm install --lockfile-only

# CI 环境：严格模式，lockfile 不一致 = 直接报错
pnpm install --frozen-lockfile
```

**lockfile 必须提交到 git！** 看到 `.gitignore` 里有 `pnpm-lock.yaml`？立刻删掉那条规则。

**lockfile 合并冲突怎么办？**

```bash
# 不要手动编辑 lockfile！用 PVM Skill 自动处理
.codebuddy\skills\pvm-env-manager\scripts\resolve-lockfile-conflict.bat

# 脚本自动做的事：
# 1. git checkout --theirs（清除冲突标记）
# 2. pnpm install --lockfile-only（基于 package.json 重新生成）
# 3. git add pnpm-lock.yaml
```

---

## 三、完整的项目安全配置模板

提交到 git 的三个文件，构成依赖安全的三角形：

### .pvmrc — 锁定工具链版本

```ini
node = 20.18.0
pnpm = 9.15.0
```

### .npmrc — 锁定安装策略

```ini
save-prefix=""
prefer-frozen-lockfile=true
auto-install-peers=true
resolution-mode=time-based
```

### pnpm-lock.yaml — 依赖唯一真相

```
自动生成，必须提交，禁止手动编辑，冲突用脚本解。
```

---

## 四、CI/CD 流水线配置

### GitHub Actions

```yaml
- name: Setup Node & pnpm
  run: |
    # PVM 会自动读取 .pvmrc 锁定版本
    pvm use

- name: Install dependencies
  run: pnpm install --frozen-lockfile
  # 如果 lockfile 与 package.json 不一致 → 直接报错，拦截上线
```

### 为什么 --frozen-lockfile 很重要？

| 场景 | `pnpm install` | `pnpm install --frozen-lockfile` |
|------|----------------|----------------------------------|
| lockfile 与 package.json 一致 | ✅ 正常安装 | ✅ 正常安装 |
| lockfile 与 package.json 不一致 | ⚠️ 静默更新 lockfile | ❌ 直接报错 |
| 某个依赖有新版本 | ⚠️ 可能更新 | ❌ 严格使用 lockfile 版本 |
| 适合场景 | 本地开发 | **CI / 生产部署** |

---

## 五、常见问题

### Q: `pnpm install` 报 EPERM 权限错误？

```bash
# 原生二进制（esbuild/rollup/swc）被进程占用
# PVM Skill 一键修复：
.codebuddy\skills\pvm-env-manager\scripts\fix-pnpm-eperm.bat --reinstall
```

### Q: 如何检查当前项目依赖是否安全？

```bash
# 检查是否有 ^ ~ 等版本符号
grep -E '"[~^]' package.json

# 检查 lockfile 是否与 package.json 一致
pnpm install --frozen-lockfile --dry-run

# 检查依赖是否有已知漏洞
pnpm audit
```

### Q: 想升级某个依赖怎么办？

```bash
# 手动升级（不自动使用 ^ ~）
pnpm add lodash@4.17.22

# 或：
pnpm update lodash@4.17.22

# 升级后 lockfile 会更新，把变更一起提交
git add package.json pnpm-lock.yaml
git commit -m "upgrade lodash to 4.17.22"
```

---

## 六、总结

依赖安全的本质就三件事：

| 层次 | 锁定目标 | 工具 | 配置文件 |
|------|---------|------|----------|
| **工具链** | pnpm 版本 | PVM | `.pvmrc` |
| **策略** | 安装行为 | .npmrc | `~/.npmrc` |
| **依赖** | 每个包的精确版本 | lockfile | `pnpm-lock.yaml` |

三层都锁死之后，你的项目：

- 今天能跑 → 明天能跑 ✅
- 你的电脑能跑 → 队友的电脑能跑 ✅
- 本地能跑 → CI 能跑 ✅

**"在我机器上能跑"，从借口变成陈述。**
