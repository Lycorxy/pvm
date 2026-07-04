# pvm cross-platform build script (Windows PowerShell)
# Usage: .\scripts\build.ps1 [-Version x.x.x] [-CurrentOnly]

param(
    [string]$Version = "",
    [switch]$CurrentOnly
)

$ErrorActionPreference = "Stop"

# Auto-detect version from cmd/root.go
if (-not $Version) {
    $rootGo = Get-Content "cmd/root.go" -Raw
    if ($rootGo -match 'Version\s*=\s*"([^"]+)"') {
        $Version = $Matches[1]
    } else {
        $Version = "dev"
    }
}

$LDFLAGS = "-s -w -X github.com/pvm/pvm/cmd.Version=$Version"
$OUTPUT_DIR = "dist"

# Generate Windows resource .syso (manifest) to reduce AV false positives
# This embeds Windows compatibility info and asInvoker (no admin) declaration
if ($env:GOOS -eq "windows" -or $env:GOOS -eq "" -or $CurrentOnly) {
    Write-Host "Generating Windows resource info (.syso)..." -ForegroundColor DarkGray
    go run scripts/gen-syso.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [WARNING] Failed to generate .syso, continuing without" -ForegroundColor DarkYellow
    }
}

# Clean old build artifacts
if (Test-Path $OUTPUT_DIR) {
    Remove-Item -Recurse -Force $OUTPUT_DIR
}
New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  pvm v$Version cross-platform build" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

if ($CurrentOnly) {
    $ext = ""
    if ($env:GOOS -eq "windows" -or [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
        $ext = ".exe"
    }
    $output = "$OUTPUT_DIR/pvm$ext"
    Write-Host "[1/1] Building current platform..." -ForegroundColor Yellow
    go build -ldflags $LDFLAGS -o $output .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  -> FAILED!" -ForegroundColor Red
        exit 1
    }
    Write-Host "  -> $output" -ForegroundColor Green
} else {
    $platforms = @(
        @{OS="windows"; Arch="amd64"; Ext=".exe"},
        @{OS="windows"; Arch="arm64"; Ext=".exe"},
        @{OS="darwin";  Arch="amd64"; Ext=""},
        @{OS="darwin";  Arch="arm64"; Ext=""},
        @{OS="linux";   Arch="amd64"; Ext=""},
        @{OS="linux";   Arch="arm64"; Ext=""}
    )

    $total = $platforms.Count
    $success = 0
    $failed = 0

    foreach ($p in $platforms) {
        $i = $platforms.IndexOf($p) + 1
        $name = "$($p.OS)-$($p.Arch)"
        $output = "$OUTPUT_DIR/pvm-$name$($p.Ext)"

        Write-Host "[$i/$total] Building $name..." -ForegroundColor Yellow

        $env:GOOS = $p.OS
        $env:GOARCH = $p.Arch
        go build -ldflags $LDFLAGS -o $output .

        if ($LASTEXITCODE -ne 0) {
            Write-Host "  -> FAILED!" -ForegroundColor Red
            $failed++
        } else {
            $size = (Get-Item $output).Length
            $sizeMB = [math]::Round($size / 1MB, 2)
            Write-Host "  -> $output ($sizeMB MB)" -ForegroundColor Green
            $success++
        }
    }

    # Cleanup env vars
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

    # Cleanup generated .syso files (keep pre-built ones)
    Remove-Item "resource.syso" -ErrorAction SilentlyContinue
    Remove-Item "rsrc_windows_amd64.syso" -ErrorAction SilentlyContinue
    Remove-Item "rsrc_windows_386.syso" -ErrorAction SilentlyContinue
    Remove-Item "rsrc_windows_arm64.syso" -ErrorAction SilentlyContinue

    # Copy shim.exe to dist (for MSI packaging)
    $shimExeSrc = "internal\shim\assets\shim.exe"
    if (Test-Path $shimExeSrc) {
        Copy-Item $shimExeSrc "$OUTPUT_DIR\shim.exe" -Force
        Write-Host "  -> shim.exe copied to dist/" -ForegroundColor Green
    }

    # Generate SHA256 checksums
    Write-Host ""
    Write-Host "Generating SHA256 checksums..." -ForegroundColor Yellow
    $hashes = @()
    foreach ($p in $platforms) {
        $name = "$($p.OS)-$($p.Arch)"
        $output = "$OUTPUT_DIR/pvm-$name$($p.Ext)"
        if (Test-Path $output) {
            $hash = (Get-FileHash -Path $output -Algorithm SHA256).Hash.ToLower()
            $hashes += "$hash  pvm-$name$($p.Ext)"
        }
    }
    $hashes | Set-Content "$OUTPUT_DIR/checksums.txt" -Encoding UTF8
    Write-Host "  -> $OUTPUT_DIR/checksums.txt" -ForegroundColor Green

    Write-Host ""
    $color = "Green"
    if ($failed -gt 0) { $color = "Red" }
    Write-Host "======================================" -ForegroundColor Cyan
    Write-Host "  Build done: $success ok, $failed failed" -ForegroundColor $color
    Write-Host "======================================" -ForegroundColor Cyan
}

Write-Host ""
