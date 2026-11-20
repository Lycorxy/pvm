package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineFileNames 是流水线定义文件名，按优先级排列（项目根目录）
//
// 分工：.pvmrc 声明用什么版本（pvm 自动读写，文件不动）；
// pvm.yaml 声明怎么构建运行（人手写，pvm 只读）。
// 不放进 .pvmrc 的原因：SaveVersionFile 会整体重写 .pvmrc，非版本内容会被抹掉。
var PipelineFileNames = []string{"pvm.yaml", "pvm.yml"}

// StepKind 步骤类型
type StepKind int

const (
	StepCmd StepKind = iota // 一条命令（或对同名流水线的引用，由编译阶段判定）
	StepSeq                 // 按顺序执行的步骤列表
	StepPar                 // 同时执行的步骤列表
)

// Step 是 pvm.yaml 中的一个步骤，三种形态：
//
//	pnpm build            # 命令
//	[a, b, c]             # 顺序
//	{parallel: [a, b]}    # 并行
type Step struct {
	Kind  StepKind
	Cmd   string  // StepCmd
	Items []*Step // StepSeq / StepPar
	Line  int     // 源文件行号，用于报错
}

// PipelineFile 表示 pvm.yaml 解析出的流水线集合
type PipelineFile struct {
	Name  string           // 项目名（顶层 name 字段，可选）
	Path  string           // 来源 pvm.yaml 路径
	Steps map[string]*Step // 流水线名 → 步骤
	order []string         // 声明顺序
}

// FindPipelineFile 从 startDir 向上查找最近的 pvm.yaml / pvm.yml。
// 家目录即停止（家目录及以上不是项目目录）；返回 nil 表示未找到。
// Monorepo：根目录放一个 pvm.yaml，任意子目录执行 pvm run 都能向上找到；
// 子目录放自己的 pvm.yaml 时就近优先。
func FindPipelineFile(startDir string) (*PipelineFile, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	homeDir, _ := os.UserHomeDir()
	homeDir, _ = filepath.Abs(homeDir)

	for {
		if homeDir != "" && strings.EqualFold(dir, homeDir) {
			return nil, nil
		}
		for _, name := range PipelineFileNames {
			p := filepath.Join(dir, name)
			if fileExists(p) {
				return LoadPipelines(p)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil // 到达根目录
		}
		dir = parent
	}
}

// LoadPipelines 读取并解析 pvm.yaml
func LoadPipelines(path string) (*PipelineFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePipelines(path, data)
}

// ParsePipelines 解析 pvm.yaml 内容。
//
// 顶层 name 字段（可选）是项目名；其余每个顶层键是一条流水线，键名就是流水线名
// （可以带冒号，如 release:pro）。值的写法只有三种：
//
//	install: pnpm i              # 一条命令
//	release:                     # 列表：从上到下依次执行，失败即停
//	  - install
//	  - parallel: [lint, test]   # 列表里可以嵌套并行组
//	  - pnpm build
//	check:                       # 并行：下面的步骤同时执行
//	  parallel:
//	    - lint
//	    - test
//
// 步骤如果恰好等于某条流水线的名字，就是对它的引用；否则按命令执行。
func ParsePipelines(path string, data []byte) (*PipelineFile, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	pf := &PipelineFile{Path: path, Steps: make(map[string]*Step)}
	if doc.Kind == 0 || len(doc.Content) == 0 { // 空文件
		return pf, nil
	}
	root := resolveAlias(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s:%d: top level must be a mapping of pipeline names, e.g.\n  build: pnpm build", path, root.Line)
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i], root.Content[i+1]
		name := strings.TrimSpace(key.Value)

		if name == "name" {
			val = resolveAlias(val)
			if val.Kind != yaml.ScalarNode || val.Tag != "!!str" {
				return nil, fmt.Errorf("%s:%d: top-level \"name\" must be a string", path, val.Line)
			}
			pf.Name = strings.TrimSpace(val.Value)
			continue
		}
		if name == "" {
			return nil, fmt.Errorf("%s:%d: pipeline name must not be empty", path, key.Line)
		}
		if _, dup := pf.Steps[name]; dup {
			return nil, fmt.Errorf("%s:%d: duplicate pipeline %q", path, key.Line, name)
		}
		step, err := parseStep(val)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: pipeline %q: %w", path, key.Line, name, err)
		}
		pf.Steps[name] = step
		pf.order = append(pf.order, name)
	}
	return pf, nil
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

func parseStep(n *yaml.Node) (*Step, error) {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag != "!!str" {
			return nil, fmt.Errorf("line %d: expected a command (string), got %q", n.Line, n.Value)
		}
		cmd := strings.TrimSpace(n.Value)
		if cmd == "" {
			return nil, fmt.Errorf("line %d: empty command", n.Line)
		}
		return &Step{Kind: StepCmd, Cmd: cmd, Line: n.Line}, nil

	case yaml.SequenceNode:
		items, err := parseSteps(n)
		if err != nil {
			return nil, err
		}
		return &Step{Kind: StepSeq, Items: items, Line: n.Line}, nil

	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if k := n.Content[i].Value; k != "parallel" {
				return nil, fmt.Errorf("line %d: unknown key %q — a step is a command, a list, or `parallel:` followed by a list",
					n.Content[i].Line, k)
			}
		}
		if len(n.Content) != 2 {
			return nil, fmt.Errorf("line %d: `parallel:` must be the only key here", n.Line)
		}
		list := resolveAlias(n.Content[1])
		if list.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("line %d: `parallel:` must be followed by a list", list.Line)
		}
		items, err := parseSteps(list)
		if err != nil {
			return nil, err
		}
		return &Step{Kind: StepPar, Items: items, Line: n.Line}, nil
	}
	return nil, fmt.Errorf("line %d: expected a command, a list, or `parallel:`", n.Line)
}

func parseSteps(seq *yaml.Node) ([]*Step, error) {
	if len(seq.Content) == 0 {
		return nil, fmt.Errorf("line %d: empty list (a pipeline needs at least one step)", seq.Line)
	}
	items := make([]*Step, 0, len(seq.Content))
	for _, c := range seq.Content {
		s, err := parseStep(c)
		if err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, nil
}

// Names 返回全部流水线名（按声明顺序）
func (pf *PipelineFile) Names() []string {
	return append([]string(nil), pf.order...)
}

// Has 判断某名字是否存在
func (pf *PipelineFile) Has(name string) bool {
	_, ok := pf.Steps[name]
	return ok
}
