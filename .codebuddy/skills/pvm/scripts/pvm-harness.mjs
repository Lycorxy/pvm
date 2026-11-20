#!/usr/bin/env node
/**
 * pvm-harness.mjs — pvm 环境守护 harness（跨平台，零依赖，node ≥ 16）
 *
 * 用法：
 *   node pvm-harness.mjs check                     # 环境检测（5 项，exit 0=全过，1=有失败）
 *   node pvm-harness.mjs fix path                  # 修复 PATH（pvm setup-path）
 *   node pvm-harness.mjs fix setup                 # 首次设置（pvm setup：目录+PATH+shims）
 *   node pvm-harness.mjs fix install               # 安装 pvm（官方脚本，国内镜像优先）
 *   node pvm-harness.mjs config read               # 读取 .pvmrc / pvm.yaml（JSON 输出）
 *   node pvm-harness.mjs config write-pvmrc rt@v…  # 写入 .pvmrc（rt@version 或 rt@system），写后验证
 *   node pvm-harness.mjs config validate           # 验证 .pvmrc / pvm.yaml 可被 pvm 解析
 *
 * 输出规范：[n] 检测项 → ✅ 结果 / ❌ 原因 → 下一步；模型只分析输出与选择下一步。
 * 常量理由：EXEC_TIMEOUT=60s（doctor 单次体检足够）；INSTALL_TIMEOUT=600s（下载 pvm 发行包）。
 */
import { execSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const IS_WIN = process.platform === 'win32';
const HOME = os.homedir();
const PVM_HOME = process.env.PVM_HOME || path.join(HOME, '.pvm');
const PVM_BIN = path.join(PVM_HOME, 'bin', IS_WIN ? 'pvm.exe' : 'pvm');
const SHIMS = path.join(PVM_HOME, 'shims');
const CWD = process.cwd();
const EXEC_TIMEOUT = 60_000;
const INSTALL_TIMEOUT = 600_000;
// 官方安装脚本：gitee 镜像优先（国内快），github 国际源备用
const INSTALL_PS = 'https://gitee.com/lycorxy/pvm/raw/master/scripts/install.ps1';
const INSTALL_SH = 'https://gitee.com/lycorxy/pvm/raw/master/scripts/install.sh';

// ---------- 基础工具 ----------
function run(cmd, { timeout = EXEC_TIMEOUT } = {}) {
  try {
    const out = execSync(cmd, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout, cwd: CWD });
    return { ok: true, out: (out || '').trim() };
  } catch (e) {
    return { ok: false, out: ((e.stdout || '') + (e.stderr || '')).trim() };
  }
}

function which(name) {
  const r = run(IS_WIN ? `where ${name}` : `command -v ${name}`);
  return r.ok ? r.out.split(/\r?\n/)[0].trim() : null;
}

// 输出策略：全绿只打一行；有失败才逐项展开（含下一步）
let total = 0, failed = 0, skipped = 0;
const okParts = [];
const failLines = [];

function report(name, ok, detail, next, okText) {
  total++;
  if (ok) {
    okParts.push(okText || name);
    return;
  }
  failed++;
  failLines.push(`❌ ${name}: ${detail}`);
  if (next) failLines.push(`   下一步: ${next}`);
}
function skip(name, reason) {
  total++; skipped++;
  okParts.push(`${name} 跳过`);
}
function summary() {
  if (failed === 0) {
    console.log(`✅ ${okParts.join(' | ')}`);
  } else {
    console.log(`❌ ${failed}/${total} 项失败`);
    for (const l of failLines) console.log(l);
    if (skipped) console.log(`   （${skipped} 项跳过）`);
  }
  process.exit(failed > 0 ? 1 : 0);
}

// pvm.yaml 解析失败时区分归因：Unknown command = pvm 过旧；其余才是 yaml 格式问题
function pipelineFailNext(out) {
  if (/unknown command/i.test(out)) return 'pvm self-update 或重装 pvm（当前 pvm 过旧，无 run/pkg 命令），完成后重验';
  return '修正 pvm.yaml（规则只有四条：名字: 命令；名字: + 列表 = 顺序；parallel: + 列表 = 并行；步骤名等于流水线名 = 引用），再 config validate';
}

// ---------- check：环境检测 ----------
function cmdCheck() {
  // 1) pvm 安装
  const v = run('pvm --version');
  const installed = v.ok;
  const binExists = fs.existsSync(PVM_BIN);
  if (installed) {
    report('pvm 安装', true, '', null, v.out);
  } else if (binExists) {
    report('pvm 安装', false, `pvm 在 ${PVM_BIN}，但 PATH 中不可用`, 'node pvm-harness.mjs fix path，完成后新开终端再 check');
  } else {
    report('pvm 安装', false, '未安装 pvm', 'node pvm-harness.mjs fix install');
  }

  // pvm 不可用时，依赖 pvm 的项全部跳过
  if (!installed) {
    skip('shims PATH', 'pvm 未安装');
    skip('nvm 干扰', 'pvm 未安装');
    skip('pvm doctor', 'pvm 未安装');
    skip('项目配置', 'pvm 未安装');
    return summary();
  }

  // 2) shims 目录在 PATH（Windows 忽略大小写与结尾分隔符）
  const norm = (d) => d.toLowerCase().replace(/[\\/]+$/, '');
  const pathDirs = (process.env.PATH || '').split(path.delimiter).filter(Boolean).map(norm);
  const shimsOk = pathDirs.includes(norm(SHIMS));
  report('shims PATH', shimsOk, `${SHIMS} 不在 PATH（node 等命令不会被 pvm 接管）`,
    shimsOk ? null : 'node pvm-harness.mjs fix path，完成后新开终端再 check', 'shims ok');

  // 3) nvm 干扰：nvm 存在时，node 必须由 pvm 接管（一切以 pvm 为主）
  const nvmExists = IS_WIN
    ? Boolean(process.env.NVM_HOME || which('nvm'))
    : Boolean(process.env.NVM_DIR || fs.existsSync(path.join(HOME, '.nvm')));
  if (!nvmExists) {
    report('nvm 干扰', true, '', null, 'nvm ok');
  } else {
    const w = run('pvm which node');
    if (!w.ok) {
      report('nvm 干扰', false, 'nvm 存在，且 node 尚未由 pvm 管理', 'pvm install node@lts（装完由 shims 自动接管）');
    } else if (/[\\/](nvm|NVM)/.test(w.out)) {
      report('nvm 干扰', false, `nvm 正在劫持 node：${w.out}`, 'node pvm-harness.mjs fix path 前置 pvm shims，新开终端后重试');
    } else {
      report('nvm 干扰', true, '', null, 'nvm ok（已接管）');
    }
  }

  // 4) pvm doctor 综合体检
  const d = run('pvm doctor');
  if (d.ok) {
    report('pvm doctor', true, '', null, 'doctor ok');
  } else {
    const lastLines = d.out.split(/\r?\n/).filter(Boolean).slice(-3).join(' | ');
    report('pvm doctor', false, `体检未全过：${lastLines || '无输出（看上方 doctor 原始信息）'}`,
      '先试 pvm validate --auto-fix，仍失败则 node pvm-harness.mjs fix setup');
  }

  // 5) 项目配置（.pvmrc / pvm.yaml）：缺失只提示，存在但解析失败才算病
  const pvmrc = path.join(CWD, '.pvmrc');
  const pipeline = path.join(CWD, 'pvm.yaml');
  if (fs.existsSync(pvmrc)) {
    const c = run('pvm current');
    if (c.ok) {
      report('项目配置', true, '', null, '.pvmrc ok');
    } else {
      report('项目配置', false, '.pvmrc 存在但 pvm 解析失败',
        '检查格式（每行 "runtime version"，如 "node 20.20.2"），或 pvm config show 对照');
    }
  } else {
    report('项目配置', true, '', null, '.pvmrc 无');
  }
  if (fs.existsSync(pipeline)) {
    const l = run('pvm run --list');
    const n = l.out.split(/\r?\n/).filter(Boolean).length;
    if (l.ok) {
      report('pvm.yaml', true, '', null, `pvm.yaml ${n} 行清单`);
    } else {
      report('pvm.yaml', false,
        `解析失败：${l.out.split(/\r?\n/).filter(Boolean).slice(-2).join(' | ')}`,
        pipelineFailNext(l.out));
    }
  } else {
    report('pvm.yaml', true, '', null, 'pvm.yaml 无');
  }
  summary();
}

// ---------- fix：修复动作 ----------
function pvmCmd(sub) {
  // 优先 PATH 中的 pvm；PATH 不可用但 ~/.pvm/bin 有二进制时用绝对路径兜底
  if (run('pvm --version').ok) return run(`pvm ${sub}`);
  if (fs.existsSync(PVM_BIN)) return run(`"${PVM_BIN}" ${sub}`);
  return { ok: false, out: 'pvm 不可用（未安装且 ~/.pvm/bin 无二进制）' };
}

function cmdFix(what) {
  if (what === 'path' || what === 'setup') {
    const r = pvmCmd(what === 'path' ? 'setup-path' : 'setup');
    console.log(`[fix ${what}] ${r.ok ? '✅ 执行成功' : `❌ ${r.out.split(/\r?\n/).filter(Boolean).slice(-2).join(' | ')}`}`);
    console.log(r.ok ? '下一步: 新开终端（PATH 需重新加载），然后 node pvm-harness.mjs check' : '下一步: pvm 不可用，先 node pvm-harness.mjs fix install');
    process.exit(r.ok ? 0 : 1);
  }
  if (what === 'install') {
    if (run('pvm --version').ok) {
      console.log('[fix install] ✅ pvm 已安装，无需重装（如需强制重装: pvm self-update / --force 安装脚本）');
      process.exit(0);
    }
    console.log('[fix install] 运行官方安装脚本（gitee 镜像优先，失败可换 github.com/Lycorxy/pvm 源）…');
    const r = IS_WIN
      ? run(`powershell -NoProfile -ExecutionPolicy Bypass -Command "irm ${INSTALL_PS} | iex"`, { timeout: INSTALL_TIMEOUT })
      : run(`curl -fsSL ${INSTALL_SH} | bash`, { timeout: INSTALL_TIMEOUT });
    console.log(r.ok ? '[fix install] ✅ 安装脚本执行成功' : `[fix install] ❌ ${r.out.split(/\r?\n/).filter(Boolean).slice(-3).join(' | ') || '安装脚本失败'}`);
    console.log('下一步: 新开终端使 PATH 生效，然后 node pvm-harness.mjs check');
    process.exit(r.ok ? 0 : 1);
  }
  console.log('用法: node pvm-harness.mjs fix path|setup|install');
  process.exit(2);
}

// ---------- config：.pvmrc / pvm.yaml 读写验证 ----------
function cmdConfig(args) {
  const [action, ...rest] = args;
  const pvmrc = path.join(CWD, '.pvmrc');
  const pipeline = path.join(CWD, 'pvm.yaml');

  if (action === 'read') {
    const read = (p) => fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : null;
    console.log(JSON.stringify({
      cwd: CWD,
      pvmrc: { exists: fs.existsSync(pvmrc), path: pvmrc, content: read(pvmrc) },
      pipeline: { exists: fs.existsSync(pipeline), path: pipeline, content: read(pipeline) },
    }, null, 2));
    process.exit(0);
  }

  if (action === 'write-pvmrc') {
    if (!rest.length) { console.log('用法: config write-pvmrc node@20.20.2 pnpm@8.15.9（rt@system 表示用系统版本）'); process.exit(2); }
    const lines = [];
    for (const item of rest) {
      const m = /^([a-z][a-z0-9-]*)@(.+)$/i.exec(item);
      if (!m) { console.log(`❌ 参数 "${item}" 格式错误，应为 runtime@version`); process.exit(2); }
      const [, rt, ver] = m;
      if (!ver || !/^[a-z0-9][\w.+\-]*$/i.test(ver)) { console.log(`❌ ${item} 版本号非法`); process.exit(2); }
      lines.push(`${rt} ${ver}`); // .pvmrc 由 pvm 整体重写，纯条目即可
    }
    const old = fs.existsSync(pvmrc) ? fs.readFileSync(pvmrc, 'utf8') : '(不存在，将新建)';
    fs.writeFileSync(pvmrc, lines.join('\n') + '\n', 'utf8');
    console.log(`[write-pvmrc] 旧内容: ${old.trim().replace(/\n/g, ' | ')}`);
    console.log(`[write-pvmrc] ✅ 已写入 ${lines.length} 条 → ${pvmrc}`);
    const c = run('pvm current');
    console.log(c.ok ? '[write-pvmrc] ✅ pvm 解析通过' : `[write-pvmrc] ❌ pvm 解析失败: ${c.out.split(/\r?\n/).filter(Boolean).slice(-2).join(' | ')}`);
    process.exit(c.ok ? 0 : 1);
  }

  if (action === 'validate') {
    // 同样一行：全绿一行，失败逐项
    const ok = [];
    const bad = [];
    if (fs.existsSync(pvmrc)) {
      const c = run('pvm current');
      if (c.ok) ok.push('.pvmrc ok');
      else {
        bad.push(`❌ .pvmrc 解析失败: ${c.out.split(/\r?\n/).filter(Boolean).slice(-2).join(' | ')}`);
        bad.push('   下一步: 每行 "runtime version"（如 node 20.20.2），或 pvm config show 对照');
      }
    } else {
      ok.push('.pvmrc 无');
    }
    if (fs.existsSync(pipeline)) {
      const l = run('pvm run --list');
      if (l.ok) ok.push('pvm.yaml ok');
      else {
        bad.push(`❌ pvm.yaml 解析失败: ${l.out.split(/\r?\n/).filter(Boolean).slice(-2).join(' | ')}`);
        bad.push(`   下一步: ${pipelineFailNext(l.out)}`);
      }
    } else {
      ok.push('pvm.yaml 无');
    }
    if (bad.length === 0) {
      console.log(`✅ ${ok.join(' | ')}`);
    } else {
      console.log(`❌ ${bad.filter((l) => l.startsWith('❌')).length} 项失败`);
      for (const l of bad) console.log(l);
    }
    process.exit(bad.length > 0 ? 1 : 0);
  }

  console.log('用法: node pvm-harness.mjs config read|write-pvmrc|validate');
  process.exit(2);
}

// ---------- 入口 ----------
const [cmd, ...rest] = process.argv.slice(2);
switch (cmd) {
  case 'check': cmdCheck(); break;
  case 'fix': cmdFix(rest[0]); break;
  case 'config': cmdConfig(rest); break;
  default:
    console.log('用法: node pvm-harness.mjs check | fix path|setup|install | config read|write-pvmrc|validate');
    process.exit(2);
}
