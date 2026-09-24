package web

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func jpegPayload() string {
	data := append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 32)...)
	return base64.StdEncoding.EncodeToString(data)
}

func screenshotForm(t *testing.T, data string) *http.Request {
	t.Helper()
	body := url.Values{"data": {data}}.Encode()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/devices/screenshot/download",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestScreenshotDownloadRespondsWithAttachment(t *testing.T) {
	fi := fiber.New()
	route(fi)

	response, err := fi.Test(screenshotForm(t, jpegPayload()))
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", got)
	}
	disposition := response.Header.Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("Content-Disposition = %q, want an attachment", disposition)
	}
	if !strings.Contains(disposition, "screenshot-") || !strings.Contains(disposition, ".jpg") {
		t.Fatalf("Content-Disposition = %q, want a screenshot filename", disposition)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) == 0 || body[0] != 0xFF || body[1] != 0xD8 || body[2] != 0xFF {
		t.Fatal("response body is not the posted JPEG")
	}
}

func TestScreenshotDownloadRejectsBadInput(t *testing.T) {
	fi := fiber.New()
	route(fi)

	cases := map[string]string{
		"not base64": "!!!!",
		"not a jpeg": base64.StdEncoding.EncodeToString([]byte("hello")),
		"empty":      "",
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := fi.Test(screenshotForm(t, payload))
			if err != nil {
				t.Fatalf("download request: %v", err)
			}
			defer func() { _ = response.Body.Close() }()

			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
			}
		})
	}
}
