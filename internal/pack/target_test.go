package pack

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseType(t *testing.T) {
	for _, s := range []string{"", "all", " ALL "} {
		got, explicit, err := ParseType(s)
		if err != nil || explicit || len(got) != len(DefaultTargets) {
			t.Fatalf("%q: got %v explicit=%v err=%v", s, got, explicit, err)
		}
	}

	got, explicit, err := ParseType("HOST")
	if err != nil || !explicit || len(got) != 1 || got[0] != Host() {
		t.Fatalf("host: %v %v %v", got, explicit, err)
	}
	if got[0].OS != runtime.GOOS {
		t.Fatalf("host os = %s", got[0].OS)
	}

	cases := map[string][]Target{
		"win":         {{"windows", "amd64"}},
		"linux":       {{"linux", "amd64"}, {"linux", "arm64"}},
		"mac":         {{"darwin", "amd64"}, {"darwin", "arm64"}},
		"linux-arm64": {{"linux", "arm64"}},
		"mac-amd64":   {{"darwin", "amd64"}},
		"win,linux-x64,win": {
			{"windows", "amd64"}, {"linux", "amd64"},
		},
	}
	for in, want := range cases {
		got, explicit, err := ParseType(in)
		if err != nil || !explicit {
			t.Fatalf("%q: %v explicit=%v", in, err, explicit)
		}
		if len(got) != len(want) {
			t.Fatalf("%q: got %v want %v", in, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%q: got %v want %v", in, got, want)
			}
		}
	}

	for _, bad := range []string{"freebsd", "linux-mips", ",", "linux/amd64"} {
		if _, _, err := ParseType(bad); err == nil {
			t.Errorf("%q should error", bad)
		}
	}
}

func TestOutputPath(t *testing.T) {
	j := filepath.Join
	cases := []struct {
		out   string
		t     Target
		multi bool
		want  string
	}{
		{j("dist", "app"), Target{"windows", "amd64"}, true, j("dist", "app-windows-amd64.exe")},
		{j("dist", "app"), Target{"linux", "amd64"}, true, j("dist", "app-linux-amd64")},
		{j("dist", "app.exe"), Target{"darwin", "arm64"}, true, j("dist", "app-darwin-arm64")},
		{j("dist", "app"), Target{"windows", "amd64"}, false, j("dist", "app.exe")},
		{j("dist", "app.exe"), Target{"linux", "amd64"}, false, j("dist", "app")},
		{j("dist", "app"), Target{"linux", "arm64"}, false, j("dist", "app")},
	}
	for _, c := range cases {
		if got := OutputPath(c.out, c.t, c.multi); got != c.want {
			t.Errorf("OutputPath(%q,%v,%v) = %q, want %q", c.out, c.t, c.multi, got, c.want)
		}
	}
}

func TestOutputsUniquePerTarget(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range DefaultTargets {
		out := OutputPath(filepath.Join("dist", "pvm"), tg, true)
		if seen[out] {
			t.Fatalf("duplicate output %s", out)
		}
		seen[out] = true
		if (tg.OS == "windows") != strings.HasSuffix(out, ".exe") {
			t.Fatalf("bad .exe suffix: %s", out)
		}
	}
}

func TestDefaultOutput(t *testing.T) {
	j := filepath.Join
	for in, want := range map[string]string{
		"src/main.js": j("dist", "main"),
		"app.py":      j("dist", "app"),
		".":           j("dist", "app"),
	} {
		if got := defaultOutput(in); got != want {
			t.Errorf("defaultOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJoinTargets(t *testing.T) {
	if got := joinTargets([]Target{{"windows", "amd64"}, {"linux", "arm64"}}); got != "windows/amd64, linux/arm64" {
		t.Fatalf("got %q", got)
	}
}
