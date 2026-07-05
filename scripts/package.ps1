# PVM 打包脚本
param(
    [string]$Version = "0.0.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$DistDir = Join-Path $ProjectRoot "dist"
$PackagesDir = Join-Path $DistDir "packages"
$Version = $Version -replace '^v', ''

Write-Host "=== PVM Packager ===" -ForegroundColor Cyan
Write-Host "Version: $Version" -ForegroundColor Yellow

New-Item -ItemType Directory -Path $PackagesDir -Force | Out-Null

$packages = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe"; Compress = "zip" },
    @{ OS = "windows"; Arch = "arm64"; Ext = ".exe"; Compress = "zip" },
    @{ OS = "darwin";  Arch = "amd64"; Ext = "";     Compress = "zip" },
    @{ OS = "darwin";  Arch = "arm64"; Ext = "";     Compress = "zip" },
    @{ OS = "linux";   Arch = "amd64"; Ext = "";     Compress = "zip" },
    @{ OS = "linux";   Arch = "arm64"; Ext = "";     Compress = "zip" }
)

foreach ($pkg in $packages) {
    $os = $pkg.OS; $arch = $pkg.Arch; $ext = $pkg.Ext
    
    $exeName = "pvm-$os-$arch$ext"
    $exePath = Join-Path $DistDir $exeName
    if (-not (Test-Path $exePath)) { continue }
    
    Write-Host "  Packaging $os/$arch..." -ForegroundColor Yellow
    
    $tmpDir = Join-Path $PackagesDir "pvm-$os-$arch"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    
    $destName = if ($ext -eq ".exe") { "pvm.exe" } else { "pvm" }
    Copy-Item $exePath (Join-Path $tmpDir $destName) -Force
    
    foreach ($f in @("LICENSE", "README.md")) {
        $src = Join-Path $ProjectRoot $f
        if (Test-Path $src) { Copy-Item $src $tmpDir -Force }
    }
    
    $outFile = "pvm-v$Version-$os-$arch.zip"
    $outPath = Join-Path $PackagesDir $outFile
    Compress-Archive -Path "$tmpDir\*" -DestinationPath $outPath -Force
    Remove-Item $tmpDir -Recurse -Force
    Write-Host "    ✓ $outFile ($([math]::Round((Get-Item $outPath).Length/1KB, 1)) KB)" -ForegroundColor Green
}

# 校验和
Write-Host "`n  Updating checksums..." -ForegroundColor Yellow
Get-ChildItem $PackagesDir -File | ForEach-Object {
    $h = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
    Write-Host "    $h  $($_.Name)"
}
Write-Host "`n=== Done ===" -ForegroundColor Cyan
Get-ChildItem $PackagesDir | Format-Table Name, Length -AutoSize
