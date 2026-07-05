package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
	"github.com/pvm/pvm/internal/plugins"
)

// Version 是 pvm 自身版本号（构建时可被 ldflags 覆盖）
var Version = "0.0.0"

func Execute() error {
	// 注册所有 runtime 插件
	plugins.RegisterAll()

	// Windows .exe shim 检测：
	// 当 pvm.exe 被复制为 node.exe / npm.exe 等放在 shims 目录时，
	// 通过 argv[0] 的文件名识别命令名，直接走 shim-exec 逻辑。
	// 这样 Windows 在 PATH 里找 node.exe 时能找到 shims/node.exe 而非系统的 node.exe。
	if exeName := shimExeNameFromArgv0(); exeName != "" {
		return runShimExec(exeName, os.Args[1:])
	}

	// 如果 pvm 不在 ~/.pvm/bin/ 下且没有传入任何子命令，
	// 说明是首次双击运行 exe，自动触发 setup
	if len(os.Args) < 2 {
		if shouldAutoSetup() {
			fmt.Println("  Welcome to pvm! It looks like this is your first run.")
			fmt.Println("  Running setup to configure pvm...")
			fmt.Println()
			return runSetup(nil)
		}
		printUsage()
		return nil
	}

	// shim-exec 是内部命令，不走全局 flag 解析
	if os.Args[1] == "shim-exec" {
		if len(os.Args) < 3 {
			return fmt.Errorf("shim-exec: missing command name")
		}
		return runShimExec(os.Args[2], os.Args[3:])
	}

	// 其他命令解析全局 flag
	var filteredArgs []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--verbose", "-V":
			logger.SetLevel(logger.LevelVerbose)
		case "--quiet", "-q":
			logger.SetLevel(logger.LevelQuiet)
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}

	if len(filteredArgs) == 0 {
		printUsage()
		return nil
	}

	command := filteredArgs[0]
	args := filteredArgs[1:]

	switch command {
	case "install", "i":
		return runInstall(args)
	case "use":
		return runUse(args)
	case "list", "ls":
		return runList(args)
	case "list-remote", "lr":
		return runListRemote(args)
	case "remove", "rm":
		return runRemove(args)
	case "current":
		return runCurrent(args)
	case "which":
		return runWhich(args)
	case "where":
		return runWhere(args)
	case "init":
		return runInit(args)
	case "reshim":
		return runReshim(args)
	case "self-update":
		return runSelfUpdate(args)
	case "uninstall":
		return runSelfUninstall(args)
	case "setup":
		return runSetup(args)
	case "setup-path":
		return runSetupPath(args)
	case "doctor":
		return runDoctor(args)
	case "validate":
		return runValidate(args)
	case "diagnostics":
		return runDiagnostics(args)
	case "config", "cfg":
		return runConfig(args)
	case "git":
		if len(args) > 0 && args[0] == "ssh" {
			return runGitSSH(args[1:])
		}
		fmt.Fprintf(os.Stderr, "Unknown git subcommand: %s\n\n", args[0])
		printGitSSHUsage()
		return fmt.Errorf("unknown git subcommand: %s", args[0])
	case "ssh-config":
		return runGitSSH(args)
	case "version", "-v", "--version":
		fmt.Printf("pvm %s\n", Version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	usage := `pvm - Polyglot Version Manager v` + Version + `

Usage:
  pvm <command> [arguments] [flags]

Version management (shim-driven):
  install, i           Install a runtime version
  use                  Set version for project (.pvmrc) or user
  list, ls             List installed versions
  list-remote, lr      List available remote versions
  remove, rm           Uninstall a version or remove config entry
  current              Show currently active versions (for cwd)
  which <cmd>          Print the real path of a command
  where <runtime>      Print the install dir of a runtime
  init                 Create .pvmrc in current directory
  reshim               Rebuild shim scripts (after adding a binary)

System:
  setup                First-time setup (dirs, PATH, shims)
  setup-path           Check and fix PATH configuration (diagnose conflicts)
  config               Manage .pvmrc project config (no admin needed!)
  doctor               Check pvm installation health
  validate             Deep validation of shims & version consistency [--auto-fix]
  diagnostics <rt>     Detailed diagnostics for a specific runtime
  self-update          Update pvm itself
  uninstall            Uninstall pvm from this machine
  version              Show pvm version
  help                 Show this help

Git SSH:
  git ssh              Configure SSH for Git platforms (GitHub/GitLab/Gitee)
  ssh-config           Alias for 'git ssh'

Global flags:
  --verbose, -V        Show detailed output
  --quiet, -q          Suppress non-essential output

Scope flags (default: --user):
  --user, -u           Operate on user-level config (~/.pvm/versions) [default]
  --local, -l          Operate on project-level config (.pvmrc)
  --system, -s         Use system-installed version (only with 'pvm use')

Install flags:
  --mirror, -m         Use China mirror for download
  --official, -o       Use official source (overrides default mirror)
  --force, -f          Force reinstall even if already installed

Examples:
  # Install (default scope: --user)
  pvm install node@20.11.0                # install + set as user default
  pvm install python@3.12.0 go@1.22.0
  pvm install node@20.11.0 --local        # install + write to .pvmrc
  pvm install --force node@20.11.0        # force reinstall
  pvm install                             # auto: project .pvmrc → user fallback
  pvm install --local                     # only from .pvmrc
  pvm install --user                      # only from ~/.pvm/versions

  # Use (default scope: --user)
  pvm use node@20.11.0                    # set user default (~/.pvm/versions)
  pvm use node@20.11.0 --local            # write to ./.pvmrc (project)
  pvm use node --system                   # use system-installed node (user)
  pvm use node --system --local           # use system-installed node (project)
  pvm use                                 # show user-level config
  pvm use --local                         # show project-level config

  # List
  pvm list                                # list all installed versions
  pvm list node                           # list installed node versions
  pvm list --user                         # show user version config
  pvm list --local                        # show project version config

  # Remove
  pvm remove node@20.11.0                 # uninstall a version completely
  pvm remove git                          # auto-detect & remove all git versions
  pvm remove node@20.11.0 --force         # force remove active version
  pvm remove node --user                  # only remove from user config
  pvm remove node --local                 # only remove from .pvmrc

  # Init (default scope: --user)
  pvm init                                # initialize ~/.pvm/versions
  pvm init node@20 go@1.22                # init user config with versions
  pvm init --local                        # create .pvmrc in current dir
  pvm init --local node@20                # create .pvmrc with versions

Version resolution order (highest to lowest priority):
  1. Env var:   PVM_NODE_VERSION=20.11.0
  2. Local:     .pvmrc / .nvmrc (searched upward)
  3. User:      ~/.pvm/versions
  4. System:    system-installed (auto fallback, no config needed)

Project config (` + "`.pvmrc`" + `):
  node 20.11.0
  python 3.12.0
  go 1.22.0
  pnpm 9.0.0
`
	fmt.Println(usage)
}

// parseRuntimeArg parses "node@20.11.0" into ("node", "20.11.0")
// 也接受纯版本号但必须带 runtime 前缀
func parseRuntimeArg(arg string) (runtime string, ver string, err error) {
	parts := strings.SplitN(arg, "@", 2)
	if parts[0] == "" {
		return "", "", fmt.Errorf("invalid format: %q, expected runtime[@version] (e.g. node or node@20.11.0)", arg)
	}
	runtime = strings.ToLower(parts[0])
	// 未指定版本时默认安装最新稳定版
	if len(parts) == 1 || parts[1] == "" {
		ver = "latest"
	} else {
		ver = strings.TrimPrefix(parts[1], "v")
	}

	// 别名
	switch runtime {
	case "nodejs":
		runtime = "node"
	case "golang":
		runtime = "go"
	case "py":
		runtime = "python"
	// npm/npx/corepack 是 Node.js 自带的工具，映射到 node runtime
	case "npm", "npx", "corepack":
		runtime = "node"
	}

	// poetry / pdm 等 Python CLI 工具不再由 pvm 管理，引导用户用 pipx
	if runtime == "poetry" || runtime == "pdm" {
		return "", "", fmt.Errorf(
			"%s is a Python CLI tool, not a runtime managed by pvm.\n"+
				"  Install it with pipx instead:\n"+
				"    1. Make sure python is installed:\n"+
				"         pvm install python\n"+
				"    2. Install pipx (one-time):\n"+
				"         python -m pip install --user pipx\n"+
				"         python -m pipx ensurepath\n"+
				"    3. Install %s:\n"+
				"         pipx install %s",
			runtime, runtime, runtime)
	}

	if !config.IsSupportedRuntime(runtime) {
		return "", "", fmt.Errorf("unsupported runtime: %q, supported: %s",
			runtime, strings.Join(config.SupportedRuntimes, ", "))
	}
	return runtime, ver, nil
}

// hasFlag 判断 args 是否包含某个 flag，并返回去除后的 args
func hasFlag(args []string, flags ...string) ([]string, bool) {
	out := make([]string, 0, len(args))
	found := false
	flagSet := make(map[string]bool, len(flags))
	for _, f := range flags {
		flagSet[f] = true
	}
	for _, a := range args {
		if flagSet[a] {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}

// shimExeNameFromArgv0 检测当前进程是否以 shim .exe 名字被调用。
// 例如 shims/node.exe 被调用时，可执行文件 base 是 "node.exe"，
// 此时返回 "node"；如果是 pvm.exe 本身则返回 ""。
//
// 关键：必须用 os.Executable() 取真实磁盘路径，
//
//	os.Args[0] 在 Windows 上常常只是 "node" 这样的命令名，
//	无法用来判断是否在 shims 目录下。
func shimExeNameFromArgv0() string {
	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(exePath))
	// 去掉 .exe 后缀
	name := strings.TrimSuffix(base, ".exe")
	// 排除 pvm 自身
	if name == "pvm" {
		return ""
	}

	// 必须位于 shims 目录下才认为是 shim 调用，避免误判
	// 使用 filepath.Abs + 大小写不敏感比较，处理 Windows 8.3 短路径名问题
	shimsDir := config.ShimsDir()
	absExe, err1 := filepath.Abs(exePath)
	absShims, err2 := filepath.Abs(shimsDir)
	if err1 != nil || err2 != nil {
		// 如果无法获取绝对路径，回退到简单字符串比较
		lowerExe := strings.ToLower(filepath.ToSlash(exePath))
		lowerShims := strings.ToLower(filepath.ToSlash(shimsDir))
		if strings.HasPrefix(lowerExe, lowerShims) {
			return name
		}
		return ""
	}

	// Windows 下使用 EqualFold 做大小写不敏感比较
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(absExe, absShims) && !strings.HasPrefix(strings.ToLower(absExe), strings.ToLower(absShims)+string(os.PathSeparator)) {
			return ""
		}
	} else {
		if absExe != absShims && !strings.HasPrefix(absExe, absShims+string(os.PathSeparator)) {
			return ""
		}
	}
	return name
}
