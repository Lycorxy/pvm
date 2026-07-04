package download

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pvm/pvm/internal/logger"
)

const (
	defaultTimeout    = 30 * time.Minute // download timeout (large files like Git 111MB)
	connectTimeout    = 60 * time.Second // connection timeout
	maxRetries        = 5
	retryBaseInterval = 1 * time.Second
)

// progressWriter wraps an io.Writer to display download progress
type progressWriter struct {
	total      int64
	downloaded int64
	writer     io.Writer
	startTime  time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.downloaded += int64(n)
	pw.printProgress()
	return n, err
}

func (pw *progressWriter) printProgress() {
	if pw.total > 0 {
		pct := float64(pw.downloaded) / float64(pw.total) * 100
		bar := int(pct / 2.5)
		elapsed := time.Since(pw.startTime).Seconds()
		speed := float64(pw.downloaded) / elapsed
		logger.ProgressF("\r  [%-40s] %6.1f%% (%s / %s) %s/s",
			strings.Repeat("█", bar)+strings.Repeat("░", 40-bar),
			pct,
			formatBytes(pw.downloaded),
			formatBytes(pw.total),
			formatBytes(int64(speed)))
	} else {
		logger.ProgressF("\r  Downloaded: %s", formatBytes(pw.downloaded))
	}
}

func formatBytes(b int64) string {
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

// newHTTPClient creates an HTTP client with timeout and proxy support
func newHTTPClient() *http.Client {
	transport := &http.Transport{
		// Proxy from environment: HTTP_PROXY, HTTPS_PROXY, NO_PROXY
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: connectTimeout,
		TLSHandshakeTimeout:   30 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // enforce TLS 1.2+
		},
		DisableKeepAlives: false, // enable keep-alive for better performance
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
	}

	return &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}
}

// isRetryableError determines whether an error is worth retrying.
// Non-retryable errors include HTTP 4xx (except 429) such as 404/403.
// Retryable errors include network timeouts, connection resets, and HTTP 5xx/429.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Network-level errors that are typically transient
	retryablePatterns := []string{
		"timeout", "connection reset", "connection refused",
		"EOF", "broken pipe", "no such host",
		"i/o timeout", "network is unreachable",
	}
	lower := strings.ToLower(errStr)
	for _, p := range retryablePatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	// HTTP 5xx server errors are retryable
	if strings.Contains(errStr, "HTTP 5") {
		return true
	}
	// HTTP 429 Too Many Requests is retryable
	if strings.Contains(errStr, "HTTP 429") {
		return true
	}
	return false
}

// retryDelay returns the backoff duration for a given attempt number (1-based).
// The first retry is fast (1s); subsequent retries use exponential backoff (2s, 4s, 8s, 16s).
func retryDelay(attempt int) time.Duration {
	if attempt == 1 {
		return 1 * time.Second
	}
	return retryBaseInterval * time.Duration(1<<(attempt-1))
}

// DownloadFile downloads a URL to a local file path with progress display,
// retries and timeout. Optionally verifies SHA256 if expectedSHA256 is non-empty.
func DownloadFile(url string, destPath string) error {
	return downloadWithChecksum(url, destPath, "")
}

// DownloadFileWithChecksum downloads a URL and verifies SHA256 checksum.
func DownloadFileWithChecksum(url string, destPath string, expectedSHA256 string) error {
	return downloadWithChecksum(url, destPath, expectedSHA256)
}

func downloadWithChecksum(url string, destPath string, expectedSHA256 string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			wait := retryDelay(attempt)
			logger.Info("  ⟳ Retry %d/%d in %v...", attempt, maxRetries, wait)
			time.Sleep(wait)
		}

		err := downloadOnce(url, destPath)
		if err != nil {
			lastErr = err
			logger.Verbose("  Download attempt %d failed: %v", attempt, err)
			// Clean up partial file
			os.Remove(destPath)
			// If the error is not retryable, stop immediately to save time
			if !isRetryableError(err) {
				logger.Verbose("  Error is not retryable, aborting")
				break
			}
			continue
		}

		// Validate the downloaded file for completeness and magic-number integrity
		if err := validateDownloadedFile(destPath, 0, ""); err != nil {
			lastErr = fmt.Errorf("file validation failed: %w", err)
			logger.Verbose("  Downloaded file validation failed: %v", err)
			os.Remove(destPath)
			continue
		}

		// Verify SHA256 checksum if provided
		if expectedSHA256 != "" {
			logger.Verbose("  Verifying SHA256 checksum...")
			actualHash, err := fileSHA256(destPath)
			if err != nil {
				os.Remove(destPath)
				return fmt.Errorf("checksum calculation failed: %w", err)
			}
			if !strings.EqualFold(actualHash, expectedSHA256) {
				os.Remove(destPath)
				return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualHash)
			}
			logger.Verbose("  ✓ SHA256 checksum verified: %s", actualHash[:16]+"...")
		}

		return nil
	}

	return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

func downloadOnce(url string, destPath string) error {
	client := newHTTPClient()

	logger.Verbose("  HTTP GET %s", url)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	logger.Verbose("  Content-Length: %d, Content-Type: %s", resp.ContentLength, resp.Header.Get("Content-Type"))

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	pw := &progressWriter{
		total:     resp.ContentLength,
		writer:    out,
		startTime: time.Now(),
	}

	if _, err := io.Copy(pw, resp.Body); err != nil {
		return fmt.Errorf("download write: %w", err)
	}

	fmt.Fprintln(os.Stderr) // newline after progress bar

	// Verify file size if Content-Length was provided
	if resp.ContentLength > 0 && pw.downloaded != resp.ContentLength {
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", pw.downloaded, resp.ContentLength)
	}

	return nil
}

// validateDownloadedFile validates the downloaded file for completeness and
// magic-number integrity. expectedSize of 0 skips size validation.
// archiveType of "" skips magic-number validation.
// Known archive types: "zip", "tar.gz", "tar.bz2", "7z".
func validateDownloadedFile(path string, expectedSize int64, archiveType string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat downloaded file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("downloaded file is empty")
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return fmt.Errorf("file size mismatch: got %d, expected %d", info.Size(), expectedSize)
	}
	// Validate file header magic numbers when an archive type is specified
	return validateFileMagic(path, archiveType)
}

// validateFileMagic checks the file header magic bytes against the expected
// archive type. Supported types: "zip", "tar.gz", "tar.bz2", "7z". An empty
// archiveType skips validation.
func validateFileMagic(path string, archiveType string) error {
	if archiveType == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 6)
	n, err := f.Read(header)
	if err != nil || n < 2 {
		return fmt.Errorf("cannot read file header")
	}

	switch archiveType {
	case "zip":
		// ZIP files start with PK\x03\x04
		if header[0] != 'P' || header[1] != 'K' {
			return fmt.Errorf("invalid zip file: bad magic number")
		}
	case "tar.gz":
		// gzip files start with 0x1f 0x8b
		if header[0] != 0x1f || header[1] != 0x8b {
			return fmt.Errorf("invalid gzip file: bad magic number")
		}
	case "tar.bz2":
		// bzip2 files start with "BZ"
		if header[0] != 'B' || header[1] != 'Z' {
			return fmt.Errorf("invalid bzip2 file: bad magic number")
		}
	case "7z":
		// 7z files start with "7z\xBC\xAF\x27\x1C"
		if n < 6 || !bytes.Equal(header[:6], []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}) {
			return fmt.Errorf("invalid 7z file: bad magic number")
		}
	}
	return nil
}

// fileSHA256 computes the SHA256 hash of a file
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
