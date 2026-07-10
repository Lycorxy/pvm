package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/semver"
)

// getDenoInfo 返回 Deno 的下载信息
// 官方: https://github.com/denoland/deno/releases/download/v{version}/deno-{target}.zip
// 镜像: https://registry.npmmirror.com/-/binary/deno/v{version}/deno-{target}.zip
func getDenoInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	target, err := denoTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://github.com/denoland/deno/releases/download/v%s/deno-%s.zip",
		version, target,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// getDenoInfoMirror 国内镜像下载
func getDenoInfoMirror(version, goos, goarch string) (*RuntimeInfo, error) {
	target, err := denoTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://registry.npmmirror.com/-/binary/deno/v%s/deno-%s.zip",
		version, target,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// denoTarget 将 GOOS/GOARCH 映射为 Deno 的 target 字符串
func denoTarget(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return "x86_64-pc-windows-msvc", nil
		}
		return "", fmt.Errorf("deno on windows only supports x64 (got %s)", goarch)
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		case "arm64":
			return "aarch64-unknown-linux-gnu", nil
		default:
			return "", fmt.Errorf("unsupported arch for deno on linux: %s", goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", nil
		case "arm64":
			return "aarch64-apple-darwin", nil
		default:
			return "", fmt.Errorf("unsupported arch for deno on darwin: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS for deno: %s", goos)
	}
}

// listRemoteDeno 获取 Deno 可用版本列表
// useMirror=true 时优先使用 npmmirror 镜像，失败回退到 GitHub API
func listRemoteDeno(useMirror bool) ([]VersionInfo, error) {
	if useMirror {
		if versions, err := listRemoteDenoMirror(); err == nil && len(versions) > 0 {
			return versions, nil
		}
	}
	return listRemoteDenoGitHub()
}

// listRemoteDenoMirror 从 npmmirror 目录 API 获取 Deno 版本列表
// URL: https://registry.npmmirror.com/-/binary/deno/
func listRemoteDenoMirror() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://registry.npmmirror.com/-/binary/deno/")
	if err != nil {
		return nil, fmt.Errorf("fetch deno versions from mirror: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("npmmirror deno API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// npmmirror 目录 API 返回: [{"name":"v1.40.0/","date":"..."},...]
	var entries []struct {
		Name string `json:"name"`
		Date string `json:"date"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, e := range entries {
		// name 格式: "v1.40.0/"
		ver := strings.TrimPrefix(e.Name, "v")
		ver = strings.TrimSuffix(ver, "/")
		// 跳过空值和预发版（含 -）
		if ver == "" || strings.Contains(ver, "-") {
			continue
		}
		if seen[ver] {
			continue
		}
		seen[ver] = true
		date := ""
		if len(e.Date) >= 10 {
			date = e.Date[:10]
		}
		versions = append(versions, VersionInfo{Version: ver, Date: date})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// listRemoteDenoGitHub 从 GitHub Releases API 获取 Deno 可用版本列表
func listRemoteDenoGitHub() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/denoland/deno/releases?per_page=50", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch deno versions: %w", err)
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
		Prerelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, r := range releases {
		if r.Prerelease || r.Draft {
			continue
		}
		// tag 格式: "v1.40.0"
		ver := strings.TrimPrefix(r.TagName, "v")
		if strings.Contains(ver, "-") {
			continue
		}
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

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}
