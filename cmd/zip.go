package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/pvm/pvm/internal/zipx"
)

const zipUsage = `usage: pvm zip <src...> --output <file.zip> [flags]

  --output, -o <file>   输出 zip 路径（默认 <首个源名>.zip）
  --exclude, -x <glob>  排除匹配的文件/目录（可重复），如 -x node_modules -x "*.map"
  --reproducible        固定时间戳与权限：同样输入得到逐字节相同的 zip

  目录写法同 cp -r：dist 保留外层目录名，dist/ 或 . 只打目录内容`

const unzipUsage = `usage: pvm unzip <file.zip> [--output <dir>] [--list]

  --output, -o <dir>    解压目录（默认当前目录）
  --list, -l            只列出内容，不解压`

// runZip: pvm zip <src...> --output x.zip [--exclude glob] [--reproducible]
func runZip(args []string) error {
	fs := flag.NewFlagSet("zip", flag.ContinueOnError)
	var output string
	var exclude listFlag
	var repro bool
	fs.StringVar(&output, "output", "", "")
	fs.StringVar(&output, "o", "", "")
	fs.Var(&exclude, "exclude", "")
	fs.Var(&exclude, "x", "")
	fs.BoolVar(&repro, "reproducible", false, "")

	srcs, err := parseFlags(fs, args)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, zipUsage)
	}
	if len(srcs) == 0 {
		return fmt.Errorf("%s", zipUsage)
	}
	if output == "" {
		base := filepath.Base(filepath.Clean(srcs[0]))
		if base == "." || base == string(filepath.Separator) {
			if abs, err := filepath.Abs(srcs[0]); err == nil {
				base = filepath.Base(abs)
			}
		}
		output = strings.TrimSuffix(base, filepath.Ext(base)) + ".zip"
	}

	st, err := zipx.Zip(output, srcs, zipx.Options{Exclude: exclude, Reproducible: repro})
	if err != nil {
		return err
	}
	ratio := ""
	if st.Size > 0 && st.Packed < st.Size {
		ratio = fmt.Sprintf(", %.0f%%", float64(st.Packed)*100/float64(st.Size))
	}
	fmt.Printf("✔ %s  %d files, %s → %s%s  (%s)\n",
		output, st.Files, zipx.FormatSize(st.Size), zipx.FormatSize(st.Packed), ratio, st.Elapsed.Round(10*1e6))
	return nil
}

// runUnzip: pvm unzip x.zip [--output dir] [--list]
func runUnzip(args []string) error {
	fs := flag.NewFlagSet("unzip", flag.ContinueOnError)
	var output string
	var list bool
	fs.StringVar(&output, "output", ".", "")
	fs.StringVar(&output, "o", ".", "")
	fs.BoolVar(&list, "list", false, "")
	fs.BoolVar(&list, "l", false, "")

	pos, err := parseFlags(fs, args)
	if err != nil {
		return fmt.Errorf("%v\n%s", err, unzipUsage)
	}
	if len(pos) != 1 {
		return fmt.Errorf("%s", unzipUsage)
	}
	archive := pos[0]

	if list {
		infos, err := zipx.List(archive)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		var total int64
		for _, i := range infos {
			size := ""
			if !i.IsDir() {
				size = zipx.FormatSize(i.Size)
				total += i.Size
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", i.ModTime.Format("2006-01-02 15:04"), size, i.Name)
		}
		tw.Flush()
		fmt.Printf("%d entries, %s\n", len(infos), zipx.FormatSize(total))
		return nil
	}

	st, err := zipx.Unzip(archive, output, zipx.Options{})
	for _, w := range st.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if err != nil {
		return err
	}
	fmt.Printf("✔ %s → %s  %d files, %s  (%s)\n",
		archive, output, st.Files, zipx.FormatSize(st.Size), st.Elapsed.Round(10*1e6))
	return nil
}
