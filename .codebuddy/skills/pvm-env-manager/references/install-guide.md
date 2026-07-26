# PVM 安装指南

## 多级安装策略（按顺序尝试）

> **核心原则：从最简单到最复杂，成功即停。**

```
方式 A：一键脚本（推荐）     ← 最快，但依赖作者已推送脚本
  ↓ 失败（404 / 脚本不存在）
方式 B：Releases 下载         ← 需手动下载，最可靠
  ↓ 失败（无 Release）
方式 C：源码自建              ← 需要 Go 环境
```

---

## Windows

### 方式 A：PowerShell 一键安装

**国内用户（推荐，从 Gitee 下载）：**

```powershell
# 执行安装脚本
iwr -useb https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex
```

**国际用户（从 GitHub 下载）：**

```powershell
# 执行安装脚本
iwr -useb https://github.com/Lycorxy/pvm/raw/master/scripts/install.ps1 | iex
```

安装脚本会自动：
1. 检测系统架构（amd64 / arm64）
2. 下载最新版 PVM 到 `~/.pvm/bin/pvm.exe`
3. 运行 `pvm reshim` 生成 shim 硬链接
4. 将 `~/.pvm/shims` 和 `~/.pvm/bin` 前置到用户 PATH
5. 检测并移除 PATH 中的冲突路径（nodejs、nvm、python 等）
6. 检测系统 PATH 冲突（需管理员权限自动清理）

**如果返回 404 → 跳到方式 B**

### 方式 B：Releases 手动下载

**国内用户（Gitee）：**

1. 浏览器打开 https://gitee.com/lycorxy/pvm/releases
2. 选择对应文件：

| 系统 | 文件 |
|------|------|
| Windows x64 | `pvm-windows-amd64.exe` 或 `.msi` |
| Windows ARM | `pvm-windows-arm64.exe` 或 `.msi` |

**国际用户（GitHub）：**

1. 浏览器打开 https://github.com/Lycorxy/pvm/releases
2. 选择对应文件：

| 系统 | 文件 |
|------|------|
| Windows x64 | `pvm-windows-amd64.exe` 或 `.msi` |
| Windows ARM | `pvm-windows-arm64.exe` 或 `.msi` |

**安装步骤：**

3. 创建目录并放置文件：
   ```powershell
   New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.pvm\bin"
   # 把下载的 exe 复制进去，重命名为 pvm.exe
   Copy-Item "下载的文件.exe" "$env:USERPROFILE\.pvm\bin\pvm.exe"
   ```

4. 初始化：
   ```powershell
   & "$env:USERPROFILE\.pvm\bin\pvm.exe" setup"
   ```

### 方式 C：源码自建

需要 Go 1.22+ 编译环境。

```powershell
git clone https://github.com/Lycorxy/pvm.git $env:TEMP\pvm
cd $env:TEMP\pvm
go build -o pvm.exe .
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.pvm\bin"
Copy-Item pvm.exe "$env:USERPROFILE\.pvm\bin\pvm.exe"
& "$env:USERPROFILE\.pvm\bin\pvm.exe" setup
```

### 安装后验证

```powershell
# 重启终端后
pvm --version    # 应输出版本号
pvm doctor       # 应全部通过 ✓
```

### 特殊情况

#### pvm.exe 被进程占用无法覆盖

```powershell
# 方案：复制新文件 → 删除旧文件 → 重命名
Get-Process -Name "pvm" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500
Copy-Item "新pvm.exe" "$env:USERPROFILE\.pvm\bin\pvm-new.exe" -Force
Remove-Item "$env:USERPROFILE\.pvm\bin\pvm.exe" -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300
Rename-Item "$env:USERPROFILE\.pvm\bin\pvm-new.exe" "pvm.exe" -Force
```

#### VS Code / 编辑器终端里 pvm 不生效

编辑器启动时缓存环境变量。必须**完全重启编辑器**：
- VS Code：`File` → `Exit`（不是 reload window）→ 重新打开
- 重开后在新终端验证 `pvm -v`

#### 杀毒软件误报

PVM 的 shim 是 pvm.exe 的硬链接，杀软可能误报。

处理方式：
- 点击「更多信息」→「仍要运行」
- 或在 Windows Defender 中添加排除项：
  ```
  设置 → Windows 安全中心 → 病毒和威胁防护 → 管理设置 → 排除项
  → 添加文件夹 → %USERPROFILE%\.pvm
  ```
- 或提交文件给微软分析：https://www.microsoft.com/en-us/wdsi/filesubmission

---

## macOS / Linux

### 方式 A：一键安装

**国内用户（推荐，从 Gitee 下载）：**

```bash
curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
```

**国际用户（从 GitHub 下载）：**

```bash
curl -fsSL https://github.com/Lycorxy/pvm/raw/master/scripts/install.sh | bash
```

安装脚本自动完成：
1. 检测 OS（darwin / linux）和架构（amd64 / arm64）
2. 下载最新版 PVM 到 `~/.pvm/bin/pvm`
3. 运行 `pvm reshim` 生成 shim 硬链接
4. 在 shell rc 文件中添加 PATH 配置

**如果 404 → 跳到方式 B**

### 方式 B：Releases 手动下载

**国内用户（Gitee）：**

1. 打开 https://gitee.com/lycorxy/pvm/releases
2. 选择文件：

| 系统 | 文件 |
|------|------|
| macOS Intel | `pvm-darwin-amd64` |
| macOS Apple Silicon | `pvm-darwin-arm64` |
| Linux x64 | `pvm-linux-amd64` |
| Linux ARM | `pvm-linux-arm64` |

**国际用户（GitHub）：**

1. 打开 https://github.com/Lycorxy/pvm/releases
2. 选择文件：

| 系统 | 文件 |
|------|------|
| macOS Intel | `pvm-darwin-amd64` |
| macOS Apple Silicon | `pvm-darwin-arm64` |
| Linux x64 | `pvm-linux-amd64` |
| Linux ARM | `pvm-linux-arm64` |

**安装步骤：**

3. 安装：
   ```bash
   mkdir -p ~/.pvm/bin
   cp 下载的文件 ~/.pvm/bin/pvm
   chmod +x ~/.pvm/bin/pvm
   ~/.pvm/bin/pvm setup
   ```

### 方式 C：源码自建

需要 Go 1.22+。

```bash
git clone https://github.com/Lycorxy/pvm.git /tmp/pvm && cd /tmp/pvm
go build -o pvm .
mkdir -p ~/.pvm/bin && cp pvm ~/.pvm/bin/
chmod +x ~/.pvm/bin/pvm
~/.pvm/bin/pvm setup
```

### 指定版本安装（仅方式 A 支持）

```bash
PVM_INSTALL_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/main/scripts/install.sh | bash
```

### 安装后验证

```bash
# 重启 shell 或 source rc 文件
source ~/.bashrc   # 或 source ~/.zshrc

pvm --version
pvm doctor
```

---

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PVM_HOME` | PVM 安装根目录 | `~/.pvm` |
| `PVM_REPO` | GitHub 仓库（owner/name） | `Lycorxy/pvm` |
| `PVM_INSTALL_VERSION` | 指定安装版本 | 最新版 |
| `PVM_NO_MODIFY_PATH` | 设为 `1` 则不修改 PATH | — |
| `PVM_NO_MODIFY_PROFILE` | 设为 `1` 则不修改 shell rc | — |

---

## 安装目录结构

```
~/.pvm/
├── bin/           # pvm 主程序
│   └── pvm.exe (或 pvm)
├── shims/         # shim 硬链接（已加入 PATH）
│   ├── node       → 硬链接到 pvm 主程序
│   ├── npm        → 硬链接到 pvm 主程序
│   ├── python     → 硬链接到 pvm 主程序
│   └── ...
├── installs/      # 运行时安装目录
│   ├── node/
│   │   ├── 20.11.0/
│   │   └── 22.0.0/
│   ├── python/
│   │   └── 3.12.3/
│   └── ...
├── cache/         # 下载缓存
└── versions       # 用户级版本配置文件
```
