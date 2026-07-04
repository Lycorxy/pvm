package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/semver"
)

// runList 列出已安装的 runtime 版本
//
//	pvm list              # 全部 runtime 已安装版本
//	pvm list node         # 某个 runtime
//	pvm list --user       # 只显示用户全局版本配置
//	pvm list --local      # 只显示当前项目配置文件内容
func runList(args []string) error {
	args, userOnly := hasFlag(args, "--user", "-u")
	args, localOnly := hasFlag(args, "--local", "-l")

	var filterRT string
	if len(args) > 0 {
		filterRT = args[0]
	}

	// --user：只显示用户全局版本配置（~/.pvm/versions）
	if userOnly {
		return runListGlobal(filterRT)
	}

	// --local：只显示当前项目配置文件内容
	if localOnly {
		return runListProject(filterRT)
	}

	cwd, _ := os.Getwd()

	// 解析当前生效版本及来源
	activeVer := make(map[string]string)
	activeSrc := make(map[string]string)
	for _, rt := range config.SupportedRuntimes {
		v, src := config.ResolveVersion(rt, cwd)
		activeVer[rt] = v
		activeSrc[rt] = src
	}

	printed := false
	for _, rt := range config.SupportedRuntimes {
		if filterRT != "" && filterRT != rt {
			continue
		}

		rtDir := filepath.Join(config.InstallsDir(), rt)
		entries, err := os.ReadDir(rtDir)
		if err != nil {
			entries = nil
		}

		versions := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() {
				versions = append(versions, e.Name())
			}
		}

		active := activeVer[rt]
		src := activeSrc[rt]

		// 如果没有任何已安装版本，但当前是 system，也要显示
		if len(versions) == 0 && active != config.SystemVersion {
			continue
		}

		semver.SortStringsDesc(versions)

		if printed {
			fmt.Println()
		}
		printed = true

		fmt.Printf("%s:\n", rt)

		// 显示 system 条目（如果当前生效的是 system）
		if active == config.SystemVersion {
			sysPath, err := config.SystemCommandPath(config.RuntimeMainCommand(rt))
			sysInfo := ""
			if err == nil {
				sysInfo = "  (" + sysPath + ")"
			}
			fmt.Printf("  * system   [active via %s]%s\n", srcLabel(src), sysInfo)
		}

		for _, v := range versions {
			marker := "  "
			srcTag := ""
			if v == active {
				marker = "* "
				srcTag = fmt.Sprintf("   [active via %s]", srcLabel(src))
			}
			fmt.Printf("  %s%s%s\n", marker, v, srcTag)
		}
	}

	if !printed {
		fmt.Println("  No versions installed. Run: pvm install <runtime>@<version>")
	}

	return nil
}

// runListGlobal 显示用户全局版本配置（~/.pvm/versions）
func runListGlobal(filterRT string) error {
	globalFile := config.GlobalVersionsFile()
	vf, err := config.LoadVersionFile(globalFile)

	fmt.Printf("User versions (%s):\n", globalFile)

	if err != nil || vf == nil || len(vf.Versions) == 0 {
		fmt.Println("  (none configured — use: pvm use <runtime>@<version> --user)")
		return nil
	}

	for _, rt := range config.SupportedRuntimes {
		if filterRT != "" && filterRT != rt {
			continue
		}
		v, ok := vf.Versions[rt]
		if !ok || v == "" {
			continue
		}
		installed := ""
		if v != config.SystemVersion && !config.IsInstalled(rt, v) {
			installed = "  [NOT INSTALLED]"
		}
		if v == config.SystemVersion {
			sysPath, serr := config.SystemCommandPath(config.RuntimeMainCommand(rt))
			if serr == nil {
				fmt.Printf("  %-7s system   (%s)\n", rt, sysPath)
			} else {
				fmt.Printf("  %-7s system   (not found in system PATH)\n", rt)
			}
		} else {
			fmt.Printf("  %-7s %s%s\n", rt, v, installed)
		}
	}
	return nil
}

// runListProject 显示当前目录的项目版本配置（.pvmrc，即 local）
func runListProject(filterRT string) error {
	cwd, _ := os.Getwd()
	vf, err := config.FindVersionFile(cwd)

	if err != nil {
		return fmt.Errorf("read project config: %w", err)
	}
	if vf == nil || len(vf.Versions) == 0 {
		fmt.Printf("Project versions (cwd: %s):\n", cwd)
		fmt.Println("  (none configured — use: pvm use <runtime>@<version>)")
		return nil
	}

	fmt.Printf("Project versions (%s):\n", vf.Path)
	for _, rt := range config.SupportedRuntimes {
		if filterRT != "" && filterRT != rt {
			continue
		}
		v, ok := vf.Versions[rt]
		if !ok || v == "" {
			continue
		}
		installed := ""
		if v != config.SystemVersion && !config.IsInstalled(rt, v) {
			installed = "  [NOT INSTALLED — run: pvm install " + rt + "@" + v + "]"
		}
		if v == config.SystemVersion {
			fmt.Printf("  %-7s system%s\n", rt, installed)
		} else {
			fmt.Printf("  %-7s %s%s\n", rt, v, installed)
		}
	}
	return nil
}

// srcLabel 将来源路径转换为简短标签
func srcLabel(src string) string {
	switch {
	case src == "system":
		return "system"
	case strings.EqualFold(src, config.GlobalVersionsFile()):
		// Windows 路径大小写不敏感，用 EqualFold 比较
		return "user"
	case len(src) > 4 && src[:4] == "env:":
		return src
	case src != "":
		return "local"
	default:
		return "unknown"
	}
}
