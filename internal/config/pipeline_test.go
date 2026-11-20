package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const testPvmYaml = `# 流水线
name: test-project

install: pnpm i
lint: pnpm lint
test: pnpm test

check:                    # 并行
  parallel:
    - lint
    - test

release:pro:              # 顺序，里面嵌套并行
  - install
  - parallel:
      - pnpm lint
      - pnpm test
  - pnpm build --mode pro
  - pvm zip dist --output app.zip
`

func TestParsePipelines(t *testing.T) {
	pf, err := LoadPipelines(writeTestFile(t, "", "pvm.yaml", testPvmYaml))
	if err != nil {
		t.Fatal(err)
	}
	if pf.Name != "test-project" {
		t.Fatalf("Name = %q", pf.Name)
	}
	if pf.Has("name") {
		t.Fatal("top-level name must not become a pipeline")
	}

	// 声明顺序，而不是字母序
	want := []string{"install", "lint", "test", "check", "release:pro"}
	got := pf.Names()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v, want %v", got, want)
	}

	if s := pf.Steps["install"]; s.Kind != StepCmd || s.Cmd != "pnpm i" {
		t.Fatalf("install = %+v", s)
	}
	if s := pf.Steps["check"]; s.Kind != StepPar || len(s.Items) != 2 || s.Items[0].Cmd != "lint" {
		t.Fatalf("check = %+v", s)
	}
	rel := pf.Steps["release:pro"]
	if rel.Kind != StepSeq || len(rel.Items) != 4 {
		t.Fatalf("release:pro = %+v", rel)
	}
	if rel.Items[1].Kind != StepPar || len(rel.Items[1].Items) != 2 {
		t.Fatalf("nested parallel = %+v", rel.Items[1])
	}
	if rel.Items[2].Cmd != "pnpm build --mode pro" {
		t.Fatalf("step 3 = %+v", rel.Items[2])
	}
}

func TestParsePipelinesAliasAndBlockScalar(t *testing.T) {
	pf, err := ParsePipelines("pvm.yaml", []byte(`
common: &common
  - pnpm i
  - pnpm build
web: *common
script: |
  cd web
  pnpm i
`))
	if err != nil {
		t.Fatal(err)
	}
	if s := pf.Steps["web"]; s.Kind != StepSeq || len(s.Items) != 2 {
		t.Fatalf("alias not resolved: %+v", s)
	}
	if s := pf.Steps["script"]; s.Kind != StepCmd || !strings.Contains(s.Cmd, "\n") {
		t.Fatalf("block scalar = %+v", s)
	}
}

func TestParsePipelinesEmptyFile(t *testing.T) {
	pf, err := ParsePipelines("pvm.yaml", nil)
	if err != nil || len(pf.Names()) != 0 {
		t.Fatalf("empty file: %v %v", pf, err)
	}
}

func TestParsePipelinesErrors(t *testing.T) {
	cases := []struct {
		name, yaml, want string
	}{
		{"name not string", "name: 42\nbuild: echo ok\n", "name"},
		{"non-command scalar", "a: 123\n", "expected a command"},
		{"null step", "a:\n", "expected a command"},
		{"non-command in list", "a:\n  - echo ok\n  - true\n", "expected a command"},
		{"unknown key", "a:\n  desc: hello\n  run: x\n", `unknown key "desc"`},
		{"parallel needs list", "a:\n  parallel: pnpm lint\n", "must be followed by a list"},
		{"parallel with sibling key", "a:\n  parallel: [x]\n  env: y\n", `unknown key "env"`},
		{"empty list", "a: []\n", "empty list"},
		{"empty parallel", "a:\n  parallel: []\n", "empty list"},
		{"duplicate pipeline", "a: x\na: y\n", "duplicate"},
		{"top level not mapping", "- a\n- b\n", "top level"},
		{"syntax error", "a: [unclosed\n", "parse"},
	}
	for _, c := range cases {
		_, err := ParsePipelines("pvm.yaml", []byte(c.yaml))
		if err == nil {
			t.Errorf("%s: expected error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not contain %q", c.name, err, c.want)
		}
	}
}

func TestParsePipelinesErrorHasLine(t *testing.T) {
	_, err := ParsePipelines("pvm.yaml", []byte("ok: echo\nbad:\n  - echo\n  - nope: 1\n"))
	if err == nil || !strings.Contains(err.Error(), "pvm.yaml:2") || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("error should locate pipeline and line, got %v", err)
	}
}

func TestFindPipelineFileUpward(t *testing.T) {
	// monorepo 结构：根 pvm.yaml，子目录向上查找
	root := t.TempDir()
	writeTestFile(t, root, "pvm.yaml", "build: echo root\n")

	sub := filepath.Join(root, "web", "app")
	pf, err := FindPipelineFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	if pf == nil || pf.Path != filepath.Join(root, "pvm.yaml") {
		t.Fatalf("expected root pvm.yaml, got %+v", pf)
	}

	// 子目录自己的 pvm.yaml 就近优先
	writeTestFile(t, filepath.Join(root, "web"), "pvm.yaml", "build: echo web\n")
	pf, err = FindPipelineFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	if pf == nil || pf.Path != filepath.Join(root, "web", "pvm.yaml") {
		t.Fatalf("expected nearest pvm.yaml, got %+v", pf)
	}
}

func TestFindPipelineFileYmlFallback(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "pvm.yml", "build: echo yml\n")
	pf, err := FindPipelineFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if pf == nil {
		t.Fatal("expected pvm.yml to be found")
	}
}

func TestFindPipelineFileMissing(t *testing.T) {
	pf, err := FindPipelineFile(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if pf != nil {
		t.Fatalf("expected nil, got %v", pf.Path)
	}
}
