@echo off
setlocal enabledelayedexpansion

echo ========================================
echo   Replace Local Files from Remote
echo ========================================
echo.

REM Parse arguments
set MODE=backup
if "%~1"=="--no-backup" set MODE=no-backup
if "%~1"=="--backup" set MODE=backup
if "%~1"=="--list" set MODE=list

REM Check if inside a git repo
git rev-parse --is-inside-work-tree >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Not a Git repository
    exit /b 1
)

REM Fetch latest remote info
echo [SYNC] Fetching latest...
git fetch origin >nul 2>&1
if errorlevel 1 echo [WARN] Unable to fetch, using cached info

echo.
echo [SCAN] Searching for local files...
echo.

REM Build file list
set "LIST_FILE=%CD%\._pvm_list.tmp"
git ls-files > "%LIST_FILE%" 2>nul

REM Filter and process
set COUNT=0
if exist "%LIST_FILE%" (
    for /f "usebackq tokens=*" %%f in ("%LIST_FILE%") do (
        set "name=%%f"
        echo !name! | findstr /i "local" >nul
        if !errorlevel! equ 0 (
            set /a COUNT+=1
            echo [!COUNT!] !name!
            if not "!MODE!"=="list" call :do_replace "!name!"
        )
    )
    del "%LIST_FILE%" >nul 2>&1
)

echo.
echo ========================================
echo   Results
echo ========================================
echo Local files found: !COUNT!

if "!MODE!"=="list" goto :show_list
if !COUNT! gtr 0 goto :show_done
goto :show_empty

:show_list
echo [DONE] Listed only
goto :end

:show_done
echo [DONE] Replaced !COUNT! file(s)
goto :end

:show_empty
echo [DONE] No local files found
goto :end

:end
exit /b 0

REM ============================================================================
REM Replace a single file
REM ============================================================================
:do_replace
set "target=%~1"
if not exist "!target!" (
    echo [SKIP] !target! - not found
    exit /b 0
)

REM Fetch from remote
git checkout origin/HEAD -- "!target!" >nul 2>&1
if !errorlevel! equ 0 (
    echo [REPLACE] !target! - done
    exit /b 0
)
for /f "usebackq tokens=*" %%b in (`git rev-parse --abbrev-ref HEAD`) do set "br=%%b"
git checkout origin/!br! -- "!target!" >nul 2>&1
if !errorlevel! equ 0 (
    echo [REPLACE] !target! - done
    exit /b 0
)
echo [FAIL] !target! - unable to fetch
exit /b 0
