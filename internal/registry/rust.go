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

// listRemoteRust 从 rust-lang/rust 的 GitHub Releases API 获取版本列表
func listRemoteRust() ([]VersionInfo, error) {
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
