package pack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runStream 执行命令并透传标准输入输出，失败时附带上下文
func runStream(context string, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// runCapture 执行命令并捕获输出：成功时静默，失败时把输出带进错误里。
// 打包默认走这条路径，避免把 esbuild/pyinstaller 的日志刷满屏。
func runCapture(context string, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out != "" {
			return fmt.Errorf("%s: %w\n%s", context, err, out)
		}
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// runStep 按 --verbose 选择透传还是静默
func runStep(verbose bool, context, name string, arg ...string) error {
	if verbose {
		return runStream(context, name, arg...)
	}
	return runCapture(context, name, arg...)
}

// captureOutput 执行命令并捕获 stdout
func captureOutput(name string, arg ...string) (string, error) {
	out, err := exec.Command(name, arg...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveCommand 查找命令；找不到时给出带上下文的错误
func resolveCommand(context, name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s: %s not found in PATH", context, name)
	}
	return p, nil
}

// copyExecutable 复制二进制文件并保留可执行权限
func copyExecutable(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func formatSize(b int64) string {
	const mb = 1024 * 1024
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
