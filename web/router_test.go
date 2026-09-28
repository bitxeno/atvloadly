package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/gofiber/fiber/v2"
)

// newTestServer returns the routed server over a fresh data directory and
// database. It returns the data directory.
func newTestServer(t *testing.T) (*fiber.App, string) {
	t.Helper()
	previous := app.Config
	dataDir := t.TempDir()
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir
	t.Cleanup(func() { app.Config = previous })

	store := db.Open(db.Config{Path: t.TempDir(), FileName: "test.db"})
	if err := store.AutoMigrate(&model.InstalledApp{}, &model.SigningIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.Store().DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	server := fiber.New()
	route(server)
	return server, dataDir
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The icon route serves the icons stored with installed apps only: a record
// pointing anywhere else must not expose that file.
func TestAppIconServesOnlyStoredIcons(t *testing.T) {
	server, dataDir := newTestServer(t)
	stored := writeFile(t, filepath.Join(dataDir, "ipa", "1", "app.png"), "stored icon")
	uploaded := writeFile(t, filepath.Join(dataDir, "tmp", "app_1.png"), "uploaded icon")
	outside := writeFile(t, filepath.Join(t.TempDir(), "secret"), "secret")

	tests := []struct {
		name     string
		icon     string
		wantCode int
	}{
		{name: "stored icon", icon: stored, wantCode: http.StatusOK},
		{name: "outside the data directory", icon: outside, wantCode: http.StatusNotFound},
		{name: "upload directory", icon: uploaded, wantCode: http.StatusNotFound},
		{name: "no icon", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := model.InstalledApp{IpaName: "Test", Icon: tt.icon}
			if err := db.Store().Create(&record).Error; err != nil {
				t.Fatal(err)
			}
			resp, err := server.Test(httptest.NewRequest(http.MethodGet, "/apps/"+strconv.FormatUint(uint64(record.ID), 10)+"/icon", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}
			if tt.wantCode == http.StatusOK && string(body) != "stored icon" {
				t.Fatalf("body = %q, want the stored icon", body)
			}
			if tt.wantCode != http.StatusOK && len(body) != 0 {
				t.Fatalf("refused icon leaked %q", body)
			}
		})
	}
}

// A custom bundle identifier is refused before anything is queued when the
// installation cannot use it.
func TestInstallRefusesUnusableCustomIdentifier(t *testing.T) {
	server, dataDir := newTestServer(t)
	identity := model.SigningIdentity{Name: "Test", CertificateSHA256: "cert", ProfileUUID: "profile", TeamID: "ABCDE12345"}
	if err := db.Store().Create(&identity).Error; err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		fields   map[string]string
		wantCode string
	}{
		{
			name:   "apple id",
			fields: map[string]string{"account": "user@example.com", "custom_identifier": "com.example.custom"},
		},
		{
			name:     "invalid identifier",
			fields:   map[string]string{"signing_mode": string(model.SigningModeExternalCertificate), "signing_identity_id": strconv.FormatUint(uint64(identity.ID), 10), "custom_identifier": "com.example/custom"},
			wantCode: signing.CodeCustomIdentifierInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			for k, v := range tt.fields {
				_ = form.WriteField(k, v)
			}
			part, err := form.CreateFormFile("file", "App.ipa")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = part.Write([]byte("ipa"))
			_ = form.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/install", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			resp, err := server.Test(req)
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
			if gotCode != tt.wantCode {
				t.Fatalf("signing code = %q, want %q", gotCode, tt.wantCode)
			}
			if entries, _ := os.ReadDir(filepath.Join(dataDir, "tmp")); len(entries) != 0 {
				t.Fatalf("the refused upload was kept: %v", entries)
			}
		})
	}
}
