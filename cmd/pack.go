package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pvm/pvm/internal/archive"
	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/download"
	nodeplugin "github.com/pvm/pvm/internal/plugins/node"
)

// seaFuseSentinel 是 Node SEA 官方 sentinel fuse 值（Node 文档固定值）
const seaFuseSentinel = "NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2"

// runPack 把项目打包为单可执行二进制：默认跨平台全量，--platform 指定则只打对应目标
//
// 用法：
//
//	pvm pack node   --entry <main.js> [--bundled <file.js>] [--output <path>] [--platform <target>] [--official]
//	pvm pack python --entry <main.py> [--output <path>] [--platform <target>]
//	pvm pack go     [--entry <pkg>] [--output <path>] [--platform <target>]
//
// --platform 目标平台：
//   - 不指定 / all    → 跨平台全量打包（默认矩阵：windows/amd64, linux/amd64, linux/arm64,
//     darwin/amd64, darwin/arm64）
//   - host / current  → 仅当前平台
//   - <os>/<arch>     → 只打一个（如 linux/amd64）；逗号分隔可一次指定多个
//
// 多目标时产物名在 --output 扩展名前插入 -<os>-<arch>（如 dist/app-linux-amd64）；
// Windows 目标自动补 .exe，非 Windows 目标自动去掉误写的 .exe。
//
// 各运行时的打包方案（只走稳定、官方或事实标准的路线）：
//   - node   : 官方 SEA（Single Executable Application），要求 Node >= 20；
//     跨平台时自动下载各目标平台的官方 node 发行版作为底座（缓存在 ~/.pvm/cache/pack-node/）
//   - python : PyInstaller（--onefile）；PyInstaller 本身不支持交叉打包 → 默认仅产出当前平台，
//     显式指定非当前平台的 --platform 会直接报错
//   - go     : go build -trimpath -ldflags "-s -w"；交叉编译置 CGO_ENABLED=0
//
// 在项目根目录的 pvm.yaml 中作为普通命令使用：
//
//	pkg:
//	  go:
//	    pro:
//	      - pvm pack go --entry . --output dist/app   # 跨平台全量
//	      - pvm pack go --entry . --output dist/app --platform host   # 仅当前平台
func runPack(args []string) error {
	const usage = "usage: pvm pack <node|python|go> [--entry <path>] [--output <path>] [--platform <target>] [--bundled <file>] [--official]"
	if len(args) == 0 {
		return fmt.Errorf("%s", usage)
	}

	rt := strings.ToLower(args[0])
	rest := args[1:]

	rest, entry, err := takeFlagValue(rest, "--entry")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, usage)
	}
	rest, output, err := takeFlagValue(rest, "--output")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, usage)
	}
	rest, platform, err := takeFlagValue(rest, "--platform")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, usage)
	}
	rest, bundled, err := takeFlagValue(rest, "--bundled")
	if err != nil {
		return fmt.Errorf("%v\n%s", err, usage)
	}
	rest, useOfficial := takeFlagBool(rest, "--official")
	if len(rest) > 0 {
		return fmt.Errorf("unknown arguments: %s\n%s", strings.Join(rest, " "), usage)
	}

	switch rt {
	case "node":
		return packNode(entry, bundled, output, platform, !useOfficial)
	case "python":
		return packPython(entry, output, platform)
	case "go":
		return packGo(entry, output, platform)
	default:
		return fmt.Errorf("unsupported pack target: %q (supported: node, python, go)\n%s", rt, usage)
	}
}

// ---- 目标平台 ----

// packTarget 一个打包目标（使用 Go 的 GOOS/GOARCH 命名，各运行时自行映射到官方发行版命名）
type packTarget struct {
	OS   string
	Arch string
}

// defaultPackTargets 跨平台默认矩阵：主流桌面 + 服务器平台
var defaultPackTargets = []packTarget{
	{"windows", "amd64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// hostPackTarget 当前平台
func hostPackTarget() packTarget {
	return packTarget{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// isHost 判断目标是否当前平台
func (t packTarget) isHost() bool {
	return t.OS == runtime.GOOS && t.Arch == runtime.GOARCH
}

// resolvePackTargets 解析 --platform：
//   - ""（未指定）/ "all" → 跨平台全量矩阵（默认行为）
//   - "host" / "current"  → 仅当前平台
//   - "<os>/<arch>"       → 只打一个；逗号分隔可指定多个（自动去重）
func resolvePackTargets(platform string) ([]packTarget, error) {
	p := strings.ToLower(strings.TrimSpace(platform))
	if p == "" || p == "all" {
		out := make([]packTarget, len(defaultPackTargets))
		copy(out, defaultPackTargets)
		return out, nil
	}
	if p == "host" || p == "current" {
		return []packTarget{hostPackTarget()}, nil
	}

	var targets []packTarget
	seen := make(map[packTarget]bool)
	for _, part := range strings.Split(p, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --platform %q, expected <os>/<arch> (e.g. linux/amd64), comma-separated list, 'host' or 'all'", platform)
		}
		t := packTarget{OS: parts[0], Arch: parts[1]}
		if !seen[t] {
			seen[t] = true
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("invalid --platform %q", platform)
	}
	return targets, nil
}

// joinTargets 把目标列表拼成 "windows/amd64, linux/amd64" 形式
func joinTargets(targets []packTarget) string {
	parts := make([]string, len(targets))
	for i, t := range targets {
		parts[i] = t.OS + "/" + t.Arch
	}
	return strings.Join(parts, ", ")
}

// platformOutputPath 多目标打包时在扩展名前插入 -<os>-<arch>，避免产物互相覆盖
// （dist/app → dist/app-linux-amd64；.exe 后缀交给 normalizeExeSuffix 规范）
func platformOutputPath(output string, t packTarget) string {
	dir, base := filepath.Split(output)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return filepath.Join(dir, base+"-"+t.OS+"-"+t.Arch)
}

// normalizeExeSuffix 按目标平台规范可执行后缀：
// windows 补 .exe；非 windows 去掉误写的 .exe（跨平台产物不带 .exe）
func normalizeExeSuffix(output, targetOS string) string {
	ext := strings.ToLower(filepath.Ext(output))
	if targetOS == "windows" {
		if ext != ".exe" {
			return output + ".exe"
		}
		return output
	}
	if ext == ".exe" {
		return strings.TrimSuffix(output, filepath.Ext(output))
	}
	return output
}

// targetOutput 计算目标产物路径：
// 单目标保持 --output 原名；多目标插入平台后缀；最后统一规范 .exe
func targetOutput(output string, t packTarget, multi bool) string {
	out := output
	if multi {
		out = platformOutputPath(output, t)
	}
	return normalizeExeSuffix(out, t.OS)
}

// ---- 通用辅助 ----

// takeFlagValue 从 args 中提取 --name <value>，返回剩余参数
func takeFlagValue(args []string, name string) ([]string, string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] != name {
			continue
		}
		if i+1 >= len(args) {
			return nil, "", fmt.Errorf("%s requires a value", name)
		}
		rest := append([]string{}, args[:i]...)
		rest = append(rest, args[i+2:]...)
		return rest, args[i+1], nil
	}
	return args, "", nil
}

// takeFlagBool 提取 --name 布尔标志，返回剩余参数与是否出现
func takeFlagBool(args []string, name string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == name {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, found
}

// runStream 执行命令并透传标准输入输出，失败时附带上下文
func runStream(context string, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// captureOutput 执行命令并捕获 stdout
func captureOutput(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveCommand 查找命令；找不到时给出带上下文的错误
func resolveCommand(context, name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s: %s not found in PATH", context, name)
	}
	return p, nil
}

// defaultOutputEntry 返回默认产物路径：dist/<入口文件主干名>
func defaultOutputEntry(entry string) string {
	stem := filepath.Base(entry)
	if ext := filepath.Ext(stem); ext != "" {
		stem = strings.TrimSuffix(stem, ext)
	}
	if stem == "" || stem == "." {
		stem = "app"
	}
	return filepath.Join("dist", stem)
}

// copyExecutable 复制二进制文件并保留可执行权限
func copyExecutable(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// reportArtifact 打印产物信息
func reportArtifact(kind string, entry, output string) {
	info, err := os.Stat(output)
	if err != nil {
		fmt.Printf("==> packed %s: %s (from %s)\n", kind, output, entry)
		return
	}
	fmt.Printf("==> packed %s: %s (%.1f MB, from %s)\n",
		kind, output, float64(info.Size())/1024/1024, entry)
}

// ---- Go build ----

// packGo 用 go build 打包；默认跨平台全量，--platform 指定则只打对应目标。
// 交叉编译置 CGO_ENABLED=0 保证静态产物；当前平台目标保持宿主默认 CGO 行为。
func packGo(entry, output, platform string) error {
	if entry == "" {
		entry = "."
	}
	if output == "" {
		if entry == "." {
			cwd, _ := os.Getwd()
			output = filepath.Join("dist", filepath.Base(cwd))
		} else {
			output = defaultOutputEntry(entry)
		}
	}

	targets, err := resolvePackTargets(platform)
	if err != nil {
		return err
	}

	goPath, err := resolveCommand("pvm pack go", "go")
	if err != nil {
		return err
	}

	multi := len(targets) > 1
	if multi {
		fmt.Printf("==> go build: %d cross-platform targets (%s)\n", len(targets), joinTargets(targets))
	}

	for i, t := range targets {
		out := targetOutput(output, t, multi)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}

		env := os.Environ()
		if !t.isHost() {
			env = append(env, "GOOS="+t.OS, "GOARCH="+t.Arch, "CGO_ENABLED=0")
		}

		if multi {
			fmt.Printf("==> [%d/%d] go build (%s/%s) -> %s\n", i+1, len(targets), t.OS, t.Arch, out)
		} else {
			fmt.Printf("==> go build (%s/%s) -> %s\n", t.OS, t.Arch, out)
		}
		cmd := exec.Command(goPath, "build", "-trimpath", "-ldflags", "-s -w", "-o", out, entry)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go build %s/%s: %w", t.OS, t.Arch, err)
		}
		reportArtifact("go", entry, out)
	}
	return nil
}

// ---- Node SEA ----

// packNode 用官方 SEA 打包（要求 Node >= 20），默认跨平台全量。
// SEA 的底座是 node 二进制本身：host 目标直接复用当前 node；
// 跨平台目标自动下载对应平台的官方 node 发行版（缓存在 ~/.pvm/cache/pack-node/）。
// blob 与平台无关，只生成一次，逐平台 postject 注入（postject 按文件格式 PE/ELF/Mach-O 注入，与宿主平台无关）。
func packNode(entry, bundled, output, platform string, useMirror bool) error {
	if entry == "" && bundled == "" {
		return fmt.Errorf("pvm pack node requires --entry <main.js> (or --bundled <pre-bundled.js>)")
	}
	if output == "" {
		if bundled != "" {
			output = defaultOutputEntry(bundled)
		} else {
			output = defaultOutputEntry(entry)
		}
	}

	// 入口预检：提前给清晰错误，而不是等 esbuild/SEA 报晦涩信息
	for _, f := range []struct{ kind, path string }{
		{"--entry", entry}, {"--bundled", bundled},
	} {
		if f.path == "" {
			continue
		}
		if _, err := os.Stat(f.path); err != nil {
			return fmt.Errorf("%s file not found: %s", f.kind, f.path)
		}
	}

	nodePath, err := resolveCommand("pvm pack node", "node")
	if err != nil {
		return err
	}
	// esbuild / postject 依赖 npx 分发（项目 node_modules 里的本地安装优先）
	if _, err := resolveCommand("pvm pack node", "npx"); err != nil {
		return err
	}

	// 1. Node 版本门禁：SEA 要求 >= 20（兼容 v 前缀输出）
	versionOut, err := captureOutput(nodePath, "-p", "process.versions.node")
	if err != nil {
		return fmt.Errorf("check node version: %w", err)
	}
	major, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(versionOut, ".", 2)[0], "v"))
	if err != nil || major < 20 {
		return fmt.Errorf("node SEA packaging requires Node >= 20 (current: %s)", versionOut)
	}

	targets, err := resolvePackTargets(platform)
	if err != nil {
		return err
	}
	multi := len(targets) > 1
	if multi {
		fmt.Printf("==> node SEA: %d cross-platform targets (%s)\n", len(targets), joinTargets(targets))
	}

	tmpDir, err := os.MkdirTemp("", "pvm-pack-node-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// 2. bundle：优先使用用户预打包产物；否则用 esbuild。
	// 供应链安全：固定版本范围（esbuild@0.25 / postject@1.0.0-alpha.4），
	// 防止同名恶意包劫持；项目 node_modules 中已安装的版本仍优先（npx 语义）。
	bundleFile := bundled
	if bundleFile == "" {
		bundleFile = filepath.Join(tmpDir, "bundle.js")
		fmt.Println("==> [1/5] bundling with esbuild")
		if err := runStream("esbuild bundle", "npx", "--yes", "esbuild@0.25",
			entry, "--bundle", "--platform=node", "--format=cjs",
			"--outfile="+bundleFile); err != nil {
			return fmt.Errorf("%v\n"+
				"  common causes: native .node addons cannot be bundled; dynamic require/require.resolve;\n"+
				"  hint: pre-bundle manually (any bundler) and pass --bundled <file.js>", err)
		}
	} else {
		fmt.Println("==> [1/5] using pre-bundled entry")
	}
	bundleAbs, err := filepath.Abs(bundleFile)
	if err != nil {
		return err
	}
	if info, err := os.Stat(bundleAbs); err != nil || info.Size() == 0 {
		return fmt.Errorf("bundle output missing or empty: %s", bundleAbs)
	}

	// 3. sea-config.json（disableExperimentalSEAWarning：20/21 上消除实验告警噪音）+ 生成 blob
	blobFile := filepath.Join(tmpDir, "sea-prep.blob")
	seaConfig := fmt.Sprintf(
		`{"main": %q, "output": %q, "disableExperimentalSEAWarning": true}`,
		filepath.ToSlash(bundleAbs), filepath.ToSlash(blobFile))
	seaConfigFile := filepath.Join(tmpDir, "sea-config.json")
	if err := os.WriteFile(seaConfigFile, []byte(seaConfig), 0o644); err != nil {
		return err
	}
	fmt.Println("==> [2/5] generating SEA blob")
	if err := runStream("node --experimental-sea-config", nodePath, "--experimental-sea-config", seaConfigFile); err != nil {
		return err
	}

	// 4. 当前 node 二进制：host 目标直接复用，无需下载
	hostNodeExe, err := captureOutput(nodePath, "-p", "process.execPath")
	if err != nil {
		return fmt.Errorf("locate node binary: %w", err)
	}
	hostNodeExe = strings.Trim(hostNodeExe, `"'`)
	if info, err := os.Stat(hostNodeExe); err != nil || info.IsDir() {
		return fmt.Errorf("resolved node binary is not a file: %s", hostNodeExe)
	}

	// 5. 逐目标：取底座 → 拷贝 → (darwin 去签名) → postject 注入 → (darwin 重签名)
	for i, t := range targets {
		base := hostNodeExe
		if !t.isHost() {
			base, err = ensureNodeDistBinary(versionOut, t, useMirror)
			if err != nil {
				return err
			}
		}

		out := targetOutput(output, t, multi)
		fmt.Printf("==> [%d/%d] (%s/%s) copying node binary -> %s\n", i+1, len(targets), t.OS, t.Arch, out)
		if err := copyExecutable(base, out); err != nil {
			return fmt.Errorf("copy node binary to %s: %w", out, err)
		}
		outAbs, err := filepath.Abs(out)
		if err != nil {
			return err
		}

		// macOS 目标：注入会破坏签名，注入前必须去签名（仅 macOS 主机有 codesign）
		if t.OS == "darwin" && runtime.GOOS == "darwin" {
			codesign, err := exec.LookPath("codesign")
			if err != nil {
				return fmt.Errorf("macOS packaging requires codesign (install Xcode Command Line Tools)")
			}
			if err := runStream("codesign --remove-signature", codesign, "--remove-signature", outAbs); err != nil {
				return err
			}
		}

		fmt.Printf("==> [%d/%d] (%s/%s) injecting SEA blob with postject\n", i+1, len(targets), t.OS, t.Arch)
		if err := runStream("postject inject", "npx", "--yes", "postject@1.0.0-alpha.4",
			outAbs, "NODE_SEA_BLOB", blobFile,
			"--sentinel-fuse", seaFuseSentinel); err != nil {
			return err
		}

		if t.OS == "darwin" {
			if runtime.GOOS == "darwin" {
				if codesign, _ := exec.LookPath("codesign"); codesign != "" {
					if err := runStream("codesign --sign", codesign, "--sign", "-", outAbs); err != nil {
						return err
					}
				}
			} else {
				// 非 macOS 主机没有 codesign：产物未签名，分发前请在 macOS 上补 ad-hoc 签名
				fmt.Println("    note: darwin artifact is unsigned; run `codesign --sign -` on macOS before distributing")
			}
		}
		reportArtifact("node", entryOr(entry, bundled), out)
	}

	fmt.Println("==> [5/5] done")
	return nil
}

func entryOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ensureNodeDistBinary 返回目标平台的官方 node 二进制，缺失时自动下载解压。
// 发行版缓存于 ~/.pvm/cache/pack-node/v<version>/<os>-<arch>/，重复打包零下载。
func ensureNodeDistBinary(version string, t packTarget, useMirror bool) (string, error) {
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

	fmt.Printf("==> downloading node v%s dist for %s/%s\n", version, t.OS, t.Arch)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	// 解压先落位到临时目录，成功后原子改名进缓存，避免残留半成品
	archiveFile := filepath.Join(cacheDir, "dist."+info.ArchiveType)
	if err := download.DownloadFile(info.URL, archiveFile); err != nil {
		return "", fmt.Errorf("download node %s/%s: %w", t.OS, t.Arch, err)
	}
	defer os.Remove(archiveFile)

	extractDir := cacheDir + ".extracting"
	os.RemoveAll(extractDir)
	if err := archive.Extract(archiveFile, extractDir, info.ArchiveType); err != nil {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("extract node dist for %s/%s: %w", t.OS, t.Arch, err)
	}
	bin, ok := findNodeBinary(extractDir, t.OS)
	if !ok {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("node binary not found in extracted dist for %s/%s", t.OS, t.Arch)
	}
	os.RemoveAll(cacheDir)
	if err := os.Rename(extractDir, cacheDir); err != nil {
		return "", err
	}
	// rename 后原 extractDir 路径已失效，在缓存目录中重新定位
	bin, ok = findNodeBinary(cacheDir, t.OS)
	if !ok {
		return "", fmt.Errorf("node binary not found in cached dist for %s/%s", t.OS, t.Arch)
	}
	return bin, nil
}

// nodeDistCacheDir 跨平台 SEA 底座缓存目录：~/.pvm/cache/pack-node/v<version>/<os>-<arch>
func nodeDistCacheDir(version string, t packTarget) string {
	return filepath.Join(config.CacheDir(), "pack-node", "v"+version, t.OS+"-"+t.Arch)
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

// ---- Python PyInstaller ----

// packPython 用 PyInstaller --onefile 打包为单可执行文件。
// PyInstaller 本身不支持交叉打包：默认（跨平台全量）实际仅产出当前平台；
// --platform 显式指定非当前平台时直接报错，避免产生"已打全平台"的错觉。
// PyInstaller 缺失时自动安装到当前解析到的 python 环境（pvm 托管版本间相互隔离）
func packPython(entry, output, platform string) error {
	if entry == "" {
		return fmt.Errorf("pvm pack python requires --entry <main.py>")
	}
	if output == "" {
		output = defaultOutputEntry(entry)
	}

	targets, err := resolvePackTargets(platform)
	if err != nil {
		return err
	}
	host := hostPackTarget()
	for _, t := range targets {
		if t.OS != host.OS || t.Arch != host.Arch {
			return fmt.Errorf("PyInstaller cannot cross-compile: target %s/%s requested, but host is %s/%s\n"+
				"  hint: run `pvm pack python` on each target platform (e.g. via a CI matrix)", t.OS, t.Arch, host.OS, host.Arch)
		}
	}
	if platform == "" || len(targets) > 1 {
		fmt.Printf("==> PyInstaller can only package for the current platform; building %s/%s only\n", host.OS, host.Arch)
	}

	pyName := "python3"
	if runtime.GOOS == "windows" {
		pyName = "python"
	}
	pyPath, err := resolveCommand("pvm pack python", pyName)
	if err != nil {
		return err
	}

	// 1. PyInstaller 可用性检查，缺失则安装
	if _, err := captureOutput(pyPath, "-m", "PyInstaller", "--version"); err != nil {
		fmt.Println("==> [1/3] installing PyInstaller into the resolved python environment")
		if err := runStream("pip install pyinstaller", pyPath, "-m", "pip",
			"install", "--disable-pip-version-check", "--no-input", "pyinstaller"); err != nil {
			return fmt.Errorf("%v\n  hint: install manually with `python -m pip install pyinstaller`", err)
		}
	} else {
		fmt.Println("==> [1/3] PyInstaller available")
	}

	// 2. 输出目录与工作目录
	output = normalizeExeSuffix(output, host.OS)
	outDir := filepath.Dir(output)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	workDir, err := os.MkdirTemp("", "pvm-pack-py-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	// 3. --onefile 单文件产物；--clean 清理缓存保证可复现
	fmt.Println("==> [2/3] running PyInstaller (--onefile)")
	if err := runStream("pyinstaller", pyPath, "-m", "PyInstaller",
		"--onefile", "--clean", "--noconfirm",
		"--distpath", outDir, "--workpath", workDir, "--specpath", workDir,
		entry); err != nil {
		return err
	}

	// 4. PyInstaller 产物名为入口主干名；重命名为 --output 指定名
	stem := filepath.Base(entry)
	if ext := filepath.Ext(stem); ext != "" {
		stem = strings.TrimSuffix(stem, ext)
	}
	produced := filepath.Join(outDir, normalizeExeSuffix(stem, host.OS))
	if filepath.Clean(produced) != filepath.Clean(output) {
		if err := os.Rename(produced, output); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", produced, output, err)
		}
	}

	fmt.Println("==> [3/3] done")
	reportArtifact("python", entry, output)
	return nil
}
