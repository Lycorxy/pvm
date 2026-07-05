// Package bun 提供 Bun runtime 插件
package bun

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

const name = "bun"

// BunPlugin Bun 插件实现
type BunPlugin struct {
	plugin.BasePlugin
}

// New 创建 Bun 插件
func New() *BunPlugin {
	return &BunPlugin{
		BasePlugin: plugin.NewBasePlugin(
			name,
			plugin.TypeRuntime,
			false,
			"bun",
			[]string{"bun", "bunx"},
		),
	}
}

// GetDownloadInfo 返回 Bun 下载信息
func (p *BunPlugin) GetDownloadInfo(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.bunTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://github.com/oven-sh/bun/releases/download/bun-v%s/bun-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// GetDownloadInfoMirror 使用国内镜像
func (p *BunPlugin) GetDownloadInfoMirror(version, goos, goarch string) (*registry.RuntimeInfo, error) {
	target, err := p.bunTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(
		"https://registry.npmmirror.com/-/binary/bun/bun-v%s/bun-%s.zip",
		version, target,
	)
	return &registry.RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// bunTarget 将 GOOS/GOARCH 映射为 Bun 的 target 字符串
func (p *BunPlugin) bunTarget(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "windows-x64", nil
		default:
			return "", fmt.Errorf("bun on windows only supports x64 (got %s)", goarch)
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-x64", nil
		case "arm64":
			return "linux-aarch64", nil
		default:
			return "", fmt.Errorf("unsupported arch for bun on linux: %s", goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "darwin-x64", nil
		case "arm64":
			return "darwin-aarch64", nil
		default:
			return "", fmt.Errorf("unsupported arch for bun on darwin: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS for bun: %s", goos)
	}
}

// ListRemoteVersions 返回 Bun 可用版本列表
func (p *BunPlugin) ListRemoteVersions(stableOnly bool) ([]registry.VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/oven-sh/bun/releases?per_page=50", nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bun versions: %w", err)
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
		ver := strings.TrimPrefix(r.TagName, "bun-v")
		ver = strings.TrimPrefix(ver, "v")
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

// Install 安装 Bun
func (p *BunPlugin) Install(extractTmp, version, installDir string) error {
	// Bun 解压后目录结构需要处理
	return nil
}

// Verify 验证 Bun 安装
func (p *BunPlugin) Verify(installDir string) error {
	return nil
}
