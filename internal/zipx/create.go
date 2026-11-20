package zipx

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zip"
)

const (
	// smallFileMax 以内的文件整体读入内存，交给 worker 并行压缩；超过则流式串行压缩
	smallFileMax = 4 << 20
	// memBudgetMB 并行压缩在途数据的内存预算（MB）
	memBudgetMB = 256
)

// reproducibleTime 可复现模式下统一使用的时间戳（zip 能表示的最早日期）
var reproducibleTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// storedExts 已经是压缩格式的扩展名：再压缩只会浪费 CPU，直接存储
var storedExts = map[string]bool{
	".zip": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".7z": true,
	".rar": true, ".zst": true, ".lz4": true, ".br": true, ".jar": true, ".war": true,
	".apk": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".heic": true,
	".mp3": true, ".aac": true, ".ogg": true, ".flac": true,
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
	".woff": true, ".woff2": true,
}

var flatePool = sync.Pool{New: func() any {
	w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression)
	return w
}}

var copyBufPool = sync.Pool{New: func() any {
	b := make([]byte, 256<<10)
	return &b
}}

// Zip 把 srcs 打成 output。
//
// 目录写法与 cp -r / rsync 一致：
//   - "dist"  → 包内为 dist/...（保留目录名）
//   - "dist/" 或 "."  → 只打目录内容，包内无外层目录
func Zip(output string, srcs []string, opt Options) (Stats, error) {
	if len(srcs) == 0 {
		return Stats{}, errors.New("no source given")
	}
	outAbs, err := filepath.Abs(output)
	if err != nil {
		return Stats{}, err
	}
	entries, err := Collect(srcs, opt.Exclude, outAbs)
	if err != nil {
		return Stats{}, err
	}
	return Create(output, entries, opt)
}

// Collect 遍历 srcs 得到有序、无重名的条目列表；skip 为需要跳过的磁盘绝对路径（如输出文件本身）。
func Collect(srcs []string, exclude []string, skip ...string) ([]Entry, error) {
	for _, p := range exclude {
		if _, err := path.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid --exclude pattern %q: %w", p, err)
		}
	}
	skipSet := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipSet[strings.ToLower(filepath.Clean(s))] = true
	}

	var entries []Entry
	owner := make(map[string]string) // 包内名 → 来源，检测重名
	add := func(e Entry, from string) error {
		if prev, dup := owner[e.Name]; dup {
			return fmt.Errorf("duplicate entry %q (from %s and %s)\n"+
				"  hint: use a trailing slash (dist/) to zip a directory's contents without its folder name", e.Name, prev, from)
		}
		owner[e.Name] = from
		entries = append(entries, e)
		return nil
	}

	for _, src := range srcs {
		contentsOnly := src == "." || strings.HasSuffix(src, "/") || strings.HasSuffix(src, `\`)
		abs, err := filepath.Abs(src)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs) // 顶层符号链接按用户意图跟随
		if err != nil {
			return nil, fmt.Errorf("source not found: %s", src)
		}

		if !info.IsDir() {
			if skipSet[strings.ToLower(abs)] {
				continue
			}
			base := filepath.Base(abs)
			if matchAny(exclude, base, base) {
				continue
			}
			if err := add(fileEntry(base, abs, info), abs); err != nil {
				return nil, err
			}
			continue
		}

		rootName := filepath.Base(abs)
		if contentsOnly || rootName == "." || rootName == string(filepath.Separator) {
			rootName = ""
		}
		walkErr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == abs {
				if rootName == "" {
					return nil
				}
				fi, err := d.Info()
				if err != nil {
					return err
				}
				return add(dirEntry(rootName+"/", p, fi), p)
			}
			if skipSet[strings.ToLower(p)] {
				return nil
			}
			rel, err := filepath.Rel(abs, p)
			if err != nil {
				return err
			}
			relSlash := filepath.ToSlash(rel)
			name := relSlash
			if rootName != "" {
				name = rootName + "/" + relSlash
			}
			if matchAny(exclude, d.Name(), relSlash) || matchAny(exclude, d.Name(), name) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			switch {
			case fi.IsDir():
				return add(dirEntry(name+"/", p, fi), p)
			case fi.Mode()&fs.ModeSymlink != 0:
				link, err := os.Readlink(p)
				if err != nil {
					return err
				}
				return add(Entry{Name: name, Path: p, Mode: fi.Mode(), ModTime: fi.ModTime(),
					Link: filepath.ToSlash(link)}, p)
			case fi.Mode().IsRegular():
				return add(fileEntry(name, p, fi), p)
			default:
				return nil // 设备、管道、套接字等不打包
			}
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	if len(entries) == 0 {
		return nil, errors.New("nothing to zip (all sources are empty or excluded)")
	}
	return entries, nil
}

func fileEntry(name, p string, fi fs.FileInfo) Entry {
	return Entry{Name: name, Path: p, Mode: fi.Mode(), ModTime: fi.ModTime(), Size: fi.Size()}
}

func dirEntry(name, p string, fi fs.FileInfo) Entry {
	return Entry{Name: name, Path: p, Mode: fi.Mode(), ModTime: fi.ModTime()}
}

// matchAny 判断 base 或 rel 是否匹配任一 glob
func matchAny(patterns []string, base, rel string) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(p, base); ok {
			return true
		}
		if ok, _ := path.Match(p, rel); ok {
			return true
		}
	}
	return false
}

// Create 把 entries 按顺序写入 output（先写临时文件，成功后原子改名）。
func Create(output string, entries []Entry, opt Options) (Stats, error) {
	start := time.Now()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return Stats{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".pvm-zip-*.tmp")
	if err != nil {
		return Stats{}, err
	}
	tmpName := tmp.Name()
	fail := func(err error) (Stats, error) {
		tmp.Close()
		os.Remove(tmpName)
		return Stats{}, err
	}

	bw := bufio.NewWriterSize(tmp, 1<<20)
	zw := zip.NewWriter(bw)
	stats, err := writeEntries(zw, entries, opt)
	if err != nil {
		return fail(err)
	}
	if err := zw.Close(); err != nil {
		return fail(err)
	}
	if err := bw.Flush(); err != nil {
		return fail(err)
	}
	if err := tmp.Chmod(0o644); err != nil && !errors.Is(err, os.ErrPermission) {
		return fail(err)
	}
	if info, err := tmp.Stat(); err == nil {
		stats.Packed = info.Size()
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return Stats{}, err
	}
	if err := os.Rename(tmpName, output); err != nil {
		os.Remove(tmpName)
		return Stats{}, err
	}
	stats.Elapsed = time.Since(start)
	return stats, nil
}

type job struct {
	e     Entry
	units int
	res   chan result
}

type result struct {
	hdr    *zip.FileHeader
	data   []byte // 已压缩（或原样存储）的内容
	direct bool   // 由写入方自行处理（目录、符号链接、空文件、大文件）
	err    error
}

// budget 用带缓冲 channel 实现的内存预算（每个 token 代表 1MB 在途数据）
type budget chan struct{}

func (b budget) acquire(n int, stop <-chan struct{}) bool {
	for i := 0; i < n; i++ {
		select {
		case b <- struct{}{}:
		case <-stop:
			b.release(i)
			return false
		}
	}
	return true
}

func (b budget) release(n int) {
	for i := 0; i < n; i++ {
		<-b
	}
}

func unitsFor(size int64) int {
	u := int((size + (1 << 20) - 1) >> 20)
	if u < 1 {
		u = 1
	}
	if u > memBudgetMB {
		u = memBudgetMB
	}
	return u
}

// writeEntries 并行压缩小文件、按原顺序写入。
//
//	dispatcher ──jobs──▶ workers（并行压缩，结果进各自 job.res）
//	     └──pending（有序）──▶ 写入方（本 goroutine，按顺序取结果写 zip）
func writeEntries(zw *zip.Writer, entries []Entry, opt Options) (Stats, error) {
	workers := opt.workers(16)
	jobs := make(chan *job, workers*2)
	pending := make(chan *job, workers*4)
	stop := make(chan struct{})
	mem := make(budget, memBudgetMB)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				j.res <- compressSmall(j.e, opt)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(pending)
		defer close(jobs)
		for _, e := range entries {
			j := &job{e: e, res: make(chan result, 1)}
			small := e.Mode.IsRegular() && e.Size > 0 && e.Size <= smallFileMax
			if small {
				j.units = unitsFor(e.Size)
				if !mem.acquire(j.units, stop) {
					return
				}
				select {
				case jobs <- j:
				case <-stop:
					mem.release(j.units)
					return
				}
			} else {
				j.res <- result{direct: true}
			}
			select {
			case pending <- j:
			case <-stop:
				return
			}
		}
	}()

	var files, bytesDone atomic.Int64
	stopProgress := startProgress(opt.Progress, &files, &bytesDone)
	defer stopProgress()

	var stats Stats
	var werr error
	for j := range pending {
		r := <-j.res
		if r.err != nil {
			werr = r.err
		} else {
			werr = writeOne(zw, j.e, r, opt, &stats)
		}
		if j.units > 0 {
			mem.release(j.units)
		}
		if werr != nil {
			break
		}
		files.Add(1)
		bytesDone.Add(j.e.Size)
	}
	if werr != nil {
		close(stop)
	}
	wg.Wait()
	if werr != nil {
		return Stats{}, werr
	}
	return stats, nil
}

// newHeader 构造条目头；可复现模式下统一时间戳与权限。
//
// raw 表示该条目将通过 CreateRaw 写入。CreateRaw 不会处理时间戳
// （CreateHeader 才会），所以这里要自己写 DOS 时间和扩展时间戳（0x5455），
// 否则条目的修改时间会是 DOS 零值（1979-11-30）。
func newHeader(e Entry, opt Options, raw bool) *zip.FileHeader {
	hdr := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
	mode, mt := e.Mode, e.ModTime
	if opt.Reproducible {
		mt = reproducibleTime
		perm := fs.FileMode(0o644)
		if mode.IsDir() || mode.Perm()&0o111 != 0 {
			perm = 0o755
		}
		mode = mode&^fs.ModePerm | perm
	}
	hdr.SetMode(mode)
	switch {
	case mt.IsZero():
	case raw:
		hdr.SetModTime(mt) // Modified（UTC）+ DOS 日期/时间
		var ext [9]byte
		binary.LittleEndian.PutUint16(ext[0:], 0x5455) // extended timestamp
		binary.LittleEndian.PutUint16(ext[2:], 5)
		ext[4] = 1 // flags: mtime present
		binary.LittleEndian.PutUint32(ext[5:], uint32(mt.Unix()))
		hdr.Extra = append(hdr.Extra, ext[:]...)
	default:
		hdr.Modified = mt // CreateHeader 会自行生成 DOS 时间与扩展时间戳
	}
	return hdr
}

// compressSmall 在 worker 中压缩一个小文件
func compressSmall(e Entry, opt Options) result {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return result{err: err}
	}
	hdr := newHeader(e, opt, true)
	hdr.UncompressedSize64 = uint64(len(data))
	hdr.CRC32 = crc32.ChecksumIEEE(data)

	if len(data) < 64 || storedExts[strings.ToLower(path.Ext(e.Name))] {
		hdr.Method = zip.Store
		hdr.CompressedSize64 = uint64(len(data))
		return result{hdr: hdr, data: data}
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(data)/2+64))
	fw := flatePool.Get().(*flate.Writer)
	fw.Reset(buf)
	_, werr := fw.Write(data)
	cerr := fw.Close()
	flatePool.Put(fw)
	if werr != nil || cerr != nil {
		return result{err: errors.Join(werr, cerr)}
	}
	if buf.Len() >= len(data) { // 压缩无收益则存储
		hdr.Method = zip.Store
		hdr.CompressedSize64 = uint64(len(data))
		return result{hdr: hdr, data: data}
	}
	hdr.Method = zip.Deflate
	hdr.CompressedSize64 = uint64(buf.Len())
	return result{hdr: hdr, data: buf.Bytes()}
}

// writeOne 按条目类型写入一个条目
func writeOne(zw *zip.Writer, e Entry, r result, opt Options, st *Stats) error {
	switch {
	case e.Mode.IsDir():
		hdr := newHeader(e, opt, false)
		hdr.Method = zip.Store
		if _, err := zw.CreateHeader(hdr); err != nil {
			return err
		}
		st.Dirs++
		return nil

	case e.Mode&fs.ModeSymlink != 0:
		data := []byte(e.Link)
		hdr := newHeader(e, opt, true)
		hdr.Method = zip.Store
		hdr.CRC32 = crc32.ChecksumIEEE(data)
		hdr.CompressedSize64, hdr.UncompressedSize64 = uint64(len(data)), uint64(len(data))
		w, err := zw.CreateRaw(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		st.Files++
		return err

	case !r.direct: // 小文件：worker 已压缩
		w, err := zw.CreateRaw(r.hdr)
		if err != nil {
			return err
		}
		if _, err := w.Write(r.data); err != nil {
			return err
		}
		st.Files++
		st.Size += int64(r.hdr.UncompressedSize64)
		return nil

	case e.Size == 0: // 空文件
		hdr := newHeader(e, opt, true)
		hdr.Method = zip.Store
		w, err := zw.CreateRaw(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write(nil)
		st.Files++
		return err

	default: // 大文件：流式压缩，不占用大块内存
		return streamFile(zw, e, opt, st)
	}
}

func streamFile(zw *zip.Writer, e Entry, opt Options, st *Stats) error {
	f, err := os.Open(e.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	hdr := newHeader(e, opt, false)
	if storedExts[strings.ToLower(path.Ext(e.Name))] {
		hdr.Method = zip.Store
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	bufp := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufp)
	n, err := io.CopyBuffer(w, struct{ io.Reader }{f}, *bufp)
	if err != nil {
		return fmt.Errorf("compress %s: %w", e.Name, err)
	}
	st.Files++
	st.Size += n
	return nil
}

// startProgress 启动进度回调；返回的函数停止它
func startProgress(fn func(files, bytes int64), files, bytes *atomic.Int64) func() {
	if fn == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				fn(files.Load(), bytes.Load())
			case <-done:
				return
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}
