package manager

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "image/png" // register PNG decoder for image.Decode

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"golang.org/x/image/draw"
)

// jpegMaxWidth and jpegMaxHeight bound the encoded screenshot. The original
// PNG is rescaled to fit within these dimensions while preserving aspect
// ratio and re-encoded as JPEG at jpegQuality.
const (
	jpegMaxWidth  = 1920
	jpegMaxHeight = 1080
	jpegQuality   = 80
)

// mountedDevices tracks devices for which the personalized developer disk
// image has already been mounted. The mount operation is expensive so we
// keep a per-process cache and skip it on subsequent screenshot requests.
var mountedDevices sync.Map

const defaultDeveloperDiskImageRepoBaseURL = "https://raw.githubusercontent.com/bitxeno/DeveloperDiskImages/main"

var developerDiskImageRepoBaseURL string

// ScreenshotManager orchestrates mounting the developer disk image
// and capturing a screenshot of the target device over an RSD tunnel.
type ScreenshotManager struct {
	outputDir string
}

func NewScreenshotManager() *ScreenshotManager {
	return &ScreenshotManager{
		outputDir: filepath.Join(os.TempDir(), "atvloadly-screenshot"),
	}
}

// EnsureMounted mounts the personalized developer disk image on the device
// if it has not been mounted yet in this process. The mount state is shared
// across all callers, matching the requirement that mount only needs to
// happen once before the first screenshot.
func (m *ScreenshotManager) EnsureMounted(ctx context.Context, dev *model.Device) (string, error) {
	if dev.Connection != model.DeviceConnectionRemote {
		return "", fmt.Errorf("mount is only supported on remote paired (RSD) devices")
	}

	if _, ok := mountedDevices.Load(dev.ID); ok {
		return "", nil
	}

	imagePath, manifestPath, trustCachePath, err := resolveDeveloperDiskImage(dev)
	if err != nil {
		return "", err
	}

	port := fmt.Sprintf("%d", dev.Port)
	args := []string{"mount", "--ip", dev.IP, "--port", port, "--udid", dev.UDID}
	args = append(args, "--image", imagePath, "--manifest", manifestPath, "--trustcache", trustCachePath)

	log.Debugf("Mount Command: plumesign %s", strings.Join(args, " "))

	timeout := 2 * time.Minute
	output, err := exec.CommandContext(ctx, "plumesign", args...).
		WithTimeout(timeout).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Mount developer disk image failed: %s", string(output))
		return string(output), fmt.Errorf("mount developer disk image failed: %s%s", err.Error(), string(output))
	}

	mountedDevices.Store(dev.ID, struct{}{})
	log.Infof("Mounted developer disk image for device %s (%s)", dev.Name, dev.UDID)
	return string(output), nil
}

// TakeScreenshot runs plumesign screenshot, transcodes the resulting PNG into
// a 1920x1080 (max) JPEG at 80% quality and returns the JPEG bytes. The
// intermediate PNG file is removed before returning.
func (m *ScreenshotManager) TakeScreenshot(ctx context.Context, dev *model.Device) ([]byte, error) {
	if dev.Connection != model.DeviceConnectionRemote {
		return nil, fmt.Errorf("screenshot is only supported on remote paired (RSD) devices")
	}

	if err := os.MkdirAll(m.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create screenshot output dir: %w", err)
	}

	outputPath := filepath.Join(m.outputDir, fmt.Sprintf("screenshot-%d.png", time.Now().UnixNano()))
	port := fmt.Sprintf("%d", dev.Port)

	args := []string{"screenshot", "--ip", dev.IP, "--port", port, "--udid", dev.UDID, "--output", outputPath}

	log.Debugf("Screenshot Command: plumesign %s", strings.Join(args, " "))

	timeout := 30 * time.Second
	_, err := exec.CommandContext(ctx, "plumesign", args...).
		WithTimeout(timeout).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	// Always remove the original PNG, success or failure.
	defer func() { _ = os.Remove(outputPath) }()
	if err != nil {
		log.Err(err).Msgf("Screenshot failed")
		return nil, fmt.Errorf("screenshot failed: %s", err.Error())
	}

	pngBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot file: %w", err)
	}

	jpgBytes, err := transcodeToJPEG(pngBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to transcode screenshot: %w", err)
	}
	return jpgBytes, nil
}

// transcodeToJPEG decodes a PNG, rescales it to fit within jpegMaxWidth /
// jpegMaxHeight (preserving aspect ratio) and re-encodes it as JPEG at
// jpegQuality. The original PNG byte slice is no longer needed once the
// decoded image has been produced.
func transcodeToJPEG(pngBytes []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	dstW, dstH := fitWithin(srcW, srcH, jpegMaxWidth, jpegMaxHeight)

	if dstW != srcW || dstH != srcH {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// fitWithin returns the largest (width, height) pair whose aspect ratio
// matches (srcW, srcH) and that fits inside (maxW, maxH).
func fitWithin(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH
	}
	ratioW := float64(maxW) / float64(srcW)
	ratioH := float64(maxH) / float64(srcH)
	scale := ratioW
	if ratioH < ratioW {
		scale = ratioH
	}
	return int(float64(srcW) * scale), int(float64(srcH) * scale)
}

// ScreenshotDataBase64 returns the base64 encoding of the screenshot bytes.
func ScreenshotDataBase64(png []byte) string {
	return base64.StdEncoding.EncodeToString(png)
}

// ResetMountedState clears the cached mount state. Intended for tests.
func ResetMountedState() {
	mountedDevices = sync.Map{}
}

// resolveDeveloperDiskImage locates the developer disk image files for the device.
func resolveDeveloperDiskImage(dev *model.Device) (string, string, string, error) {
	root := developerDiskImageRoot()
	if root == "" {
		return "", "", "", fmt.Errorf("developer disk image root directory not found")
	}

	subdir := platformSubdir(dev)
	if subdir == "" {
		return "", "", "", fmt.Errorf("unsupported device class: %s", dev.DeviceClass)
	}

	ddiDir := filepath.Join(root, subdir)
	dmgPath := filepath.Join(ddiDir, "Image.dmg")
	manifestPath := filepath.Join(ddiDir, "BuildManifest.plist")
	trustCachePath := filepath.Join(ddiDir, "Image.dmg.trustcache")

	if err := ensureDeveloperDiskImageFiles(subdir, map[string]string{
		"Image.dmg":            dmgPath,
		"BuildManifest.plist":  manifestPath,
		"Image.dmg.trustcache": trustCachePath,
	}); err != nil {
		return "", "", "", err
	}

	return dmgPath, manifestPath, trustCachePath, nil
}

func developerDiskImageRoot() string {
	if app.Config == nil || app.Config.Server.DataDir == "" {
		return ""
	}
	return filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImages")
}

func ensureDeveloperDiskImageFiles(subdir string, files map[string]string) error {
	for name, fullPath := range files {
		if _, err := os.Stat(fullPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", name, err)
		}

		if err := downloadDeveloperDiskImageFile(subdir, name, fullPath); err != nil {
			return err
		}
	}
	return nil
}

func downloadDeveloperDiskImageFile(subdir, fileName, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create developer disk image dir: %w", err)
	}

	url := strings.TrimRight(developerDiskImageDownloadBaseURL(), "/") + "/" + subdir + "/" + fileName
	resp, err := stdhttp.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", fileName, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != stdhttp.StatusOK {
		return fmt.Errorf("download %s failed with status %d", fileName, resp.StatusCode)
	}

	tmpPath := destination + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", fileName, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("save %s: %w", fileName, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file for %s: %w", fileName, err)
	}
	if err := os.Rename(tmpPath, destination); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move %s into place: %w", fileName, err)
	}

	return nil
}

func developerDiskImageDownloadBaseURL() string {
	if developerDiskImageRepoBaseURL != "" {
		return developerDiskImageRepoBaseURL
	}
	if app.Config != nil {
		imageSource := strings.TrimSpace(app.Config.App.DeveloperDiskImage.ImageSource)
		if imageSource != "" && !strings.Contains(imageSource, "{0}") {
			return imageSource
		}
	}
	return defaultDeveloperDiskImageRepoBaseURL
}

func platformSubdir(dev *model.Device) string {
	class := strings.ToLower(dev.DeviceClass)
	switch class {
	case "appletv":
		return "tvOS_DDI"
	case "iphone", "ipad":
		return "iOS_DDI"
	case "watch":
		return "watchOS_DDI"
	case "vision", "xros":
		return "xrOS_DDI"
	}
	return ""
}
