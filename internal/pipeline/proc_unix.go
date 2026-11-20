//go:build !windows

package pipeline

import (
	"os/exec"
	"syscall"
)

// isolate 让子进程自成进程组，便于整组终止。
// 只用于并行分支（它们不读终端输入）；顺序步骤保持前台进程组以支持交互。
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree 终止进程；已 isolate 时整组终止。
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
}
