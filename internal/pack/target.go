package pack

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// Target 一个打包目标（GOOS/GOARCH 命名；各运行时自行映射到官方发行版命名）
type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string { return t.OS + "/" + t.Arch }

// IsHost 判断目标是否当前平台
func (t Target) IsHost() bool { return t == Host() }

// Host 当前平台
func Host() Target { return Target{OS: runtime.GOOS, Arch: runtime.GOARCH} }

// DefaultTargets --type all 的目标矩阵：主流桌面与服务器平台
var DefaultTargets = []Target{
	{"windows", "amd64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// ParseType 解析 --type。
//
//	all                      全部目标（默认）
//	host                     当前平台
//	win | linux | mac        某个系统的常用架构（win=amd64；linux、mac=amd64+arm64）
//	linux-arm64 | mac-amd64  系统-架构，指定单个目标
//	win,linux                逗号分隔多选，自动去重
//
// explicit 表示用户明确限定了目标（非空且非 all）。
func ParseType(s string) (targets []Target, explicit bool, err error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "all" {
		return append([]Target(nil), DefaultTargets...), false, nil
	}

	seen := make(map[Target]bool)
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		ts, err := parseTypeToken(tok)
		if err != nil {
			return nil, false, err
		}
		for _, t := range ts {
			if !seen[t] {
				seen[t] = true
				targets = append(targets, t)
			}
		}
	}
	if len(targets) == 0 {
		return nil, false, fmt.Errorf("invalid --type %q", s)
	}
	return targets, true, nil
}

func parseTypeToken(tok string) ([]Target, error) {
	switch tok {
	case "all":
		return append([]Target(nil), DefaultTargets...), nil
	case "host":
		return []Target{Host()}, nil
	}

	osPart, archPart := tok, ""
	if i := strings.Index(tok, "-"); i > 0 {
		osPart, archPart = tok[:i], tok[i+1:]
	}

	var goos string
	var defaultArchs []string
	switch osPart {
	case "win", "windows":
		goos, defaultArchs = "windows", []string{"amd64"}
	case "linux":
		goos, defaultArchs = "linux", []string{"amd64", "arm64"}
	case "mac", "macos", "darwin":
		goos, defaultArchs = "darwin", []string{"amd64", "arm64"}
	default:
		return nil, fmt.Errorf("invalid --type %q: expected all, host, win, linux, mac (optionally with -amd64/-arm64, comma-separated)", tok)
	}

	archs := defaultArchs
	if archPart != "" {
		switch archPart {
		case "amd64", "x64", "x86_64":
			archs = []string{"amd64"}
		case "arm64", "aarch64":
			archs = []string{"arm64"}
		default:
			return nil, fmt.Errorf("invalid --type %q: unknown arch %q (use amd64 or arm64)", tok, archPart)
		}
	}

	out := make([]Target, len(archs))
	for i, a := range archs {
		out[i] = Target{OS: goos, Arch: a}
	}
	return out, nil
}

// OutputPath 计算目标产物路径：
// 多目标时在扩展名前插入 -<os>-<arch>（dist/app → dist/app-linux-amd64）；
// 最后统一规范 .exe：windows 补齐，其余平台去掉误写的 .exe。
func OutputPath(output string, t Target, multi bool) string {
	out := output
	if multi {
		dir, base := filepath.Split(output)
		if ext := filepath.Ext(base); ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
		out = filepath.Join(dir, base+"-"+t.OS+"-"+t.Arch)
	}
	return normalizeExeSuffix(out, t.OS)
}

func normalizeExeSuffix(output, targetOS string) string {
	ext := strings.ToLower(filepath.Ext(output))
	if targetOS == "windows" {
		if ext != ".exe" {
			return output + ".exe"
		}
		return output
	}
	if ext == ".exe" {
		return strings.TrimSuffix(output, filepath.Ext(output))
	}
	return output
}

// defaultOutput 默认产物路径：dist/<入口文件主干名>
func defaultOutput(entry string) string {
	stem := filepath.Base(entry)
	if ext := filepath.Ext(stem); ext != "" {
		stem = strings.TrimSuffix(stem, ext)
	}
	if stem == "" || stem == "." || stem == string(filepath.Separator) {
		stem = "app"
	}
	return filepath.Join("dist", stem)
}

func joinTargets(targets []Target) string {
	parts := make([]string, len(targets))
	for i, t := range targets {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}
