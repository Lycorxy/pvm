@echo off
REM ============================================================================
REM PVM 环境管理器 - local 文件合并冲突处理脚本 (Windows)
REM ============================================================================
REM 功能：自动处理 git 合并时产生的 local 文件冲突
REM 用法：resolve-local-conflict.bat [--local|--remote|--auto]
REM   --local  : 保留本地版本
REM   --remote : 使用远程版本
REM   --auto   : 自动选择（默认：保留远程）
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   Local 文件合并冲突处理工具
echo ========================================
echo.

REM 参数解析
set MODE=auto
if "%~1"=="--local" set MODE=local
if "%~1"=="--remote" set MODE=remote
if "%~1"=="--auto" set MODE=auto

REM 检查是否在 git 仓库中
git rev-parse --is-inside-work-tree >nul 2>&1
if errorlevel 1 (
    echo [错误] 当前目录不是 Git 仓库
    exit /b 1
)

REM 检查是否有冲突
git diff --name-only --diff-filter=U >nul 2>&1
if errorlevel 1 (
    echo [提示] 没有发现合并冲突
    exit /b 0
)

echo [扫描] 正在检测合并冲突...
echo.

REM 获取所有冲突文件
set CONFLICT_COUNT=0
set LOCAL_CONFLICT_COUNT=0
for /f "tokens=*" %%f in ('git diff --name-only --diff-filter=U') do (
    set /a CONFLICT_COUNT+=1
    set "FILE=%%f"
    
    REM 检查是否是 local 文件
    echo !FILE! | findstr /i /c:"local" >nul
    if !errorlevel!==0 (
        set /a LOCAL_CONFLICT_COUNT+=1
        echo [发现] !FILE!
        call :resolve_conflict "!FILE!"
    )
)

echo.
echo ========================================
echo   处理结果
echo ========================================
echo 总冲突文件数: !CONFLICT_COUNT!
echo Local 文件冲突数: !LOCAL_CONFLICT_COUNT!
echo.

if !LOCAL_CONFLICT_COUNT!==0 (
    echo [完成] 未发现 local 文件冲突
) else (
    echo [完成] 已处理 !LOCAL_CONFLICT_COUNT! 个 local 文件冲突
)

exit /b 0

REM ============================================================================
REM 解决单个文件冲突
REM ============================================================================
:resolve_conflict
set "FILE=%~1"

if "%MODE%"=="local" (
    echo [处理] !FILE! -^> 保留本地版本
    git checkout --ours "!FILE!"
    git add "!FILE!"
) else if "%MODE%"=="remote" (
    echo [处理] !FILE! -^> 使用远程版本
    git checkout --theirs "!FILE!"
    git add "!FILE!"
) else (
    REM auto 模式：默认使用远程版本
    echo [处理] !FILE! -^> 自动使用远程版本
    git checkout --theirs "!FILE!"
    git add "!FILE!"
)

exit /b 0