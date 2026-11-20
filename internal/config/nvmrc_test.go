package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadNvmrc(t *testing.T) {
	dir := t.TempDir()

	writeTmp(t, dir, ".nvmrc", "20.11.0\n")
	vf, err := FindVersionFile(dir)
	if err != nil || vf == nil {
		t.Fatalf("err=%v vf=%v", err, vf)
	}
	if vf.Versions["node"] != "20.11.0" {
		t.Fatalf("got %+v", vf.Versions)
	}

	writeTmp(t, dir, ".nvmrc", "# comment\n\nv18.20.4\n")
	vf, _ = FindVersionFile(dir)
	if vf.Versions["node"] != "18.20.4" {
		t.Fatalf("v-prefix: %+v", vf.Versions)
	}

	// lts/* 需要联网解析，忽略后回退到用户/系统版本
	writeTmp(t, dir, ".nvmrc", "lts/*\n")
	vf, _ = FindVersionFile(dir)
	if len(vf.Versions) != 0 {
		t.Fatalf("lts should be ignored: %+v", vf.Versions)
	}
}

func TestPvmrcWinsOverNvmrc(t *testing.T) {
	dir := t.TempDir()
	writeTmp(t, dir, ".nvmrc", "18.0.0\n")
	writeTmp(t, dir, ".pvmrc", "node 20.11.0\n")

	vf, err := FindVersionFile(dir)
	if err != nil || vf == nil {
		t.Fatal(err)
	}
	if filepath.Base(vf.Path) != ".pvmrc" || vf.Versions["node"] != "20.11.0" {
		t.Fatalf("got path=%s versions=%v", vf.Path, vf.Versions)
	}
}

func TestWriteNeverTouchesNvmrc(t *testing.T) {
	dir := t.TempDir()
	nvmrc := writeTmp(t, dir, ".nvmrc", "18.0.0\n")

	path, err := WriteProjectVersion(dir, "node", "20.11.0")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != ".pvmrc" {
		t.Fatalf("wrote to %s", path)
	}
	data, _ := os.ReadFile(nvmrc)
	if string(data) != "18.0.0\n" {
		t.Fatalf(".nvmrc was modified: %q", data)
	}
}
