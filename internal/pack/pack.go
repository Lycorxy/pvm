// Package pack 把项目打包为单可执行二进制，三种运行时共用一套命令行与产物约定：
//
//	pvm pkg <node|go|python> --entry <入口> --output <产物> --type <all|host|win|linux|mac>
//
// 各运行时只实现 packer 接口里与自身相关的部分；目标解析、产物命名、
// .exe 后缀、汇总输出、zip 联动都在这里写一份。
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pvm/pvm/internal/zipx"
)

// Options 是 pvm pkg 的全部参数
type Options struct {
	Entry    string   // 入口：node/python 为脚本，go 为包路径（默认 .）
	Output   string   // 产物路径（不含平台后缀）；默认 dist/<入口主干名>
	Type     string   // 目标：all（默认）| host | win | linux | mac，可带 -amd64/-arm64，可逗号多选
	Zip      bool     // 每个产物额外打成 zip
	Bundled  string   // node：已打包好的单文件入口（跳过 esbuild）
	External []string // node：esbuild 不打包的模块（如 pino-pretty）
	Base     string   // node：自定义基座二进制（最高优先级）
	Libc     string   // node：linux 基座，legacy = glibc-217（兼容 CentOS 7）
	Official bool     // node：跨平台底座走官方源而非镜像
	Include  []string // 随产物分发的文件/目录，见 ParseIncludes
	Verbose  bool     // 透传子进程输出（默认只打结果）
}

// Artifact 一个已生成的产物
type Artifact struct {
	Target  Target
	Path    string
	Size    int64
	Elapsed time.Duration
}

// packer 是运行时相关部分的统一接口
type packer interface {
	// defaultOutput 未指定 --output 时的默认产物路径
	defaultOutput() string
	// targets 校验/收敛目标；explicit 表示用户显式限定了目标
	targets(ts []Target, explicit bool) ([]Target, error)
	// prepare 做一次性的前置检查与准备（工具链、bundle、内嵌资源等）；返回的 cleanup 必须被调用
	prepare(ts []Target, assets []Asset) (cleanup func(), err error)
	// build 构建单个目标，产物写到 out
	build(t Target, out string, idx, total int) error
}

// Run 执行一次打包并打印汇总表
func Run(rt string, o Options) ([]Artifact, error) {
	var p packer
	switch strings.ToLower(rt) {
	case "node":
		p = &nodePacker{opts: o}
	case "go":
		p = &goPacker{opts: o}
	case "python", "py":
		p = &pythonPacker{opts: o}
	default:
		return nil, fmt.Errorf("unsupported runtime: %q (supported: node, go, python)", rt)
	}

	targets, explicit, err := ParseType(o.Type)
	if err != nil {
		return nil, err
	}
	targets, err = p.targets(targets, explicit)
	if err != nil {
		return nil, err
	}

	output := o.Output
	if output == "" {
		output = p.defaultOutput()
	}

	assets, err := ParseIncludes(o.Include)
	if err != nil {
		return nil, err
	}

	cleanup, err := p.prepare(targets, assets)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	multi := len(targets) > 1
	if multi {
		fmt.Printf("==> pvm pkg %s: %d targets (%s)\n", strings.ToLower(rt), len(targets), joinTargets(targets))
	}

	var artifacts []Artifact
	for i, t := range targets {
		out := OutputPath(output, t, multi)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return artifacts, err
		}
		start := time.Now()
		if err := p.build(t, out, i+1, len(targets)); err != nil {
			return artifacts, fmt.Errorf("%s: %w", t, err)
		}
		a := Artifact{Target: t, Path: out, Elapsed: time.Since(start)}
		if info, err := os.Stat(out); err == nil {
			a.Size = info.Size()
		}
		artifacts = append(artifacts, a)
	}

	var zips []Artifact
	if o.Zip {
		for _, a := range artifacts {
			z, err := zipArtifact(a)
			if err != nil {
				return artifacts, fmt.Errorf("zip %s: %w", a.Path, err)
			}
			zips = append(zips, z)
		}
	}

	printSummary(artifacts, zips)
	return artifacts, nil
}

// zipArtifact 把单个产物打成 zip（同目录，去掉 .exe；包内保持原文件名并带可执行位）
func zipArtifact(a Artifact) (Artifact, error) {
	start := time.Now()
	zipPath := strings.TrimSuffix(a.Path, ".exe") + ".zip"
	entry := zipx.Entry{
		Name:    filepath.Base(a.Path),
		Path:    a.Path,
		Mode:    0o755,
		ModTime: time.Now(),
		Size:    a.Size,
	}
	if _, err := zipx.Create(zipPath, []zipx.Entry{entry}, zipx.Options{}); err != nil {
		return Artifact{}, err
	}
	z := Artifact{Target: a.Target, Path: zipPath, Elapsed: time.Since(start)}
	if info, err := os.Stat(zipPath); err == nil {
		z.Size = info.Size()
	}
	return z, nil
}

func printSummary(artifacts, zips []Artifact) {
	if len(artifacts) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("==> done: %d artifact(s)\n", len(artifacts))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "    TARGET\tFILE\tSIZE\tTIME")
	for _, a := range artifacts {
		fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\n", a.Target, a.Path, formatSize(a.Size), a.Elapsed.Round(10*time.Millisecond))
	}
	for _, z := range zips {
		fmt.Fprintf(tw, "    %s (zip)\t%s\t%s\t%s\n", z.Target, z.Path, formatSize(z.Size), z.Elapsed.Round(10*time.Millisecond))
	}
	tw.Flush()
}
