# 路线图

## 已完成 ✅

- [x] 核心版本管理（node, python, go, rust, bun, deno）
- [x] 包管理器支持（pnpm, yarn）
- [x] 工具支持（git）
- [x] Shim 机制 - 无感版本切换
- [x] .pvmrc 项目级版本配置
- [x] Windows MSI 安装包
- [x] macOS/Linux Shell 安装脚本
- [x] 国内镜像源自动切换
- [x] 系统 PATH 冲突自动修复（UAC 提权）
- [x] pvm doctor 诊断命令
- [x] pvm self-update 自更新
- [x] pvm setup 一键初始化

## 进行中 🚧

- [ ] 完善开源社区文档
- [ ] GitHub Actions CI/CD
- [ ] 代码签名证书申请
- [ ] 更多运行时插件

## 计划 📋

### 近期（1-2 个月）

- [ ] Windows 安装包代码签名（减少杀软误报）
- [ ] Winget / Chocolatey / Scoop 包管理器发布
- [ ] 安装进度优化（大文件断点续传）
- [ ] 离线安装模式
- [ ] `pvm exec` - 指定版本执行单条命令

### 中期（3-6 个月）

- [ ] 更多运行时支持：
  - Java（通过 SDKMAN 集成）
  - Dotnet SDK
  - Zig
- [ ] 版本别名（`pvm alias default node@20`）
- [ ] 全局包迁移（切换版本时自动迁移 npm/pip 包）
- [ ] shell 自动补全（PowerShell、bash、zsh、fish）
- [ ] pvm update（自动更新已安装的运行时到最新版）

### 长期

- [ ] pvm daemon - 后台服务加速版本切换
- [ ] 多用户共享安装目录
- [ ] Web Dashboard（可视化版本管理界面）
- [ ] IDE 插件（VS Code / JetBrains）

## 想要新功能？

在 [Issues](https://github.com/your-org/pvm/issues) 中提出你的想法！我们会根据社区反馈调整优先级。
