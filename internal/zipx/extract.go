package zipx

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zip"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type extractTask struct {
	f      *zip.File
	target string
}

type linkTask struct {
	f      *zip.File
	target string
}

// Unzip 把 archive 解压到 dest。
//
// 先顺序扫描并校验全部条目、一次性建好目录，再由 worker 池并行写盘；
// 符号链接最后创建，保证不会有文件经由链接写出目标目录之外。
func Unzip(archive, dest string, opt Options) (Stats, error) {
	start := time.Now()
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return Stats{}, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	if len(zr.File) > MaxFiles {
		return Stats{}, fmt.Errorf("archive contains too many files (%d > %d), possible zip bomb", len(zr.File), MaxFiles)
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return Stats{}, err
	}

	var (
		stats    Stats
		dirs     = make(map[string]struct{})
		tasks    []extractTask
		links    []linkTask
		index    = make(map[string]int) // 目标路径（大小写不敏感文件系统归一化）→ tasks 下标，重名以后者为准
		declared int64
	)
	fold := runtime.GOOS == "windows" || runtime.GOOS == "darwin"

	for _, f := range zr.File {
		target, err := safeJoin(destAbs, decodeName(f))
		if err != nil {
			return Stats{}, err
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			dirs[target] = struct{}{}
			stats.Dirs++
		case mode&fs.ModeSymlink != 0:
			links = append(links, linkTask{f, target})
			dirs[filepath.Dir(target)] = struct{}{}
		case mode.IsRegular():
			if int64(f.UncompressedSize64) > MaxFileSize {
				return Stats{}, fmt.Errorf("file %s exceeds max size (%s > %s)", f.Name,
					formatSize(int64(f.UncompressedSize64)), formatSize(MaxFileSize))
			}
			declared += int64(f.UncompressedSize64)
			if declared > MaxTotalSize {
				return Stats{}, fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb", formatSize(MaxTotalSize))
			}
			dirs[filepath.Dir(target)] = struct{}{}
			key := target
			if fold {
				key = strings.ToLower(key)
			}
			if i, dup := index[key]; dup {
				tasks[i] = extractTask{f, target}
			} else {
				index[key] = len(tasks)
				tasks = append(tasks, extractTask{f, target})
			}
		default:
			stats.Warnings = append(stats.Warnings, "skipped special file: "+f.Name)
		}
	}

	for d := range dirs {
		if err := os.MkdirAll(longPath(d), 0o755); err != nil {
			return Stats{}, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	var (
		written, filesDone atomic.Int64
		firstErr           error
		errOnce            sync.Once
		failed             atomic.Bool
	)
	stopProgress := startProgress(opt.Progress, &filesDone, &written)
	defer stopProgress()

	ch := make(chan extractTask, len(tasks))
	for _, t := range tasks {
		ch <- t
	}
	close(ch)

	var wg sync.WaitGroup
	for i := opt.workers(16); i > 0; i-- {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range ch {
				if failed.Load() {
					continue
				}
				n, err := extractFile(t)
				if err == nil && written.Add(n) > MaxTotalSize {
					err = fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb", formatSize(MaxTotalSize))
				}
				if err != nil {
					errOnce.Do(func() { firstErr = err })
					failed.Store(true)
					continue
				}
				filesDone.Add(1)
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return Stats{}, firstErr
	}

	for _, l := range links {
		if w := makeSymlink(destAbs, l); w != "" {
			stats.Warnings = append(stats.Warnings, w)
		}
	}

	stats.Files = int(filesDone.Load())
	stats.Size = written.Load()
	stats.Elapsed = time.Since(start)
	return stats, nil
}

// extractFile 写出一个普通文件，返回写入字节数
func extractFile(t extractTask) (int64, error) {
	rc, err := t.f.Open()
	if err != nil {
		return 0, fmt.Errorf("open zip entry %s: %w", t.f.Name, err)
	}
	defer rc.Close()

	perm := t.f.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}
	path := longPath(t.target)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		// 已存在的只读文件：删除后重试一次
		_ = os.Remove(path)
		if out, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm); err != nil {
			return 0, fmt.Errorf("create file: %w", err)
		}
	}

	bufp := copyBufPool.Get().(*[]byte)
	n, err := io.CopyBuffer(struct{ io.Writer }{out}, io.LimitReader(rc, MaxFileSize+1), *bufp)
	copyBufPool.Put(bufp)
	if err == nil && !t.f.Modified.IsZero() {
		_ = setModTime(out, path, t.f.Modified)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxFileSize {
		err = fmt.Errorf("file %s exceeds max size during extraction", t.f.Name)
	}
	if err != nil {
		os.Remove(path)
		return 0, fmt.Errorf("extract %s: %w", t.f.Name, err)
	}
	return n, nil
}

// makeSymlink 创建符号链接；不安全或失败时返回警告文本（不中断解压）
func makeSymlink(destAbs string, l linkTask) string {
	rc, err := l.f.Open()
	if err != nil {
		return fmt.Sprintf("skipped symlink %s: %v", l.f.Name, err)
	}
	data, err := io.ReadAll(io.LimitReader(rc, 4096))
	rc.Close()
	if err != nil {
		return fmt.Sprintf("skipped symlink %s: %v", l.f.Name, err)
	}
	link := strings.ReplaceAll(string(data), `\`, "/")
	if link == "" || strings.HasPrefix(link, "/") || (len(link) >= 2 && link[1] == ':') {
		return fmt.Sprintf("skipped symlink %s: absolute target %q", l.f.Name, link)
	}
	resolved := filepath.Join(filepath.Dir(l.target), filepath.FromSlash(link))
	rel, err := filepath.Rel(destAbs, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Sprintf("skipped symlink %s: target %q escapes the destination", l.f.Name, link)
	}
	_ = os.Remove(l.target)
	if err := os.Symlink(filepath.FromSlash(link), l.target); err != nil {
		return fmt.Sprintf("symlink %s -> %s not created: %v", l.f.Name, link, err)
	}
	return ""
}

// List 返回压缩包内的条目（名称已做 GBK 转码）
func List(archive string) ([]Info, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	out := make([]Info, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, Info{
			Name:    decodeName(f),
			Size:    int64(f.UncompressedSize64),
			Packed:  int64(f.CompressedSize64),
			Mode:    f.Mode(),
			ModTime: f.Modified,
		})
	}
	return out, nil
}

// decodeName 处理非 UTF-8 文件名：中文 Windows 压缩工具常写入 GBK 而不置 UTF-8 标志
func decodeName(f *zip.File) string {
	if utf8.ValidString(f.Name) {
		return f.Name
	}
	if s, err := simplifiedchinese.GBK.NewDecoder().String(f.Name); err == nil {
		return s
	}
	return f.Name
}

// FormatSize 人类可读的字节数
func FormatSize(b int64) string { return formatSize(b) }

func formatSize(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
