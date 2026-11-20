package pack

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// goPacker 用 go build 打包。交叉编译置 CGO_ENABLED=0 保证静态产物；
// 当前平台目标保持宿主默认的 CGO 行为。
type goPacker struct {
	opts   Options
	goPath string
}

func (p *goPacker) entry() string {
	if p.opts.Entry == "" {
		return "."
	}
	return p.opts.Entry
}

func (p *goPacker) defaultOutput() string {
	if p.entry() == "." {
		cwd, _ := os.Getwd()
		return filepath.Join("dist", filepath.Base(cwd))
	}
	return defaultOutput(p.entry())
}

func (p *goPacker) targets(ts []Target, _ bool) ([]Target, error) { return ts, nil }

func (p *goPacker) prepare(_ []Target, assets []Asset) (func(), error) {
	if len(assets) > 0 {
		return nil, fmt.Errorf(`pvm pkg go does not support --include: Go embeds files at compile time.
  Use //go:embed in your source instead:
    import _ "embed"
    //go:embed config.yaml
    var configYAML []byte
  (--include is supported for node and python)`)
	}
	goPath, err := resolveCommand("pvm pkg go", "go")
	if err != nil {
		return nil, err
	}
	p.goPath = goPath
	return func() {}, nil
}

func (p *goPacker) build(t Target, out string, idx, total int) error {
	cmd := exec.Command(p.goPath, "build", "-trimpath", "-ldflags", "-s -w", "-o", out, p.entry())
	cmd.Env = os.Environ()
	if !t.IsHost() {
		cmd.Env = append(cmd.Env, "GOOS="+t.OS, "GOARCH="+t.Arch, "CGO_ENABLED=0")
	}
	if total > 1 {
		fmt.Printf("==> [%d/%d] %s\n", idx, total, t)
	}
	var buf bytes.Buffer
	if p.opts.Verbose {
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	} else {
		cmd.Stdout, cmd.Stderr = &buf, &buf
	}
	if err := cmd.Run(); err != nil {
		if !p.opts.Verbose && buf.Len() > 0 {
			return fmt.Errorf("go build %s: %w\n%s", t, err, strings.TrimSpace(buf.String()))
		}
		return fmt.Errorf("go build %s: %w", t, err)
	}
	return nil
}
