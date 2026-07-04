package compat

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/pvm/pvm/internal/registry"
	"github.com/pvm/pvm/internal/semver"
)

// Prerequisite 表示某个 runtime 的前置依赖
type Prerequisite struct {
	Runtime     string // 前置依赖的 runtime（如 "node"）
	MinVersion  int    // 最低大版本号（仅检查主版本）
	Description string // 用户友好的描述
}

// RuntimePrerequisites 定义各个 runtime 的前置依赖关系
// Key: runtime name，Value: slice of prerequisites
var RuntimePrerequisites = map[string][]Prerequisite{
	"pnpm": {
		{Runtime: "node", MinVersion: 12, Description: "pnpm 6.x 需要 Node.js >= 12"},
	},
	"corepack": {
		{Runtime: "node", MinVersion: 16, Description: "corepack 需要 Node.js >= 16"},
	},
	"yarn": {
		{Runtime: "node", MinVersion: 12, Description: "Yarn >= 2.x 需要 Node.js >= 12"},
	},
	// 未来可扩展更多依赖关系
}

// pnpmNodeFallback 硬编码兜底：各 pnpm 大版本所需的最低 Node.js 版本
// 仅在无法联网查询 engines.node 时使用
// 来源：https://pnpm.io/installation#compatibility
var pnpmNodeFallback = map[int]int{
	10: 18,
	9:  18,
	8:  16,
	7:  14,
	6:  12,
}

// reNodeRange 匹配 engines.node 中的最低版本号，如 ">=18.12" ">= 18" ">=16"
var reNodeRange = regexp.MustCompile(`>=\s*(\d+)`)

// parseMinNodeFromRange 从 engines.node 字符串中提取最低 Node 大版本号
// 例如 ">=18.12" → 18，"^18.0.0" → 18，解析失败返回 0
func parseMinNodeFromRange(engines string) int {
	m := reNodeRange.FindStringSubmatch(engines)
	if len(m) < 2 {
		// 尝试匹配 ^N 或 ~N
		re2 := regexp.MustCompile(`[\^~](\d+)`)
		m2 := re2.FindStringSubmatch(engines)
		if len(m2) < 2 {
			return 0
		}
		n, _ := strconv.Atoi(m2[1])
		return n
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// CheckPnpmNodeCompat 检查 pnpm 版本与 node 版本是否兼容
// pnpmVer: 如 "10.33.2"、"9.0.0"
// nodeVer: 如 "20.17.0"、"18.19.0"
// useMirror: 是否使用 npmmirror 镜像查询
// 返回 nil 表示兼容，否则返回带提示的错误
func CheckPnpmNodeCompat(pnpmVer, nodeVer string, useMirror bool) error {
	pnpmMajor := semver.Parse(pnpmVer).Major
	nodeMajor := semver.Parse(nodeVer).Major

	if pnpmMajor == 0 || nodeMajor == 0 {
		return nil // 无法解析版本，不阻断
	}

	// 优先：从 npm registry 动态读取 engines.node
	minNode := 0
	if enginesNode := registry.GetPnpmNodeRequirement(pnpmVer, useMirror); enginesNode != "" {
		minNode = parseMinNodeFromRange(enginesNode)
	}

	// 降级：查询失败或解析失败，使用硬编码兜底
	if minNode == 0 {
		if fallback, ok := pnpmNodeFallback[pnpmMajor]; ok {
			minNode = fallback
		}
	}

	// 仍然未知：不阻断，直接放行
	if minNode == 0 {
		return nil
	}

	if nodeMajor < minNode {
		return fmt.Errorf(
			`✗ Incompatible versions:
  pnpm %s requires Node.js >= %d, but current node is %s (major: %d)

  Fix options:
  1. Use a compatible pnpm version:  pvm use pnpm@%s
  2. Upgrade node:                   pvm use node@%d`,
			pnpmVer, minNode, nodeVer, nodeMajor,
			suggestPnpmForNode(nodeMajor),
			minNode,
		)
	}

	return nil
}

// suggestPnpmForNode 根据 node 大版本推荐最高兼容的 pnpm 大版本
func suggestPnpmForNode(nodeMajor int) string {
	best := 0
	for pnpmMajor, minNode := range pnpmNodeFallback {
		if nodeMajor >= minNode && pnpmMajor > best {
			best = pnpmMajor
		}
	}
	if best == 0 {
		return "6"
	}
	return strconv.Itoa(best)
}

// CheckPrerequisites 检查 runtime 的前置依赖
// rt: 要安装的 runtime（如 "pnpm"）
// rtVersion: runtime 的版本（如 "10.33.2"）
// getCurrentVersion: 获取当前活跃版本的回调函数
// 返回 error 如果存在不满足的前置依赖
func CheckPrerequisites(rt, rtVersion string, getCurrentVersion func(runtime string) string) error {
	prereqs, ok := RuntimePrerequisites[rt]
	if !ok {
		return nil // 无前置依赖
	}

	var errors []string
	for _, req := range prereqs {
		currentVer := getCurrentVersion(req.Runtime)
		if currentVer == "" {
			errors = append(errors, fmt.Sprintf(
				"  ✗ Missing prerequisite: %s is not installed\n"+
					"    %s requires %s to be installed first\n"+
					"    Fix: pvm install %s@%d",
				req.Runtime, rt, req.Runtime, req.Runtime, req.MinVersion,
			))
			continue
		}

		currentMajor := semver.Parse(currentVer).Major
		if currentMajor == 0 {
			continue // 无法解析版本，不阻断
		}

		if currentMajor < req.MinVersion {
			errors = append(errors, fmt.Sprintf(
				"  ✗ Incompatible prerequisite: %s@%s is too old\n"+
					"    %s — %s\n"+
					"    Current: %s@%s (major: %d)\n"+
					"    Required: %s@%d or later\n"+
					"    Fix: pvm use %s@%d",
				req.Runtime, currentVer,
				req.Description,
				rt, req.Runtime, currentVer, currentMajor,
				req.Runtime, req.MinVersion,
				req.Runtime, req.MinVersion,
			))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("prerequisite check failed:\n%s", joinErrors(errors))
	}

	return nil
}

// joinErrors 将多个错误信息用换行符连接
func joinErrors(errors []string) string {
	result := ""
	for i, err := range errors {
		result += err
		if i < len(errors)-1 {
			result += "\n\n"
		}
	}
	return result
}
