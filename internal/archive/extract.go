package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/pvm/pvm/internal/logger"
)

const (
	// maxFileSize is the maximum allowed size for a single extracted file (2 GB)
	maxFileSize int64 = 2 * 1024 * 1024 * 1024
	// maxTotalSize is the maximum allowed total extracted size (10 GB)
	maxTotalSize int64 = 10 * 1024 * 1024 * 1024
	// maxFiles is the maximum number of files in an archive
	maxFiles = 100000
)

// Extract extracts an archive to the destination directory
func Extract(archivePath string, destDir string, archiveType string) error {
	switch archiveType {
	case "tar.gz":
		return extractTarGz(archivePath, destDir)
	case "tar.bz2":
		return extractTarBz2(archivePath, destDir)
	case "zip":
		return extractZip(archivePath, destDir)
	case "7z":
		return extract7z(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}
}

func extractTarGz(archivePath string, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var totalSize int64
	fileCount := 0
	lastProgressTime := time.Now()

	logger.Info("  → Extracting tar.gz archive...")

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reader: %w", err)
		}

		// Check file count limit
		fileCount++
		if fileCount > maxFiles {
			return fmt.Errorf("archive contains too many files (>%d), possible zip bomb", maxFiles)
		}

		// Show progress every 500ms
		if time.Since(lastProgressTime) > 500*time.Millisecond {
			logger.ProgressF("\r  → Extracting... %d files, %s", fileCount, formatSize(totalSize))
			lastProgressTime = time.Now()
		}

		// Check individual file size from header
		if header.Size > maxFileSize {
			return fmt.Errorf("file %s exceeds max size (%s > %s)", header.Name,
				formatSize(header.Size), formatSize(maxFileSize))
		}

		target := filepath.Join(destDir, header.Name)

		// Security: prevent path traversal
		cleanTarget := filepath.Clean(target)
		cleanDest := filepath.Clean(destDir)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
			return fmt.Errorf("invalid file path in archive (path traversal): %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}

			// Use LimitReader to prevent decompression bombs
			limitedReader := io.LimitReader(tarReader, maxFileSize+1)

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			written, err := io.Copy(outFile, limitedReader)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("extract file %s: %w", header.Name, err)
			}

			if written > maxFileSize {
				os.Remove(target)
				return fmt.Errorf("file %s exceeds max size during extraction", header.Name)
			}

			totalSize += written
			if totalSize > maxTotalSize {
				return fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb",
					formatSize(maxTotalSize))
			}

		case tar.TypeSymlink:
			// Validate symlink target doesn't escape destDir
			linkTarget := header.Linkname
			if filepath.IsAbs(linkTarget) {
				// Absolute symlinks in archives are suspicious
				logger.Verbose("  Skipping absolute symlink: %s -> %s", header.Name, linkTarget)
				continue
			}
			resolvedLink := filepath.Join(filepath.Dir(target), linkTarget)
			cleanLink := filepath.Clean(resolvedLink)
			if !strings.HasPrefix(cleanLink, cleanDest+string(os.PathSeparator)) && cleanLink != cleanDest {
				logger.Verbose("  Skipping escaping symlink: %s -> %s", header.Name, linkTarget)
				continue
			}

			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				// On Windows, symlink may fail without admin rights; skip gracefully
				logger.Verbose("  Warning: symlink failed: %s -> %s: %v", header.Name, header.Linkname, err)
			}
		}
	}

	logger.Verbose("  Extracted %d files, total size: %s", fileCount, formatSize(totalSize))
	return nil
}

func extractTarBz2(archivePath string, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	bz2Reader := bzip2.NewReader(f)

	tarReader := tar.NewReader(bz2Reader)

	var totalSize int64
	fileCount := 0
	lastProgressTime := time.Now()

	logger.Info("  → Extracting tar.bz2 archive...")

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reader: %w", err)
		}

		// Check file count limit
		fileCount++
		if fileCount > maxFiles {
			return fmt.Errorf("archive contains too many files (>%d), possible zip bomb", maxFiles)
		}

		// Show progress every 500ms
		if time.Since(lastProgressTime) > 500*time.Millisecond {
			logger.ProgressF("\r  → Extracting... %d files, %s", fileCount, formatSize(totalSize))
			lastProgressTime = time.Now()
		}

		// Check individual file size from header
		if header.Size > maxFileSize {
			return fmt.Errorf("file %s exceeds max size (%s > %s)", header.Name,
				formatSize(header.Size), formatSize(maxFileSize))
		}

		target := filepath.Join(destDir, header.Name)

		// Security: prevent path traversal
		cleanTarget := filepath.Clean(target)
		cleanDest := filepath.Clean(destDir)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
			return fmt.Errorf("invalid file path in archive (path traversal): %s", header.Name)
		}

		// Windows 长路径支持：添加 \\?\ 前缀突破 260 字符限制
		target = longPath(target)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}

			// Use LimitReader to prevent decompression bombs
			limitedReader := io.LimitReader(tarReader, maxFileSize+1)

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}

			written, err := io.Copy(outFile, limitedReader)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("extract file %s: %w", header.Name, err)
			}

			if written > maxFileSize {
				os.Remove(target)
				return fmt.Errorf("file %s exceeds max size during extraction", header.Name)
			}

			totalSize += written
			if totalSize > maxTotalSize {
				return fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb",
					formatSize(maxTotalSize))
			}

		case tar.TypeSymlink:
			// Validate symlink target doesn't escape destDir
			linkTarget := header.Linkname
			if filepath.IsAbs(linkTarget) {
				// Absolute symlinks in archives are suspicious
				logger.Verbose("  Skipping absolute symlink: %s -> %s", header.Name, linkTarget)
				continue
			}
			resolvedLink := filepath.Join(filepath.Dir(target), linkTarget)
			cleanLink := filepath.Clean(resolvedLink)
			if !strings.HasPrefix(cleanLink, cleanDest+string(os.PathSeparator)) && cleanLink != cleanDest {
				logger.Verbose("  Skipping escaping symlink: %s -> %s", header.Name, linkTarget)
				continue
			}

			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				// On Windows, symlink may fail without admin rights; skip gracefully
				logger.Verbose("  Warning: symlink failed: %s -> %s: %v", header.Name, header.Linkname, err)
			}
		}
	}

	logger.Verbose("  Extracted %d files, total size: %s", fileCount, formatSize(totalSize))
	return nil
}

func extractZip(archivePath string, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Check file count
	if len(r.File) > maxFiles {
		return fmt.Errorf("archive contains too many files (%d > %d), possible zip bomb", len(r.File), maxFiles)
	}

	var totalSize int64
	lastProgressTime := time.Now()

	logger.Info("  → Extracting zip archive...")

	for i, f := range r.File {
		// Show progress every 500ms or every 100 files
		if time.Since(lastProgressTime) > 500*time.Millisecond || i%100 == 0 {
			logger.ProgressF("\r  → Extracting... %d/%d files, %s", i+1, len(r.File), formatSize(totalSize))
			lastProgressTime = time.Now()
		}

		target := filepath.Join(destDir, f.Name)

		// Security: prevent path traversal
		cleanTarget := filepath.Clean(target)
		cleanDest := filepath.Clean(destDir)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
			return fmt.Errorf("invalid file path in archive (path traversal): %s", f.Name)
		}

		// Check uncompressed size from header
		if int64(f.UncompressedSize64) > maxFileSize {
			return fmt.Errorf("file %s exceeds max size (%s > %s)", f.Name,
				formatSize(int64(f.UncompressedSize64)), formatSize(maxFileSize))
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("open zip entry: %w", err)
		}

		// Use LimitReader to prevent decompression bombs
		limitedReader := io.LimitReader(rc, maxFileSize+1)
		written, err := io.Copy(outFile, limitedReader)
		rc.Close()
		outFile.Close()

		if err != nil {
			return fmt.Errorf("extract file %s: %w", f.Name, err)
		}

		if written > maxFileSize {
			os.Remove(target)
			return fmt.Errorf("file %s exceeds max size during extraction", f.Name)
		}

		totalSize += written
		if totalSize > maxTotalSize {
			return fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb",
				formatSize(maxTotalSize))
		}
	}

	logger.Verbose("  Extracted %d files, total size: %s", len(r.File), formatSize(totalSize))
	return nil
}

func extract7z(archivePath string, destDir string) error {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open 7z: %w", err)
	}
	defer r.Close()

	// Check file count
	if len(r.File) > maxFiles {
		return fmt.Errorf("archive contains too many files (%d > %d), possible zip bomb", len(r.File), maxFiles)
	}

	var totalSize int64
	lastProgressTime := time.Now()

	logger.Info("  → Extracting 7z archive...")

	for i, f := range r.File {
		// Show progress every 500ms or every 100 files
		if time.Since(lastProgressTime) > 500*time.Millisecond || i%100 == 0 {
			logger.ProgressF("\r  → Extracting... %d/%d files, %s", i+1, len(r.File), formatSize(totalSize))
			lastProgressTime = time.Now()
		}

		// Sanitize path: normalize backslashes to forward slashes (7z may use \)
		name := strings.ReplaceAll(f.Name, `\`, "/")
		// Reject absolute paths and path traversal
		if strings.HasPrefix(name, "/") || strings.Contains(name, "../") {
			logger.Verbose("  Skipping unsafe path in 7z: %s", f.Name)
			continue
		}

		target := filepath.Join(destDir, name)

		// Security: prevent path traversal
		cleanTarget := filepath.Clean(target)
		cleanDest := filepath.Clean(destDir)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
			logger.Verbose("  Skipping escaping path in 7z: %s", f.Name)
			continue
		}

		// Windows 长路径支持
		target = longPath(target)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("open 7z entry: %w", err)
		}

		// Use LimitReader to prevent decompression bombs
		limitedReader := io.LimitReader(rc, maxFileSize+1)
		written, err := io.Copy(outFile, limitedReader)
		rc.Close()
		outFile.Close()

		if err != nil {
			return fmt.Errorf("extract file %s: %w", f.Name, err)
		}

		if written > maxFileSize {
			os.Remove(target)
			return fmt.Errorf("file %s exceeds max size during extraction", f.Name)
		}

		totalSize += written
		if totalSize > maxTotalSize {
			return fmt.Errorf("total extracted size exceeds limit (%s), possible zip bomb",
				formatSize(maxTotalSize))
		}
	}

	logger.Verbose("  Extracted %d files, total size: %s", len(r.File), formatSize(totalSize))
	return nil
}

// longPath 为 Windows 路径添加 \\?\ 前缀以支持超过 260 字符的长路径
// 参考：https://docs.microsoft.com/en-us/windows/win32/fileio/naming-a-file#maximum-path-length-limitation
func longPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	// 已经有前缀则直接返回
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}
	// 必须是绝对路径才能添加前缀
	if !filepath.IsAbs(path) {
		return path
	}
	// 转换为 Windows 风格路径并添加前缀
	return `\\?\` + filepath.Clean(path)
}

func formatSize(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
