package pack

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// pythonPacker 用 PyInstaller --onefile 打包。
// PyInstaller 不支持交叉打包：--type all 时自动收敛为当前平台；
// 显式指定了其他平台则直接报错，避免产生"已打全平台"的错觉。
// PyInstaller 缺失时自动安装到当前解析到的 python 环境。
type pythonPacker struct {
	opts   Options
	pyPath string
	assets []Asset
}

func (p *pythonPacker) defaultOutput() string { return defaultOutput(p.opts.Entry) }

func (p *pythonPacker) targets(ts []Target, explicit bool) ([]Target, error) {
	host := Host()
	if !explicit {
		fmt.Printf("==> PyInstaller can only package for the current platform; building %s only\n", host)
		return []Target{host}, nil
	}
	for _, t := range ts {
		if !t.IsHost() {
			return nil, fmt.Errorf("PyInstaller cannot cross-compile: target %s requested, but host is %s\n"+
				"  hint: run `pvm pkg python --type host` on each target platform (e.g. via a CI matrix)", t, host)
		}
	}
	return ts, nil
}

func (p *pythonPacker) prepare(_ []Target, assets []Asset) (func(), error) {
	if p.opts.Entry == "" {
		return nil, fmt.Errorf("pvm pkg python requires --entry <main.py>")
	}
	if _, err := os.Stat(p.opts.Entry); err != nil {
		return nil, fmt.Errorf("--entry file not found: %s", p.opts.Entry)
	}
	// PyInstaller 的 --add-data 只能指定目标目录，文件名保持不变
	for _, a := range assets {
		if path.Base(a.Name) != filepath.Base(a.Path) {
			return nil, fmt.Errorf("--include %s -> %s: PyInstaller cannot rename files, keep the same name\n"+
				"  hint: rename the source file, or read the original name inside your code", a.Path, a.Name)
		}
	}
	p.assets = assets

	pyName := "python3"
	if runtime.GOOS == "windows" {
		pyName = "python"
	}
	pyPath, err := resolveCommand("pvm pkg python", pyName)
	if err != nil {
		return nil, err
	}
	p.pyPath = pyPath

	if _, err := captureOutput(pyPath, "-m", "PyInstaller", "--version"); err != nil {
		fmt.Println("==> installing PyInstaller into the resolved python environment")
		if err := runStream("pip install pyinstaller", pyPath, "-m", "pip",
			"install", "--disable-pip-version-check", "--no-input", "pyinstaller"); err != nil {
			return nil, fmt.Errorf("%v\n  hint: install manually with `python -m pip install pyinstaller`", err)
		}
	}
	return func() {}, nil
}

func (p *pythonPacker) build(t Target, out string, idx, total int) error {
	if total > 1 {
		fmt.Printf("==> [%d/%d] %s\n", idx, total, t)
	}

	outDir := filepath.Dir(out)
	workDir, err := os.MkdirTemp("", "pvm-pkg-py-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	// --onefile 单文件产物；--clean 清理缓存保证可复现
	args := []string{
		"-m", "PyInstaller",
		"--onefile", "--clean", "--noconfirm",
		"--distpath", outDir, "--workpath", workDir, "--specpath", workDir,
	}
	// --add-data 的 dest 是包内目录；运行时用 sys._MEIPASS 拼接文件名读取
	for _, a := range p.assets {
		dest := path.Dir(a.Name)
		args = append(args, "--add-data", a.Path+string(os.PathListSeparator)+dest)
	}
	args = append(args, p.opts.Entry)
	if len(p.assets) > 0 {
		fmt.Printf("==> embedding %d asset(s): %s\n", len(p.assets), assetNames(p.assets))
	}
	if err := runStep(p.opts.Verbose, "pyinstaller", p.pyPath, args...); err != nil {
		return err
	}

	// PyInstaller 产物名为入口主干名；重命名为 --output 指定名
	stem := strings.TrimSuffix(filepath.Base(p.opts.Entry), filepath.Ext(p.opts.Entry))
	produced := filepath.Join(outDir, normalizeExeSuffix(stem, t.OS))
	if filepath.Clean(produced) != filepath.Clean(out) {
		if err := os.Rename(produced, out); err != nil {
			return fmt.Errorf("rename %s -> %s: %w", produced, out, err)
		}
	}
	return nil
}
