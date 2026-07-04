<#
.SYNOPSIS
  pvm — Polyglot Version Manager · one-liner installer (Windows)

.EXAMPLE
  # 默认安装最新版：
  iwr -useb https://gitee.com/lucky-zsh/pvm/raw/main/scripts/install.ps1 | iex

  # 指定版本：
  $env:PVM_INSTALL_VERSION = "v1.0.0"
  iwr -useb https://gitee.com/lucky-zsh/pvm/raw/main/scripts/install.ps1 | iex

.NOTES
  支持的环境变量：
    PVM_HOME                安装根目录 (默认 %USERPROFILE%\.pvm)
    PVM_REPO                Gitee 仓库 (默认 lucky-zsh/pvm)
    PVM_INSTALL_VERSION     指定版本 (默认最新)
    PVM_NO_MODIFY_PATH=1    不修改用户 PATH
#>

[CmdletBinding()]
param(
  [string]$Version
)

$ErrorActionPreference = 'Stop'

function Write-Step($msg)  { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Warn2($msg) { Write-Host "!!  $msg" -ForegroundColor Yellow }
function Write-Die($msg)   { Write-Host "✗   $msg" -ForegroundColor Red; exit 1 }

# ---- 参数解析 ----
$Repo = $env:PVM_REPO
if (-not $Repo) { $Repo = 'lucky-zsh/pvm' }

$PvmHome = $env:PVM_HOME
if (-not $PvmHome) { $PvmHome = Join-Path $env:USERPROFILE '.pvm' }

if (-not $Version) { $Version = $env:PVM_INSTALL_VERSION }

# ---- 体系结构检测 ----
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { Write-Die "unsupported arch: $($env:PROCESSOR_ARCHITECTURE)" }
}

# ---- 解析最新版本 ----
if (-not $Version) {
  Write-Step "resolving latest release of $Repo ..."
  try {
    $release = Invoke-RestMethod -UseBasicParsing `
      -Uri "https://gitee.com/api/v3/repos/$Repo/releases/latest" `
      -Headers @{ 'User-Agent' = 'pvm-installer' }
    $Version = $release.tag_name
  } catch {
    Write-Die "failed to query Gitee API: $_"
  }
  if (-not $Version) { Write-Die 'could not resolve latest version' }
}

Write-Step "installing pvm $Version (windows/$arch) to $PvmHome"

# ---- 创建目录 ----
$null = New-Item -ItemType Directory -Force -Path `
  (Join-Path $PvmHome 'bin'), `
  (Join-Path $PvmHome 'shims'), `
  (Join-Path $PvmHome 'installs'), `
  (Join-Path $PvmHome 'cache')

# ---- 下载 ----
$asset = "pvm-windows-$arch.exe"
$url   = "https://gitee.com/$Repo/releases/download/$Version/$asset"
$dest  = Join-Path $PvmHome 'bin\pvm.exe'

Write-Step "downloading $url"
try {
  Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $dest
} catch {
  Write-Die "download failed: $_"
}

# ---- 初始 reshim ----
try {
  & $dest reshim | Out-Null
} catch {
  # 首次可能失败，忽略
}

# ---- 更新用户 PATH ----
if ($env:PVM_NO_MODIFY_PATH -ne '1') {
  $shimsDir = Join-Path $PvmHome 'shims'
  $binDir   = Join-Path $PvmHome 'bin'

  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if ($null -eq $userPath) { $userPath = '' }

  $parts = $userPath -split ';' | Where-Object { $_ -ne '' }

  # 已知会与 pvm shims 冲突的系统 runtime 目录名
  $conflictNames = @('nodejs', 'python', 'go', 'golang')
  $conflictsRemoved = @()

  # 1. 移除用户 PATH 中与 pvm 冲突的目录
  $filtered = @()
  foreach ($p in $parts) {
    $base = [System.IO.Path]::GetFileName([System.IO.Path]::TrimEndingDirectorySeparator($p))
    $isConflict = $false
    foreach ($c in $conflictNames) {
      if ($base -ieq $c) {
        $isConflict = $true
        break
      }
      # 前缀匹配：处理 Python 版本号目录 (如 Python312, Python39)
      if ($base.Length -gt $c.Length -and $base.Substring(0, $c.Length) -ieq $c) {
        $isConflict = $true
        break
      }
    }
    if ($isConflict -and $p -ine $shimsDir -and $p -ine $binDir) {
      $conflictsRemoved += $p
    } else {
      $filtered += $p
    }
  }
  $parts = $filtered

  # 2. 移除已有的 pvm 条目（确保重新前置到最前面）
  $parts = $parts | Where-Object { $_ -ine $shimsDir -and $_ -ine $binDir }

  # 3. 将 shims 和 bin 前置到最前面
  $parts = @($shimsDir, $binDir) + $parts

  if ($conflictsRemoved.Count -gt 0) {
    Write-Host "!!  Removed conflicting paths from user PATH:" -ForegroundColor Yellow
    foreach ($c in $conflictsRemoved) {
      Write-Host "    - $c" -ForegroundColor Yellow
    }
    Write-Host "    (pvm shims will take priority instead)" -ForegroundColor Yellow
  }

  $newPath = ($parts -join ';')
  [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
  Write-Step "updated user PATH (pvm shims first)"

  # 同步当前会话，避免用户再开一个终端
  $env:Path = "$shimsDir;$binDir;$env:Path"

  # ---- 检测并修复系统 PATH（Machine）中的冲突目录 ----
  # Windows PATH 合并顺序：系统 PATH + 用户 PATH
  # 若系统 PATH 中存在冲突的 runtime 目录，它们会排在 shims 前面，导致 pvm use 无效
  $sysPath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
  if ($sysPath) {
    $sysParts = $sysPath -split ';' | Where-Object { $_ -ne '' }
    $sysConflicts = @()
    $sysClean = @()
    foreach ($p in $sysParts) {
      $base = [System.IO.Path]::GetFileName([System.IO.Path]::TrimEndingDirectorySeparator($p))
      $isConflict = $false
      foreach ($c in $conflictNames) {
        if ($base -ieq $c) { $isConflict = $true; break }
        if ($base.Length -gt $c.Length -and $base.Substring(0, $c.Length) -ieq $c) {
          $isConflict = $true; break
        }
      }
      if ($isConflict) { $sysConflicts += $p } else { $sysClean += $p }
    }

    if ($sysConflicts.Count -gt 0) {
      Write-Host ""
      Write-Host "!!  Conflicting runtime paths found in SYSTEM PATH:" -ForegroundColor Yellow
      foreach ($c in $sysConflicts) {
        Write-Host "    - $c" -ForegroundColor Yellow
      }
      Write-Host "!!  These system paths override pvm shims, causing 'pvm use' to have no effect." -ForegroundColor Yellow

      # 检测是否有管理员权限
      $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator)

      if ($isAdmin) {
        $newSysPath = ($sysClean -join ';')
        [Environment]::SetEnvironmentVariable('Path', $newSysPath, 'Machine')
        Write-Host "==> Removed conflicting paths from SYSTEM PATH (admin)" -ForegroundColor Green
        Write-Host "==> pvm shims will take effect after reopening terminal" -ForegroundColor Green
      } else {
        Write-Host ""
        Write-Host "!!  To fix, re-run this installer as Administrator, or manually remove via:" -ForegroundColor Yellow
        Write-Host "    System Properties -> Environment Variables -> System variables -> Path" -ForegroundColor Yellow
        Write-Host "    Remove the entries listed above, then reopen your terminal." -ForegroundColor Yellow
      }
    }
  }
}

Write-Host ""
Write-Step "pvm $Version installed to $dest"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Open a new terminal (so PATH changes take effect)"
Write-Host "  2. pvm install node@20.11.0"
Write-Host "  3. cd into a project and run: pvm init"
Write-Host ""
