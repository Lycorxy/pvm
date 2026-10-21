//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/pvm/pvm/internal/config"
	"github.com/pvm/pvm/internal/logger"
)

// Unix 说明：
//   - pvm 的常规安装/使用从不需要 root（装 ~/.pvm、写 shell rc 文件），
//     因此不存在 Windows 那种"必须选择管理员/普通用户"的问题。
//   - 但 `sudo pvm install` / `sudo pvm setup` 会把 ~/.pvm 下的文件写成
//     root 所有，普通用户随后无法写入 —— 与 Windows 的管理员污染对称。
//   - 修复方式：sudo pvm repair-owner（root 下 chown -R 归还给交互用户）。

// currentAccountName 返回当前 uid 对应的用户名
func currentAccountName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return ""
}

// sudoUserName 返回 sudo 提升前的交互用户（SUDO_USER），非 sudo 会话返回空
func sudoUserName() string {
	su := os.Getenv("SUDO_USER")
	if su == "" {
		return ""
	}
	if u, err := user.Lookup(su); err == nil {
		return u.Username
	}
	return ""
}

// isRoot 当前进程是否以 root 运行
func isRoot() bool { return os.Geteuid() == 0 }

// resolveUid 把用户名解析为 uid；允许直接传数字 uid。
// 名称解析失败且当前非 root 时，回落到当前用户 uid。
func resolveUid(name string) (int, error) {
	if u, err := user.Lookup(name); err == nil {
		return strconv.Atoi(u.Uid)
	}
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	if !isRoot() {
		if u, err := user.Current(); err == nil {
			return strconv.Atoi(u.Uid)
		}
	}
	return -1, fmt.Errorf("cannot resolve user %q", name)
}

// countOwnershipPollution 统计 root 下 uid 不是预期用户的文件/目录数量。
// 扫描上限 ownerScanLimit 条。
func countOwnershipPollution(root, expectOwner string) (int, error) {
	expectUid, err := resolveUid(expectOwner)
	if err != nil {
		return 0, err
	}

	polluted := 0
	scanned := 0
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可访问条目
		}
		if scanned >= ownerScanLimit {
			return filepath.SkipAll
		}
		scanned++
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			if int(st.Uid) != expectUid {
				polluted++
			}
		}
		return nil
	})
	if walkErr != nil {
		return polluted, walkErr
	}
	return polluted, nil
}

// repairOwnership 将 root 下全部条目归还给 owner（chown -R）。
// 必须以 root 运行：普通用户无法修改他人所有文件的属主。
func repairOwnership(root, owner string) error {
	u, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("cannot resolve user %q: %w", owner, err)
	}
	out, err := exec.Command("chown", "-R", u.Uid+":"+u.Gid, root).CombinedOutput()
	if err != nil {
		return fmt.Errorf("chown failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ownershipHealthy Unix 文件系统遍历足够快，直接全量扫描判定
func ownershipHealthy(home, owner string) bool {
	polluted, err := countOwnershipPollution(home, owner)
	return err != nil || polluted == 0
}

// ensureCleanOwnership 所有权守护入口（Unix 实现，见 ownership.go 总说明）：
//   - root（sudo）态：自动 chown 归还给交互用户（防污染）
//   - 普通用户：检测污染并提示 sudo pvm repair-owner
func ensureCleanOwnership(autoElevate bool) {
	home := config.PvmHome()

	if isRoot() {
		// sudo 会话：把文件归还给交互用户（防污染）
		interactive := sudoUserName()
		if interactive == "" {
			// 纯 root 登录会话（无 SUDO_USER）：无法确定目标用户，跳过
			return
		}
		logger.Info("  → Restoring ownership of %s to %s (root-mode cleanup)...", home, interactive)
		if err := repairOwnership(home, interactive); err != nil {
			logger.Info("  ⚠ Ownership cleanup failed: %v", err)
			return
		}
		logger.Info("  ✓ Ownership restored to %s — normal (non-root) shells stay fully writable", interactive)
		return
	}

	owner := currentAccountName()
	if owner == "" {
		return
	}
	polluted, err := countOwnershipPollution(home, owner)
	if err != nil || polluted == 0 {
		return
	}

	fmt.Println()
	logOwnershipPollution(home, owner, polluted)
	logger.Info("  → Fix it with:  sudo pvm repair-owner")
	_ = autoElevate // Unix 不自动提权：sudo 需要密码输入，交由用户执行
}

// runRepairOwner 处理 `pvm repair-owner`（Unix 实现）：
// 普通用户检测污染并提示 sudo 修复；root（sudo）直接 chown 修复
func runRepairOwner(args []string) error {
	_ = args // elevatedFlag 不适用于 Unix：sudo 由用户显式执行

	home := config.PvmHome()

	fmt.Println()
	fmt.Printf("  pvm repair-owner (v%s, %s)\n", Version, runtime.GOOS)
	fmt.Println()

	if isRoot() {
		// sudo 会话：修复目标 = 交互用户；纯 root 登录则无目标，提示后退出
		interactive := sudoUserName()
		if interactive == "" {
			return fmt.Errorf("running as root without SUDO_USER; start a normal user shell and run 'sudo pvm repair-owner'")
		}
		logger.Info("  → Restoring ownership of %s to %s ...", home, interactive)
		if err := repairOwnership(home, interactive); err != nil {
			return err
		}
		polluted, err := countOwnershipPollution(home, interactive)
		if err != nil {
			logger.Info("  ✓ Ownership restored (re-check skipped: %v)", err)
			return nil
		}
		if polluted > 0 {
			return fmt.Errorf("%d entries under %s are still not owned by %s", polluted, home, interactive)
		}
		logger.Info("  ✓ All files under %s are now owned by %s", home, interactive)
		logOwnershipRepaired()
		return nil
	}

	owner := currentAccountName()
	polluted, err := countOwnershipPollution(home, owner)
	if err != nil {
		return err
	}
	if polluted == 0 {
		logger.Info("  ✓ No ownership pollution found under %s", home)
		return nil
	}

	logOwnershipPollution(home, owner, polluted)
	return fmt.Errorf("ownership repair requires root; run: sudo pvm repair-owner")
}
