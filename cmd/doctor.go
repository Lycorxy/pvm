package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/config"
)

// doctorResult 存储单项检查结果
type doctorResult struct {
	name string
	ok   bool
	msg  string
}

// runDoctor 检查 pvm 安装健康度
func runDoctor(args []string) error {
	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"PVM_HOME directory", checkPvmHome},
		{"shims directory exists", checkShimsDir},
		{"shims in PATH", checkShimsInPath},
		{"pvm binary in expected location", checkPvmBinary},
		{"writable PVM_HOME", checkWritable},
		{"no legacy .ps1 shims", checkLegacyPs1Shims},
		{"no conflicting version managers", checkConflictingManagers},
	}

	allOk := true
	fmt.Printf("pvm doctor (version %s, %s/%s)\n\n", Version, runtime.GOOS, runtime.GOARCH)

	var results []doctorResult
	for _, c := range checks {
		ok, msg := c.fn()
		if !ok {
			allOk = false
		}
		results = append(results, doctorResult{c.name, ok, msg})
	}

	for _, r := range results {
		mark := "✓"
		if !r.ok {
			mark = "✗"
		}
		fmt.Printf("  %s %-36s %s\n", mark, r.name, r.msg)
	}

	fmt.Println()
	if !allOk {
		// 打印详细修复指引
		printDoctorFixGuide(results)
		return fmt.Errorf("doctor: not all checks passed")
	}
	fmt.Println("Everything looks good!")
	return nil
}

// printDoctorFixGuide 根据失败的检查项打印详细的修复指引
func printDoctorFixGuide(results []doctorResult) {
	fmt.Println("  ─────────────────────────────────────────────")
	fmt.Println("  Fix guide:")
	fmt.Println()
	for _, r := range results {
		if r.ok {
			continue
		}
		switch r.name {
		case "PVM_HOME directory":
			fmt.Println("  [PVM_HOME missing]")
			fmt.Println("    Run: pvm setup")
		case "shims directory exists":
			fmt.Println("  [shims directory missing]")
			fmt.Println("    Run: pvm reshim")
		case "shims in PATH":
			fmt.Println("  [shims not in PATH or not highest priority]")
			fmt.Println("    Run: pvm setup")
			if runtime.GOOS == "windows" {
				fmt.Println("    If nvm/volta/fnm is installed, run as Administrator: pvm setup")
			}
		case "pvm binary in expected location":
			fmt.Println("  [pvm binary not in expected location]")
			fmt.Println("    Run: pvm setup")
		case "writable PVM_HOME":
			fmt.Println("  [PVM_HOME not writable]")
			fmt.Printf("    Check permissions on: %s\n", config.PvmHome())
		case "no legacy .ps1 shims":
			fmt.Println("  [Legacy .ps1 shims found — they break under PowerShell ExecutionPolicy]")
			fmt.Println("    Run: pvm reshim")
			fmt.Println("    (will regenerate shims using .exe instead of .ps1)")
		case "no conflicting version managers":
			fmt.Println("  [Conflicting version manager(s) detected]")
			fmt.Println("  pvm must have the highest PATH priority to work correctly.")
			fmt.Println()
			fmt.Println("  Option A — Uninstall the conflicting tool(s) [Recommended]:")
			printUninstallGuides()
			fmt.Println()
			if runtime.GOOS == "windows" {
				fmt.Println("  Option B — Auto-fix PATH (requires Administrator):")
				fmt.Println("    Right-click terminal → Run as administrator → pvm setup")
				fmt.Println()
				fmt.Println("  Option C — Manual fix:")
				fmt.Println("    1. System → Advanced system settings → Environment Variables")
				fmt.Println("    2. System variables → Path → Edit")
				fmt.Println("    3. Remove nodejs / nvm / volta entries")
				fmt.Println("    4. Reopen terminal")
			}
		}
		fmt.Println()
	}
}

// printUninstallGuides 打印已检测到的版本管理工具的卸载指引
func printUninstallGuides() {
	type vmTool struct {
		exe   string
		guide string
	}
	tools := []vmTool{
		{"nvm", "nvm: uninstall all versions first, then run the nvm uninstaller"},
		{"volta", "volta: volta uninstall node  (or uninstall Volta via its installer)"},
		{"fnm", "fnm: remove fnm from PATH and delete its data directory"},
		{"nodenv", "nodenv: nodenv uninstall <version>  (or remove nodenv)"},
	}
	for _, t := range tools {
		if _, err := exec.LookPath(t.exe); err == nil {
			fmt.Printf("    • %s\n", t.guide)
		}
	}
}

func checkPvmHome() (bool, string) {
	home := config.PvmHome()
	if _, err := os.Stat(home); err != nil {
		return false, home + " (missing)"
	}
	return true, home
}

func checkShimsDir() (bool, string) {
	d := config.ShimsDir()
	if _, err := os.Stat(d); err != nil {
		return false, d + " (missing — run `pvm reshim`)"
	}
	return true, d
}

func checkShimsInPath() (bool, string) {
	shims := config.ShimsDir()
	path := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	want := filepath.Clean(shims)

	shimsIndex := -1
	conflictIndex := -1
	conflictName := ""

	// 已知会与 pvm shims 冲突的系统 runtime 目录名
	conflictBases := []string{"nodejs", "node", "python", "go", "golang", "ruby"}

	for i, p := range strings.Split(path, sep) {
		cleaned := filepath.Clean(p)
		if cleaned == want {
			shimsIndex = i
		}
		// 检查冲突目录
		if conflictIndex == -1 {
			base := filepath.Base(cleaned)
			for _, c := range conflictBases {
				if strings.EqualFold(base, c) && cleaned != want {
					conflictIndex = i
					conflictName = cleaned
				}
				// 前缀匹配：处理 Python 版本号目录 (如 Python312, Python39)
				if conflictIndex == -1 && len(base) > len(c) && strings.EqualFold(base[:len(c)], c) && cleaned != want {
					conflictIndex = i
					conflictName = cleaned
				}
			}
		}
	}

	if shimsIndex == -1 {
		return false, fmt.Sprintf("add %q to PATH", shims)
	}

	// shims 在 PATH 中，但检查是否有冲突路径排在前面
	if conflictIndex != -1 && conflictIndex < shimsIndex {
		return false, fmt.Sprintf("shims found but %s has higher priority (run: pvm setup)", conflictName)
	}

	return true, "found in PATH (highest priority)"
}

func checkPvmBinary() (bool, string) {
	exe, err := os.Executable()
	if err != nil {
		return false, err.Error()
	}
	expected := filepath.Join(config.BinHome(), "pvm"+config.ExeExt())
	if filepath.Clean(exe) != filepath.Clean(expected) {
		return false, fmt.Sprintf("running from %s (expected %s)", exe, expected)
	}
	return true, exe
}

func checkWritable() (bool, string) {
	home := config.PvmHome()
	if err := config.EnsureDir(home); err != nil {
		return false, err.Error()
	}
	test := filepath.Join(home, ".write-test")
	if err := os.WriteFile(test, []byte("ok"), 0644); err != nil {
		return false, err.Error()
	}
	os.Remove(test)
	return true, "ok"
}

// checkConflictingManagers 检测是否安装了与 pvm 冲突的版本管理工具
// checkLegacyPs1Shims 检测 shims 目录中是否残留旧版本生成的 .ps1 文件。
// 旧版 pvm 会为每个命令生成 <cmd>.ps1，但 PowerShell 默认 ExecutionPolicy 是
// Restricted/RemoteSigned，会拦截未签名脚本，导致 "node : 无法加载文件 ...
// 因为在此系统上禁止运行脚本" 报错。
// 新版 pvm 改用 <cmd>.exe（pvm.exe 副本）作为 shim，不再生成 .ps1。
func checkLegacyPs1Shims() (bool, string) {
	shims := config.ShimsDir()
	entries, err := os.ReadDir(shims)
	if err != nil {
		return true, "shims dir not readable, skipped"
	}
	var legacy []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".ps1") {
			legacy = append(legacy, e.Name())
		}
	}
	if len(legacy) == 0 {
		return true, "clean"
	}
	preview := legacy
	if len(preview) > 3 {
		preview = preview[:3]
	}
	return false, fmt.Sprintf("%d legacy .ps1 shim(s) found (e.g. %s) — run `pvm reshim`",
		len(legacy), strings.Join(preview, ", "))
}

func checkConflictingManagers() (bool, string) {
	// 已知版本管理工具的特征路径或可执行文件
	type vmTool struct {
		name      string
		exe       string // 可执行文件名（在 PATH 中查找）
		uninstall string // 卸载提示
	}
	tools := []vmTool{
		{"nvm", "nvm", "nvm uninstall <version>  then uninstall nvm itself"},
		{"volta", "volta", "volta uninstall node  (or uninstall Volta)"},
		{"fnm", "fnm", "fnm uninstall <version>  (or remove fnm from PATH)"},
		{"nodenv", "nodenv", "nodenv uninstall <version>  (or remove nodenv)"},
	}

	var found []string
	for _, t := range tools {
		if _, err := exec.LookPath(t.exe); err == nil {
			found = append(found, fmt.Sprintf("%s (%s)", t.name, t.uninstall))
		}
	}

	// 同时检查系统 PATH 中是否有冲突目录
	conflictBases := []string{"nodejs", "node", "python", "go", "golang", "ruby"}
	path := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	shims := filepath.Clean(config.ShimsDir())
	var sysConflicts []string
	for _, p := range strings.Split(path, sep) {
		clean := filepath.Clean(p)
		if clean == shims {
			break // shims 已经在最前，后面的不算冲突
		}
		base := strings.ToLower(filepath.Base(clean))
		for _, c := range conflictBases {
			if base == c || (len(base) > len(c) && base[:len(c)] == c) {
				sysConflicts = append(sysConflicts, p)
				break
			}
		}
	}

	if len(found) == 0 && len(sysConflicts) == 0 {
		return true, "no conflicts detected"
	}

	var msgs []string
	if len(sysConflicts) > 0 {
		msgs = append(msgs, fmt.Sprintf("system PATH conflict: %s (run: pvm setup as admin)", strings.Join(sysConflicts, ", ")))
	}
	if len(found) > 0 {
		msgs = append(msgs, fmt.Sprintf("installed: %s", strings.Join(found, "; ")))
	}
	return false, strings.Join(msgs, " | ")
}
