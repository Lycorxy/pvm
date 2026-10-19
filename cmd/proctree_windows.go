//go:build windows

package cmd

import (
	"os/exec"
	"strconv"
)

// killProcessTree 终止进程及其子进程树（taskkill /T=树 /F=强制）
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
