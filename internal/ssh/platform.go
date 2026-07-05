package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Platform Git 平台类型
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab"
	PlatformGitee  Platform = "gitee"
	PlatformCustom Platform = "custom"
)

// PlatformConfig 平台配置
type PlatformConfig struct {
	Name        Platform
	APIURL      string
	SSHHost     string
	TokenHeader string
	TokenPrefix string
}

// DefaultPlatformConfigs 默认平台配置
var DefaultPlatformConfigs = map[Platform]*PlatformConfig{
	PlatformGitHub: {
		Name:        PlatformGitHub,
		APIURL:      "https://api.github.com",
		SSHHost:     "github.com",
		TokenHeader: "Authorization",
		TokenPrefix: "token ",
	},
	PlatformGitLab: {
		Name:        PlatformGitLab,
		APIURL:      "https://gitlab.com/api/v4",
		SSHHost:     "gitlab.com",
		TokenHeader: "PRIVATE-TOKEN",
		TokenPrefix: "",
	},
	PlatformGitee: {
		Name:        PlatformGitee,
		APIURL:      "https://gitee.com/api/v5",
		SSHHost:     "gitee.com",
		TokenHeader: "",
		TokenPrefix: "",
	},
}

// 工蜂（腾讯 GitLab）配置
var TencentGitLabConfig = &PlatformConfig{
	Name:        PlatformGitLab,
	APIURL:      "https://git.code.tencent.com/api/v4",
	SSHHost:     "git.code.tencent.com",
	TokenHeader: "PRIVATE-TOKEN",
	TokenPrefix: "",
}

// UploadSSHKeyRequest 上传 SSH 密钥请求
type UploadSSHKeyRequest struct {
	Platform  Platform
	Token     string
	PublicKey string
	Title     string
	CustomURL string // 自定义平台 URL
}

// UploadSSHKeyResponse 上传结果
type UploadSSHKeyResponse struct {
	Success  bool
	KeyID    int64
	Title    string
	Key      string
	Message  string
	Platform Platform
}

// UploadSSHKey 上传 SSH 公钥到指定平台
func UploadSSHKey(req *UploadSSHKeyRequest) (*UploadSSHKeyResponse, error) {
	var config *PlatformConfig

	// 获取平台配置
	if req.Platform == PlatformCustom && req.CustomURL != "" {
		config = &PlatformConfig{
			Name:        PlatformCustom,
			APIURL:      req.CustomURL,
			SSHHost:     ExtractHostFromURL(req.CustomURL),
			TokenHeader: "PRIVATE-TOKEN", // GitLab 兼容
			TokenPrefix: "",
		}
	} else if req.Platform == PlatformGitLab && strings.Contains(req.CustomURL, "tencent.com") {
		config = TencentGitLabConfig
	} else {
		config = DefaultPlatformConfigs[req.Platform]
	}

	if config == nil {
		return nil, fmt.Errorf("unsupported platform: %s", req.Platform)
	}

	switch req.Platform {
	case PlatformGitHub:
		return uploadToGitHub(config, req)
	case PlatformGitLab:
		return uploadToGitLab(config, req)
	case PlatformGitee:
		return uploadToGitee(config, req)
	case PlatformCustom:
		return uploadToGitLab(config, req) // 自定义平台使用 GitLab API 格式
	default:
		return nil, fmt.Errorf("unsupported platform: %s", req.Platform)
	}
}

// uploadToGitHub 上传到 GitHub
func uploadToGitHub(config *PlatformConfig, req *UploadSSHKeyRequest) (*UploadSSHKeyResponse, error) {
	url := config.APIURL + "/user/keys"

	payload := map[string]string{
		"title": req.Title,
		"key":   req.PublicKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(config.TokenHeader, config.TokenPrefix+req.Token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 201 {
		// 解析错误信息
		var errResp struct {
			Message string `json:"message"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if json.Unmarshal(respBody, &errResp) == nil {
			if len(errResp.Errors) > 0 {
				return nil, fmt.Errorf("GitHub API error: %s", errResp.Errors[0].Message)
			}
			return nil, fmt.Errorf("GitHub API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("GitHub API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &UploadSSHKeyResponse{
		Success:  true,
		KeyID:    result.ID,
		Title:    result.Title,
		Key:      result.Key,
		Platform: PlatformGitHub,
		Message:  "SSH key uploaded successfully to GitHub",
	}, nil
}

// uploadToGitLab 上传到 GitLab（包括工蜂）
func uploadToGitLab(config *PlatformConfig, req *UploadSSHKeyRequest) (*UploadSSHKeyResponse, error) {
	url := config.APIURL + "/user/keys"

	payload := map[string]string{
		"title": req.Title,
		"key":   req.PublicKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(config.TokenHeader, req.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 201 {
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("GitLab API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("GitLab API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &UploadSSHKeyResponse{
		Success:  true,
		KeyID:    result.ID,
		Title:    result.Title,
		Key:      result.Key,
		Platform: PlatformGitLab,
		Message:  fmt.Sprintf("SSH key uploaded successfully to %s", config.SSHHost),
	}, nil
}

// uploadToGitee 上传到 Gitee（码云）
func uploadToGitee(config *PlatformConfig, req *UploadSSHKeyRequest) (*UploadSSHKeyResponse, error) {
	url := config.APIURL + "/user/keys"

	// Gitee 使用不同的参数格式
	payload := map[string]string{
		"access_token": req.Token,
		"title":        req.Title,
		"key":          req.PublicKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("Gitee API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("Gitee API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &UploadSSHKeyResponse{
		Success:  true,
		KeyID:    result.ID,
		Title:    result.Title,
		Key:      result.Key,
		Platform: PlatformGitee,
		Message:  "SSH key uploaded successfully to Gitee",
	}, nil
}

// ExtractHostFromURL 从 URL 中提取主机名（导出供 cmd 使用）
func ExtractHostFromURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/api/v4")
	url = strings.TrimSuffix(url, "/api")
	return strings.Split(url, "/")[0]
}

// TestSSHConnection 测试 SSH 连接
func TestSSHConnection(host string) error {
	// 这里简化实现，实际应使用 SSH 客户端测试连接
	// 可以调用 ssh 命令进行测试
	return nil
}
