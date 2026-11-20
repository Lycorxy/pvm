package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pvm/pvm/internal/pack"
)

const pkgUsage = `usage: pvm pkg <node|go|python> [flags]

  --entry <path>     入口：node/python 为脚本文件，go 为包路径（默认 .）
  --output <path>    产物路径（不含平台后缀；默认 dist/<入口名>）
  --type <t>         目标平台：all（默认）| host | win | linux | mac
                     可带架构 -amd64/-arm64（如 linux-arm64），逗号分隔可多选
  --zip              每个产物额外打成 zip
  --include <src>    随产物分发的文件/目录（可重复），如配置文件、静态资源
                     config.yaml 打包后仍是 config.yaml；assets/ 只打内容
                     config.yaml:conf/app.yaml 可指定包内路径（python 不能改名）
  --bundled <file>   node：已打包好的单文件入口（跳过 esbuild）
  --external <pkg>   node：esbuild 不打包的模块（可重复），如 --external pino-pretty
  --libc <kind>      node：linux 基座。default=官方（要求 glibc ≥ 2.28）
                     legacy=glibc-217 构建（CentOS 7 可用，自动下载）
  --base <file>      node：自定义基座二进制（优先级最高，需配合单个目标）
  --official         node：跨平台底座走官方源（默认走镜像）
  --verbose          透传 esbuild/pyinstaller 等子进程日志（默认只打结果）

  读取方式（各运行时自己的 API）：
    node    require('node:sea').getAsset('config.yaml', 'utf8')
    python  pathlib.Path(sys._MEIPASS) / 'config.yaml'
    go      不支持 --include，用 //go:embed 在编译期内嵌`

// runPkg 把项目打包为单可执行二进制：node / go / python 共用同一套参数。
//
//	pvm pkg go     --entry . --output dist/app
//	pvm pkg node   --entry src/main.js --type linux,win
//	pvm pkg python --entry app.py --type host --zip
//
// 多目标时产物名在扩展名前插入 -<os>-<arch>，Windows 目标自动补 .exe。
// python 的 PyInstaller 不支持交叉打包：--type all 时只产出当前平台。
func runPkg(args []string) error {
	fs := flag.NewFlagSet("pkg", flag.ContinueOnError)
	var o pack.Options
	fs.StringVar(&o.Entry, "entry", "", "")
	fs.StringVar(&o.Output, "output", "", "")
	fs.StringVar(&o.Type, "type", "all", "")
	fs.BoolVar(&o.Zip, "zip", false, "")
	fs.StringVar(&o.Bundled, "bundled", "", "")
	fs.StringVar(&o.Base, "base", "", "")
	fs.StringVar(&o.Libc, "libc", "default", "")
	fs.BoolVar(&o.Official, "official", false, "")
	fs.BoolVar(&o.Verbose, "verbose", false, "")
	var include, external listFlag
	fs.Var(&include, "include", "")
	fs.Var(&external, "external", "")

	pos, err := parseFlags(fs, args)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, pkgUsage)
	}
	if len(pos) != 1 {
		if len(pos) == 0 {
			return fmt.Errorf("%s", pkgUsage)
		}
		return fmt.Errorf("unexpected arguments: %s\n%s", strings.Join(pos[1:], " "), pkgUsage)
	}

	o.Include = include
	o.External = external
	if _, err := pack.Run(pos[0], o); err != nil {
		return err
	}
	return nil
}
