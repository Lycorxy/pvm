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

// getJavaInfo 返回 Java/JDK 的下载信息（使用 Eclipse Temurin / Adoptium）
// API: https://api.adoptium.net/v3/binary/version/jdk-{version}/{os}/{arch}/jdk/hotspot/normal/eclipse?project=jdk
//
// 注意：Adoptium API 会自动 302 跳转到实际的下载链接（GitHub Releases），
// 我们的下载工具支持自动跟随 redirect。
//
// version 期望是完整的 Java 版本号，如：
//   - "21.0.5+11"   → API tag: jdk-21.0.5+11
//   - "21.0.5"      → API tag: jdk-21.0.5+11（需要先解析）
//   - "17.0.13"     → 类似
func getJavaInfo(version, goos, goarch string) (*RuntimeInfo, error) {
	osStr, archStr, ext, err := javaTarget(goos, goarch)
	if err != nil {
		return nil, err
	}

	// 把版本格式化为 Adoptium 期望的 tag
	tag, err := resolveAdoptiumTag(version)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://api.adoptium.net/v3/binary/version/jdk-%s/%s/%s/jdk/hotspot/normal/eclipse?project=jdk",
		tag, osStr, archStr,
	)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// getJavaInfoMirror 镜像下载（清华 TUNA）
// 注意：清华镜像目录结构与 Adoptium 不完全一致，这里降级使用 Adoptium API（其本身已是 CDN）
func getJavaInfoMirror(version, goos, goarch string) (*RuntimeInfo, error) {
	// Adoptium 本身就是全球 CDN，国内访问也较快，直接复用
	return getJavaInfo(version, goos, goarch)
}

// javaTarget 返回 Adoptium API 期望的 (os, arch) 和文件扩展名
func javaTarget(goos, goarch string) (osStr, archStr, ext string, err error) {
	switch goos {
	case "windows":
		osStr = "windows"
		ext = "zip"
	case "linux":
		osStr = "linux"
		ext = "tar.gz"
	case "darwin":
		osStr = "mac"
		ext = "tar.gz"
	default:
		return "", "", "", fmt.Errorf("unsupported OS for java: %s", goos)
	}

	switch goarch {
	case "amd64":
		archStr = "x64"
	case "arm64":
		archStr = "aarch64"
	default:
		return "", "", "", fmt.Errorf("unsupported arch for java: %s", goarch)
	}
	return osStr, archStr, ext, nil
}

// resolveAdoptiumTag 把用户输入的 java 版本（如 "21"、"21.0.5"）转成 Adoptium release tag（如 "21.0.5+11"）
// Adoptium 的 tag 形如 jdk-21.0.5+11, jdk-17.0.13+11
func resolveAdoptiumTag(version string) (string, error) {
	// 已经是完整 tag（含 build number），直接用
	if strings.Contains(version, "+") {
		return version, nil
	}

	// 解析 feature 版本（如 "21" 或 "21.0.5"）
	major := semver.Parse(version).Major
	if major == 0 {
		return "", fmt.Errorf("invalid java version: %s", version)
	}

	// 通过 Adoptium assets API 查询匹配的 tag
	client := &http.Client{Timeout: 15 * time.Second}
	apiURL := fmt.Sprintf(
		"https://api.adoptium.net/v3/assets/feature_releases/%d/ga?image_type=jdk&jvm_impl=hotspot&page=0&page_size=20&project=jdk&sort_method=DEFAULT&sort_order=DESC&vendor=eclipse",
		major,
	)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pvm")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("query adoptium api: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var releases []struct {
		ReleaseName string `json:"release_name"` // 如 "jdk-21.0.5+11"
		VersionData struct {
			Major    int    `json:"major"`
			Minor    int    `json:"minor"`
			Security int    `json:"security"`
			Build    int    `json:"build"`
			Semver   string `json:"semver"`
		} `json:"version_data"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse adoptium response: %w", err)
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("no java %s release found on adoptium", version)
	}

	// 如果用户给的是精确版本（如 "21.0.5"），找到匹配的；否则用最新的（已按 DESC 排序）
	for _, r := range releases {
		tag := strings.TrimPrefix(r.ReleaseName, "jdk-")
		// 精确版本匹配
		if !IsExactVersion(version) {
			// 用户给了模糊版本（如 "21"），直接返回第一个（最新的）
			return tag, nil
		}
		// 精确版本：tag 应以 version 开头
		if strings.HasPrefix(tag, version+"+") || tag == version {
			return tag, nil
		}
	}

	return "", fmt.Errorf("java version %s not found in adoptium releases", version)
}

// JavaCleanVersion 把 Java 版本号去掉 +build 后缀（用于安装目录命名）
//   "21.0.5+11" -> "21.0.5"
//   "21.0.5"    -> "21.0.5"
func JavaCleanVersion(v string) string {
	return stripBuild(v)
}

// listRemoteJava 列出 Adoptium 可用的 Java 版本
func listRemoteJava() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// 1. 获取所有 LTS + 当前 feature 版本号
	releasesURL := "https://api.adoptium.net/v3/info/available_releases"
	resp, err := client.Get(releasesURL)
	if err != nil {
		return nil, fmt.Errorf("fetch java available releases: %w", err)
	}
	defer resp.Body.Close()

	var rel struct {
		AvailableReleases    []int `json:"available_releases"`
		AvailableLtsReleases []int `json:"available_lts_releases"`
		MostRecentLts        int   `json:"most_recent_lts"`
		MostRecentFeature    int   `json:"most_recent_feature_release"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse java releases: %w", err)
	}

	ltsSet := make(map[int]bool)
	for _, v := range rel.AvailableLtsReleases {
		ltsSet[v] = true
	}

	// 2. 对每个 feature 版本，查询其最新的具体版本
	var versions []VersionInfo
	for _, major := range rel.AvailableReleases {
		// 只取最新一个具体版本
		apiURL := fmt.Sprintf(
			"https://api.adoptium.net/v3/assets/feature_releases/%d/ga?image_type=jdk&jvm_impl=hotspot&page=0&page_size=1&project=jdk&sort_method=DEFAULT&sort_order=DESC&vendor=eclipse",
			major,
		)
		r, err := client.Get(apiURL)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var assets []struct {
			ReleaseName string `json:"release_name"`
			Timestamp   string `json:"timestamp"`
			VersionData struct {
				Semver string `json:"semver"`
			} `json:"version_data"`
		}
		if err := json.Unmarshal(body, &assets); err != nil {
			continue
		}
		if len(assets) == 0 {
			continue
		}
		fullTag := strings.TrimPrefix(assets[0].ReleaseName, "jdk-")
		// 安装目录使用 clean version（不含 +build），方便文件系统兼容
		cleanVer := stripBuild(fullTag)
		date := ""
		if len(assets[0].Timestamp) >= 10 {
			date = assets[0].Timestamp[:10]
		}
		versions = append(versions, VersionInfo{
			Version: cleanVer,
			LTS:     ltsSet[major],
			Date:    date,
		})
	}

	// 按主版本号降序排列
	sort.Slice(versions, func(i, j int) bool {
		mi := semver.Parse(stripBuild(versions[i].Version)).Major
		mj := semver.Parse(stripBuild(versions[j].Version)).Major
		return mi > mj
	})

	return versions, nil
}

// stripBuild 去除版本号末尾的 +N 部分
func stripBuild(v string) string {
	if i := strings.Index(v, "+"); i > 0 {
		return v[:i]
	}
	return v
}
