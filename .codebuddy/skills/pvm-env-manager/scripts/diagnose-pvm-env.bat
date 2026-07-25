@echo off
REM ============================================================================
REM PVM 环境管理器 - 环境诊断脚本 (Windows)
REM ============================================================================
REM 功能：诊断 PVM 环境，检测冲突、环境变量问题、端口占用等
REM 用法：diagnose-pvm-env.bat [--fix]
REM   --fix : 自动修复发现的问题
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   PVM 环境诊断工具
echo ========================================
echo.

set FIX_MODE=0
if "%~1"=="--fix" set FIX_MODE=1

set PASS_COUNT=0
set FAIL_COUNT=0
set WARNING_COUNT=0

REM ============================================================================
REM 检查 1: PVM 安装状态
REM ============================================================================
echo [检查 1] PVM 安装状态
where.exe pvm >nul 2>&1
if !errorlevel!==0 (
    for /f "tokens=*" %%v in ('pvm --version 2^>nul') do set PVM_VERSION=%%v
    echo [√] PVM 已安装: !PVM_VERSION!
    set /a PASS_COUNT+=1
) else (
    echo [X] PVM 未安装
    set /a FAIL_COUNT+=1
)
echo.

REM ============================================================================
REM 检查 2: 环境变量
REM ============================================================================
echo [检查 2] 环境变量配置

REM 检查 PVM_HOME
set PVM_HOME=
for /f "tokens=*" %%h in ('powershell -Command "[Environment]::GetEnvironmentVariable('PVM_HOME', 'User')"') do set PVM_HOME=%%h

if defined PVM_HOME (
    echo [√] PVM_HOME = !PVM_HOME!
    set /a PASS_COUNT+=1
) else (
    echo [!] PVM_HOME 未设置（可选）
    set /a WARNING_COUNT+=1
)

REM 检查 PATH
set PATH_ISSUE=0
set USER_PATH=
for /f "tokens=*" %%p in ('powershell -Command "[Environment]::GetEnvironmentVariable('Path', 'User')"') do set USER_PATH=%%p

echo !USER_PATH! | findstr /i "\.pvm\\shims" >nul
if !errorlevel!==0 (
    echo [√] PATH 包含 .pvm\shims
    set /a PASS_COUNT+=1
) else (
    echo [X] PATH 缺少 .pvm\shims
    set /a FAIL_COUNT+=1
    set PATH_ISSUE=1
    
    if !FIX_MODE!==1 (
        echo [修复] 正在添加 .pvm\shims 到 PATH...
        powershell -Command ^
            "$path = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
             $newPath = $env:USERPROFILE + '\.pvm\shims;' + $path; ^
             [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')"
        echo [完成] PATH 已更新，请重启终端
    )
)

echo.

REM ============================================================================
REM 检查 3: 冲突工具
REM ============================================================================
echo [检查 3] 冲突工具检测

set CONFLICT_TOOLS=
for %%t in (nvm fnm pyenv rustup volta asdf) do (
    where.exe %%t >nul 2>&1
    if !errorlevel!==0 (
        echo [X] 检测到冲突工具: %%t
        set /a FAIL_COUNT+=1
        set CONFLICT_TOOLS=!CONFLICT_TOOLS! %%t
    )
)

if not defined CONFLICT_TOOLS (
    echo [√] 无冲突工具
    set /a PASS_COUNT+=1
)

if defined CONFLICT_TOOLS (
    echo.
    echo [警告] 发现冲突工具: !CONFLICT_TOOLS!
    echo [建议] 运行: pvm migrate 或手动卸载冲突工具
)

echo.

REM ============================================================================
REM 检查 4: PVM 目录结构
REM ============================================================================
echo [检查 4] PVM 目录结构

set PVM_DIR=%USERPROFILE%\.pvm
if exist "!PVM_DIR!" (
    echo [√] PVM 目录存在: !PVM_DIR!
    set /a PASS_COUNT+=1
    
    REM 检查子目录
    if exist "!PVM_DIR!\bin" (
        echo [√] bin 目录存在
    ) else (
        echo [X] bin 目录缺失
        set /a FAIL_COUNT+=1
    )
    
    if exist "!PVM_DIR!\shims" (
        echo [√] shims 目录存在
    ) else (
        echo [X] shims 目录缺失
        set /a FAIL_COUNT+=1
        
        if !FIX_MODE!==1 (
            echo [修复] 正在创建 shims 目录并生成 shim...
            pvm reshim
        )
    )
) else (
    echo [X] PVM 目录不存在
    set /a FAIL_COUNT+=1
)

echo.

REM ============================================================================
REM 检查 5: 运行时安装状态
REM ============================================================================
echo [检查 5] 运行时安装状态

if exist "!PVM_DIR!\installs" (
    pushd "!PVM_DIR!\installs"
    set RUNTIME_COUNT=0
    for /d %%r in (*) do (
        set /a RUNTIME_COUNT+=1
        echo [√] 已安装运行时: %%r
    )
    popd
    
    if !RUNTIME_COUNT! gtr 0 (
        set /a PASS_COUNT+=1
    ) else (
        echo [!] 未安装任何运行时
        set /a WARNING_COUNT+=1
    )
) else (
    echo [!] installs 目录不存在
    set /a WARNING_COUNT+=1
)

echo.

REM ============================================================================
REM 检查 6: 端口冲突
REM ============================================================================
echo [检查 6] 端口冲突检测

REM 常见的开发端口
set PORTS=3000 8080 8000 5000 4000 9000
for %%p in (!PORTS!) do (
    netstat -ano | findstr ":%%p" | findstr "LISTENING" >nul 2>&1
    if !errorlevel!==0 (
        echo [!] 端口 %%p 已被占用
        set /a WARNING_COUNT+=1
        
        if !FIX_MODE!==1 (
            echo [提示] 请手动处理端口冲突或使用: netstat -ano ^| findstr :%%p
        )
    )
)

echo.

REM ============================================================================
REM 检查 7: .npmrc 配置
REM ============================================================================
echo [检查 7] .npmrc 配置

if exist "%USERPROFILE%\.npmrc" (
    echo [√] .npmrc 文件存在
    
    REM 检查是否配置了国内镜像
    findstr /i "registry" "%USERPROFILE%\.npmrc" >nul 2>&1
    if !errorlevel!==0 (
        echo [√] 已配置 registry
        set /a PASS_COUNT+=1
    ) else (
        echo [!] 未配置 registry（建议配置国内镜像）
        set /a WARNING_COUNT+=1
    )
) else (
    echo [!] .npmrc 文件不存在
    set /a WARNING_COUNT+=1
    
    if !FIX_MODE!==1 (
        echo [修复] 正在创建 .npmrc...
        call :create_npmrc
    )
)

echo.

REM ============================================================================
REM 检查 8: shell 配置文件
REM ============================================================================
echo [检查 8] Shell 配置文件

REM 检查 PowerShell profile
set PS_PROFILE=
for /f "tokens=*" %%p in ('powershell -Command "echo $PROFILE"') do set PS_PROFILE=%%p

if exist "!PS_PROFILE!" (
    echo [√] PowerShell profile 存在: !PS_PROFILE!
    
    findstr /i "pvm" "!PS_PROFILE!" >nul 2>&1
    if !errorlevel!==0 (
        echo [√] PowerShell profile 包含 PVM 配置
        set /a PASS_COUNT+=1
    ) else (
        echo [!] PowerShell profile 缺少 PVM 配置（可选）
        set /a WARNING_COUNT+=1
    )
) else (
    echo [!] PowerShell profile 不存在（可选）
    set /a WARNING_COUNT+=1
)

echo.

REM ============================================================================
REM 总结
REM ============================================================================
echo ========================================
echo   诊断结果
echo ========================================
echo.
echo 通过: !PASS_COUNT! 项
echo 失败: !FAIL_COUNT! 项
echo 警告: !WARNING_COUNT! 项
echo.

if !FAIL_COUNT! gtr 0 (
    echo [状态] 发现 !FAIL_COUNT! 个问题需要处理
    if !FIX_MODE!==0 (
        echo [建议] 运行: %~nx0 --fix 自动修复问题
    )
) else if !WARNING_COUNT! gtr 0 (
    echo [状态] 环境基本正常，有 !WARNING_COUNT! 个警告
) else (
    echo [状态] 环境完全正常 ✓
)

exit /b 0

REM ============================================================================
REM 创建 .npmrc
REM ============================================================================
:create_npmrc
(
echo # pnpm 核心配置
echo registry=https://registry.npmmirror.com/
echo save-prefix=""
echo auto-install-peers=true
echo.
echo # lockfile 冻结策略
echo strict-peer-dependencies=false
echo prefer-frozen-lockfile=true
echo.
echo # 版本与冲突处理
echo resolution-mode=time-based
echo prefer-higher-version=true
echo.
echo # 依赖结构
echo shamefully-hoist=false
echo strict-store-content=true
) > "%USERPROFILE%\.npmrc"

echo [完成] .npmrc 已创建
set /a PASS_COUNT+=1

exit /b 0