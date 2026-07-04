package filelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileLock provides cross-platform file-based locking
type FileLock struct {
	path string
	file *os.File
}

// New creates a new file lock at the given path
func New(path string) *FileLock {
	return &FileLock{
		path: path + ".lock",
	}
}

// staleLockTimeout 是锁文件被视为僵尸锁的超时时间。
// 与 Lock() 的 timeout 参数保持一致（30s），确保进程崩溃后锁能被及时清理。
const staleLockTimeout = 30 * time.Second

// Lock acquires the file lock with a timeout.
// 锁文件内容格式："<pid>\n<unix-nano>"，用于判断僵尸锁。
func (fl *FileLock) Lock(timeout time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}

	deadline := time.Now().Add(timeout)
	retryInterval := 50 * time.Millisecond

	for {
		f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// 写入 PID + 创建时间戳，方便调试和僵尸锁检测
			fmt.Fprintf(f, "%d\n%d", os.Getpid(), time.Now().UnixNano())
			fl.file = f
			return nil
		}

		if !os.IsExist(err) {
			return fmt.Errorf("create lock file: %w", err)
		}

		// 锁文件已存在：读取时间戳判断是否为僵尸锁
		if isStaleLock(fl.path, staleLockTimeout) {
			os.Remove(fl.path)
			continue
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for lock: %s", fl.path)
		}

		time.Sleep(retryInterval)
		if retryInterval < 500*time.Millisecond {
			retryInterval *= 2
		}
	}
}

// isStaleLock 判断锁文件是否为僵尸锁。
// 优先读取文件内的时间戳；若格式不兼容（旧版本只写了 PID），则回退到 ModTime。
func isStaleLock(path string, maxAge time.Duration) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不可读，保守处理：不清理
		return false
	}

	lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(lines) == 2 {
		var nano int64
		if _, scanErr := fmt.Sscanf(lines[1], "%d", &nano); scanErr == nil {
			createdAt := time.Unix(0, nano)
			return time.Since(createdAt) > maxAge
		}
	}

	// 回退：用文件 ModTime（兼容旧格式）
	if info, statErr := os.Stat(path); statErr == nil {
		return time.Since(info.ModTime()) > maxAge
	}
	return false
}

// Unlock releases the file lock
func (fl *FileLock) Unlock() error {
	if fl.file != nil {
		fl.file.Close()
		fl.file = nil
	}
	return os.Remove(fl.path)
}
