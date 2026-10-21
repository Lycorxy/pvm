//go:build windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// currentAccountName 返回当前进程运行账户（USERDOMAIN\USERNAME）。
// UAC 提升后 USERNAME/USERDOMAIN 仍是原始交互用户，
// 因此管理员模式下用它作为 PVM_HOME 文件的预期所有者。
func currentAccountName() string {
	user := os.Getenv("USERNAME")
	if user == "" {
		return ""
	}
	domain := os.Getenv("USERDOMAIN")
	if domain == "" {
		return user
	}
	return domain + "\\" + user
}

// escapePowerShellSingleQuote 转义 PowerShell 单引号字符串
func escapePowerShellSingleQuote(s string) string {
	return escapePowerShellString(s)
}

// quickOwnerCheck 快速检查若干关键路径的文件所有者（O(1) 次 PowerShell 调用）。
// 返回 map[路径]所有者；不存在的路径不出现。
func quickOwnerCheck(paths []string, owner string) (map[string]string, error) {
	var sb strings.Builder
	sb.WriteString("$ErrorActionPreference='SilentlyContinue'\n")
	sb.WriteString(fmt.Sprintf("$me='%s'\n", escapePowerShellSingleQuote(owner)))
	sb.WriteString("foreach ($p in @(")
	for i, p := range paths {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("'%s'", escapePowerShellSingleQuote(p)))
	}
	sb.WriteString(")) {\n")
	sb.WriteString("  if (Test-Path -LiteralPath $p) {\n")
	sb.WriteString("    $o = (Get-Acl -LiteralPath $p).Owner\n")
	sb.WriteString("    Write-Output ($p + '|' + $o)\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	out, err := exec.Command("powershell", "-NoProfile", "-EncodedCommand",
		encodePowerShellCommand(sb.String())).Output()
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.LastIndex(line, "|")
		if idx <= 0 {
			continue
		}
		result[line[:idx]] = line[idx+1:]
	}
	return result, nil
}

// countOwnershipPollution 统计 root 下所有者不是 expectOwner 的文件/目录数量。
// 扫描上限 ownerScanLimit 条；无法读取所有者的条目跳过。
func countOwnershipPollution(root, expectOwner string) (int, error) {
	script := fmt.Sprintf(`
$ErrorActionPreference = 'SilentlyContinue'
$me = '%s'
$polluted = 0
$scanned = 0
Get-ChildItem -LiteralPath '%s' -Recurse -Force | ForEach-Object {
  if ($scanned -ge %d) { return }
  $scanned++
  $acl = Get-Acl -LiteralPath $_.FullName
  if ($acl -and $acl.Owner -and ($acl.Owner.Trim() -ne $me)) { $polluted++ }
}
Write-Output ("POLLUTED=" + $polluted)
`,
		escapePowerShellSingleQuote(expectOwner),
		escapePowerShellSingleQuote(root),
		ownerScanLimit,
	)

	out, err := exec.Command("powershell", "-NoProfile", "-EncodedCommand",
		encodePowerShellCommand(script)).Output()
	if err != nil {
		return 0, fmt.Errorf("cannot scan ownership of %s: %w", root, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "POLLUTED="); ok {
			var polluted int
			if _, err := fmt.Sscanf(value, "%d", &polluted); err == nil {
				return polluted, nil
			}
		}
	}
	return 0, fmt.Errorf("cannot parse ownership scan output for %s", root)
}

// repairOwnership 将 root 下全部条目的所有者改为 owner 并授予完全控制。
// 必须在管理员权限下运行：普通权限无法修改 Administrators 所有的文件。
func repairOwnership(root, owner string) error {
	// 1. 归还所有权（提升的管理员可将所有者设为自己）
	if out, err := exec.Command("icacls", root, "/setowner", owner, "/T", "/C", "/Q").CombinedOutput(); err != nil {
		return fmt.Errorf("icacls setowner failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	// 2. 授予当前用户完全控制（覆盖继承 ACL 中可能缺失的写权限）
	if out, err := exec.Command("icacls", root, "/grant", owner+":(OI)(CI)F", "/T", "/C", "/Q").CombinedOutput(); err != nil {
		return fmt.Errorf("icacls grant failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ownershipHealthy 快速判断 PVM_HOME 是否可能存在所有权污染：
// 检查根目录 / bin/pvm.exe / shims 三个关键位置，全部正确则跳过深度扫描。
func ownershipHealthy(home, owner string) bool {
	checkPaths := []string{home}
	pvmExe := filepath.Join(config.BinHome(), "pvm"+config.ExeExt())
	if _, err := os.Stat(pvmExe); err == nil {
		checkPaths = append(checkPaths, pvmExe)
	}
	checkPaths = append(checkPaths, config.ShimsDir())

	owners, err := quickOwnerCheck(checkPaths, owner)
	if err != nil {
		return true // 检查失败时不阻塞，交给深度扫描或用户手动处理
	}
	for _, p := range checkPaths {
		if o, ok := owners[p]; ok && !strings.EqualFold(o, owner) {
			return false
		}
	}
	return true
}

// ensureCleanOwnershipWindows Windows 版所有权守护（见 ownership.go 总说明）
func ensureCleanOwnershipWindows(autoElevate bool) {
	home := config.PvmHome()
	owner := currentAccountName()
	if owner == "" {
		return
	}

	if isElevated() {
		logger.Info("  → Restoring ownership of %s to %s (admin-mode cleanup)...", home, owner)
		if err := repairOwnership(home, owner); err != nil {
			logger.Info("  ⚠ Ownership cleanup failed: %v", err)
			logger.Info("    Run 'pvm repair-owner' in an Administrator terminal to fix.")
			return
		}
		logger.Info("  ✓ Ownership restored to %s — normal (non-admin) terminals stay fully writable", owner)
		return
	}

	// 普通态：快速检查 → 深度扫描 → 提示/修复
	if ownershipHealthy(home, owner) {
		return
	}
	polluted, err := countOwnershipPollution(home, owner)
	if err != nil || polluted == 0 {
		return
	}

	fmt.Println()
	logOwnershipPollution(home, owner, polluted)

	if !autoElevate {
		logger.Info("  ⚠ Skipped auto-fix (--no-elevate). To fix, run: pvm repair-owner")
		return
	}

	logger.Info("  → Requesting administrator permission to restore ownership...")
	fmt.Println()
	if err := runAsAdmin([]string{"repair-owner"}); err != nil {
		logger.Info("  ⚠ Auto-fix cancelled. Manual fix (run in an Administrator terminal):")
		logger.Info("      icacls \"%s\" /setowner %s /T /C /Q", home, owner)
		logger.Info("      icacls \"%s\" /grant %s:(OI)(CI)F /T /C /Q", home, owner)
		return
	}

	if polluted, err := countOwnershipPollution(home, owner); err == nil && polluted > 0 {
		logger.Info("  ⚠ %d entries still not owned by %s after repair", polluted, owner)
	} else {
		logger.Info("  OK Ownership restored — reinstall packages from a normal terminal.")
	}
}

// repairOwnerWindows 处理 `pvm repair-owner`（Windows 实现）
func repairOwnerWindows(args []string) error {
	_, elevatedFlag := hasFlag(args, "--elevated")

	home := config.PvmHome()
	owner := currentAccountName()
	if owner == "" {
		return fmt.Errorf("cannot determine current user (USERNAME is not set)")
	}

	fmt.Println()
	fmt.Printf("  pvm repair-owner (v%s, windows)\n", Version)
	fmt.Println()

	if elevatedFlag || isElevated() {
		logger.Info("  → Restoring ownership of %s to %s ...", home, owner)
		if err := repairOwnership(home, owner); err != nil {
			return err
		}
		polluted, err := countOwnershipPollution(home, owner)
		if err != nil {
			logger.Info("  ✓ Ownership restored (re-check skipped: %v)", err)
			return nil
		}
		if polluted > 0 {
			return fmt.Errorf("%d entries under %s are still not owned by %s", polluted, home, owner)
		}
		logger.Info("  ✓ All files under %s are now owned by %s", home, owner)
		logOwnershipRepaired()
		return nil
	}

	polluted, err := countOwnershipPollution(home, owner)
	if err != nil {
		return err
	}
	if polluted == 0 {
		logger.Info("  ✓ No ownership pollution found under %s", home)
		return nil
	}

	logOwnershipPollution(home, owner, polluted)
	logger.Info("  → Requesting administrator permission to fix...")
	fmt.Println()

	if err := runAsAdmin([]string{"repair-owner"}); err != nil {
		logger.Info("  ⚠ Auto-fix declined. Manual fix (run in an Administrator terminal):")
		logger.Info("      icacls \"%s\" /setowner %s /T /C /Q", home, owner)
		logger.Info("      icacls \"%s\" /grant %s:(OI)(CI)F /T /C /Q", home, owner)
		return err
	}

	if polluted, err := countOwnershipPollution(home, owner); err == nil && polluted > 0 {
		return fmt.Errorf("%d entries under %s are still not owned by %s", polluted, home, owner)
	}
	logger.Info("  OK Ownership restored. Reopen your normal terminal and reinstall packages.")
	return nil
}
