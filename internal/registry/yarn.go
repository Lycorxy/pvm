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

// getYarnInfo 返回 Yarn 的下载信息
// 同时支持 Yarn v1 (Classic) 和 Yarn v2+ (Berry)
//
// 策略：
//   - v1 (1.x.x): 从 npm registry 下载 yarn-{version}.tgz（包含 bin/ 和 lib/）
//   - v2+ (2.x.x+): 从 npm registry @yarnpkg/cli-dist 下载（同样 tgz 格式）
//
// 实际上两个版本系列都可以走 npm registry yarn 包：
//
//	https://registry.npmjs.org/yarn/-/yarn-{version}.tgz
//
// 这里包含了完整的 yarn CLI 入口
func getYarnInfo(version string) (*RuntimeInfo, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/yarn/-/yarn-%s.tgz", version)
	return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// getYarnInfoMirror 国内镜像下载
func getYarnInfoMirror(version string) (*RuntimeInfo, error) {
	url := fmt.Sprintf("https://registry.npmmirror.com/yarn/-/yarn-%s.tgz", version)
	return &RuntimeInfo{URL: url, ArchiveType: "tar.gz"}, nil
}

// listRemoteYarn 从 npm registry 获取 yarn 可用版本列表
func listRemoteYarn(useMirror bool) ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	registryURL := "https://registry.npmjs.org/yarn"
	if useMirror {
		registryURL = "https://registry.npmmirror.com/yarn"
	}
	resp, err := client.Get(registryURL)
	if err != nil {
		return nil, fmt.Errorf("fetch yarn versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var meta struct {
		Versions map[string]struct {
			Version string `json:"version"`
		} `json:"versions"`
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for ver := range meta.Versions {
		// 过滤预发版
		if strings.Contains(ver, "-") {
			continue
		}
		// 过滤过老版本（< 1.0）
		parts := strings.Split(ver, ".")
		if len(parts) < 1 {
			continue
		}
		var major int
		fmt.Sscanf(parts[0], "%d", &major)
		if major < 1 {
			continue
		}
		versions = append(versions, VersionInfo{Version: ver})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}
