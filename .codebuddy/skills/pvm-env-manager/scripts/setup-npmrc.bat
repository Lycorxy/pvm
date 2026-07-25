@echo off
REM ============================================================================
REM PVM 环境管理器 - .npmrc 自动配置脚本 (Windows)
REM ============================================================================
REM 功能：自动创建或更新 .npmrc 文件，配置 pnpm 最佳实践
REM 用法：setup-npmrc.bat [--force]
REM   --force : 强制覆盖现有配置
REM ============================================================================

setlocal enabledelayedexpansion

echo ========================================
echo   .npmrc 自动配置工具
echo ========================================
echo.

set FORCE_MODE=0
if "%~1"=="--force" set FORCE_MODE=1

set NPMRC_PATH=%USERPROFILE%\.npmrc

REM 检查文件是否已存在
if exist "!NPMRC_PATH!" (
    if !FORCE_MODE!==0 (
        echo [提示] .npmrc 文件已存在
        echo.
        echo 当前配置:
        type "!NPMRC_PATH!"
        echo.
        set /p OVERWRITE="是否覆盖? (yes/no): "
        if /i not "!OVERWRITE!"=="yes" (
            echo [取消] 保留现有配置
            exit /b 0
        )
    )
    echo [备份] 正在备份现有配置...
    copy "!NPMRC_PATH!" "!NPMRC_PATH!.backup-%date:~0,4%%date:~5,2%%date:~8,2%" >nul 2>&1
)

echo.
echo [创建] 正在生成 .npmrc 配置...

REM 写入配置
(
echo # ============================================
echo # pnpm 核心配置
echo # ============================================
echo registry=https://registry.npmmirror.com/
echo # 保存依赖时不添加版本前缀（^ ~），直接锁定精确版本
echo save-prefix=""
echo.
echo # 自动安装 peer 依赖（无需手动安装）
echo auto-install-peers=true
echo.
echo # ============================================
echo # lockfile 冻结策略
echo # ============================================
echo.
echo # 本地开发：严格按 lockfile 安装，不自动更新
echo # （CI 环境用 --frozen-lockfile 参数）
echo strict-peer-dependencies=false
echo.
echo # 优先使用现有 lockfile，避免意外更新
echo prefer-frozen-lockfile=true
echo.
echo # ============================================
echo # 版本与冲突处理
echo # ============================================
echo.
echo # 版本解析策略：基于时间，保持一致性
echo resolution-mode=time-based
echo.
echo # peer 依赖冲突时取最高版本
echo prefer-higher-version=true
echo.
echo # ============================================
echo # 依赖结构
echo # ============================================
echo.
echo shamefully-hoist=false
echo strict-store-content=true
) > "!NPMRC_PATH!"

if exist "!NPMRC_PATH!" (
    echo.
    echo ========================================
    echo   配置完成
    echo ========================================
    echo.
    echo [√] .npmrc 已创建: !NPMRC_PATH!
    echo.
    echo 配置内容:
    echo ----------------------------------------
    type "!NPMRC_PATH!"
    echo ----------------------------------------
    echo.
    echo [说明] 此配置已包含:
    echo   - 国内镜像源（npmmirror）
    echo   - 精确版本锁定（save-prefix=""）
    echo   - 自动安装 peer 依赖
    echo   - lockfile 冻结策略
    echo   - 版本解析策略
    echo   - 依赖结构优化
    echo.
    echo [建议] 配合 .pvmrc 文件使用，确保团队环境一致
) else (
    echo [X] 创建失败
    exit /b 1
)

exit /b 0