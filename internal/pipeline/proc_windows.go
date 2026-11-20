//go:build windows

package pipeline

import (
	"os/exec"
	"strconv"
)

// isolate 在 Windows 上无需额外设置（taskkill /T 按进程树终止）。
func isolate(cmd *exec.Cmd) {}

// killTree 终止进程及其子进程树（taskkill /T=树 /F=强制）。
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
