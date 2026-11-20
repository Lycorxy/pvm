//go:build windows

package zipx

import (
	"os"
	"syscall"
	"time"
)

// setModTime 在已打开的句柄上设置修改时间。
// 不再按路径重新打开文件：Windows 上每次打开/关闭都会触发一次安全软件扫描，成本约等于再创建一次文件。
func setModTime(f *os.File, _ string, t time.Time) error {
	ft := syscall.NsecToFiletime(t.UnixNano())
	return syscall.SetFileTime(syscall.Handle(f.Fd()), nil, &ft, &ft)
}
