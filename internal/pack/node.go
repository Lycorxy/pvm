package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/pvm/pvm/internal/archive"
	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/download"
	nodeplugin "github.com/pvm/pvm/internal/plugins/node"
)

// seaFuseRe 匹配基座二进制里的 SEA 哨兵。
// 哈希随 Node 版本演进（20.20 已从文档旧值换成 ...df1996b2），
// 硬编码会在基座升级后失配 —— postject 找不到哨兵会直接报错，所以按基座实际值扫描。
var seaFuseRe = regexp.MustCompile(`NODE_SEA_FUSE_[0-9a-f]{32}`)

// seaSentinelOf 取基座二进制内嵌的哨兵
func seaSentinelOf(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := seaFuseRe.Find(data)
	if m == nil {
		return "", fmt.Errorf("no NODE_SEA_FUSE sentinel in %s (this build may not enable SEA)", path)
	}
	return string(m), nil
}

// nodePacker 用官方 SEA（Single Executable Application）打包，要求 Node >= 20。
//
// SEA 的底座是 node 二进制本身：host 目标直接复用当前 node；
// 跨平台目标自动下载对应平台的官方 node 发行版（缓存在 ~/.pvm/cache/pack-node/）。
// blob 与平台无关，只生成一次，逐平台 postject 注入（按文件格式 PE/ELF/Mach-O 注入，与宿主平台无关）。
type nodePacker struct {
	opts Options

	nodePath    string  // node 可执行文件
	nodeVersion string  // 如 20.11.0
	hostNodeExe string  // 当前 node 的真实二进制（host 目标的底座）
	blobFile    string  // sea-prep.blob
	assets      []Asset // 写进 SEA blob 的资源
}

func (p *nodePacker) defaultOutput() string {
	if p.opts.Bundled != "" {
		return defaultOutput(p.opts.Bundled)
	}
	return defaultOutput(p.opts.Entry)
}

func (p *nodePacker) targets(ts []Target, _ bool) ([]Target, error) {
	if p.opts.Base != "" && len(ts) > 1 {
		return nil, fmt.Errorf("--base works with a single target, got %d: specify one, e.g. --type linux-amd64", len(ts))
	}
	switch p.opts.Libc {
	case "", "default", "legacy", "glibc217":
	default:
		return nil, fmt.Errorf("--libc %q: expected default or legacy (glibc-217 for CentOS 7)", p.opts.Libc)
	}
	return ts, nil
}

func (p *nodePacker) prepare(_ []Target, assets []Asset) (func(), error) {
	o := p.opts
	if o.Entry == "" && o.Bundled == "" {
		return nil, fmt.Errorf("pvm pkg node requires --entry <main.js> (or --bundled <pre-bundled.js>)")
	}
	if o.Base != "" {
		if info, err := os.Stat(o.Base); err != nil || info.IsDir() {
			return nil, fmt.Errorf("--base is not a file: %s", o.Base)
		}
	}
	// 入口预检：提前给清晰错误，而不是等 esbuild/SEA 报晦涩信息
	for _, f := range []struct{ kind, path string }{{"--entry", o.Entry}, {"--bundled", o.Bundled}} {
		if f.path == "" {
			continue
		}
		if _, err := os.Stat(f.path); err != nil {
			return nil, fmt.Errorf("%s file not found: %s", f.kind, f.path)
		}
	}

	nodePath, err := resolveCommand("pvm pkg node", "node")
	if err != nil {
		return nil, err
	}
	// esbuild / postject 依赖 npx 分发（项目 node_modules 里的本地安装优先）
	if _, err := resolveCommand("pvm pkg node", "npx"); err != nil {
		return nil, err
	}
	p.nodePath = nodePath

	// Node 版本门禁：SEA 要求 >= 20（兼容 v 前缀输出）
	version, err := captureOutput(nodePath, "-p", "process.versions.node")
	if err != nil {
		return nil, fmt.Errorf("check node version: %w", err)
	}
	major, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(version, ".", 2)[0], "v"))
	if err != nil || major < 20 {
		return nil, fmt.Errorf("node SEA packaging requires Node >= 20 (current: %s)", version)
	}
	if len(assets) > 0 && major == 20 {
		parts := strings.SplitN(version, ".", 3)
		if len(parts) >= 2 {
			if minor, err := strconv.Atoi(parts[1]); err == nil && minor < 12 {
				return nil, fmt.Errorf("--include requires Node >= 20.12 (SEA assets), current: %s", version)
			}
		}
	}
	p.nodeVersion = version
	p.assets = assets

	// 当前 node 二进制：host 目标直接复用，无需下载
	hostExe, err := captureOutput(nodePath, "-p", "process.execPath")
	if err != nil {
		return nil, fmt.Errorf("locate node binary: %w", err)
	}
	hostExe = strings.Trim(hostExe, `"'`)
	if info, err := os.Stat(hostExe); err != nil || info.IsDir() {
		return nil, fmt.Errorf("resolved node binary is not a file: %s", hostExe)
	}
	p.hostNodeExe = hostExe

	tmpDir, err := os.MkdirTemp("", "pvm-pkg-node-*")
	if err != nil {
		return nil, err
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := p.makeBlob(tmpDir); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}

// makeBlob bundle（优先用户预打包产物，否则 esbuild）并生成 SEA blob。
// 供应链安全：固定版本范围（esbuild@0.25 / postject@1.0.0-alpha.4），
// 防止同名恶意包劫持；项目 node_modules 中已安装的版本仍优先（npx 语义）。
func (p *nodePacker) makeBlob(tmpDir string) error {
	bundleFile := p.opts.Bundled
	if bundleFile == "" {
		bundleFile = filepath.Join(tmpDir, "bundle.js")
		fmt.Println("==> bundling with esbuild")
		args := []string{"--yes", "esbuild@0.25", p.opts.Entry,
			"--bundle", "--platform=node", "--format=cjs", "--outfile=" + bundleFile}
		if major := strings.SplitN(p.nodeVersion, ".", 2)[0]; major != "" {
			args = append(args, "--target=node"+major)
		}
		for _, e := range p.opts.External {
			args = append(args, "--external:"+e)
		}
		if err := runStep(p.opts.Verbose, "esbuild bundle", "npx", args...); err != nil {
			return fmt.Errorf("%v\n"+
				"  common causes: native .node addons cannot be bundled; dynamic require/require.resolve;\n"+
				"  hint: pre-bundle manually (any bundler) and pass --bundled <file.js>", err)
		}
	} else {
		fmt.Println("==> using pre-bundled entry")
	}
	bundleAbs, err := filepath.Abs(bundleFile)
	if err != nil {
		return err
	}
	if info, err := os.Stat(bundleAbs); err != nil || info.Size() == 0 {
		return fmt.Errorf("bundle output missing or empty: %s", bundleAbs)
	}

	// disableExperimentalSEAWarning：20/21 上消除实验告警噪音
	p.blobFile = filepath.Join(tmpDir, "sea-prep.blob")
	cfg := map[string]any{
		"main":                          filepath.ToSlash(bundleAbs),
		"output":                        filepath.ToSlash(p.blobFile),
		"disableExperimentalSEAWarning": true,
	}
	if len(p.assets) > 0 {
		embedded := make(map[string]string, len(p.assets))
		for _, a := range p.assets {
			embedded[a.Name] = filepath.ToSlash(a.Path)
		}
		cfg["assets"] = embedded
		fmt.Printf("==> embedding %d asset(s): %s\n", len(p.assets), assetNames(p.assets))
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	seaConfigFile := filepath.Join(tmpDir, "sea-config.json")
	if err := os.WriteFile(seaConfigFile, append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Println("==> generating SEA blob")
	return runStep(p.opts.Verbose, "node --experimental-sea-config", p.nodePath, "--experimental-sea-config", seaConfigFile)
}

func (p *nodePacker) build(t Target, out string, idx, total int) error {
	base := p.opts.Base
	if base == "" {
		switch {
		case p.opts.Libc == "legacy" && t.OS == "linux":
			var err error
			base, err = ensureLegacyNodeBinary(p.nodeVersion, t)
			if err != nil {
				return err
			}
		case t.IsHost():
			base = p.hostNodeExe
		default:
			var err error
			base, err = ensureNodeDistBinary(p.nodeVersion, t, !p.opts.Official)
			if err != nil {
				return err
			}
		}
	}

	if total > 1 {
		fmt.Printf("==> [%d/%d] %s\n", idx, total, t)
	}
	if err := copyExecutable(base, out); err != nil {
		return fmt.Errorf("copy node binary to %s: %w", out, err)
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return err
	}

	// macOS 目标：注入会破坏签名，注入前必须去签名（仅 macOS 主机有 codesign）
	onMac := runtime.GOOS == "darwin"
	if t.OS == "darwin" && onMac {
		codesign, err := exec.LookPath("codesign")
		if err != nil {
			return fmt.Errorf("macOS packaging requires codesign (install Xcode Command Line Tools)")
		}
		if err := runStep(p.opts.Verbose, "codesign --remove-signature", codesign, "--remove-signature", outAbs); err != nil {
			return err
		}
	}

	sentinel, err := seaSentinelOf(base)
	if err != nil {
		return err
	}
	if err := runStep(p.opts.Verbose, "postject inject", "npx", "--yes", "postject@1.0.0-alpha.4",
		outAbs, "NODE_SEA_BLOB", p.blobFile,
		"--sentinel-fuse", sentinel); err != nil {
		return err
	}

	if t.OS == "darwin" {
		if onMac {
			if codesign, _ := exec.LookPath("codesign"); codesign != "" {
				return runStep(p.opts.Verbose, "codesign --sign", codesign, "--sign", "-", outAbs)
			}
		} else {
			// 非 macOS 主机没有 codesign：产物未签名，分发前请在 macOS 上补 ad-hoc 签名
			fmt.Println("    note: darwin artifact is unsigned; run `codesign --sign -` on macOS before distributing")
		}
	}
	return nil
}

// ensureNodeDistBinary 返回目标平台的官方 node 二进制，缺失时自动下载解压。
// 发行版缓存于 ~/.pvm/cache/pack-node/v<version>/<os>-<arch>/，重复打包零下载。
func ensureNodeDistBinary(version string, t Target, useMirror bool) (string, error) {
	cacheDir := nodeDistCacheDir(version, t)
	if bin, ok := findNodeBinary(cacheDir, t.OS); ok {
		return bin, nil
	}

	np := nodeplugin.New()
	info, err := np.GetDownloadInfo(version, t.OS, t.Arch)
	if err != nil {
		return "", err
	}
	if useMirror {
		if mirrored, err := np.GetDownloadInfoMirror(version, t.OS, t.Arch); err == nil {
			info = mirrored
		}
	}

	fmt.Printf("==> downloading node v%s dist for %s\n", version, t)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	archiveFile := filepath.Join(cacheDir, "dist."+info.ArchiveType)
	if err := download.DownloadFile(info.URL, archiveFile); err != nil {
		return "", fmt.Errorf("download node %s: %w", t, err)
	}
	defer os.Remove(archiveFile)

	// 解压先落位到临时目录，成功后原子改名进缓存，避免残留半成品
	extractDir := cacheDir + ".extracting"
	os.RemoveAll(extractDir)
	if err := extractQuiet(archiveFile, extractDir, info.ArchiveType); err != nil {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("extract node dist for %s: %w", t, err)
	}
	if _, ok := findNodeBinary(extractDir, t.OS); !ok {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("node binary not found in extracted dist for %s", t)
	}
	os.RemoveAll(cacheDir)
	if err := os.Rename(extractDir, cacheDir); err != nil {
		return "", err
	}
	bin, ok := findNodeBinary(cacheDir, t.OS)
	if !ok {
		return "", fmt.Errorf("node binary not found in cached dist for %s", t)
	}
	return bin, nil
}

// nodeDistCacheDir 跨平台 SEA 底座缓存目录：~/.pvm/cache/pack-node/v<version>/<os>-<arch>
func nodeDistCacheDir(version string, t Target) string {
	return filepath.Join(config.CacheDir(), "pack-node", "v"+version, t.OS+"-"+t.Arch)
}

// unofficialNodeBase unofficial-builds 发布源（glibc-217 等特殊构建）
const unofficialNodeBase = "https://unofficial-builds.nodejs.org/download/release"

// ensureLegacyNodeBinary 取得 glibc-217 构建的 node（CentOS 7 等 glibc 2.17 机器可用）。
// 官方二进制要求 glibc ≥ 2.28，部署到老系统会报 "GLIBC_2.28 not found"；
// glibc-217 是同源码的旧基线构建，功能一致。缓存在 ~/.pvm/cache/pack-node/ 下。
func ensureLegacyNodeBinary(version string, t Target) (string, error) {
	if t.OS != "linux" {
		return "", fmt.Errorf("--libc legacy only applies to linux targets, got %s", t)
	}
	dir := filepath.Join(config.CacheDir(), "pack-node", "v"+version,
		t.OS+"-"+t.Arch+"-glibc217")
	if bin, ok := findNodeBinary(dir, t.OS); ok {
		return bin, nil
	}

	name := fmt.Sprintf("node-v%s-linux-%s-glibc-217", version, nodeArch(t.Arch))
	url := fmt.Sprintf("%s/v%s/%s.tar.gz", unofficialNodeBase, version, name)
	fmt.Printf("==> downloading node %s (glibc-217)\n", t)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	archiveFile := filepath.Join(dir, "dist.tar.gz")
	if err := download.DownloadFile(url, archiveFile); err != nil {
		return "", fmt.Errorf("download node %s glibc-217: %w\n  hint: use --base <path> with a manually downloaded base", t, err)
	}
	defer os.Remove(archiveFile)

	extractDir := dir + ".extracting"
	os.RemoveAll(extractDir)
	if err := extractQuiet(archiveFile, extractDir, "tar.gz"); err != nil {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("extract node glibc-217 for %s: %w", t, err)
	}
	if _, ok := findNodeBinary(extractDir, t.OS); !ok {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("node binary not found in glibc-217 dist for %s", t)
	}
	os.RemoveAll(dir)
	if err := os.Rename(extractDir, dir); err != nil {
		return "", err
	}
	bin, ok := findNodeBinary(dir, t.OS)
	if !ok {
		return "", fmt.Errorf("node binary not found in cached glibc-217 dist for %s", t)
	}
	return bin, nil
}

// extractQuiet 解压但不打印进度（打包流程只保留结果行）
func extractQuiet(file, dir, typ string) error {
	archive.Quiet = true
	defer func() { archive.Quiet = false }()
	return archive.Extract(file, dir, typ)
}

// nodeArch 把 GOARCH 映射成 node 发行版的架构名
func nodeArch(arch string) string {
	switch arch {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	}
	return arch
}

// findNodeBinary 在解压后的 node 发行版目录中定位 node 可执行文件：
//   - windows zip：node.exe 位于压缩包根目录
//   - unix tar.gz：<version-dir>/bin/node
func findNodeBinary(dir, targetOS string) (string, bool) {
	name := "node"
	if targetOS == "windows" {
		name = "node.exe"
	}
	found := ""
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), name) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", false
	}
	if info, err := os.Stat(found); err != nil || info.IsDir() || info.Size() == 0 {
		return "", false
	}
	return found, true
}
