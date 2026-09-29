package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/signing"
)

func setupGithubTokenSettings(t *testing.T, token string) {
	t.Helper()
	oldSettings := app.Settings
	oldConfig := app.Config
	t.Cleanup(func() { app.Settings = oldSettings; app.Config = oldConfig })

	app.Settings = &app.SettingsConfiguration{}
	app.Settings.Update.CheckInterval = 6
	app.Settings.Task.Enabled = true
	app.Settings.Task.CrodTime = "0,30 3-6 * * *"
	app.Settings.Task.AdvanceDays = 1
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = t.TempDir()

	keyFile := filepath.Join(t.TempDir(), "keys", "signing-identity.key")
	signing.Configure(keyFile, t.TempDir())
	t.Cleanup(func() { signing.Configure("", "") })

	if token != "" {
		if err := app.SetGitHubToken(token); err != nil {
			t.Fatalf("SetGitHubToken: %v", err)
		}
	}
}

func decodeAPI(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if result.Code != 200 {
		t.Fatalf("code = %d (%s), body %q", result.Code, result.Msg, body)
	}
	if len(result.Data) == 0 || string(result.Data) == "true" || string(result.Data) == "null" {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatalf("decode data %q: %v", result.Data, err)
	}
	return data
}

func TestGetSettingsNeverLeaksGitHubToken(t *testing.T) {
	const token = "ghp_1234567890abcdef1234567890ab"
	setupGithubTokenSettings(t, token)
	server, _ := newTestServer(t)

	resp, err := server.Test(httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if err != nil {
		t.Fatal(err)
	}
	data := decodeAPI(t, resp)
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), token) {
		t.Fatal("GET /api/settings leaks the full token")
	}
	update, _ := data["update"].(map[string]any)
	if update == nil {
		t.Fatal("missing update section")
	}
	if _, ok := update["github_token"]; ok {
		t.Fatal("GET /api/settings exposes github_token")
	}
	if _, ok := update["github_token_sealed"]; ok {
		t.Fatal("GET /api/settings exposes the sealed token")
	}
	if _, ok := update["github_token_masked"]; ok {
		t.Fatal("GET /api/settings exposes a token mask")
	}
	if update["github_token_set"] != true {
		t.Fatalf("github_token_set = %v, want true", update["github_token_set"])
	}
}

func TestSaveUpdateSettingsBlankRemovesToken(t *testing.T) {
	const token = "ghp_1234567890abcdef1234567890ab"
	setupGithubTokenSettings(t, token)
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"check_interval":12,"github_token":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = decodeAPI(t, resp)
	if app.GitHubTokenConfigured() {
		t.Fatal("blank save did not remove the token")
	}
	if app.Settings.Update.CheckInterval != 12 {
		t.Fatalf("interval = %d, want 12", app.Settings.Update.CheckInterval)
	}
}

func TestSaveUpdateSettingsAbsentTokenKeepsIt(t *testing.T) {
	const token = "ghp_1234567890abcdef1234567890ab"
	setupGithubTokenSettings(t, token)
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"check_interval":12}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = decodeAPI(t, resp)
	if got := app.GetGitHubToken(); got != token {
		t.Fatalf("absent token field cleared it, got %q", got)
	}
}

func TestSaveUpdateSettingsReplacesAndClearsToken(t *testing.T) {
	setupGithubTokenSettings(t, "")
	server, _ := newTestServer(t)

	const token = "github_pat_1234567890abcdefghij1234567890abcd"
	req := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"check_interval":6,"github_token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if resp, err := server.Test(req); err != nil {
		t.Fatal(err)
	} else {
		_ = decodeAPI(t, resp)
	}
	if got := app.GetGitHubToken(); got != token {
		t.Fatalf("token = %q, want the new value", got)
	}

	clearReq := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"check_interval":6,"clear_github_token":true}`))
	clearReq.Header.Set("Content-Type", "application/json")
	if resp, err := server.Test(clearReq); err != nil {
		t.Fatal(err)
	} else {
		_ = decodeAPI(t, resp)
	}
	if app.GitHubTokenConfigured() {
		t.Fatal("clear flag did not remove the token")
	}

	maskedReq := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"github_token":"ghp_****abcd"}`))
	maskedReq.Header.Set("Content-Type", "application/json")
	resp, err := server.Test(maskedReq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != -1 {
		t.Fatalf("masked preview accepted, code = %d", result.Code)
	}
}
