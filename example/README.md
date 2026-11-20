# pvm 功能验证案例

三个真实项目的完整流程 + 一个纯流水线语义演示，全部可重复执行。

| 目录 | 演示什么 | 需要 |
| --- | --- | --- |
| `pipeline-demo/` | 流水线语义：串行、并行、引用、失败即停 | 无（只用 `echo` / `exit`） |
| `go-hello/` | 并行检查 → 构建 → 5 平台打包 + zip | go |
| `node-hello/` | 脚本生成配置 → Node SEA 单文件（配置内嵌）；自带 `.pvmrc` 锁 node 版本 | node >= 20.12、npx |
| `python-hello/` | 脚本生成配置 → PyInstaller 单文件（配置内嵌）；自带 `.pvmrc` 锁 python 版本 | python（缺 PyInstaller 会自动装） |
| `token-compare.ps1` | 命令级实测：每条命令的真实输出 token（tiktoken） | python + tiktoken（自动装） |
| `token-task.ps1` | 任务级实测：手工 12 步打 node 单文件 vs pvm 3 步，两条都真跑 | 同上 |

一键全量验证：

```powershell
.\verify.ps1                  # go + node + python，逐项 PASS/FAIL
.\verify.ps1 -SkipNode        # 跳过 Node SEA（首次要下载 esbuild/postject）
.\verify.ps1 -SkipNode -SkipPython
```

脚本会先把当前仓库的 pvm 构建到 `dist\pvm.exe`，并把它放到 PATH 最前面——
所以案例流水线里的 `pvm pkg ...` 用的是**刚构建的版本**，不是系统里已安装的旧版。

两个测量脚本都用 `tiktoken`（cl100k_base）对真实输出编码，不做"字符数 ÷ 4"的估算：

```powershell
.\token-compare.ps1   # 命令级：摸清项目 / 切版本 / 执行链路 / 失败信息 / skill 体积
.\token-task.ps1      # 任务级：A 路径手工 12 步，B 路径 pvm 3 步，产物都会真跑一次
```

原始输出落在 `.work/cmp-logs` 与 `.work/task-logs`，CSV 在 `.work/*.csv`。
注意：脚本只统计"输出进上下文的 token"，不含模型推理和工具调用开销。

## 流水线规则（只有四条）

```yaml
名字: 命令                 # 一条命令
名字:                      # 列表 = 从上到下依次执行，失败即停
  - 命令
  - parallel: [a, b]       # 列表里可以嵌并行
名字:                      # parallel = 同时执行，任一失败则停止其余
  parallel: [a, b]
# 步骤名等于某条流水线名 = 引用，否则当命令执行
```

入口统一是 `pvm run`：

| 命令 | 作用 |
| --- | --- |
| `pvm run` | 列出全部流水线及一行概要 |
| `pvm run <名字>` | 执行 |
| `pvm run <名字> --dry-run` | 树形打印执行计划，不执行 |
| `pvm run -C <目录>` | 从别处执行某个项目（向上查找该目录的 pvm.yaml） |

`pvm.yaml` 从当前目录向上查找，命令的工作目录是 `pvm.yaml` 所在目录。

## pipeline-demo

```powershell
cd pipeline-demo
pvm run                      # 列表
pvm run release:pro --dry-run
pvm run dev
```

`pvm run release:pro --dry-run` 的真实输出：

```
release:pro
├─ hello: echo "hello from pvm.yaml"
├─ parallel
│  ├─ lint: echo "lint ok"
│  └─ test: echo "test ok"
└─ echo "packaged"
```

`pvm run dev`（`check` 被引用，展开成并行，输出带 `[名字]` 前缀）：

```
▶ dev
==> echo "hello from pvm.yaml"
hello from pvm.yaml
▶ check (parallel)
[lint] ==> echo "lint ok"
[test] ==> echo "test ok"
[lint] lint ok
[test] test ok
[lint] ✔ 2.1s
[test] ✔ 2.2s
==> echo "dev ready"
dev ready
✔ dev  5.9s
```

`pvm run fail` 验证失败即停（第 3 步不执行，退出码 1）：

```
▶ fail
==> echo "step 1 ok"
step 1 ok
==> exit 3
✘ fail  failed after 1.9s
Error: pipeline "fail" failed: exit 3
```

## go-hello

```powershell
cd go-hello
pvm run release:dev    # check(并行) → build → run
pvm run release:pro    # check → pvm pkg go --type all --zip
```

`release:pro` 真实输出（本机实测，5 个产物 + 5 个 zip，总 46 秒）：

```
▶ release:pro
▶ check (parallel)
[fmt ] ==> gofmt -l .
[test] ==> go test ./...
[vet ] ==> go vet ./...
[fmt ] ✔ 3.9s
[test] ok  	example/go-hello	(cached)
[vet ] ✔ 5.5s
[test] ✔ 5.6s
==> pvm pkg go --entry . --output dist/go-hello --type all --zip
==> pvm pkg go: 5 targets (windows/amd64, linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
==> [1/5] go build windows/amd64 -> dist\go-hello-windows-amd64.exe
...
==> done: 5 artifact(s)
    TARGET               FILE                             SIZE      TIME
    windows/amd64        dist\go-hello-windows-amd64.exe  1.6 MB    1.54s
    linux/arm64          dist\go-hello-linux-arm64        1.5 MB    10.37s
    ...
    windows/amd64 (zip)  dist\go-hello-windows-amd64.zip  758.0 KB  50ms
✔ release:pro  45.9s
```

产物可直接运行，zip 内保持原文件名：

```
> .\dist\go-hello-windows-amd64.exe pvm
go-hello: pvm (windows/amd64)

> pvm unzip dist/go-hello-linux-amd64.zip -l
2026-11-18 06:10  1.5 MB  go-hello-linux-amd64
```

## node-hello

```powershell
cd node-hello
pvm run release:dev    # install → config:dev → start → pkg
```

流程是"先用普通脚本生成配置，再打包时把配置一起塞进产物"：

```yaml
config:dev: node scripts/gen-config.js dev
pkg: pvm pkg node --entry src/main.js --include dist/config.json --output dist/node-hello --type host
```

```
> .\dist\node-hello.exe pvm
node-hello: pvm (win32/x64, env=dev, port=3000)
```

把产物复制到**没有** `dist/config.json` 的目录再运行，输出不变 —— 说明配置真的内嵌在 SEA blob 里：

```powershell
> copy .\dist\node-hello.exe $env:TEMP\iso\; cd $env:TEMP\iso; .\node-hello.exe isolated
node-hello: isolated (win32/x64, env=dev, port=3000)
```

SEA 流程是 esbuild 打包 → `node --experimental-sea-config`（带 `assets`）生成 blob → postject 注入。
打包时 postject 可能打印 `The signature seems corrupted!`：这是注入改写了 PE 证书区，
不影响运行；正式分发前在 Windows 上用 `signtool` 重签即可。产物约 69 MB（= node 二进制 + blob）。

## python-hello

```powershell
cd python-hello
pvm run release:dev    # config:dev → start → pkg
```

```
> .\dist\python-hello.exe pvm
python-hello: pvm (windows/AMD64, env=dev, port=3000)
```

配置通过 PyInstaller `--add-data` 打进产物，代码里读 `sys._MEIPASS`。
PyInstaller 不能交叉编译：`--type all` 会自动收敛到当前平台，显式指定其他平台会直接报错。

## 配置和资源一起打包（--include）

`pvm pkg` 用 `--include` 把额外文件塞进单文件产物；怎么"读出来"由各运行时的官方 API 决定：

| 运行时 | pvm 参数落到哪 | 代码里怎么读 |
| --- | --- | --- |
| node | SEA `assets`（>= 20.12） | `require('node:sea').getAsset('config.json','utf8')` |
| python | PyInstaller `--add-data` | `pathlib.Path(sys._MEIPASS) / 'config.json'` |
| go | **不支持** | 用 `//go:embed` 在编译期内嵌 |

包内路径规则与 `pvm zip` 保持一致：

```
--include config.yaml                包内 config.yaml
--include assets                     包内 assets/...
--include assets/                    只打内容，包内无 assets 前缀
--include config.yaml:conf/app.yaml  包内改名（python 不能改名，会报错）
```

go 用 `--include` 会直接报错并给出 `//go:embed` 示例，避免"以为打进去了"。

## zip / unzip

```powershell
pvm zip go-hello -o dist/app.zip -x dist        # 排除 dist 目录
pvm zip go-hello/ -o dist/contents.zip          # 尾斜杠 = 只打内容，不带外层目录名
pvm zip go-hello -o a.zip --reproducible        # 固定时间戳/权限，两次产物字节相同
pvm unzip dist/app.zip -l                       # 只看内容
pvm unzip dist/app.zip -o out                   # 解压
```

本机实测：69 MB 的 node 产物压缩到 28.5 MB（41%），解压后仍可运行；
`--reproducible` 两次产物 SHA256 相同；`../../evil.txt` 这类路径会被拒绝。

## 踩过的坑（写案例时踩到的真实问题）

1. **PowerShell 的 `echo a b c` 会输出三行**。`echo hello from pvm.yaml` 在 `sh` 是一行，
   在 PowerShell 是三个对象三行。案例里统一加引号：`echo "hello from pvm.yaml"`。
2. **`go build -o dist/go-hello .` 在 Windows 产出的文件没有 `.exe`**，PowerShell 无法直接执行。
   写成 `go build -o dist/ .`，go 会按平台自动补 `.exe`。
3. **旧版 pvm 可能还在 PATH 里**。`pvm zip` 报 `Unknown command` 时，用 `Get-Command pvm` 确认
   命中的是哪个二进制；本脚本已把新构建的放到最前面。
4. **并行分支各自独立进程**，`cd` 不会跨分支生效；同一列表里连续的命令共用一个 shell，`cd` 会保留。
5. **并行没有并发上限**：声明几个就同时跑几个（dev 服务器这类长驻任务不会被后面的任务饿死）。
6. **PowerShell 函数名不能叫 `Pvm`**：PowerShell 不区分大小写，函数 `Pvm` 会遮住外部命令 `pvm`，
   `& pvm ...` 变成递归调用自己，症状是"命令执行成功但没有输出"。`verify.ps1` 里因此用 `Invoke-Pvm`。
7. **`verify.ps1` 必须存成 UTF-8 with BOM**：PowerShell 5.1 对无 BOM 文件按 ANSI(GBK) 解码，
   中文注释可能把解析搞坏。仓库里这份已经是 BOM 版本，编辑后请检查 BOM 是否还在。
8. **不要并发跑多个 `verify.ps1`**：它们会同时 `go build -o dist\pvm.exe` 并互相删 `dist`，
   导致 `pvm` 解析失败、产物消失。
9. **`sea.getAsset()` 只在单文件产物里可用**：`node src/main.js` 直接跑会抛
   `ERR_UNKNOWN_BUILTIN_MODULE`，代码里要 `try { getAsset } catch { 读磁盘 }`，
   这样源码和产物都能跑（案例的 `src/main.js` 就是这么写的）。
10. **`--include` 的文件要在打包前存在**：先生成配置再打包，所以案例里 `pkg` 依赖
    `config:dev` / `config:pro`；单独跑 `pvm run pkg` 前要先跑一次配置步骤。
