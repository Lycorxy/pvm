// Package zipx 提供稳定、高性能的 zip 压缩与解压。
//
// 底层使用 github.com/klauspost/compress/zip（标准库 archive/zip 的高性能分支，
// 已是 pvm 的依赖），在其上实现：
//   - 压缩：worker 并行压缩小文件，按目录遍历顺序有序写入（CreateRaw）；
//     大文件流式压缩，内存占用有上限；已压缩格式（图片/视频/压缩包）直接存储；
//     Reproducible 模式下同样输入得到逐字节相同的产物。
//   - 解压：worker 池并行写盘，目录一次性建好，复制缓冲复用；
//     防 zip-slip、绝对路径、Windows 保留名；限制单文件/总量/文件数防压缩炸弹；
//     非 UTF-8（常见为 GBK）文件名自动转码。
//   - 写入先落临时文件，成功后原子改名，失败不留半成品。
package zipx

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// MaxFileSize 解压时单个文件的上限（2 GB）
	MaxFileSize int64 = 2 * 1024 * 1024 * 1024
	// MaxTotalSize 解压时总量上限（10 GB）
	MaxTotalSize int64 = 10 * 1024 * 1024 * 1024
	// MaxFiles 解压时压缩包内的文件数上限
	MaxFiles = 100000
)

// Options 压缩与解压共用的选项
type Options struct {
	Exclude      []string // 压缩：排除的 glob（匹配文件名或相对路径，如 node_modules、*.map、dist/*.log）
	Reproducible bool     // 压缩：固定时间戳与权限，同样输入得到同样字节
	Workers      int      // 并发数；0 表示按 CPU 自动
	// Progress 若非空，运行期间约每 500ms 回调一次（已处理文件数、已处理字节数）
	Progress func(files, bytes int64)
}

// Stats 一次操作的统计
type Stats struct {
	Files    int           // 文件数
	Dirs     int           // 目录数
	Size     int64         // 未压缩总字节
	Packed   int64         // 压缩包大小（压缩时有效）
	Elapsed  time.Duration // 耗时
	Warnings []string      // 非致命问题（如无权限创建符号链接）
}

// Entry 描述压缩包内的一个条目
type Entry struct {
	Name    string      // 包内路径，正斜杠；目录以 / 结尾
	Path    string      // 磁盘路径（读取文件内容用）
	Mode    fs.FileMode // 含类型位：目录、符号链接、普通文件
	ModTime time.Time
	Size    int64  // 普通文件大小
	Link    string // 符号链接目标
}

// Info 是 List 返回的包内条目信息
type Info struct {
	Name    string
	Size    int64
	Packed  int64
	Mode    fs.FileMode
	ModTime time.Time
}

func (i Info) IsDir() bool { return i.Mode.IsDir() }

func (o Options) workers(max int) int {
	n := o.Workers
	if n <= 0 {
		n = runtime.NumCPU()
	}
	if n < 1 {
		n = 1
	}
	if n > max {
		n = max
	}
	return n
}

// safeJoin 把包内路径安全地映射到 destAbs 之下。
// 拒绝：绝对路径、盘符、父目录穿越、Windows 保留名与非法字符。
// 返回 destAbs 本身表示该条目是根目录（"." 或 "./"）。
func safeJoin(destAbs, name string) (string, error) {
	n := strings.ReplaceAll(name, `\`, "/") // 部分 Windows 工具会写反斜杠
	if strings.HasPrefix(n, "/") || (len(n) >= 2 && n[1] == ':') {
		return "", fmt.Errorf("unsafe path in archive (absolute): %s", name)
	}
	clean := path.Clean(n)
	if clean == "." {
		return destAbs, nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe path in archive (path traversal): %s", name)
	}
	if runtime.GOOS == "windows" {
		if err := checkWindowsName(clean); err != nil {
			return "", fmt.Errorf("unsafe path in archive: %s: %w", name, err)
		}
	}
	target := filepath.Join(destAbs, filepath.FromSlash(clean))
	rel, err := filepath.Rel(destAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive (path traversal): %s", name)
	}
	return target, nil
}

var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// checkWindowsName 检查每个路径分量在 Windows 上是否可创建
func checkWindowsName(clean string) error {
	for _, comp := range strings.Split(clean, "/") {
		if comp == "" {
			continue
		}
		if strings.ContainsAny(comp, `<>:"|?*`) {
			return fmt.Errorf("component %q contains characters not allowed on Windows", comp)
		}
		for _, r := range comp {
			if r < 0x20 {
				return fmt.Errorf("component %q contains control characters", comp)
			}
		}
		if strings.HasSuffix(comp, ".") || strings.HasSuffix(comp, " ") {
			return fmt.Errorf("component %q ends with a dot or space", comp)
		}
		stem := comp
		if i := strings.IndexByte(stem, '.'); i >= 0 {
			stem = stem[:i]
		}
		if windowsReserved[strings.ToLower(stem)] {
			return fmt.Errorf("component %q is a reserved device name on Windows", comp)
		}
	}
	return nil
}

// longPath 为 Windows 绝对路径添加 \\?\ 前缀以支持超过 260 字符的长路径
func longPath(p string) string {
	if runtime.GOOS != "windows" || strings.HasPrefix(p, `\\?\`) || !filepath.IsAbs(p) {
		return p
	}
	if strings.HasPrefix(p, `\\`) { // UNC：\\server\share → \\?\UNC\server\share
		return `\\?\UNC\` + strings.TrimPrefix(filepath.Clean(p), `\\`)
	}
	return `\\?\` + filepath.Clean(p)
}
