package ipa

import (
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	atvhttp "github.com/bitxeno/atvloadly/internal/http"
	"github.com/bitxeno/atvloadly/internal/utils"
)

// DownloadResult holds the result of downloading and parsing an IPA.
type DownloadResult struct {
	// LocalPath is the path to the downloaded IPA file on disk.
	LocalPath string
	// Name is the app display name extracted from Info.plist.
	Name string
	// BundleIdentifier is the CFBundleIdentifier from Info.plist.
	BundleIdentifier string
	// Version is the CFBundleShortVersionString from Info.plist.
	Version string
	// IconPath is the path to the extracted icon PNG file, or empty if extraction failed.
	IconPath string
}

// DownloadProgressFn is called during download with bytes downloaded and total size.
// total may be -1 if the content length is unknown.
type DownloadProgressFn func(downloaded, total int64)

// DownloadAndParse downloads an IPA from rawURL, saves it to a temp file,
// parses its metadata, and extracts the app icon. The caller is responsible
// for removing the downloaded IPA file and icon file when no longer needed.
//
// If progressFn is nil, no progress is reported.
func DownloadAndParse(rawURL string, progressFn DownloadProgressFn) (*DownloadResult, error) {
	tmpDir := filepath.Join(app.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Download
	tmpPath, err := downloadIPA(rawURL, tmpDir, progressFn)
	if err != nil {
		return nil, err
	}

	// Parse
	result, err := parseIPAMetadata(tmpPath, tmpDir)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	result.LocalPath = tmpPath
	return result, nil
}

// ParseLocalIPA parses a locally-available IPA file and extracts its icon.
// The caller is responsible for removing the icon file when no longer needed.
func ParseLocalIPA(localPath string) (*DownloadResult, error) {
	tmpDir := filepath.Join(app.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	result, err := parseIPAMetadata(localPath, tmpDir)
	if err != nil {
		return nil, err
	}
	result.LocalPath = localPath
	return result, nil
}

// downloadIPA downloads an IPA from rawURL to a temp file in saveDir.
func downloadIPA(rawURL string, saveDir string, progressFn DownloadProgressFn) (string, error) {
	tmpFile, err := os.CreateTemp(saveDir, "install_url_*.ipa")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set(atvhttp.HEADER_USER_AGENT, atvhttp.HTTP_USER_AGENT)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to download ipa: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}

	writer := &progressWriter{
		dest:       tmpFile,
		total:      resp.ContentLength,
		progressFn: progressFn,
	}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write ipa file: %w", err)
	}
	_ = tmpFile.Close()

	return tmpPath, nil
}

// parseIPAMetadata parses an IPA file and extracts its icon.
func parseIPAMetadata(ipaPath string, saveDir string) (*DownloadResult, error) {
	info, err := ParseFile(ipaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ipa: %w", err)
	}

	result := &DownloadResult{
		Name:             info.Name(),
		BundleIdentifier: info.Identifier(),
		Version:          info.Version(),
	}

	icon := info.Icon()
	if icon != nil {
		timestamp := time.Now().UnixMicro()
		name := sanitizeName(utils.FileNameWithoutExt(ipaPath))
		iconName := fmt.Sprintf("%s_%d.png", name, timestamp)
		iconPath := filepath.Join(saveDir, iconName)
		iconFile, err := os.Create(iconPath)
		if err == nil {
			if png.Encode(iconFile, icon) == nil {
				result.IconPath = iconPath
			}
			_ = iconFile.Close()
		}
	}

	return result, nil
}

// sanitizeName returns a safe filename component from a raw name.
func sanitizeName(name string) string {
	if name == "" {
		return "unknown"
	}
	return name
}

// progressWriter wraps an io.WriteCloser and reports write progress.
type progressWriter struct {
	dest       io.WriteCloser
	total      int64
	downloaded int64
	progressFn DownloadProgressFn
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.dest.Write(p)
	pw.downloaded += int64(n)
	if pw.progressFn != nil {
		pw.progressFn(pw.downloaded, pw.total)
	}
	return n, err
}
