package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/semver"
)

// RuntimeInfo holds download information for a runtime
type RuntimeInfo struct {
	URL         string // download URL
	ArchiveType string // "tar.gz", "zip"
	FallbackURL string // fallback URL if primary fails (optional)
}

// VersionInfo represents a remote available version
type VersionInfo struct {
	Version string
	LTS     bool   // for Node.js
	Date    string // release date if available
}

// GetDownloadInfo returns the download URL and archive type for a given runtime + version
func GetDownloadInfo(rt string, version string) (*RuntimeInfo, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch rt {
	case "go":
		return getGoInfo(version, goos, goarch)
	case "node":
		return getNodeInfo(version, goos, goarch)
	case "python":
		return getPythonInfo(version, goos, goarch)
	case "pnpm":
		return getPnpmInfo(version)
	case "git":
		return getGitInfo(version, goos, goarch)
	case "yarn":
		return getYarnInfo(version)
	case "rust":
		return getRustInfo(version, goos, goarch)
	case "bun":
		return getBunInfo(version, goos, goarch)
	case "deno":
		return getDenoInfo(version, goos, goarch)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", rt)
	}
}

// getGitInfo 返回 git 的下载信息
// Windows: 完整版 (tar.bz2 格式，包含 Git Bash)
// Linux/macOS: 从 kernel.org 下载源码 tar.gz
func getGitInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	switch goos {
	case "windows":
		arch := "64-bit"
		if goarch == "386" {
			arch = "32-bit"
		}
		if goarch == "arm64" {
			arch = "arm64"
		}
		// 优先使用国内镜像 (npmmirror)，仅提供 tar.bz2 格式
		url := fmt.Sprintf(
			"https://registry.npmmirror.com/-/binary/git-for-windows/v%s.windows.1/Git-%s-%s.tar.bz2",
			version, version, arch,
		)
		fallbackURL := fmt.Sprintf(
			"https://github.com/git-for-windows/git/releases/download/v%s.windows.1/Git-%s-%s.tar.bz2",
			version, version, arch,
		)
		return &RuntimeInfo{
			URL:         url,
			ArchiveType: "tar.bz2",
			FallbackURL: fallbackURL,
		}, nil
	case "darwin":
		// macOS 使用 Homebrew (官方没有预编译包)
		return nil, fmt.Errorf(
			"git on macOS: please use Homebrew instead\n  brew install git\n  or: brew upgrade git",
		)
	default: // linux
		// 从 kernel.org 下载源码包
		url := fmt.Sprintf(
			"https://mirrors.edge.kernel.org/pub/software/scm/git/git-%s.tar.gz",
			version,
		)
		return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
	}
}

// getPnpmInfo 返回 pnpm 的下载信息
// pnpm 从 npm registry 下载 tgz，不区分平台
func getPnpmInfo(version string) (*RuntimeInfo, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/pnpm/-/pnpm-%s.tgz", version)
	return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

func getGoInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	url := fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.%s", version, goos, goarch, ext)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

func getNodeInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	nodeOS := goos
	nodeArch := goarch

	switch goos {
	case "windows":
		nodeOS = "win"
	}

	switch goarch {
	case "amd64":
		nodeArch = "x64"
	case "386":
		nodeArch = "x86"
	}

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	url := fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-%s-%s.%s",
		version, version, nodeOS, nodeArch, ext)

	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

func getPythonInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	return getPythonInfoWithMirror(version, goos, goarch, false)
}

func getPythonInfoWithMirror(version, goos, goarch string, useMirror bool) (*RuntimeInfo, error) {
	pyArch := goarch
	switch goarch {
	case "amd64":
		pyArch = "x86_64"
	case "arm64":
		pyArch = "aarch64"
	case "386":
		pyArch = "i686"
	}

	// Get the build tag dynamically
	buildTag, err := resolvePythonBuildTag(version, useMirror)
	if err != nil {
		return nil, fmt.Errorf("resolve Python build tag: %w", err)
	}

	var triple string
	ext := "tar.gz"

	switch goos {
	case "windows":
		// Windows 安装包使用 .tar.gz 格式（与 Linux/macOS 相同）
		// 注意：文件名不含 "shared-" 前缀
		triple = fmt.Sprintf("cpython-%s+%s-%s-pc-windows-msvc-install_only.tar.gz",
			version, buildTag, pyArch)
	case "darwin":
		triple = fmt.Sprintf("cpython-%s+%s-%s-apple-darwin-install_only.%s",
			version, buildTag, pyArch, ext)
	case "linux":
		triple = fmt.Sprintf("cpython-%s+%s-%s-unknown-linux-gnu-install_only.%s",
			version, buildTag, pyArch, ext)
	default:
		return nil, fmt.Errorf("unsupported OS for Python: %s", goos)
	}

	var url string
	if useMirror {
		// npmmirror 提供了 python-build-standalone 的镜像
		// 格式：https://registry.npmmirror.com/-/binary/python-build-standalone/<tag>/<filename>
		url = fmt.Sprintf(
			"https://registry.npmmirror.com/-/binary/python-build-standalone/%s/%s",
			buildTag, triple)
	} else {
		url = fmt.Sprintf(
			"https://github.com/indygreg/python-build-standalone/releases/download/%s/%s",
			buildTag, triple)
	}

	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// pythonBuildTagCache caches resolved build tags for Python versions
var pythonBuildTagCache = make(map[string]string)

// Known fallback tags (used if API is unreachable)
var knownPythonTags = map[string]string{
	"3.13.0":  "20241016",
	"3.12.7":  "20241016",
	"3.12.6":  "20240909",
	"3.12.5":  "20240814",
	"3.12.4":  "20240713",
	"3.12.3":  "20240415",
	"3.12.2":  "20240224",
	"3.12.1":  "20240107",
	"3.12.0":  "20231002",
	"3.11.10": "20241016",
	"3.11.9":  "20240415",
	"3.11.8":  "20240224",
	"3.11.7":  "20240107",
	"3.10.15": "20241016",
	"3.10.14": "20240415",
	"3.10.13": "20240107",
	"3.9.20":  "20241016",
	"3.9.19":  "20240415",
	"3.9.18":  "20240107",
}

// resolvePythonBuildTag gets the build tag for a Python version.
// First checks cache/known tags, then queries GitHub API (or skips if useMirror).
func resolvePythonBuildTag(version string, useMirror bool) (string, error) {
	// Check cache first (may be populated by listRemotePythonMirror)
	if tag, ok := pythonBuildTagCache[version]; ok {
		return tag, nil
	}

	// 镜像模式下：跳过 knownPythonTags（可能不准确），直接从 npmmirror 动态查找
	if useMirror {
		logger.Verbose("  Querying npmmirror for Python %s build tag...", version)
		tag, err := fetchPythonBuildTagFromMirror(version)
		if err != nil {
			// 如果 npmmirror 查找失败，再尝试 knownPythonTags 作为 fallback
			if tag2, ok := knownPythonTags[version]; ok {
				logger.Verbose("  Fallback to known tag: %s", tag2)
				pythonBuildTagCache[version] = tag2
				return tag2, nil
			}
			return "", fmt.Errorf("cannot resolve build tag for Python %s via mirror: %w", version, err)
		}
		pythonBuildTagCache[version] = tag
		return tag, nil
	}

	// 非镜像模式：检查 known tags，然后查询 GitHub API
	if tag, ok := knownPythonTags[version]; ok {
		pythonBuildTagCache[version] = tag
		return tag, nil
	}

	// Query GitHub API for releases that contain this Python version
	logger.Verbose("  Querying GitHub API for Python %s build tag...", version)
	tag, err := fetchPythonBuildTag(version)
	if err != nil {
		logger.Verbose("  GitHub API query failed: %v, trying latest tag fallback", err)
		// Try fetching the latest release tag
		tag, err = fetchLatestPythonBuildTag()
		if err != nil {
			return "", fmt.Errorf("cannot resolve build tag for Python %s: %w", version, err)
		}
		// 验证该 tag 下是否真的包含这个 Python 版本
		if !verifyPythonVersionInTag(version, tag) {
			return "", fmt.Errorf("Python %s not found in the latest python-build-standalone release (%s). "+
				"This version may not be released yet. Available versions: use 'pvm list remote python' to check", version, tag)
		}
	}

	pythonBuildTagCache[version] = tag
	return tag, nil
}

// fetchPythonBuildTagFromMirror 通过 npmmirror 的目录列表查找包含指定 Python 版本的 build tag
// 遍历所有 build tag 目录，找到包含该 Python 版本文件的 tag
func fetchPythonBuildTagFromMirror(version string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// 先检查缓存
	if tag, ok := pythonBuildTagCache[version]; ok {
		logger.Verbose("  Using cached build tag: %s for Python %s", tag, version)
		return tag, nil
	}

	// 获取所有 tag 目录
	url := "https://registry.npmmirror.com/-/binary/python-build-standalone/"
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("npmmirror returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return "", fmt.Errorf("parse mirror directory: %w", err)
	}

	// 收集所有 tag（按降序排列，最新的优先）
	var tags []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name, "/")
		if len(name) == 8 {
			tags = append(tags, name)
		}
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no build tags found on npmmirror")
	}
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))

	// 遍历 tag，查找包含该 Python 版本的文件
	searchStr := fmt.Sprintf("cpython-%s+", version)
	for _, tag := range tags {
		filesURL := fmt.Sprintf("https://registry.npmmirror.com/-/binary/python-build-standalone/%s/", tag)
		resp2, err := client.Get(filesURL)
		if err != nil {
			continue
		}
		if resp2.StatusCode != 200 {
			resp2.Body.Close()
			continue
		}

		body2, err := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if err != nil {
			continue
		}

		var files []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body2, &files); err != nil {
			continue
		}

		// 检查是否有匹配的文件
		for _, f := range files {
			if strings.Contains(f.Name, searchStr) && strings.Contains(f.Name, "install_only") {
				logger.Verbose("  Found build tag: %s for Python %s", tag, version)
				pythonBuildTagCache[version] = tag
				return tag, nil
			}
		}
	}

	return "", fmt.Errorf("no build tag found containing Python %s on npmmirror", version)
}

// fetchPythonBuildTag queries GitHub releases to find a tag that contains the given Python version
func fetchPythonBuildTag(version string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// GitHub API: list releases
	url := "https://api.github.com/repos/indygreg/python-build-standalone/releases?per_page=30"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var releases []struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}

	if err := json.Unmarshal(body, &releases); err != nil {
		return "", err
	}

	// Look for a release that has an asset matching this Python version
	searchStr := fmt.Sprintf("cpython-%s+", version)
	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.Contains(asset.Name, searchStr) && strings.Contains(asset.Name, "install_only") {
				logger.Verbose("  Found build tag %s for Python %s", release.TagName, version)
				return release.TagName, nil
			}
		}
	}

	return "", fmt.Errorf("no release found containing Python %s", version)
}

// fetchLatestPythonBuildTag gets the latest release tag from python-build-standalone
func fetchLatestPythonBuildTag() (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get("https://github.com/indygreg/python-build-standalone/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// The redirect location contains the tag
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no redirect for latest release")
	}

	parts := strings.Split(loc, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("cannot parse tag from URL: %s", loc)
	}

	tag := parts[len(parts)-1]
	logger.Verbose("  Latest python-build-standalone tag: %s", tag)
	return tag, nil
}

// verifyPythonVersionInTag 检查指定 tag 下是否包含某个 Python 版本的安装文件
func verifyPythonVersionInTag(version, tag string) bool {
	url := fmt.Sprintf("https://api.github.com/repos/indygreg/python-build-standalone/releases/tags/%s", tag)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var release struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return false
	}

	searchStr := fmt.Sprintf("cpython-%s+", version)
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, searchStr) && strings.Contains(asset.Name, "install_only") {
			return true
		}
	}
	return false
}

// ListRemoteVersions returns available versions for a runtime from remote sources.
// useMirror=true 时优先使用国内镜像源。
func ListRemoteVersions(rt string, useMirror bool) ([]VersionInfo, error) {
	switch rt {
	case "go":
		return listRemoteGo(useMirror)
	case "node":
		return listRemoteNode(useMirror)
	case "python":
		return listRemotePython(useMirror)
	case "pnpm":
		return listRemotePnpm(useMirror)
	case "git":
		return listRemoteGit()
	case "yarn":
		return listRemoteYarn(useMirror)
	case "rust":
		return listRemoteRust(useMirror)
	case "bun":
		return listRemoteBun(useMirror)
	case "deno":
		return listRemoteDeno(useMirror)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", rt)
	}
}

// listRemoteGit 从 GitHub API 或 npmmirror 获取 Git for Windows 的可用版本列表
func listRemoteGit() ([]VersionInfo, error) {
	// 先尝试 npmmirror 目录 API（国内稳定），失败再回退 GitHub API
	if versions, err := listRemoteGitMirror(); err == nil && len(versions) > 0 {
		return versions, nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/git-for-windows/git/releases?per_page=20", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch git versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		TagName     string `json:"tag_name"` // e.g. "v2.44.0.windows.1"
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	// 从 tag 提取纯版本号，如 "v2.44.0.windows.1" → "2.44.0"
	tagRe := regexp.MustCompile(`^v?(\d+\.\d+\.\d+)`)
	seen := make(map[string]bool)
	var versions []VersionInfo

	for _, r := range releases {
		if r.Prerelease {
			continue
		}
		m := tagRe.FindStringSubmatch(r.TagName)
		if len(m) < 2 {
			continue
		}
		ver := m[1]
		if seen[ver] {
			continue
		}
		seen[ver] = true
		date := ""
		if len(r.PublishedAt) >= 10 {
			date = r.PublishedAt[:10]
		}
		versions = append(versions, VersionInfo{Version: ver, Date: date})
	}

	return versions, nil
}

// listRemoteGitMirror 通过 npmmirror 目录 API 获取 Git for Windows 版本列表
// npmmirror 镜像地址: https://registry.npmmirror.com/-/binary/git-for-windows/
func listRemoteGitMirror() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://registry.npmmirror.com/-/binary/git-for-windows/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("npmmirror git API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// npmmirror 目录 API 返回 JSON 数组: [{"name":"v2.44.0.windows.1/","date":"..."},...]
	var entries []struct {
		Name string `json:"name"`
		Date string `json:"date"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}

	tagRe := regexp.MustCompile(`^v(\d+\.\d+\.\d+)`)
	seen := make(map[string]bool)
	var versions []VersionInfo

	for _, e := range entries {
		m := tagRe.FindStringSubmatch(e.Name)
		if len(m) < 2 {
			continue
		}
		ver := m[1]
		if seen[ver] {
			continue
		}
		// 过滤预发版本（包含 -rc, -beta, -alpha 等后缀）
		if strings.Contains(strings.ToLower(e.Name), "-rc") ||
			strings.Contains(strings.ToLower(e.Name), "-beta") ||
			strings.Contains(strings.ToLower(e.Name), "-alpha") ||
			strings.Contains(strings.ToLower(e.Name), "-prerelease") {
			continue
		}
		seen[ver] = true
		date := ""
		if len(e.Date) >= 10 {
			date = e.Date[:10]
		}
		versions = append(versions, VersionInfo{Version: ver, Date: date})
	}

	// 按版本号降序排列（确保 latest 取到真正的最新版）
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// listRemotePnpm 从 npm registry 获取 pnpm 可用版本列表
func listRemotePnpm(useMirror bool) ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	registryURL := "https://registry.npmjs.org/pnpm"
	if useMirror {
		registryURL = "https://registry.npmmirror.com/pnpm"
	}
	resp, err := client.Get(registryURL)
	if err != nil {
		return nil, fmt.Errorf("fetch pnpm versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type pnpmVersionMeta struct {
		Version string `json:"version"`
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	var meta struct {
		Versions map[string]pnpmVersionMeta `json:"versions"`
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for ver := range meta.Versions {
		// 过滤预发版和旧版本（< 6）
		parts := strings.Split(ver, ".")
		if len(parts) < 1 {
			continue
		}
		var major int
		fmt.Sscanf(parts[0], "%d", &major)
		if major < 6 {
			continue
		}
		// 过滤预发版（包含 - 的版本号）
		if strings.Contains(ver, "-") {
			continue
		}
		versions = append(versions, VersionInfo{Version: ver})
	}

	// 按版本降序排序
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// GetPnpmNodeRequirement 从 npm registry 查询指定 pnpm 版本的 engines.node 字段
// 返回如 ">=18.12" 的字符串，查询失败返回空字符串
func GetPnpmNodeRequirement(pnpmVer string, useMirror bool) string {
	client := &http.Client{Timeout: 10 * time.Second}
	registryURL := fmt.Sprintf("https://registry.npmjs.org/pnpm/%s", pnpmVer)
	if useMirror {
		registryURL = fmt.Sprintf("https://registry.npmmirror.com/pnpm/%s", pnpmVer)
	}
	resp, err := client.Get(registryURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var meta struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return ""
	}
	return meta.Engines.Node
}

func listRemoteGo(useMirror bool) ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	// golang.google.cn 在国内可直接访问，与 go.dev 数据同步
	baseURL := "https://go.dev/dl/?mode=json&include=all"
	if useMirror {
		baseURL = "https://golang.google.cn/dl/?mode=json&include=all"
	}
	resp, err := client.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch Go versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, r := range releases {
		ver := strings.TrimPrefix(r.Version, "go")
		if seen[ver] || !r.Stable {
			continue
		}
		seen[ver] = true
		versions = append(versions, VersionInfo{Version: ver})
	}

	return versions, nil
}

func listRemoteNode(useMirror bool) ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "https://nodejs.org/dist/index.json"
	if useMirror {
		baseURL = "https://registry.npmmirror.com/-/binary/node/index.json"
	}
	resp, err := client.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("fetch Node versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		Version string `json:"version"`
		LTS     any    `json:"lts"` // can be false or string like "Iron"
		Date    string `json:"date"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for _, r := range releases {
		ver := strings.TrimPrefix(r.Version, "v")
		isLTS := false
		if ltsStr, ok := r.LTS.(string); ok && ltsStr != "" {
			isLTS = true
		}
		versions = append(versions, VersionInfo{
			Version: ver,
			LTS:     isLTS,
			Date:    r.Date,
		})
	}

	return versions, nil
}

func listRemotePython(useMirror bool) ([]VersionInfo, error) {
	if useMirror {
		return listRemotePythonMirror()
	}
	// 先尝试 GitHub API，失败时自动回退到镜像源（避免 GitHub API 504/超时导致无法使用）
	versions, err := listRemotePythonGitHub()
	if err != nil {
		logger.Verbose("  GitHub API failed (%v), falling back to npmmirror...", err)
		return listRemotePythonMirror()
	}
	return versions, nil
}

// listRemotePythonGitHub 从 GitHub API 获取 Python 可用版本列表
func listRemotePythonGitHub() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/indygreg/python-build-standalone/releases?per_page=20")
	if err != nil {
		return nil, fmt.Errorf("fetch Python versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}

	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	// Extract unique Python versions from asset names
	versionRe := regexp.MustCompile(`cpython-(\d+\.\d+\.\d+)\+`)
	seen := make(map[string]bool)
	var versions []VersionInfo

	for _, release := range releases {
		for _, asset := range release.Assets {
			if !strings.Contains(asset.Name, "install_only") {
				continue
			}
			matches := versionRe.FindStringSubmatch(asset.Name)
			if len(matches) < 2 {
				continue
			}
			ver := matches[1]
			if seen[ver] {
				continue
			}
			seen[ver] = true

			// Cache the build tag for later use
			pythonBuildTagCache[ver] = release.TagName

			date := ""
			if len(release.PublishedAt) >= 10 {
				date = release.PublishedAt[:10]
			}
			versions = append(versions, VersionInfo{
				Version: ver,
				Date:    date,
			})
		}
	}

	// Sort by version descending
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// listRemotePythonMirror 从 npmmirror 获取 Python 可用版本列表
// 遍历所有 build tag 目录，获取完整的 Python 版本列表
func listRemotePythonMirror() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// 第一步：获取所有 tag 目录
	tagsURL := "https://registry.npmmirror.com/-/binary/python-build-standalone/"
	resp, err := client.Get(tagsURL)
	if err != nil {
		return nil, fmt.Errorf("fetch Python versions from mirror: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("npmmirror returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tagEntries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &tagEntries); err != nil {
		return nil, fmt.Errorf("parse mirror tags: %w", err)
	}

	// 收集所有 tag（日期格式：20241016）
	var tags []string
	for _, e := range tagEntries {
		name := strings.TrimSuffix(e.Name, "/")
		if len(name) == 8 {
			tags = append(tags, name)
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("no build tags found on npmmirror")
	}

	// 第二步：遍历所有 tag，收集当前平台可用的 Python 版本
	versionRe := regexp.MustCompile(`cpython-(\d+\.\d+\.\d+)\+`)
	seen := make(map[string]bool)
	var versions []VersionInfo

	// 根据当前平台构建文件名特征字符串用于过滤
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	pyArch := goarch
	switch goarch {
	case "amd64":
		pyArch = "x86_64"
	case "arm64":
		pyArch = "aarch64"
	case "386":
		pyArch = "i686"
	}
	var platformFilter string
	switch goos {
	case "windows":
		platformFilter = fmt.Sprintf("%s-pc-windows-msvc", pyArch)
	case "darwin":
		platformFilter = fmt.Sprintf("%s-apple-darwin", pyArch)
	default:
		platformFilter = fmt.Sprintf("%s-unknown-linux-gnu", pyArch)
	}

	// 按 tag 降序遍历（最新的优先），限制遍历数量避免超时
	maxTags := 10
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	if len(tags) > maxTags {
		tags = tags[:maxTags]
	}

	for _, tag := range tags {
		filesURL := fmt.Sprintf("https://registry.npmmirror.com/-/binary/python-build-standalone/%s/", tag)
		resp2, err := client.Get(filesURL)
		if err != nil {
			continue // 跳过无法访问的 tag
		}
		if resp2.StatusCode != 200 {
			resp2.Body.Close()
			continue
		}

		body2, err := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if err != nil {
			continue
		}

		var fileEntries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body2, &fileEntries); err != nil {
			continue
		}

		// 从文件名提取 Python 版本（只匹配当前平台的安装包）
		for _, e := range fileEntries {
			if !strings.Contains(e.Name, "install_only") {
				continue
			}
			// 平台过滤：只保留当前平台的文件，避免列出其他平台的版本号
			if !strings.Contains(e.Name, platformFilter) {
				continue
			}
			matches := versionRe.FindStringSubmatch(e.Name)
			if len(matches) < 2 {
				continue
			}
			ver := matches[1]
			if seen[ver] {
				continue
			}
			seen[ver] = true
			// 缓存 build tag（每个版本使用找到它的第一个 tag）
			if _, exists := pythonBuildTagCache[ver]; !exists {
				pythonBuildTagCache[ver] = tag
			}
			versions = append(versions, VersionInfo{Version: ver})
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no Python versions found on npmmirror for %s-%s", goos, goarch)
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// IsExactVersion 判断版本号是否是精确的三段式版本（如 8.15.9）
// 如果是，则不需要解析
func IsExactVersion(version string) bool {
	// 预发版（如 "1.2.3-beta"）不视为精确版本，需要走解析流程
	if strings.Contains(version, "-") {
		return false
	}
	// build metadata（如 "1.2.3+build"）也不视为精确版本
	if strings.Contains(version, "+") {
		return false
	}
	parts := strings.Split(version, ".")
	return len(parts) >= 3
}

// ResolveExactVersion 将模糊版本（如 "8"、"20"、"1.22"、"latest"）解析为精确版本号。
// 如果已经是精确版本（三段式），直接返回原值。
// useMirror=true 时使用国内镜像源。
func ResolveExactVersion(rt, version string, useMirror bool) (string, error) {
	// 已经是精确版本，无需解析
	if IsExactVersion(version) {
		return version, nil
	}

	logger.Verbose("  Resolving %s@%s to exact version...", rt, version)

	versions, err := ListRemoteVersions(rt, useMirror)
	if err != nil {
		return "", fmt.Errorf("resolve version: %w", err)
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for %s", rt)
	}

	// "latest" → 取第一个（列表已按降序排列）
	if version == "latest" || version == "lts" {
		// node 的 lts 取第一个 LTS 版本
		if rt == "node" && version == "lts" {
			for _, v := range versions {
				if v.LTS {
					logger.Verbose("  Resolved %s@lts → %s", rt, v.Version)
					return v.Version, nil
				}
			}
		}
		logger.Verbose("  Resolved %s@%s → %s", rt, version, versions[0].Version)
		return versions[0].Version, nil
	}

	// 前缀匹配：找到最新的匹配版本
	// 例如 "8" 匹配 "8.x.x"，"1.22" 匹配 "1.22.x"
	prefix := version + "."
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefix) {
			logger.Verbose("  Resolved %s@%s → %s", rt, version, v.Version)
			return v.Version, nil
		}
	}

	// 未找到匹配版本：给出有用的错误提示（列出可用版本，帮助用户选择）
	latest := versions[0].Version
	hint := fmt.Sprintf("no matching version found for %s@%s", rt, version)
	hint += fmt.Sprintf("\n  Latest available: %s", latest)
	// 列出最近 5 个版本
	n := 5
	if n > len(versions) {
		n = len(versions)
	}
	var sample []string
	for i := 0; i < n; i++ {
		sample = append(sample, versions[i].Version)
	}
	hint += fmt.Sprintf("\n  Recent versions: %s", strings.Join(sample, ", "))
	hint += fmt.Sprintf("\n  Run `pvm list-remote %s` to see all available versions", rt)

	// Python 2 特殊提示：python-build-standalone 仅提供 Python 3.8+
	if rt == "python" && (version == "2" || strings.HasPrefix(version, "2.")) {
		hint += "\n  Note: pvm uses python-build-standalone which only provides Python 3.8+."
		hint += "\n        Python 2.7 reached EOL on 2020-01-01 and is not available."
		hint += "\n        Please use Python 3, e.g.: pvm install python@3.12"
		hint += "\n        Or use system Python 2 if installed: pvm use python --system"
	}
	return "", fmt.Errorf("%s", hint)
}

// GetMirrorURL returns a mirror URL for users in China
func GetMirrorURL(rt string, version string) (*RuntimeInfo, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch rt {
	case "go":
		info, err := getGoInfo(version, goos, goarch)
		if err != nil {
			return nil, err
		}
		// golang.google.cn 是 Google 在国内提供的官方镜像，稳定可靠
		info.URL = strings.Replace(info.URL, "https://go.dev/dl/", "https://golang.google.cn/dl/", 1)
		return info, nil
	case "node":
		info, err := getNodeInfo(version, goos, goarch)
		if err != nil {
			return nil, err
		}
		// npmmirror Node.js 镜像格式: https://npmmirror.com/mirrors/node/v{version}/node-v{version}-{os}-{arch}.{ext}
		info.URL = strings.Replace(info.URL, "https://nodejs.org/dist/", "https://npmmirror.com/mirrors/node/", 1)
		return info, nil
	case "python":
		// 使用 npmmirror 的 python-build-standalone 镜像
		return getPythonInfoWithMirror(version, goos, goarch, true)
	case "pnpm":
		// 使用 npmmirror 镜像
		url := fmt.Sprintf("https://registry.npmmirror.com/pnpm/-/pnpm-%s.tgz", version)
		return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
	case "git":
		// Windows 使用 npmmirror 的 git-for-windows 镜像
		if goos == "windows" {
			arch := "64-bit"
			if goarch == "386" {
				arch = "32-bit"
			}
			if goarch == "arm64" {
				arch = "arm64"
			}
			// Git 完整版 (tar.bz2 格式)，包含 Git Bash、SSH 等所有工具
			url := fmt.Sprintf(
				"https://registry.npmmirror.com/-/binary/git-for-windows/v%s.windows.1/Git-%s-%s.tar.bz2",
				version, version, arch,
			)
			return &RuntimeInfo{URL: url, ArchiveType: "tar.bz2"}, nil
		}
		// Linux 使用 kernel.org（国内访问尚可）
		return getGitInfo(version, goos, goarch)
	case "yarn":
		return getYarnInfoMirror(version)
	case "rust":
		return getRustInfoMirror(version, goos, goarch)
	case "bun":
		return getBunInfoMirror(version, goos, goarch)
	case "deno":
		return getDenoInfoMirror(version, goos, goarch)
	default:
		return GetDownloadInfo(rt, version)
	}
}
