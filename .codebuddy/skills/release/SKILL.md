# pvm-release — 发版助手（提交推送 → 自动 tag → 自动 Release → 双平台同步）

## 快捷命令

仓库根的 `release.cmd`（Windows）/ `release`（Unix）包装了 harness，任意目录可执行：

```bash
release init            # 一次性：粘贴 GitHub/Gitee 令牌 → 写 ~/.release.yaml
release check-tag       # 发版前置检查
release push            # 双远端推送
release actions-status  # Actions 构建状态
release sync-gitee      # 同步资产到 Gitee Release
release verify          # 双平台资产验证（发布闭环）
```

等价于 `node .codebuddy/skills/release/scripts/release-harness.cjs <cmd>`。

## 概述

驱动 pvm 的完整发版流程：**版本号由用户通过 `VERSION` 文件定义**，AI 按本工作流执行：
提交推送 master → 自动打 tag → GitHub Actions 自动构建并发布 GitHub Release（含 6 平台产物、
zip、skill 包、checksums.txt）→ 同步 Gitee Release → 双平台验证。

## 🛑 入口守卫

1. 在 pvm 仓库（`d:\gitfile\pvm`）内才可执行本流程；其他仓库直接拒绝。
2. 版本号**只能由用户定义**：一切以仓库根目录 `VERSION` 文件内容为准（如 `0.0.2`）。
   用户要求改版本 → 只改 `VERSION` 文件，禁止修改 `cmd/root.go` 的硬编码（那是开发兜底值）。
3. tag 已存在时**默认拒绝强推**，必须向用户确认后才能 `git tag -f`。

## 核心工作流

### Step 1：确认版本与待发布内容

1. 读 `VERSION` 文件 → `VERSION=x.x.x`
2. `git log v<上一个tag>..HEAD --oneline`（无旧 tag 时取最近 20 条）生成发布说明草稿
3. 向用户确认：版本号、发布说明（用户可修改）——确认后才继续

### Step 0：环境初始化检查（一次性）

1. 检查 `%USERPROFILE%\.release.yaml` 是否存在
2. 不存在 → 引导用户运行（交互粘贴 GitHub/Gitee 令牌）：
   `release init`
   自动写入 `~/.release.yaml` + 配置 `git remote gitee` + 自动配置 GitHub Actions Secret
   （GITEE_TOKEN，供 workflow 同步 Gitee）
3. 已存在 → 继续后续步骤（所有脚本自动读取，无需再粘贴令牌）

### Step 2：提交推送 master（双远端）

1. `git status --short` 有未提交改动 → 按变更内容分组提交（禁止把无关文件混入）
2. **走 harness**（自动推 origin + gitee 双远端）：
   ```bash
   release push
   ```
3. 失败 → 原样透传错误，停止流程

### Step 3：发版前置检查 + 创建并推送 tag

1. **前置检查（必须走 harness）**：
   ```bash
   release check-tag --version $VERSION
   ```
   检查：工作区、版本格式、本地/远端 tag 冲突、master 同步。有任何 ❌ → 先解决再继续
   （远端 tag 已存在 = 需用户确认强推）
2. `git tag v$VERSION` → `git push origin master` → `git push origin v$VERSION`
3. 推送 tag 自动触发 `.github/workflows/release.yml`：
   6 平台构建（ldflags 注入版本）→ GitHub Release 发布 → Gitee 同步（见 Step 4）

### Step 4：监控构建

1. **走 harness**：
   ```bash
   release actions-status
   ```
2. 未登录 gh → 提示用户查看 `https://github.com/Lycorxy/pvm/actions`，等待构建完成（约 1-2 分钟）
3. 构建失败 → 透传 Actions 日志链接与错误，停止流程

### Step 5：Gitee 同步（自动）

- 方式 A（已配 GitHub Secret，init 自动配置）：Actions 在 Step 3 已自动同步 Gitee，
  本步骤用 Step 6 的 verify 确认即可
- 方式 B（本地同步，令牌自动从 `~/.release.yaml` 读取）：
  ```bash
  release sync-gitee --notes '<Step 1 的发布说明>'
  ```
- **安全规则**：令牌保存在 `~/.release.yaml`（用户主目录，不进仓库）；
  播报中必须脱敏（只显示前 4 位）；禁止把令牌写入任何提交内容。

### Step 6：双平台验证（闭环，最多 2 轮）

1. **走 harness（自动下载比对 version + SHA-256 + 附加资产存在性）**：
   ```bash
   release verify --version $VERSION
   ```
2. 全部 ✅ → Step 7；有 ❌ → 回到 Step 4/5 排查（常见：GitHub CDN 缓存延迟 5 分钟内可接受；
   Gitee 失败重跑 sync-gitee.ps1）
3. 两轮仍不一致 → 输出差异明细，请用户网页检查 Release 资产列表

### Step 7：收尾播报

```
✅ v$VERSION 发布完成
   GitHub: https://github.com/Lycorxy/pvm/releases/tag/v$VERSION
   Gitee:  https://gitee.com/lycorxy/pvm/releases/tag/v$VERSION
   文档站: GitHub Pages（Actions Deploy Docs job 的 URL）/ Gitee Pages（gh-pages 分支）
   资产: 14 个（6 二进制 + 6 zip + skill 包 + checksums.txt）
   双平台验证: ✅ 一致
```

## 输出格式（精简播报）

```
[Step n] <执行了什么>
→ ✅/❌ <结果或原因>
→ 下一步: <命令或说明>（失败时必给）
```

## 错误处理

| 现象 | 处理 |
|------|------|
| tag 已存在 | 默认拒绝强推；用户确认后才 `git tag -f`（会重新触发全量发布） |
| Actions 构建失败 | 透传失败 job 链接；常见原因：测试失败、网络；修复后重打 tag（用户确认强推） |
| GitHub CDN 缓存返回旧资产 | 下载 URL 加 `?nocache=<时间戳>` 重试；5 分钟内属正常延迟 |
| Gitee 上传 404/401 | 令牌无效或权限不足 → 提示用户重新生成令牌（勾选 projects） |
| 版本一致性校验失败（安装端报 tag≠binary） | 二进制版本未注入 → 检查 workflow 的 ldflags 是否生效 |
