package cmd

import (
	"flag"
	"io"
)

// parseFlags 解析命令行：参数与位置参数可以任意交错
// （标准库 flag 遇到第一个非 flag 就停止，这里循环继续解析）。
// 返回所有位置参数；"--" 之后的全部当作位置参数。
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// listFlag 是可重复使用的字符串列表 flag（如 --exclude a --exclude b）
type listFlag []string

func (l *listFlag) String() string     { return "" }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }
