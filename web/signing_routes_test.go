package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/gofiber/fiber/v2"
)

func TestReadSigningUploadSizeLimit(t *testing.T) {
	const limit = 16
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		data, err := readSigningUpload(c, "p12", "P12", limit)
		if err != nil {
			return c.JSON(apiSigningError(err))
		}
		return c.JSON(apiSuccess(len(data)))
	})

	cases := []struct {
		name     string
		size     int
		noFile   bool
		wantCode string
		wantOK   bool
	}{
		{name: "at the limit", size: limit, wantOK: true},
		{name: "one byte over the limit", size: limit + 1, wantCode: signing.CodeUploadTooLarge},
		{name: "missing file", noFile: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			if !tc.noFile {
				part, err := form.CreateFormFile("p12", "identity.p12")
				if err != nil {
					t.Fatal(err)
				}
				_, _ = part.Write(bytes.Repeat([]byte{0x30}, tc.size))
			}
			_ = form.WriteField("password", "")
			_ = form.Close()

			req := httptest.NewRequest(http.MethodPost, "/", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()

			var result struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if tc.wantOK {
				if result.Code != 200 || string(result.Data) != strconv.Itoa(tc.size) {
					t.Fatalf("code %d data %s, want success with %d bytes", result.Code, result.Data, tc.size)
				}
				return
			}
			if result.Code != -1 {
				t.Fatalf("code = %d, want -1", result.Code)
			}
			var data *SigningErrorData
			if err := json.Unmarshal(result.Data, &data); err != nil {
				t.Fatal(err)
			}
			gotCode := ""
			if data != nil {
				gotCode = data.Code
			}
			if gotCode != tc.wantCode {
				t.Fatalf("signing code = %q, want %q", gotCode, tc.wantCode)
			}
		})
	}
}
