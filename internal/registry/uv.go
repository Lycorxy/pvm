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

// getUvInfo 返回 uv 的下载信息
// 官方: https://github.com/astral-sh/uv/releases/download/{version}/uv-{target}.{ext}
// 镜像: https://registry.npmmirror.com/-/binary/uv/{version}/uv-{target}.{ext}
//
// uv 是 Rust 写的独立二进制，不依赖 Python
func getUvInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	target, ext, err := uvTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://github.com/astral-sh/uv/releases/download/%s/uv-%s.%s",
		version, target, ext,
	)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// getUvInfoMirror 国内镜像下载（中科大 USTC GitHub Release 镜像）
// npmmirror 未收录 uv，使用 USTC 镜像替代
func getUvInfoMirror(version, goos, goarch string) (*RuntimeInfo, error) {
	target, ext, err := uvTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://mirrors.ustc.edu.cn/github-release/astral-sh/uv/%s/uv-%s.%s",
		version, target, ext,
	)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// uvTarget 返回 uv 的 target 字符串和压缩格式
func uvTarget(goos, goarch string) (target, ext string, err error) {
	switch goos {
	case "windows":
		ext = "zip"
		switch goarch {
		case "amd64":
			target = "x86_64-pc-windows-msvc"
		case "arm64":
			target = "aarch64-pc-windows-msvc"
		default:
			return "", "", fmt.Errorf("unsupported arch for uv on windows: %s", goarch)
		}
	case "linux":
		ext = "tar.gz"
		switch goarch {
		case "amd64":
			target = "x86_64-unknown-linux-gnu"
		case "arm64":
			target = "aarch64-unknown-linux-gnu"
		default:
			return "", "", fmt.Errorf("unsupported arch for uv on linux: %s", goarch)
		}
	case "darwin":
		ext = "tar.gz"
		switch goarch {
		case "amd64":
			target = "x86_64-apple-darwin"
		case "arm64":
			target = "aarch64-apple-darwin"
		default:
			return "", "", fmt.Errorf("unsupported arch for uv on darwin: %s", goarch)
		}
	default:
		return "", "", fmt.Errorf("unsupported OS for uv: %s", goos)
	}
	return target, ext, nil
}

// listRemoteUv 从 GitHub Releases API 获取 uv 可用版本列表
func listRemoteUv() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/astral-sh/uv/releases?per_page=50", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch uv versions: %w", err)
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
		// uv 的 tag 是 "0.4.0"（无 v 前缀）
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
