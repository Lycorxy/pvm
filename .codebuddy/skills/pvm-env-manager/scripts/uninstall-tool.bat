@echo off
REM ============================================================================
REM PVM 环境管理器 - 软件彻底卸载脚本 (Windows)
REM ============================================================================
REM 功能：彻底卸载指定软件，包括文件、环境变量、注册表、端口冲突
REM 用法：uninstall-tool.bat <软件名> [--yes]
REM   软件名: node, git, nvm, pvm, python, yarn, pnpm
REM   --yes   : 跳过确认直接卸载
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   软件彻底卸载工具
echo ========================================
echo.

REM 参数检查
if "%~1"=="" (
    echo [错误] 请指定要卸载的软件名称
    echo 用法: %~nx0 ^<软件名^> [--yes]
    echo.
    echo 支持的软件:
    echo   node   - Node.js
    echo   git    - Git
    echo   nvm    - Node Version Manager
    echo   pvm    - Polyglot Version Manager
    echo   python - Python
    echo   yarn   - Yarn
    echo   pnpm   - PNPM
    exit /b 1
)

set TOOL_NAME=%~1
set SKIP_CONFIRM=0
if "%~2"=="--yes" set SKIP_CONFIRM=1

REM 转换为小写
for %%i in (node git nvm pvm python yarn pnpm) do (
    if /i "!TOOL_NAME!"=="%%i" set TOOL_NAME=%%i
)

echo [目标] 正在准备卸载: !TOOL_NAME!
echo.

REM 确认操作
if !SKIP_CONFIRM!==0 (
    echo [警告] 此操作将彻底删除 !TOOL_NAME! 及其所有相关文件、环境变量、注册表项。
    echo         此操作不可逆！
    echo.
    set /p CONFIRM="确认卸载? (yes/no): "
    if /i not "!CONFIRM!"=="yes" (
        echo [取消] 用户取消操作
        exit /b 0
    )
)

echo.
echo ========================================
echo   开始卸载: !TOOL_NAME!
echo ========================================
echo.

REM 根据软件类型执行不同的卸载流程
if "!TOOL_NAME!"=="node" call :uninstall_node
if "!TOOL_NAME!"=="git" call :uninstall_git
if "!TOOL_NAME!"=="nvm" call :uninstall_nvm
if "!TOOL_NAME!"=="pvm" call :uninstall_pvm
if "!TOOL_NAME!"=="python" call :uninstall_python
if "!TOOL_NAME!"=="yarn" call :uninstall_yarn
if "!TOOL_NAME!"=="pnpm" call :uninstall_pnpm

echo.
echo ========================================
echo   卸载完成: !TOOL_NAME!
echo ========================================
echo.
echo [建议] 请重启终端或重新打开命令行窗口以使环境变量生效

exit /b 0

REM ============================================================================
REM 卸载 Node.js
REM ============================================================================
:uninstall_node
echo [步骤 1] 终止 Node.js 进程
taskkill /F /IM node.exe >nul 2>&1
taskkill /F /IM npm.exe >nul 2>&1
taskkill /F /IM npx.exe >nul 2>&1
echo [完成] 进程已终止

echo.
echo [步骤 2] 查找 Node.js 安装路径
set NODE_PATHS=
for /f "tokens=*" %%p in ('where.exe node 2^>nul') do (
    for %%d in ("%%p") do (
        set "NODE_DIR=%%~dpd"
        echo [发现] !NODE_DIR!
        set "NODE_PATHS=!NODE_PATHS! !NODE_DIR!"
    )
)

echo.
echo [步骤 3] 清理环境变量
REM 调用 PowerShell 清理 PATH
powershell -Command ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
     $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'node|npm|Nodejs' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"
echo [完成] 用户 PATH 已清理

powershell -Command ^
    "try { $sysPath = [Environment]::GetEnvironmentVariable('Path', 'Machine'); ^
     $cleaned = ($sysPath -split ';' | Where-Object { $_ -notmatch 'node|npm|Nodejs' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'Machine') } catch {}"
echo [完成] 系统 PATH 已清理（需要管理员权限）

echo.
echo [步骤 4] 删除安装目录
for %%d in (!NODE_PATHS!) do (
    if exist "%%d" (
        echo [删除] %%d
        rd /s /q "%%d" 2>nul
    )
)

REM 删除常见的 Node.js 安装位置
for %%d in ("%ProgramFiles%\nodejs" "%ProgramFiles(x86)%%\nodejs" "%APPDATA%\npm" "%LOCALAPPDATA%\npm-cache") do (
    if exist "%%d" (
        echo [删除] %%d
        rd /s /q "%%d" 2>nul
    )
)

echo.
echo [步骤 5] 清理注册表
powershell -Command "Remove-Item -Path 'HKCU:\Software\Node.js' -Recurse -Force -ErrorAction SilentlyContinue"
powershell -Command "Remove-Item -Path 'HKLM:\Software\Node.js' -Recurse -Force -ErrorAction SilentlyContinue"
echo [完成] 注册表已清理

exit /b 0

REM ============================================================================
REM 卸载 Git
REM ============================================================================
:uninstall_git
echo [步骤 1] 终止 Git 进程
taskkill /F /IM git.exe >nul 2>&1
taskkill /F /IM git-cmd.exe >nul 2>&1
echo [完成] 进程已终止

echo.
echo [步骤 2] 查找 Git 安装路径
set GIT_PATHS=
for /f "tokens=*" %%p in ('where.exe git 2^>nul') do (
    for %%d in ("%%p") do (
        set "GIT_DIR=%%~dpd"
        echo [发现] !GIT_DIR!
        set "GIT_PATHS=!GIT_PATHS! !GIT_DIR!"
    )
)

echo.
echo [步骤 3] 清理环境变量
powershell -Command ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
     $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'Git\\bin|Git\\cmd' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"
echo [完成] 用户 PATH 已清理

echo.
echo [步骤 4] 删除安装目录
for %%d in (!GIT_PATHS!) do (
    if exist "%%d" (
        echo [删除] %%d
        rd /s /q "%%d" 2>nul
    )
)

REM 删除常见的 Git 安装位置
for %%d in ("%ProgramFiles%\Git" "%ProgramFiles(x86)%\Git" "%LOCALAPPDATA%\Programs\Git") do (
    if exist "%%d" (
        echo [删除] %%d
        rd /s /q "%%d" 2>nul
    )
)

echo.
echo [步骤 5] 清理环境变量
powershell -Command "[Environment]::SetEnvironmentVariable('GIT_HOME', $null, 'User')"
powershell -Command "[Environment]::SetEnvironmentVariable('GIT_HOME', $null, 'Machine')"
echo [完成] GIT_HOME 已清除

exit /b 0

REM ============================================================================
REM 卸载 NVM
REM ============================================================================
:uninstall_nvm
echo [步骤 1] 使用 Node.js 脚本卸载 NVM
if exist "%~dp0uninstall_nvm.js" (
    node "%~dp0uninstall_nvm.js" --yes
) else (
    echo [警告] uninstall_nvm.js 未找到，执行手动卸载
    
    echo.
    echo [步骤 1a] 终止 NVM 相关进程
    taskkill /F /IM nvm.exe >nul 2>&1
    echo [完成] 进程已终止
    
    echo.
    echo [步骤 1b] 清理环境变量
    powershell -Command "[Environment]::SetEnvironmentVariable('NVM_HOME', $null, 'User')"
    powershell -Command "[Environment]::SetEnvironmentVariable('NVM_SYMLINK', $null, 'User')"
    powershell -Command "[Environment]::SetEnvironmentVariable('NVM_HOME', $null, 'Machine')"
    powershell -Command "[Environment]::SetEnvironmentVariable('NVM_SYMLINK', $null, 'Machine')"
    
    powershell -Command ^
        "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
         $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'nvm' }) -join ';'; ^
         [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"
    
    echo.
    echo [步骤 1c] 删除 NVM 目录
    for %%d in ("%APPDATA%\nvm" "%USERPROFILE%\nvm" "C:\nvm") do (
        if exist "%%d" (
            echo [删除] %%d
            rd /s /q "%%d" 2>nul
        )
    )
)

exit /b 0

REM ============================================================================
REM 卸载 PVM
REM ============================================================================
:uninstall_pvm
echo [步骤 1] 使用 PVM 自卸载命令
if exist "%USERPROFILE%\.pvm\bin\pvm.exe" (
    "%USERPROFILE%\.pvm\bin\pvm.exe" uninstall --yes
) else (
    echo [警告] pvm.exe 未找到，执行手动卸载
    
    echo.
    echo [步骤 1a] 终止 PVM 进程
    taskkill /F /IM pvm.exe >nul 2>&1
    echo [完成] 进程已终止
    
    echo.
    echo [步骤 1b] 清理环境变量
    powershell -Command "[Environment]::SetEnvironmentVariable('PVM_HOME', $null, 'User')"
    powershell -Command "[Environment]::SetEnvironmentVariable('PVM_HOME', $null, 'Machine')"
    
    powershell -Command ^
        "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
         $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch '\.pvm' }) -join ';'; ^
         [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"
    
    echo.
    echo [步骤 1c] 删除 PVM 目录
    if exist "%USERPROFILE%\.pvm" (
        echo [删除] %USERPROFILE%\.pvm
        rd /s /q "%USERPROFILE%\.pvm" 2>nul
    )
)

exit /b 0

REM ============================================================================
REM 卸载 Python
REM ============================================================================
:uninstall_python
echo [步骤 1] 终止 Python 进程
taskkill /F /IM python.exe >nul 2>&1
taskkill /F /IM pythonw.exe >nul 2>&1
taskkill /F /IM pip.exe >nul 2>&1
echo [完成] 进程已终止

echo.
echo [步骤 2] 查找 Python 安装路径
set PYTHON_PATHS=
for /f "tokens=*" %%p in ('where.exe python 2^>nul') do (
    for %%d in ("%%p") do (
        set "PYTHON_DIR=%%~dpd"
        echo [发现] !PYTHON_DIR!
        set "PYTHON_PATHS=!PYTHON_PATHS! !PYTHON_DIR!"
    )
)

echo.
echo [步骤 3] 清理环境变量
powershell -Command ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
     $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'Python|python|Scripts' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"
echo [完成] 用户 PATH 已清理

powershell -Command "[Environment]::SetEnvironmentVariable('PYTHON_HOME', $null, 'User')"

echo.
echo [步骤 4] 删除安装目录
for %%d in (!PYTHON_PATHS!) do (
    if exist "%%d" (
        echo [删除] %%d
        rd /s /q "%%d" 2>nul
    )
)

exit /b 0

REM ============================================================================
REM 卸载 Yarn
REM ============================================================================
:uninstall_yarn
echo [步骤 1] 终止 Yarn 进程
taskkill /F /IM yarn.exe >nul 2>&1
taskkill /F /IM yarnpkg.exe >nul 2>&1
echo [完成] 进程已终止

echo.
echo [步骤 2] 删除 Yarn 全局目录
if exist "%LOCALAPPDATA%\Yarn" (
    echo [删除] %LOCALAPPDATA%\Yarn
    rd /s /q "%LOCALAPPDATA%\Yarn" 2>nul
)

echo.
echo [步骤 3] 清理环境变量
powershell -Command ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
     $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'Yarn|yarn' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"

exit /b 0

REM ============================================================================
REM 卸载 PNPM
REM ============================================================================
:uninstall_pnpm
echo [步骤 1] 终止 PNPM 进程
taskkill /F /IM pnpm.exe >nul 2>&1
taskkill /F /IM pnpm-cmd.exe >nul 2>&1
echo [完成] 进程已终止

echo.
echo [步骤 2] 删除 PNPM 全局目录
if exist "%LOCALAPPDATA%\pnpm-cache" (
    echo [删除] %LOCALAPPDATA%\pnpm-cache
    rd /s /q "%LOCALAPPDATA%\pnpm-cache" 2>nul
)

if exist "%APPDATA%\pnpm" (
    echo [删除] %APPDATA%\pnpm
    rd /s /q "%APPDATA%\pnpm" 2>nul
)

echo.
echo [步骤 3] 清理环境变量
powershell -Command ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User'); ^
     $cleaned = ($userPath -split ';' | Where-Object { $_ -notmatch 'pnpm' }) -join ';'; ^
     [Environment]::SetEnvironmentVariable('Path', $cleaned, 'User')"

exit /b 0