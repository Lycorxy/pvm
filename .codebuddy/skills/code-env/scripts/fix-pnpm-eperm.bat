@echo off
chcp 65001 >nul 2>&1
REM ============================================================================
REM PVM Env Manager - pnpm EPERM Permission Error Fix Script (Windows)
REM ============================================================================
REM Fix: ERR_PNPM_EPERM / EPERM unlink errors during pnpm install
REM Cause: native binaries (esbuild.exe/rollup.exe/swc.exe) locked by processes
REM
REM IMPORTANT: Most EPERM errors are caused by IDE lacking admin privileges
REM            during "pvm install pnpm". Try running IDE as Administrator FIRST.
REM            This script only handles the "process locking" scenario.
REM
REM Usage: fix-pnpm-eperm.bat [--reinstall|--kill-only|--check]
REM   --reinstall  : kill procs + rm node_modules + pnpm install (default)
REM   --kill-only  : kill locked procs only, no reinstall
REM   --check      : detect locked procs only, no action
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   pnpm EPERM Permission Error Fix
echo ========================================
echo.
echo [TIP] Most EPERM = IDE lacks admin privileges.
echo       Try: Right-click IDE -^> Run as Administrator -^> pvm install pnpm
echo       This script only handles process-locking EPERM.
echo.

REM Parse args
set MODE=reinstall
if "%~1"=="--reinstall" set MODE=reinstall
if "%~1"=="--kill-only" set MODE=kill-only
if "%~1"=="--check" set MODE=check

REM Check project dir
if not exist "package.json" (
    echo [ERROR] No package.json in current dir. Run in project root.
    exit /b 1
)

echo [Mode] !MODE!
echo.

REM ============================================================
REM Step 1: Detect locking processes
REM ============================================================
echo [1/4] Scanning for processes locking node_modules...

set KILL_COUNT=0

REM Common processes that lock node_modules binaries
for %%p in (esbuild.exe rollup.exe swc.exe vite.exe webpack.exe) do (
    for /f "tokens=2 delims=," %%a in ('tasklist /FI "IMAGENAME eq %%p" /NH /FO CSV 2^>nul ^| findstr /i "%%p"') do (
        echo   [FOUND] %%p ^(PID: %%~a^)
        set /a KILL_COUNT+=1
    )
)

if !KILL_COUNT!==0 (
    echo   [OK] No locking processes found
) else (
    echo   Found !KILL_COUNT! locking process^(es^)
)
echo.

REM check mode: detect only
if "!MODE!"=="check" (
    echo [DONE] Check-only mode, no fix applied
    if !KILL_COUNT! gtr 0 (
        echo [TIP] Run: fix-pnpm-eperm.bat --kill-only   (kill procs only^)
        echo       Run: fix-pnpm-eperm.bat --reinstall   (full fix^)
    )
    exit /b 0
)

REM ============================================================
REM Step 2: Kill locking processes
REM ============================================================
if !KILL_COUNT! gtr 0 (
    echo [2/4] Terminating locking processes...
    for %%p in (esbuild.exe rollup.exe swc.exe vite.exe webpack.exe) do (
        taskkill /F /IM %%p >nul 2>&1 && echo   [KILLED] %%p
    )
    REM Wait for file handles to release (ping is more reliable than timeout in non-interactive shells)
    ping -n 3 127.0.0.1 >nul 2>&1
) else (
    echo [2/4] No procs to kill
)
echo.

REM kill-only mode
if "!MODE!"=="kill-only" (
    echo [DONE] Locking processes terminated. Re-run: pnpm install
    exit /b 0
)

REM ============================================================
REM Step 3: Clean node_modules
REM ============================================================
echo [3/4] Cleaning node_modules...

if exist "node_modules" (
    rd /s /q "node_modules" >nul 2>&1
    if exist "node_modules" (
        echo   [RETRY] Normal delete failed, forcing...
        attrib -r -h -s "node_modules\*" /s /d >nul 2>&1
        rd /s /q "node_modules" >nul 2>&1
    )
    if exist "node_modules" (
        echo   [WARN] node_modules still cannot be deleted
        echo   [TIP] Close your IDE / antivirus, then retry
        echo         Or manually delete: rmdir /s /q node_modules
        exit /b 1
    )
    echo   [OK] node_modules deleted
) else (
    echo   [SKIP] node_modules does not exist
)

REM Clean pnpm store cache (optional)
if exist ".pnpm-store" (
    rd /s /q ".pnpm-store" >nul 2>&1
    echo   [OK] .pnpm-store cleaned
)
echo.

REM ============================================================
REM Step 4: Reinstall dependencies
REM ============================================================
echo [4/4] Reinstalling dependencies...
echo.

where pnpm >nul 2>&1
if !errorlevel!==0 (
    echo   [RUN] pnpm install
    call pnpm install
    if !errorlevel!==0 (
        echo.
        echo ========================================
        echo   Fix Complete
        echo ========================================
        echo Dependencies reinstalled. EPERM error should be resolved.
    ) else (
        echo.
        echo [WARN] pnpm install still fails. Possible causes:
        echo   1. Antivirus blocking - add project dir to exclusions
        echo   2. IDE index service locking - fully close IDE and retry
        echo   3. Insufficient permission - run terminal as Administrator
        echo   4. Disk error - run chkdsk to check disk
        exit /b 1
    )
) else (
    echo [ERROR] pnpm not available. Install first: pvm install pnpm@latest
    exit /b 1
)

exit /b 0
