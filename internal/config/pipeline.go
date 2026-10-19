package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineFileNames 是流水线定义文件名，按优先级排列（项目根目录）
//
// 分工：.pvmrc 声明用什么版本（pvm 自动读写，文件不动）；
// pvm.yaml 声明怎么构建运行（人手写，pvm 只读）。
// 不放进 .pvmrc 的原因：SaveVersionFile 会整体重写 .pvmrc，非版本内容会被抹掉。
var PipelineFileNames = []string{"pvm.yaml", "pvm.yml"}

// PipelineFile 表示 pvm.yaml 解析出的流水线集合
type PipelineFile struct {
	Name     string              // 项目名（顶层 name 字段，可选）
	Path     string              // 来源 pvm.yaml 路径
	Commands map[string][]string // 名字 -> 命令列表
	order    []string            // 全部流水线名（排序后，保证输出确定）
}

// PipelineGroup 用于 --list 按首段（冒号前）分组展示
type PipelineGroup struct {
	Prefix string
	Names  []string
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

// LoadPipelines 解析 pvm.yaml。
//
// 顶层 name 字段（可选）：项目名，--list 与执行头部显示，供人与模型快速识别。
// 其余顶层键为流水线；嵌套键用 ":" 连接成流水线名：
//
//	name: my-app
//	build:
//	  dev:                     # 名字 = build:dev（2 段：动作:环境）
//	    - cd web
//	    - pnpm i
//	pkg:
//	  go:                      # 名字 = pkg:go:pro（3 段：动作:目标:环境）
//	    pro:
//	      - pvm pack go --entry . --output dist/app
//
// 叶子节点两种写法：
//   - 命令列表（多步）
//   - 单个字符串（单步简写）
//
// 引用其他流水线：以 @ 开头的命令项；后台并行步骤以 & 开头。
// YAML 中 @ 和 & 都是保留指示符，必须加引号：
//
//	dev:
//	  all:
//	    - "&@dev:web"          # 前端后台启动
//	    - "&@dev:api"          # 后端后台启动
func LoadPipelines(path string) (*PipelineFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if root == nil {
		root = map[string]any{}
	}

	pf := &PipelineFile{
		Path:     path,
		Commands: make(map[string][]string),
	}
	// 顶层保留字段：name（项目名），不进入流水线命名空间
	if nameVal, ok := root["name"]; ok {
		name, isStr := nameVal.(string)
		if !isStr {
			return nil, fmt.Errorf("%s: top-level \"name\" must be a string, got %T", path, nameVal)
		}
		pf.Name = strings.TrimSpace(name)
		delete(root, "name")
	}

	if err := walkPipelines(root, "", pf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sort.Strings(pf.order) // map 遍历无序，排序保证 --list 输出确定
	return pf, nil
}

// walkPipelines 递归展开嵌套键；":" 连接父子键构成流水线名
func walkPipelines(node map[string]any, prefix string, pf *PipelineFile) error {
	for key, val := range node {
		name := key
		if prefix != "" {
			name = prefix + ":" + key
		}
		if sub, ok := val.(map[string]any); ok {
			if err := walkPipelines(sub, name, pf); err != nil {
				return err
			}
			continue
		}
		cmds, err := leafCommands(name, val)
		if err != nil {
			return err
		}
		if _, dup := pf.Commands[name]; dup {
			return fmt.Errorf("duplicate pipeline name %q", name)
		}
		pf.Commands[name] = cmds
		pf.order = append(pf.order, name)
	}
	return nil
}

// leafCommands 解析叶子节点为命令列表：字符串（单步简写）或字符串列表（多步）
func leafCommands(name string, val any) ([]string, error) {
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []string{strings.TrimSpace(v)}, nil
	case []any:
		var cmds []string
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("pipeline %q step %d: expected a command string, got %T", name, i+1, item)
			}
			s = strings.TrimSpace(s)
			if s != "" {
				cmds = append(cmds, s)
			}
		}
		return cmds, nil
	default:
		return nil, fmt.Errorf("pipeline %q: expected a command list, a single command, or nested keys; got %T", name, val)
	}
}

// Names 返回全部流水线名（排序后）
func (pf *PipelineFile) Names() []string {
	return append([]string(nil), pf.order...)
}

// Has 判断某名字是否存在
func (pf *PipelineFile) Has(name string) bool {
	_, ok := pf.Commands[name]
	return ok
}

// ExpandPipeline 把一条流水线展开为扁平命令列表。
// 以 @ 开头的命令项递归引用其他流水线；检测循环引用。
func (pf *PipelineFile) ExpandPipeline(name string) ([]string, error) {
	return pf.expand(name, nil)
}

func (pf *PipelineFile) expand(name string, visiting []string) ([]string, error) {
	for _, v := range visiting {
		if v == name {
			return nil, fmt.Errorf("circular pipeline reference: %s -> %s",
				strings.Join(visiting, " -> "), name)
		}
	}
	cmds, ok := pf.Commands[name]
	if !ok {
		return nil, fmt.Errorf("pipeline %q is not defined in %s", name, pf.Path)
	}
	visiting = append(visiting, name)

	var out []string
	for _, cmd := range cmds {
		if ref, isRef := pipelineRef(cmd); isRef {
			sub, err := pf.expand(ref, visiting)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
			continue
		}
		out = append(out, cmd)
	}
	return out, nil
}

// pipelineRef 判断命令项是否为 @名字 形式的流水线引用
func pipelineRef(cmd string) (string, bool) {
	if strings.HasPrefix(cmd, "@") && len(cmd) > 1 {
		return strings.TrimSpace(cmd[1:]), true
	}
	return "", false
}

// ListGroups 按首段（第一个冒号前）分组返回全部流水线名，用于 --list 展示
func (pf *PipelineFile) ListGroups() []PipelineGroup {
	groupMap := make(map[string][]string)
	for _, name := range pf.order {
		group := name
		if i := strings.Index(name, ":"); i > 0 {
			group = name[:i]
		}
		groupMap[group] = append(groupMap[group], name)
	}

	prefixes := make([]string, 0, len(groupMap))
	for p := range groupMap {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	groups := make([]PipelineGroup, 0, len(prefixes))
	for _, p := range prefixes {
		groups = append(groups, PipelineGroup{Prefix: p, Names: groupMap[p]})
	}
	return groups
}
