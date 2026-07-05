// Package deno 提供 Deno runtime 插件
package deno

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/plugin"
	"github.com/pvm/pvm/internal/registry"
	"github.com/pvm/pvm/internal/semver"
)

const name = "deno"

// DenoPlugin Deno 插件实现
type DenoPlugin struct {
	plugin.BasePlugin
}

// New 创建 Deno 插件
func New() *DenoPlugin {
	return &DenoPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeRuntime,
			false,
			"deno",
			[]string{"deno"},
		),
	}
}

// GetDownloadInfo 返回 Deno 下载信息
func (p *DenoPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.denoTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://github.com/denoland/deno/releases/download/v%s/deno-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// GetDownloadInfoMirror 使用国内镜像
func (p *DenoPlugin) GetDownloadInfoMirror(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.denoTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://registry.npmmirror.com/-/binary/deno/v%s/deno-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// denoTarget 将 GOOS/GOARCH 映射为 Deno 的 target 字符串
func (p *DenoPlugin) denoTarget(goos, goarch string) (string, error) {
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

// ListRemoteVersions 返回 Deno 可用版本列表
func (p *DenoPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
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

	var versions []registry.VersionInfo
	seen := make(map[string]bool)
	for _, r := range releases {
		if r.Prerelease || r.Draft {
			continue
		}
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
		versions = append(versions, registry.VersionInfo{Version: ver, Date: date})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}

// Install 安装 Deno
func (p *DenoPlugin) Install(extractTmp, version, installDir string) error {
	return nil
}

// Verify 验证 Deno 安装
func (p *DenoPlugin) Verify(installDir string) error {
	return nil
}
