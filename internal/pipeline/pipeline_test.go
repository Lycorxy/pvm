package pipeline

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pvm/pvm/internal/config"
)

func load(t *testing.T, yml string) *config.PipelineFile {
	t.Helper()
	pf, err := config.ParsePipelines("pvm.yaml", []byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	return pf
}

const sample = `
install: pnpm i
lint: pnpm lint
test: pnpm test
check:
  parallel: [lint, test]
release:pro:
  - install
  - check
  - pnpm build
`

func TestCompileExpandsRefs(t *testing.T) {
	root, err := Compile(load(t, sample), "release:pro")
	if err != nil {
		t.Fatal(err)
	}
	if root.Kind != Seq || root.Label != "release:pro" || len(root.Items) != 3 {
		t.Fatalf("root = %+v", root)
	}
	if it := root.Items[0]; it.Kind != Cmd || it.Label != "install" || it.Cmd != "pnpm i" {
		t.Fatalf("item0 = %+v", it)
	}
	if it := root.Items[1]; it.Kind != Par || it.Label != "check" || len(it.Items) != 2 || it.Items[0].Cmd != "pnpm lint" {
		t.Fatalf("item1 = %+v", it)
	}
	if it := root.Items[2]; it.Kind != Cmd || it.Label != "" || it.Cmd != "pnpm build" {
		t.Fatalf("item2 = %+v", it)
	}
}

func TestCompileErrors(t *testing.T) {
	pf := load(t, "a: [b]\nb: [a]\nc: c\n")
	if _, err := Compile(pf, "a"); err == nil || !strings.Contains(err.Error(), "a → b → a") {
		t.Fatalf("want cycle error, got %v", err)
	}
	if _, err := Compile(pf, "c"); err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("self reference should be a cycle, got %v", err)
	}
	if _, err := Compile(pf, "zzz"); err == nil || !strings.Contains(err.Error(), "available: a, b, c") {
		t.Fatalf("want not found with list, got %v", err)
	}
	if errs := Validate(pf); len(errs) != 3 {
		t.Fatalf("Validate = %v", errs)
	}
	if errs := Validate(load(t, sample)); len(errs) != 0 {
		t.Fatalf("Validate(sample) = %v", errs)
	}
}

func TestRender(t *testing.T) {
	root, _ := Compile(load(t, sample), "release:pro")
	var b bytes.Buffer
	root.Render(&b)
	want := "release:pro\n" +
		"├─ install: pnpm i\n" +
		"├─ check  (parallel)\n" +
		"│  ├─ lint: pnpm lint\n" +
		"│  └─ test: pnpm test\n" +
		"└─ pnpm build\n"
	if b.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestSummary(t *testing.T) {
	pf := load(t, sample)
	if got := Summary(pf, "install"); got != "pnpm i" {
		t.Fatal(got)
	}
	if got := Summary(pf, "check"); got != "parallel: lint | test" {
		t.Fatal(got)
	}
	if got := Summary(pf, "release:pro"); got != "install → check → pnpm build" {
		t.Fatal(got)
	}
}

func run(t *testing.T, yml, name string) (string, error) {
	t.Helper()
	root, err := Compile(load(t, yml), name)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Run(context.Background(), root, Options{Dir: t.TempDir(), Stdout: &out, Stderr: &out})
	return out.String(), err
}

func TestRunSequence(t *testing.T) {
	out, err := run(t, "go:\n  - echo one\n  - echo two\n", "go")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "one") || !strings.Contains(out, "two") || !strings.Contains(out, "✔ go") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestRunStopsOnFailure(t *testing.T) {
	out, err := run(t, "go:\n  - exit 3\n  - echo should-not-run\n", "go")
	if err == nil || !strings.Contains(err.Error(), "exit 3") {
		t.Fatalf("want exit 3, got %v\n%s", err, out)
	}
	if strings.Contains(out, "should-not-run") {
		t.Fatalf("step after failure ran:\n%s", out)
	}
}

func TestRunParallelPrefixes(t *testing.T) {
	out, err := run(t, "a: echo AAA\nb: echo BBB\nboth:\n  parallel: [a, b]\n", "both")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"[a] ==> echo AAA", "[b] ==> echo BBB", "[a] ✔", "[b] ✔"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunParallelFailFast(t *testing.T) {
	start := time.Now()
	out, err := run(t, "bad: exit 1\nslow: sleep 30\nboth:\n  parallel: [bad, slow]\n", "both")
	if err == nil || !strings.Contains(err.Error(), "bad: exit 1") {
		t.Fatalf("want bad failure, got %v\n%s", err, out)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Fatalf("slow branch was not stopped (%v)", d)
	}
	if !strings.Contains(out, "[slow] – stopped") {
		t.Fatalf("slow branch should be reported stopped:\n%s", out)
	}
}

func TestRunCancel(t *testing.T) {
	root, _ := Compile(load(t, "s: sleep 30\n"), "s")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	start := time.Now()
	err := Run(ctx, root, Options{Dir: t.TempDir(), Stdout: &out, Stderr: &out})
	if err == nil || err.Error() != "interrupted" {
		t.Fatalf("got %v", err)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Fatalf("not cancelled in time (%v)", d)
	}
}

func TestPrefixWriter(t *testing.T) {
	var b bytes.Buffer
	p := &prefixWriter{w: &b, prefix: "[x] "}
	p.Write([]byte("hel"))
	p.Write([]byte("lo\nwor"))
	p.Write([]byte("ld"))
	p.Flush()
	if b.String() != "[x] hello\n[x] world\n" {
		t.Fatalf("got %q", b.String())
	}
}
