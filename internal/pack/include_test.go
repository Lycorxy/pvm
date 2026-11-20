package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(assets []Asset) []string {
	out := make([]string, len(assets))
	for i, a := range assets {
		out[i] = a.Name
	}
	return out
}

func TestParseIncludes(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "env: dev\n")

	dist := filepath.Join(dir, "dist")
	writeFile(t, filepath.Join(dist, "config.json"), "{}")
	writeFile(t, filepath.Join(dist, "assets", "logo.png"), "png")

	// 单文件：包内保持文件名
	got, err := ParseIncludes([]string{cfg})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "config.yaml" || !filepath.IsAbs(got[0].Path) {
		t.Fatalf("single file: %+v", got)
	}

	// 改名
	got, err = ParseIncludes([]string{cfg + ":conf/app.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "conf/app.yaml" {
		t.Fatalf("rename: %+v", got)
	}

	// 目录：保留目录名
	got, err = ParseIncludes([]string{dist})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dist/assets/logo.png", "dist/config.json"}
	if len(got) != len(want) {
		t.Fatalf("dir: %v", names(got))
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("dir: %v, want %v", names(got), want)
		}
	}

	// 尾斜杠：只打内容
	got, err = ParseIncludes([]string{dist + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "assets/logo.png" {
		t.Fatalf("trailing slash: %v", names(got))
	}

	// 目录 + 指定包内前缀
	got, err = ParseIncludes([]string{dist + ":static"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "static/assets/logo.png" {
		t.Fatalf("prefix: %v", names(got))
	}

	// 重名
	if _, err := ParseIncludes([]string{cfg, cfg + ":config.yaml"}); err == nil {
		t.Fatal("duplicate should fail")
	}
	// 不存在
	if _, err := ParseIncludes([]string{filepath.Join(dir, "nope.yaml")}); err == nil {
		t.Fatal("missing source should fail")
	}
}

func TestSplitIncludeSpec(t *testing.T) {
	cases := []struct{ spec, src, dest string }{
		{"config.yaml", "config.yaml", ""},
		{"config.yaml:conf/app.yaml", "config.yaml", "conf/app.yaml"},
		{`C:\proj\config.yaml`, `C:\proj\config.yaml`, ""},
		{`C:\proj\a.yaml:conf/b.yaml`, `C:\proj\a.yaml`, "conf/b.yaml"},
		{"/proj/a.yaml:/etc/b.yaml", "/proj/a.yaml", "/etc/b.yaml"},
	}
	for _, c := range cases {
		src, dest := splitIncludeSpec(c.spec)
		if src != c.src || dest != c.dest {
			t.Errorf("splitIncludeSpec(%q) = (%q,%q), want (%q,%q)", c.spec, src, dest, c.src, c.dest)
		}
	}
}

func TestGoRejectsInclude(t *testing.T) {
	p := &goPacker{}
	if _, err := p.prepare(nil, []Asset{{Name: "a", Path: "a"}}); err == nil {
		t.Fatal("go should reject --include")
	}
}
