package cmd

import (
	"fmt"

	"github.com/pvm/pvm/internal/logger"
)

// ---- PVM_HOME 文件所有权守护（跨平台）----
//
// 问题背景（Windows 与 Unix 对称存在）：
//   - Windows：用"管理员身份"运行安装器 / pvm setup / pvm install 后，
//     提升进程在 ~/.pvm 下创建的文件所有者是 BUILTIN\Administrators，
//     之后普通权限的 pnpm/node shim 无法写入（EPERM）。
//   - Unix：用 sudo 运行上述命令后，~/.pvm 下的文件归 root 所有，
//     普通用户同样写不进去。
//
// 两种情况的表现一致：普通终端/IDE 装包永远失败，删除重装也无效
// （普通权限删不掉污染文件，或重装时再次被提升进程污染），
// 必须再次提升权限才能成功——形成死循环。
//
// 通用方案（三层，两端一致）：
//  1. 预防：提升态（管理员/root）运行 pvm setup 完成后，自动把 PVM_HOME
//     的所有权归还给交互用户 —— 见 ensureCleanOwnership。
//  2. 自愈：pvm repair-owner 检测污染并修复
//     （Windows 经 UAC 提权；Unix 提示 sudo pvm repair-owner）。
//  3. 选择：pvm setup --scope user|admin（Windows）让用户显式选择安装方式；
//     Unix 安装从不需要 root，无需选择。

// ownerScanLimit 单次扫描的最大条目数，避免超大目录时扫描过久
const ownerScanLimit = 20000

// 平台入口（在各平台文件中以同名实现，build tag 互斥）：
//   - ensureCleanOwnership(autoElevate bool)：setup 完成时的所有权守护
//   - runRepairOwner(args []string)：pvm repair-owner 命令实现

// logOwnershipPollution 输出统一的污染警告（两端共用）
func logOwnershipPollution(home, owner string, polluted int) {
	logger.Info("  !! Found %d file(s)/dir(s) under %s not owned by %s", polluted, home, owner)
	logger.Info("     Files created by an elevated session block normal-user writes,")
	logger.Info("     which causes package installs to fail in non-elevated terminals.")
}

// logOwnershipRepaired 输出统一的修复成功信息（两端共用）
func logOwnershipRepaired() {
	fmt.Println()
	logger.Info("  Normal (non-elevated) terminals can now update pvm-managed files.")
	logger.Info("  Reinstall your packages WITHOUT elevation — no more 'xxx install failed'.")
	fmt.Println()
	logger.Info("  Tip: avoid running terminals/IDEs elevated for daily work;")
	logger.Info("  if you must, run 'pvm repair-owner' afterwards.")
	fmt.Println()
}
