# pvm 使用指南（pvm 官方查表手册）

> 事实来源：pvm 仓库（cmd/root.go 帮助、dockit 文档）。回答用法前先查本表，禁止编造命令。

## 目录

0. 能力与局限（边界，先读）
1. pvm 是什么 / 版本解析链
2. 安装与卸载
3. 运行时命令速查
4. .pvmrc（项目版本声明）
5. pvm.yaml（流水线）
6. pvm pkg（打包）与 zip/unzip
7. 排障速查

## 0. 能力与局限（边界，先读）

**能做什么（功能范围）：**

- 版本管理：9 种运行时（node/bun/deno/python/pnpm/yarn/go/git/rust）的安装、切换、声明式锁定（.pvmrc）、shim 统一切换
- 项目环境：.pvmrc 提交 Git 后团队版本一致；`pvm install`/`pvm use` 自动装齐激活
- 流水线：pvm.yaml 声明构建/打包/发布流程，`pvm run` 一条命令执行，执行前自动装齐运行时
- 打包：`pvm pkg` 把 go/node/python 项目打成单可执行文件（默认 5 平台矩阵），`--include` 可把配置文件/资源一并内嵌
- 压缩：`pvm zip` / `pvm unzip` 通用压缩解压（并行、可复现、防 zip-slip）
- AI 协作：本 skill（环境守护 harness + 官方查表手册）

**局限与边界（做不到的，提前说清）：**

| 局限 | 说明 | 应对 |
|------|------|------|
| python 打包不能交叉 | PyInstaller 硬限制，`pvm pkg python` 仅产出当前平台；`--type` 指定其他平台直接报错 | 各目标平台分别跑，或 CI matrix |
| node SEA 要求 Node ≥ 20 | 低版本 node 直接拒绝打包；`--include` 还要求 ≥ 20.12 | `pvm install node@lts` 后重试 |
| go 不能用 `--include` | Go 只能编译期 `//go:embed`，pvm 改不了源码 | 在源码里写 `//go:embed`，`pvm pkg go` 会对此直接报错 |
| darwin 产物在非 macOS 主机上未签名 | 注入会破坏签名，非 macOS 主机没有 codesign | 在 macOS 上 `codesign --sign -` 补签后分发 |
| go 交叉编译要求纯 Go | 交叉编译强制 `CGO_ENABLED=0`，依赖 cgo 的项目交叉会失败 | 在对应平台原生构建，或消除 cgo 依赖 |
| go/git/rust 全局唯一 | 不支持项目级 .pvmrc，同一时刻只保留一个版本 | 用 `--user`/`--system` |
| self-update 只走 Gitee | GitHub Release 不会被 `pvm self-update` 看到 | 发布时 Gitee 同步建 release |
| 无代码签名 | Windows Defender/360 可能误报 | 「仍要运行」；已嵌 manifest 降低误报 |
| 环境变量进程级缓存 | PATH/版本修复后，已开的终端/IDE 读不到 | 新开终端 / 完全重启 IDE |
| pvm.yaml 人手写、pvm 只读 | pvm 不生成/改写 pvm.yaml；规则只有四条 | 照第 5 节格式手写，写后 `pvm run` 验证 |
| 不管应用依赖 | node_modules/pip 包等应用级依赖归 npm/pip 管，pvm 只管运行时版本 | 依赖安装走 code-env / 各包管理器 |

## 1. pvm 是什么 / 版本解析链

pvm = Polyglot Version Manager：单二进制，用 shim 机制统一管理 node/python/bun/deno/pnpm/yarn/go/git/rust。

版本解析优先级（高→低）：
1. 环境变量 `PVM_<RT大写>_VERSION`（如 PVM_NODE_VERSION）
2. 项目 `.pvmrc` / `.nvmrc`（从当前目录向上查找）
3. 用户全局 `~/.pvm/versions`
4. 系统 PATH 中已装版本（`system` 标记，自动回退）

目录约定：`PVM_HOME`（默认 `~/.pvm`）→ `bin/`（pvm 本体）、`shims/`（唯一需要进 PATH 的目录）、
`installs/<rt>/<version>/`（各运行时）、`cache/`（下载缓存）。

## 2. 安装与卸载

| 平台 | 命令（国内 gitee 镜像优先） |
|------|------|
| Windows PowerShell | `irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 \| iex` |
| Windows MSI | gitee.com/lycorxy/pvm/releases 下载对应平台 msi 双击 |
| macOS / Linux | `curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh \| bash` |
| 国际网络 | 把 gitee.com/lycorxy 换成 raw.githubusercontent.com/Lycorxy（master 分支） |

- 安装后：新开终端 → `pvm --version` 出版本即成功；首次跑 `pvm setup`（目录+PATH+shims）。
- 安装方式选择（Windows）：`pvm setup --scope user|admin`。`user`（默认）全程无需管理员；
  `admin` 弹 UAC 提权重跑，额外把 pvm shims 前置到系统 PATH。一键安装脚本对应 `-Scope admin`。
- **文件所有权守护**：以管理员（Windows）/ sudo（Unix）跑完 setup 会自动把 `~/.pvm`
  所有权归还给当前用户；历史污染用 `pvm repair-owner` 一次修复（详见排障节）。
- Apple Silicon / ARM：安装脚本自动识别架构，无需选择。
- 升级：`pvm self-update`。
- 卸载：`pvm uninstall`（会列 PATH 条目需重启终端生效；不会删运行时数据时按提示确认）。
- Windows 安全提示：无代码签名证书，Defender/360 可能误报，「仍要运行」即可。

## 3. 运行时命令速查

```
pvm install node@20.20.2            # 安装（可多个；--local 写项目 .pvmrc）
pvm install                         # 无参数：按 .pvmrc → 用户全局 自动装齐
pvm use node@20.20.2 [--local]      # 切换；--local 写项目 .pvmrc；--system 用系统版本
pvm list / list <rt> / list-remote <rt>
pvm current                         # 当前生效版本（按解析链）
pvm which node                      # node 实际解析到的二进制路径
pvm where node                      # node 安装目录
pvm remove node@20.20.2             # 卸载某版本
pvm doctor                         # 环境体检
pvm validate [--auto-fix]          # shim/版本一致性深度校验，--auto-fix 自动修复
pvm diagnostics <rt>                # 单运行时详细诊断
pvm reshim                         # 重建 shim（装了新二进制后）
pvm config init/show/set/remove     # .pvmrc 管理
全局标志: --user(默认) --local --system --mirror(默认国内镜像) --official --force --verbose
```

- 全局唯一运行时：go/git/rust 只支持用户级/系统级，不进 .pvmrc。
- Windows 已装 Git for Windows：`pvm install git` 自动 adopt（建 junction 不重新下载）。

## 4. .pvmrc（项目版本声明）

- 格式：每行一条 `runtime version`；`runtime system` = 用系统 PATH 中的版本。`#` 注释。
  - 例：`node 20.20.2` / `pnpm 8.15.9` / `python 3.12.0` / `node system`
- 由 pvm 自动整体读写（`pvm use --local` 会重写），人手改动只用于初始声明。
- 提交到 Git：团队成员 clone 后 `pvm use` / `pvm install` 即获得一致版本。
- 项目级优先于用户全局、低于环境变量。
- **兼容 nvm**：同一目录的 `.nvmrc` 也会被读取（两者都在时 `.pvmrc` 优先）；pvm 只读不写 `.nvmrc`，
  迁移不需要改文件。`lts/*` 这类需要联网解析的写法会被忽略，请换成具体版本号（如 `20.20.2`）。
- 因此可以卸掉 nvm：删 `~/.nvm` 与 shell 配置里的 `nvm.sh`，`.nvmrc` 留着照样生效。

## 5. pvm.yaml（流水线）

放项目根（monorepo 根放一个，子目录向上查找；`.yml` 兜底）。**人手写，pvm 只读**。

规则只有四条：

```yaml
name: my-app            # 项目名（可选，列表里显示）

install: pnpm i         # 1) 名字: 命令 = 一条命令
lint: pnpm lint
test: pnpm test
build: pnpm build

check:                  # 3) parallel: 加列表 = 同时执行
  parallel: [lint, test]

release:pro:            # 2) 名字: 加列表 = 从上到下依次执行，失败即停
  - install
  - check               # 4) 步骤名等于某条流水线名 = 引用，否则当命令执行
  - build
  - pvm pkg go --entry . --output dist/app
```

- 顺序列表里连续的命令共用一个 shell 会话（`cd` 会保留），工作目录 = pvm.yaml 所在目录；任一步非零退出即停止，后续步骤不执行。
- 并行组每个分支独立进程，输出带 `[名字]` 前缀；任一分支失败即终止其余分支。
- `parallel:` 可写在顶层，也可写在列表里的某一项（两种写法等价）；`--dry-run` 以树形打印计划。
- Shell：Unix 用 bash；Windows 优先 pwsh（PowerShell 7），未装回退 5.1——5.1 不支持 `&&`/`||`，pvm 会提前报错并给改法（装 pwsh / 拆步骤 / 用 `;`）。
- 命令里的文本带空格时加引号：PowerShell 的 `echo a b c` 会输出三行，`echo "a b c"` 才是一行。
- 执行前自动安装并激活 .pvmrc 声明的运行时版本——流水线里无需写环境安装命令。
- 运行：`pvm run`（列出全部）、`pvm run release:pro`、`pvm run release:pro --dry-run`、`pvm run -C <目录> <名字>`。
- 没有后台作业语法（`&` 已移除）；需要常驻进程（dev server）就单独开终端，并行组会等所有分支结束。

## 6. pvm pkg（打包）与 zip/unzip

`pvm pkg` 把项目打成单可执行文件；`--type` 控制目标，默认 `all` = 5 平台矩阵。

```
pvm pkg go     [--entry <pkg>]   [--output <path>] [--type <t>] [--zip]
pvm pkg node   --entry <main.js> [--bundled <file>] [--output <path>] [--type <t>] [--official]
pvm pkg python --entry <main.py> [--output <path>] [--type <t>]

--include <src>   随产物分发的文件/目录（可重复）
--external <pkg>  node：esbuild 不打包的模块（可重复），如 --external pino-pretty
--libc <kind>     node linux 基座：default（官方，glibc ≥ 2.28）/ legacy（glibc-217，CentOS 7 可用，自动下载）
--base <file>     node：完全自定义基座（优先级最高，配单目标）
--zip             每个产物额外打一个同名 zip
--verbose         透传 esbuild/pyinstaller 日志（默认只打结果）
```

- node 与 go 同形：`pvm pkg node --entry <入口> --output <产物> --type <目标>`；
  只有部署到老 glibc 时才多一个 `--libc legacy`。
- 官方 node 基座要求 glibc ≥ 2.28；CentOS 7（2.17）用 `--libc legacy`，pvm 自动下载 glibc-217 构建。
- SEA 哨兵按基座二进制实际值扫描，不硬编码，基座换版本不会失配。

`--type` 取值：

| 取值 | 行为 |
|------|------|
| 省略 / `all` | 全量矩阵：windows/amd64, linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 |
| `host` | 仅当前平台 |
| `win` / `linux` / `mac` | 该系统的常用架构（win=amd64；linux、mac=amd64+arm64） |
| `linux-arm64` | 系统-架构，指定单个 |
| `win,linux` | 逗号分隔多选，自动去重 |

- 产物命名：多目标时在 `--output` 扩展名前插 `-<os>-<arch>`（dist/app → dist/app-linux-amd64）；
  Windows 目标自动补 `.exe`，非 Windows 目标自动去掉误写的 `.exe`。
- **go**：`go build -trimpath -ldflags "-s -w"`，交叉编译自动 `CGO_ENABLED=0`（静态产物）。
- **node**：官方 SEA（要求 Node ≥ 20），esbuild bundle → blob → postject 注入。
  跨平台时自动下载各目标平台官方 node 发行版作底座（缓存 `~/.pvm/cache/pack-node/`）；
  `--official` 强制官方源（默认国内镜像）；`--bundled` 跳过 esbuild 用预打包产物。
- **python**：PyInstaller `--onefile`（缺失自动 pip 安装）。**不支持交叉**：
  `--type all` 自动收敛到当前平台，显式指定其他平台直接报错（去各目标平台或 CI matrix 跑）。
- postject 对 PE 的 `The signature seems corrupted!` 是已知噪音，产物可运行；分发前用 `signtool` 重签。

### 配置/资源一起打包（--include）

```
--include config.yaml                包内 config.yaml
--include assets                     包内 assets/...
--include assets/                    只打内容（无 assets 前缀）
--include config.yaml:conf/app.yaml  包内改名（python 只能改目录，不能改文件名）
```

| 运行时 | 落到哪 | 代码里怎么读 |
|--------|--------|--------------|
| node | SEA `assets`（需 ≥ 20.12） | `require('node:sea').getAsset('config.json','utf8')` |
| python | PyInstaller `--add-data` | `pathlib.Path(sys._MEIPASS) / 'config.json'` |
| go | 不支持，直接报错 | 用 `//go:embed` 编译期内嵌 |

`sea.getAsset()` 只在产物里可用，源码直接跑会抛 `ERR_UNKNOWN_BUILTIN_MODULE`，代码里要 try/catch 回退读磁盘。
`--include` 的文件必须在打包前存在（先生成配置，再打包）。

### zip / unzip

```
pvm zip <src...> -o app.zip [-x <glob>] [--reproducible]
pvm unzip app.zip [-o <dir>] [-l]
```

- 目录写法同 `cp -r`：`dist` 保留外层目录名，`dist/` 或 `.` 只打目录内容。
- `-x` / `--exclude` 可重复（如 `-x node_modules -x "*.map"`）；`--reproducible` 固定时间戳与权限，同样输入得到逐字节相同的 zip。
- 解压防 zip-slip、限制单文件 2GB / 总量 10GB / 10 万文件，GBK 文件名自动转码；`-l` 只看内容。

## 7. 排障速查

| 症状 | 原因 → 处理 |
|------|------|
| nvm 劫持 node（`pvm which node` 指向 nvm 目录） | pvm shims 未前置 → `pvm setup-path`（需管理员，会 UAC）→ **新开终端**再验。若决定统一到 pvm，可直接卸载 nvm（`.nvmrc` 仍被 pvm 读取） |
| 切版本后 IDE 终端还是旧版本 | IDE 启动时缓存 PATH，**完全退出重启 IDE**（仅 Reload Window 不够） |
| 「装了但命令找不到」 | 先 `pvm reshim`（重建 shim），再查 PATH；别急着重装 |
| shim/版本行为异常 | `pvm validate --auto-fix`（升级/重装后首选） |
| 环境整体不明 | `pvm doctor`（7 项体检）+ `pvm diagnostics <rt>` |
| 下载 403/慢 | 默认已走国内镜像；`--official` 切官方源，反之亦然 |
| `pvm run` 提示 Unknown command: run | pvm 版本过旧 → `pvm self-update` 或重装 |
| `pvm pkg` / `pvm zip` 提示 Unknown command | 同上：旧版只有 `pack` 且没有 zip → `pvm self-update` 或装新版 |
| `pvm run` 报 circular reference | pvm.yaml 里流水线互相引用（含自引用）→ 按报错的 `a → b → a` 链路拆开 |
| **普通终端装包永远失败，管理员身份打开就成功**（删除重装也无效） | 曾用管理员（Windows）/ sudo（Unix）安装或装包，`~/.pvm` 下文件归 Administrators/root 所有，普通权限写不进去 → `pvm repair-owner` 一次修复（Windows 弹 UAC 自动修+复验；Unix 提示 `sudo pvm repair-owner`），之后用**普通权限**重装包；新版管理员模式 setup 会自动归还所有权，不会复发 |
| PATH 修复不生效 | PATH 在进程启动时读取：新开终端 / 重启 IDE / 重启机器（系统级 PATH 时） |
| 同一环境多项目并发，git 提交人串了 | 各仓库 `git config user.name/email`（**--local 优先于 global**，仓库间天然隔离）；pvm 不管 git 身份 |
| `pvm run` 报 uses '&&' or '\|\|' ... PowerShell 5.1 | 未装 PowerShell 7 → `winget install Microsoft.PowerShell`（pvm 自动优先用 pwsh），或拆步骤 / 用 `;` |
