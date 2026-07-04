package cmd

import (
	"github.com/pvm/pvm/internal/shim"
)

// runShimExec 是 shim 脚本的执行入口
// 例如 ~/.pvm/shims/node 最终会调用 `pvm shim-exec node <args>`
func runShimExec(cmdName string, args []string) error {
	return shim.Exec(cmdName, args)
}
