package cmd

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePackTargets(t *testing.T) {
	// 未指定 → 默认跨平台矩阵
	got, err := resolvePackTargets("")
	if err != nil {
		t.Fatalf("empty platform: %v", err)
	}
	if len(got) != len(defaultPackTargets) {
		t.Fatalf("empty platform: got %d targets, want %d", len(got), len(defaultPackTargets))
	}
	for i := range got {
		if got[i] != defaultPackTargets[i] {
			t.Fatalf("empty platform: got %v, want %v", got, defaultPackTargets)
		}
	}

	// all → 同默认矩阵
	got, err = resolvePackTargets("all")
	if err != nil || len(got) != len(defaultPackTargets) {
		t.Fatalf("all: got %v, err=%v", got, err)
	}

	// host → 当前平台
	got, err = resolvePackTargets("HOST")
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	if len(got) != 1 || got[0].OS != runtime.GOOS || got[0].Arch != runtime.GOARCH {
		t.Fatalf("host: got %v", got)
	}

	// 单目标
	got, err = resolvePackTargets("linux/amd64")
	if err != nil || len(got) != 1 || got[0].OS != "linux" || got[0].Arch != "amd64" {
		t.Fatalf("single: got %v, err=%v", got, err)
	}

	// 逗号分隔多目标 + 去重
	got, err = resolvePackTargets("windows/amd64, linux/amd64,windows/amd64")
	if err != nil {
		t.Fatalf("multi: %v", err)
	}
	if len(got) != 2 || got[0].OS != "windows" || got[1].OS != "linux" {
		t.Fatalf("multi dedupe: got %v", got)
	}

	// 非法输入
	for _, bad := range []string{"linux", "linux/", "/amd64", "host,linux"} {
		if _, err := resolvePackTargets(bad); err == nil {
			t.Fatalf("invalid %q should error", bad)
		}
	}
}

func TestTargetOutput(t *testing.T) {
	join := filepath.Join

	// 多目标：插入平台后缀；windows 补 .exe，非 windows 去掉 .exe
	if got := targetOutput(join("dist", "app"), packTarget{"windows", "amd64"}, true); got != join("dist", "app-windows-amd64.exe") {
		t.Errorf("multi windows: %q", got)
	}
	if got := targetOutput(join("dist", "app"), packTarget{"linux", "amd64"}, true); got != join("dist", "app-linux-amd64") {
		t.Errorf("multi linux: %q", got)
	}
	if got := targetOutput(join("dist", "app.exe"), packTarget{"darwin", "arm64"}, true); got != join("dist", "app-darwin-arm64") {
		t.Errorf("multi darwin from .exe output: %q", got)
	}

	// 单目标：保持原名，仅规范 .exe
	if got := targetOutput(join("dist", "app"), packTarget{"windows", "amd64"}, false); got != join("dist", "app.exe") {
		t.Errorf("single windows: %q", got)
	}
	if got := targetOutput(join("dist", "app.exe"), packTarget{"linux", "amd64"}, false); got != join("dist", "app") {
		t.Errorf("single linux strips .exe: %q", got)
	}
	if got := targetOutput(join("dist", "app"), packTarget{"linux", "arm64"}, false); got != join("dist", "app") {
		t.Errorf("single linux: %q", got)
	}
}

func TestJoinTargets(t *testing.T) {
	got := joinTargets([]packTarget{{"windows", "amd64"}, {"linux", "arm64"}})
	if got != "windows/amd64, linux/arm64" {
		t.Fatalf("got %q", got)
	}
}

func TestTakeFlagBool(t *testing.T) {
	rest, ok := takeFlagBool([]string{"--entry", "main.js", "--official"}, "--official")
	if !ok || len(rest) != 2 || rest[0] != "--entry" || rest[1] != "main.js" {
		t.Fatalf("got rest=%v ok=%v", rest, ok)
	}
	if _, ok := takeFlagBool([]string{"--entry", "x"}, "--official"); ok {
		t.Fatal("flag absent but reported")
	}
}

// 验证产物的平台命名与各目标一一对应、互不覆盖
func TestPlatformOutputsUnique(t *testing.T) {
	targets, err := resolvePackTargets("")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, t2 := range targets {
		out := targetOutput(filepath.Join("dist", "pvm"), t2, true)
		if seen[out] {
			t.Fatalf("duplicate output %s", out)
		}
		seen[out] = true
		if t2.OS == "windows" && !strings.HasSuffix(out, ".exe") {
			t.Fatalf("windows output missing .exe: %s", out)
		}
		if t2.OS != "windows" && strings.HasSuffix(out, ".exe") {
			t.Fatalf("non-windows output has .exe: %s", out)
		}
	}
}
