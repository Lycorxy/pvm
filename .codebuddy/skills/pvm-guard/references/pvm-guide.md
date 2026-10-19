# pvm 使用指南（pvm-guard 查表手册）

> 事实来源：pvm 仓库（cmd/root.go 帮助、dockit 文档）。回答用法前先查本表，禁止编造命令。

## 目录

1. pvm 是什么 / 版本解析链
2. 安装与卸载
3. 运行时命令速查
4. .pvmrc（项目版本声明）
5. pvm.yaml（流水线）
6. pvm pack（跨平台打包）
7. 排障速查

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

## 5. pvm.yaml（流水线）

放项目根（monorepo 根放一个，子目录向上查找；`.yml` 兜底）。**人手写，pvm 只读**。

```yaml
name: my-app                 # 项目名（可选，--list 显示）
dev:                         # 嵌套键 = 流水线名；2 层 动作:环境，3 层 动作:目标:环境
  web:
    - cd web && pnpm i && npm run dev
  api:
    - go run ./cmd/api
  all:
    - "&@dev:web"            # & 后台并行（dev 服务器不阻塞）；@ 引用其他流水线
    - "&@dev:api"            # @ 与 & 是 YAML 保留字符，必须加引号
build:
  pro:
    - pnpm run build:pro
pkg:
  go:
    pro:
      - pvm pack go --entry . --output dist/app
release:
  pro:
    - "@build:pro"           # 递归展开；循环引用会报错
    - "@pkg:go:pro"
```

- 叶子 = 命令列表（多步）或单字符串（单步），**原样执行**；同一 shell 会话顺序跑，工作目录 = pvm.yaml 所在目录。
- 执行前自动安装并激活 .pvmrc 声明的运行时版本——流水线里无需写环境安装命令。
- 运行：`pvm run --list`（列出全部）、`pvm run dev:all`、`pvm run release:pro --dry-run`（只打印计划）。
- 后台作业（`&`）：末尾等待全部结束，任一失败即报错；Ctrl+C 全部停止。

## 6. pvm pack（跨平台打包）

默认**跨平台全量**打包为单可执行文件；`--platform` 指定则只打对应目标。

| 取值 | 行为 |
|------|------|
| 省略 / `all` | 全量矩阵：windows/amd64, linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 |
| `host` / `current` | 仅当前平台 |
| `<os>/<arch>` | 只打一个；逗号分隔可多个（自动去重） |

```
pvm pack go     [--entry <pkg>] [--output <path>] [--platform <t>]
pvm pack node   --entry <main.js> [--bundled <file>] [--output <path>] [--platform <t>] [--official]
pvm pack python --entry <main.py> [--output <path>] [--platform <t>]
```

- 产物命名：多目标时在 `--output` 扩展名前插 `-<os>-<arch>`（dist/app → dist/app-linux-amd64）；
  Windows 目标自动补 `.exe`，非 Windows 目标自动去掉误写的 `.exe`。
- **go**：`go build -trimpath -ldflags "-s -w"`，交叉编译自动 `CGO_ENABLED=0`（静态产物）。
- **node**：官方 SEA（要求 Node ≥ 20），esbuild bundle → blob → postject 注入。
  跨平台时自动下载各目标平台官方 node 发行版作底座（缓存 `~/.pvm/cache/pack-node/`，重复打包零下载）；
  `--official` 强制官方源（默认国内镜像）；`--bundled` 跳过 esbuild 用预打包产物。
- **python**：PyInstaller `--onefile`（缺失自动 pip 安装）。**PyInstaller 不支持交叉打包**：
  默认仅产出当前平台，`--platform` 指定非当前平台会直接报错（去各目标平台或 CI matrix 跑）。
- PyInstaller 例外：postject 对 PE 的「signature corrupted」告警为已知噪音，产物可运行。

## 7. 排障速查

| 症状 | 原因 → 处理 |
|------|------|
| nvm 劫持 node（`pvm which node` 指向 nvm 目录） | pvm shims 未前置 → `pvm setup-path`（需管理员，会 UAC）→ **新开终端**再验 |
| 切版本后 IDE 终端还是旧版本 | IDE 启动时缓存 PATH，**完全退出重启 IDE**（仅 Reload Window 不够） |
| 「装了但命令找不到」 | 先 `pvm reshim`（重建 shim），再查 PATH；别急着重装 |
| shim/版本行为异常 | `pvm validate --auto-fix`（升级/重装后首选） |
| 环境整体不明 | `pvm doctor`（7 项体检）+ `pvm diagnostics <rt>` |
| 下载 403/慢 | 默认已走国内镜像；`--official` 切官方源，反之亦然 |
| `pvm run` 提示 Unknown command: run | pvm 版本过旧 → `pvm self-update` 或重装 |
| PATH 修复不生效 | PATH 在进程启动时读取：新开终端 / 重启 IDE / 重启机器（系统级 PATH 时） |
