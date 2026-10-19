package config

import (
	"os"
	"path/filepath"
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
build:
  dev:
    - cd web
    - pnpm i
    - npm run build:dev
  pro:
    - pnpm i
    - npm run build:pro
    - cd dist
    - zip -q -r ./release.zip ./*
pkg:
  go:
    pro:
      - go build -trimpath -ldflags "-s -w" -o dist/app .
shorthand:
  hello: echo hi          # 单命令简写
empty:
  none: []                # 空列表
release:
  pro:
    - "@build:pro"
    - "@pkg:go:pro"
`

func TestLoadPipelinesYAML(t *testing.T) {
	path := writeTestFile(t, "", "pvm.yaml", testPvmYaml)
	pf, err := LoadPipelines(path)
	if err != nil {
		t.Fatal(err)
	}

	// 顶层 name 不进入流水线命名空间
	if pf.Has("name") {
		t.Fatal("top-level name must not become a pipeline")
	}

	// 名字集合（排序后）
	got := pf.Names()
	want := []string{
		"build:dev", "build:pro", "empty:none",
		"pkg:go:pro", "release:pro", "shorthand:hello",
	}
	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// 多步列表
	pro := pf.Commands["build:pro"]
	if len(pro) != 4 || pro[0] != "pnpm i" || pro[3] != "zip -q -r ./release.zip ./*" {
		t.Fatalf("build:pro = %v", pro)
	}

	// 单命令简写
	if h := pf.Commands["shorthand:hello"]; len(h) != 1 || h[0] != "echo hi" {
		t.Fatalf("shorthand = %v", h)
	}

	// 空列表保留为空流水线
	if e := pf.Commands["empty:none"]; len(e) != 0 {
		t.Fatalf("empty = %v", e)
	}

	// 项目名
	if pf.Name != "test-project" {
		t.Fatalf("Name = %q, want %q", pf.Name, "test-project")
	}
}

func TestLoadPipelinesNameErrors(t *testing.T) {
	// name 非字符串
	path := writeTestFile(t, "", "pvm.yaml", "name: 42\nbuild:\n  dev: echo ok\n")
	if _, err := LoadPipelines(path); err == nil {
		t.Fatal("expected error for non-string name")
	}
}

func TestExpandPipelinePreservesBackgroundMarker(t *testing.T) {
	// "&@ref" 不在展开时被吞掉——执行器按 & 后台作业处理
	path := writeTestFile(t, "", "pvm.yaml",
		"dev:\n  web: echo web\n  api: echo api\n  all:\n    - \"&@dev:web\"\n    - \"&@dev:api\"\n")
	pf, err := LoadPipelines(path)
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := pf.ExpandPipeline("dev:all")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 || cmds[0] != "&@dev:web" || cmds[1] != "&@dev:api" {
		t.Fatalf("expanded = %v", cmds)
	}
}

func TestLoadPipelinesYAMLErrors(t *testing.T) {
	// 非字符串步骤
	path := writeTestFile(t, "", "pvm.yaml", "a:\n  b:\n    - echo ok\n    - 123\n")
	if _, err := LoadPipelines(path); err == nil {
		t.Fatal("expected error for non-string step")
	}

	// 非法叶子类型
	path = writeTestFile(t, "", "pvm.yaml", "a:\n  b: 42\n")
	if _, err := LoadPipelines(path); err == nil {
		t.Fatal("expected error for non-command leaf")
	}

	// 名字冲突（引号键 "x:y" 嵌套 与 x.y 路径展开相同）
	path = writeTestFile(t, "", "pvm.yaml",
		"x:\n  y:\n    z: echo one\n\"x:y\":\n  z: echo two\n")
	if _, err := LoadPipelines(path); err == nil {
		t.Fatal("expected duplicate-name error")
	}

	// YAML 语法错误
	path = writeTestFile(t, "", "pvm.yaml", "a: [unclosed\n")
	if _, err := LoadPipelines(path); err == nil {
		t.Fatal("expected yaml syntax error")
	}
}

func TestExpandPipeline(t *testing.T) {
	path := writeTestFile(t, "", "pvm.yaml", testPvmYaml)
	pf, err := LoadPipelines(path)
	if err != nil {
		t.Fatal(err)
	}

	cmds, err := pf.ExpandPipeline("release:pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 5 {
		t.Fatalf("expanded = %v", cmds)
	}
	if cmds[0] != "pnpm i" || cmds[4] != "go build -trimpath -ldflags \"-s -w\" -o dist/app ." {
		t.Fatalf("expanded = %v", cmds)
	}

	// 未定义名字
	if _, err := pf.ExpandPipeline("nope"); err == nil {
		t.Fatal("expected error for undefined pipeline")
	}
}

func TestExpandPipelineCycle(t *testing.T) {
	path := writeTestFile(t, "", "pvm.yaml", "a:\n  x:\n    - echo 1\n    - \"@b:x\"\nb:\n  x:\n    - echo 2\n    - \"@a:x\"\n")
	pf, err := LoadPipelines(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pf.ExpandPipeline("a:x"); err == nil {
		t.Fatal("expected circular reference error")
	}
}

func TestFindPipelineFileUpward(t *testing.T) {
	// monorepo 结构：根 pvm.yaml，子目录向上查找
	root := t.TempDir()
	writeTestFile(t, root, "pvm.yaml", "build:\n  dev: echo root\n")

	sub := filepath.Join(root, "web", "app")
	pf, err := FindPipelineFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	if pf == nil || pf.Path != filepath.Join(root, "pvm.yaml") {
		t.Fatalf("expected root pvm.yaml, got %+v", pf)
	}

	// 子目录自己的 pvm.yaml 就近优先
	writeTestFile(t, filepath.Join(root, "web"), "pvm.yaml", "build:\n  dev: echo web\n")
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
	writeTestFile(t, root, "pvm.yml", "build:\n  dev: echo yml\n")
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
