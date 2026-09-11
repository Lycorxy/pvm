<#
.SYNOPSIS
  pvm — Polyglot Version Manager · one-line installer (Windows)

.EXAMPLE
  irm https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1 | iex

.EXAMPLE
  irm https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.ps1 | iex

.EXAMPLE
  $env:PVM_INSTALL_VERSION = 'v0.0.1'
  $env:PVM_INSTALL_SOURCE = 'github'
  irm https://raw.githubusercontent.com/Lycorxy/pvm/master/scripts/install.ps1 | iex

.NOTES
  Environment variables:
    PVM_HOME                 Install root (default: %USERPROFILE%\.pvm)
    PVM_INSTALL_VERSION      Release tag to install (default: latest)
    PVM_INSTALL_SOURCE       auto, gitee, or github (default: auto)
    PVM_GITEE_REPO           Gitee repository (default: lycorxy/pvm)
    PVM_GITHUB_REPO          GitHub repository (default: Lycorxy/pvm)
    PVM_REPO                 Compatibility override for both repositories
    PVM_REQUIRE_CHECKSUM=1   Fail if checksums.txt is unavailable
    PVM_NO_MODIFY_PATH=1     Do not modify persistent PATH settings
#>

[CmdletBinding()]
param(
  [string]$Version,
  [string]$Source
)

if ($MyInvocation.InvocationName -eq 'Invoke-Expression') {
  $scriptText = $MyInvocation.MyCommand.Definition
  & ([scriptblock]::Create($scriptText)) @PSBoundParameters
  return
}

$ErrorActionPreference = 'Stop'

function Write-Step {
  param([string]$Message)
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-InstallerWarning {
  param([string]$Message)
  Write-Host "!!  $Message" -ForegroundColor Yellow
}

function Stop-Installer {
  param([string]$Message)
  throw "pvm installer: $Message"
}

function Get-Repository {
  param([string]$ReleaseSource)
  if ($ReleaseSource -eq 'gitee') { return $script:GiteeRepo }
  return $script:GitHubRepo
}

function Get-ReleaseApiUrl {
  param([string]$ReleaseSource)
  $repo = Get-Repository $ReleaseSource
  if ($ReleaseSource -eq 'gitee') {
    return "https://gitee.com/api/v5/repos/$repo/releases/latest"
  }
  return "https://api.github.com/repos/$repo/releases/latest"
}

function Get-AssetUrl {
  param(
    [string]$ReleaseSource,
    [string]$Release,
    [string]$Asset
  )

  $repo = Get-Repository $ReleaseSource
  $hostName = if ($ReleaseSource -eq 'gitee') { 'gitee.com' } else { 'github.com' }
  if ($Release -eq 'latest') {
    return "https://$hostName/$repo/releases/latest/download/$Asset"
  }
  return "https://$hostName/$repo/releases/download/$Release/$Asset"
}

function Invoke-PvmDownload {
  param(
    [string]$Uri,
    [string]$OutFile,
    [switch]$Quiet
  )

  for ($attempt = 1; $attempt -le 3; $attempt++) {
    try {
      Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
      $parameters = @{
        Uri         = $Uri
        OutFile     = $OutFile
        Headers     = @{ 'User-Agent' = 'pvm-installer' }
        TimeoutSec  = 60
        ErrorAction = 'Stop'
      }
      if ((Get-Command Invoke-WebRequest).Parameters.ContainsKey('UseBasicParsing')) {
        $parameters.UseBasicParsing = $true
      }
      Invoke-WebRequest @parameters | Out-Null
      if (-not (Test-Path -LiteralPath $OutFile -PathType Leaf)) {
        throw 'download did not create a file'
      }
      if ((Get-Item -LiteralPath $OutFile).Length -le 0) {
        throw 'downloaded file is empty'
      }
      return $true
    } catch {
      Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
      if ($attempt -lt 3) {
        Start-Sleep -Seconds $attempt
      } elseif (-not $Quiet) {
        Write-InstallerWarning "download failed: $Uri ($($_.Exception.Message))"
      }
    }
  }
  return $false
}

function Get-LatestReleaseTag {
  param([string]$ReleaseSource)

  try {
    $parameters = @{
      Uri         = Get-ReleaseApiUrl $ReleaseSource
      Headers     = @{ 'User-Agent' = 'pvm-installer' }
      TimeoutSec  = 30
      ErrorAction = 'Stop'
    }
    if ((Get-Command Invoke-RestMethod).Parameters.ContainsKey('UseBasicParsing')) {
      $parameters.UseBasicParsing = $true
    }
    $release = Invoke-RestMethod @parameters
    if ($release.tag_name) {
      return [string]$release.tag_name
    }
  } catch {
    return $null
  }
  return $null
}

function Normalize-Version {
  param([string]$Value)
  if (-not $Value -or $Value -eq 'latest' -or $Value.StartsWith('v')) {
    return $Value
  }
  if ($Value -match '^\d') {
    return "v$Value"
  }
  return $Value
}

function Get-Sha256 {
  param([string]$Path)

  if (Get-Command Get-FileHash -ErrorAction SilentlyContinue) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
  }

  $stream = [System.IO.File]::OpenRead($Path)
  $sha256 = [System.Security.Cryptography.SHA256]::Create()
  try {
    $hash = $sha256.ComputeHash($stream)
    return ([System.BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
  } finally {
    $sha256.Dispose()
    $stream.Dispose()
  }
}

function Test-PvmChecksum {
  param(
    [string]$ReleaseSource,
    [string]$Release,
    [string]$Asset,
    [string]$BinaryPath,
    [string]$ChecksumPath
  )

  $checksumUrl = Get-AssetUrl $ReleaseSource $Release 'checksums.txt'
  if (-not (Invoke-PvmDownload -Uri $checksumUrl -OutFile $ChecksumPath -Quiet)) {
    if ($script:RequireChecksum) {
      Write-InstallerWarning "checksums.txt is unavailable from $ReleaseSource"
      return $false
    }
    Write-InstallerWarning 'checksums.txt is unavailable; continuing with executable validation'
    return $true
  }

  $expected = $null
  foreach ($line in Get-Content -LiteralPath $ChecksumPath) {
    if ([string]$line -match '^\uFEFF?([0-9A-Fa-f]{64})\s+\*?(.+?)\s*$') {
      if ($Matches[2] -eq $Asset) {
        $expected = $Matches[1].ToLowerInvariant()
        break
      }
    }
  }

  if (-not $expected) {
    if ($script:RequireChecksum) {
      Write-InstallerWarning "no checksum entry found for $Asset"
      return $false
    }
    Write-InstallerWarning "no checksum entry found for $Asset; continuing with executable validation"
    return $true
  }

  $actual = Get-Sha256 $BinaryPath
  if ($actual -ne $expected) {
    Stop-Installer "checksum mismatch for $Asset"
  }
  Write-Step "verified SHA-256 for $Asset"
  return $true
}

if ($env:OS -ne 'Windows_NT') {
  Stop-Installer 'this installer supports Windows only; use install.sh on macOS or Linux'
}
if (-not $env:USERPROFILE) {
  Stop-Installer 'USERPROFILE is not set'
}

try {
  [Net.ServicePointManager]::SecurityProtocol = `
    [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
  Write-InstallerWarning 'could not explicitly enable TLS 1.2; using the system default'
}

if (-not $Version) { $Version = $env:PVM_INSTALL_VERSION }
if (-not $Source) { $Source = $env:PVM_INSTALL_SOURCE }
if (-not $Source) { $Source = 'auto' }
$Source = $Source.ToLowerInvariant()
if (@('auto', 'gitee', 'github') -notcontains $Source) {
  Stop-Installer "unsupported source: $Source (use auto, gitee, or github)"
}

$repoOverride = $env:PVM_REPO
$script:GiteeRepo = $env:PVM_GITEE_REPO
if (-not $script:GiteeRepo) {
  $script:GiteeRepo = if ($repoOverride) { $repoOverride } else { 'lycorxy/pvm' }
}
$script:GitHubRepo = $env:PVM_GITHUB_REPO
if (-not $script:GitHubRepo) {
  $script:GitHubRepo = if ($repoOverride) { $repoOverride } else { 'Lycorxy/pvm' }
}

foreach ($setting in @('PVM_REQUIRE_CHECKSUM', 'PVM_NO_MODIFY_PATH')) {
  $settingValue = [Environment]::GetEnvironmentVariable($setting)
  if ($settingValue -and $settingValue -notin @('0', '1')) {
    Stop-Installer "$setting must be 0 or 1"
  }
}
$script:RequireChecksum = $env:PVM_REQUIRE_CHECKSUM -eq '1'
$noModifyPath = $env:PVM_NO_MODIFY_PATH -eq '1'

$pvmHome = $env:PVM_HOME
if (-not $pvmHome) { $pvmHome = Join-Path $env:USERPROFILE '.pvm' }
$pvmHome = [Environment]::ExpandEnvironmentVariables($pvmHome)

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
  default { Stop-Installer "unsupported architecture: $architectureName" }
}

$binDir = Join-Path $pvmHome 'bin'
$shimsDir = Join-Path $pvmHome 'shims'
$tmpDir = Join-Path $pvmHome 'tmp'
$destination = Join-Path $binDir 'pvm.exe'
$asset = "pvm-windows-$architecture.exe"

$null = New-Item -ItemType Directory -Force -Path `
  $binDir, $shimsDir, (Join-Path $pvmHome 'installs'), `
  (Join-Path $pvmHome 'cache'), $tmpDir

$stagePath = Join-Path $binDir ".pvm-install-$([Guid]::NewGuid().ToString('N')).exe"
$checksumPath = Join-Path $tmpDir ".pvm-checksums-$([Guid]::NewGuid().ToString('N')).txt"
$backupPath = "$destination.old"

$requestedVersion = Normalize-Version $Version
$sourceOrder = if ($Source -eq 'auto') { @('gitee', 'github') } else { @($Source) }
$selectedSource = $null
$downloadUrl = $null
$binaryVersion = $null

try {
  foreach ($candidateSource in $sourceOrder) {
    $candidateRelease = $requestedVersion
    if (-not $candidateRelease) {
      Write-Step "resolving latest release from $candidateSource ..."
      $candidateRelease = Get-LatestReleaseTag $candidateSource
      if (-not $candidateRelease) {
        Write-InstallerWarning "could not query the latest release from $candidateSource; trying its latest download URL"
        $candidateRelease = 'latest'
      }
    }

    $candidateUrl = Get-AssetUrl $candidateSource $candidateRelease $asset
    Write-Step "downloading $asset from $candidateSource"
    if (-not (Invoke-PvmDownload -Uri $candidateUrl -OutFile $stagePath)) {
      continue
    }

    if (-not (Test-PvmChecksum `
      -ReleaseSource $candidateSource `
      -Release $candidateRelease `
      -Asset $asset `
      -BinaryPath $stagePath `
      -ChecksumPath $checksumPath)) {
      Write-InstallerWarning "checksum verification is required but unavailable from $candidateSource"
      continue
    }

    if (Get-Command Unblock-File -ErrorAction SilentlyContinue) {
      Unblock-File -LiteralPath $stagePath -ErrorAction SilentlyContinue
    }

    try {
      $candidateOutput = @(& $stagePath version 2>&1)
      $candidateExitCode = $LASTEXITCODE
    } catch {
      Write-InstallerWarning "the downloaded file from $candidateSource could not run on windows/$architecture"
      continue
    }
    if ($candidateExitCode -ne 0) {
      Write-InstallerWarning "the downloaded file from $candidateSource exited with code $candidateExitCode"
      continue
    }

    $candidateBinaryVersion = $null
    foreach ($line in $candidateOutput) {
      if ([string]$line -match '^pvm\s+v?(\S+)\s*$') {
        $candidateBinaryVersion = $Matches[1]
        break
      }
    }
    if (-not $candidateBinaryVersion) {
      Write-InstallerWarning "the downloaded file from $candidateSource did not report a valid pvm version"
      continue
    }

    if ($candidateRelease -ne 'latest') {
      $expectedVersion = $candidateRelease.TrimStart('v')
      if ($candidateBinaryVersion -ne $expectedVersion) {
        Write-InstallerWarning "release $candidateRelease contains pvm $candidateBinaryVersion; trying another source"
        continue
      }
    }

    $selectedSource = $candidateSource
    $downloadUrl = $candidateUrl
    $binaryVersion = $candidateBinaryVersion
    break
  }

  if (-not $selectedSource) {
    Stop-Installer "could not download a valid $asset release"
  }

  Write-Step "installing pvm v$binaryVersion (windows/$architecture) to $pvmHome"
  if (Test-Path -LiteralPath $destination -PathType Container) {
    Stop-Installer "installation target is a directory: $destination"
  }

  Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $destination -PathType Leaf) {
    try {
      [System.IO.File]::Replace($stagePath, $destination, $backupPath, $true)
    } catch {
      Stop-Installer "could not replace $destination; close running pvm processes and retry ($($_.Exception.Message))"
    }
  } else {
    [System.IO.File]::Move($stagePath, $destination)
  }
  $stagePath = $null

  $env:PVM_HOME = $pvmHome
  if ($noModifyPath) {
    Write-Step 'initializing shims without modifying persistent PATH settings'
    & $destination reshim
    if ($LASTEXITCODE -ne 0) {
      Write-InstallerWarning "reshim reported warnings; run '$destination reshim' after configuring PATH"
    }
  } else {
    Write-Step 'configuring pvm and Windows PATH'
    & $destination setup --no-elevate
    if ($LASTEXITCODE -ne 0) {
      if (Test-Path -LiteralPath $backupPath) {
        Write-InstallerWarning "the previous executable is available at $backupPath"
      }
      Stop-Installer "pvm was installed to $destination, but setup failed; run '$destination setup' to retry"
    }

    $currentPathEntries = @($env:Path -split ';')
    $prefix = @()
    if ($currentPathEntries -inotcontains $shimsDir) { $prefix += $shimsDir }
    if ($currentPathEntries -inotcontains $binDir) { $prefix += $binDir }
    if ($prefix.Count -gt 0) {
      $env:Path = (($prefix + $currentPathEntries) -join ';')
    }
  }

  $installedVersion = @(& $destination version 2>&1)
  if ($LASTEXITCODE -ne 0 -or -not $installedVersion) {
    Stop-Installer "pvm was installed but failed its final validation: $destination"
  }

  Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue

  Write-Host ''
  Write-Step "$($installedVersion -join ' ') installed successfully"
  Write-Host "  Source: $selectedSource"
  Write-Host "  URL:    $downloadUrl"
  Write-Host "  Path:   $destination"
  Write-Host ''

  if ($noModifyPath) {
    Write-Host 'Persist these settings before opening a new terminal:'
    Write-Host "  [Environment]::SetEnvironmentVariable('PVM_HOME', '$($pvmHome.Replace("'", "''"))', 'User')"
    Write-Host 'Add these directories to your user PATH:'
    Write-Host "  $shimsDir"
    Write-Host "  $binDir"
  } else {
    Write-Host 'Open a new terminal so persistent PATH changes take effect.'
  }
  Write-Host ''
  Write-Host 'Verify the installation:'
  Write-Host '  pvm version'
  Write-Host '  pvm doctor'
  Write-Host ''
} finally {
  if ($stagePath) {
    Remove-Item -LiteralPath $stagePath -Force -ErrorAction SilentlyContinue
  }
  Remove-Item -LiteralPath $checksumPath -Force -ErrorAction SilentlyContinue
}
