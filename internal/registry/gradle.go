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

// getGradleInfo 返回 Gradle 的下载信息
// 官方: https://services.gradle.org/distributions/gradle-{version}-bin.zip
// 镜像: https://mirrors.cloud.tencent.com/gradle/gradle-{version}-bin.zip
func getGradleInfo(version string) (*RuntimeInfo, error) {
	url := fmt.Sprintf(
		"https://services.gradle.org/distributions/gradle-%s-bin.zip",
		version,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// getGradleInfoMirror 国内镜像
func getGradleInfoMirror(version string) (*RuntimeInfo, error) {
	url := fmt.Sprintf(
		"https://mirrors.cloud.tencent.com/gradle/gradle-%s-bin.zip",
		version,
	)
	return &RuntimeInfo{URL: url, ArchiveType: "zip"}, nil
}

// listRemoteGradle 列出 Gradle 可用版本
// API: https://services.gradle.org/versions/all
func listRemoteGradle() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := "https://services.gradle.org/versions/all"
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch gradle versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []struct {
		Version       string `json:"version"`
		BuildTime     string `json:"buildTime"`
		ReleaseNightly bool  `json:"releaseNightly"`
		Nightly       bool   `json:"nightly"`
		Snapshot      bool   `json:"snapshot"`
		ActiveRC      bool   `json:"activeRc"`
		Broken        bool   `json:"broken"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parse gradle response: %w", err)
	}

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, r := range releases {
		if r.Nightly || r.ReleaseNightly || r.Snapshot || r.Broken {
			continue
		}
		// 过滤 RC、milestone 等
		if strings.Contains(r.Version, "-") {
			continue
		}
		if seen[r.Version] {
			continue
		}
		seen[r.Version] = true
		date := ""
		if len(r.BuildTime) >= 8 {
			// "20240312134803+0000" → "2024-03-12"
			t := r.BuildTime
			if len(t) >= 8 {
				date = fmt.Sprintf("%s-%s-%s", t[:4], t[4:6], t[6:8])
			}
		}
		versions = append(versions, VersionInfo{Version: r.Version, Date: date})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}
