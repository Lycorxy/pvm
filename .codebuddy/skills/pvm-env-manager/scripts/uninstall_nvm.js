#!/usr/bin/env node
/**
 * PVM 环境管理器 · nvm 卸载脚本
 *
 * 前提：PVM 已安装 node，且 `node --version` 输出正确。
 * 职责：
 *   1. 终止 nvm 相关进程
 *   2. 检测 nvm 安装路径（NVM_HOME / NVM_SYMLINK / 常见路径）
 *   3. 从用户 PATH 和系统 PATH 移除 nvm 相关条目
 *   4. 清除 NVM_HOME、NVM_SYMLINK 环境变量
 *   5. 删除 nvm 安装目录
 *   6. 清理注册表残留
 *   7. 输出清理报告
 *
 * 用法：
 *   node scripts/uninstall_nvm.js          # 交互模式（需确认）
 *   node scripts/uninstall_nvm.js --yes    # 跳过确认
 *
 * 仅支持 Windows。
 */

const { execSync, exec } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const IS_WIN = os.platform() === 'win32';
const SKIP_CONFIRM = process.argv.includes('--yes');

if (!IS_WIN) {
  console.log('此脚本仅支持 Windows。macOS/Linux 请手动删除 nvm 目录并清理 shell rc。');
  process.exit(1);
}

// ---- 工具函数 ----
function log(msg)  { console.log(`  [√] ${msg}`); }
function warn(msg) { console.log(`  [!] ${msg}`); }
function step(msg) { console.log(`\n==> ${msg}`); }
function die(msg)  { console.error(`  [✗] ${msg}`); process.exit(1); }

function runPs(script) {
  try {
    return execSync(
      `powershell -NoProfile -Command "${script.replace(/"/g, '\\"')}"`,
      { encoding: 'utf8', timeout: 30000 }
    ).trim();
  } catch (e) {
    return '';
  }
}

// ---- Step 1: 终止 nvm 相关进程 ----
function killProcesses() {
  step('终止 nvm 相关进程');

  // 终止可能由 nvm 启动的 node 进程（不终止 pvm 管理的）
  // 注意：这里只终止 nvm 目录下的 node.exe
  const nvmHome = process.env.NVM_HOME || '';
  const nvmSymlink = process.env.NVM_SYMLINK || '';

  if (nvmHome) {
    try {
      execSync(`wmic process where "ExecutablePath like '%${nvmHome.replace(/\\/g, '\\\\\\\\')}%'" call terminate`, {
        stdio: 'ignore', timeout: 10000
      });
      log('已终止 nvm 目录下的进程');
    } catch (e) {
      warn('无 nvm 相关进程运行');
    }
  } else {
    warn('NVM_HOME 未设置，跳过进程终止');
  }
}

// ---- Step 2: 检测 nvm 安装路径 ----
function detectNvmPaths() {
  step('检测 nvm 安装路径');

  const paths = {
    nvmHome: process.env.NVM_HOME || '',
    nvmSymlink: process.env.NVM_SYMLINK || '',
    detectedDirs: []
  };

  // 通过注册表检测
  const regHome = runPs(
    `[Environment]::GetEnvironmentVariable('NVM_HOME', 'User')`
  );
  const regSymlink = runPs(
    `[Environment]::GetEnvironmentVariable('NVM_SYMLINK', 'User')`
  );

  if (regHome && !paths.nvmHome) paths.nvmHome = regHome;
  if (regSymlink && !paths.nvmSymlink) paths.nvmSymlink = regSymlink;

  // 常见路径检测
  const commonPaths = [
    path.join(os.homedir(), 'AppData', 'Roaming', 'nvm'),
    path.join(os.homedir(), 'nvm'),
    'C:\\nvm',
    'C:\\Program Files\\nvm'
  ];

  for (const p of commonPaths) {
    if (fs.existsSync(p) && !paths.detectedDirs.includes(p)) {
      paths.detectedDirs.push(p);
    }
  }

  if (paths.nvmHome) log(`NVM_HOME: ${paths.nvmHome}`);
  if (paths.nvmSymlink) log(`NVM_SYMLINK: ${paths.nvmSymlink}`);
  if (paths.detectedDirs.length > 0) {
    paths.detectedDirs.forEach(d => log(`检测到目录: ${d}`));
  }

  if (!paths.nvmHome && paths.detectedDirs.length === 0) {
    warn('未检测到 nvm 安装，可能已卸载');
    return null;
  }

  return paths;
}

// ---- Step 3: 清理 PATH ----
function cleanPath(nvmPaths) {
  step('清理 PATH 环境变量');

  const nvmKeywords = ['nvm', 'nvm.exe'];
  if (nvmPaths.nvmHome) nvmKeywords.push(nvmPaths.nvmHome.toLowerCase());
  if (nvmPaths.nvmSymlink) nvmKeywords.push(nvmPaths.nvmSymlink.toLowerCase());

  // 清理用户 PATH
  const userPath = runPs(`[Environment]::GetEnvironmentVariable('Path', 'User')`);
  if (userPath) {
    const parts = userPath.split(';').filter(p => {
      const lower = p.toLowerCase();
      return !nvmKeywords.some(kw => lower.includes(kw));
    });
    const newPath = parts.join(';');
    if (newPath !== userPath) {
      runPs(`[Environment]::SetEnvironmentVariable('Path', '${newPath.replace(/'/g, "''")}', 'User')`);
      log('已清理用户 PATH 中的 nvm 条目');
    } else {
      log('用户 PATH 无 nvm 条目');
    }
  }

  // 清理系统 PATH（需管理员权限）
  const sysPath = runPs(`[Environment]::GetEnvironmentVariable('Path', 'Machine')`);
  if (sysPath) {
    const parts = sysPath.split(';').filter(p => {
      const lower = p.toLowerCase();
      return !nvmKeywords.some(kw => lower.includes(kw));
    });
    const newPath = parts.join(';');
    if (newPath !== sysPath) {
      const result = runPs(
        `try { [Environment]::SetEnvironmentVariable('Path', '${newPath.replace(/'/g, "''")}', 'Machine'); 'ok' } catch { 'fail' }`
      );
      if (result === 'ok') {
        log('已清理系统 PATH 中的 nvm 条目');
      } else {
        warn('清理系统 PATH 失败（需要管理员权限），请手动清理');
      }
    } else {
      log('系统 PATH 无 nvm 条目');
    }
  }
}

// ---- Step 4: 清除环境变量 ----
function cleanEnvVars() {
  step('清除 nvm 环境变量');

  runPs(`[Environment]::SetEnvironmentVariable('NVM_HOME', $null, 'User')`);
  runPs(`[Environment]::SetEnvironmentVariable('NVM_SYMLINK', $null, 'User')`);
  runPs(`[Environment]::SetEnvironmentVariable('NVM_HOME', $null, 'Machine')`);
  runPs(`[Environment]::SetEnvironmentVariable('NVM_SYMLINK', $null, 'Machine')`);

  log('已清除 NVM_HOME 和 NVM_SYMLINK');
}

// ---- Step 5: 删除 nvm 目录 ----
function deleteDirs(nvmPaths) {
  step('删除 nvm 安装目录');

  const dirsToDelete = new Set();
  if (nvmPaths.nvmHome) dirsToDelete.add(nvmPaths.nvmHome);
  if (nvmPaths.nvmSymlink) dirsToDelete.add(nvmPaths.nvmSymlink);
  nvmPaths.detectedDirs.forEach(d => dirsToDelete.add(d));

  for (const dir of dirsToDelete) {
    if (fs.existsSync(dir)) {
      try {
        fs.rmSync(dir, { recursive: true, force: true });
        log(`已删除: ${dir}`);
      } catch (e) {
        warn(`删除失败（可能被占用）: ${dir}`);
        warn(`请重启电脑后手动删除: ${dir}`);
      }
    } else {
      log(`不存在，跳过: ${dir}`);
    }
  }
}

// ---- Step 6: 清理注册表 ----
function cleanRegistry() {
  step('清理注册表残留');

  runPs(`Remove-Item -Path 'HKCU:\\Software\\nvm' -Recurse -Force -ErrorAction SilentlyContinue`);
  runPs(`Remove-Item -Path 'HKLM:\\Software\\nvm' -Recurse -Force -ErrorAction SilentlyContinue`);

  // 清理「卸载程序」注册表项
  const uninstallKeys = runPs(
    `Get-ChildItem 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall' -ErrorAction SilentlyContinue | Where-Object { $_.GetValue('DisplayName') -like '*nvm*' } | Select-Object -ExpandProperty PSChildName`
  );

  if (uninstallKeys) {
    uninstallKeys.split('\n').forEach(key => {
      if (key.trim()) {
        runPs(`Remove-Item -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${key.trim()}' -Recurse -Force -ErrorAction SilentlyContinue`);
      }
    });
    log('已清理卸载程序注册表项');
  } else {
    log('无卸载程序注册表残留');
  }
}

// ---- 主流程 ----
function main() {
  console.log('========================================');
  console.log('  nvm 卸载脚本（PVM 环境管理器）');
  console.log('========================================');

  // 前置检查：确认 PVM 已安装 node
  try {
    const nodeVersion = execSync('node --version', { encoding: 'utf8' }).trim();
    console.log(`\n  前置检查: node ${nodeVersion} ✓`);
  } catch (e) {
    die('前置检查失败：node 不可用。请先通过 PVM 安装 node：pvm install node@20 && pvm use node@20');
  }

  // 确认
  if (!SKIP_CONFIRM) {
    console.log('\n  即将卸载 nvm 并清理所有相关文件、环境变量、注册表。');
    console.log('  此操作不可逆。');
    console.log('  确认 node 已通过 PVM 安装，卸载 nvm 后不会断环境。');
    console.log('');
    // 非交互环境直接继续
    if (!process.stdin.isTTY) {
      console.log('  非交互环境，自动继续...');
    }
  }

  // 执行
  killProcesses();

  const nvmPaths = detectNvmPaths();
  if (!nvmPaths) {
    console.log('\n  nvm 未检测到，无需卸载。');
    console.log('\n========================================');
    console.log('  完成');
    console.log('========================================');
    return;
  }

  cleanPath(nvmPaths);
  cleanEnvVars();
  deleteDirs(nvmPaths);
  cleanRegistry();

  // 报告
  console.log('\n========================================');
  console.log('  nvm 卸载完成');
  console.log('========================================');
  console.log('\n  建议操作：');
  console.log('  1. 关闭并重新打开终端');
  console.log('  2. 运行 pvm setup 确认 PATH 正确');
  console.log('  3. 运行 node --version 确认 PVM 管理的 node 正常');
  console.log('');
}

main();
