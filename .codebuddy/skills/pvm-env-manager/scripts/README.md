# 实用工具脚本

所有脚本位于 `.codebuddy/skills/pvm-env-manager/scripts/` 目录，支持 Windows (.bat) 和 macOS/Linux (.sh)。

---

## 1. local 文件合并冲突处理

**脚本**：`resolve-local-conflict.bat` / `resolve-local-conflict.sh`

**用法**：
```bash
# Windows
scripts\resolve-local-conflict.bat [--local|--remote|--auto]

# macOS/Linux
./scripts/resolve-local-conflict.sh [--local|--remote|--auto]
```

**参数**：
- `--local`：保留本地版本
- `--remote`：使用远程版本（默认）
- `--auto`：自动选择远程版本

**功能**：
- 扫描 git 合并冲突文件
- 只处理文件名包含 "local" 的文件（不区分大小写）
- 根据参数选择保留本地或远程版本
- 自动 `git add` 解决冲突

---

## 2. 从远程替换本地 local 文件

**脚本**：`replace-local-from-remote.bat` / `replace-local-from-remote.sh`

**用法**：
```bash
# Windows
scripts\replace-local-from-remote.bat [--backup|--no-backup|--list]

# macOS/Linux
./scripts/replace-local-from-remote.sh [--backup|--no-backup|--list]
```

**参数**：
- `--backup`：替换前备份原有文件（默认）
- `--no-backup`：不备份直接替换
- `--list`：仅列出 local 文件，不替换

**功能**：
- 扫描仓库中所有包含 "local" 的文件
- 从远程仓库获取最新版本
- 可选备份原有文件（带时间戳）
- 支持仅查看模式（不实际替换）

---

## 3. 软件彻底卸载

**脚本**：`uninstall-tool.bat` / `uninstall-tool.sh`

**用法**：
```bash
# Windows
scripts\uninstall-tool.bat <软件名> [--yes]

# macOS/Linux
./scripts/uninstall-tool.sh <软件名> [--yes]
```

**支持的软件**：
| 软件 | 说明 |
|------|------|
| `node` | Node.js（终止进程、清理 PATH、删除目录、清理注册表） |
| `git` | Git（卸载、清理配置） |
| `nvm` | Node Version Manager（清理环境变量、删除目录） |
| `pvm` | Polyglot Version Manager（自卸载） |
| `python` | Python（卸载、清理配置） |
| `yarn` | Yarn（卸载、删除全局目录） |
| `pnpm` | PNPM（卸载、删除全局目录） |

**参数**：
- `--yes`：跳过确认直接卸载

**功能**：
- 终止相关进程
- 查找并删除安装目录
- 清理环境变量（用户级 + 系统级）
- 清理注册表（Windows）
- 清理 shell 配置（macOS/Linux）
- 生成卸载报告

---

## 4. PVM 环境诊断

**脚本**：`diagnose-pvm-env.bat` / `diagnose-pvm-env.sh`

**用法**：
```bash
# Windows
scripts\diagnose-pvm-env.bat [--fix]

# macOS/Linux
./scripts/diagnose-pvm-env.sh [--fix]
```

**参数**：
- `--fix`：自动修复发现的问题

**诊断项目**：
1. PVM 安装状态
2. 环境变量配置（PVM_HOME、PATH）
3. 冲突工具检测（nvm、fnm、pyenv 等）
4. PVM 目录结构
5. 运行时安装状态
6. 端口冲突检测（3000、8080 等）
7. .npmrc 配置检查
8. Shell 配置文件检查

---

## 5. .npmrc 自动配置

**脚本**：`setup-npmrc.bat` / `setup-npmrc.sh`

**用法**：
```bash
# Windows
scripts\setup-npmrc.bat [--force]

# macOS/Linux
./scripts/setup-npmrc.sh [--force]
```

**参数**：
- `--force`：强制覆盖现有配置

**配置内容**：
```ini
# pnpm 核心配置
registry=https://registry.npmmirror.com/
save-prefix=""
auto-install-peers=true

# lockfile 冻结策略
strict-peer-dependencies=false
prefer-frozen-lockfile=true

# 版本与冲突处理
resolution-mode=time-based
prefer-higher-version=true

# 依赖结构
shamefully-hoist=false
strict-store-content=true
```

**功能**：
- 检查 .npmrc 是否已存在
- 备份现有配置（如果存在）
- 创建新的 .npmrc 文件
- 配置国内镜像、版本锁定、依赖策略等

---

## 6. nvm 卸载脚本（Node.js）

**脚本**：`uninstall_nvm.js`

**用法**：
```bash
node scripts/uninstall_nvm.js [--yes]
```

**参数**：
- `--yes`：跳过确认直接卸载

**前提**：PVM 已安装 node

**功能**：
- 终止 nvm 相关进程
- 检测 nvm 安装路径
- 从用户 PATH 和系统 PATH 移除 nvm 相关条目
- 清除 NVM_HOME、NVM_SYMLINK 环境变量
- 删除 nvm 安装目录
- 清理注册表残留（Windows）
- 输出清理报告

---

## 脚本执行规则

### local 文件冲突处理

- **前提**：当前目录必须是 Git 仓库
- **检测**：先运行 `git diff --name-only --diff-filter=U` 检查是否有冲突
- **策略**：默认使用远程版本（`--remote`），除非用户明确要求保留本地
- **验证**：处理后运行 `git status` 确认冲突已解决

### 从远程替换文件

- **前提**：当前目录必须是 Git 仓库
- **备份**：默认备份（`--backup`），除非用户指定 `--no-backup`
- **同步**：替换前先运行 `git fetch origin` 确保远程信息最新
- **验证**：替换后运行 `git status` 确认文件已更新

### 软件卸载

- **前提**：确认目标软件已安装
- **替代品检查**：
  - 卸载 node 前：确认 PVM 已安装其他运行时或用户明确确认
  - 卸载 nvm 前：确认 PVM 已安装 node
  - 卸载 pvm 前：用户明确确认
- **进程终止**：卸载前必须先终止相关进程
- **环境变量清理**：必须清理用户级和系统级 PATH
- **注册表清理**：Windows 上必须清理注册表残留

### 环境诊断

- **完整性**：必须检查所有 8 个诊断项
- **自动修复**：使用 `--fix` 参数时自动修复发现的问题
- **报告**：诊断完成后生成详细报告，包括通过/失败/警告数量

### .npmrc 配置

- **备份**：覆盖前备份现有配置
- **内容**：必须包含国内镜像、版本锁定、peer 依赖、lockfile 策略
- **验证**：配置完成后验证文件格式正确