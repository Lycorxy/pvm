@echo off
setlocal enabledelayedexpansion

REM PVM Uninstall Tool (Windows) - 18 tools supported
REM Usage: uninstall-tool.bat <name> [--yes]

set "TOOL_NAME=%~1"
if "%TOOL_NAME%"=="" (
    echo.
    echo   PVM Uninstaller - Supported:
    echo   Runtime: node git python rust go bun deno pnpm yarn pvm
    echo   Conflict: nvm volta fnm nodenv pyenv rustup asdf conda
    echo.
    echo   Usage: uninstall-tool.bat ^<name^> [--yes]
    exit /b 1
)

set "SKIP_CONFIRM="
if /i "%~2"=="--yes" set "SKIP_CONFIRM=y"

for %%i in (node git python rust go bun deno pnpm yarn pvm nvm volta fnm nodenv pyenv rustup asdf conda) do (
    if /i "!TOOL_NAME!"=="%%i" set "TOOL_NAME=%%i"
)

echo.
echo [Target] Uninstall: %TOOL_NAME%

if not defined SKIP_CONFIRM (
    set /p "CONFIRM=Confirm? (y/n): "
    if /i not "!CONFIRM!"=="y" echo Cancelled & exit /b 0
)

if "%TOOL_NAME%"=="node"   goto :do_node
if "%TOOL_NAME%"=="git"    goto :do_git
if "%TOOL_NAME%"=="python" goto :do_python
if "%TOOL_NAME%"=="rust"   goto :do_rust
if "%TOOL_NAME%"=="go"     goto :do_go
if "%TOOL_NAME%"=="bun"    goto :do_bun
if "%TOOL_NAME%"=="deno"   goto :do_deno
if "%TOOL_NAME%"=="pnpm"   goto :do_pnpm
if "%TOOL_NAME%"=="yarn"   goto :do_yarn
if "%TOOL_NAME%"=="pvm"    goto :do_pvm
if "%TOOL_NAME%"=="nvm"    goto :do_nvm
if "%TOOL_NAME%"=="volta"  goto :do_volta
if "%TOOL_NAME%"=="fnm"    goto :do_fnm
if "%TOOL_NAME%"=="nodenv" goto :do_nodenv
if "%TOOL_NAME%"=="pyenv"  goto :do_pyenv
if "%TOOL_NAME%"=="rustup" goto :do_rustup
if "%TOOL_NAME%"=="asdf"   goto :do_asdf
if "%TOOL_NAME%"=="conda"  goto :do_conda
echo [ERROR] Unsupported: %TOOL_NAME% & exit /b 1

:do_node
echo [1/4] Killing processes...
taskkill /F /IM node.exe >nul 2>&1
taskkill /F /IM npm.exe >nul 2>&1
taskkill /F /IM npx.exe >nul 2>&1
echo [2/4] Removing directories...
for %%d in ("%ProgramFiles%\nodejs" "%ProgramFiles(x86)%\nodejs" "%APPDATA%\npm" "%LOCALAPPDATA%\npm-cache" "%USERPROFILE%\.npm") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [3/4] Cleaning PATH + registry...
call :clean_path "node|npm|Nodejs"
powershell -NoP -C "Remove-Item 'HKCU:\Software\Node.js' -Rec -Force -EA 0; Remove-Item 'HKLM:\Software\Node.js' -Rec -Force -EA 0"
echo [4/4] Done
goto :eof_done

:do_git
echo [1/3] Killing processes...
taskkill /F /IM git.exe >nul 2>&1
echo [2/3] Removing directories...
for %%d in ("%ProgramFiles%\Git" "%ProgramFiles(x86)%\Git" "%LOCALAPPDATA%\Programs\Git") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [3/3] Cleaning PATH...
call :clean_path "Git\\bin|Git\\cmd"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('GIT_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('GIT_HOME',$null,'Machine')"
goto :eof_done

:do_python
echo [1/4] Killing processes...
taskkill /F /IM python.exe >nul 2>&1
taskkill /F /IM pythonw.exe >nul 2>&1
taskkill /F /IM pip.exe >nul 2>&1
echo [2/4] Removing directories...
for %%d in ("%LOCALAPPDATA%\Programs\Python" "%USERPROFILE%\AppData\Local\Python") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [3/4] Cleaning PATH...
call :clean_path "Python|python|Scripts"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('PYTHON_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('PYTHONPATH',$null,'User')"
echo [4/4] Done
goto :eof_done

:do_rust
echo [1/3] Killing processes...
taskkill /F /IM rustc.exe >nul 2>&1
taskkill /F /IM cargo.exe >nul 2>&1
taskkill /F /IM rustup.exe >nul 2>&1
echo [2/3] Removing .cargo .rustup...
@if exist "%USERPROFILE%\.cargo" rd /s /q "%USERPROFILE%\.cargo" >nul 2>&1
@if exist "%USERPROFILE%\.rustup" rd /s /q "%USERPROFILE%\.rustup" >nul 2>&1
echo [3/3] Cleaning env vars...
call :clean_path "cargo|rustup|\\.rust"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('CARGO_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('RUSTUP_HOME',$null,'User')"
goto :eof_done

:do_go
echo [1/4] Killing processes...
taskkill /F /IM go.exe >nul 2>&1
echo [2/4] Removing directories...
for %%d in ("%ProgramFiles%\Go" "%ProgramFiles(x86)%\Go" "%USERPROFILE%\go" "%LOCALAPPDATA%\Go") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [3/4] Cleaning PATH...
call :clean_path "\\Go\\bin|\\Go\\"
echo [4/4] Clearing GOROOT/GOPATH...
powershell -NoP -C "[Environment]::SetEnvironmentVariable('GOROOT',$null,'User'); [Environment]::SetEnvironmentVariable('GOPATH',$null,'User'); [Environment]::SetEnvironmentVariable('GOROOT',$null,'Machine'); [Environment]::SetEnvironmentVariable('GOPATH',$null,'Machine')"
goto :eof_done

:do_bun
echo [1/2] Killing + removing .bun...
taskkill /F /IM bun.exe >nul 2>&1
@if exist "%USERPROFILE%\.bun" rd /s /q "%USERPROFILE%\.bun" >nul 2>&1
echo [2/2] Cleaning PATH...
call :clean_path "\\.bun"
goto :eof_done

:do_deno
echo [1/2] Killing + removing .deno...
taskkill /F /IM deno.exe >nul 2>&1
@if exist "%USERPROFILE%\.deno" rd /s /q "%USERPROFILE%\.deno" >nul 2>&1
@if exist "%LOCALAPPDATA%\deno" rd /s /q "%LOCALAPPDATA%\deno" >nul 2>&1
echo [2/2] Cleaning PATH...
call :clean_path "deno"
goto :eof_done

:do_pnpm
echo [1/2] Killing + removing cache...
taskkill /F /IM pnpm.exe >nul 2>&1
for %%d in ("%LOCALAPPDATA%\pnpm-cache" "%LOCALAPPDATA%\pnpm" "%APPDATA%\pnpm") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [2/2] Cleaning PATH...
call :clean_path "pnpm"
goto :eof_done

:do_yarn
echo [1/2] Killing + removing .yarn...
taskkill /F /IM yarn.exe >nul 2>&1
taskkill /F /IM yarnpkg.exe >nul 2>&1
@if exist "%LOCALAPPDATA%\Yarn" rd /s /q "%LOCALAPPDATA%\Yarn" >nul 2>&1
@if exist "%USERPROFILE%\.yarn" rd /s /q "%USERPROFILE%\.yarn" >nul 2>&1
echo [2/2] Cleaning PATH...
call :clean_path "Yarn|yarn"
goto :eof_done

:do_pvm
echo [1/3] Running pvm self-uninstall...
@if exist "%USERPROFILE%\.pvm\bin\pvm.exe" "%USERPROFILE%\.pvm\bin\pvm.exe" uninstall --yes >nul 2>&1
echo [2/3] Removing .pvm dir...
taskkill /F /IM pvm.exe >nul 2>&1
@if exist "%USERPROFILE%\.pvm" rd /s /q "%USERPROFILE%\.pvm" >nul 2>&1
echo [3/3] Cleaning PATH...
call :clean_path "\\.pvm"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('PVM_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('PVM_HOME',$null,'Machine')"
goto :eof_done

:do_nvm
echo [1/3] Cleaning registry + env vars...
powershell -NoP -C "[Environment]::SetEnvironmentVariable('NVM_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('NVM_SYMLINK',$null,'User'); [Environment]::SetEnvironmentVariable('NVM_HOME',$null,'Machine'); [Environment]::SetEnvironmentVariable('NVM_SYMLINK',$null,'Machine')"
echo [2/3] Removing directories...
for %%d in ("%APPDATA%\nvm" "%USERPROFILE%\nvm" "C:\nvm") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [3/3] Cleaning PATH + registry...
call :clean_path "nvm|NVM_HOME|NVM_SYMLINK"
powershell -NoP -C "Remove-Item 'HKCU:\Software\nvm' -Rec -Force -EA 0; Remove-Item 'HKLM:\Software\nvm' -Rec -Force -EA 0"
goto :eof_done

:do_volta
echo [1/2] Removing .volta...
taskkill /F /IM volta.exe >nul 2>&1
@if exist "%USERPROFILE%\.volta" rd /s /q "%USERPROFILE%\.volta" >nul 2>&1
@if exist "%LOCALAPPDATA%\Volta" rd /s /q "%LOCALAPPDATA%\Volta" >nul 2>&1
echo [2/2] Cleaning env vars...
call :clean_path "volta|Volta"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('VOLTA_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('VOLTA_HOME',$null,'Machine')"
goto :eof_done

:do_fnm
echo [1/2] Removing .fnm...
taskkill /F /IM fnm.exe >nul 2>&1
for %%d in ("%USERPROFILE%\.fnm" "%LOCALAPPDATA%\fnm" "%APPDATA%\fnm") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
echo [2/2] Cleaning PATH...
call :clean_path "fnm"
goto :eof_done

:do_nodenv
echo [1/2] Removing .nodenv...
taskkill /F /IM nodenv.exe >nul 2>&1
@if exist "%USERPROFILE%\.nodenv" rd /s /q "%USERPROFILE%\.nodenv" >nul 2>&1
echo [2/2] Cleaning env vars...
call :clean_path "nodenv"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('NODENV',$null,'User'); [Environment]::SetEnvironmentVariable('NODENV_ROOT',$null,'User')"
goto :eof_done

:do_pyenv
echo [1/2] Removing .pyenv...
taskkill /F /IM pyenv.exe >nul 2>&1
@if exist "%USERPROFILE%\.pyenv" rd /s /q "%USERPROFILE%\.pyenv" >nul 2>&1
echo [2/2] Cleaning env vars...
call :clean_path "pyenv|\\.pyenv"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('PYENV',$null,'User'); [Environment]::SetEnvironmentVariable('PYENV_ROOT',$null,'User'); [Environment]::SetEnvironmentVariable('PYENV_HOME',$null,'User')"
goto :eof_done

:do_rustup
echo [1/2] Killing + removing .rustup/.cargo...
taskkill /F /IM rustup.exe >nul 2>&1
taskkill /F /IM rustc.exe >nul 2>&1
taskkill /F /IM cargo.exe >nul 2>&1
@if exist "%USERPROFILE%\.rustup" rd /s /q "%USERPROFILE%\.rustup" >nul 2>&1
@if exist "%USERPROFILE%\.cargo" rd /s /q "%USERPROFILE%\.cargo" >nul 2>&1
echo [2/2] Cleaning env vars...
call :clean_path "rustup|cargo|\\.rust"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('RUSTUP_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('CARGO_HOME',$null,'User')"
goto :eof_done

:do_asdf
echo [1/2] Removing .asdf...
@if exist "%USERPROFILE%\.asdf" rd /s /q "%USERPROFILE%\.asdf" >nul 2>&1
echo [2/2] Cleaning env vars...
call :clean_path "asdf"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('ASDF_HOME',$null,'User'); [Environment]::SetEnvironmentVariable('ASDF_DIR',$null,'User')"
goto :eof_done

:do_conda
echo [1/3] Killing processes...
taskkill /F /IM conda.exe >nul 2>&1
echo [2/3] Removing Anaconda/Miniconda dirs...
for %%d in ("%USERPROFILE%\anaconda3" "%USERPROFILE%\miniconda3" "%USERPROFILE%\Anaconda3" "%ProgramFiles%\Anaconda3" "%LOCALAPPDATA%\anaconda3" "%LOCALAPPDATA%\miniconda3") do @if exist "%%d" rd /s /q "%%d" >nul 2>&1
@if exist "%USERPROFILE%\.condarc" del /f "%USERPROFILE%\.condarc" >nul 2>&1
@if exist "%USERPROFILE%\.conda" rd /s /q "%USERPROFILE%\.conda" >nul 2>&1
echo [3/3] Cleaning env vars...
call :clean_path "anaconda|miniconda|conda"
powershell -NoP -C "[Environment]::SetEnvironmentVariable('CONDA_DEFAULT_ENV',$null,'User'); [Environment]::SetEnvironmentVariable('CONDA_PREFIX',$null,'User')"
goto :eof_done

:clean_path
powershell -NoP -C "$u=[Environment]::GetEnvironmentVariable('Path','User'); $c=($u -split ';' | ? { $_ -notmatch '%~1' }) -join ';'; [Environment]::SetEnvironmentVariable('Path',$c,'User'); try { $s=[Environment]::GetEnvironmentVariable('Path','Machine'); $c=($s -split ';' | ? { $_ -notmatch '%~1' }) -join ';'; [Environment]::SetEnvironmentVariable('Path',$c,'Machine') } catch {}"
exit /b 0

:eof_done
echo.
echo [done] %TOOL_NAME% uninstalled. Restart terminal to take effect.
exit /b 0
