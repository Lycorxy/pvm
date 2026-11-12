# pvm-release harness — 发版流程检测与验证（AI 只跑本脚本，不手拼命令）
#
# 用法：
#   release-harness.ps1 check-tag  -Version 0.0.3    # 发版前置检查：tag 冲突 + 工作区状态
#   release-harness.ps1 actions-status              # 最新 workflow run 状态
#   release-harness.ps1 verify     -Version 0.0.2   # 双平台资产验证（发布闭环）
#
# 退出码：0 = 全部通过；1 = 有失败项；2 = 用法错误

param(
    [Parameter(Position = 0)][string]$Action = "",
    [string]$Version = "",
    [string]$Repo = "Lycorxy/pvm",
    [string]$GiteeRepo = "lycorxy/pvm",
    [string]$Platform = "windows-amd64"
)

$ErrorActionPreference = "Continue"
# PSScriptRoot = <root>\.codebuddy\skills\release\scripts → 上四级 = 仓库根
$ProjectRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
$script:pass = 0
$script:fail = 0

function Report {
    param([string]$Name, [bool]$Ok, [string]$Detail = "")
    if ($Ok) { $script:pass++; Write-Host "  $Name → ✅ $Detail" }
    else     { $script:fail++; Write-Host "  $Name → ❌ $Detail" -ForegroundColor Yellow }
}

function Read-VersionFromFile {
    $vf = Join-Path $ProjectRoot "VERSION"
    if (-not (Test-Path $vf)) { return "" }
    return ((Get-Content $vf -Raw).Trim())
}

# 读取 ~/.release.yaml（release-init.ps1 生成）：github.token / gitee.token / gitee.remote
function Read-ReleaseConfig {
    $cfgPath = Join-Path $env:USERPROFILE ".release.yaml"
    $cfg = @{ ghToken = ""; giteeToken = ""; giteeRemote = "" }
    if (-not (Test-Path $cfgPath)) { return $cfg }
    $sec = $null
    foreach ($line in (Get-Content $cfgPath)) {
        if ($line -match '^github:') { $sec = 'gh' }
        elseif ($line -match '^gitee:') { $sec = 'gt' }
        elseif ($line -match '^\s+token:\s*(.*)$') {
            if ($sec -eq 'gh') { $cfg.ghToken = $Matches[1].Trim().Trim('"') }
            elseif ($sec -eq 'gt') { $cfg.giteeToken = $Matches[1].Trim().Trim('"') }
        }
        elseif ($line -match '^\s+remote:\s*(.*)$') {
            if ($sec -eq 'gt') { $cfg.giteeRemote = $Matches[1].Trim().Trim('"') }
        }
    }
    return $cfg
}

# ---------- check-tag：发版前置检查 ----------
function Invoke-CheckTag {
    if (-not $Version) { $Version = Read-VersionFromFile }
    if (-not $Version) {
        Write-Host "error: 未提供 -Version 且未找到 VERSION 文件" -ForegroundColor Red
        exit 2
    }
    Write-Host "[pvm-release check-tag] 目标版本: v$Version"
    Write-Host ""

    Push-Location $ProjectRoot
    try {
        $dirty = git status --short
        $dirtyText = ($dirty | Out-String).Trim()
        Report "工作区干净" ([string]::IsNullOrWhiteSpace($dirtyText)) `
            $(if ($dirtyText) { "有未提交改动：" + (($dirty | Select-Object -First 5) -join '; ') } else { "" })

        Report "版本号格式正确" ($Version -match '^\d+\.\d+\.\d+$') "VERSION=$Version"

        $localTag = git tag -l "v$Version"
        Report "本地 tag v$Version 不存在" ([string]::IsNullOrEmpty(($localTag | Out-String).Trim())) `
            $(if ($localTag) { "已存在，强推需用户确认（git tag -f）" } else { "" })

        $remote = (git ls-remote --tags origin "refs/tags/v$Version" 2>$null | Out-String).Trim()
        Report "远端 tag v$Version 不存在" ([string]::IsNullOrEmpty($remote)) `
            $(if ($remote) { "远端已存在，强推需用户确认（会重新触发全量发布）" } else { "" })

        git fetch origin master --quiet 2>$null
        $ahead = git rev-list --count "origin/master..master" 2>$null
        $behind = git rev-list --count "master..origin/master" 2>$null
        Report "master 与 origin/master 同步" (($ahead -eq "0") -and ($behind -eq "0")) `
            "ahead=$ahead behind=$behind"
    } finally {
        Pop-Location
    }

    Write-Host ""
    Write-Host "结果: $script:pass 通过, $script:fail 失败"
    exit $(if ($script:fail -eq 0) { 0 } else { 1 })
}

# ---------- actions-status：最新 workflow run 状态 ----------
function Invoke-ActionsStatus {
    Write-Host "[pvm-release actions-status]"
    # ~/.release.yaml 有 GitHub token 时直接注入 gh（无需 gh auth login）
    $cfg = Read-ReleaseConfig
    if ($cfg.ghToken -and -not $env:GH_TOKEN) { $env:GH_TOKEN = $cfg.ghToken }

    $ghOk = $false
    try {
        $null = & gh auth status 2>&1
        $ghOk = ($LASTEXITCODE -eq 0)
    } catch { $ghOk = $false }

    if ($ghOk) {
        & gh run list --repo $Repo --limit 3
        exit $(if ($LASTEXITCODE -eq 0) { 0 } else { 1 })
    }
    Write-Host "  gh 不可用（未登录且 ~/.release.yaml 无 github.token）→ 请网页查看："
    Write-Host "  https://github.com/$Repo/actions"
    Write-Host "  或运行 .\scripts\release-init.ps1 完成初始化"
    exit 2
}

# ---------- push：双远端推送（GitHub origin + Gitee gitee） ----------
function Invoke-Push {
    Push-Location $ProjectRoot
    try {
        $cfg = Read-ReleaseConfig
        $remotes = (git remote | Out-String)
        $hasGitee = $remotes -match '(^|\s)gitee(\s|$)'

        Write-Host "  → push origin master"
        git push origin master
        if ($LASTEXITCODE -ne 0) { exit 1 }

        if ($hasGitee) {
            Write-Host "  → push gitee master"
            git push gitee master
            if ($LASTEXITCODE -ne 0) {
                Write-Host "⚠️ gitee 推送失败（凭证问题可跑 .\scripts\release-init.ps1 重新配置）"
                exit 1
            }
        } else {
            Write-Host "  ℹ 无 gitee 远端：运行 .\scripts\release-init.ps1 配置后自动双推"
        }
        Write-Host "✓ 推送完成"
        exit 0
    } finally { Pop-Location }
}

# ---------- verify：双平台资产验证 ----------
function Invoke-VerifyImpl {
    if (-not $Version) { $Version = Read-VersionFromFile }
    if (-not $Version) {
        Write-Host "error: 未提供 -Version 且未找到 VERSION 文件" -ForegroundColor Red
        exit 2
    }
    Write-Host "[pvm-release verify] v$Version @ $Platform"
    Write-Host ""

    $ext = if ($Platform -like "windows*") { ".exe" } else { "" }
    $assetName = "pvm-$Platform$ext"
    $localExe = Join-Path $ProjectRoot "dist\$assetName"
    $localSha = $null
    $isWindowsPlat = ($Platform -like "windows*")

    # 0. 本地基准
    if (Test-Path $localExe) {
        $localSha = (Get-FileHash $localExe -Algorithm SHA256).Hash.ToLowerInvariant()
        Report "本地 dist 基准 ($assetName)" ($localSha.Length -eq 64) `
            "sha=$($localSha.Substring(0, 12))..."
    } else {
        Write-Host "  ℹ 本地无 dist\$assetName，跳过 SHA 对比，仅验证远端 version"
    }

    $tmp = Join-Path $env:TEMP "pvm-verify-$Platform-$([guid]::NewGuid().ToString('N'))$ext"
    $extra = @("checksums.txt", "pvm-env-manager.zip")

    foreach ($src in @(
        @{ Name = "GitHub"; Base = "https://github.com/$Repo/releases/download/v$Version" },
        @{ Name = "Gitee";  Base = "https://gitee.com/$GiteeRepo/releases/download/v$Version" }
    )) {
        $name = $src.Name; $base = $src.Base
        Write-Host "  --- $name ---"
        try {
            # 下载（nocache 穿透 CDN 缓存）
            Invoke-WebRequest "$base/$assetName`?nocache=$(Get-Date -Format yyyyMMddHHmmss)" `
                -OutFile $tmp -TimeoutSec 300
            $sha = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLowerInvariant()

            # version 验证（仅 Windows 平台产物可在本机执行）
            if ($isWindowsPlat) {
                $ver = (& $tmp version 2>&1 | Select-Object -First 1)
                Report "$name version 输出" ($ver -match "pvm\s+$Version") "ver=$ver"
            }

            # SHA 对比（有本地基准时）
            if ($localSha) {
                Report "$name SHA-256 与本地一致" ($sha -eq $localSha) "sha=$($sha.Substring(0, 12))..."
            }

            # 附加资产存在性
            foreach ($f in $extra) {
                $ok = $false
                try {
                    Invoke-WebRequest "$base/$f" -Method Head -TimeoutSec 60 | Out-Null
                    $ok = $true
                } catch { $ok = $false }
                Report "$name $f" $ok $(if (-not $ok) { "缺失，请上传" } else { "" })
            }
        } catch {
            Report "$name $assetName 下载" $false "$($_.Exception.Message)"
        }
    }

    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    Write-Host ""
    Write-Host "结果: $script:pass 通过, $script:fail 失败"
    Write-Host $(if ($script:fail -eq 0) { "✅ 双平台资产验证通过，发布闭环完成" } else { "❌ 存在不一致项，见上方 ❌ 条目" })
    exit $(if ($script:fail -eq 0) { 0 } else { 1 })
}

# ---------- 分发 ----------
switch ($Action) {
    "check-tag"      { Invoke-CheckTag }
    "actions-status" { Invoke-ActionsStatus }
    "verify"         { Invoke-VerifyImpl }
    "push"           { Invoke-Push }
    default {
        Write-Host "用法: release-harness.ps1 check-tag|actions-status|verify|push [-Version x.y.z] [-Platform windows-amd64]"
        exit 2
    }
}
