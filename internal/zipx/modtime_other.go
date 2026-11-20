//go:build !windows

package zipx

import (
	"os"
	"time"
)

// setModTime 设置修改时间；类 Unix 上 utimensat 按路径设置不需要再打开文件，开销很小。
func setModTime(_ *os.File, path string, t time.Time) error {
	return os.Chtimes(path, t, t)
}
