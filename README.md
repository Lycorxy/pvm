# pvm - Polyglot Version Manager

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-blue)]()

一个 Rust 理念，Go 实现的**多语言版本管理器**。统一管理 node、python、go、rust、bun、deno、git 等多种运行时的版本，一个工具搞定所有。

---

## 为什么用 pvm？

| | nvm | pyenv | rustup | gvm | pvm |
|---|:---:|:---:|:---:|:---:|:---:|
| Node.js | ✅ | | | | ✅ |
| Python | | ✅ | | | ✅ |
| Rust | | | ✅ | | ✅ |
| Go | | | | ✅ | ✅ |
| Bun / Deno | | | | | ✅ |
| Git | | | | | ✅ |
| 一条命令切换 | | | | | ✅ |
| .pvmrc 项目配置 | | | | | ✅ |

**安装一个工具，管理所有运行时，零上下文切换，你只需要这一个。**

---

## 快速开始

### Windows

下载最新 [MSI 安装包](https://github.com/your-org/pvm/releases/latest)，双击安装。

或通过 PowerShell 安装：

```powershell
irm https://raw.githubusercontent.com/your-org/pvm/main/scripts/install.ps1 | iex
```

> **安全提示**：pvm 目前没有代码签名证书（$300-500/年），Windows Defender / 360 可能误报。点击「更多信息」→「仍要运行」即可。详见 [安全说明](#windows-安全提示)。

### macOS / Linux

```sh
curl -fsSL https://raw.githubusercontent.com/your-org/pvm/main/scripts/install.sh | bash
```

或手动安装：

```sh
git clone https://github.com/your-org/pvm.git ~/.pvm
cd ~/.pvm
go build -o bin/pvm .
export PATH="$HOME/.pvm/shims:$HOME/.pvm/bin:$PATH"
```

---

## 常用命令

```sh
# 安装运行时
pvm install node@20          # 安装 Node.js 20
pvm install python@3.12      # 安装 Python 3.12
pvm install go@1.22          # 安装 Go 1.22
pvm install rust@latest      # 安装最新 Rust

# 切换版本
pvm use node@20              # 用户级（全局生效）
pvm use node@20 --local      # 项目级（写入 .pvmrc）

# 查看状态
pvm list                     # 已安装的版本
pvm list node                # 查看所有可用版本
pvm doctor                   # 诊断环境配置
pvm current                  # 查看当前版本

# 管理
pvm setup                    # 初始化/修复环境
pvm self-update              # 更新 pvm 自身
```

---

## 支持的运行时

| 运行时 | 安装 | 用户级切换 | 项目级 (.pvmrc) | 国内镜像 |
|--------|:---:|:---:|:---:|:---:|
| **node** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **python** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **bun** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **deno** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **pnpm** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **yarn** | ✅ | ✅ | ✅ | ✅ npmmirror |
| **go** | ✅ | ✅ | — | ✅ golang.google.cn |
| **git** | ✅ | ✅ | — | ✅ npmmirror |
| **rust** | ✅ | ✅ | — | ✅ rsproxy.cn |

> go / git / rust 为全局工具，不支持项目级版本（实际场景中不需要项目级隔离）。

---

## 项目配置 (.pvmrc)

在项目根目录创建 `.pvmrc` 文件：

```ini
# .pvmrc
node = 20
python = 3.12
pnpm = 9
```

团队成员 clone 项目后，`pvm use` 自动切换到正确版本：

```sh
$ cd my-project
$ pvm use
✓ node 20.11.0
✓ python 3.12.3
✓ pnpm 9.1.0
```

---

## 工作原理

```
~/.pvm/
├── bin/           # pvm 主程序 + pvm-shim
├── shims/         # 统一 shim 目录（已加入 PATH）
├── installs/      # 运行时安装目录
│   ├── node/
│   │   ├── 20.11.0/
│   │   └── 22.0.0/
│   ├── python/
│   │   └── 3.12.3/
│   └── ...
└── versions       # 用户级版本配置
```

### Shim 机制

```
$ which node
~/.pvm/shims/node          ← shim 拦截

$ node -v
v20.11.0                   ← shim 转发到当前激活版本
```

所有命令通过 `~/.pvm/shims/` 拦截，根据当前配置自动转发到正确的版本。**无需修改任何项目脚本，PATH 配置一次即可。**

---

## 从其他工具迁移

```sh
# 从 nvm 迁移
nvm current                 # 记下当前版本
pvm install node@<version>  # 安装相同版本
pvm use node@<version>

# 从 pyenv 迁移
pyenv version                # 记下当前版本
pvm install python@<version>
pvm use python@<version>
```

---

## 国内镜像

遇到 `GitHub API 403` 或下载慢时，pvm 会自动切换到国内镜像源：

| 运行时 | 镜像源 |
|--------|--------|
| node/python/bun/deno/pnpm/yarn/git | npmmirror.com |
| go | golang.google.cn |
| rust | rsproxy.cn |

也可以手动指定：

```sh
pvm install node@20 --mirror
```

---

## 开发

```sh
# 克隆项目
git clone https://github.com/your-org/pvm.git
cd pvm

# 编译
go build -o pvm.exe .      # Windows
go build -o pvm .           # macOS/Linux

# 运行测试
go test ./...
```

### 项目结构

```
pvm/
├── cmd/             # CLI 命令实现
├── internal/
│   ├── config/      # 配置管理
│   ├── installer/   # 安装逻辑
│   ├── registry/    # 版本信息与下载源
│   ├── shim/        # Shim 管理
│   └── plugins/     # 各运行时插件
├── scripts/         # 构建与安装脚本
└── dist/            # 构建产物
```

---

## Windows 安全提示

pvm 的 `.msi` 和 `.exe` 文件目前**未使用商业代码签名证书**，部分杀毒软件可能误报为可疑文件。

- 这是**误报**，请放心使用
- 安装时点击「更多信息」→「仍要运行」
- 或在 Windows Defender 中临时添加排除项
- 欢迎[提交文件给微软分析](https://www.microsoft.com/en-us/wdsi/filesubmission)

---

## 常见问题

<details>
<summary><b>安装后终端找不到 pvm 命令？</b></summary>

重启终端（CMD/PowerShell），或运行：
```powershell
refreshenv
```
如果用的是 VS Code / CodeBuddy，需要**完全退出并重启**编辑器（不是 reload window）。
</details>

<details>
<summary><b>pvm use 后 node 版本没变？</b></summary>

运行 `pvm doctor` 诊断。常见原因：
- 系统 PATH 中有旧版 node，需要执行 `pvm setup` 自动修复
- 终端/编辑器缓存了旧的 PATH，重启即可
</details>

<details>
<summary><b>安装被 Windows Defender 拦截？</b></summary>

见上方 [Windows 安全提示](#windows-安全提示)。
</details>

---

## License

[MIT](LICENSE) © PVM Contributors
