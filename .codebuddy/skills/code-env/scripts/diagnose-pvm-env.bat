@echo off
setlocal enabledelayedexpansion
REM PVM Environment Diagnose (Windows)
REM Usage: diagnose-pvm-env.bat [--fix]

set "FIX=%~1"
set P=0& set F=0& set W=0

echo [1/8] PVM install status...
where.exe pvm >nul 2>&1
if !errorlevel!==0 (for /f %%v in ('pvm --version 2^>nul') do echo [OK] pvm %%v & set /a P+=1) else (echo [FAIL] pvm not found & set /a F+=1)

echo [2/8] Env vars...
for /f %%h in ('powershell -NoP -C "[Environment]::GetEnvironmentVariable('PVM_HOME','User')"') do set PH=%%h
if defined PH (echo [OK] PVM_HOME=!PH! & set /a P+=1) else (echo [WARN] PVM_HOME not set & set /a W+=1)
for /f %%p in ('powershell -NoP -C "[Environment]::GetEnvironmentVariable('Path','User')"') do set UP=%%p
echo !UP! | findstr /i "\.pvm\\shims" >nul && (echo [OK] PATH has .pvm\shims & set /a P+=1) || (echo [FAIL] PATH missing .pvm\shims & set /a F+=1)
if "%FIX%"=="--fix" (
    powershell -NoP -C "$u=[Environment]::GetEnvironmentVariable('Path','User');$n=$env:USERPROFILE+'\.pvm\shims;'+$u;[Environment]::SetEnvironmentVariable('Path',$n,'User')" >nul 2>&1
    echo [FIXED] PATH updated, restart terminal
)

echo [3/8] Conflict tools...
set CT=
for %%t in (nvm fnm nodenv pyenv rustup volta asdf conda) do (
    where.exe %%t >nul 2>&1 && (echo [WARN] conflict: %%t & set /a W+=1 & set CT=!CT! %%t)
)
if not defined CT (echo [OK] no conflicts & set /a P+=1)

echo [4/8] PVM directories...
set PD=%USERPROFILE%\.pvm
if exist "!PD!" (echo [OK] !PD! & if exist "!PD!\bin" (echo [OK] bin/) else echo [FAIL] bin/ missing & set /a F+=1 & if exist "!PD!\shims" (echo [OK] shims/) else (echo [FAIL] shims/ missing & set /a F+=1)) else (echo [FAIL] .pvm/ missing & set /a F+=1)

echo [5/8] Installed runtimes...
if exist "!PD!\installs" (pushd "!PD!\installs"& for /d %%r in (*) do echo [OK] runtime: %%r & popd & set /a P+=1) else (echo [WARN] no runtimes installed & set /a W+=1)

echo [6/8] Port check...
for %%p in (3000 8080 8000 5000 4000 9000) do netstat -ano | findstr ":%%p" | findstr "LISTENING" >nul 2>&1 && (echo [WARN] port %%p in use & set /a W+=1)

echo [7/8] .npmrc config...
if exist "%USERPROFILE%\.npmrc" (findstr /i "registry" "%USERPROFILE%\.npmrc" >nul 2>&1 && (echo [OK] .npmrc configured & set /a P+=1) || echo [WARN] no registry in .npmrc & set /a W+=1) else (echo [WARN] .npmrc missing & set /a W+=1 & if "%FIX%"=="--fix" call :mk_npmrc)

echo [8/8] Shell profile...
for /f %%p in ('powershell -NoP -C "$PROFILE"') do set PP=%%p
if exist "!PP!" (findstr /i "pvm" "!PP!" >nul 2>&1 && (echo [OK] PS profile OK & set /a P+=1) || echo [WARN] PS profile missing pvm & set /a W+=1) else (echo [WARN] no PS profile & set /a W+=1)

echo.
echo --- Result ---
echo Pass: !P!  Fail: !F!  Warn: !W!
if !F! gtr 0 (echo [STATUS] !F! issues found. Run with --fix to auto-repair) else if !W! gtr 0 (echo [STATUS] OK with !W! warnings) else (echo [STATUS] All good)
exit /b 0

:mk_npmrc
(
echo registry=https://registry.npmmirror.com/
echo save-prefix=""
echo auto-install-peers=true
echo prefer-frozen-lockfile=true
echo strict-peer-dependencies=false
) > "%USERPROFILE%\.npmrc"
echo [FIXED] .npmrc created
exit /b 0
