# PVM 安装指南

## 自动检测安装策略

> **核心原则：不让用户手动选择平台、架构或资产。**

```text
检查现有 PVM
  → 检测 Windows/macOS/Linux
  → 检测原生 amd64/arm64（Rosetta 下识别真实 Apple Silicon）
  → 用户指定版本则使用指定版本，否则解析最新稳定 Release
  → Gitee 下载失败自动回退 GitHub
  → 校验并执行对应平台安装器
  → 使用绝对路径执行 version 和 doctor
```

优先调用 Skill 自带的 `scripts/install-pvm.ps1` 或 `scripts/install-pvm.sh`。只有自动安装失败时才进入手动 Release；没有匹配 Release 时才考虑源码构建。

---

## Windows

### 方式 A：自动检测并安装

当前仓库可直接复制：

```powershell
& "D:\gitfile\pvm\.codebuddy\skills\pvm-env-manager\scripts\install-pvm.ps1"
```

通用 Skill 路径：

```powershell
# <SKILL_ROOT> 替换为当前已加载 Skill 的绝对目录
& "<SKILL_ROOT>\scripts\install-pvm.ps1"

# 指定版本或下载源
& "<SKILL_ROOT>\scripts\install-pvm.ps1" -Version v0.0.1
& "<SKILL_ROOT>\scripts\install-pvm.ps1" -Source github
```

不依赖 Skill 的远程一键安装：

```powershell
irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex
```

引导脚本会自动：
1. 检测 Windows x64 / ARM64，并确定对应资产名。
2. 优先调用当前仓库中的正式安装器；分发到其他项目时自动下载正式安装器。
3. 未指定版本时解析最新稳定 Release。
4. 从 Gitee 下载，失败后回退 GitHub。
5. 下载到临时文件，进行校验和可执行版本验证后再替换旧版本。
6. 安装到 `%USERPROFILE%\.pvm\bin\pvm.exe`，运行 `pvm setup` 并配置用户 PATH。

仅探测不安装：

```powershell
& "<SKILL_ROOT>\scripts\install-pvm.ps1" -DetectOnly
```

自动安装失败后再进入方式 B。

### 方式 B：Releases 手动下载

**国内用户（Gitee）：**

1. 浏览器打开 https://gitee.com/lycorxy/pvm/releases
2. 选择对应文件：

| 系统 | 文件 |
|------|------|
| Windows x64 | `pvm-windows-amd64.exe` |
| Windows ARM64 | `pvm-windows-arm64.exe` |

**国际用户（GitHub）：**

1. 浏览器打开 https://github.com/Lycorxy/pvm/releases
2. 选择对应文件：

| 系统 | 文件 |
|------|------|
| Windows x64 | `pvm-windows-amd64.exe` |
| Windows ARM64 | `pvm-windows-arm64.exe` |

**安装步骤：**

3. 创建目录并放置文件：
   ```powershell
   New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.pvm\bin"
   # 把下载的 exe 复制进去，重命名为 pvm.exe
   Copy-Item "下载的文件.exe" "$env:USERPROFILE\.pvm\bin\pvm.exe"
   ```

4. 初始化：
   ```powershell
   & "$env:USERPROFILE\.pvm\bin\pvm.exe" setup
   ```

### 方式 C：源码自建

需要与项目 `go.mod` 一致的 Go 1.25.5+ 编译环境。

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

### 方式 A：自动检测并安装

Skill 位于当前仓库时：

```bash
bash .codebuddy/skills/pvm-env-manager/scripts/install-pvm.sh
```

通用 Skill 路径：

```bash
# <SKILL_ROOT> 替换为当前已加载 Skill 的绝对目录
bash "<SKILL_ROOT>/scripts/install-pvm.sh"

# 指定版本或下载源
bash "<SKILL_ROOT>/scripts/install-pvm.sh" --version v0.0.1
bash "<SKILL_ROOT>/scripts/install-pvm.sh" --source github
```

不依赖 Skill 的远程一键安装：

```bash
curl -fsSL https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh | bash
```

引导脚本会自动：
1. 检测 macOS/Linux 和 amd64/arm64。
2. 在 Apple Silicon 的 Rosetta 终端中识别真实 ARM64，选择原生资产。
3. 未指定版本时解析最新稳定 Release。
4. 从 Gitee 下载，失败后回退 GitHub。
5. 下载到临时文件，验证校验值、可执行性和版本后安装。
6. 执行 `pvm setup`，创建目录并配置 zsh/bash/fish 的 PATH。

仅探测不安装：

```bash
bash "<SKILL_ROOT>/scripts/install-pvm.sh" --detect-only
```

自动安装失败后再进入方式 B。

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

需要与项目 `go.mod` 一致的 Go 1.25.5+。

```bash
git clone https://github.com/Lycorxy/pvm.git /tmp/pvm && cd /tmp/pvm
go build -o pvm .
mkdir -p ~/.pvm/bin && cp pvm ~/.pvm/bin/
chmod +x ~/.pvm/bin/pvm
~/.pvm/bin/pvm setup
```

### 指定版本安装

```bash
bash "<SKILL_ROOT>/scripts/install-pvm.sh" --version v0.0.1
```

如果脱离 Skill 直接使用远程正式安装器，参数必须传给 Bash，而不是只传给 `curl`：

```bash
curl -fsSL https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.sh | bash -s -- --version v0.0.1
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
| `PVM_INSTALL_VERSION` | 指定安装版本 | 最新稳定版 |
| `PVM_INSTALL_SOURCE` | `auto` / `gitee` / `github` | `auto` |
| `PVM_GITEE_REPO` | Gitee 仓库 | `lycorxy/pvm` |
| `PVM_GITHUB_REPO` | GitHub 仓库 | `Lycorxy/pvm` |
| `PVM_REQUIRE_CHECKSUM` | 设为 `1` 时缺少校验文件即失败 | `0` |
| `PVM_NO_MODIFY_PATH` | Windows 设为 `1` 则不修改 PATH | `0` |
| `PVM_NO_MODIFY_PROFILE` | macOS/Linux 设为 `1` 则不修改 shell profile | `0` |

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
