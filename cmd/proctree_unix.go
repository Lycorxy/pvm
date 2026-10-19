//go:build !windows

package cmd

import "syscall"

// killProcessTree 终止进程（SIGKILL）。
// 子进程默认与 pvm 同进程组，终端 Ctrl+C 已同组送达；此处为程序化终止的兜底。
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
