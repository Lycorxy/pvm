package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Options 运行选项
type Options struct {
	Dir    string    // 工作目录（pvm.yaml 所在目录）
	Stdout io.Writer // 默认 os.Stdout
	Stderr io.Writer // 默认 os.Stderr
	Stdin  io.Reader // 默认 os.Stdin；并行分支内不接 stdin
}

type runner struct {
	dir    string
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
}

// Run 执行计划。
//
//   - 顺序：同一列表里连续的命令在同一个 shell 会话中运行（cd 保留），失败即停；
//   - 并行：每个分支独立运行，输出带 [名字] 前缀；任一失败则终止其余分支；
//   - ctx 取消（Ctrl+C）时终止全部子进程树。
func Run(ctx context.Context, root *Node, opt Options) error {
	r := &runner{dir: opt.Dir, stdout: opt.Stdout, stderr: opt.Stderr, stdin: opt.Stdin}
	if r.stdout == nil {
		r.stdout = os.Stdout
	}
	if r.stderr == nil {
		r.stderr = os.Stderr
	}
	if r.stdin == nil {
		r.stdin = os.Stdin
	}

	name := root.Title()
	start := time.Now()
	err := r.node(ctx, root, r.stdout, r.stderr, true)
	took := fmtDur(time.Since(start))

	switch {
	case err == nil:
		fmt.Fprintf(r.stdout, "✔ %s  %s\n", name, took)
		return nil
	case ctx.Err() != nil:
		fmt.Fprintf(r.stdout, "✘ %s  interrupted after %s\n", name, took)
		return errors.New("interrupted")
	}
	fmt.Fprintf(r.stdout, "✘ %s  failed after %s\n", name, took)
	return fmt.Errorf("pipeline %q failed: %w", name, err)
}

func (r *runner) node(ctx context.Context, n *Node, out, errw io.Writer, interactive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch n.Kind {
	case Cmd:
		return r.script(ctx, []string{n.Cmd}, out, errw, interactive)
	case Par:
		if n.Label != "" {
			fmt.Fprintf(out, "▶ %s (parallel)\n", n.Label)
		}
		return r.par(ctx, n, out, errw)
	}
	if n.Label != "" {
		fmt.Fprintf(out, "▶ %s\n", n.Label)
	}
	return r.seq(ctx, n, out, errw, interactive)
}

// seq 顺序执行。连续的命令（含被引用的顺序流水线展开后的命令）攒成一批，在同一个 shell 里跑；
// 遇到并行组则先跑完这一批，再执行并行组。
func (r *runner) seq(ctx context.Context, n *Node, out, errw io.Writer, interactive bool) error {
	var batch []string
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		cmds := batch
		batch = nil
		return r.script(ctx, cmds, out, errw, interactive)
	}
	var walk func(items []*Node) error
	walk = func(items []*Node) error {
		for _, it := range items {
			switch it.Kind {
			case Cmd:
				batch = append(batch, it.Cmd)
			case Seq:
				if err := walk(it.Items); err != nil {
					return err
				}
			case Par:
				if err := flush(); err != nil {
					return err
				}
				if err := r.node(ctx, it, out, errw, interactive); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(n.Items); err != nil {
		return err
	}
	return flush()
}

// par 并行执行：全部同时启动，任一分支失败即取消其余分支。
func (r *runner) par(ctx context.Context, n *Node, out, errw io.Writer) error {
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	width := 0
	for _, it := range n.Items {
		if w := len([]rune(it.Title())); w > width {
			width = w
		}
	}

	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)
	for _, it := range n.Items {
		title := it.Title()
		prefix := fmt.Sprintf("[%-*s] ", width, title)
		po := &prefixWriter{w: out, prefix: prefix}
		pe := &prefixWriter{w: errw, prefix: prefix}

		wg.Add(1)
		go func(it *Node) {
			defer wg.Done()
			t0 := time.Now()
			err := r.node(ctx2, it, po, pe, false)
			took := fmtDur(time.Since(t0))
			switch {
			case err == nil:
				fmt.Fprintf(po, "✔ %s\n", took)
			case ctx2.Err() != nil:
				fmt.Fprintf(po, "– stopped\n")
			default:
				fmt.Fprintf(po, "✘ %v  %s\n", err, took)
				once.Do(func() {
					first = fmt.Errorf("%s: %w", title, err)
					cancel()
				})
			}
			po.Flush()
			pe.Flush()
		}(it)
	}
	wg.Wait()

	if first != nil {
		return first
	}
	return ctx.Err()
}

// script 在一个 shell 会话里依次执行 cmds。
func (r *runner) script(ctx context.Context, cmds []string, out, errw io.Writer, interactive bool) error {
	cmd, cleanup, err := newShellCmd(cmds, r.dir)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd.Stdout, cmd.Stderr = out, errw
	if interactive {
		cmd.Stdin = r.stdin
	} else {
		isolate(cmd)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killTree(cmd)
		case <-done:
		}
	}()
	err = cmd.Wait()
	close(done)

	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Errorf("exit %d", ee.ExitCode())
	}
	return err
}

func fmtDur(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	d = d.Round(time.Second)
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// prefixWriter 按行加前缀写出，保证并行输出不会在行中间交错。
type prefixWriter struct {
	mu     sync.Mutex
	w      io.Writer
	prefix string
	buf    []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	used := 0
	for {
		i := bytes.IndexByte(p.buf[used:], '\n')
		if i < 0 {
			break
		}
		p.emit(p.buf[used : used+i+1])
		used += i + 1
	}
	p.buf = append(p.buf[:0], p.buf[used:]...)
	return len(b), nil
}

// Flush 写出末尾没有换行的残留内容
func (p *prefixWriter) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.buf) > 0 {
		p.emit(append(p.buf, '\n'))
		p.buf = p.buf[:0]
	}
}

func (p *prefixWriter) emit(line []byte) {
	var sb strings.Builder
	sb.Grow(len(p.prefix) + len(line))
	sb.WriteString(p.prefix)
	sb.Write(line)
	_, _ = io.WriteString(p.w, sb.String())
}
