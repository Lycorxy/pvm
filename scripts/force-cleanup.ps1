# Force cleanup PVM - Force remove all PVM residues
# Run as Administrator

$ErrorActionPreference = "SilentlyContinue"

Write-Host "=== PVM Force Cleanup Tool ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Kill all pvm related processes
Write-Host "[1/7] Killing all pvm related processes..." -ForegroundColor Yellow

# 定义需要终止的进程列表
$processNames = @(
    "pvm",
    "node",
    "npm",
    "npx",
    "pnpm",
    "yarn",
    "go",
    "python",
    "pythonw",
    "git",
    "git-bash",
    "bash",
    "sh",
    "ssh",
    "ssh-agent",
    "mingw32-make"
)

$killed = @()
foreach ($procName in $processNames) {
    $processes = Get-Process $procName -ErrorAction SilentlyContinue
    foreach ($proc in $processes) {
        # 只终止路径中包含 .pvm 的进程（避免误杀系统进程）
        if ($proc.Path -like "*pvm*" -or $procName -eq "pvm") {
            Write-Host "  Killing $procName (PID: $($proc.Id))" -ForegroundColor Gray
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            $killed += "$procName (PID: $($proc.Id))"
        }
    }
}

Start-Sleep -Milliseconds 500
if ($killed.Count -gt 0) {
    Write-Host "  [OK] Killed $($killed.Count) process(es)" -ForegroundColor Green
} else {
    Write-Host "  [INFO] No pvm-related processes found" -ForegroundColor Gray
}

# Step 2: Also try to find processes that have loaded .pvm DLLs
Write-Host "[2/7] Checking for processes using .pvm files..." -ForegroundColor Yellow
$processesWithPvm = Get-Process | Where-Object {
    try {
        $_.Modules | Where-Object { $_.FileName -like "*pvm*" }
    } catch {}
}
if ($processesWithPvm) {
    foreach ($proc in $processesWithPvm) {
        Write-Host "  Killing $($proc.ProcessName) (PID: $($proc.Id)) - using .pvm files" -ForegroundColor Gray
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    Write-Host "  [OK] Killed processes using .pvm files" -ForegroundColor Green
} else {
    Write-Host "  [INFO] No additional processes found" -ForegroundColor Gray
}

# Step 3: Delete .pvm directory
Write-Host "[3/7] Deleting .pvm directories..." -ForegroundColor Yellow
$pvmDirs = @(
    "$env:USERPROFILE\.pvm",
    "$env:LOCALAPPDATA\pvm"
)
foreach ($pvmDir in $pvmDirs) {
    if (Test-Path $pvmDir) {
        Write-Host "  Deleting: $pvmDir" -ForegroundColor Gray
        # Try robust deletion (handles locked files)
        $lockedFiles = @()
        Get-ChildItem -Path $pvmDir -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object {
            try {
                Remove-Item -Path $_.FullName -Force -Recurse -ErrorAction Stop
            } catch {
                $lockedFiles += $_.FullName
            }
        }
        # Try remove root dir
        try {
            Remove-Item -Path $pvmDir -Force -Recurse -ErrorAction Stop
            Write-Host "  [OK] Deleted: $pvmDir" -ForegroundColor Green
        } catch {
            Write-Host "  [WARN] Cannot delete (files in use): $pvmDir" -ForegroundColor Yellow
            if ($lockedFiles.Count -gt 0) {
                Write-Host "    Locked files:" -ForegroundColor Gray
                $lockedFiles | Select-Object -First 5 | ForEach-Object { Write-Host "      $_" -ForegroundColor DarkGray }
            }
            # Schedule reboot delete
            $pvmDirEsc = $pvmDir -replace "'", "''"
            Start-Process "powershell" -ArgumentList "-NoProfile -WindowStyle Hidden -Command `"Remove-Item -Path '$pvmDirEsc' -Force -Recurse -ErrorAction SilentlyContinue`"" -WindowStyle Hidden
        }
    } else {
        Write-Host "  [INFO] Not found: $pvmDir" -ForegroundColor Gray
    }
}

# Step 4: Clean PATH environment variables
Write-Host "[4/7] Cleaning PATH environment variables..." -ForegroundColor Yellow
$scopes = @('User', 'Machine')
foreach ($scope in $scopes) {
    try {
        $path = [Environment]::GetEnvironmentVariable('PATH', $scope)
        if ($path -and $path -like '*pvm*') {
            $newPath = ($path -split ';' | Where-Object { $_ -notlike '*pvm*' -and $_ -ne '' }) -join ';'
            [Environment]::SetEnvironmentVariable('PATH', $newPath, $scope)
            Write-Host "  [OK] Cleaned PVM paths from $scope PATH" -ForegroundColor Green
        } else {
            Write-Host "  [INFO] No PVM residues in $scope PATH" -ForegroundColor Gray
        }
    } catch {
        Write-Host "  [WARN] Cannot clean $scope PATH (may need admin): $_" -ForegroundColor Yellow
    }
}

# Step 5: Clean registry residues
Write-Host "[5/7] Cleaning registry residues..." -ForegroundColor Yellow
$regCleanups = @(
    'HKCU:\Software\pvm',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\App Paths\pvm.exe',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\pvm'
)
foreach ($regPath in $regCleanups) {
    if (Test-Path $regPath) {
        Remove-Item -Path $regPath -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "  [OK] Removed: $regPath" -ForegroundColor Green
    }
}
# Clean Run/RunOnce entries
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
if (Test-Path $runKey) {
    $runValues = Get-ItemProperty $runKey -ErrorAction SilentlyContinue
    $runValues.PSObject.Properties | Where-Object { $_.Name -like '*pvm*' } | ForEach-Object {
        Remove-ItemProperty $runKey -Name $_.Name -ErrorAction SilentlyContinue
        Write-Host "  [OK] Removed Run entry: $($_.Name)" -ForegroundColor Green
    }
}
Write-Host "  [OK] Registry cleanup complete" -ForegroundColor Green

# Step 6: Remove Start Menu shortcuts
Write-Host "[6/7] Removing Start Menu shortcuts..." -ForegroundColor Yellow
$startMenuDirs = @(
    "$env:APPDATA\Microsoft\Windows\Start Menu\Programs",
    "$env:ProgramData\Microsoft\Windows\Start Menu\Programs"
)
$shortcutFound = $false
foreach ($dir in $startMenuDirs) {
    if (Test-Path $dir) {
        Get-ChildItem -Path $dir -Recurse -Filter "*pvm*" -Include "*.lnk", "*.url" -ErrorAction SilentlyContinue | ForEach-Object {
            Remove-Item $_.FullName -Force -ErrorAction SilentlyContinue
            Write-Host "  [OK] Removed: $($_.Name)" -ForegroundColor Green
            $shortcutFound = $true
        }
    }
}
if (-not $shortcutFound) {
    Write-Host "  [INFO] No Start Menu shortcuts found" -ForegroundColor Gray
}

# Step 7: Check for MSI installation records
Write-Host "[7/7] Checking for MSI installation records..." -ForegroundColor Yellow
$regPaths = @(
    'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
$msiFound = $false
foreach ($regPath in $regPaths) {
    $items = Get-ItemProperty $regPath -ErrorAction SilentlyContinue | Where-Object { $_.DisplayName -like '*pvm*' -or $_.Publisher -like '*pvm*' -or $_.InstallLocation -like '*pvm*' }
    if ($items) {
        $msiFound = $true
        foreach ($item in $items) {
            Write-Host "  [WARN] Found MSI record: $($item.DisplayName)" -ForegroundColor Yellow
            if ($item.UninstallString) {
                Write-Host "         Uninstall: $($item.UninstallString)" -ForegroundColor Gray
                # Try to run uninstall silently
                if ($item.QuietUninstallString) {
                    Write-Host "         Running quiet uninstall..." -ForegroundColor Gray
                    Start-Process "cmd.exe" -ArgumentList "/c $($item.QuietUninstallString)" -WindowStyle Hidden -Wait
                }
            }
        }
    }
}
if (-not $msiFound) {
    Write-Host "  [OK] No MSI installation records found" -ForegroundColor Green
}

Write-Host ""
Write-Host "=== Cleanup Complete ===" -ForegroundColor Cyan
Write-Host "Please:" -ForegroundColor Yellow
Write-Host "  1. Close and reopen your terminal" -ForegroundColor Yellow
Write-Host "  2. Restart your computer if files could not be deleted" -ForegroundColor Yellow
