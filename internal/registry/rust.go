package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/semver"
)

// getRustInfo 返回 Rust 的下载信息
// 官方: https://static.rust-lang.org/dist/rust-{version}-{target}.tar.gz
// 镜像: https://rsproxy.cn/dist/rust-{version}-{target}.tar.gz
func getRustInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	target, err := rustTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://static.rust-lang.org/dist/rust-%s-%s.tar.gz",
		version, target,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// getRustInfoMirror 使用国内镜像（rsproxy.cn 字节跳动）
func getRustInfoMirror(version, goos, goarch string) (*RuntimeInfo, error) {
	target, err := rustTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://rsproxy.cn/dist/rust-%s-%s.tar.gz",
		version, target,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// rustTarget 返回 Rust 的 target triple
func rustTarget(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on windows: %s", goarch)
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		case "arm64":
			return "aarch64-unknown-linux-gnu", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on linux: %s", goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", nil
		case "arm64":
			return "aarch64-apple-darwin", nil
		default:
			return "", fmt.Errorf("unsupported arch for rust on darwin: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS for rust: %s", goos)
	}
}

// rustReleaseRe 匹配 Rust GitHub release tag，如 "1.78.0"
var rustReleaseRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// listRemoteRust 获取 Rust 可用版本列表
// useMirror=true 时优先使用 rsproxy.cn 镜像，失败回退到 GitHub API
func listRemoteRust(useMirror bool) ([]VersionInfo, error) {
	if useMirror {
		if versions, err := listRemoteRustMirror(); err == nil && len(versions) > 0 {
			return versions, nil
		}
	}
	return listRemoteRustGitHub()
}

// listRemoteRustMirror 从 rsproxy.cn 镜像获取 Rust 版本列表
// rsproxy.cn 返回 HTML 目录列表，需要解析 <a> 标签
func listRemoteRustMirror() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://rsproxy.cn/dist/")
	if err != nil {
		return nil, fmt.Errorf("fetch rust versions from mirror: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rsproxy API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// rsproxy.cn 返回 HTML 目录列表，格式如:
	// <a href="rust-1.78.0-x86_64-pc-windows-msvc.tar.gz">rust-1.78.0-x86_64-pc-windows-msvc.tar.gz</a>
	html := string(body)
	// 匹配 rust-<version>-<target>.tar.gz 格式的文件名
	re := regexp.MustCompile(`rust-(\d+\.\d+\.\d+)-[a-z0-9_-]+\.tar\.gz`)
	matches := re.FindAllStringSubmatch(html, -1)

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		ver := m[1]
		if seen[ver] {
			continue
		}
		seen[ver] = true
		versions = append(versions, VersionInfo{Version: ver})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// listRemoteRustGitHub 从 rust-lang/rust 的 GitHub Releases API 获取版本列表
func listRemoteRustGitHub() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/rust-lang/rust/releases?per_page=50", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch rust versions: %w", err)
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
		ver := strings.TrimPrefix(r.TagName, "v")
		if !rustReleaseRe.MatchString(ver) {
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
