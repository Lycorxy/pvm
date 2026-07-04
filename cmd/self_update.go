package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/download"
	"github.com/pvm/pvm/internal/logger"
)

// ReleaseRepo 是 pvm 发布仓库（构建时可通过 ldflags 覆盖）
var ReleaseRepo = "lucky-zsh/pvm"
const releaseBaseURL = "https://gitee.com/%s/releases/download/%s/%s"

// runSelfUpdate 从 Gitee Releases 下载最新 pvm 覆盖当前二进制
func runSelfUpdate(args []string) error {
	logger.Info("  → Checking latest release of %s ...", ReleaseRepo)

	tag, err := fetchLatestTag(ReleaseRepo)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}
	logger.Info("  → Latest: %s (current: %s)", tag, Version)

	if strings.TrimPrefix(tag, "v") == strings.TrimPrefix(Version, "v") {
		logger.Info("  ✓ already up-to-date")
		return nil
	}

	assetName := fmt.Sprintf("pvm-%s-%s%s", runtime.GOOS, runtime.GOARCH, config.ExeExt())
	url := fmt.Sprintf(releaseBaseURL,
		ReleaseRepo, tag, assetName)

	tmp := filepath.Join(config.TempDir(), assetName+".new")
	if err := config.EnsureDir(filepath.Dir(tmp)); err != nil {
		return err
	}
	if err := download.DownloadFile(url, tmp); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp, 0755); err != nil {
			return err
		}
	}

	target, err := os.Executable()
	if err != nil {
		return err
	}

	// Windows 不能覆盖正在运行的 exe，先重命名当前文件再替换
	if runtime.GOOS == "windows" {
		backup := target + ".old"
		os.Remove(backup)
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("rename current exe: %w", err)
		}
		if err := os.Rename(tmp, target); err != nil {
			// 回滚
			os.Rename(backup, target)
			return fmt.Errorf("install new exe: %w", err)
		}
	} else {
		if err := os.Rename(tmp, target); err != nil {
			return fmt.Errorf("install new exe: %w", err)
		}
	}

	logger.Info("  ✓ updated to %s", tag)
	return nil
}

func fetchLatestTag(repo string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://gitee.com/api/v3/repos/%s/releases/latest", repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pvm")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Gitee API returned %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.TagName == "" {
		return "", fmt.Errorf("no tag in response")
	}
	return out.TagName, nil
}
