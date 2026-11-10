package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register the decoders accepted for a custom app icon
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	conf "github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"golang.org/x/image/draw"
)

const (
	// MaxCustomIconUploadSize bounds one uploaded icon. An app icon is small;
	// this only has to keep a malformed or huge upload from being decoded in
	// memory.
	MaxCustomIconUploadSize = 10 << 20
	// maxCustomIconDimension and maxCustomIconPixels bound the decoded image,
	// so a small compressed file cannot expand into a huge bitmap.
	maxCustomIconDimension = 4096
	maxCustomIconPixels    = 16 << 20

	// maxStoredCustomIconDimension bounds the PNG kept on disk. An app icon is
	// only ever shown at a few hundred pixels and plumesign resizes it again,
	// so larger uploads are downscaled without changing their aspect ratio.
	maxStoredCustomIconDimension = 1024

	// customIconFileName is the name of the custom icon stored with an
	// installed app. Every upload is normalized to PNG, so the engine always
	// receives one format regardless of what the user picked.
	customIconFileName = "custom-icon.png"
)

// SaveCustomIconUpload decodes the uploaded image, normalizes it to PNG and
// writes it into <DataDir>/tmp, returning the path the install form submits.
// The engine (plumesign --custom-icon) reads the file when the app is signed.
func SaveCustomIconUpload(data []byte) (string, error) {
	if len(data) > MaxCustomIconUploadSize {
		return "", signing.Errorf(signing.ClassSigning, signing.CodeUploadTooLarge,
			"the custom icon exceeds the maximum size of %d bytes", MaxCustomIconUploadSize)
	}
	img, err := decodeCustomIcon(data)
	if err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon is not a readable PNG, JPEG or GIF image")
	}
	img = fitCustomIcon(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon could not be converted to PNG")
	}
	// Encoding can expand a compressed input (e.g. a noisy GIF): keep the
	// normalized file within the same bound as the upload itself.
	if buf.Len() > MaxCustomIconUploadSize {
		return "", signing.Errorf(signing.ClassSigning, signing.CodeUploadTooLarge,
			"the normalized custom icon exceeds the maximum size of %d bytes", MaxCustomIconUploadSize)
	}

	dir := filepath.Join(conf.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	dst := filepath.Join(dir, fmt.Sprintf("custom_icon_%d.png", time.Now().UnixMicro()))
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("failed to save the custom icon: %w", err)
	}
	return dst, nil
}

// fitCustomIcon downscales an oversized upload to
// maxStoredCustomIconDimension while preserving its aspect ratio. It never
// enlarges a small icon.
func fitCustomIcon(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxStoredCustomIconDimension && h <= maxStoredCustomIconDimension {
		return img
	}
	scale := float64(maxStoredCustomIconDimension) / float64(w)
	if r := float64(maxStoredCustomIconDimension) / float64(h); r < scale {
		scale = r
	}
	dw, dh := int(float64(w)*scale), int(float64(h)*scale)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// decodeCustomIcon decodes one uploaded icon after checking its dimensions, so
// plumesign never has to read a file it cannot open as an image.
func decodeCustomIcon(data []byte) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if format != "png" && format != "jpeg" && format != "gif" {
		return nil, fmt.Errorf("unsupported image format %q", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxCustomIconDimension || cfg.Height > maxCustomIconDimension ||
		int64(cfg.Width)*int64(cfg.Height) > maxCustomIconPixels {
		return nil, errors.New("image dimensions are too large")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// ResolveCustomIconPath validates a client-supplied custom icon path and
// returns its canonical form. Empty means no custom icon. Only a readable
// image inside the upload directory (<DataDir>/tmp) or the installed apps
// directory (<DataDir>/ipa) is accepted, so an install request cannot point
// the engine at an arbitrary file on the server.
func ResolveCustomIconPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	dataDir := conf.Config.Server.DataDir
	resolved, err := resolvePathWithin(path, filepath.Join(dataDir, "tmp"), filepath.Join(dataDir, "ipa"))
	if err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon is not an uploaded or stored icon")
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon could not be read")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxCustomIconUploadSize+1))
	if err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon could not be read")
	}
	if len(data) > MaxCustomIconUploadSize {
		return "", signing.Errorf(signing.ClassSigning, signing.CodeCustomIconInvalid,
			"the custom icon exceeds the maximum size of %d bytes", MaxCustomIconUploadSize)
	}
	if _, err := decodeCustomIcon(data); err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeCustomIconInvalid, err,
			"the custom icon is not a readable image")
	}
	return resolved, nil
}

// ResolveAppDisplayIconPath returns the icon to show for an installed app: the
// user's custom icon when one is stored, otherwise the icon extracted from the
// IPA. A stored custom icon whose file is missing or unreadable falls back to
// the IPA icon, so a stale record never leaves the UI with a broken image.
func ResolveAppDisplayIconPath(app model.InstalledApp) (string, error) {
	if app.CustomIconPath != "" {
		if path, err := resolvePathWithin(app.CustomIconPath, filepath.Join(conf.Config.Server.DataDir, "ipa")); err == nil {
			return path, nil
		}
	}
	return ResolveAppIconPath(app.Icon)
}

// storeCustomIcon persists the user-chosen icon of app into saveDir and
// returns the path to record. The icon is copied rather than moved because the
// same upload can be referenced by several install requests and the temp-file
// cleanup owns files in <DataDir>/tmp. An empty icon clears the choice.
func storeCustomIcon(icon, saveDir, current string) string {
	if icon == "" {
		return ""
	}
	dataDir := conf.Config.Server.DataDir
	resolved, err := resolvePathWithin(icon, filepath.Join(dataDir, "tmp"), filepath.Join(dataDir, "ipa"))
	if err != nil {
		log.Warnf("Ignoring the custom app icon %s: %s", icon, err)
		return current
	}
	dst := filepath.Join(saveDir, customIconFileName)
	if resolved == dst {
		return dst
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		log.Warnf("Cannot read the custom app icon %s: %s", resolved, err)
		return current
	}
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Warnf("Cannot create %s: %s", saveDir, err)
		return current
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		log.Warnf("Cannot store the custom app icon %s: %s", dst, err)
		return current
	}
	return dst
}
