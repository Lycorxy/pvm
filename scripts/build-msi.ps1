# PVM MSI 打包脚本（自带 WiX Toolset，无需安装）
# 使用说明：
#   .\scripts\build-msi.ps1                    # 打包 amd64 架构
#   .\scripts\build-msi.ps1 -Arch arm64        # 打包 arm64 架构
#   .\scripts\build-msi.ps1 -Version 1.2.3     # 指定版本号
#
# 注意：首次使用需要将 WiX 工具放到 scripts\wix 目录
#       下载地址：https://github.com/wixtoolset/wix3/releases/download/wix314rtm/wix314-binaries.zip
#       解压后将所有文件放到 scripts\wix 目录

param(
    [string]$Version = "",
    [string]$Arch = "amd64"
)

# 自动从 cmd/root.go 读取版本号
if (-not $Version) {
    $rootGo = Get-Content (Join-Path (Split-Path -Parent $PSScriptRoot) "cmd/root.go") -Raw -ErrorAction SilentlyContinue
    if ($rootGo -match 'Version\s*=\s*"([^"]+)"') {
        $Version = $Matches[1]
    } else {
        $Version = "0.0.0"
    }
}

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not $ProjectRoot) { $ProjectRoot = (Get-Location).Path -replace "\\scripts$", "" }

Write-Host "=== PVM MSI Builder ===" -ForegroundColor Cyan
Write-Host "Version: $Version"
Write-Host "Architecture: $Arch"

# 检查 WiX Toolset
Write-Host "`n[1/4] Checking WiX Toolset..." -ForegroundColor Yellow

$wixBundledDir = Join-Path $PSScriptRoot "wix"
$wixPath = $null

# 函数：查找 WiX（优先级：项目自带 > 系统安装 > 环境变量）
function Find-WiX {
    # 1. 优先使用项目自带的 WiX（scripts\wix 目录）
    if (Test-Path "$wixBundledDir\candle.exe") {
        return $wixBundledDir
    }
    # 2. 检查环境变量 WIX
    if ($env:WIX -and (Test-Path "$env:WIX\bin\candle.exe")) {
        return "$env:WIX\bin"
    }
    # 3. 检查常见安装路径
    $wixDirs = @(
        "${env:ProgramFiles(x86)}\WiX Toolset*\bin",
        "${env:ProgramFiles}\WiX Toolset*\bin"
    )
    foreach ($pattern in $wixDirs) {
        $dirs = Get-ChildItem $pattern -ErrorAction SilentlyContinue
        foreach ($d in $dirs) {
            if (Test-Path "$($d.FullName)\candle.exe") {
                return $d.FullName
            }
        }
    }
    return $null
}

# 查找 WiX
$wixPath = Find-WiX

if (-not $wixPath) {
    Write-Host "ERROR: WiX Toolset not found!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please download WiX and extract to: $wixBundledDir" -ForegroundColor Yellow
    Write-Host "Download URL: https://github.com/wixtoolset/wix3/releases/download/wix314rtm/wix314-binaries.zip" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Steps:" -ForegroundColor Yellow
    Write-Host "  1. Download wix314-binaries.zip" -ForegroundColor Yellow
    Write-Host "  2. Extract all files to: $wixBundledDir" -ForegroundColor Yellow
    Write-Host "  3. Run this script again" -ForegroundColor Yellow
    exit 1
}

Write-Host "  WiX Toolset: $wixPath" -ForegroundColor Green

# 准备文件
Write-Host "`n[2/4] Preparing files..." -ForegroundColor Yellow

$distDir = Join-Path $ProjectRoot "dist"
$msiDir = Join-Path $distDir "msi"
$srcExe = Join-Path $distDir "pvm-windows-$Arch.exe"

if (-not (Test-Path $srcExe)) {
    Write-Host "ERROR: $srcExe not found! Run build.ps1 first." -ForegroundColor Red
    exit 1
}

# 创建 MSI 工作目录
New-Item -ItemType Directory -Path $msiDir -Force | Out-Null
Copy-Item $srcExe (Join-Path $msiDir "pvm.exe") -Force
Copy-Item (Join-Path $ProjectRoot "LICENSE") $msiDir -Force -ErrorAction SilentlyContinue

# 复制 pvm-shim.exe（统一 shim 方案：命令转发器，被复制为各命令名 node/git/go...）
$shimExeSrc = Join-Path $distDir "pvm-shim.exe"
if (-not (Test-Path $shimExeSrc)) {
    # 尝试带平台后缀的命名（build-all.ps1 产物）
    $shimExeSrc = Join-Path $distDir "pvm-shim-windows-amd64.exe"
}
if (-not (Test-Path $shimExeSrc)) {
    # 尝试直接编译
    Write-Host "  Building pvm-shim.exe from source..." -ForegroundColor Gray
    $shimBuild = go build -ldflags "-s -w" -o (Join-Path $msiDir "pvm-shim.exe") ./cmd/shim 2>&1
    if ($LASTEXITCODE -eq 0) {
        $shimExeSrc = $null  # already in target location, no need to copy
        Write-Host "  pvm-shim.exe built" -ForegroundColor Green
    } else {
        Write-Host "  pvm-shim build failed: $shimBuild" -ForegroundColor DarkRed
        $shimExeSrc = $null
    }
}
if ($shimExeSrc -and (Test-Path $shimExeSrc)) {
    Copy-Item $shimExeSrc (Join-Path $msiDir "pvm-shim.exe") -Force
    Write-Host "  pvm-shim.exe included" -ForegroundColor Green
} elseif (Test-Path (Join-Path $msiDir "pvm-shim.exe")) {
    Write-Host "  pvm-shim.exe built" -ForegroundColor Green
} else {
    Write-Host "  Warning: pvm-shim.exe not found, shims will not work" -ForegroundColor Yellow
}

# 生成 WXS 文件
Write-Host "`n[3/4] Generating WiX source..." -ForegroundColor Yellow

$wxsContent = @"
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" 
           Name="PVM - Polyglot Version Manager" 
           Language="1033" 
           Version="$Version.0" 
           Manufacturer="lucky-zsh" 
           UpgradeCode="c5d7a8e2-4b3f-4a1e-9c8d-2e5f6a7b8c9d">
    
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perUser" />
    <!-- 手动定义升级规则，支持同版本重装（IncludeThisVersion="yes"）和降级安装 -->
    <Upgrade Id="c5d7a8e2-4b3f-4a1e-9c8d-2e5f6a7b8c9d">
      <!-- 匹配所有版本（含同版本），触发卸载旧版本 -->
      <UpgradeVersion Minimum="0.0.0" IncludeMinimum="yes"
                      OnlyDetect="no"
                      Property="PREVIOUSVERSIONSINSTALLED" />
    </Upgrade>
    <MediaTemplate EmbedCab="yes" />

    <Feature Id="ProductFeature" Title="PVM" Level="1">
      <ComponentGroupRef Id="ProductComponents" />
      <ComponentRef Id="PathEnvVar" />
      <ComponentRef Id="ShimsPathEnvVar" />
    </Feature>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="LocalAppDataFolder">
        <Directory Id="INSTALLDIR" Name="pvm" />
      </Directory>
    </Directory>

    <ComponentGroup Id="ProductComponents" Directory="INSTALLDIR">
      <Component Id="MainExecutable" Guid="a1b2c3d4-e5f6-7890-abcd-ef1234567890">
        <!-- perUser 安装时必须用注册表作为 KeyPath -->
        <RegistryValue Root="HKCU" Key="Software\pvm" Name="InstallDir" Value="[INSTALLDIR]" Type="string" KeyPath="yes" />
        <File Id="PvmExe" Source="pvm.exe" />
        <!-- 卸载时删除安装目录 -->
        <RemoveFolder Id="RemoveINSTALLDIR" Directory="INSTALLDIR" On="uninstall" />
      </Component>
      <Component Id="ShimExecutable" Guid="d4e5f6a7-b8c9-0123-def0-234567890123">
        <RegistryValue Root="HKCU" Key="Software\pvm" Name="ShimExe" Value="1" Type="string" KeyPath="yes" />
        <File Id="PvmShimExe" Source="pvm-shim.exe" />
      </Component>
    </ComponentGroup>

    <!-- 添加 INSTALLDIR 到 PATH（用于找到 pvm.exe）- 前置以确保优先级高于系统层已安装的 runtime -->
    <Component Id="PathEnvVar" Directory="INSTALLDIR" Guid="b2c3d4e5-f6a7-8901-bcde-f12345678901">
      <RegistryValue Root="HKCU" Key="Software\pvm" Name="PathSet" Value="1" Type="string" KeyPath="yes" />
      <Environment Id="PATH" Name="PATH" Value="[INSTALLDIR]" Permanent="no" Part="first" Action="set" System="no" />
    </Component>

    <!-- 添加 shims 目录到 PATH（用于版本切换）- 前置以确保最高优先级 -->
    <Component Id="ShimsPathEnvVar" Directory="INSTALLDIR" Guid="c3d4e5f6-a7b8-9012-cdef-123456789012">
      <RegistryValue Root="HKCU" Key="Software\pvm" Name="ShimsPathSet" Value="1" Type="string" KeyPath="yes" />
      <Environment Id="SHIMS_PATH" Name="PATH" Value="[INSTALLDIR]shims" Permanent="no" Part="first" Action="set" System="no" />
    </Component>

    <!-- 安装完成后自动运行 pvm setup（创建 ~/.pvm 目录结构并配置 PATH） -->
    <CustomAction Id="RunPvmSetup"
                  FileKey="PvmExe"
                  ExeCommand="setup --yes"
                  Execute="deferred"
                  Impersonate="yes"
                  Return="ignore" />

    <InstallExecuteSequence>
      <!-- 先卸载旧版本（含同版本），再安装新版本 -->
      <RemoveExistingProducts After="InstallInitialize" />
      <Custom Action="RunPvmSetup" After="InstallFiles">NOT Installed OR REINSTALL</Custom>
    </InstallExecuteSequence>

    <UI>
      <UIRef Id="WixUI_Minimal" />
    </UI>
    <WixVariable Id="WixUILicenseRtf" Value="license.rtf" />
  </Product>
</Wix>
"@

$wxsFile = Join-Path $msiDir "pvm.wxs"
$wxsContent | Out-File -FilePath $wxsFile -Encoding UTF8

# 创建 RTF 许可证文件
$licenseRtf = Join-Path $msiDir "license.rtf"
@"
{\rtf1\ansi\deff0
{\fonttbl{\f0 Consolas;}}
\f0\fs20
MIT License\par
\par
Copyright (c) 2026 lucky-zsh\par
\par
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:\par
\par
The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.\par
}
"@ | Out-File -FilePath $licenseRtf -Encoding ASCII

# 编译 MSI
Write-Host "`n[4/4] Building MSI..." -ForegroundColor Yellow

Push-Location $msiDir
try {
    # 根据架构设置 WiX 参数
    $wixArch = if ($Arch -eq "arm64") { "arm64" } else { "x64" }
    & "$wixPath\candle.exe" -arch $wixArch pvm.wxs -ext WixUIExtension
    if ($LASTEXITCODE -ne 0) { throw "candle.exe failed" }
    
    & "$wixPath\light.exe" -ext WixUIExtension pvm.wixobj -o "pvm-windows-$Arch.msi" -sice:ICE40 -sice:ICE61 -sice:ICE91
    if ($LASTEXITCODE -ne 0) { throw "light.exe failed" }
    
    # 移动到 dist 目录（先删除旧文件）
    $destMsi = Join-Path $distDir "pvm-windows-$Arch.msi"
    if (Test-Path $destMsi) {
        try {
            Remove-Item $destMsi -Force -ErrorAction Stop
        }
        catch {
            Write-Host "  Warning: Cannot delete old MSI (file in use), will use alternative name" -ForegroundColor Yellow
            $destMsi = Join-Path $distDir "pvm-windows-$Arch-new.msi"
        }
    }
    Move-Item "pvm-windows-$Arch.msi" $destMsi -Force
    
    Write-Host "`n=== SUCCESS ===" -ForegroundColor Green
    Write-Host "MSI created: dist\pvm-windows-$Arch.msi" -ForegroundColor Cyan
}
finally {
    Pop-Location
}
