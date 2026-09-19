package web

import (
	"bytes"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

const appleTouchIconPath = "/apple-touch-icon.png"

func TestAppleTouchIconSource(t *testing.T) {
	icon, err := os.ReadFile(filepath.Join("static", "public", "apple-touch-icon.png"))
	if err != nil {
		t.Fatalf("read source icon: %v", err)
	}
	assertAppleTouchIcon(t, icon)

	index, err := os.ReadFile(filepath.Join("static", "index.html"))
	if err != nil {
		t.Fatalf("read source index: %v", err)
	}
	want := `<link rel="apple-touch-icon" type="image/png" sizes="180x180" href="/apple-touch-icon.png" >`
	if !strings.Contains(string(index), want) {
		t.Fatalf("source index does not contain %q", want)
	}
	if !strings.Contains(string(index), `rel="shortcut icon"`) ||
		!strings.Contains(string(index), `href="/img/app.svg"`) {
		t.Fatal("source index no longer references the existing SVG favicon")
	}
}

func TestAppleTouchIconProductionAsset(t *testing.T) {
	assets := StaticAssets()
	index, err := assets.Open("index.html")
	if err != nil {
		t.Fatalf("open built index: %v", err)
	}
	defer func() { _ = index.Close() }()

	indexData, err := io.ReadAll(index)
	if err != nil {
		t.Fatalf("read built index: %v", err)
	}
	if !strings.Contains(string(indexData), `rel="apple-touch-icon"`) ||
		!strings.Contains(string(indexData), appleTouchIconPath) {
		t.Fatal("built index does not reference the Apple touch icon")
	}
	if !strings.Contains(string(indexData), `rel="shortcut icon"`) ||
		!strings.Contains(string(indexData), `href="/img/app.svg"`) {
		t.Fatal("built index no longer references the existing SVG favicon")
	}

	icon, err := assets.Open("apple-touch-icon.png")
	if err != nil {
		t.Fatalf("open built icon: %v", err)
	}
	defer func() { _ = icon.Close() }()
	iconData, err := io.ReadAll(icon)
	if err != nil {
		t.Fatalf("read built icon: %v", err)
	}
	assertAppleTouchIcon(t, iconData)

	app := fiber.New()
	app.Use("/", filesystem.New(filesystem.Config{Root: http.FS(assets)}))
	response, err := app.Test(httptest.NewRequest(http.MethodGet, appleTouchIconPath, nil))
	if err != nil {
		t.Fatalf("request icon: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("icon status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "image/png") {
		t.Fatalf("icon content type = %q, want image/png", response.Header.Get("Content-Type"))
	}
}

func assertAppleTouchIcon(t *testing.T, data []byte) {
	t.Helper()
	icon, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode icon: %v", err)
	}
	if format != "png" {
		t.Fatalf("icon format = %q, want png", format)
	}
	if got := icon.Bounds().Size(); got.X != 180 || got.Y != 180 {
		t.Fatalf("icon dimensions = %dx%d, want 180x180", got.X, got.Y)
	}

	for y := icon.Bounds().Min.Y; y < icon.Bounds().Max.Y; y++ {
		for x := icon.Bounds().Min.X; x < icon.Bounds().Max.X; x++ {
			_, _, _, alpha := icon.At(x, y).RGBA()
			if alpha != 0xffff {
				t.Fatalf("icon has transparent pixel at (%d, %d)", x, y)
			}
		}
	}
}
