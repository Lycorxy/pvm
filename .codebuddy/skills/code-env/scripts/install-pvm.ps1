<#
.SYNOPSIS
  Detect Windows and native CPU architecture, then run the canonical PVM installer.
#>

[CmdletBinding()]
param(
  [string]$Version = $env:PVM_INSTALL_VERSION,
  [string]$Source = $env:PVM_INSTALL_SOURCE,
  [switch]$DetectOnly
)

$ErrorActionPreference = 'Stop'
if (-not $Source) { $Source = 'auto' }
$Source = $Source.ToLowerInvariant()
if (@('auto', 'gitee', 'github') -notcontains $Source) {
  throw "unsupported source: $Source (use auto, gitee, or github)"
}
if ($env:OS -ne 'Windows_NT') {
  throw 'install-pvm.ps1 supports Windows only; use install-pvm.sh on macOS/Linux'
}

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

switch ($architectureName.ToUpperInvariant()) {
  { $_ -in @('X64', 'AMD64') } { $architecture = 'amd64'; break }
  { $_ -in @('ARM64', 'AARCH64') } { $architecture = 'arm64'; break }
  default { throw "no PVM release is available for architecture: $architectureName" }
}

$asset = "pvm-windows-$architecture.exe"
Write-Output "platform=windows architecture=$architecture asset=$asset"
if ($DetectOnly) { return }

$localRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..\..'))
$localInstaller = Join-Path $localRoot 'scripts\install.ps1'
$localModule = Join-Path $localRoot 'go.mod'

function Invoke-CanonicalInstaller {
  param(
    [string]$InstallerPath,
    [string]$InstallerSource = $Source
  )

  $arguments = @{ Source = $InstallerSource }
  if ($Version) { $arguments.Version = $Version }
  & $InstallerPath @arguments
}

$useLocalInstaller = $false
if ((Test-Path -LiteralPath $localInstaller -PathType Leaf) -and
    (Test-Path -LiteralPath $localModule -PathType Leaf)) {
  $moduleText = [IO.File]::ReadAllText($localModule)
  $useLocalInstaller = $moduleText -match '(?m)^module\s+github\.com/pvm/pvm\s*$'
}
if ($useLocalInstaller) {
  Invoke-CanonicalInstaller $localInstaller
  return
}

try {
  [Net.ServicePointManager]::SecurityProtocol = `
    [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
  # Use the operating system default when TLS configuration is unavailable.
}

$tempInstaller = Join-Path ([IO.Path]::GetTempPath()) "pvm-bootstrap-$([Guid]::NewGuid().ToString('N')).ps1"

function Receive-Installer {
  param([string]$Uri)

  for ($attempt = 1; $attempt -le 3; $attempt++) {
    try {
      Remove-Item -LiteralPath $tempInstaller -Force -ErrorAction SilentlyContinue
      $parameters = @{
        Uri         = $Uri
        OutFile     = $tempInstaller
        Headers     = @{ 'User-Agent' = 'pvm-skill-installer' }
        TimeoutSec  = 60
        ErrorAction = 'Stop'
      }
      if ((Get-Command Invoke-WebRequest).Parameters.ContainsKey('UseBasicParsing')) {
        $parameters.UseBasicParsing = $true
      }
      Invoke-WebRequest @parameters | Out-Null
      if (-not (Test-Path -LiteralPath $tempInstaller -PathType Leaf) -or
          (Get-Item -LiteralPath $tempInstaller).Length -le 0) {
        throw 'downloaded installer is empty'
      }
      $installerText = [IO.File]::ReadAllText($tempInstaller)
      if (-not $installerText.Contains('PVM_INSTALL_SOURCE') -or
          -not $installerText.Contains('pvm-installer')) {
        throw 'downloaded content is not the canonical PVM installer'
      }

      $tokens = $null
      $parseErrors = $null
      [Management.Automation.Language.Parser]::ParseFile(
        $tempInstaller,
        [ref]$tokens,
        [ref]$parseErrors
      ) | Out-Null
      if ($parseErrors.Count -gt 0) {
        throw "downloaded installer has invalid PowerShell syntax: $($parseErrors[0].Message)"
      }
      return $true
    } catch {
      Remove-Item -LiteralPath $tempInstaller -Force -ErrorAction SilentlyContinue
      if ($attempt -lt 3) { Start-Sleep -Seconds $attempt }
    }
  }
  return $false
}

$installerUrls = switch ($Source) {
  'gitee' { @('https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1') }
  'github' { @('https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.ps1') }
  default {
    @(
      'https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1',
      'https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.ps1'
    )
  }
}

try {
  $installed = $false
  foreach ($url in $installerUrls) {
    Write-Host "downloading installer from $url"
    if (-not (Receive-Installer $url)) {
      Write-Warning "invalid installer from $url; trying another source"
      continue
    }

    $installerSource = if ($url -like '*gitee.com*') { 'gitee' } else { 'github' }
    try {
      Invoke-CanonicalInstaller $tempInstaller $installerSource
      $installed = $true
      break
    } catch {
      Write-Warning "installer from $url failed: $($_.Exception.Message)"
    }
  }
  if (-not $installed) {
    throw 'unable to install PVM from any configured source'
  }
} finally {
  Remove-Item -LiteralPath $tempInstaller -Force -ErrorAction SilentlyContinue
}
