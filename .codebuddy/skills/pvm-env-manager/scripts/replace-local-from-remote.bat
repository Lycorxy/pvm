@echo off
REM ============================================================================
REM PVM 环境管理器 - 从远程替换本地 local 文件脚本 (Windows)
REM ============================================================================
REM 功能：从远程仓库强制替换本地 local 文件
REM 用法：replace-local-from-remote.bat [--backup|--no-backup|--list]
REM   --backup    : 替换前备份原有文件（默认）
REM   --no-backup : 不备份直接替换
REM   --list      : 仅列出 local 文件，不替换
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   从远程替换本地 local 文件
echo ========================================
echo.

REM 参数解析
set MODE=backup
if "%~1"=="--no-backup" set MODE=no-backup
if "%~1"=="--backup" set MODE=backup
if "%~1"=="--list" set MODE=list

REM 检查是否在 git 仓库中
git rev-parse --is-inside-work-tree >nul 2>&1
if errorlevel 1 (
    echo [错误] 当前目录不是 Git 仓库
    exit /b 1
)

REM 确保远程信息是最新的
echo [同步] 正在从远程获取最新信息...
git fetch origin 2>nul
if errorlevel 1 (
    echo [警告] 无法从远程获取信息，将使用本地缓存的远程信息
)

echo.
echo [扫描] 正在查找 local 文件...
echo.

REM 查找所有包含 local 的文件
set LOCAL_COUNT=0
set BACKUP_DIR=.local-backup-%date:~0,4%%date:~5,2%%date:~8,2%_%time:~0,2%%time:~3,2%
set BACKUP_DIR=%BACKUP_DIR: =0%

for /f "tokens=*" %%f in ('git ls-files ^| findstr /i "local"') do (
    set /a LOCAL_COUNT+=1
    set "FILE=%%f"
    echo [!LOCAL_COUNT!] !FILE!
    
    if "!MODE!"=="list" (
        REM 仅列出，不操作
    ) else (
        call :replace_file "!FILE!"
    )
)

echo.
echo ========================================
echo   处理结果
echo ========================================
echo 找到 local 文件数: !LOCAL_COUNT!

if "!MODE!"=="list" (
    echo [完成] 仅列出文件，未执行替换
) else if !LOCAL_COUNT! gtr 0 (
    echo [完成] 已从远程替换 !LOCAL_COUNT! 个文件
    if "!MODE!"=="backup" (
        echo [备份] 备份文件位于: !BACKUP_DIR!
    )
) else (
    echo [完成] 未找到 local 文件
)

exit /b 0

REM ============================================================================
REM 替换单个文件
REM ============================================================================
:replace_file
set "FILE=%~1"

REM 检查文件是否存在
if not exist "!FILE!" (
    echo [跳过] !FILE! - 文件不存在
    exit /b 0
)

REM 备份原有文件
if "%MODE%"=="backup" (
    if not exist "!BACKUP_DIR!" mkdir "!BACKUP_DIR!"
    
    REM 保持目录结构
    set "BACKUP_PATH=!BACKUP_DIR!\!FILE!"
    for %%d in ("!BACKUP_PATH!") do set "BACKUP_DIR_PATH=%%~pd"
    if not exist "!BACKUP_DIR_PATH!" mkdir "!BACKUP_DIR_PATH!"
    
    copy "!FILE!" "!BACKUP_PATH!" >nul 2>&1
    if !errorlevel!==0 (
        echo [备份] !FILE! -^> !BACKUP_PATH!
    ) else (
        echo [警告] 备份失败: !FILE!
    )
)

REM 从远程获取文件
git checkout origin/HEAD -- "!FILE!" >nul 2>&1
if !errorlevel!==0 (
    echo [替换] !FILE! - 完成
) else (
    REM 尝试从当前分支的远程获取
    for /f "tokens=*" %%b in ('git rev-parse --abbrev-ref HEAD') do set BRANCH=%%b
    git checkout origin/!BRANCH! -- "!FILE!" >nul 2>&1
    if !errorlevel!==0 (
        echo [替换] !FILE! - 完成
    ) else (
        echo [失败] !FILE! - 无法从远程获取
    )
)

exit /b 0