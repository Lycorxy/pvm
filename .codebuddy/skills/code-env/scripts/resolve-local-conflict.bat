@echo off
REM ============================================================================
REM PVM Env Manager - Local file merge conflict resolver (Windows)
REM ============================================================================
REM Resolve git merge conflicts for files matching "local" in filename
REM Usage: resolve-local-conflict.bat [--local|--remote|--auto]
REM   --local  : keep local version
REM   --remote : take remote version
REM   --auto   : auto-select (default: take remote)
REM ============================================================================

chcp 65001 >nul 2>&1

setlocal enabledelayedexpansion

echo ========================================
echo   Local File Merge Conflict Resolver
echo ========================================
echo.

REM Parse arguments
set MODE=auto
if "%~1"=="--local" set MODE=local
if "%~1"=="--remote" set MODE=remote
if "%~1"=="--auto" set MODE=auto

REM Check if inside a git repo
git rev-parse --is-inside-work-tree >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Not a Git repository
    exit /b 1
)

echo [SCAN] Detecting merge conflicts...
echo.

REM Collect conflicting files (use temp file to avoid pipe issues)
set "TMP_FILE=%CD%\._pvm_conflict_files.txt"
git diff --name-only --diff-filter=U 2>nul > "%TMP_FILE%"
set CONFLICT_COUNT=0
set LOCAL_CONFLICT_COUNT=0
if exist "%TMP_FILE%" (
    for /f "usebackq tokens=*" %%f in ("%TMP_FILE%") do (
        set /a CONFLICT_COUNT+=1
        set "FILE=%%f"

        REM Check if filename contains "local" (case-insensitive)
        echo !FILE! | findstr /i /c:"local" >nul
        if !errorlevel!==0 (
            set /a LOCAL_CONFLICT_COUNT+=1
            echo [FOUND] !FILE!
            call :resolve_conflict "!FILE!"
        )
    )
    del "%TMP_FILE%" 2>nul
)
if !CONFLICT_COUNT!==0 echo [INFO] No merge conflicts found

echo.
echo ========================================
echo   Results
echo ========================================
echo Total conflicts: !CONFLICT_COUNT!
echo Local file conflicts: !LOCAL_CONFLICT_COUNT!
echo.

if !LOCAL_CONFLICT_COUNT!==0 (
    echo [DONE] No local file conflicts found
) else (
    echo [DONE] Resolved !LOCAL_CONFLICT_COUNT! local file conflict(s)
)

exit /b 0

REM ============================================================================
REM Resolve a single file conflict
REM ============================================================================
:resolve_conflict
set "FILE=%~1"

if "%MODE%"=="local" (
    echo [ACTION] !FILE! -^> keep local version
    git checkout --ours "!FILE!"
    git add "!FILE!"
) else if "%MODE%"=="remote" (
    echo [ACTION] !FILE! -^> take remote version
    git checkout --theirs "!FILE!"
    git add "!FILE!"
) else (
    REM auto mode: default to remote
    echo [ACTION] !FILE! -^> auto (take remote)
    git checkout --theirs "!FILE!"
    git add "!FILE!"
)

exit /b 0
