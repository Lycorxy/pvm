@echo off
REM ====================================
REM   PVM 彻底卸载清理脚本
REM   支持：双击运行 或 命令行运行
REM   建议：右键"以管理员身份运行"
REM ====================================

setlocal EnableDelayedExpansion

echo ====================================
echo   PVM 彻底卸载清理脚本
echo ====================================
echo.

REM 获取脚本所在目录（用于查找 pvm.exe）
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

REM 可能的 pvm 安装位置
set "PVM_HOME=%USERPROFILE%\.pvm"
set "PVM_LOCAL=%LOCALAPPDATA%\pvm"

REM ====================================
echo [1/7] 检测 pvm 安装...
echo.

REM 尝试用 pvm 自卸载命令
where pvm >nul 2>&1
if %errorlevel% equ 0 (
    echo   找到 pvm 命令，尝试自卸载...
    pvm uninstall --yes 2>nul
    if %errorlevel% equ 0 (
        echo   [√] pvm 自卸载完成
    ) else (
        echo   [!] 自卸载失败，继续手动清理...
    )
) else (
    echo   未找到 pvm 命令，执行手动清理...
)
echo.

REM ====================================
echo [2/7] 终止占用进程...
echo.

REM 终止 pvm.exe
taskkill /F /IM pvm.exe >nul 2>&1
if %errorlevel% equ 0 (
    echo   [√] 已终止 pvm.exe
) else (
    echo   [√] pvm.exe 未运行
)

REM 终止 node.exe（PVM 管理的）
for /f "tokens=1" %%i in ('tasklist /FI "IMAGENAME eq node.exe" /NH ^| findstr /i "node.exe"') do (
    wmic process where "ProcessId=%%i AND CommandLine like '%%pvm%%'" call terminate >nul 2>&1
)

REM 终止其他可能占用的运行时进程
for %%p in (git.exe git-bash.exe bash.exe) do (
    taskkill /F /IM %%p >nul 2>&1
)

echo   [√] 进程清理完成
echo.

REM ====================================
echo [3/7] 等待进程完全结束...
timeout /T 3 /NOBREAK >nul
echo   [√] 等待完成
echo.

REM ====================================
echo [4/7] 清理 PATH 环境变量...
echo.

REM 调用 PowerShell 清理 PATH
powershell -NoProfile -Command "& {
    $scopes = @('User', 'Machine')
    foreach ($scope in $scopes) {
        try {
            $path = [Environment]::GetEnvironmentVariable('PATH', $scope)
            if ($path -and $path -like '*pvm*') {
                $newPath = ($path -split ';' | Where-Object { $_ -notlike '*pvm*' -and $_ -ne '' }) -join ';'
                [Environment]::SetEnvironmentVariable('PATH', $newPath, $scope)
                Write-Host ('  [√] 已清理 ' + $scope + ' PATH')
            } else {
                Write-Host ('  [√] ' + $scope + ' PATH 无 pvm 记录')
            }
        } catch {
            Write-Host ('  [!] 清理 ' + $scope + ' PATH 失败: ' + $_.Exception.Message)
        }
    }
    [Environment]::SetEnvironmentVariable('PVM_HOME', $null, 'User')
    [Environment]::SetEnvironmentVariable('PVM_HOME', $null, 'Machine')
    Write-Host('  [√] 已清除 PVM_HOME 环境变量')
}"
echo.

REM ====================================
echo [5/7] 删除 pvm 目录...
echo.

set "DIR_COUNT=0"

REM 删除 ~/.pvm
if exist "%PVM_HOME%" (
    echo   删除 %PVM_HOME%...
    rmdir /S /Q "%PVM_HOME%" 2>nul
    if exist "%PVM_HOME%" (
        echo   [!] 部分文件删除失败（可能仍被占用）
        REM 列出被占用的文件
        for /f "delims=" %%i in ('dir /b /a "%PVM_HOME%" 2^>nul') do (
            echo     - %%i
        )
    ) else (
        echo   [√] 已删除 %PVM_HOME%
    )
    set /a DIR_COUNT+=1
)

REM 删除 LocalAppData\pvm（MSI 安装位置）
if exist "%PVM_LOCAL%" (
    echo   删除 %PVM_LOCAL%...
    rmdir /S /Q "%PVM_LOCAL%" 2>nul
    if exist "%PVM_LOCAL%" (
        echo   [!] 部分文件删除失败
    ) else (
        echo   [√] 已删除 %PVM_LOCAL%
    )
    set /a DIR_COUNT+=1
)

if "!DIR_COUNT!"=="0" (
    echo   [√] 未找到 pvm 目录
)
echo.

REM ====================================
echo [6/7] 清理注册表残留...
echo.

powershell -NoProfile -Command "& {
    try {
        Remove-Item -Path 'HKCU:\Software\pvm' -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\App Paths\pvm.exe' -Recurse -Force -ErrorAction SilentlyContinue
        Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -ErrorAction SilentlyContinue |
            Where-Object { $_.PSObject.Properties.Name -like '*pvm*' } |
            ForEach-Object {
                Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name $_.PSObject.Properties.Name -ErrorAction SilentlyContinue
            }
        Write-Host '  [√] 注册表清理完成'
    } catch {
        Write-Host ('  [!] 注册表清理异常: ' + $_.Exception.Message)
    }
}"
echo.

REM ====================================
echo [7/7] 清理开始菜单快捷方式...
echo.

set "START_MENU_1=%APPDATA%\Microsoft\Windows\Start Menu\Programs"
set "START_MENU_2=%ProgramData%\Microsoft\Windows\Start Menu\Programs"

set "SHORTCUT_FOUND=0"

if exist "%START_MENU_1%" (
    for /f "delims=" %%i in ('dir /b /s "%START_MENU_1%\*pvm*.lnk" 2^>nul') do (
        del /F /Q "%%i" >nul 2>&1
        if exist "%%i" (
            echo   [!] 无法删除: %%~nxi
        ) else (
            echo   [√] 已删除: %%~nxi
            set /a SHORTCUT_FOUND+=1
        )
    )
)

if exist "%START_MENU_2%" (
    for /f "delims=" %%i in ('dir /b /s "%START_MENU_2%\*pvm*.lnk" 2^>nul') do (
        del /F /Q "%%i" >nul 2>&1
        if exist "%%i" (
            echo   [!] 无法删除: %%~nxi
        ) else (
            echo   [√] 已删除: %%~nxi
            set /a SHORTCUT_FOUND+=1
        )
    )
)

if "!SHORTCUT_FOUND!"=="0" (
    echo   [√] 未找到开始菜单快捷方式
)
echo.

REM ====================================
echo ====================================
echo   pvm 卸载清理完成！
echo ====================================
echo.
echo   建议操作：
echo   1. 关闭并重新打开终端
echo   2. 如果仍有文件残留，请重启电脑后手动删除：
echo      %USERPROFILE%\.pvm
echo.
pause
endlocal
