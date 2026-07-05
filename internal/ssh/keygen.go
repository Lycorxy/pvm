package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// KeyType SSH 密钥类型
type KeyType string

const (
	KeyTypeRSA     KeyType = "rsa"
	KeyTypeEd25519 KeyType = "ed25519"
)

// KeyInfo SSH 密钥信息
type KeyInfo struct {
	PrivateKeyPath string
	PublicKeyPath  string
	PublicKey      string
	KeyType        KeyType
	Comment        string
}

// DefaultSSHDir 默认 SSH 目录
func DefaultSSHDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".ssh")
}

// GenerateSSHKey 使用 ssh-keygen 命令生成 SSH 密钥
func GenerateSSHKey(keyType KeyType, comment string) (*KeyInfo, error) {
	sshDir := DefaultSSHDir()
	if sshDir == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}

	// 确保 .ssh 目录存在
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	var privateKeyPath, publicKeyPath string
	var keyTypeArg string

	switch keyType {
	case KeyTypeRSA:
		privateKeyPath = filepath.Join(sshDir, "id_rsa")
		publicKeyPath = filepath.Join(sshDir, "id_rsa.pub")
		keyTypeArg = "rsa"
	case KeyTypeEd25519:
		privateKeyPath = filepath.Join(sshDir, "id_ed25519")
		publicKeyPath = filepath.Join(sshDir, "id_ed25519.pub")
		keyTypeArg = "ed25519"
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	// 使用 ssh-keygen 命令生成密钥
	args := []string{
		"-t", keyTypeArg,
		"-f", privateKeyPath,
		"-N", "", // 无密码
		"-C", comment,
	}

	cmd := exec.Command("ssh-keygen", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key: %w\nOutput: %s", err, output)
	}

	// 读取生成的公钥
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKeyStr := strings.TrimSpace(string(publicKeyBytes))

	return &KeyInfo{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		PublicKey:      publicKeyStr,
		KeyType:        keyType,
		Comment:        comment,
	}, nil
}

// GetExistingSSHKey 获取现有的 SSH 密钥
func GetExistingSSHKey(keyType KeyType) (*KeyInfo, error) {
	sshDir := DefaultSSHDir()
	if sshDir == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}

	var privateKeyPath, publicKeyPath string
	switch keyType {
	case KeyTypeRSA:
		privateKeyPath = filepath.Join(sshDir, "id_rsa")
		publicKeyPath = filepath.Join(sshDir, "id_rsa.pub")
	case KeyTypeEd25519:
		privateKeyPath = filepath.Join(sshDir, "id_ed25519")
		publicKeyPath = filepath.Join(sshDir, "id_ed25519.pub")
	default:
		// 默认检查 RSA
		privateKeyPath = filepath.Join(sshDir, "id_rsa")
		publicKeyPath = filepath.Join(sshDir, "id_rsa.pub")
	}

	// 检查私钥是否存在
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return nil, nil // 密钥不存在
	}

	// 读取公钥
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKeyStr := strings.TrimSpace(string(publicKeyBytes))

	// 解析公钥获取 comment
	parts := strings.Split(publicKeyStr, " ")
	comment := ""
	if len(parts) >= 3 {
		comment = parts[2]
	}

	return &KeyInfo{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		PublicKey:      publicKeyStr,
		KeyType:        keyType,
		Comment:        comment,
	}, nil
}

// GetOrGenerateSSHKey 获取现有密钥或生成新密钥
func GetOrGenerateSSHKey(keyType KeyType, comment string) (*KeyInfo, bool, error) {
	// 先检查现有密钥
	existingKey, err := GetExistingSSHKey(keyType)
	if err != nil {
		return nil, false, err
	}

	if existingKey != nil {
		return existingKey, false, nil // 使用现有密钥
	}

	// 生成新密钥
	newKey, err := GenerateSSHKey(keyType, comment)
	if err != nil {
		return nil, false, err
	}

	return newKey, true, nil // 新生成的密钥
}

// CheckSSHKeygen 检查 ssh-keygen 命令是否可用
func CheckSSHKeygen() bool {
	_, err := exec.LookPath("ssh-keygen")
	return err == nil
}
