#!/usr/bin/env node
/**
 * pvm-release harness — 发版流程检测与验证（零依赖，node >= 16）
 *
 * 用法：
 *   node release-harness.cjs init            # 初始化 ~/.release.yaml（交互式，粘贴令牌）
 *   node release-harness.cjs check-tag       # 发版前置检查：tag 冲突 + 工作区状态
 *   node release-harness.cjs push            # 双远端推送（GitHub origin + Gitee gitee）
 *   node release-harness.cjs actions-status  # 最新 workflow run 状态
 *   node release-harness.cjs sync-gitee      # 同步本地 dist 资产到 Gitee Release
 *   node release-harness.cjs verify          # 双平台资产验证（发布闭环）
 *
 * 选项：--version x.y.z  --platform windows-amd64  --notes "发布说明"  --repo Lycorxy/pvm
 *
 * 配置：~/.release.yaml（由 init 生成）：
 *   github: { token, repo }   gitee: { token, repo, remote }
 *
 * 退出码：0 = 全部通过；1 = 有失败项；2 = 用法/环境错误
 */
"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const cp = require("child_process");
const crypto = require("crypto");
const https = require("https");
const readline = require("readline");

// ---------- 基础路径 ----------
const SCRIPT_DIR = __dirname;                 // <root>/.codebuddy/skills/release/scripts
const PROJECT_ROOT = path.resolve(SCRIPT_DIR, "..", "..", "..", "..");
const DIST_DIR = path.join(PROJECT_ROOT, "dist");
const PACKAGES_DIR = path.join(DIST_DIR, "packages");
const CONFIG_PATH = path.join(os.homedir(), ".release.yaml");

// ---------- 参数解析 ----------
const argv = process.argv.slice(2);
const action = argv[0] && !argv[0].startsWith("--") ? argv[0] : "";
const opts = { version: "", platform: "windows-amd64", notes: "", repo: "Lycorxy/pvm" };
for (let i = 1; i < argv.length; i++) {
  const a = argv[i];
  if (a === "--version") opts.version = argv[++i] || "";
  else if (a === "--platform") opts.platform = argv[++i] || "windows-amd64";
  else if (a === "--notes") opts.notes = argv[++i] || "";
  else if (a === "--repo") opts.repo = argv[++i] || "Lycorxy/pvm";
}

let pass = 0, fail = 0;
function report(name, ok, detail) {
  if (ok) { pass++; console.log(`  ${name} -> OK ${detail || ""}`); }
  else { fail++; console.log(`  ${name} -> FAIL ${detail || ""}`); }
}
function finish() {
  console.log(`\n结果: ${pass} 通过, ${fail} 失败`);
  process.exit(fail === 0 ? 0 : 1);
}
function git(args, opts2 = {}) {
  return cp.execSync(`git ${args}`, { cwd: PROJECT_ROOT, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"], ...opts2 }).trim();
}
function gitOk(args) {
  try { git(args); return true; } catch { return false; }
}

// ---------- HTTP（跟随重定向；返回 { status, headers, body(Buffer) }） ----------
function httpGet(url, { headers = {}, method = "GET", body = null, maxRedirects = 5 } = {}) {
  return new Promise((resolve, reject) => {
    const u = new URL(url);
    const req = https.request(u, {
      method,
      headers: { "User-Agent": "pvm-release-harness", ...headers },
    }, (res) => {
      if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location && maxRedirects > 0) {
        res.resume();
        const next = new URL(res.headers.location, u).toString();
        return resolve(httpGet(next, { headers, method, body, maxRedirects: maxRedirects - 1 }));
      }
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    req.on("error", reject);
    if (body) req.write(body);
    req.end();
  });
}

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

// ---------- 配置：~/.release.yaml ----------
function readConfig() {
  const cfg = { ghToken: "", ghRepo: "Lycorxy/pvm", giteeToken: "", giteeRepo: "lycorxy/pvm", giteeRemote: "https://gitee.com/lycorxy/pvm.git" };
  if (!fs.existsSync(CONFIG_PATH)) return cfg;
  let sec = null;
  for (const line of fs.readFileSync(CONFIG_PATH, "utf8").split(/\r?\n/)) {
    if (/^github:/.test(line)) sec = "gh";
    else if (/^gitee:/.test(line)) sec = "gt";
    else if (/^\s+token:\s*(.*)$/.test(line)) {
      const v = RegExp.$1.trim().replace(/^"|"$/g, "");
      if (sec === "gh") cfg.ghToken = v;
      else if (sec === "gt") cfg.giteeToken = v;
    } else if (/^\s+repo:\s*(.*)$/.test(line)) {
      const v = RegExp.$1.trim().replace(/^"|"$/g, "");
      if (sec === "gh") cfg.ghRepo = v;
      else if (sec === "gt") cfg.giteeRepo = v;
    } else if (/^\s+remote:\s*(.*)$/.test(line)) {
      if (sec === "gt") cfg.giteeRemote = RegExp.$1.trim().replace(/^"|"$/g, "");
    }
  }
  return cfg;
}

function readVersionFromFile() {
  const vf = path.join(PROJECT_ROOT, "VERSION");
  if (!fs.existsSync(vf)) return "";
  return fs.readFileSync(vf, "utf8").trim();
}

// ---------- 分发 ----------
const commands = { init, "check-tag": checkTag, push, "actions-status": actionsStatus, "sync-gitee": syncGitee, verify };

const fn = commands[action];
if (!fn) {
  console.log("用法: node release-harness.cjs init|check-tag|push|actions-status|sync-gitee|verify");
  console.log("选项: --version x.y.z  --platform windows-amd64  --notes \"说明\"  --repo Lycorxy/pvm");
  process.exit(2);
}
(async () => { await fn(); })();

// ==================== init ====================
async function init() {
  const cfg = readConfig();
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  const ask = (q, def) => new Promise((res) => rl.question(q, (a) => res(a.trim() || def)));
  console.log("=== pvm 发版环境初始化 ===");
  console.log(`配置将保存到: ${CONFIG_PATH}（用户主目录，不进任何仓库；回车保留原值）`);

  cfg.ghToken = await ask(`GitHub Personal Access Token ${cfg.ghToken ? "[已配置]" : ""}: `, cfg.ghToken);
  cfg.giteeToken = await ask(`Gitee 私人令牌 ${cfg.giteeToken ? "[已配置]" : ""}: `, cfg.giteeToken);
  cfg.giteeRemote = await ask(`Gitee 仓库地址 [${cfg.giteeRemote}]: `, cfg.giteeRemote);

  const yaml = [
    "# pvm 发版配置（由 release-harness.cjs init 生成）",
    "# 含令牌：请勿提交到任何仓库、请勿分享",
    "github:",
    `  token: "${cfg.ghToken}"`,
    `  repo: "${cfg.ghRepo}"`,
    "gitee:",
    `  token: "${cfg.giteeToken}"`,
    `  repo: "${cfg.giteeRepo}"`,
    `  remote: "${cfg.giteeRemote}"`,
    "",
  ].join("\n");
  fs.writeFileSync(CONFIG_PATH, yaml, "utf8");
  console.log(`OK 配置已写入 ${CONFIG_PATH}`);

  const remotes = (() => { try { return git("remote"); } catch { return ""; } })();
  if (/(^|\s)gitee(\s|$)/.test(remotes)) {
    git(`remote set-url gitee "${cfg.giteeRemote}"`);
    console.log(`OK git remote gitee 已更新: ${cfg.giteeRemote}`);
  } else {
    git(`remote add gitee "${cfg.giteeRemote}"`);
    console.log(`OK git remote gitee 已添加: ${cfg.giteeRemote}`);
  }

  if (cfg.ghToken && cfg.giteeToken) {
    process.env.GH_TOKEN = cfg.ghToken;
    try {
      cp.execSync(`gh secret set GITEE_TOKEN --repo ${cfg.ghRepo} --body "${cfg.giteeToken}"`, { stdio: "pipe" });
      console.log("OK GitHub Actions Secret 已配置: GITEE_TOKEN（push tag 时 workflow 自动同步 Gitee）");
    } catch {
      console.log("WARN GitHub Secret 配置失败（gh 未登录或不可用）");
      console.log("  手动: gh auth login 后重跑 init；或网页 Settings -> Secrets -> Actions 添加 GITEE_TOKEN");
    }
  }
  console.log("\n=== 初始化完成 ===");
  rl.close();
}

// ==================== check-tag ====================
function checkTag() {
  const version = opts.version || readVersionFromFile();
  if (!version) { console.log("error: 未提供 --version 且未找到 VERSION 文件"); process.exit(2); }
  if (!/^\d+\.\d+\.\d+$/.test(version)) { console.log(`error: 版本号格式不正确: ${version}（应为 x.y.z）`); process.exit(2); }
  console.log(`[pvm-release check-tag] 目标版本: v${version}\n`);

  report("工作区干净", git("status --short").trim() === "",
    git("status --short").trim() ? "有未提交改动：" + git("status --short").split("\n").slice(0, 5).join("; ") : "");
  report(`本地 tag v${version} 不存在`, !git(`tag -l "v${version}"`).trim(),
    git(`tag -l "v${version}"`).trim() ? "已存在，强推需用户确认（git tag -f）" : "");

  const remote = (() => { try { return git(`ls-remote --tags origin "refs/tags/v${version}"`); } catch { return ""; } })();
  report(`远端 tag v${version} 不存在`, !remote.trim(),
    remote.trim() ? "远端已存在，强推需用户确认（会重新触发全量发布）" : "");

  try {
    git("fetch origin master --quiet");
    const ahead = git("rev-list --count origin/master..master");
    const behind = git("rev-list --count master..origin/master");
    report("master 与 origin/master 同步", ahead === "0" && behind === "0", `ahead=${ahead} behind=${behind}`);
  } catch (e) { report("master 同步检查", false, e.message); }

  finish();
}

// ==================== push：双远端推送 ====================
function push() {
  const cfg = readConfig();
  const remotes = git("remote");
  const hasGitee = /(^|\s)gitee(\s|$)/.test(remotes);

  console.log("  -> push origin master");
  git("push origin master");

  if (hasGitee) {
    console.log("  -> push gitee master");
    try {
      git("push gitee master");
    } catch (e) {
      console.log("FAIL gitee 推送失败（凭证问题可重跑 init 重新配置）");
      process.exit(1);
    }
  } else {
    console.log("  INFO 无 gitee 远端：运行 init 配置后自动双推");
  }
  console.log("OK 推送完成");
  process.exit(0);
}

// ==================== actions-status ====================
function actionsStatus() {
  const cfg = readConfig();
  if (cfg.ghToken && !process.env.GH_TOKEN) process.env.GH_TOKEN = cfg.ghToken;
  try {
    const out = cp.execSync(`gh auth status`, { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  } catch {
    console.log("  gh 不可用（未登录且 ~/.release.yaml 无 github.token）");
    console.log(`  请网页查看: https://github.com/${opts.repo}/actions`);
    console.log("  或运行 init 完成配置");
    process.exit(2);
  }
  const out = cp.execSync(`gh run list --repo ${opts.repo} --limit 3`, { encoding: "utf8" });
  console.log(out);
  process.exit(0);
}

// ==================== sync-gitee：本地 dist 资产同步到 Gitee Release ====================
function giteeApi(path2, token) { return `https://gitee.com/api/v5/repos/${opts.giteeRepo || readConfig().giteeRepo}${path2}`; }

async function syncGitee() {
  const cfg = readConfig();
  const token = cfg.giteeToken;
  if (!token) { console.log("error: 无 Gitee 令牌（运行 init 配置 ~/.release.yaml）"); process.exit(2); }
  const version = opts.version || readVersionFromFile();
  if (!version) { console.log("error: 未提供 --version 且未找到 VERSION 文件"); process.exit(2); }
  const tag = `v${version}`;

  // 收集资产
  const assets = [];
  for (const f of fs.readdirSync(DIST_DIR)) {
    if (/^pvm-(windows|darwin|linux)-(amd64|arm64)(\.exe)?$/.test(f)) assets.push(path.join(DIST_DIR, f));
  }
  if (fs.existsSync(PACKAGES_DIR)) {
    for (const f of fs.readdirSync(PACKAGES_DIR)) {
      if (f.startsWith(`pvm-${tag}-`) || f === "checksums.txt" || f === "pvm-env-manager.zip") {
        assets.push(path.join(PACKAGES_DIR, f));
      }
    }
  }
  if (!assets.length) { console.log("error: 未找到可上传的资产（先跑 build-all）"); process.exit(1); }
  console.log(`[pvm-release sync-gitee] ${tag}，资产 ${assets.length} 个`);

  const api = (p2) => `https://gitee.com/api/v5/repos/${cfg.giteeRepo}${p2}`;

  // 查询/创建 release
  let releaseId = "";
  const q = await httpGet(`${api(`/releases/tags/${tag}`)}?access_token=${token}`);
  if (q.status === 200) {
    releaseId = JSON.parse(q.body.toString("utf8")).id;
    console.log(`  Gitee release 已存在 (id=${releaseId})`);
  } else {
    const body = JSON.stringify({ access_token: token, tag_name: tag, name: `pvm ${tag}`, target_commitish: "master", body: opts.notes || `pvm ${tag}` });
    const r = await httpGet(`${api("/releases")}`, { method: "POST", headers: { "Content-Type": "application/json" }, body });
    if (r.status !== 201) { console.log(`FAIL 创建 Gitee release: HTTP ${r.status} ${r.body.toString("utf8").slice(0, 200)}`); process.exit(1); }
    releaseId = JSON.parse(r.body.toString("utf8")).id;
    console.log(`  已创建 Gitee release (id=${releaseId})`);
  }

  // 更新发布说明
  if (opts.notes) {
    await httpGet(`${api(`/releases/${releaseId}`)}?access_token=${token}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ access_token: token, body: opts.notes }),
    });
    console.log("  OK 发布说明已更新");
  }

  // 上传资产（multipart 用 curl 子进程，跨平台自带）
  let failCount = 0;
  for (const a of assets) {
    process.stdout.write(`  上传 ${path.basename(a)} ...`);
    try {
      cp.execSync(`curl -sS -X POST "${api(`/releases/${releaseId}/attach_files`)}?access_token=${token}" -F "file=@${a}"`, { stdio: "pipe" });
      console.log(" OK");
    } catch { console.log(" 失败"); failCount++; }
  }

  console.log(`\n${failCount ? `FAIL ${failCount} 个上传失败（重跑本命令可补传）` : `OK 同步完成: ${cfg.giteeRepo} @${tag}`}`);
  process.exit(failCount ? 1 : 0);
}

// ==================== verify：双平台资产验证 ====================
async function verify() {
  const version = opts.version || readVersionFromFile();
  if (!version) { console.log("error: 未提供 --version 且未找到 VERSION 文件"); process.exit(2); }
  const tag = `v${version}`;
  const platform = opts.platform;
  const ext = platform.startsWith("windows") ? ".exe" : "";
  const assetName = `pvm-${platform}${ext}`;
  const isWin = platform.startsWith("windows");
  console.log(`[pvm-release verify] ${tag} @ ${platform}\n`);

  // 本地基准
  let localSha = null;
  const localExe = path.join(DIST_DIR, assetName);
  if (fs.existsSync(localExe)) {
    localSha = sha256(localExe);
    report(`本地 dist 基准 (${assetName})`, true, `sha=${localSha.slice(0, 12)}...`);
  } else {
    console.log(`  INFO 本地无 dist/${assetName}，跳过 SHA 对比，仅验证远端 version`);
  }

  const tmp = path.join(os.tmpdir(), `pvm-verify-${platform}-${Date.now()}${ext}`);
  const sources = [
    { name: "GitHub", base: `https://github.com/${opts.repo}/releases/download/${tag}` },
    { name: "Gitee", base: `https://gitee.com/${readConfig().giteeRepo}/releases/download/${tag}` },
  ];
  const extras = ["checksums.txt", "pvm-env-manager.zip"];

  for (const src of sources) {
    console.log(`  --- ${src.name} ---`);
    try {
      const dl = await httpGet(`${src.base}/${assetName}?nocache=${Date.now()}`);
      if (dl.status !== 200) { report(`${src.name} ${assetName} 下载`, false, `HTTP ${dl.status}`); continue; }
      fs.writeFileSync(tmp, dl.body);
      const sha = sha256(tmp);

      if (isWin) {
        let ver = "";
        try { ver = cp.execFileSync(tmp, ["version"], { encoding: "utf8" }).split(/\r?\n/)[0]; }
        catch (e) { ver = "执行失败: " + e.message.slice(0, 80); }
        report(`${src.name} version 输出`, ver.includes(`pvm ${version}`), `ver=${ver}`);
      }

      if (localSha) report(`${src.name} SHA-256 与本地一致`, sha === localSha, `sha=${sha.slice(0, 12)}...`);

      for (const f of extras) {
        let ok = false;
        try { const h = await httpGet(`${src.base}/${f}`, { method: "HEAD" }); ok = h.status === 200; }
        catch { ok = false; }
        report(`${src.name} ${f}`, ok, ok ? "" : "缺失，请上传");
      }
    } catch (e) {
      report(`${src.name} ${assetName} 下载`, false, String(e.message || e).slice(0, 120));
    }
  }

  try { fs.unlinkSync(tmp); } catch {}
  finish();
}
