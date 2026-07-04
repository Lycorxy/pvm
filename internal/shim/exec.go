package shim

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/installer"
	"github.com/pvm/pvm/internal/logger"
)

// Exec 是 shim 的运行时入口：
//  1. 根据 cmdName 反查属于哪个 runtime
//  2. 从 cwd 解析出应用版本
//  3. 找到真实二进制并 exec 之（保留 stdin/out/err 和退出码）
//
// 版本解析顺序：环境变量 → 项目 .pvmrc → 全局 ~/.pvm/versions → 系统 PATH
// 该函数要么成功替换进程（Unix），要么以相同退出码返回（Windows）
func Exec(cmdName string, args []string) error {
	rt := FindRuntimeForCommand(cmdName)
	if rt == "" {
		// 未知命令：尝试直接从系统 PATH 执行
		return execSystemCommand(cmdName, args)
	}

	cwd, _ := os.Getwd()
	version, _ := config.ResolveVersion(rt, cwd)

	// 系统 fallback：项目和全局均未配置，使用系统 PATH 中的全局安装
	if version == config.SystemVersion {
		return execSystemCommand(cmdName, args)
	}

	if version == "" {
		return fmt.Errorf(`pvm: no version configured for %s.

Set a version via one of:
  • pvm use %s@<version>          (local, writes .pvmrc)
  • pvm use %s@<version> --user   (user default)
  • export PVM_%s_VERSION=<ver>   (environment)
  • Install %s globally on your system (system fallback)`,
			rt, rt, rt, strings.ToUpper(rt), rt)
	}

	if !config.IsInstalled(rt, version) {
		// 先检查是否有已安装版本能满足该版本要求（处理模糊版本如 "20"、"3.12"）
		// 避免版本已安装但因模糊匹配导致误触发网络安装
		if matched := config.FindInstalledMatch(rt, version); matched != "" {
			logger.Verbose("pvm: %s@%s matched installed version %s", rt, version, matched)
			version = matched
		} else {
			// 确实未安装：自动下载安装，默认使用国内镜像加速
			logger.Info("pvm: %s@%s is not installed, auto-installing... [国内镜像]", rt, version)
			logger.Info("pvm: tip: use `pvm install %s@%s --official` to force official source", rt, version)
			if err := config.EnsureAllDirs(); err != nil {
				return fmt.Errorf("pvm: failed to create directories: %w", err)
			}
			installed, err := installer.Install(rt, version, true, false)
			if err != nil {
				// 镜像失败时回退到官方源重试
				logger.Info("pvm: mirror failed, retrying with official source... [官方源]")
				installed, err = installer.Install(rt, version, false, false)
			}
			if err != nil {
				return fmt.Errorf("pvm: auto-install %s@%s failed: %w\n\nYou can also install manually:\n  pvm install %s@%s",
					rt, version, err, rt, version)
			}
			version = installed
			// 安装成功后重建 shims（新版本可能带来新命令）
			if err := Reshim(); err != nil {
				logger.Verbose("pvm: reshim after auto-install: %v", err)
			}
		}
	}

	bin, err := ResolveBinary(rt, version, cmdName)
	if err != nil {
		return err
	}

	// 为子进程准备环境：把主 runtime 及其依赖 runtime 的 bin 目录前置到 PATH。
	// 关键：pnpm/yarn 依赖 node，必须把项目级 node 的 bin 也加入 PATH，
	// 否则 pnpm.cmd → node 会用到系统 PATH 里错误的 node 版本（项目级失效）。
	env := prepareEnv(rt, version, cwd)

	return execBinary(bin, args, env)
}

// execSystemCommand 在系统 PATH 中查找并执行命令（跳过 pvm shims）
func execSystemCommand(cmdName string, args []string) error {
	bin, err := config.SystemCommandPath(cmdName)
	if err != nil {
		return fmt.Errorf("pvm: %s not found in pvm installs or system PATH", cmdName)
	}
	return execBinary(bin, args, os.Environ())
}

// execBinary 执行二进制文件（Unix exec 替换进程，Windows 子进程透传退出码）
// Windows 特殊处理：确保 Ctrl+C 正确传递，避免 stdin 阻塞长运行进程
func execBinary(bin string, args []string, env []string) error {
	// Unix: exec 替换当前进程，退出码天然传递
	if runtime.GOOS != "windows" {
		argv := append([]string{bin}, args...)
		return syscall.Exec(bin, argv, env)
	}

	// Windows: 无法真 exec，用子进程 + 退出码透传
	// 改进：添加信号转发，确保 Ctrl+C 正确传播到子进程
	// .cmd / .bat 文件必须通过 cmd.exe /c 执行，否则会报"不是可执行文件"
	var cmd *exec.Cmd
	lower := strings.ToLower(bin)
	if strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat") {
		cmdArgs := append([]string{"/c", bin}, args...)
		cmd = exec.Command("cmd.exe", cmdArgs...)
	} else {
		cmd = exec.Command(bin, args...)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	// 设置信号处理：拦截 Ctrl+C 并转发给子进程
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if err := cmd.Start(); err != nil {
		signal.Stop(sigChan)
		return err
	}

	// 后台监听信号，如果收到则转发给子进程
	go func() {
		sig := <-sigChan
		if cmd.Process != nil {
			// Windows 下用 os.Kill 发送 CTRL+C
			if runtime.GOOS == "windows" {
				cmd.Process.Signal(os.Interrupt)
			} else {
				cmd.Process.Signal(sig)
			}
		}
	}()

	// 等待子进程结束
	err := cmd.Wait()
	signal.Stop(sigChan)

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// 子进程非零退出：直接传递退出码，不包装为 pvm 错误。
			// 避免打印多余的 "Error: process exited with code N"（子进程已输出自己的错误信息），
			// 且保证退出码与真实工具一致（如 pnpm/node 退出码透传给 shell 和 CI）。
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

// prepareEnv 在当前 env 基础上，把主 runtime 及其依赖 runtime 的 bin 目录前置到 PATH。
//
// 项目级支持的关键（参考 mise 的 toolset 环境构建）：
//   - pnpm/yarn 依赖 node：执行 pnpm 时，必须把项目级 node 的 bin 也加入 PATH，
//     否则 pnpm.cmd → node 会用到系统 PATH 里错误的 node 版本，导致项目级 node 失效。
//   - node/python 等独立 runtime：只加自己的 bin 即可。
//
// cwd 用于解析依赖 runtime 的项目级版本（从 .pvmrc 向上查找）。
func prepareEnv(rt, version, cwd string) []string {
	binDir := config.BinDir(rt, version)
	env := os.Environ()

	// 收集所有需要前置到 PATH 的目录（顺序即优先级，主 runtime 最前）
	var prependDirs []string
	prependDirs = append(prependDirs, binDir)

	// 依赖的 runtime：pnpm/yarn → node
	// 把项目级（或用户级）node 的 bin 也加入 PATH，确保包管理器调用 node 时用对版本
	activeVersions := map[string]string{rt: version}
	if depRt, ok := config.RuntimeDeps[rt]; ok {
		depVer, _ := config.ResolveVersion(depRt, cwd)
		if depVer != "" && depVer != config.SystemVersion {
			// 依赖版本可能未安装（模糊版本），尝试匹配已安装版本
			if !config.IsInstalled(depRt, depVer) {
				if matched := config.FindInstalledMatch(depRt, depVer); matched != "" {
					depVer = matched
				}
			}
			if config.IsInstalled(depRt, depVer) {
				depBinDir := config.BinDir(depRt, depVer)
				prependDirs = append(prependDirs, depBinDir)
				activeVersions[depRt] = depVer
				logger.Verbose("pvm: %s@%s depends on %s@%s, added %s to PATH", rt, version, depRt, depVer, depBinDir)
			}
		}
	}

	sep := string(os.PathListSeparator)

	// 特殊：Git for Windows 的 bash.exe 在 bin/ 目录下，而 BinDir 返回 cmd/。
	// git commit 等命令内部会调用 bash，所以必须把 bin/、mingw64/libexec/git-core、usr/bin 也加入 PATH。
	if rt == "git" {
		installDir := config.InstallDir(rt, version)
		if gitBin := filepath.Join(installDir, "bin"); gitBin != binDir {
			prependDirs = append(prependDirs, gitBin)
		}
		prependDirs = append(prependDirs,
			filepath.Join(installDir, "mingw64", "libexec", "git-core"),
			filepath.Join(installDir, "usr", "bin"),
		)
	}

	// 找到 PATH 条目并前置所有目录
	foundIdx := -1
	foundKey := "PATH"
	for i, kv := range env {
		idx := strings.Index(kv, "=")
		if idx < 0 {
			continue
		}
		k := kv[:idx]
		if strings.EqualFold(k, "PATH") {
			foundIdx = i
			foundKey = k
			break
		}
	}
	if foundIdx >= 0 {
		cur := env[foundIdx][len(foundKey)+1:]
		env[foundIdx] = foundKey + "=" + strings.Join(prependDirs, sep) + sep + cur
	} else {
		env = append(env, "PATH="+strings.Join(prependDirs, sep))
	}

	// 特殊：Go 需要 GOROOT 指向安装目录
	if rt == "go" {
		goroot := filepath.Join(config.InstallDir(rt, version), "go")
		env = setEnv(env, "GOROOT", goroot)
	}

	// 记录当前正在运行的版本（含依赖 runtime），便于子进程/脚本感知
	for activeRt, activeVer := range activeVersions {
		env = setEnv(env, fmt.Sprintf("PVM_ACTIVE_%s", strings.ToUpper(activeRt)), activeVer)
	}

	return env
}

// setEnv 在 env 切片中设置/替换键值
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
