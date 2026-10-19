#!/usr/bin/env node
/**
 * check-env.mjs — 环境检测流程客观校验（路径1·脚本 exit code·100% 确定）
 *
 * GateLoop 各 anchor 的 custom 校验器，按 --phase 分发：
 *   env       git 可用 + pvm 可用（PATH 或 ~/.pvm/bin 绝对路径兜底）
 *   repo      repo-report 的 repoPath 是 git 仓库，当前分支 === report.branch
 *   toolchain toolchain-report 声明的每个 runtime 版本命令 exit 0 且版本一致
 *   deps      deps-report 的每个 installs[].dir 下 node_modules 存在
 *   run       run-report：mode=http 真实请求 url（<500）；mode=build 重跑 buildCommand exit 0
 *
 * 用法：node check-env.mjs --phase <env|repo|toolchain|deps|run> --report <path>
 * exit 0 = 通过，exit 1 = 失败（附缺失项清单）
 */
import { readFileSync, existsSync } from 'node:fs'
import { execSync } from 'node:child_process'
import { homedir } from 'node:os'
import { join } from 'node:path'

function parseArgs(argv) {
    const args = { phase: '', report: '' }
    for (let i = 2; i < argv.length; i++) {
        if (argv[i] === '--phase') args.phase = argv[++i]
        else if (argv[i] === '--report') args.report = argv[++i]
    }
    return args
}

/** 执行命令，返回 {ok, output}（不抛异常） */
function tryCmd(cmd, cwd, timeout = 30000) {
    try {
        const output = execSync(cmd, {
            cwd,
            stdio: 'pipe',
            timeout,
            encoding: 'utf-8',
            env: { ...process.env, FORCE_COLOR: '0' }
        })
        return { ok: true, output: (output || '').trim() }
    } catch (e) {
        const err = e.stdout || e.stderr || e.message || ''
        return { ok: false, output: err.trim() }
    }
}

/** pvm 可用性：PATH 优先，绝对路径兜底（刚安装 PATH 未刷新场景） */
function pvmAvailable() {
    const direct = tryCmd('pvm version')
    if (direct.ok) return { ok: true, output: direct.output, via: 'PATH' }
    const ext = process.platform === 'win32' ? '.exe' : ''
    const abs = join(homedir(), '.pvm', 'bin', `pvm${ext}`)
    const absRun = tryCmd(`"${abs}" version`)
    if (absRun.ok) return { ok: true, output: absRun.output, via: abs }
    return { ok: false, output: `${direct.output} | ${absRun.output}`, via: 'none' }
}

function loadReport(path) {
    if (!path || !existsSync(path)) {
        console.error(`[check-env] FAIL: 报告文件不存在: ${path}`)
        process.exit(1)
    }
    try {
        return JSON.parse(readFileSync(path, 'utf-8'))
    } catch (e) {
        console.error(`[check-env] FAIL: 报告 JSON 解析失败: ${e.message}`)
        process.exit(1)
    }
}

// ---------- phase: env ----------
function checkEnv(errors) {
    const git = tryCmd('git --version')
    if (!git.ok) errors.push(`git: 不可用（git --version 失败: ${git.output.slice(0, 80)}）`)
    const pvm = pvmAvailable()
    if (!pvm.ok) errors.push(`pvm: 不可用（PATH 与 ~/.pvm/bin 均失败: ${pvm.output.slice(0, 80)}）`)
}

// ---------- phase: repo ----------
function checkRepo(report, errors) {
    const { repoPath, branch } = report
    if (!repoPath || !branch) {
        errors.push('repo-report: 缺 repoPath/branch')
        return
    }
    if (!existsSync(repoPath)) {
        errors.push(`repo: 仓库路径不存在: ${repoPath}`)
        return
    }
    const cur = tryCmd('git branch --show-current', repoPath)
    if (!cur.ok) errors.push(`repo: ${repoPath} 不是可用的 git 仓库`)
    else if (cur.output !== branch) {
        errors.push(`repo: 当前分支 ${cur.output} ≠ 报告声明分支 ${branch}（须在修复分支上）`)
    }
    const remote = tryCmd('git remote get-url origin', repoPath)
    if (!remote.ok) errors.push('repo: 无 origin 远端（未正确拉取代码）')
}

// ---------- phase: toolchain ----------
const RT_CMDS = {
    node: 'node --version',
    pnpm: 'pnpm --version',
    go: 'go version',
    python: 'python --version',
    deno: 'deno --version'
}

function checkToolchain(report, errors) {
    const runtimes = report.runtimes || []
    if (!Array.isArray(runtimes) || runtimes.length === 0) {
        errors.push('toolchain-report: runtimes 为空（至少声明一个运行时）')
        return
    }
    for (const rt of runtimes) {
        const { name, version } = rt
        const cmd = RT_CMDS[name]
        if (!cmd) {
            errors.push(`toolchain: 未知运行时 ${name}（支持: ${Object.keys(RT_CMDS).join('/')}）`)
            continue
        }
        const r = tryCmd(cmd)
        if (!r.ok) {
            errors.push(`toolchain: ${name} 不可用（${cmd} 失败: ${r.output.slice(0, 80)}）`)
            continue
        }
        // 版本一致性：输出应包含声明版本（去 v 前缀宽松匹配，如 20.11.0 匹配 v20.11.0）
        if (version && version !== 'system') {
            const bare = String(version).replace(/^v/, '')
            if (!r.output.includes(bare)) {
                errors.push(
                    `toolchain: ${name} 版本不一致（声明 ${version}，实际输出: ${r.output.slice(0, 60)}）`
                )
            }
        }
    }
}

// ---------- phase: deps ----------
function checkDeps(report, errors) {
    const installs = report.installs || []
    if (!Array.isArray(installs) || installs.length === 0) {
        errors.push('deps-report: installs 为空（至少一处 pnpm i）')
        return
    }
    for (const ins of installs) {
        const dir = ins.dir || ''
        if (!dir) {
            errors.push('deps: installs 项缺 dir')
            continue
        }
        if (ins.ok === false) {
            errors.push(`deps: ${dir} 声明安装失败（ok=false）`)
            continue
        }
        if (!existsSync(join(dir, 'node_modules'))) {
            errors.push(`deps: ${dir} 下 node_modules 不存在（pnpm i 未真正完成）`)
        }
    }
}

// ---------- phase: run ----------
async function checkRun(report, errors) {
    const { mode, url, buildCommand, repoPath } = report
    if (mode === 'http') {
        if (!url) {
            errors.push('run: mode=http 但缺 url')
            return
        }
        try {
            const res = await fetch(url, { signal: AbortSignal.timeout(15000) })
            if (res.status >= 500) {
                errors.push(`run: HTTP ${res.status} ≥ 500（${url} 服务异常）`)
            }
        } catch (e) {
            errors.push(`run: 请求失败 ${url}（${e.message}）——mode=http 须保持 dev server 运行到 verify 通过`)
        }
    } else if (mode === 'build') {
        if (!buildCommand) {
            errors.push('run: mode=build 但缺 buildCommand')
            return
        }
        const r = tryCmd(buildCommand, repoPath || process.cwd(), 300000)
        if (!r.ok) errors.push(`run: 构建失败（${buildCommand}: ${r.output.slice(0, 120)}）`)
    } else {
        errors.push(`run: mode 非法（${mode}，应为 http|build）`)
    }
}

async function main() {
    const args = parseArgs(process.argv)
    const errors = []
    if (!args.phase) {
        console.error('[check-env] FAIL: 缺 --phase 参数')
        process.exit(1)
    }

    switch (args.phase) {
        case 'env':
            checkEnv(errors)
            break
        case 'repo':
        case 'toolchain':
        case 'deps':
        case 'run': {
            const report = loadReport(args.report)
            if (args.phase === 'repo') checkRepo(report, errors)
            else if (args.phase === 'toolchain') checkToolchain(report, errors)
            else if (args.phase === 'deps') checkDeps(report, errors)
            else await checkRun(report, errors)
            break
        }
        default:
            console.error(`[check-env] FAIL: 未知 phase ${args.phase}`)
            process.exit(1)
    }

    if (errors.length > 0) {
        console.error(`[check-env] FAIL: ${args.phase} 阶段未通过（${errors.length} 项）`)
        errors.forEach((e) => console.error('  ✖', e))
        process.exit(1)
    }
    console.log(`[check-env] PASS: ${args.phase} 阶段客观校验通过`)
    process.exit(0)
}

main()
