package cmd

import (
	"fmt"
	"os"

	"github.com/pvm/pvm/internal/config"
)

// runCurrent 显示当前目录下生效的版本及来源
//
//	pvm current           # 显示所有 runtime 当前生效版本
//	pvm current node      # 只显示某个 runtime
func runCurrent(args []string) error {
	cwd, _ := os.Getwd()

	var filterRT string
	if len(args) > 0 {
		filterRT = args[0]
	}

	fmt.Printf("Active versions (cwd: %s):\n", cwd)

	for _, rt := range config.SupportedRuntimes {
		if filterRT != "" && filterRT != rt {
			continue
		}

		ver, src := config.ResolveVersion(rt, cwd)

		if ver == "" {
			if config.IsGlobalOnly(rt) {
				fmt.Printf("  %-7s (not set — use: pvm install %s@<version>)\n", rt, rt)
			} else {
				fmt.Printf("  %-7s (not set — use: pvm use %s@<version>)\n", rt, rt)
			}
			continue
		}

		// system 版本：显示系统全局安装的真实路径
		if ver == config.SystemVersion {
			sysPath, err := config.SystemCommandPath(config.RuntimeMainCommand(rt))
			label := srcLabel(src)
			if err != nil {
				fmt.Printf("  %-7s system   [%s]  ⚠ not found in system PATH\n", rt, label)
			} else {
				fmt.Printf("  %-7s system   [%s]  → %s\n", rt, label, sysPath)
			}
			continue
		}

		installed := ""
		if !config.IsInstalled(rt, ver) {
			installed = "  ⚠ not installed — run: pvm install " + rt + "@" + ver
		}
		fmt.Printf("  %-7s %-12s [%s]%s\n", rt, ver, srcLabel(src), installed)
	}

	return nil
}
