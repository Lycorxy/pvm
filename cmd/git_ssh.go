package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/ssh"
)

// runGitSSH 处理 git ssh 子命令
func runGitSSH(args []string) error {
	if len(args) == 0 {
		printGitSSHUsage()
		return nil
	}

	// 解析参数
	var platform ssh.Platform
	var token string
	var customURL string
	var keyType ssh.KeyType = ssh.KeyTypeRSA
	var title string
	var testOnly bool
	var showKey bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--platform", "-p":
			if i+1 >= len(args) {
				return fmt.Errorf("--platform requires a value")
			}
			p := strings.ToLower(args[i+1])
			switch p {
			case "github", "gh":
				platform = ssh.PlatformGitHub
			case "gitlab", "gl":
				platform = ssh.PlatformGitLab
			case "gitee":
				platform = ssh.PlatformGitee
			case "custom":
				platform = ssh.PlatformCustom
			default:
				return fmt.Errorf("unsupported platform: %s", p)
			}
			i++
		case "--token", "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("--token requires a value")
			}
			token = args[i+1]
			i++
		case "--url", "-u":
			if i+1 >= len(args) {
				return fmt.Errorf("--url requires a value")
			}
			customURL = args[i+1]
			i++
		case "--key-type", "-k":
			if i+1 >= len(args) {
				return fmt.Errorf("--key-type requires a value")
			}
			kt := strings.ToLower(args[i+1])
			switch kt {
			case "rsa":
				keyType = ssh.KeyTypeRSA
			case "ed25519":
				keyType = ssh.KeyTypeEd25519
			default:
				return fmt.Errorf("unsupported key type: %s", kt)
			}
			i++
		case "--title":
			if i+1 >= len(args) {
				return fmt.Errorf("--title requires a value")
			}
			title = args[i+1]
			i++
		case "--test":
			testOnly = true
		case "--show-key":
			showKey = true
		case "--help", "-h":
			printGitSSHUsage()
			return nil
		default:
			if strings.HasPrefix(arg, "--") {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			// 如果第一个参数是平台名（不带 --platform）
			if platform == "" && i == 0 {
				p := strings.ToLower(arg)
				switch p {
				case "github", "gh":
					platform = ssh.PlatformGitHub
				case "gitlab", "gl":
					platform = ssh.PlatformGitLab
				case "gitee":
					platform = ssh.PlatformGitee
				case "tencent":
					platform = ssh.PlatformGitLab
					customURL = "https://git.code.tencent.com"
				case "custom":
					platform = ssh.PlatformCustom
				}
			}
		}
	}

	// 验证必要参数
	if testOnly {
		return testSSHConnection(platform, customURL)
	}

	if showKey {
		return showSSHKey(keyType)
	}

	if platform == "" {
		return fmt.Errorf("platform is required, use --platform or specify github/gitlab/gitee")
	}

	if token == "" {
		return fmt.Errorf("token is required, use --token")
	}

	// 生成默认标题
	if title == "" {
		title = fmt.Sprintf("pvm-%s-%s", platform, runtime.GOOS)
	}

	// 执行配置流程
	return configureSSH(platform, token, customURL, keyType, title)
}

// configureSSH 配置 SSH
func configureSSH(platform ssh.Platform, token, customURL string, keyType ssh.KeyType, title string) error {
	logger.Info("Configuring SSH for %s...", platform)

	// 1. 获取或生成 SSH 密钥
	logger.Verbose("Checking SSH key...")
	keyInfo, generated, err := ssh.GetOrGenerateSSHKey(keyType, title)
	if err != nil {
		return fmt.Errorf("failed to get/generate SSH key: %w", err)
	}

	if generated {
		logger.Info("  ✓ Generated new SSH key: %s", keyInfo.PrivateKeyPath)
	} else {
		logger.Info("  ✓ Using existing SSH key: %s", keyInfo.PrivateKeyPath)
	}

	logger.Info("  Public key: %s", truncateKey(keyInfo.PublicKey, 50))

	// 2. 上传公钥到平台
	logger.Verbose("Uploading public key to %s...", platform)
	result, err := ssh.UploadSSHKey(&ssh.UploadSSHKeyRequest{
		Platform:  platform,
		Token:     token,
		PublicKey: keyInfo.PublicKey,
		Title:     title,
		CustomURL: customURL,
	})
	if err != nil {
		return fmt.Errorf("failed to upload SSH key: %w", err)
	}

	logger.Info("  ✓ %s", result.Message)
	logger.Verbose("  Key ID: %d, Title: %s", result.KeyID, result.Title)

	// 3. 配置 SSH config
	logger.Verbose("Configuring SSH config...")
	configManager := ssh.NewSSHConfigManager()
	hostConfig := ssh.GenerateDefaultHostConfig(platform, customURL)
	if err := configManager.AddHostConfig(hostConfig); err != nil {
		return fmt.Errorf("failed to update SSH config: %w", err)
	}

	logger.Info("  ✓ SSH config updated for %s", hostConfig.Host)

	// 4. 测试连接
	logger.Info("")
	logger.Info("Testing SSH connection...")
	if err := testSSHConnection(platform, customURL); err != nil {
		logger.Warn("  ⚠ Connection test failed: %s", err)
		logger.Info("  This may be normal if the key needs time to propagate.")
	} else {
		logger.Info("  ✓ SSH connection successful!")
	}

	// 5. 打印使用说明
	logger.Info("")
	logger.Info("========================================")
	logger.Info("  SSH configuration complete!")
	logger.Info("========================================")
	logger.Info("")
	logger.Info("You can now use Git commands:")
	logger.Info("")
	switch platform {
	case ssh.PlatformGitHub:
		logger.Info("  git clone git@github.com:user/repo.git")
	case ssh.PlatformGitLab:
		if customURL != "" {
			host := ssh.ExtractHostFromURL(customURL)
			logger.Info("  git clone git@%s:user/repo.git", host)
		} else {
			logger.Info("  git clone git@gitlab.com:user/repo.git")
		}
	case ssh.PlatformGitee:
		logger.Info("  git clone git@gitee.com:user/repo.git")
	case ssh.PlatformCustom:
		host := ssh.ExtractHostFromURL(customURL)
		logger.Info("  git clone git@%s:user/repo.git", host)
	}
	logger.Info("")

	return nil
}

// testSSHConnection 测试 SSH 连接
func testSSHConnection(platform ssh.Platform, customURL string) error {
	var host string
	switch platform {
	case ssh.PlatformGitHub:
		host = "github.com"
	case ssh.PlatformGitLab:
		if customURL != "" {
			host = ssh.ExtractHostFromURL(customURL)
		} else {
			host = "gitlab.com"
		}
	case ssh.PlatformGitee:
		host = "gitee.com"
	case ssh.PlatformCustom:
		host = ssh.ExtractHostFromURL(customURL)
	default:
		return fmt.Errorf("unsupported platform")
	}

	// 使用 ssh 命令测试连接
	cmd := exec.Command("ssh", "-T", "git@"+host, "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10")
	output, err := cmd.CombinedOutput()

	// GitHub/GitLab/Gitee 正常情况下会返回 "successfully authenticated" 但 exit code 为 1
	outputStr := string(output)
	if strings.Contains(outputStr, "successfully authenticated") ||
		strings.Contains(outputStr, "authenticated") ||
		strings.Contains(outputStr, "logged in") {
		return nil // 连接成功
	}

	if err != nil {
		return fmt.Errorf("SSH test failed: %s", outputStr)
	}

	return nil
}

// showSSHKey 显示当前 SSH 公钥
func showSSHKey(keyType ssh.KeyType) error {
	keyInfo, err := ssh.GetExistingSSHKey(keyType)
	if err != nil {
		return err
	}

	if keyInfo == nil {
		logger.Info("No SSH key found. Use 'pvm git ssh --platform <name> --token <token>' to generate one.")
		return nil
	}

	logger.Info("SSH Key Information:")
	logger.Info("  Type:       %s", keyInfo.KeyType)
	logger.Info("  Private:    %s", keyInfo.PrivateKeyPath)
	logger.Info("  Public:     %s", keyInfo.PublicKeyPath)
	logger.Info("")
	logger.Info("Public Key:")
	logger.Info("  %s", keyInfo.PublicKey)
	logger.Info("")
	logger.Info("Copy this public key to your Git platform if needed.")

	return nil
}

// truncateKey 截断密钥显示
func truncateKey(key string, maxLen int) string {
	if len(key) <= maxLen {
		return key
	}
	return key[:maxLen] + "..."
}

// printGitSSHUsage 打印使用说明
func printGitSSHUsage() {
	usage := `pvm git ssh - Configure SSH for Git platforms

Usage:
  pvm git ssh <platform> --token <token> [options]
  pvm git ssh --platform <name> --token <token> [options]

Platforms:
  github, gh          GitHub (github.com)
  gitlab, gl          GitLab (gitlab.com)
  gitee               Gitee/码云 (gitee.com)
  tencent             腾讯工蜂 (git.code.tencent.com)
  custom              Custom GitLab server (requires --url)

Options:
  --platform, -p <name>    Platform name (github/gitlab/gitee/custom)
  --token, -t <token>      API token for the platform
  --url, -u <url>          Custom server URL (for gitlab/custom)
  --key-type, -k <type>    Key type: rsa (default) or ed25519
  --title <title>          Key title (default: pvm-<platform>-<os>)
  --test                   Test SSH connection only
  --show-key               Show existing SSH public key
  --help, -h               Show this help

Token Types:
  GitHub:  Personal Access Token (classic or fine-grained) with 'write:public_key' scope
           https://github.com/settings/tokens
  GitLab:  Personal Access Token with 'write_ssh_key' scope
           https://gitlab.com/-/profile/personal_access_tokens
  Gitee:   Personal Access Token
           https://gitee.com/profile/personal_access_token_tokens

Examples:
  # Configure GitHub SSH
  pvm git ssh github --token ghp_xxxxxxxxxxxx

  # Configure GitLab SSH
  pvm git ssh gitlab --token glpat-xxxxxxxxxxxx

  # Configure 腾讯工蜂
  pvm git ssh tencent --token xxxxxxxxxxxx

  # Configure custom GitLab server
  pvm git ssh custom --token xxx --url https://git.company.com

  # Use Ed25519 key type
  pvm git ssh github --token ghp_xxx --key-type ed25519

  # Test SSH connection
  pvm git ssh --test --platform github

  # Show existing SSH key
  pvm git ssh --show-key

What it does:
  1. Generate SSH key pair (if not exists)
  2. Upload public key to the platform via API
  3. Configure ~/.ssh/config for the platform
  4. Test SSH connection
`
	fmt.Println(usage)
}
