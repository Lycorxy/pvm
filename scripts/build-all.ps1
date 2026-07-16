# PVM 一键打包脚本（全部平台）
# 用法：
#   .\scripts\build-all.ps1              # 自动读取版本号，打包所有平台
#   .\scripts\build-all.ps1 -Version 1.2.3  # 指定版本号
#   .\scripts\build-all.ps1 -NoMSI       # 跳过 MSI 打包
#
# 依赖：
#   1. Go 编译器（需要已安装并加入 PATH）
#   2. WiX Toolset（仅 MSI 打包需要）

param(
    [string]$Version = "",
    [switch]$NoMSI,
    [string]$Arch = "all"  # all | amd64 | arm64
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot

# ────────────────────────────────────────
# 1. 确定版本号
# ────────────────────────────────────────
if (-not $Version) {
    $rootGo = Get-Content (Join-Path $ProjectRoot "cmd/root.go") -Raw -ErrorAction SilentlyContinue
    if ($rootGo -match 'Version\s*=\s*"([^"]+)"') {
        $Version = $Matches[1]
    } else {
        $Version = "0.0.0"
    }
}
$Version = $Version -replace '^v', ''
Write-Host "=== PVM Build All ===" -ForegroundColor Cyan
Write-Host "Version : $Version" -ForegroundColor Yellow
Write-Host "Project : $ProjectRoot" -ForegroundColor Yellow
Write-Host ""

# ────────────────────────────────────────
# 2. 检查 Go 编译器
# ────────────────────────────────────────
Write-Host "[1/3] Checking Go compiler..." -ForegroundColor Yellow
try {
    $goVersion = go version 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Go not found" }
    Write-Host "  $goVersion" -ForegroundColor Green
} catch {
    Write-Host "  ERROR: Go not found in PATH!" -ForegroundColor Red
    Write-Host "  Please install Go from: https://go.dev/dl/" -ForegroundColor Yellow
    exit 1
}

# ────────────────────────────────────────
# 3. 编译所有平台
# ────────────────────────────────────────
Write-Host "`n[2/3] Building binaries..." -ForegroundColor Yellow

$platforms = @()
if ($Arch -eq "all" -or $Arch -eq "amd64") {
    $platforms += @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe"; Suffix = "windows-amd64" }
    $platforms += @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "";     Suffix = "darwin-amd64"  }
    $platforms += @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "";     Suffix = "linux-amd64"   }
}
if ($Arch -eq "all" -or $Arch -eq "arm64") {
    $platforms += @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe"; Suffix = "windows-arm64" }
    $platforms += @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "";     Suffix = "darwin-arm64"  }
    $platforms += @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "";     Suffix = "linux-arm64"   }
}

$DistDir = Join-Path $ProjectRoot "dist"
New-Item -ItemType Directory -Path $DistDir -Force | Out-Null

$exePaths = @()  # 收集编译产物路径，供后续打包

foreach ($p in $platforms) {
    $env:GOOS   = $p.GOOS
    $env:GOARCH = $p.GOARCH

    # 编译 pvm 主程序
    $outputName = "pvm-$($p.GOOS)-$($p.GOARCH)$($p.Ext)"
    $outputPath = Join-Path $DistDir $outputName

    Write-Host "  Building $($p.Suffix)..." -ForegroundColor Gray -NoNewline
    $buildOut = go build -o $outputPath . 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host "    $buildOut" -ForegroundColor DarkRed
        continue
    }
    Write-Host " OK" -ForegroundColor Green
    $exePaths += $outputPath

    # 单二进制方案：pvm 本身即 shim 源（reshim 硬链接它为各命令名），无需单独编译 pvm-shim
}

# 恢复当前平台环境变量
$env:GOOS   = ""
$env:GOARCH = ""

# ────────────────────────────────────────
# 4. 打包（zip / MSI）
# ────────────────────────────────────────
Write-Host "`n[3/3] Packaging..." -ForegroundColor Yellow

# 4a. 打包 zip
& "$PSScriptRoot\package.ps1" -Version $Version
if ($LASTEXITCODE -ne 0) {
    Write-Host "  [WARN] package.ps1 failed" -ForegroundColor Yellow
}

# 4b. 打包 MSI（Windows only）
if (-not $NoMSI -and $platforms[0].GOOS -eq "windows") {
    # 检查是否有 Windows amd64 产物
    $msiSource = $exePaths | Where-Object { $_ -like "*windows-amd64*" }
    if ($msiSource) {
        # 只在源和目标路径不同时才复制
        $msiDest = Join-Path $DistDir "pvm-windows-amd64.exe"
        if ($msiSource -ne $msiDest) {
            Copy-Item $msiSource $msiDest -Force
        }
        & "$PSScriptRoot\build-msi.ps1" -Version $Version -Arch "amd64"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  MSI: OK" -ForegroundColor Green
        } else {
            Write-Host "  [WARN] MSI build failed" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  [SKIP] No Windows binary found, skipping MSI" -ForegroundColor Gray
    }
}

# ────────────────────────────────────────
# 完成
# ────────────────────────────────────────
Write-Host "`n=== BUILD COMPLETE ===" -ForegroundColor Cyan
Write-Host "Output directory: $DistDir" -ForegroundColor Yellow
Get-ChildItem $DistDir -File | Format-Table Name, Length, LastWriteTime -AutoSize
