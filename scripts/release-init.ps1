# pvm-release 初始化 — 一次性配置令牌与双平台远端
#
# 用法：.\scripts\release-init.ps1
#
# 产出：
#   1. %USERPROFILE%\.release.yaml   令牌与仓库配置（用户主目录，不进任何仓库，勿提交）
#   2. GitHub Actions Secret         GITEE_TOKEN（gh 可用时自动配置，供 workflow 同步 Gitee）
#   3. git remote gitee              双远端推送（release-harness.ps1 push 使用）
#
# 之后所有发版脚本自动读取 ~/.release.yaml，无需再粘贴令牌。

param(
    [string]$ConfigPath = (Join-Path $env:USERPROFILE ".release.yaml"),
    [switch]$NonInteractive    # 令牌从 GITHUB_TOKEN/GITEE_TOKEN 环境变量读取（CI 场景）
)

$ErrorActionPreference = "Stop"
# 本脚本位于 <root>\scripts → 上一级 = 仓库根
$ProjectRoot = Split-Path -Parent $PSScriptRoot

Write-Host "=== pvm 发版环境初始化 ===" -ForegroundColor Cyan
Write-Host "配置将保存到: $ConfigPath（用户主目录，不进任何仓库）"
Write-Host ""

$github = @{ token = ""; repo = "Lycorxy/pvm" }
$gitee  = @{ token = ""; repo = "lycorxy/pvm"; remote = "https://gitee.com/lycorxy/pvm.git" }

# 已有配置则载入（重跑可只改某一项，回车保留原值）
if (Test-Path $ConfigPath) {
    Write-Host "检测到已有配置，直接回车可保留原值：" -ForegroundColor Yellow
    $sec = $null
    foreach ($line in (Get-Content $ConfigPath)) {
        if ($line -match '^github:') { $sec = 'gh' }
        elseif ($line -match '^gitee:') { $sec = 'gt' }
        elseif ($line -match '^\s+token:\s*(.*)$') {
            if ($sec -eq 'gh') { $github.token = $Matches[1].Trim() }
            elseif ($sec -eq 'gt') { $gitee.token = $Matches[1].Trim() }
        }
        elseif ($line -match '^\s+remote:\s*(.*)$') {
            if ($sec -eq 'gt') { $gitee.remote = $Matches[1].Trim() }
        }
    }
}

if ($NonInteractive) {
    if ($env:GITHUB_TOKEN) { $github.token = $env:GITHUB_TOKEN }
    if ($env:GITEE_TOKEN)  { $gitee.token  = $env:GITEE_TOKEN }
} else {
    $in = Read-Host "GitHub Personal Access Token（repo + workflow 权限；回车保留原值）"
    if ($in) { $github.token = $in }
    $in = Read-Host "Gitee 私人令牌（projects 权限；回车保留原值）"
    if ($in) { $gitee.token = $in }
    $in = Read-Host "Gitee 仓库地址（回车保留默认）"
    if ($in) { $gitee.remote = $in }
}

if (-not $github.token -and -not $gitee.token) {
    Write-Host "error: 两个令牌均为空，未做任何修改" -ForegroundColor Red
    exit 1
}

# ---- 1. 写入 ~/.release.yaml ----
$yaml = @"
# pvm 发版配置（由 release-init.ps1 生成）
# 含令牌：请勿提交到任何仓库、请勿分享
github:
  token: "$($github.token)"
  repo: "$($github.repo)"
gitee:
  token: "$($gitee.token)"
  repo: "$($gitee.repo)"
  remote: "$($gitee.remote)"
"@
[IO.File]::WriteAllText($ConfigPath, $yaml, [Text.UTF8Encoding]::new($true))
Write-Host "OK 配置已写入 $ConfigPath"

# ---- 2. git remote gitee（双远端推送用）----
Push-Location $ProjectRoot
try {
    $remotes = (git remote | Out-String)
    if ($remotes -match '(^|\s)gitee(\s|$)') {
        git remote set-url gitee $gitee.remote
        Write-Host "OK git remote gitee 已更新: $($gitee.remote)"
    } else {
        git remote add gitee $gitee.remote
        Write-Host "OK git remote gitee 已添加: $($gitee.remote)"
    }
} finally { Pop-Location }

# ---- 3. GitHub Actions Secret GITEE_TOKEN（workflow 同步 Gitee 用）----
if ($github.token -and $gitee.token) {
    $env:GH_TOKEN = $github.token
    $secretOut = & gh secret set GITEE_TOKEN --repo $github.repo --body $gitee.token 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "OK GitHub Actions Secret 已配置: GITEE_TOKEN（push tag 时 workflow 自动同步 Gitee）"
    } else {
        Write-Host "WARN GitHub Secret 配置失败: $secretOut" -ForegroundColor Yellow
        Write-Host "   手动配置: gh auth login 后重跑本脚本；或网页 Settings -> Secrets -> Actions 添加 GITEE_TOKEN"
    }
}

Write-Host ""
Write-Host "=== 初始化完成 ===" -ForegroundColor Cyan
Write-Host "之后所有发版脚本自动读取 ~/.release.yaml："
Write-Host "  发版   : .codebuddy\skills\release\scripts\release-harness.ps1 check-tag -> push -> verify"
Write-Host "  同步码云: .\scripts\sync-gitee.ps1"
Write-Host "  静态页面: push master 自动触发 .github/workflows/pages.yml"
