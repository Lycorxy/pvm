// Package pipeline 把 pvm.yaml 里的流水线编译成执行计划并运行。
//
// 规则只有四条（见 config.ParsePipelines）：
//
//	名字: 命令                  # 一条命令
//	名字: [步骤, 步骤]          # 顺序执行，失败即停
//	名字: {parallel: [步骤…]}   # 同时执行，任一失败则停止其余
//	步骤等于某条流水线的名字     # 就是引用，否则当命令执行
package pipeline

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pvm/pvm/internal/config"
)

// Kind 节点类型
type Kind int

const (
	Cmd Kind = iota // 一条 shell 命令
	Seq             // 顺序
	Par             // 并行
)

// Node 是编译后的执行计划节点（引用已展开）。
type Node struct {
	Kind  Kind
	Label string  // 来自流水线名（引用或顶层）；匿名节点为空
	Cmd   string  // Kind == Cmd
	Items []*Node // Kind == Seq / Par
}

// Compile 把名为 name 的流水线编译成计划树：展开引用，检测循环。
func Compile(pf *config.PipelineFile, name string) (*Node, error) {
	if !pf.Has(name) {
		return nil, notFound(pf, name)
	}
	c := &compiler{pf: pf}
	return c.ref(name)
}

// Validate 编译全部流水线，返回所有问题（引用循环等）。
func Validate(pf *config.PipelineFile) []error {
	var errs []error
	for _, name := range pf.Names() {
		if _, err := Compile(pf, name); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func notFound(pf *config.PipelineFile, name string) error {
	names := pf.Names()
	if len(names) == 0 {
		return fmt.Errorf("pipeline %q not found: %s declares no pipelines", name, pf.Path)
	}
	return fmt.Errorf("pipeline %q not found in %s\n  available: %s",
		name, pf.Path, strings.Join(sortedCopy(names), ", "))
}

func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

type compiler struct {
	pf    *config.PipelineFile
	stack []string
}

func (c *compiler) ref(name string) (*Node, error) {
	for i, s := range c.stack {
		if s == name {
			cycle := append(append([]string(nil), c.stack[i:]...), name)
			return nil, fmt.Errorf("circular reference: %s", strings.Join(cycle, " → "))
		}
	}
	c.stack = append(c.stack, name)
	defer func() { c.stack = c.stack[:len(c.stack)-1] }()

	n, err := c.step(c.pf.Steps[name])
	if err != nil {
		return nil, err
	}
	n.Label = name
	return n, nil
}

func (c *compiler) step(s *config.Step) (*Node, error) {
	switch s.Kind {
	case config.StepCmd:
		if c.pf.Has(s.Cmd) {
			return c.ref(s.Cmd)
		}
		return &Node{Kind: Cmd, Cmd: s.Cmd}, nil
	case config.StepSeq, config.StepPar:
		kind := Seq
		if s.Kind == config.StepPar {
			kind = Par
		}
		n := &Node{Kind: kind}
		for _, it := range s.Items {
			child, err := c.step(it)
			if err != nil {
				return nil, err
			}
			n.Items = append(n.Items, child)
		}
		return n, nil
	}
	return nil, fmt.Errorf("line %d: unknown step kind", s.Line)
}

// ---------------------------------------------------------------- 展示

// Title 节点在树和日志里的短名字
func (n *Node) Title() string {
	switch {
	case n.Kind == Cmd && n.Label != "":
		return n.Label
	case n.Kind == Cmd:
		return oneLine(n.Cmd, 32)
	case n.Label != "":
		return n.Label
	case n.Kind == Par:
		return "parallel"
	}
	return "steps"
}

// Render 以树形打印执行计划（--dry-run）。
//
//	release:pro
//	├─ install: pnpm i
//	├─ parallel
//	│  ├─ lint: pnpm lint
//	│  └─ test: pnpm test
//	└─ pnpm build
func (n *Node) Render(w io.Writer) {
	fmt.Fprintln(w, n.line())
	n.renderChildren(w, "")
}

func (n *Node) line() string {
	switch n.Kind {
	case Cmd:
		if n.Label != "" {
			return n.Label + ": " + oneLine(n.Cmd, 80)
		}
		return oneLine(n.Cmd, 80)
	case Par:
		if n.Label != "" {
			return n.Label + "  (parallel)"
		}
		return "parallel"
	}
	if n.Label != "" {
		return n.Label
	}
	return "steps"
}

func (n *Node) renderChildren(w io.Writer, prefix string) {
	for i, it := range n.Items {
		branch, next := "├─ ", "│  "
		if i == len(n.Items)-1 {
			branch, next = "└─ ", "   "
		}
		fmt.Fprintln(w, prefix+branch+it.line())
		it.renderChildren(w, prefix+next)
	}
}

// Summary 返回某条流水线的一行概要（用于 pvm run --list）：
//
//	pnpm build
//	install → lint → pnpm build
//	parallel: lint | test
func Summary(pf *config.PipelineFile, name string) string {
	return summarize(pf.Steps[name], true)
}

func summarize(s *config.Step, top bool) string {
	switch s.Kind {
	case config.StepCmd:
		return oneLine(s.Cmd, 48)
	case config.StepSeq:
		parts := make([]string, len(s.Items))
		for i, it := range s.Items {
			parts[i] = summarize(it, false)
		}
		return oneLine(strings.Join(parts, " → "), 72)
	}
	parts := make([]string, len(s.Items))
	for i, it := range s.Items {
		parts[i] = summarize(it, false)
	}
	body := strings.Join(parts, " | ")
	if top {
		return oneLine("parallel: "+body, 72)
	}
	return "(" + body + ")"
}

func oneLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i]) + " …"
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}
