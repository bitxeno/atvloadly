package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func TestStaticDocumentsDisableCaching(t *testing.T) {
	app := fiber.New()
	app.Use("/", preventDocumentCaching)
	app.Use("/", filesystem.New(filesystem.Config{Root: http.FS(StaticAssets())}))

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, requestPath := range []string{"/", "/index.html"} {
			response, err := app.Test(httptest.NewRequest(method, requestPath, nil))
			if err != nil {
				t.Fatalf("%s %s: %v", method, requestPath, err)
			}
			defer func() { _ = response.Body.Close() }()

			if got := response.Header.Get(fiber.HeaderCacheControl); got != documentCacheControl {
				t.Errorf("%s %s Cache-Control = %q, want %q", method, requestPath, got, documentCacheControl)
			}
		}
	}
}
