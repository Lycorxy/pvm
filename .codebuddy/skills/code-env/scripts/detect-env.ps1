<#
.SYNOPSIS
  Detect the current operating system, CPU architecture, shell, and existing PVM
  installation, then print the matching PVM install target.

.DESCRIPTION
  Prints key=value lines and a human-readable recommendation. Exits 0 when the
  environment is supported, 1 when unsupported, 2 on usage error.
#>

[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

if ($env:OS -ne 'Windows_NT') {
  throw 'detect-env.ps1 supports Windows only; use detect-env.sh on macOS/Linux'
}

# --- operating system ---
$os = 'windows'

# --- CPU architecture (native, not WOW64 emulation) ---
$architectureName = $null
try {
  $architectureName = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
} catch {
  if ($env:PROCESSOR_ARCHITEW6432) {
    $architectureName = $env:PROCESSOR_ARCHITEW6432
  } else {
    $architectureName = $env:PROCESSOR_ARCHITECTURE
  }
}

$arch = $null
switch ($architectureName.ToUpperInvariant()) {
  { $_ -in @('X64', 'AMD64') } { $arch = 'amd64'; break }
  { $_ -in @('ARM64', 'AARCH64') } { $arch = 'arm64'; break }
  default { $arch = $null }
}

$supported = $null -ne $arch
$asset = if ($supported) { "pvm-windows-$arch.exe" } else { $null }

# --- existing PVM installation ---
$pvmPath = $null
$command = Get-Command pvm -ErrorAction SilentlyContinue
if ($command) { $pvmPath = $command.Source }

$candidates = @()
if ($env:PVM_HOME) {
  $candidates += Join-Path ([Environment]::ExpandEnvironmentVariables($env:PVM_HOME)) 'bin\pvm.exe'
}
$candidates += Join-Path $env:USERPROFILE '.pvm\bin\pvm.exe'
foreach ($candidate in $candidates) {
  if (-not $pvmPath -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
    $pvmPath = $candidate
  }
}

$pvmVersion = $null
if ($pvmPath) {
  try {
    $output = @(& $pvmPath version 2>&1)
    if ($LASTEXITCODE -eq 0) {
      foreach ($line in $output) {
        if ([string]$line -match '^pvm\s+v?(\S+)\s*$') {
          $pvmVersion = $Matches[1]
          break
        }
      }
    }
  } catch {
    $pvmVersion = $null
  }
}

# --- report ---
Write-Output "os=$os"
Write-Output "arch=$arch"
Write-Output "arch_raw=$architectureName"
Write-Output "shell=powershell"
Write-Output "supported=$($supported.ToString().ToLowerInvariant())"
Write-Output "pvm_installed=$(([bool]$pvmPath).ToString().ToLowerInvariant())"
if ($pvmVersion) { Write-Output "pvm_version=$pvmVersion" }
if ($pvmPath) { Write-Output "pvm_path=$pvmPath" }
if ($supported) { Write-Output "asset=$asset" }
Write-Output 'installer=install-pvm.ps1'
Write-Output ''

$skillRoot = $PSScriptRoot
if (-not $supported) {
  Write-Host "No PVM release is available for Windows $architectureName." -ForegroundColor Red
  exit 1
}

if ($pvmVersion) {
  Write-Host "PVM $pvmVersion is already installed at:" -ForegroundColor Green
  Write-Host "  $pvmPath"
  Write-Host 'No reinstallation is needed. To upgrade, run:'
  Write-Host '  pvm self-update'
} else {
  Write-Host "Detected Windows $arch. Recommended install:" -ForegroundColor Cyan
  Write-Host "  & `"$skillRoot\install-pvm.ps1`""
  Write-Host 'Or the one-line remote installer:'
  Write-Host '  irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex'
}
