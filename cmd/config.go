package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// runConfig 管理项目级版本配置文件 (.pvmrc)
// 用途：完全避免系统 PATH 冲突，使用项目级版本隔离
func runConfig(args []string) error {
	if len(args) == 0 {
		return showConfigHelp()
	}

	subCmd := args[0]
	switch subCmd {
	case "init":
		return configInit(args[1:])
	case "show":
		return configShow(args[1:])
	case "set":
		return configSet(args[1:])
	case "remove":
		return configRemove(args[1:])
	case "help", "-h", "--help":
		return showConfigHelp()
	default:
		return fmt.Errorf("unknown config subcommand: %s", subCmd)
	}
}

// configInit 初始化 .pvmrc 文件（基于当前活跃版本）
func configInit(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pvmrcPath := filepath.Join(cwd, ".pvmrc")

	// 检查是否已存在
	if _, err := os.Stat(pvmrcPath); err == nil {
		logger.Info("  ℹ .pvmrc already exists at %s", pvmrcPath)
		logger.Info("  Use 'pvm config set <runtime> <version>' to update")
		return nil
	}

	fmt.Println()
	logger.Info("  → Initializing .pvmrc in %s", cwd)

	// 收集所有已安装的 runtime 的当前版本
	runtimes := []string{"node", "python", "go", "git", "ruby", "pnpm", "yarn", "corepack"}
	var entries []string

	for _, rt := range runtimes {
		ver, _ := config.ResolveVersion(rt, cwd)
		if ver != "" && ver != config.SystemVersion {
			entries = append(entries, fmt.Sprintf("%s=%s", rt, ver))
		}
	}

	if len(entries) == 0 {
		logger.Info("  ⚠ No managed runtimes found. Using defaults:")
		entries = []string{
			"# Add your runtime versions below (uncomment to use)",
			"# node=20.11.0",
			"# python=3.12.0",
			"# go=1.21.0",
		}
	} else {
		// 在现有条目前添加注释
		entries = append([]string{"# pvm project-level version config"}, entries...)
	}

	content := strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(pvmrcPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write .pvmrc: %w", err)
	}

	fmt.Println()
	logger.Info("  ✓ .pvmrc created")
	logger.Info("  📋 Content:")
	for _, line := range strings.Split(content, "\n") {
		if line != "" {
			logger.Info("     %s", line)
		}
	}

	fmt.Println()
	logger.Info("  💡 Next steps:")
	logger.Info("    1. Commit .pvmrc to version control: git add .pvmrc")
	logger.Info("    2. Share with team: they'll auto-load these versions")
	logger.Info("    3. Update versions: pvm config set node@22")

	return nil
}

// configShow 显示当前 .pvmrc 的内容
func configShow(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pvmrcPath := filepath.Join(cwd, ".pvmrc")
	data, err := os.ReadFile(pvmrcPath)
	if err != nil {
		return fmt.Errorf(".pvmrc not found in %s\n  Use 'pvm config init' to create one", cwd)
	}

	fmt.Println()
	logger.Info("  📄 .pvmrc content:")
	fmt.Println()

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			if line != "" {
				logger.Info("     %s", line)
			}
		} else {
			logger.Info("     ✓ %s", line)
		}
	}

	return nil
}

// configSet 设置 .pvmrc 中的 runtime 版本
// 用法: pvm config set node@20 或 pvm config set node 20
func configSet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pvm config set <runtime>@<version>")
	}

	spec := args[0]
	parts := strings.Split(spec, "@")
	var rt, ver string

	if len(parts) == 2 {
		rt = parts[0]
		ver = parts[1]
	} else if len(parts) == 1 && len(args) >= 2 {
		rt = parts[0]
		ver = args[1]
	} else {
		return fmt.Errorf("usage: pvm config set node@20 or pvm config set node 20")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pvmrcPath := filepath.Join(cwd, ".pvmrc")
	data, err := os.ReadFile(pvmrcPath)
	if err != nil {
		return fmt.Errorf(".pvmrc not found\n  Use 'pvm config init' to create one")
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(rt)+"=") {
			lines[i] = fmt.Sprintf("%s=%s", rt, ver)
			found = true
			break
		}
	}

	if !found {
		// 在最后一行前插入新条目
		newLines := []string{}
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "#") && !found {
				newLines = append(newLines, fmt.Sprintf("%s=%s", rt, ver))
				found = true
			}
			newLines = append(newLines, line)
		}
		if !found {
			newLines = append(newLines, fmt.Sprintf("%s=%s", rt, ver))
		}
		lines = newLines
	}

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(pvmrcPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write .pvmrc: %w", err)
	}

	fmt.Println()
	logger.Info("  ✓ Updated .pvmrc: %s=%s", rt, ver)
	logger.Info("  💡 Run 'pvm use' to apply changes, or commit to share with team")
	return nil
}

// configRemove 从 .pvmrc 中移除 runtime 版本
func configRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pvm config remove <runtime>")
	}

	rt := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pvmrcPath := filepath.Join(cwd, ".pvmrc")
	data, err := os.ReadFile(pvmrcPath)
	if err != nil {
		return fmt.Errorf(".pvmrc not found")
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(rt)+"=") {
			found = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !found {
		return fmt.Errorf("runtime '%s' not found in .pvmrc", rt)
	}

	content := strings.Join(newLines, "\n")
	if err := os.WriteFile(pvmrcPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write .pvmrc: %w", err)
	}

	fmt.Println()
	logger.Info("  ✓ Removed %s from .pvmrc", rt)
	return nil
}

// showConfigHelp 显示 config 命令帮助
func showConfigHelp() error {
	fmt.Println()
	fmt.Println("  pvm config - Manage project-level version configuration")
	fmt.Println()
	fmt.Println("  USAGE")
	fmt.Println("    pvm config <subcommand> [args]")
	fmt.Println()
	fmt.Println("  SUBCOMMANDS")
	fmt.Println()
	fmt.Println("    init              Create .pvmrc from current active versions")
	fmt.Println("    show              Display .pvmrc content")
	fmt.Println("    set <runtime>@v   Set or update a runtime version")
	fmt.Println("    remove <runtime>  Remove a runtime from .pvmrc")
	fmt.Println()
	fmt.Println("  EXAMPLES")
	fmt.Println()
	fmt.Println("    pvm config init                    # Create .pvmrc")
	fmt.Println("    pvm config show                    # Show content")
	fmt.Println("    pvm config set node@20             # Update node version")
	fmt.Println("    pvm config set python 3.12         # Alternative syntax")
	fmt.Println("    pvm config remove node             # Remove node")
	fmt.Println()
	fmt.Println("  BENEFITS")
	fmt.Println("    • No admin privileges needed")
	fmt.Println("    • Share versions with team via git")
	fmt.Println("    • Completely avoids system PATH conflicts")
	fmt.Println("    • Auto-loads on 'pvm use' command")
	fmt.Println()
	return nil
}
