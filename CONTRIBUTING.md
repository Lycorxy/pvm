# 贡献指南

感谢你对 pvm 的关注！无论是报告 Bug、提出新功能、还是提交代码，所有形式的贡献都欢迎。

## 行为准则

本项目遵循 [贡献者公约](CODE_OF_CONDUCT.md)。参与即表示同意遵守其条款。

## 如何贡献

### 报告 Bug

1. 先在 [Issues](https://github.com/your-org/pvm/issues) 中搜索，确认是否已有相同问题
2. 如果没有，[创建新 Issue](https://github.com/your-org/pvm/issues/new)
3. 请包含以下信息：
   - 操作系统和版本（`winver` / `uname -a`）
   - pvm 版本（`pvm -v`）
   - 复现步骤
   - 期望行为 vs 实际行为
   - 如果方便，附上 `pvm doctor` 的输出

### 功能请求

1. 先在 Issues 中搜索，看看是否有人提出过
2. 创建新 Issue，描述你的使用场景和期望的功能

### 提交代码

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 编写代码
4. 确保通过测试：`go test ./...`
5. 提交：`git commit -m "feat: add my feature"`
6. Push：`git push origin feature/my-feature`
7. 创建 Pull Request

### Commit 规范

使用 [约定式提交](https://www.conventionalcommits.org/zh-hans/)：

```
feat: 新功能
fix: 修复 Bug
docs: 文档变更
refactor: 重构（不改变功能）
test: 测试相关
chore: 构建/工具链相关
```

示例：
```
feat: add bun mirror support for remote version list
fix: auto-fix system PATH conflicts on Windows
docs: add Windows security notice in README
```

## 开发环境

```sh
# 克隆
git clone https://github.com/your-org/pvm.git
cd pvm

# 编译
go build -o pvm .

# 测试
go test ./...

# 完整测试（含安装测试，较慢）
go test ./... -v
```

### Windows 额外依赖

- WiX Toolset 3（用于打包 MSI，非必需）
  - 下载 [wix314-binaries.zip](https://github.com/wixtoolset/wix3/releases/download/wix314rtm/wix314-binaries.zip)
  - 解压到 `scripts/wix/`

## 项目结构

```
cmd/             # CLI 命令入口
   root.go       # 主命令注册与路由
   setup.go      # pvm setup 实现
   use.go        # pvm use 实现
   install.go    # pvm install 实现
   doctor.go     # pvm doctor 诊断实现
   shim/main.go  # pvm-shim 转发器
internal/
   config/       # 配置管理（PATH、目录结构、.pvmrc）
   installer/    # 运行时安装核心逻辑
   registry/     # 版本信息获取与下载 URL
   plugins/      # 各运行时插件实现
   shim/         # Shim 创建和管理
   semver/       # 语义化版本处理
   download/     # HTTP 下载器
   logger/       # 日志输出
scripts/         # 构建、安装、打包脚本
```

## 添加新的运行时

1. 在 `internal/registry/` 下创建 `xxx.go`
2. 实现 `getXxxInfo`、`listRemoteXxx` 函数
3. 在 `internal/registry/registry.go` 的 `ListRemoteVersions` 添加 case
4. 在 `internal/plugins/xxx/` 下创建插件
5. 在 `internal/config/paths.go` 的 `SupportedRuntimes` 添加名称
6. 运行 `pvm doctor` 验证

## 许可证

贡献的代码将采用与项目相同的 [MIT 许可证](LICENSE)。
