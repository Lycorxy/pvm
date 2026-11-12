# pvm — 同步本地构建产物到 Gitee Release
#
# 用法：
#   $env:GITEE_TOKEN = '你的 Gitee 私人令牌'   # Gitee → 设置 → 私人令牌（需 releases 权限）
#   .\scripts\sync-gitee.ps1                    # 同步 VERSION 文件对应的 tag
#   .\scripts\sync-gitee.ps1 -Tag v0.0.3        # 指定 tag
#
# 行为：
#   1. 收集 dist\ 下 6 个二进制 + dist\packages\ 下 zip / checksums.txt / skill 包
#   2. Gitee 上不存在该 tag 的 release 则自动创建（否则复用）
#   3. 逐个上传资产（curl multipart）
#
# 推荐流程：GitHub 网页（或 Actions）发布后，跑一次本脚本即可双平台同步。

param(
    [string]$Token = $env:GITEE_TOKEN,
    [string]$Tag = "",
    [string]$Repo = "lycorxy/pvm",
    [string]$DistDir = "",
    [string]$Notes = ""    # 发布说明：创建 release 时作为描述；release 已存在时更新描述
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not $DistDir) { $DistDir = Join-Path $ProjectRoot "dist" }
$PackagesDir = Join-Path $DistDir "packages"

# 令牌优先级：-Token 参数 → GITEE_TOKEN 环境变量 → ~/.release.yaml（release-init.ps1 生成）
if (-not $Token) {
    $cfgPath = Join-Path $env:USERPROFILE ".release.yaml"
    if (Test-Path $cfgPath) {
        $sec = $null
        foreach ($line in (Get-Content $cfgPath)) {
            if ($line -match '^gitee:') { $sec = 'gt' }
            elseif ($line -match '^\s+token:\s*(.*)$') {
                if ($sec -eq 'gt') { $Token = $Matches[1].Trim().Trim('"'); break }
            }
        }
    }
}
if (-not $Token) {
    Write-Host "error: 需要 Gitee 私人令牌。三选一：" -ForegroundColor Red
    Write-Host "  1. 运行 .\scripts\release-init.ps1 一次性配置（推荐，存 ~/.release.yaml）"
    Write-Host "  2. `$env:GITEE_TOKEN = '令牌'"
    Write-Host "  3. -Token 参数"
    exit 1
}

# 确定 tag（默认取 VERSION 文件，前加 v）
if (-not $Tag) {
    $versionFile = Join-Path $ProjectRoot "VERSION"
    if (-not (Test-Path $versionFile)) {
        Write-Host "error: 未找到 VERSION 文件，请用 -Tag 指定" -ForegroundColor Red
        exit 1
    }
    $Tag = "v$((Get-Content $versionFile -Raw).Trim())"
}
Write-Host "=== 同步到 Gitee: $Repo @$Tag ===" -ForegroundColor Cyan

# 1. 收集资产
$assets = @()
$assets += @(Get-ChildItem $DistDir -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -match '^pvm-(windows|darwin|linux)-(amd64|arm64)(\.exe)?$' })
if (Test-Path $PackagesDir) {
    $assets += @(Get-ChildItem $PackagesDir -File |
        Where-Object { $_.Name -match "^pvm-$Tag-" -or $_.Name -in @('checksums.txt', 'pvm-env-manager.zip') })
}
if ($assets.Count -eq 0) {
    Write-Host "error: 未找到可上传的资产（先跑 .\scripts\build-all.ps1）" -ForegroundColor Red
    exit 1
}
Write-Host "待上传资产 $($assets.Count) 个:"
$assets | ForEach-Object { Write-Host ("  {0}  ({1} KB)" -f $_.Name, [math]::Round($_.Length / 1KB)) }

$api = "https://gitee.com/api/v5/repos/$Repo"

# 2. 查询 release 是否已存在（不存在则创建）
$release = $null
try {
    $release = Invoke-RestMethod -Uri "$api/releases/tags/$Tag`?access_token=$Token" -TimeoutSec 30
    Write-Host "Gitee release 已存在 (id=$($release.id))"
} catch {
    Write-Host "Gitee release 不存在，创建中..."
    $releaseBody = if ($Notes) { $Notes } else { "pvm $Tag (synced by sync-gitee.ps1)" }
    $body = @{
        access_token     = $Token
        tag_name         = $Tag
        name             = "pvm $Tag"
        body             = $releaseBody
        target_commitish = "master"
    } | ConvertTo-Json
    $release = Invoke-RestMethod -Uri "$api/releases" -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 30
    Write-Host "已创建 Gitee release (id=$($release.id))"
}

# 已存在且指定了发布说明 → 更新描述
if ($release -and $Notes) {
    $patchBody = @{ access_token = $Token; body = $Notes } | ConvertTo-Json
    Invoke-RestMethod -Uri "$api/releases/$($release.id)" -Method Patch -ContentType 'application/json' -Body $patchBody -TimeoutSec 30 | Out-Null
    Write-Host "✓ 发布说明已更新"
}

# 3. 逐个上传（curl multipart，Windows 10+ 自带 curl.exe；PS5.1 的 Invoke-RestMethod 不支持 -Form）
$uploadUrl = "$api/releases/$($release.id)/attach_files"
$failed = @()
foreach ($a in $assets) {
    Write-Host "  上传 $($a.Name) ..." -NoNewline
    $resp = & curl.exe -sS -X POST "$uploadUrl`?access_token=$Token" -F "file=@$($a.FullName)" 2>&1
    if ($LASTEXITCODE -eq 0 -and $resp -notmatch '"message"') {
        Write-Host " OK" -ForegroundColor Green
    } else {
        Write-Host " 失败" -ForegroundColor Yellow
        $failed += $a.Name
    }
}

Write-Host ""
if ($failed.Count -gt 0) {
    Write-Host "⚠️  $($failed.Count) 个资产上传失败: $($failed -join ', ')" -ForegroundColor Yellow
    Write-Host "   重复运行本脚本即可重试（同名资产在 Gitee 网页上删除后可重传）"
    exit 1
}
Write-Host "✓ 同步完成: $Repo @$Tag（共 $($assets.Count) 个资产）" -ForegroundColor Green
Write-Host "  地址: https://gitee.com/$Repo/releases/tag/$Tag"
