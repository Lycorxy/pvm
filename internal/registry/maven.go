package registry

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/semver"
)

// getMavenInfo 返回 Maven 的下载信息
//
// 优先使用 Maven Central（repo1.maven.org），它包含所有历史版本（包括 3.x 和 4.x）。
// 而 dlcdn.apache.org / 镜像站只保留当前活跃版本，archive.apache.org 经常超时限速。
//
// URL 模板: https://repo1.maven.org/maven2/org/apache/maven/apache-maven/{ver}/apache-maven-{ver}-bin.{ext}
func getMavenInfo(version string) (*RuntimeInfo, error) {
	ext := mavenExt()
	url := fmt.Sprintf(
		"https://repo1.maven.org/maven2/org/apache/maven/apache-maven/%s/apache-maven-%s-bin.%s",
		version, version, ext,
	)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

// getMavenInfoMirror 国内镜像
// 阿里云的 Maven Central 镜像保留了完整的历史版本
func getMavenInfoMirror(version string) (*RuntimeInfo, error) {
	ext := mavenExt()
	url := fmt.Sprintf(
		"https://maven.aliyun.com/repository/central/org/apache/maven/apache-maven/%s/apache-maven-%s-bin.%s",
		version, version, ext,
	)
	return &RuntimeInfo{URL: url, ArchiveType: ext}, nil
}

func mavenExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// listRemoteMaven 从 Maven Central 元数据获取可用版本列表
// metadata: https://repo1.maven.org/maven2/org/apache/maven/apache-maven/maven-metadata.xml
func listRemoteMaven() ([]VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := "https://repo1.maven.org/maven2/org/apache/maven/apache-maven/maven-metadata.xml"
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch maven versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var meta struct {
		Versioning struct {
			Versions struct {
				Version []string `xml:"version"`
			} `xml:"versions"`
			LastUpdated string `xml:"lastUpdated"`
		} `xml:"versioning"`
	}
	if err := xml.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse maven metadata: %w", err)
	}

	var versions []VersionInfo
	seen := make(map[string]bool)
	for _, v := range meta.Versioning.Versions.Version {
		// 过滤预发版（alpha/beta/rc）
		if strings.Contains(v, "-") {
			continue
		}
		// 同时支持 3.x 和 4.x 稳定版
		if !strings.HasPrefix(v, "3.") && !strings.HasPrefix(v, "4.") {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		versions = append(versions, VersionInfo{Version: v})
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(semver.Parse(versions[i].Version), semver.Parse(versions[j].Version)) > 0
	})

	return versions, nil
}
