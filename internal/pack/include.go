package pack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Asset 是随产物一起分发的额外文件（配置文件、静态资源等）。
type Asset struct {
	Name string // 包内路径，正斜杠，如 config.yaml、assets/logo.png
	Path string // 磁盘路径（绝对路径）
}

// ParseIncludes 解析 --include 列表：
//
//	config.yaml                 包内 config.yaml
//	assets                      包内 assets/...（保留目录名）
//	assets/                     包内 ...（只打目录内容，与 pvm zip 一致）
//	config.yaml:conf/app.yaml   包内改名（python 不支持改名，会报错）
//
// 目录会递归展开成文件；同一个包内路径出现两次视为错误。
func ParseIncludes(specs []string) ([]Asset, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	var assets []Asset
	owner := make(map[string]string)

	add := func(name, file string) error {
		if prev, dup := owner[name]; dup {
			return fmt.Errorf("--include: duplicate entry %q (from %s and %s)", name, prev, file)
		}
		abs, err := filepath.Abs(file)
		if err != nil {
			return err
		}
		owner[name] = abs
		assets = append(assets, Asset{Name: name, Path: abs})
		return nil
	}

	for _, spec := range specs {
		src, dest := splitIncludeSpec(spec)
		if src == "" {
			return nil, fmt.Errorf("--include %q: empty source", spec)
		}
		info, err := os.Stat(src)
		if err != nil {
			return nil, fmt.Errorf("--include %s: %w", src, err)
		}

		if !info.IsDir() {
			name := cleanAssetName(dest)
			if name == "" {
				name = filepath.Base(src)
			}
			if err := add(name, src); err != nil {
				return nil, err
			}
			continue
		}

		// 目录：dest 为空时用目录名做前缀；spec 带尾斜杠表示只打内容
		prefix := ""
		switch {
		case dest != "":
			prefix = strings.Trim(cleanAssetName(dest), "/")
		case strings.HasSuffix(spec, "/") || strings.HasSuffix(spec, `\`):
			prefix = ""
		default:
			prefix = filepath.Base(filepath.Clean(src))
		}

		root := filepath.Clean(src)
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == root {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return nil // 不跟随符号链接，避免循环与越界
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			name := path.Join(prefix, filepath.ToSlash(rel))
			return add(name, p)
		})
		if err != nil {
			return nil, fmt.Errorf("--include %s: %w", src, err)
		}
	}
	return assets, nil
}

// splitIncludeSpec 按最后一个冒号拆 "源:包内路径"。
// Windows 盘符（C:\a\b）里的冒号不算分隔符；没有冒号时 dest 为空。
func splitIncludeSpec(spec string) (src, dest string) {
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		return spec, ""
	}
	if i == 1 && isLetter(spec[0]) {
		return spec, "" // 盘符
	}
	return spec[:i], spec[i+1:]
}

// assetNames 生成简短的名字列表，用于日志
func assetNames(assets []Asset) string {
	const max = 6
	names := make([]string, 0, max)
	for i, a := range assets {
		if i == max {
			return strings.Join(names, ", ") + fmt.Sprintf(", … (%d total)", len(assets))
		}
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// cleanAssetName 规范化包内路径：反斜杠转正斜杠，去掉开头的 ./ 和 /
func cleanAssetName(name string) string {
	name = filepath.ToSlash(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "./")
	return strings.TrimPrefix(name, "/")
}
