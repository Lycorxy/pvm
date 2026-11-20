package zipx

import (
	stdzip "archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func write(t testing.TB, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// compressible 生成可压缩但非平凡的内容
func compressible(n int, seed int64) string {
	r := rand.New(rand.NewSource(seed))
	words := []string{"alpha", "beta", "gamma", "delta", "pvm", "zip", "stream"}
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(words[r.Intn(len(words))])
		b.WriteByte(' ')
	}
	return b.String()[:n]
}

// treeHash 返回目录内所有文件（相对路径 → 内容哈希）
func treeHash(t testing.TB, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = fmt.Sprintf("%x", sum[:8])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func buildTree(t testing.TB, root string, small int) {
	for i := 0; i < small; i++ {
		write(t, root, fmt.Sprintf("src/d%d/f%d.txt", i%7, i), compressible(100+i*13%3000, int64(i)))
	}
	write(t, root, "empty.txt", "")
	write(t, root, "src/tiny.txt", "hi")
	write(t, root, "assets/logo.png", compressible(5000, 99)) // 已压缩扩展名走 Store
	write(t, root, "big/large.bin", compressible(6<<20, 7))   // 超过 smallFileMax，走流式
	if err := os.MkdirAll(filepath.Join(root, "emptydir", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	src := t.TempDir()
	buildTree(t, src, 300)
	zipPath := filepath.Join(t.TempDir(), "out.zip")

	st, err := Zip(zipPath, []string{src + string(filepath.Separator)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Files < 300 || st.Packed <= 0 {
		t.Fatalf("stats = %+v", st)
	}

	dest := t.TempDir()
	if _, err := Unzip(zipPath, dest, Options{}); err != nil {
		t.Fatal(err)
	}
	if !equalMaps(treeHash(t, src), treeHash(t, dest)) {
		t.Fatal("extracted tree differs from source")
	}
	// 空目录保留
	if info, err := os.Stat(filepath.Join(dest, "emptydir", "inner")); err != nil || !info.IsDir() {
		t.Fatalf("empty dir not preserved: %v", err)
	}
}

func TestDirNameVsContents(t *testing.T) {
	src := t.TempDir()
	write(t, src, "a.txt", "A")
	zipPath := filepath.Join(t.TempDir(), "x.zip")

	// 无尾斜杠：保留目录名
	if _, err := Zip(zipPath, []string{src}, Options{}); err != nil {
		t.Fatal(err)
	}
	names := listNames(t, zipPath)
	base := filepath.Base(src)
	if !contains(names, base+"/") || !contains(names, base+"/a.txt") {
		t.Fatalf("named dir: %v", names)
	}

	// 尾斜杠：只打内容
	if _, err := Zip(zipPath, []string{src + "/"}, Options{}); err != nil {
		t.Fatal(err)
	}
	names = listNames(t, zipPath)
	if len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("contents only: %v", names)
	}
}

func TestExcludeAndSelfSkip(t *testing.T) {
	src := t.TempDir()
	write(t, src, "keep.txt", "k")
	write(t, src, "node_modules/x/index.js", "x")
	write(t, src, "a.map", "m")
	write(t, src, "sub/b.map", "m")
	write(t, src, "sub/ok.txt", "ok")
	zipPath := filepath.Join(src, "self.zip") // 输出放在被打包目录内

	if _, err := Zip(zipPath, []string{src + "/"}, Options{Exclude: []string{"node_modules", "*.map"}}); err != nil {
		t.Fatal(err)
	}
	names := listNames(t, zipPath)
	sort.Strings(names)
	want := []string{"keep.txt", "sub/", "sub/ok.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestReproducible(t *testing.T) {
	src := t.TempDir()
	buildTree(t, src, 200)
	out := t.TempDir()
	var sums [2][32]byte
	for i := range sums {
		p := filepath.Join(out, fmt.Sprintf("r%d.zip", i))
		if _, err := Zip(p, []string{src + "/"}, Options{Reproducible: true, Workers: 1 + i*7}); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(p)
		sums[i] = sha256.Sum256(data)
	}
	if sums[0] != sums[1] {
		t.Fatal("reproducible zips differ (should be byte-identical regardless of worker count)")
	}
	infos, err := List(filepath.Join(out, "r0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range infos {
		if !i.ModTime.Equal(reproducibleTime) {
			t.Fatalf("%s: mtime = %v, want fixed %v", i.Name, i.ModTime, reproducibleTime)
		}
	}
}

func TestDuplicateNames(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write(t, a, "dist/x.txt", "1")
	write(t, b, "dist/y.txt", "2")
	_, err := Zip(filepath.Join(t.TempDir(), "d.zip"), []string{filepath.Join(a, "dist"), filepath.Join(b, "dist")}, Options{})
	if err == nil || !strings.Contains(err.Error(), "duplicate entry") {
		t.Fatalf("expected duplicate entry error, got %v", err)
	}
}

func TestZipSlipRejected(t *testing.T) {
	for _, name := range []string{"../evil.txt", "a/../../evil.txt", "/abs.txt", "C:/win.txt", `..\evil.txt`} {
		var buf bytes.Buffer
		zw := stdzip.NewWriter(&buf)
		w, _ := zw.Create(name)
		w.Write([]byte("x"))
		zw.Close()
		zipPath := filepath.Join(t.TempDir(), "evil.zip")
		os.WriteFile(zipPath, buf.Bytes(), 0o644)

		dest := t.TempDir()
		if _, err := Unzip(zipPath, dest, Options{}); err == nil {
			t.Errorf("entry %q should be rejected", name)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); err == nil {
			t.Errorf("entry %q escaped destination", name)
		}
	}
}

func TestWindowsReservedNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	for _, name := range []string{"aux.txt", "dir/CON", "a:b.txt", "trail."} {
		if err := checkWindowsName(name); err == nil {
			t.Errorf("%q should be rejected on windows", name)
		}
	}
	if err := checkWindowsName("auxiliary/console.txt"); err != nil {
		t.Errorf("normal names rejected: %v", err)
	}
}

func TestGBKNames(t *testing.T) {
	gbk, err := simplifiedchinese.GBK.NewEncoder().String("测试目录/文件.txt")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := stdzip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&stdzip.FileHeader{Name: gbk, NonUTF8: true})
	w.Write([]byte("hello"))
	zw.Close()
	zipPath := filepath.Join(t.TempDir(), "gbk.zip")
	os.WriteFile(zipPath, buf.Bytes(), 0o644)

	dest := t.TempDir()
	if _, err := Unzip(zipPath, dest, Options{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "测试目录", "文件.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("GBK name not decoded: %v %q", err, data)
	}
}

func TestZipBombLimits(t *testing.T) {
	var buf bytes.Buffer
	zw := stdzip.NewWriter(&buf)
	for i := 0; i <= MaxFiles; i++ {
		zw.Create(fmt.Sprintf("f%d", i))
	}
	zw.Close()
	zipPath := filepath.Join(t.TempDir(), "many.zip")
	os.WriteFile(zipPath, buf.Bytes(), 0o644)
	if _, err := Unzip(zipPath, t.TempDir(), Options{}); err == nil || !strings.Contains(err.Error(), "too many files") {
		t.Fatalf("expected too many files error, got %v", err)
	}
}

func TestExecBitAndEntryMode(t *testing.T) {
	src := t.TempDir()
	write(t, src, "tool", "#!/bin/sh\n")
	zipPath := filepath.Join(t.TempDir(), "m.zip")
	e := Entry{Name: "tool", Path: filepath.Join(src, "tool"), Mode: 0o755, Size: 10}
	if _, err := Create(zipPath, []Entry{e}, Options{}); err != nil {
		t.Fatal(err)
	}
	infos, err := List(zipPath)
	if err != nil || len(infos) != 1 || infos[0].Mode.Perm() != 0o755 {
		t.Fatalf("mode lost: %v %+v", err, infos)
	}
}

func TestAtomicOutputOnFailure(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "never.zip")
	_, err := Create(zipPath, []Entry{{Name: "x", Path: filepath.Join(dir, "missing"), Mode: 0o644, Size: 5}}, Options{})
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
	if _, err := os.Stat(zipPath); err == nil {
		t.Fatal("output must not exist after failure")
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".pvm-zip-*"))
	if len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestSymlinkRoundTrip(t *testing.T) {
	src := t.TempDir()
	write(t, src, "real.txt", "data")
	if err := os.Symlink("real.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "l.zip")
	if _, err := Zip(zipPath, []string{src + "/"}, Options{}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	st, err := Unzip(zipPath, dest, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Warnings) != 0 {
		t.Fatalf("warnings: %v", st.Warnings)
	}
	got, err := os.Readlink(filepath.Join(dest, "link.txt"))
	if err != nil || got != "real.txt" {
		t.Fatalf("readlink = %q, %v", got, err)
	}
}

func TestModTimePreserved(t *testing.T) {
	src := t.TempDir()
	write(t, src, "a.txt", compressible(500, 1))
	want := time.Date(2021, 3, 4, 5, 6, 8, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(src, "a.txt"), want, want); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "t.zip")
	if _, err := Zip(zipPath, []string{src + "/"}, Options{}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if _, err := Unzip(zipPath, dest, Options{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if d := info.ModTime().Sub(want); d < -2*time.Second || d > 2*time.Second {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func listNames(t *testing.T, zipPath string) []string {
	t.Helper()
	infos, err := List(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, i := range infos {
		names = append(names, i.Name)
	}
	return names
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---- 性能对比：小文件很多 + 一个大文件 ----

func benchTree(b *testing.B) (src, zipPath string) {
	src = b.TempDir()
	for i := 0; i < 5000; i++ {
		write(b, src, fmt.Sprintf("many/d%d/f%d.txt", i%50, i), compressible(2000+i%4000, int64(i)))
	}
	write(b, src, "big.bin", compressible(64<<20, 1))
	zipPath = filepath.Join(b.TempDir(), "bench.zip")
	if _, err := Zip(zipPath, []string{src + "/"}, Options{}); err != nil {
		b.Fatal(err)
	}
	return src, zipPath
}

func BenchmarkZip(b *testing.B) {
	src, _ := benchTree(b)
	out := filepath.Join(b.TempDir(), "o.zip")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Zip(out, []string{src + "/"}, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnzipParallel(b *testing.B) {
	_, zipPath := benchTree(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Unzip(zipPath, b.TempDir(), Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnzipSerial(b *testing.B) {
	_, zipPath := benchTree(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Unzip(zipPath, b.TempDir(), Options{Workers: 1}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnzipStdlibLegacy 复刻旧实现（串行、标准库）作为基线
func BenchmarkUnzipStdlibLegacy(b *testing.B) {
	_, zipPath := benchTree(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest := b.TempDir()
		r, err := stdzip.OpenReader(zipPath)
		if err != nil {
			b.Fatal(err)
		}
		for _, f := range r.File {
			target := filepath.Join(dest, f.Name)
			if f.FileInfo().IsDir() {
				os.MkdirAll(target, 0o755)
				continue
			}
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
			if err != nil {
				b.Fatal(err)
			}
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			out.Write(buf.Bytes())
			rc.Close()
			out.Close()
		}
		r.Close()
	}
}
